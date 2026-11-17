package main

import "testing"

// A branch for a different observed state is not a candidate and cannot impose
// its flow identity requirement on an otherwise complete scoped partition.
func TestRound3BusinessUnrelatedPrestateDoesNotRequireItsFlow(t *testing.T) {
	for _, unrelatedFirst := range []bool{false, true} {
		name := "unrelated-last"
		if unrelatedFirst {
			name = "unrelated-first"
		}
		t.Run(name, func(t *testing.T) {
			g := contractModel(t)
			for i := range g.BusinessContract.Branches {
				g.BusinessContract.Branches[i].Flow = ""
			}
			other, err := cloneBusinessJSON(g.BusinessContract.Branches[0])
			if err != nil {
				t.Fatal(err)
			}
			other.ID, other.Flow, other.FromStates = "ready-only-flow", "flow.enrollment", []string{"ready"}
			other.When = BusinessPredicate{Op: "true"}
			other.SourceRef = "round3-synthetic-spec:flow-is-required-only-for-ready"
			if unrelatedFirst {
				g.BusinessContract.Branches = append([]BusinessBranch{other}, g.BusinessContract.Branches...)
			} else {
				g.BusinessContract.Branches = append(g.BusinessContract.Branches, other)
			}
			g.BusinessContract.Coverage.Scopes[0].Flow = ""
			g.BusinessContract.Coverage.Scope = "both activation values in pending; the ready-state branch is outside this finite scope"
			if err := validateGraph(g); err != nil {
				t.Fatalf("counterexample must use a valid sourced model: %v", err)
			}
			for _, active := range []bool{false, true} {
				before := contractObservation(active)
				before.Flow = ""
				want := "reject"
				if active {
					want = "accept"
				}
				decision := FreezeBusinessExpectation(g, "enroll", before)
				if decision.Status != "READY" || decision.Branch != want || decision.Frozen == nil {
					t.Errorf("unrelated prestate imposed its flow requirement: want=%s decision=%+v", want, decision)
				}
			}
			proof := ProveBusinessCoverage(g)
			if proof.Status != "PROVEN_IN_SCOPE" || !proof.EnumerationComplete || !proof.CompleteInScope || proof.CheckedCases != 2 {
				t.Errorf("unrelated state corrupted a complete finite partition: %+v", proof)
			}
			// A candidate in the actual state still needs its declared flow.
			ready := contractObservation(true)
			ready.State, ready.Flow = "ready", ""
			if decision := FreezeBusinessExpectation(g, "enroll", ready); decision.Status != "BLOCKED" || !containsBusinessID(decision.Unknown, "flow") {
				t.Fatalf("applicable branch's unknown flow must remain blocked: %+v", decision)
			}
			ready.Flow = "flow.enrollment"
			if decision := FreezeBusinessExpectation(g, "enroll", ready); decision.Status != "READY" || decision.Branch != other.ID {
				t.Fatalf("known matching flow must retain its branch: %+v", decision)
			}
		})
	}
}
