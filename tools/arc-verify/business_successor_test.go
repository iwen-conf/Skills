package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBusinessSuccessorIntersectionNeverWidensExplicitPolicy(t *testing.T) {
	for _, test := range []struct {
		name              string
		left, right, want BusinessSuccessorPolicy
	}{
		{"both undeclared", nil, nil, nil},
		{"left undeclared", nil, BusinessSuccessorPolicy{"confirm"}, BusinessSuccessorPolicy{"confirm"}},
		{"right undeclared", BusinessSuccessorPolicy{"confirm"}, nil, BusinessSuccessorPolicy{"confirm"}},
		{"explicit empty", BusinessSuccessorPolicy{}, nil, BusinessSuccessorPolicy{}},
		{"disjoint", BusinessSuccessorPolicy{"enroll"}, BusinessSuccessorPolicy{"query"}, BusinessSuccessorPolicy{}},
		{"common", BusinessSuccessorPolicy{"query", "confirm"}, BusinessSuccessorPolicy{"confirm", "enroll"}, BusinessSuccessorPolicy{"confirm"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := test.left.Intersect(test.right)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("intersection widened or lost declaration: %#v want %#v", got, test.want)
			}
			copy := got.Clone()
			if !reflect.DeepEqual(copy, got) {
				t.Fatal("clone lost empty/undeclared distinction")
			}
			if len(copy) > 0 {
				copy[0] = "other"
				if got[0] == "other" {
					t.Fatal("clone aliases its input")
				}
			}
			for _, action := range []string{"enroll", "query", "confirm", "clear"} {
				if got.Allows(action) && (!test.left.Allows(action) || !test.right.Allows(action)) {
					t.Fatalf("intersection allowed forbidden %s", action)
				}
			}
		})
	}
}

func TestBusinessSuccessorJSONFreezingPreservesEmptyAndOmitted(t *testing.T) {
	for _, next := range []BusinessSuccessorPolicy{nil, {}, {"enroll"}} {
		g := contractModel(t)
		g.BusinessContract.Branches[0].Expected.Outcomes[0].NextAllowed = next
		body, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		if next == nil && strings.Contains(string(body), `"next_allowed"`) || next != nil && len(next) == 0 && !strings.Contains(string(body), `"next_allowed":[]`) {
			t.Fatalf("model lost successor declaration: %s", body)
		}
		var decoded Graph
		if err := decodeStrictJSON(body, &decoded); err != nil || !reflect.DeepEqual(decoded.BusinessContract.Branches[0].Expected.Outcomes[0].NextAllowed, next) {
			t.Fatalf("model round-trip changed policy: %+v %v", decoded, err)
		}
		frozen := FreezeBusinessExpectation(decoded, "enroll", contractObservation(true))
		if frozen.Status != "READY" || !reflect.DeepEqual(frozen.Frozen.Expected.Outcomes[0].NextAllowed, next) {
			t.Fatalf("freeze changed explicit policy: %+v", frozen)
		}
		after := contractObservation(true)
		after.State = "ready"
		comparison := CompareBusinessExpectation(decoded, *frozen.Frozen, after, false)
		if comparison.Status != "PASS" || !reflect.DeepEqual(comparison.NextAllowed, next) {
			t.Fatalf("comparison changed policy: %+v", comparison)
		}
		captured, err := cloneBusinessJSON(BusinessCampaign{NextAllowed: comparison.NextAllowed, Visits: []BusinessCampaignVisit{{Comparison: &comparison}}})
		if err != nil || !reflect.DeepEqual(captured.NextAllowed, next) || !reflect.DeepEqual(captured.Visits[0].Comparison.NextAllowed, next) {
			t.Fatalf("RAW/campaign round-trip erased the policy: %+v %v", captured, err)
		}
	}
	g := contractModel(t)
	undeclared, _ := businessJSONHash(g)
	g.BusinessContract.Branches[0].Expected.Outcomes[0].NextAllowed = BusinessSuccessorPolicy{}
	empty, _ := businessJSONHash(g)
	if undeclared == empty {
		t.Fatal("declaring a terminal outcome must change the immutable model hash")
	}
}

func TestBusinessSuccessorUnrestrictedAllowedOutcomeDoesNotEraseOtherRestriction(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		g := contractModel(t)
		expected := &g.BusinessContract.Branches[0].Expected
		expected.Mode, expected.Reason = "allowed_outcomes", "synthetic documented overlapping outcomes"
		second := expected.Outcomes[0]
		second.ID, second.NextAllowed = "restricted", BusinessSuccessorPolicy{"enroll"}
		expected.Outcomes = append(expected.Outcomes, second)
		if reversed {
			expected.Outcomes[0], expected.Outcomes[1] = expected.Outcomes[1], expected.Outcomes[0]
		}
		decision := FreezeBusinessExpectation(g, "enroll", contractObservation(true))
		after := contractObservation(true)
		after.State = "ready"
		comparison := CompareBusinessExpectation(g, *decision.Frozen, after, false)
		if comparison.Status != "PASS" || !reflect.DeepEqual(comparison.NextAllowed, BusinessSuccessorPolicy{"enroll"}) {
			t.Fatalf("undeclared outcome erased a matching explicit restriction: %+v", comparison)
		}
	}
}
