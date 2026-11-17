package main

import (
	"encoding/json"
	"strconv"
	"testing"
)

// Exercise integer boundaries and branch permutations. The seed corpus runs in
// ordinary go test; -fuzz expands it without contacting a target environment.
func FuzzBusinessContractUniqueFreezeDoesNotFollowActual(f *testing.F) {
	for _, values := range [][2]int64{{0, 0}, {-1, 0}, {1, 0}, {9223372036854775807, -1}, {-9223372036854775808, 1}, {100, 101}} {
		f.Add(values[0], values[1])
	}
	f.Fuzz(func(t *testing.T, amount, threshold int64) {
		g := referenceModel(t)
		less := businessPredicate("lt", "request", "amount", "integer", strconv.FormatInt(threshold, 10))
		accepted := BusinessExpectedOutcome{ID: "accepted", State: "ready", Denied: businessBool(false), Predicate: businessPredicate("eq", "result", "stored", "boolean", "true")}
		rejected := BusinessExpectedOutcome{ID: "rejected", State: "pending", Denied: businessBool(true), Predicate: businessPredicate("eq", "result", "stored", "boolean", "false")}
		g.BusinessContract = &BusinessContract{Version: 1,
			Subjects: []BusinessSubject{
				{ID: "request", Kind: "entity", Attributes: []BusinessAttribute{{Name: "amount", Type: "integer"}}},
				{ID: "result", Kind: "entity", Attributes: []BusinessAttribute{{Name: "stored", Type: "boolean"}}},
			},
			Branches: []BusinessBranch{
				{ID: "less", Action: "enroll", Rule: "R1", FromStates: []string{"pending"}, When: less, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{accepted}}, Source: "document", SourceRef: "test:ordered-integers", Confirmed: true},
				{ID: "otherwise", Action: "enroll", Rule: "R1", FromStates: []string{"pending"}, When: businessNot(less), Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{rejected}}, Source: "document", SourceRef: "test:ordered-integers", Confirmed: true},
			},
		}
		before := BusinessObservation{State: "pending", Values: map[string]json.RawMessage{"request.amount": json.RawMessage(strconv.FormatInt(amount, 10))}}
		decision := FreezeBusinessExpectation(g, "enroll", before)
		if decision.Status != "READY" || decision.Frozen == nil {
			t.Fatalf("complete complementary integer predicates lost unique match: %+v", decision)
		}
		want := "otherwise"
		if amount < threshold {
			want = "less"
		}
		if decision.Branch != want {
			t.Fatalf("exact integer ordering changed: amount=%d threshold=%d result=%+v", amount, threshold, decision)
		}
		frozenBytes, err := json.Marshal(decision.Frozen)
		if err != nil {
			t.Fatal(err)
		}
		g.BusinessContract.Branches[0], g.BusinessContract.Branches[1] = g.BusinessContract.Branches[1], g.BusinessContract.Branches[0]
		if permuted := FreezeBusinessExpectation(g, "enroll", before); permuted.Branch != want || permuted.Status != "READY" {
			t.Fatalf("rule order changed a mutually exclusive match: %+v", permuted)
		}
		// Restore the immutable model version before comparing its frozen result.
		g.BusinessContract.Branches[0], g.BusinessContract.Branches[1] = g.BusinessContract.Branches[1], g.BusinessContract.Branches[0]
		after := BusinessObservation{State: "pending", Values: map[string]json.RawMessage{"result.stored": json.RawMessage("false")}}
		actualDenied := true
		if want == "otherwise" {
			after.State, after.Values["result.stored"], actualDenied = "ready", json.RawMessage("true"), false
		}
		comparison := CompareBusinessExpectation(g, *decision.Frozen, after, actualDenied)
		if comparison.Status != "FAIL" || comparison.Branch != want {
			t.Fatalf("actual outcome selected the other branch: %+v", comparison)
		}
		before.Values["request.amount"] = json.RawMessage("null")
		after.Values["result.stored"] = json.RawMessage("null")
		afterBytes, err := json.Marshal(decision.Frozen)
		if err != nil || string(afterBytes) != string(frozenBytes) {
			t.Fatal("caller mutation changed the frozen business contract")
		}
	})
}
