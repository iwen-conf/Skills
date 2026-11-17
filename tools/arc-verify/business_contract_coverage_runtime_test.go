package main

import "testing"

func contractCoverageFact(t *testing.T, graph Graph, run string, active, successful bool) UnifiedStepFact {
	t.Helper()
	before := contractObservation(active)
	decision := FreezeBusinessExpectation(graph, "enroll", before)
	if decision.Status != "READY" {
		t.Fatal(decision)
	}
	after := contractObservation(active)
	denied := !active
	if active && successful {
		after.State = "ready"
	}
	comparison := CompareBusinessExpectation(graph, *decision.Frozen, after, denied)
	return UnifiedStepFact{EvidenceVersion: runtimeEvidenceVersion, EvidenceStatus: "PASS", SourceKind: "execute", Run: run, Seq: 1, Graph: graph.ID, Task: graph.ID + "/test.idempotency", Verdict: comparison.Status,
		Actions:              []ActionObservation{{Node: "enroll", Actor: "primary", ActionIndex: 0, Principal: "acct-A", Denied: denied}},
		BusinessExpectations: []BusinessExpectationObservation{{Actor: "primary", ActionIndex: 0, Decision: decision, After: &after, Comparison: &comparison}},
	}
}

func TestBusinessContractExecutionCoverageDoesNotSpreadOneTestPassToOtherBranches(t *testing.T) {
	graph := contractModel(t)
	branches := businessBranchExecutions(graph)
	accept := contractCoverageFact(t, graph, "accept-run", true, true)
	if err := collectBusinessBranchFacts(graph, "accept-run", []UnifiedStepFact{accept}, branches); err != nil {
		t.Fatal(err)
	}
	coverage := summarizeBusinessBranchExecutions("round", branches, nil)
	if coverage.Status != "NOT_RUN" || coverage.Passed != 1 || coverage.NotRun != 1 || branches["reject"].Status != "NOT_RUN" {
		t.Fatalf("same shared test made an unexecuted branch pass: %+v", coverage)
	}
	reject := contractCoverageFact(t, graph, "reject-run", false, true)
	if reject.Task != accept.Task {
		t.Fatal("test does not exercise a shared task identity")
	}
	if err := collectBusinessBranchFacts(graph, "reject-run", []UnifiedStepFact{reject}, branches); err != nil {
		t.Fatal(err)
	}
	coverage = summarizeBusinessBranchExecutions("round", branches, nil)
	if coverage.Status != "PASS" || coverage.Passed != 2 {
		t.Fatal(coverage)
	}
}

func TestBusinessContractExecutionCoveragePreservesVerifiedFailureAcrossLaterPass(t *testing.T) {
	graph := contractModel(t)
	branches := businessBranchExecutions(graph)
	failed := contractCoverageFact(t, graph, "first-failure", true, false)
	passed := contractCoverageFact(t, graph, "later-pass", true, true)
	for _, fact := range []UnifiedStepFact{failed, passed} {
		if err := collectBusinessBranchFacts(graph, fact.Run, []UnifiedStepFact{fact}, branches); err != nil {
			t.Fatal(err)
		}
	}
	coverage := summarizeBusinessBranchExecutions("round", branches, nil)
	if coverage.Status != "FAIL" || branches["accept"].Status != "FAIL" || len(branches["accept"].Runs) != 2 {
		t.Fatalf("a later success erased first branch failure: %+v", coverage)
	}
}

func TestBusinessContractExecutionCoverageRejectsIncompleteForgedOrMisboundProjection(t *testing.T) {
	for _, mode := range []string{"legacy", "wrong-graph", "no-freeze", "tampered-before", "missing-action", "ambiguous-action"} {
		t.Run(mode, func(t *testing.T) {
			graph := contractModel(t)
			fact := contractCoverageFact(t, graph, "run", true, true)
			switch mode {
			case "legacy":
				fact.EvidenceStatus = "LEGACY"
			case "wrong-graph":
				fact.Graph = "another-graph"
			case "no-freeze":
				fact.BusinessExpectations[0].Decision.Frozen = nil
			case "tampered-before":
				fact.BusinessExpectations[0].Decision.Frozen.Before.State = "ready"
			case "missing-action":
				fact.Actions = nil
			case "ambiguous-action":
				fact.Actions = append(fact.Actions, fact.Actions[0])
			}
			if err := collectBusinessBranchFacts(graph, "run", []UnifiedStepFact{fact}, businessBranchExecutions(graph)); err == nil {
				t.Fatalf("%s projection was accepted as coverage", mode)
			}
		})
	}
}

func TestBusinessContractGatePublishesProofAndUnexecutedBranchGap(t *testing.T) {
	graph := contractModel(t)
	d := modelTestDB(t, graph)
	gate, err := d.modelGate(graph)
	if err != nil {
		t.Fatal(err)
	}
	completeness, ok := gate["completeness"].(map[string]any)
	if !ok || completeness["complete_business_e2e"] != false || gate["proof_scope"] != "declared_finite_contract_scope_and_verified_branch_execution" {
		t.Fatalf("finite partition proof was presented as executed E2E: %+v", gate)
	}
	if gate["verdict"] == "PASS" || gate["inputs_unchanged"] != false {
		t.Fatalf("top-level gate passed despite unexecuted typed branches: %+v", gate)
	}
	execution, ok := gate["contract_execution"].(*BusinessExecutionCoverage)
	if !ok || execution.NotRun != 2 || execution.Passed != 0 {
		t.Fatalf("unexecuted branch coverage missing: %+v", gate["contract_execution"])
	}
}

func TestBusinessContractLegacyGateCannotClaimDeterministicCompleteness(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	gate, err := d.modelGate(graph)
	if err != nil {
		t.Fatal(err)
	}
	completeness := gate["completeness"].(map[string]any)
	if completeness["complete_business_e2e"] != false || gate["proof_scope"] != "legacy_business_model_obligations" {
		t.Fatalf("legacy graph gained a new strong coverage claim: %+v", gate)
	}
	proof := gate["contract_proof"].(*BusinessCoverageProof)
	if proof.Status != "LEGACY_UNDECLARED" {
		t.Fatal(proof)
	}
}
