package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func cliJSON(t *testing.T, args ...string) []byte {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = old; reader.Close(); writer.Close() }()
	type readResult struct {
		body []byte
		err  error
	}
	read := make(chan readResult, 1)
	go func() { body, err := io.ReadAll(reader); read <- readResult{body, err} }()
	err = run(args)
	writer.Close()
	os.Stdout = old
	output := <-read
	if err != nil {
		t.Fatalf("CLI %s: %v", args[0], err)
	}
	if output.err != nil {
		t.Fatal(output.err)
	}
	return output.body
}

func correlatedModel(t *testing.T) Graph {
	graph := referenceModel(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "quota", Kind: "condition", Name: "Reference quota"})
	graph.Constraints = []ConditionClause{{ID: "authenticated-quota", Literals: []ConditionLiteral{{Condition: "login", Value: false}, {Condition: "quota", Value: true}}, Source: "document", SourceRef: "reference-fixture"}}
	return graph
}

func TestAnalysisConditionProofThroughCLI(t *testing.T) {
	graph := correlatedModel(t)
	body, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	file := writeBusinessModel(t, string(body))
	for _, scenario := range []struct{ assignment, expected string }{
		{`{"login":true,"quota":true}`, "REACHABLE"},
		{`{"login":true,"quota":false}`, "UNREACHABLE"},
	} {
		var result Reachability
		if err := json.Unmarshal(cliJSON(t, "explore", "--file", file, "--assignment", scenario.assignment), &result); err != nil {
			t.Fatal(err)
		}
		if result.Verdict != scenario.expected || result.Scope == "" || len(result.Constraints) != 1 {
			t.Fatalf("missing bounded proof: %+v", result)
		}
	}
	var unknown Reachability
	if err := json.Unmarshal(cliJSON(t, "explore", "--file", file, "--max-states", "1"), &unknown); err != nil {
		t.Fatal(err)
	}
	if unknown.Verdict != "UNKNOWN" || !unknown.Truncated {
		t.Fatalf("truncation was mistaken for proof: %+v", unknown)
	}
}

func TestAnalysisSixViewsKeepCanonicalIdentity(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	seen := 0
	for _, view := range []string{"overview", "capabilities", "boundaries", "flows", "verification", "impact"} {
		var result struct {
			Nodes    []Node `json:"nodes"`
			Coverage struct {
				Obligations struct {
					Total int `json:"total"`
				} `json:"obligations"`
			} `json:"coverage"`
		}
		if err := json.Unmarshal(cliJSON(t, "view", "--db", d.file, "--graph", graph.ID, "--view", view), &result); err != nil {
			t.Fatal(err)
		}
		for _, node := range result.Nodes {
			if node.ID == "enrollment.submit" {
				seen++
				if node.Name != "提交报名" || node.Kind != "capability" {
					t.Fatalf("view %s replaced canonical identity: %+v", view, node)
				}
			}
		}
		if result.Coverage.Obligations.Total != len(modelObligations(graph)) {
			t.Fatalf("view %s shrank the full denominator", view)
		}
	}
	if seen < 3 {
		t.Fatal("the capability cannot be followed across related views")
	}
}

func TestAnalysisPlanningAndFilteringDoNotShrinkObligations(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	if _, err := d.plan(graph.ID, 1); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Coverage struct {
			Obligations struct{ Total, Passed int } `json:"obligations"`
		} `json:"coverage"`
		Nodes []struct{ ID, Status string } `json:"nodes"`
	}
	if err := json.Unmarshal(cliJSON(t, "view", "--db", d.file, "--graph", graph.ID, "--view", "verification", "--root", "enrollment.submit", "--limit", "1"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Coverage.Obligations.Total != len(modelObligations(graph)) || result.Coverage.Obligations.Passed != 0 {
		t.Fatalf("partial planning changed the proof denominator: %+v", result.Coverage)
	}
	for _, node := range result.Nodes {
		if node.Status == "PASS" || node.Status == "FAIL" {
			t.Fatalf("unexecuted model was colored as a business verdict: %+v", node)
		}
	}
}

func TestAnalysisImpactUsesBusinessDependencies(t *testing.T) {
	graph := referenceModel(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "reporting", Kind: "capability", Name: "Reference unrelated capability"})
	graph.Edges = append(graph.Edges, Edge{ID: "contains-reporting", From: "enrollment", To: "reporting", Kind: "contains"})
	d := modelTestDB(t, graph)
	var result struct {
		Affected []string `json:"affected"`
		Tests    []string `json:"tests"`
	}
	if err := json.Unmarshal(cliJSON(t, "diff", "--db", d.file, "--graph", graph.ID, "--changed", "login"), &result); err != nil {
		t.Fatal(err)
	}
	joined := "|" + strings.Join(result.Affected, "|") + "|"
	for _, id := range []string{"R1", "enroll", "enrollment.submit"} {
		if !strings.Contains(joined, "|"+id+"|") {
			t.Fatalf("missing affected %s: %+v", id, result)
		}
	}
	if strings.Contains(joined, "|reporting|") {
		t.Fatal("unrelated capability was reached by mixing containment into execution dependencies")
	}
	if len(result.Tests) == 0 {
		t.Fatal("affected tests were omitted")
	}
}

func TestAnalysisFlowExplorationIsBoundedAndModelOnly(t *testing.T) {
	graph := referenceModel(t)
	graph.Edges = append(graph.Edges, Edge{ID: "retry-ready", From: "ready", To: "ready", Kind: "transitions_to"})
	graph.Transitions = append(graph.Transitions, TransitionContract{Edge: "retry-ready", Action: "enroll", Rule: "R1", Preconditions: ConditionAssignment{"login": true}, Outcome: branchRetry})
	body, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Truncated bool       `json:"truncated"`
		Scope     string     `json:"scope"`
		Paths     [][]string `json:"paths"`
		Merges    []struct {
			Merged []string `json:"merged"`
		} `json:"merges"`
	}
	if err := json.Unmarshal(cliJSON(t, "explore", "--file", writeBusinessModel(t, string(body)), "--flow", "flow.enrollment", "--max-steps", "2"), &result); err != nil {
		t.Fatal(err)
	}
	// W10 contract: a cycle back to an equivalent state closes through a recorded merge, not the budget.
	if result.Truncated || result.Scope == "" || len(result.Merges) != 1 || strings.Join(result.Merges[0].Merged, ",") != "submit-flow,retry-ready" {
		t.Fatalf("retry cycle was not closed by an equivalence merge: %+v", result)
	}
	for _, path := range append(result.Paths, result.Merges[0].Merged) {
		for _, id := range path {
			if id != "submit-flow" && id != "retry-ready" {
				t.Fatalf("non-transition relation was used as an execution path: %s", id)
			}
		}
	}
}

func TestAnalysisScenarioApplicabilityHasExplicitSource(t *testing.T) {
	body := strings.TrimSuffix(businessModelExample, "}") + `,"applicability":[{"capability":"enrollment.submit","kind":"CONCURRENCY","applicable":false,"reason":"This acceptance fixture is single-threaded; not a real business concurrency proof","source":"document","source_ref":"reference-fixture"}]}`
	if _, err := loadGraph(writeBusinessModel(t, body)); err != nil {
		t.Fatalf("explicit sourced applicability is unavailable: %v", err)
	}
	body = strings.Replace(body, `"applicable":false`, `"applicable":null`, 1)
	if _, err := loadGraph(writeBusinessModel(t, body)); err == nil {
		t.Fatal("unknown applicability was turned into an exclusion")
	}
}

func TestAnalysisBoundaryRootKeepsOwnedSubjects(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	projection, err := d.viewModel(ViewQuery{Graph: graph.ID, View: viewBoundaries, Root: "course.boundary", Limit: 128})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, node := range projection.Nodes {
		seen[node.ID] = true
	}
	for _, id := range []string{"course.boundary", "enrollment.submit", "enrollment.records"} {
		if !seen[id] {
			t.Fatalf("boundary root lost owned business subject %s: %+v", id, projection.Nodes)
		}
	}
}

func TestAnalysisFlowChangeReachesCapabilityAndTests(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	impact, err := d.diffModel(graph.ID, "", []string{"flow.enrollment"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, id := range impact.Affected {
		seen[id] = true
	}
	if !seen["enrollment.submit"] || len(impact.Tests) == 0 {
		t.Fatalf("flow change lost its capability or impacted tests: %+v", impact)
	}
}

func TestAnalysisRoleFilterKeepsServedDomainCapabilities(t *testing.T) {
	graph := referenceModel(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "SYSTEM", Kind: "role", Name: "系统"})
	graph.Rules[0].Actor = "SYSTEM"
	d := modelTestDB(t, graph)
	projection, err := d.viewModel(ViewQuery{Graph: graph.ID, View: viewCapabilities, Role: "STUDENT", Limit: 128})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, node := range projection.Nodes {
		if node.ID == "enrollment.submit" {
			found = true
		}
	}
	if !found {
		t.Fatalf("role served-domain descendants were lost: %+v", projection.Nodes)
	}
}

func TestAnalysisFlowRootsKeepUntestedBehaviors(t *testing.T) {
	graph := referenceModel(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "check.eligibility", Kind: "action", Name: "校验报名资格"})
	graph.Edges = append(graph.Edges, Edge{ID: "implements-eligibility", From: "enrollment.submit", To: "check.eligibility", Kind: "implements"})
	d := modelTestDB(t, graph)
	for _, root := range []string{"enrollment", "enrollment.submit"} {
		projection, err := d.viewModel(ViewQuery{Graph: graph.ID, View: viewFlows, Root: root, Limit: 128})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, node := range projection.Nodes {
			if node.ID == "check.eligibility" {
				found = true
			}
		}
		if !found {
			t.Fatalf("root %s hid an untested business behavior: %+v", root, projection.Nodes)
		}
	}
}
