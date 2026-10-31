package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// derivationModel adds a second declared transition leaving "pending" so one
// observed state can derive more than one obligation.
func derivationModel(t *testing.T) Graph {
	t.Helper()
	g := kindsModel(t)
	g.Edges = append(g.Edges, Edge{ID: "retry-submit", From: "pending", To: "pending", Kind: "transitions_to"})
	g.Transitions = append(g.Transitions, TransitionContract{Edge: "retry-submit", Action: "enroll", Rule: "R1", Preconditions: ConditionAssignment{"login": true}, Outcome: branchRetry})
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	return g
}

func positiveManifest(t *testing.T, id string) string {
	return kindsManifest(t, id, "R1/POSITIVE", kindsStep("enroll", "", "/enroll", kindsOracle("fixture-independent-store", "/oracle")))
}

// failedEnrollment runs R1/POSITIVE against a target whose enroll leaves the state pending.
func failedEnrollment(t *testing.T, g Graph) (executionFixture, *kindsServer, ExecuteResult) {
	t.Helper()
	target, server := newKindsServer(t)
	target.enrollBroken = true
	fixture := prepareExecutionFixture(t, g, server.URL)
	cliJSON(t, "runner", "--db", fixture.d.file, "--file", kindsManifest(t, "submit-runner", "transition/submit-flow", kindsStep("enroll", "submit-flow", "/enroll", kindsOracle("fixture-independent-store", "/oracle"))))
	fixture.registerAndRound(t, positiveManifest(t, "positive-runner"))
	result := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	if result.Verdict != "FAIL" {
		t.Fatalf("setup needs an observed failure: %+v", result)
	}
	return fixture, target, result
}

func derivedStatus(t *testing.T, d db, g Graph, round, id string) (ObligationStatus, ModelAnalysis) {
	t.Helper()
	analysis, err := d.analyzeModel(g, "independent", round)
	if err != nil {
		t.Fatal(err)
	}
	for _, obligation := range analysis.Obligations {
		if obligation.Task.ID == id {
			return obligation, analysis
		}
	}
	t.Fatalf("derived obligation %s is not in the complete denominator", id)
	return ObligationStatus{}, analysis
}

func TestDerivedObligationLinksSourceStep(t *testing.T) {
	g := kindsModel(t)
	fixture, _, failed := failedEnrollment(t, g)
	before, err := fixture.d.analyzeModel(g, "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	var report DerivationReport
	if err := json.Unmarshal(cliJSON(t, "derive", "--db", fixture.d.file, "--run", failed.Run, "--budget", "16"), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Derived) != 1 || report.Truncated {
		t.Fatalf("one declared transition leaves the observed state: %+v", report)
	}
	item := report.Derived[0]
	if item.Template != "FROM_OBSERVED_STATE" || item.Origin != "rule" || item.Base != "transition/submit-flow" || item.StartState != "pending" || item.SourceRun != failed.Run || item.SourceStep != 1 {
		t.Fatalf("derived obligation lost its source or base: %+v", item)
	}
	status, after := derivedStatus(t, fixture.d, g, fixture.round, item.ID)
	if after.Coverage.Obligations.Total != before.Coverage.Obligations.Total+1 || status.Status != "NOT_RUN" || status.Definition.Kind != testDerived {
		t.Fatalf("a rule-derived obligation must join the denominator unproven: %+v %d->%d", status, before.Coverage.Obligations.Total, after.Coverage.Obligations.Total)
	}
	for _, target := range []string{"submit-flow", "pending", "R1"} {
		if !strings.Contains(","+strings.Join(status.Targets, ",")+",", ","+target+",") {
			t.Fatalf("derived obligation is not attached to %s: %v", target, status.Targets)
		}
	}
	if !strings.Contains(status.Definition.SourceRef, failed.Run) {
		t.Fatalf("derived definition does not cite its source run: %+v", status.Definition)
	}
}

func TestDerivedObligationDedupByEquivalenceKey(t *testing.T) {
	g := kindsModel(t)
	fixture, _, first := failedEnrollment(t, g)
	if _, err := fixture.d.derive(first.Run, 16); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.d.retry("course-v2/R1/POSITIVE", "fixture independently restored", first.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	second := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	report, err := fixture.d.derive(second.Run, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Derived) != 0 || len(report.Merged) != 1 {
		t.Fatalf("an equivalent observed state must not derive a second obligation: %+v", report)
	}
	rows, err := fixture.d.query("SELECT COUNT(*) FROM derived_obligations WHERE graph='course-v2'", 1)
	if err != nil || rows[0][0] != "1" {
		t.Fatalf("duplicate derived obligations: %v %v", rows, err)
	}
	sources, err := fixture.d.query("SELECT COUNT(*) FROM derivation_sources", 1)
	if err != nil || sources[0][0] != "2" {
		t.Fatalf("both source runs must stay traceable: %v %v", sources, err)
	}
	again, err := fixture.d.derive(second.Run, 16)
	if err != nil || len(again.Derived) != 0 || len(again.Merged) != 1 {
		t.Fatalf("deriving the same run twice must be idempotent: %+v %v", again, err)
	}
}

func TestDerivedObligationBudgetTruncates(t *testing.T) {
	g := derivationModel(t)
	fixture, _, failed := failedEnrollment(t, g)
	report, err := fixture.d.derive(failed.Run, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Derived) != 1 || !report.Truncated || len(report.Skipped) != 1 {
		t.Fatalf("the per-run budget must cap derivation and say so: %+v", report)
	}
	if _, err := fixture.d.derive(failed.Run, 0); err == nil {
		t.Fatal("an unbounded derivation budget was accepted")
	}
}

func TestDerivedObligationSatisfiedByMatchingStart(t *testing.T) {
	g := kindsModel(t)
	fixture, target, failed := failedEnrollment(t, g)
	report, err := fixture.d.derive(failed.Run, 16)
	if err != nil {
		t.Fatal(err)
	}
	target.mu.Lock()
	target.enrollBroken = false
	target.mu.Unlock()
	if result := executeKinds(t, fixture, "submit-runner", "transition/submit-flow"); result.Verdict != "PASS" {
		t.Fatalf("base transition did not pass after the fix: %+v", result)
	}
	if status, _ := derivedStatus(t, fixture.d, g, fixture.round, report.Derived[0].ID); status.Status != "PASS" {
		t.Fatalf("a qualified base run from the equivalent start state must satisfy the derivation: %+v", status)
	}
}

func TestInferredDerivationStaysRisk(t *testing.T) {
	g := kindsModel(t)
	target, server := newKindsServer(t)
	target.unknownState = true
	fixture := prepareExecutionFixture(t, g, server.URL)
	fixture.registerAndRound(t, positiveManifest(t, "positive-runner"))
	failed := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	before, err := fixture.d.analyzeModel(g, "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	report, err := fixture.d.derive(failed.Run, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Derived) != 1 || report.Derived[0].Origin != "inference" || report.Derived[0].Template != "UNDECLARED_STATE" || report.Derived[0].StartState != "mystery" {
		t.Fatalf("an undeclared state must become an inferred exploration item: %+v", report)
	}
	after, err := fixture.d.analyzeModel(g, "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	if after.Coverage.Obligations.Total != before.Coverage.Obligations.Total {
		t.Fatal("an inferred derivation entered the acceptance denominator")
	}
	risks, err := fixture.d.riskReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(risks)
	if !strings.Contains(string(body), report.Derived[0].ID) || !strings.Contains(string(body), "mystery") {
		t.Fatalf("the inferred item is missing from the risk report: %s", body)
	}
}

func TestOriginalFailUnchangedAfterDerivation(t *testing.T) {
	g := kindsModel(t)
	fixture, _, failed := failedEnrollment(t, g)
	traceBefore, err := fixture.d.trace(failed.Run)
	if err != nil {
		t.Fatal(err)
	}
	tasksBefore, err := fixture.d.query("SELECT id,rule,kind,condition,expected,status FROM tasks ORDER BY id", 6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.d.derive(failed.Run, 16); err != nil {
		t.Fatal(err)
	}
	traceAfter, err := fixture.d.trace(failed.Run)
	if err != nil {
		t.Fatal(err)
	}
	tasksAfter, err := fixture.d.query("SELECT id,rule,kind,condition,expected,status FROM tasks ORDER BY id", 6)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(traceBefore)
	b, _ := json.Marshal(traceAfter)
	c, _ := json.Marshal(tasksBefore)
	e, _ := json.Marshal(tasksAfter)
	if string(a) != string(b) || string(c) != string(e) || traceAfter.Status != "FAIL" {
		t.Fatal("derivation changed the original run, its first divergence or the frozen tasks")
	}
	status, _ := derivedStatus(t, fixture.d, g, fixture.round, "course-v2/R1/POSITIVE")
	if status.Status != "FAIL" {
		t.Fatalf("the original obligation must stay FAIL: %+v", status)
	}
}

func TestDerivationNeedsObservedFailure(t *testing.T) {
	g := kindsModel(t)
	_, server := newKindsServer(t)
	fixture := prepareExecutionFixture(t, g, server.URL)
	fixture.registerAndRound(t, positiveManifest(t, "positive-runner"))
	passed := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	if _, err := fixture.d.derive(passed.Run, 16); err == nil || !strings.Contains(err.Error(), "failure") {
		t.Fatalf("a passing run must not derive obligations: %v", err)
	}
}

func TestDerivedObligationStaleWhenBaseEvidenceChanges(t *testing.T) {
	g := kindsModel(t)
	fixture, target, failed := failedEnrollment(t, g)
	report, err := fixture.d.derive(failed.Run, 16)
	if err != nil {
		t.Fatal(err)
	}
	target.mu.Lock()
	target.enrollBroken = false
	target.mu.Unlock()
	passed := executeKinds(t, fixture, "submit-runner", "transition/submit-flow")
	record, err := fixture.d.modelRun(passed.Run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.Recovery.Evidence, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if status, _ := derivedStatus(t, fixture.d, g, fixture.round, report.Derived[0].ID); status.Status != "STALE" {
		t.Fatalf("a base run whose evidence changed must not satisfy the derivation: %+v", status)
	}
}

func TestDerivationSkipsUnapprovedRules(t *testing.T) {
	g := kindsModel(t)
	g.Edges = append(g.Edges, Edge{ID: "confirm-early", From: "pending", To: "pending", Kind: "transitions_to"})
	g.Transitions = append(g.Transitions, TransitionContract{Edge: "confirm-early", Action: "confirm", Rule: "R2", Preconditions: ConditionAssignment{"login": true}, Outcome: branchRetry})
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	target, server := newKindsServer(t)
	target.enrollBroken = true
	fixture := prepareExecutionFixture(t, g, server.URL)
	// Simulates a rule the user has not approved yet (the CLI only approves
	// interactively); it is set before the round, which freezes approvals.
	if err := fixture.d.tx([]string{"UPDATE rules SET approved=0 WHERE graph='course-v2' AND id='R2'"}); err != nil {
		t.Fatal(err)
	}
	fixture.registerAndRound(t, positiveManifest(t, "positive-runner"))
	failed := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	report, err := fixture.d.derive(failed.Run, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Derived) != 1 || report.Derived[0].Base != "transition/submit-flow" {
		t.Fatalf("only approved rules may derive acceptance obligations: %+v", report)
	}
}

func TestDerivedObligationNeedsEquivalentStart(t *testing.T) {
	g := kindsModel(t)
	fixture, target, _ := failedEnrollment(t, g)
	x := indexModel(g)
	other := observedFacts{facts: IndependentFacts{State: "pending", Role: "STUDENT", Conditions: ObservedConditions{"login": true}, Values: map[string]json.RawMessage{"enrolled": json.RawMessage("5")}}, actor: "primary"}
	key := runtimeKey(g, x, "enroll", other, "elsewhere")
	if err := fixture.d.tx([]string{"INSERT INTO derived_obligations(id,graph,template,origin,base,rule,start_state,equivalence_key,score,source_run,source_step,reason) VALUES('course-v2/derived/elsewhere','course-v2','FROM_OBSERVED_STATE','rule','transition/submit-flow','R1','pending'," + quote(key) + ",1,'R-elsewhere',1,'derived from a different start')"}); err != nil {
		t.Fatal(err)
	}
	target.mu.Lock()
	target.enrollBroken = false
	target.mu.Unlock()
	if result := executeKinds(t, fixture, "submit-runner", "transition/submit-flow"); result.Verdict != "PASS" {
		t.Fatalf("base run did not pass: %+v", result)
	}
	if status, _ := derivedStatus(t, fixture.d, g, fixture.round, "course-v2/derived/elsewhere"); status.Status != "NOT_RUN" {
		t.Fatalf("a base run from a non-equivalent start must not satisfy the derivation: %+v", status)
	}
}

func TestEquivalenceIgnoreValidatedAndApplied(t *testing.T) {
	g := kindsModel(t)
	g.Equivalence = &EquivalencePolicy{Ignore: []IgnoredMeasurement{{Measurement: "enrolled", Reason: "counter read by the oracle, not a business distinction", Source: "document", SourceRef: "fixture"}}}
	if err := validateGraph(g); err != nil {
		t.Fatalf("a sourced ignore entry was rejected: %v", err)
	}
	invalid := g
	invalid.Equivalence = &EquivalencePolicy{Ignore: []IgnoredMeasurement{{Measurement: "enrolled", Reason: "no source"}}}
	if err := validateGraph(invalid); err == nil || !strings.Contains(err.Error(), "equivalence.ignore") {
		t.Fatalf("an unsourced ignore entry was accepted: %v", err)
	}
	x := indexModel(g)
	facts := func(enrolled string) observedFacts {
		return observedFacts{facts: IndependentFacts{State: "pending", Role: "STUDENT", Conditions: ObservedConditions{"login": true}, Values: map[string]json.RawMessage{"enrolled": json.RawMessage(enrolled)}}, actor: "primary"}
	}
	if runtimeKey(g, x, "enroll", facts("0"), "a") != runtimeKey(g, x, "enroll", facts("3"), "b") {
		t.Fatal("an ignored measurement still split the state")
	}
	plain := kindsModel(t)
	if runtimeKey(plain, indexModel(plain), "enroll", facts("0"), "a") == runtimeKey(plain, indexModel(plain), "enroll", facts("3"), "b") {
		t.Fatal("without an ignore entry every declared measurement must split the state")
	}
}
