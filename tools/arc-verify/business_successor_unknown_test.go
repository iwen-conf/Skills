package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func uncertainSuccessorGraph(t *testing.T, next BusinessSuccessorPolicy, sourceCheck bool) Graph {
	t.Helper()
	g := successorSequenceGraph(t, BusinessSuccessorPolicy{"confirm"})
	expected := &g.BusinessContract.Branches[0].Expected
	expected.Mode = "allowed_outcomes"
	expected.Reason = "Synthetic sourced outcomes accept the observed state; an independently observed flag can further restrict continuation"
	other := expected.Outcomes[0]
	other.ID, other.NextAllowed = "restricted-if-enabled", next
	if sourceCheck {
		other.Checks = []BusinessExpectedCheck{{ID: "enabled-readback", Kind: "persistence", Oracle: "flag-store", SourceRef: "synthetic-contract:continuation-flag", Predicate: businessPredicate("eq", "fixture", "enabled", "boolean", "true")}}
	} else {
		other.Predicate = businessPredicate("eq", "fixture", "enabled", "boolean", "true")
	}
	expected.Outcomes = append(expected.Outcomes, other)
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestBusinessSuccessorUnknownAlternativeCannotPermitUnprovenContinuation(t *testing.T) {
	for _, sourceCheck := range []bool{false, true} {
		for _, reversed := range []bool{false, true} {
			t.Run(fmt.Sprintf("source_check=%t/reversed=%t", sourceCheck, reversed), func(t *testing.T) {
				g := uncertainSuccessorGraph(t, BusinessSuccessorPolicy{"enroll"}, sourceCheck)
				if reversed {
					outcomes := g.BusinessContract.Branches[0].Expected.Outcomes
					outcomes[0], outcomes[1] = outcomes[1], outcomes[0]
				}
				before := BusinessObservation{Flow: "flow.enrollment", State: "pending", Values: map[string]json.RawMessage{}}
				decision := FreezeBusinessExpectation(g, "enroll", before)
				if decision.Status != "READY" {
					t.Fatal(decision)
				}
				after := BusinessObservation{Flow: "flow.enrollment", State: "ready", Values: map[string]json.RawMessage{}}
				if sourceCheck {
					after.Values["fixture.enabled"] = json.RawMessage("true")
				}
				comparison := CompareBusinessExpectation(g, *decision.Frozen, after, false)
				if comparison.Status == "PASS" && comparison.NextAllowed.Allows("confirm") {
					t.Fatalf("unknown alternative erased a possible continuation restriction: oracleCheck=%t reversed=%t comparison=%+v", sourceCheck, reversed, comparison)
				}
				if comparison.Status != "PASS" || comparison.Outcome != "" || comparison.NextAllowed == nil || len(comparison.NextAllowed) != 0 || len(comparison.Unknown) == 0 {
					t.Fatalf("acceptance, unresolved outcomes and safe continuation were not separated: %+v", comparison)
				}
			})
		}
	}
}

func TestBusinessSuccessorUnknownAlternativeNeverDispatchesForbiddenNextRequest(t *testing.T) {
	target, server := newKindsServer(t)
	g := uncertainSuccessorGraph(t, BusinessSuccessorPolicy{"enroll"}, false)
	f := prepareExecutionFixture(t, g, server.URL)
	f.registerAndRound(t, sequenceManifest(t, "unknown-successor-sequence"))
	result := executeKinds(t, f, "unknown-successor-sequence", "test.sequence.enroll-confirm")
	verified, err := f.d.verifiedRunFacts(result.Run)
	t.Logf("verdict=%s confirms=%d evidence=%s error=%v", result.Verdict, target.confirmCalls, verified.EvidenceStatus, err)
	if target.confirmCalls != 0 || result.Verdict != "BLOCKED" {
		t.Fatalf("incomplete outcome evidence authorized a real successor request: result=%+v confirms=%d", result, target.confirmCalls)
	}
	if err != nil || verified.EvidenceStatus != "PASS" {
		t.Fatalf("the blocked decision must retain independently verifiable evidence: %+v %v", verified, err)
	}
	raw, _, err := f.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	comparison := raw.Model.BusinessExpectations[0].Comparison
	if comparison.Status != "PASS" || comparison.NextAllowed == nil || len(comparison.NextAllowed) != 0 || len(comparison.Unknown) == 0 {
		t.Fatalf("first accepted action lost its safe policy or uncertainty: %+v", comparison)
	}
	comparison.NextAllowed = BusinessSuccessorPolicy{"confirm"}
	comparison.Outcome, comparison.Unknown = "created", nil
	comparison.Reason = "actual result satisfies the frozen expected outcome and every common invariant"
	hash := replaceSuccessorHistoricalCapture(t, f.d, raw)
	contract, err := f.d.runTaskContract(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	verdict, reason, _, err := f.d.evaluateModelObservation(contract, raw, 1, hash)
	if err != nil || verdict != "BLOCKED" || !strings.Contains(reason, "comparison differs") {
		t.Fatalf("self-consistent historical widened policy survived re-evaluation: verdict=%s reason=%s error=%v", verdict, reason, err)
	}
}

func TestBusinessSuccessorObservedFalseAlternativeDoesNotRestrictContinuation(t *testing.T) {
	for _, state := range []string{"flag-false", "state-mismatch", "assertion-false"} {
		t.Run(state, func(t *testing.T) {
			g := uncertainSuccessorGraph(t, BusinessSuccessorPolicy{}, false)
			before := BusinessObservation{Flow: "flow.enrollment", State: "pending", Values: map[string]json.RawMessage{}}
			after := BusinessObservation{Flow: "flow.enrollment", State: "ready", Values: map[string]json.RawMessage{}}
			switch state {
			case "flag-false":
				after.Values["fixture.enabled"] = json.RawMessage("false")
			case "state-mismatch":
				g.BusinessContract.Branches[0].Expected.Outcomes[1].State = "pending"
			case "assertion-false":
				g.BusinessContract.Branches[0].Expected.Outcomes[1].Checks = []BusinessExpectedCheck{{ID: "response-status", Kind: "response", Oracle: "flag-store", SourceRef: "synthetic-contract:observed-false", Predicate: businessPredicate("eq", "fixture", "enabled", "boolean", "false")}}
				after.Values["fixture.enabled"] = json.RawMessage("true")
				after.Sources = map[string][]string{"fixture.enabled": {"flag-store"}}
			}
			decision := FreezeBusinessExpectation(g, "enroll", before)
			if decision.Status != "READY" {
				t.Fatal(decision)
			}
			comparison := CompareBusinessExpectation(g, *decision.Frozen, after, false)
			if comparison.Status != "PASS" || comparison.Outcome != "created" || !comparison.NextAllowed.Allows("confirm") || comparison.NextAllowed.Allows("enroll") || len(comparison.Unknown) != 0 {
				t.Fatalf("independently excluded alternative still restricted the outcome: %+v", comparison)
			}
		})
	}
}

func TestBusinessSuccessorUnknownUnrestrictiveAlternativePreservesProvenPolicy(t *testing.T) {
	for _, policy := range []BusinessSuccessorPolicy{nil, {"confirm"}, {"confirm", "enroll"}} {
		g := uncertainSuccessorGraph(t, policy, false)
		before := BusinessObservation{Flow: "flow.enrollment", State: "pending", Values: map[string]json.RawMessage{}}
		decision := FreezeBusinessExpectation(g, "enroll", before)
		after := BusinessObservation{Flow: "flow.enrollment", State: "ready", Values: map[string]json.RawMessage{}}
		comparison := CompareBusinessExpectation(g, *decision.Frozen, after, false)
		if comparison.Status != "PASS" || !comparison.NextAllowed.Allows("confirm") || comparison.NextAllowed.Allows("enroll") {
			t.Fatalf("irrelevant alternative uncertainty changed the proven continuation: policy=%v comparison=%+v", policy, comparison)
		}
	}
}
