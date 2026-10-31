package main

import "testing"

func TestHandoffTargetRejectsInventedSkill(t *testing.T) {
	for _, name := range []string{"arc-explore", "arc-plan", "arc-code", "arc-verify", "arc-audit", "arc-vet"} {
		if e := validateHandoffTarget(name); e != nil {
			t.Fatal(e)
		}
	}
	if e := validateHandoffTarget("arc-approve"); e == nil {
		t.Fatal("invented skill accepted")
	}
}
func TestHandoffModelPreconditionsFollowObligation(t *testing.T) {
	observation := StepRAW{Preconditions: map[string]bool{"login": true, "course_open": true, "is_admin": false}}
	model := runContract{
		Task: taskSpec{Kind: "MODEL", Expected: "pending"},
		Obligation: &ModelObligation{Definition: TestDefinition{
			Kind: testPositive, Preconditions: map[string]bool{"login": true, "course_open": true}, Expected: "pending",
		}},
	}
	if !handoffConditionsMatch(model, observation) || handoffNeedsRejectionEvidence(model) {
		t.Fatal("a MODEL success obligation must hand off when its declared preconditions held")
	}
	denied := model
	denied.Obligation = &ModelObligation{Definition: TestDefinition{
		Kind: testNegative, Preconditions: map[string]bool{"login": true, "is_admin": false}, Expected: "UNCHANGED", Denied: true,
	}}
	if !handoffNeedsRejectionEvidence(denied) {
		t.Fatal("a MODEL rejection obligation still needs independent readback")
	}
	legacy := runContract{Task: taskSpec{Kind: "POSITIVE"}, Rule: Rule{Conditions: []string{"login"}}}
	if !handoffConditionsMatch(legacy, StepRAW{Preconditions: map[string]bool{"login": true}}) {
		t.Fatal("legacy POSITIVE handoff must stay unchanged")
	}
}

func TestHandoffFirstFailureIsImmutable(t *testing.T) {
	steps := []StepTrace{{Index: "1", Verdict: "PASS"}, {Index: "2", Verdict: "FAIL", Expected: "APPROVED", Observed: "PENDING"}, {Index: "3", Verdict: "PASS"}}
	divergence, ok := firstDivergence(steps)
	if !ok || divergence.Index != "2" {
		t.Fatal("first divergence must be earliest failed step")
	}
}
func TestHandoffTruncatesContext(t *testing.T) {
	rows := make([][]string, 17)
	limited, wasTruncated := takeRows(rows, 8)
	if !wasTruncated || len(limited) != 8 {
		t.Fatal("unbounded handoff context")
	}
}
