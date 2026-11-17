package main

import (
	"fmt"
)

// businessSuccessorsAfter combines only the checked outcomes of one completed
// batch. Parallel actions do not invent an order between themselves; the next
// ordered batch must satisfy every explicit successor restriction. This state
// belongs to a single registered run/flow, never to an oracle name or account
// cache, so a declared actor change cannot bypass the preceding operation.
func businessSuccessorsAfter(observed *ModelObservation) (BusinessSuccessorPolicy, error) {
	if observed == nil || len(observed.BusinessExpectations) == 0 {
		return nil, fmt.Errorf("previous operation has no verified business expectation")
	}
	var policy BusinessSuccessorPolicy
	for _, entry := range observed.BusinessExpectations {
		if entry.Decision.Status != "READY" || entry.Decision.Frozen == nil || entry.After == nil || entry.Comparison == nil || entry.Comparison.Status != "PASS" {
			return nil, fmt.Errorf("previous operation has no passing frozen business comparison")
		}
		policy = policy.Intersect(entry.Comparison.NextAllowed)
	}
	return policy, nil
}

func checkBusinessSuccessors(policy BusinessSuccessorPolicy, batch []RunnerStep) error {
	for _, step := range batch {
		if !policy.Allows(step.Node) {
			return fmt.Errorf("next_allowed from the previous verified operation forbids action %s for actor %s", step.Node, step.Actor)
		}
	}
	return nil
}

// precedingBusinessSuccessors is the offline counterpart of the live run's
// ordered policy. Recheck the original previous RAW and its local expectation,
// rather than trusting a cached policy or PASS column. Whole-run verification
// checks each adjacent pair; local revalidation avoids recursive/quadratic IO.
func (d db) precedingBusinessSuccessors(contract runContract, run string, index int) (BusinessSuccessorPolicy, error) {
	if index <= 1 {
		return nil, nil
	}
	rows, err := d.query(fmt.Sprintf("SELECT verdict FROM steps WHERE run=%s AND idx=%d", quote(run), index-1), 1)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 || rows[0][0] != "PASS" {
		return nil, fmt.Errorf("previous ordered operation has no recorded PASS")
	}
	raw, hash, err := d.workbenchEvidence(run, "action", index-1)
	if err != nil {
		return nil, err
	}
	if err := contract.Binding.matchesRAW(raw); err != nil {
		return nil, err
	}
	verdict, reason, _, err := d.evaluateModelObservationLocal(contract, raw, index-1, hash)
	if err != nil {
		return nil, err
	}
	if verdict != "PASS" {
		return nil, fmt.Errorf("previous operation's independent evidence does not pass: %s", reason)
	}
	return businessSuccessorsAfter(raw.Model)
}
