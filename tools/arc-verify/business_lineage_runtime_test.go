package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBusinessLineageSnapshotScopesBeforeObservations(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", nil)
	executed := executeRuntimeBusiness(t, fixture)
	if executed.Verdict != "PASS" {
		t.Fatal(executed)
	}
	snapshot, err := fixture.d.buildSnapshot(SnapshotRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round, Config: fixture.config})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Runs) != 1 || len(snapshot.Runs[0].BusinessLineage) != 1 {
		t.Fatal("snapshot has no scoped dependency reading for the actual branch occurrence")
	}
	overlay := snapshot.Runs[0].BusinessLineage[0]
	if overlay.Run != executed.Run || overlay.Step != 1 || overlay.Actor != "primary" || overlay.ActionIndex != 1 || overlay.Branch != "accept" || overlay.Phase != "before" || overlay.EvidenceStatus != "PASS" {
		t.Fatalf("dependency reading lost run/step/actor/action/branch identity: %+v", overlay)
	}
	found := false
	for _, node := range overlay.Nodes {
		if node.Node == "branch:accept:state" {
			found = node.ObservedState == "pending" && node.Status == "SATISFIED"
		}
	}
	if !found {
		t.Fatal("dependency reading used the actual ready poststate instead of the independently observed pending prestate")
	}
	if err := fixture.d.tx([]string{"UPDATE step_assertions SET actual_json='corrupt-lineage-evidence' WHERE run=" + quote(executed.Run) + " AND seq=1"}); err != nil {
		t.Fatal(err)
	}
	damaged, err := fixture.d.buildSnapshot(SnapshotRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round, Config: fixture.config})
	if err != nil {
		t.Fatal(err)
	}
	if len(damaged.Runs) != 1 || damaged.Runs[0].EvidenceStatus != "BLOCKED" || len(damaged.Runs[0].BusinessLineage) != 0 {
		t.Fatal("damaged assertion evidence left satisfied lineage readings in the exported run")
	}
}

func lineageTestOccurrence(t *testing.T, g Graph, actor, binding string, index int, active bool) (BusinessExpectationObservation, OracleObservation) {
	t.Helper()
	before := contractObservation(active)
	before.Bindings["primary"] = binding
	for key := range before.Values {
		before.Sources[key] = []string{"shared-db"}
	}
	decision := FreezeBusinessExpectation(g, "enroll", before)
	if decision.Frozen == nil {
		t.Fatal(decision)
	}
	after := contractObservation(!active)
	after.State = "ready"
	return BusinessExpectationObservation{Actor: actor, ActionIndex: index, Decision: decision, After: &after}, OracleObservation{Actor: actor, ActionIndex: index, ID: "shared-db", Boundary: "persistence", Before: IndependentFacts{State: before.State, Principal: binding, Values: before.Values}, BeforeSHA256: digestBytes([]byte(actor + binding))}
}

func lineageTestRun(g Graph, entries []BusinessExpectationObservation, oracles []OracleObservation) SnapshotRun {
	return SnapshotRun{Trace: TraceReport{Run: "R-lineage"}, EvidenceStatus: "PASS", Facts: []UnifiedStepFact{{Run: "R-lineage", Seq: 1, Graph: g.ID, SourceKind: "execute", EvidenceVersion: runtimeEvidenceVersion, EvidenceStatus: "PASS", BusinessExpectations: entries, Oracles: oracles}}}
}

func TestBusinessLineageSameNamedOraclesKeepActionActorAndRunScope(t *testing.T) {
	g := contractModel(t)
	lineage, err := buildBusinessLineage(g)
	if err != nil {
		t.Fatal(err)
	}
	first, oracleA := lineageTestOccurrence(t, g, "buyer-A", "account-A", 1, true)
	second, oracleB := lineageTestOccurrence(t, g, "buyer-B", "account-B", 2, false)
	third, oracleC := lineageTestOccurrence(t, g, "buyer-A", "account-A", 3, false)
	run := lineageTestRun(g, []BusinessExpectationObservation{first, second, third}, []OracleObservation{oracleA, oracleB, oracleC})
	beforeHash, err := businessJSONHash(run)
	if err != nil {
		t.Fatal(err)
	}
	overlays, err := buildBusinessLineageOverlays(g, lineage, run)
	if err != nil || len(overlays) != 3 {
		t.Fatalf("same-named Oracle occurrences collapsed: %+v %v", overlays, err)
	}
	for index, overlay := range overlays {
		want := []struct{ actor, binding, active string }{{"buyer-A", "account-A", "true"}, {"buyer-B", "account-B", "false"}, {"buyer-A", "account-A", "false"}}[index]
		if overlay.Run != "R-lineage" || overlay.Step != 1 || overlay.Actor != want.actor || overlay.ActionIndex != index+1 || overlay.EvidenceStatus != "PASS" {
			t.Fatalf("occurrence identity changed: %+v", overlay)
		}
		values := 0
		for _, node := range overlay.Nodes {
			for _, value := range node.Values {
				if value.Subject == "primary" && value.Attribute == "active" {
					values++
					if value.Binding != want.binding || string(value.Actual) != want.active || len(value.Sources) != 1 || value.Sources[0].Oracle != "shared-db" || value.Sources[0].SHA256 != run.Facts[0].Oracles[index].BeforeSHA256 {
						t.Fatalf("another actor/action Oracle overwrote a condition: %+v", value)
					}
				}
			}
		}
		if values == 0 {
			t.Fatal("attribute-level observations disappeared")
		}
	}
	afterHash, err := businessJSONHash(run)
	if err != nil || beforeHash != afterHash {
		t.Fatal("lineage projection rewrote frozen facts or actual results")
	}
	run.Trace.Run, run.Facts[0].Run = "R-other", "R-other"
	other, err := buildBusinessLineageOverlays(g, lineage, run)
	if err != nil || other[0].Run != "R-other" || overlays[0].Run != "R-lineage" {
		t.Fatal("run identity leaked between read-only projections")
	}
}

func TestBusinessLineageDamagedOrIncompleteOccurrenceCannotBecomeSatisfied(t *testing.T) {
	g := contractModel(t)
	lineage, err := buildBusinessLineage(g)
	if err != nil {
		t.Fatal(err)
	}
	entry, oracle := lineageTestOccurrence(t, g, "primary", "account-A", 1, true)
	original := lineageTestRun(g, []BusinessExpectationObservation{entry}, []OracleObservation{oracle})
	for name, change := range map[string]func(*SnapshotRun){
		"damaged-run":           func(run *SnapshotRun) { run.EvidenceStatus = "BLOCKED" },
		"missing-step-evidence": func(run *SnapshotRun) { run.Facts[0].EvidenceStatus = "MISSING" },
		"cross-run-facts":       func(run *SnapshotRun) { run.Facts[0].Run = "R-other" },
		"cross-actor-Oracle":    func(run *SnapshotRun) { run.Facts[0].Oracles[0].Actor = "other" },
		"cross-action-Oracle":   func(run *SnapshotRun) { run.Facts[0].Oracles[0].ActionIndex = 2 },
		"duplicate-Oracle":      func(run *SnapshotRun) { run.Facts[0].Oracles = append(run.Facts[0].Oracles, run.Facts[0].Oracles[0]) },
		"duplicate-occurrence": func(run *SnapshotRun) {
			run.Facts[0].BusinessExpectations = append(run.Facts[0].BusinessExpectations, run.Facts[0].BusinessExpectations[0])
		},
		"changed-frozen-value": func(run *SnapshotRun) {
			run.Facts[0].BusinessExpectations[0].Decision.Frozen.Before.Values["primary.active"] = json.RawMessage("false")
		},
		"source-disagrees": func(run *SnapshotRun) {
			run.Facts[0].Oracles[0].Before.Values["primary.active"] = json.RawMessage("false")
		},
		"missing-source": func(run *SnapshotRun) {
			before := run.Facts[0].BusinessExpectations[0].Decision.Frozen.Before
			delete(before.Sources, "primary.active")
			run.Facts[0].BusinessExpectations[0].Decision = FreezeBusinessExpectation(g, "enroll", before)
		},
	} {
		t.Run(name, func(t *testing.T) {
			run, err := cloneBusinessJSON(original)
			if err != nil {
				t.Fatal(err)
			}
			change(&run)
			overlays, err := buildBusinessLineageOverlays(g, lineage, run)
			if err != nil || len(overlays) == 0 {
				t.Fatalf("damaged occurrence was hidden instead of explained: %+v %v", overlays, err)
			}
			for _, overlay := range overlays {
				if overlay.EvidenceStatus != "BLOCKED" {
					t.Fatal("damaged occurrence remained a verified source")
				}
				for _, node := range overlay.Nodes {
					if node.Status != "BLOCKED" || node.Reason == "" {
						t.Fatalf("damaged occurrence produced an unexplained or satisfied condition: %+v", node)
					}
				}
			}
		})
	}
}

func TestBusinessLineagePreparationRequiresAllObservedEffects(t *testing.T) {
	g := preparationModel(t)
	g.BusinessContract.Preparation[0].Ensures = BusinessPredicate{Op: "true"}
	lineage, err := buildBusinessLineage(g)
	if err != nil {
		t.Fatal(err)
	}
	entry, oracle := lineageTestOccurrence(t, g, "primary", "account-A", 1, true)
	run := lineageTestRun(g, []BusinessExpectationObservation{entry}, []OracleObservation{oracle})
	branch := lineage.Branches[0]
	var preparation BusinessLineageNode
	for _, node := range branch.Nodes {
		if node.Kind == "preparation" && node.Preparation == "activate-primary" {
			preparation = node
		}
	}
	if preparation.ID == "" {
		t.Fatal("test lacks the declared activation provider")
	}
	attrs := businessAttributeIndex(g.BusinessContract)
	declarations := map[string]PreparationAction{"activate-primary": g.BusinessContract.Preparation[0]}
	for _, value := range []string{"true", "false", "null", ""} {
		local, err := cloneBusinessJSON(run)
		if err != nil {
			t.Fatal(err)
		}
		before := &local.Facts[0].BusinessExpectations[0].Decision.Frozen.Before
		if value == "" {
			delete(before.Values, "primary.active")
		} else {
			before.Values["primary.active"] = json.RawMessage(value)
			local.Facts[0].Oracles[0].Before.Values["primary.active"] = json.RawMessage(value)
		}
		reading := businessLineageReadNode(preparation, attrs, declarations, local.Facts[0], local.Facts[0].BusinessExpectations[0])
		want := "NOT_OBSERVED"
		if value == "true" {
			want = "SATISFIED"
		} else if value == "false" {
			want = "UNSATISFIED"
		}
		if reading.Status != want {
			t.Fatalf("true ensures hid an absent or contradictory actual effect %q: %+v", value, reading)
		}
	}
	for _, node := range branch.Nodes {
		if strings.Contains(node.ID, "declared-subject:") {
			t.Fatal("static proof symbols leaked into exported business identities")
		}
	}
}
