package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBusinessLineageSnapshotExportsSemanticDependencies(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Coverage = nil
	g.BusinessContract.Branches[0].When = BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		businessPredicate("eq", "primary", "active", "boolean", "true"),
		businessPredicate("eq", "other", "active", "boolean", "true"),
		businessPredicate("eq", "order", "stored", "boolean", "true"),
	}}
	g.BusinessContract.Branches[1].When = businessNot(g.BusinessContract.Branches[0].When)
	d := modelTestDB(t, g)
	snapshot, err := d.buildSnapshot(SnapshotRequest{Graph: g.ID})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	if len(document["business_lineage"]) == 0 {
		t.Fatal("snapshot exports preparation declarations but no branch target, state requirement, condition tree or declared provider genealogy")
	}
}

func TestBusinessLineageRelationNeedsAllDeclaredValues(t *testing.T) {
	g := contractModel(t)
	g.BusinessContract.Coverage = nil
	guard := BusinessPredicate{Op: "eq", Left: businessRef("primary", "id"), Right: businessRef("other", "id")}
	g.BusinessContract.Branches[0].When, g.BusinessContract.Branches[1].When = guard, businessNot(guard)
	g.BusinessContract.Preparation = []PreparationAction{{ID: "same-owner", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: BusinessPredicate{Op: "true"}, Effects: []PreparationEffect{{Subject: "primary", Attribute: "id", Value: json.RawMessage(`"acct-A"`)}}, Source: "document", SourceRef: "test-spec:owner"}}
	branch := lineageTestBranch(t, g, "accept")
	if lineageTestEdge(branch, "prep:same-owner", "branch:accept:when", "candidate_provider") {
		t.Fatal("a relationship missing one side became a complete provider")
	}
	g.BusinessContract.Preparation[0].Effects = append(g.BusinessContract.Preparation[0].Effects, PreparationEffect{Subject: "other", Attribute: "id", Value: json.RawMessage(`"acct-A"`)})
	branch = lineageTestBranch(t, g, "accept")
	if !lineageTestEdge(branch, "prep:same-owner", "branch:accept:when", "candidate_provider") {
		t.Fatal("two explicitly equal declared values lost their relationship provider")
	}
	g.BusinessContract.Preparation[0].Effects[1].Value = json.RawMessage(`"acct-B"`)
	branch = lineageTestBranch(t, g, "accept")
	if lineageTestEdge(branch, "prep:same-owner", "branch:accept:when", "candidate_provider") {
		t.Fatal("opposite relationship results became a provider")
	}
}

func TestBusinessLineagePreservesBooleanStructureAndNegativeProviders(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Coverage = nil
	guard := BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		businessPredicate("eq", "primary", "active", "boolean", "true"),
		{Op: "or", Args: []BusinessPredicate{
			businessPredicate("eq", "other", "active", "boolean", "true"),
			businessNot(businessPredicate("eq", "order", "stored", "boolean", "true")),
		}},
	}}
	g.BusinessContract.Branches[0].When, g.BusinessContract.Branches[1].When = guard, businessNot(guard)
	g.BusinessContract.Preparation = append(g.BusinessContract.Preparation, PreparationAction{ID: "remove-order", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: businessPredicate("eq", "order", "stored", "boolean", "false"), Effects: []PreparationEffect{{Subject: "order", Attribute: "stored", Value: json.RawMessage("false")}}, Source: "document", SourceRef: "test-spec:remove-order"})
	branch := lineageTestBranch(t, g, "accept")
	want := map[string]string{"branch:accept:when": "and", "branch:accept:when:1": "or", "branch:accept:when:1:1": "not"}
	for _, node := range branch.Nodes {
		if op, exists := want[node.ID]; exists {
			if node.Kind != "logic" || node.Predicate == nil || node.Predicate.Op != op {
				t.Fatalf("boolean expression lost its declared operator: %+v", node)
			}
			delete(want, node.ID)
		}
	}
	if len(want) != 0 || !lineageTestEdge(branch, "prep:remove-order", "branch:accept:when:1:1", "candidate_provider") {
		t.Fatalf("negative condition lost its true provider or expression node: %+v", want)
	}
	for _, edge := range branch.Edges {
		if edge.Kind == "candidate_provider" && edge.To == "branch:accept:when:1:1:0" {
			t.Fatal("a provider for NOT's positive child was advertised as preparing the negative condition")
		}
		if edge.Kind == "logical_operand" && edge.To == "branch:accept:when:1" && !edge.Alternative {
			t.Fatal("OR operands became simultaneous preparation obligations")
		}
	}
}

func TestBusinessLineageSeparatesStateCyclesAndDoesNotInventPreparationAncestry(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Coverage = nil
	g.Edges = append(g.Edges, Edge{ID: "retry-enroll", From: "ready", To: "pending", Kind: "transitions_to"})
	g.Transitions = append(g.Transitions, TransitionContract{Edge: "retry-enroll", Action: "enroll", Rule: "R1", Preconditions: ConditionAssignment{"login": true}, Outcome: branchRetry})
	g.BusinessContract.Branches[0].When = businessPredicate("eq", "order", "stored", "boolean", "true")
	g.BusinessContract.Branches[1].When = businessNot(g.BusinessContract.Branches[0].When)
	g.BusinessContract.Preparation[2].DependsOn = nil
	branch := lineageTestBranch(t, g, "accept")
	for _, node := range branch.Nodes {
		if node.Preparation == "activate-primary" || node.Preparation == "activate-other" {
			t.Fatal("shared attribute names invented an undeclared preparation ancestor")
		}
	}
	parents := map[string][]string{}
	for _, edge := range branch.Edges {
		if edge.From == "ready" || edge.From == "pending" || edge.Kind == "transitions_to" {
			t.Fatal("business state cycle was copied into the dependency DAG")
		}
		parents[edge.To] = append(parents[edge.To], edge.From)
	}
	if cycle := dependencyCycle(parents); len(cycle) > 0 {
		t.Fatalf("lineage invented a dependency cycle: %+v", cycle)
	}
	g.BusinessContract.Preparation[0].DependsOn = []string{"prepare-order"}
	g.BusinessContract.Preparation[2].DependsOn = []string{"activate-primary"}
	if _, err := buildBusinessLineage(g); err == nil || !strings.Contains(err.Error(), "preparation dependency cycle") {
		t.Fatalf("invalid preparation cycle was accepted: %v", err)
	}
}

func TestBusinessLineageConstantGuardDoesNotSelectEveryPreparation(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Coverage = nil
	g.BusinessContract.Branches[0].When = BusinessPredicate{Op: "true"}
	branch := lineageTestBranch(t, g, "accept")
	for _, node := range branch.Nodes {
		if node.Kind == "preparation" {
			t.Fatal("constant guard invented a preparation requirement")
		}
	}
}

func TestBusinessLineageContradictoryPreparationDoesNotOfferProvider(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Preparation[0].Ensures = businessNot(businessPredicate("eq", "primary", "active", "boolean", "true"))
	branch := lineageTestBranch(t, g, "accept")
	if lineageTestEdge(branch, "prep:activate-primary", "branch:accept:when", "candidate_provider") {
		t.Fatal("a preparation whose own declared effects contradict its ensures was offered as a provider")
	}
	g.BusinessContract.Preparation[0].Ensures = businessPredicate("eq", "other", "active", "boolean", "true")
	branch = lineageTestBranch(t, g, "accept")
	if !lineageTestEdge(branch, "prep:activate-primary", "branch:accept:when", "candidate_provider") {
		t.Fatal("an ensures needing additional independent observations was incorrectly treated as a proven contradiction")
	}
}

func TestBusinessLineageTypesMatchFrontend(t *testing.T) {
	interfaces := tsInterfaces(t, "workbench/src/lineage-types.ts")
	for name, value := range map[string]reflect.Type{
		"BusinessLineage":          reflect.TypeOf(BusinessLineage{}),
		"BusinessBranchLineage":    reflect.TypeOf(BusinessBranchLineage{}),
		"BusinessLineageReference": reflect.TypeOf(BusinessLineageReference{}),
		"BusinessLineageNode":      reflect.TypeOf(BusinessLineageNode{}),
		"BusinessLineageEdge":      reflect.TypeOf(BusinessLineageEdge{}),
		"BusinessLineageOverlay":   reflect.TypeOf(BusinessLineageOverlay{}),
		"BusinessLineageReading":   reflect.TypeOf(BusinessLineageReading{}),
		"BusinessLineageValue":     reflect.TypeOf(BusinessLineageValue{}),
		"BusinessLineageSource":    reflect.TypeOf(BusinessLineageSource{}),
		"LineageEvaluation":        reflect.TypeOf(TruthEvaluation{}),
	} {
		want := uniqueIDs(goJSONFields(value))
		if got, exists := interfaces[name]; !exists || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s frontend fields %v differ from lineage export %v", name, got, want)
		}
	}
}

func TestBusinessLineageBudgetRefusesOversizedProjection(t *testing.T) {
	for _, limit := range []string{"nodes", "edges"} {
		t.Run(limit, func(t *testing.T) {
			g := preparationModel(t)
			g.BusinessContract.Coverage = nil
			base, preparation := g.BusinessContract.Branches[0], g.BusinessContract.Preparation[0]
			args, branches, preparations := 128, 512, 0
			if limit == "edges" {
				args, branches, preparations = 1024, 1, 128
			}
			guard := BusinessPredicate{Op: "and", Args: []BusinessPredicate{}}
			for index := 0; index < args; index++ {
				guard.Args = append(guard.Args, businessPredicate("eq", "primary", "active", "boolean", "true"))
			}
			g.BusinessContract.Branches, g.BusinessContract.Preparation = nil, nil
			for index := 0; index < branches; index++ {
				branch := base
				branch.ID, branch.When = fmt.Sprintf("branch-%d", index), guard
				g.BusinessContract.Branches = append(g.BusinessContract.Branches, branch)
			}
			for index := 0; index < preparations; index++ {
				candidate := preparation
				candidate.ID = fmt.Sprintf("provider-%d", index)
				g.BusinessContract.Preparation = append(g.BusinessContract.Preparation, candidate)
			}
			lineage, err := buildBusinessLineage(g)
			if lineage != nil || !errors.Is(err, errBusinessLineageBudget) || !strings.Contains(err.Error(), limit) {
				t.Fatalf("oversized %s must refuse export as a budget failure, never return a partial graph or downgrade to damaged evidence: graph=%v error=%v", limit, lineage != nil, err)
			}
		})
	}
}

func lineageTestBranch(t *testing.T, g Graph, id string) BusinessBranchLineage {
	t.Helper()
	lineage, err := buildBusinessLineage(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, branch := range lineage.Branches {
		if branch.Branch == id {
			return branch
		}
	}
	t.Fatalf("lineage branch %s is missing", id)
	return BusinessBranchLineage{}
}

func lineageTestEdge(branch BusinessBranchLineage, from, to, kind string) bool {
	for _, edge := range branch.Edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return true
		}
	}
	return false
}

func TestBusinessLineageBoundSubjectsHaveDeclaredProviders(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Coverage = nil
	g.BusinessContract.Subjects[2].RequiresBinding = true
	g.BusinessContract.Subjects[2].Attributes[2].SourceRef = "test-spec:order-stored"
	guard := BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		businessPredicate("eq", "primary", "active", "boolean", "true"),
		businessPredicate("eq", "other", "active", "boolean", "true"),
		businessPredicate("eq", "order", "stored", "boolean", "true"),
	}}
	g.BusinessContract.Branches[0].When, g.BusinessContract.Branches[1].When = guard, businessNot(guard)
	branch := lineageTestBranch(t, g, "accept")
	for _, edge := range [][2]string{
		{"prep:activate-primary", "branch:accept:when:0"},
		{"prep:activate-other", "branch:accept:when:1"},
		{"prep:prepare-order", "branch:accept:when:2"},
		{"prep:activate-primary", "prep:prepare-order:requires:0"},
		{"prep:activate-other", "prep:prepare-order:requires:1"},
	} {
		if !lineageTestEdge(branch, edge[0], edge[1], "candidate_provider") {
			t.Fatalf("declared provider of bound subject vanished from model-only lineage: %s -> %s", edge[0], edge[1])
		}
	}
	foundSource := false
	for _, node := range branch.Nodes {
		for _, ref := range node.References {
			if ref.Subject == "order" && ref.Attribute == "stored" {
				foundSource = foundSource || ref.SourceRef == "test-spec:order-stored" && ref.SubjectKind == "entity"
			}
		}
	}
	if !foundSource {
		t.Fatal("condition genealogy lost the attribute-level rule source")
	}
	g.BusinessContract.Preparation[0].Effects[0].Value = json.RawMessage("false")
	branch = lineageTestBranch(t, g, "accept")
	if lineageTestEdge(branch, "prep:activate-primary", "branch:accept:when:0", "candidate_provider") {
		t.Fatal("same attribute with the opposite effect became a provider")
	}
}

// A real field is a first-class dependency, not only an opaque condition label.
// The post-action expectation is a declaration, never an observed before fact.
func TestBusinessLineageVersion2ExportsDataAndExpectedWithoutInventingExecution(t *testing.T) {
    g := preparationModel(t)
    g.BusinessContract.Coverage = nil
    branch := lineageTestBranch(t, g, "accept")
    lineage, err := buildBusinessLineage(g)
    if err != nil || lineage.Version != 2 {
        t.Fatalf("expected v2 genealogy: %v, version=%d", err, lineage.Version)
    }
    data, expected, reads, expects := 0, 0, 0, 0
    for _, node := range branch.Nodes {
        switch node.Kind {
        case "input_data":
            data++
            if len(node.References) != 1 || node.References[0].Attribute == "" {
                t.Fatalf("input data lacks exact attributed source: %+v", node)
            }
        case "expected":
            expected++
            if node.Predicate != nil || len(node.References) != 0 {
                t.Fatalf("post expectation incorrectly reads before-state: %+v", node)
            }
        }
    }
    for _, edge := range branch.Edges {
        switch edge.Kind {
        case "reads":
            reads++
        case "expects":
            expects++
            if edge.From != branch.Target {
                t.Fatalf("outcome lost its owning target: %+v", edge)
            }
        }
    }
    if data == 0 || reads == 0 ||
       expected != len(g.BusinessContract.Branches[0].Expected.Outcomes) ||
       expected != expects {
        t.Fatalf("incomplete v2 source/outcome genealogy: data=%d reads=%d expected=%d expects=%d", data, reads, expected, expects)
    }
}
