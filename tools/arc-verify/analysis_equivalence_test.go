package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// draftFlowGraph builds an exploration-only graph; transitions are {edge, from, to, "cond=1,cond=0"}.
func draftFlowGraph(states, conditions []string, transitions [][4]string) Graph {
	g := Graph{ID: "draft", Flows: []BehaviorFlow{{ID: "flow", Capability: "cap", Initial: states[0]}}}
	g.Nodes = append(g.Nodes, Node{ID: "cap", Kind: "capability", Name: "cap"})
	for _, state := range states {
		g.Nodes = append(g.Nodes, Node{ID: state, Kind: "state", Name: state})
	}
	for _, condition := range conditions {
		g.Nodes = append(g.Nodes, Node{ID: condition, Kind: "condition", Name: condition})
	}
	for _, item := range transitions {
		preconditions := ConditionAssignment{}
		for _, literal := range strings.Split(item[3], ",") {
			if literal != "" {
				parts := strings.Split(literal, "=")
				preconditions[parts[0]] = parts[1] == "1"
			}
		}
		action := "do-" + item[0]
		g.Nodes = append(g.Nodes, Node{ID: action, Kind: "action", Name: action})
		g.Edges = append(g.Edges, Edge{ID: "impl-" + action, From: "cap", To: action, Kind: "implements"}, Edge{ID: item[0], From: item[1], To: item[2], Kind: "transitions_to"})
		g.Transitions = append(g.Transitions, TransitionContract{Edge: item[0], Action: action, Rule: "rule-" + item[0], Preconditions: preconditions, Outcome: branchSuccess})
	}
	return g
}

// supportCycleDraft follows IM-mongo support_service_cycle.go (pending/queued/assigned/resolved and
// RatingEligible needing a responsible agent); the edges are a tool fixture, not an IM-mongo contract.
func supportCycleDraft(withHistory bool) Graph {
	g := draftFlowGraph([]string{"pending", "queued", "assigned", "resolved", "rated"}, []string{"agent_available"}, [][4]string{
		{"enqueue", "pending", "queued", "agent_available=0"},
		{"dispatch", "pending", "assigned", "agent_available=1"},
		{"claim", "queued", "assigned", "agent_available=1"},
		{"agent-end", "assigned", "resolved", ""},
		{"user-end-pending", "pending", "resolved", ""},
		{"user-end-queued", "queued", "resolved", ""},
		{"rate", "resolved", "rated", ""},
		{"reopen", "resolved", "pending", ""},
	})
	if withHistory {
		g.History = []HistoryAttribute{{ID: "responsible_agent", Flow: "flow", SetBy: []string{"dispatch", "claim"}, ClearedBy: []string{"reopen"}, Enables: []string{"rate"}, Source: "document", SourceRef: "fixture"}}
	}
	return g
}

func TestExplorationMergesEquivalentStates(t *testing.T) {
	result, err := exploreFlow(supportCycleDraft(true), "flow", ExplorationBudget{MaxStates: 500, MaxSteps: 12, MaxPaths: 256, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if result.Truncated || len(result.Merges) == 0 {
		t.Fatalf("cycles must close through equivalence merges, not the budget: %+v", result)
	}
	for _, merge := range result.Merges {
		if merge.Key == "" || len(merge.Merged) == 0 || len(merge.Basis) == 0 {
			t.Fatalf("merge record cannot be traced back: %+v", merge)
		}
	}
	rated := false
	for _, path := range append(append([][]string{}, result.Paths...), mergedPaths(result)...) {
		responsible := false
		for _, edge := range path {
			switch edge {
			case "dispatch", "claim":
				responsible = true
			case "reopen":
				responsible = false
			case "rate":
				if !responsible {
					t.Fatalf("rate explored without a responsible agent: %v", path)
				}
				rated = true
			}
		}
	}
	if !rated {
		t.Fatalf("history-distinct resolved state was merged away; rating is unreachable: %+v", result.Paths)
	}
	for _, merge := range result.Merges {
		responsible := false
		for _, edge := range merge.Merged {
			responsible = edge == "dispatch" || edge == "claim" || responsible && edge != "reopen"
		}
		if want := fmt.Sprintf("h:responsible_agent=%t", responsible); !strings.HasSuffix(merge.Key, want) {
			t.Fatalf("merge key disagrees with the history replayed from its path (%s): %+v", want, merge)
		}
	}
}

func mergedPaths(result FlowExploration) [][]string {
	paths := [][]string{}
	for _, merge := range result.Merges {
		paths = append(paths, merge.Merged)
	}
	return paths
}

func TestExplorationKeepsHistoryDistinctWorlds(t *testing.T) {
	result, err := exploreFlow(supportCycleDraft(true), "flow", ExplorationBudget{MaxStates: 500, MaxSteps: 12, MaxPaths: 256, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	worlds := map[string]bool{}
	for _, path := range append(append([][]string{}, result.Paths...), mergedPaths(result)...) {
		responsible, sawResolved := false, false
		for _, edge := range path {
			switch edge {
			case "dispatch", "claim":
				responsible = true
			case "reopen":
				responsible = false
			case "agent-end", "user-end-pending", "user-end-queued":
				sawResolved = true
				worlds[fmt.Sprintf("responsible=%t", responsible)] = true
			}
		}
		_ = sawResolved
	}
	if !worlds["responsible=true"] || !worlds["responsible=false"] {
		t.Fatalf("declared history must keep the rate-eligible and user-ended resolved worlds apart: %v", worlds)
	}
	for _, merge := range result.Merges {
		if strings.Contains(merge.Key, "state=resolved|h:responsible_agent=true") && strings.Contains(strings.Join(merge.Merged, ","), "user-end") {
			t.Fatalf("a user-ended path was merged into the rate-eligible resolved world: %+v", merge)
		}
	}
}

func TestExplorationWithoutHistoryMergesByStateOnly(t *testing.T) {
	result, err := exploreFlow(supportCycleDraft(false), "flow", ExplorationBudget{MaxStates: 500, MaxSteps: 12, MaxPaths: 256, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if result.Truncated {
		t.Fatalf("state-only equivalence must still terminate: %+v", result)
	}
	for _, merge := range result.Merges {
		if strings.Contains(merge.Key, "h:") {
			t.Fatalf("undeclared history entered the key: %+v", merge)
		}
	}
}

func TestExplorationBudgetStillTruncates(t *testing.T) {
	result, err := exploreFlow(supportCycleDraft(true), "flow", ExplorationBudget{MaxStates: 3, MaxSteps: 12, MaxPaths: 256, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated || result.Reason == "" {
		t.Fatalf("a budget smaller than the distinct states must truncate: %+v", result)
	}
}

func academicVariantGraph() Graph {
	return draftFlowGraph([]string{"pending", "enrolled", "attended", "seated", "cancelled"}, []string{"approved", "activity_active", "user_enabled", "quota_ok", "marketing_opt_in"}, [][4]string{
		{"enroll", "pending", "enrolled", "user_enabled=1,activity_active=1"},
		{"attend", "enrolled", "attended", "approved=1,activity_active=1,user_enabled=1"},
		{"confirm-seat", "enrolled", "seated", "approved=1,quota_ok=1"},
		{"user-cancel", "enrolled", "cancelled", "activity_active=1,user_enabled=1"},
	})
}

func TestEquivalenceKeySeparatesBehaviorRelevantConditions(t *testing.T) {
	g := academicVariantGraph()
	base := map[string]bool{"approved": true, "activity_active": true, "user_enabled": true, "quota_ok": true, "marketing_opt_in": false}
	variant := func(change map[string]bool, drop string) EquivalenceInput {
		conditions := map[string]bool{}
		for id, value := range base {
			conditions[id] = value
		}
		for id, value := range change {
			conditions[id] = value
		}
		delete(conditions, drop)
		return EquivalenceInput{Flow: "flow", State: "enrolled", Role: "STUDENT", Principal: "primary", Conditions: conditions}
	}
	inputs := []EquivalenceInput{
		variant(map[string]bool{"approved": false}, ""),
		variant(nil, ""),
		variant(map[string]bool{"activity_active": false}, ""),
		variant(map[string]bool{"user_enabled": false}, ""),
		variant(map[string]bool{"quota_ok": false}, ""),
		variant(map[string]bool{"marketing_opt_in": true}, ""),
	}
	classes := map[string]int{}
	for position, input := range inputs {
		key, basis, undecided := equivalenceKey(g, input)
		if undecided != "" || key == "" || len(basis) == 0 {
			t.Fatalf("variant %d has a complete basis but was undecided: %q %v", position+1, undecided, basis)
		}
		classes[key]++
	}
	if len(classes) != 5 {
		t.Fatalf("expected the five business variants to stay apart and the irrelevant preference to merge: %v", classes)
	}
	first, _, _ := equivalenceKey(g, inputs[1])
	sixth, _, _ := equivalenceKey(g, inputs[5])
	if first != sixth {
		t.Fatal("a condition no reachable transition reads must not split the state")
	}
	constrained := g
	constrained.Constraints = []ConditionClause{{ID: "opt-in-approval", Literals: []ConditionLiteral{{Condition: "approved", Value: false}, {Condition: "marketing_opt_in", Value: true}}, Source: "document", SourceRef: "fixture"}}
	left, _, _ := equivalenceKey(constrained, inputs[1])
	right, _, _ := equivalenceKey(constrained, inputs[5])
	if left == right {
		t.Fatal("a condition tied to a relevant one by a constraint must stay in the key")
	}
	if key, _, undecided := equivalenceKey(g, variant(nil, "quota_ok")); key != "" || !strings.Contains(undecided, "quota_ok") {
		t.Fatalf("a missing relevant observation must not merge: %q %q", key, undecided)
	}
	withHistory := g
	withHistory.History = []HistoryAttribute{{ID: "notified", Flow: "flow", SetBy: []string{"enroll"}, Measurement: "notified", Source: "document", SourceRef: "fixture"}}
	if _, _, undecided := equivalenceKey(withHistory, inputs[1]); !strings.Contains(undecided, "notified") {
		t.Fatalf("a declared history attribute without an observation must not merge: %q", undecided)
	}
	observed := inputs[1]
	observed.History = map[string]bool{"notified": true}
	other := inputs[1]
	other.History = map[string]bool{"notified": false}
	left, _, _ = equivalenceKey(withHistory, observed)
	right, _, _ = equivalenceKey(withHistory, other)
	if left == "" || left == right {
		t.Fatal("different declared history must keep states apart")
	}
	measured := inputs[1]
	measured.Measurements = map[string]json.RawMessage{"enrolled": json.RawMessage(`1`)}
	remeasured := inputs[1]
	remeasured.Measurements = map[string]json.RawMessage{"enrolled": json.RawMessage(`2`)}
	a, _, _ := equivalenceKey(g, measured)
	b, _, _ := equivalenceKey(g, remeasured)
	if a == b {
		t.Fatal("runtime observations with different declared measurements must not merge")
	}
}

func TestHistoryDeclarationValidated(t *testing.T) {
	valid := HistoryAttribute{ID: "confirmed-once", Flow: "flow.enrollment", SetBy: []string{"confirm-flow"}, Enables: []string{"confirm-flow"}, Source: "document", SourceRef: "fixture"}
	g := kindsModel(t)
	g.History = []HistoryAttribute{valid}
	if err := validateGraph(g); err != nil {
		t.Fatalf("valid history rejected: %v", err)
	}
	for _, scenario := range []struct {
		name   string
		change func(*HistoryAttribute)
	}{
		{"unknown flow", func(h *HistoryAttribute) { h.Flow = "flow.missing" }},
		{"not a transition", func(h *HistoryAttribute) { h.SetBy = []string{"implements-enroll"} }},
		{"no set_by and no measurement", func(h *HistoryAttribute) { h.SetBy = nil }},
		{"missing source", func(h *HistoryAttribute) { h.SourceRef = "" }},
		{"set and cleared by the same edge", func(h *HistoryAttribute) { h.ClearedBy = []string{"confirm-flow"} }},
		{"duplicated identity", func(h *HistoryAttribute) { h.ID = "R1" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			item := valid
			scenario.change(&item)
			changed := kindsModel(t)
			changed.History = []HistoryAttribute{item}
			if err := validateGraph(changed); err == nil || !strings.Contains(err.Error(), "history") && !strings.Contains(err.Error(), "identity") {
				t.Fatalf("illegal history accepted or misreported: %v", err)
			}
		})
	}
	legacy := `{"id":"legacy-v1","scope":"legacy","nodes":[],"edges":[],"rules":[],"risks":[],"history":[{"id":"h","flow":"f","set_by":["e"],"source":"document","source_ref":"x"}]}`
	if _, err := loadGraph(writeBusinessModel(t, legacy)); err == nil || !strings.Contains(err.Error(), "schema_version 2") {
		t.Fatalf("v1 graphs must not accept history: %v", err)
	}
}

func TestAnalysisOwnershipConflictNeedsSharedDeclaration(t *testing.T) {
	g := referenceModel(t)
	g.Nodes = append(g.Nodes, Node{ID: "audit.boundary", Kind: "boundary", Name: "审计边界"})
	g.Edges = append(g.Edges, Edge{ID: "owns-submit-audit", From: "audit.boundary", To: "enrollment.submit", Kind: "owns"})
	d := modelTestDB(t, g)
	analysis, err := d.analyzeModel(g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(analysis, "OWNERSHIP_CONFLICT", "error") || analysis.Verdict != "BLOCKED" {
		t.Fatalf("undeclared shared ownership must block: %+v", analysis.Diagnostics)
	}
	shared := g
	shared.ID = "course-v2-shared"
	shared.SharedOwners = []SharedOwnership{{Subject: "enrollment.submit", Owners: []string{"course.boundary", "audit.boundary"}, Reason: "audit reads every submission", SourceRef: "fixture"}}
	d = modelTestDB(t, shared)
	analysis, err = d.analyzeModel(shared, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if hasDiagnostic(analysis, "OWNERSHIP_CONFLICT", "error") {
		t.Fatalf("sourced shared ownership is legal: %+v", analysis.Diagnostics)
	}
}

func TestAnalysisDependencyCycleIsDiagnosed(t *testing.T) {
	g := referenceModel(t)
	g.Nodes = append(g.Nodes, Node{ID: "enrollment.review", Kind: "capability", Name: "审核报名"}, Node{ID: "review", Kind: "action", Name: "审核"})
	g.Edges = append(g.Edges,
		Edge{ID: "contains-review", From: "enrollment", To: "enrollment.review", Kind: "contains"},
		Edge{ID: "implements-review", From: "enrollment.review", To: "review", Kind: "implements"},
		Edge{ID: "dep-a", From: "enrollment.submit", To: "enrollment.review", Kind: "depends_on"},
		Edge{ID: "dep-b", From: "enrollment.review", To: "enrollment.submit", Kind: "depends_on"})
	analysis := ModelAnalysis{Diagnostics: modelDiagnostics(g)}
	if !hasDiagnostic(analysis, "DEPENDENCY_CYCLE", "warning") {
		t.Fatalf("business dependency cycle not diagnosed: %+v", analysis.Diagnostics)
	}
}

func hasDiagnostic(analysis ModelAnalysis, code, severity string) bool {
	for _, diagnostic := range analysis.Diagnostics {
		if diagnostic.Code == code && diagnostic.Severity == severity {
			return true
		}
	}
	return false
}

func passingCourseFixture(t *testing.T) (executionFixture, func()) {
	t.Helper()
	enrolled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/enroll":
			enrolled = 1
			fmt.Fprint(w, `{}`)
		case "/reset":
			enrolled = 0
			fmt.Fprint(w, `{}`)
		default:
			state := "pending"
			if enrolled > 0 {
				state = "ready"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":1}`, state, enrolled)
		}
	}))
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "course-runner", "R1/POSITIVE"))
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "course-runner", "R1/POSITIVE"), &result); err != nil || result.Verdict != "PASS" {
		t.Fatalf("baseline run did not pass: %+v %v", result, err)
	}
	return fixture, server.Close
}

func positiveStatus(t *testing.T, analysis ModelAnalysis) string {
	t.Helper()
	for _, obligation := range analysis.Obligations {
		if obligation.Definition.ID == "R1/POSITIVE" {
			return obligation.Status
		}
	}
	t.Fatal("R1/POSITIVE missing")
	return ""
}

func TestAnalysisStalePassIsNotGreen(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	analysis, err := fixture.d.analyzeModel(referenceModel(t), "independent", fixture.round)
	if err != nil || positiveStatus(t, analysis) != "PASS" {
		t.Fatalf("current evidence must pass first: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "changed.txt"), []byte("candidate changed after the run"), 0600); err != nil {
		t.Fatal(err)
	}
	analysis, err = fixture.d.analyzeModel(referenceModel(t), "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	if status := positiveStatus(t, analysis); status != "STALE" || analysis.Nodes["R1"].Status == "PASS" || analysis.Verdict == "PASS" {
		t.Fatalf("a changed candidate must not stay green: %s %s %s", status, analysis.Nodes["R1"].Status, analysis.Verdict)
	}
}

func TestAnalysisSeparateRoundsAreNotCombined(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	first := fixture.round
	fixture.openRound(t)
	analysis, err := fixture.d.analyzeModel(referenceModel(t), "independent", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(analysis, "ROUND_REQUIRED", "warning") || positiveStatus(t, analysis) == "PASS" {
		t.Fatalf("passes from separate rounds were combined: %+v", analysis.Diagnostics)
	}
	if _, err := fixture.d.analyzeModel(referenceModel(t), "another-environment", first); err == nil || !strings.Contains(err.Error(), "another graph or environment") {
		t.Fatalf("a round of another environment must be rejected: %v", err)
	}
	keys := []string{}
	for _, diagnostic := range analysis.Diagnostics {
		keys = append(keys, diagnostic.Code)
	}
	sort.Strings(keys)
	t.Logf("diagnostics: %v", keys)
}

func TestAnalysisChangedEvidenceIsNotGreen(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	analysis, err := fixture.d.analyzeModel(referenceModel(t), "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	var run string
	for _, obligation := range analysis.Obligations {
		if obligation.Definition.ID == "R1/POSITIVE" {
			run = obligation.Run
		}
	}
	trace, err := fixture.d.trace(run)
	if err != nil || len(trace.Steps) == 0 {
		t.Fatalf("passing run has no recorded step: %v", err)
	}
	if err := os.WriteFile(trace.Steps[0].Evidence, []byte(`{"tampered":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	analysis, err = fixture.d.analyzeModel(referenceModel(t), "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	if status := positiveStatus(t, analysis); status != "STALE" || analysis.Nodes["R1"].Status == "PASS" {
		t.Fatalf("changed RAW evidence must not stay green: %s", status)
	}
}
