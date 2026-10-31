package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

type FaultVerdict string

const (
	FaultDetected      FaultVerdict = "detected"
	FaultSurvived      FaultVerdict = "survived"
	FaultUnreachable   FaultVerdict = "unreachable"
	FaultInconclusive  FaultVerdict = "inconclusive"
	FaultFalsePositive FaultVerdict = "false_positive"
	FaultFalseNegative FaultVerdict = "false_negative"
)

type MutationActivation struct {
	Action              string
	Role                string
	ObservedState       string
	SameRequestIDGrowth bool
	DistinctRequestIDs  int
	OverCapacity        bool
	GuardFalse          string
	Changed             bool
}

type MutationLink struct {
	Rule          string
	Kind          string
	Action        string
	ObservedState string
	SameRequestID bool
	DistinctIDs   bool
	GuardFalse    string
}

type SealedMutation struct {
	ID         string
	Category   string
	InjectSite string
	Rule       string
	Activation MutationActivation
	Link       MutationLink
}

type MutationAttribution struct {
	MutationID     string           `json:"mutation_id"`
	Category       string           `json:"category"`
	InjectSite     string           `json:"inject_site"`
	Activated      bool             `json:"activated"`
	Verdict        FaultVerdict     `json:"verdict"`
	Sequence       []string         `json:"sequence"`
	RunID          string           `json:"run_id"`
	OracleEvidence []string         `json:"oracle_evidence"`
	Linked         []ExploreFinding `json:"linked"`
	Reason         string           `json:"reason"`
}

type BlindCampaignResult struct {
	Attributions []MutationAttribution `json:"attributions"`
	Counts       map[FaultVerdict]int  `json:"counts"`
	Budget       ExploreRuntimeBudget  `json:"budget"`
	Spent        ExploreSpent          `json:"spent"`
}

type ExploreSpent struct {
	Actions   int   `json:"actions"`
	States    int   `json:"states"`
	Replans   int   `json:"replans"`
	ElapsedMS int64 `json:"elapsed_ms"`
}

func (r BlindCampaignResult) ByID(id string) MutationAttribution {
	for _, item := range r.Attributions {
		if item.MutationID == id {
			return item
		}
	}
	return MutationAttribution{MutationID: id}
}

type scoredFinding struct {
	step    int
	finding ExploreFinding
	index   int
	score   int
}

func AttributeBlindCampaign(report ExploreReport, sealed []SealedMutation, budget ExploreRuntimeBudget) BlindCampaignResult {
	result := BlindCampaignResult{
		Attributions: []MutationAttribution{},
		Counts:       map[FaultVerdict]int{},
		Budget:       budget,
		Spent:        report.Spent,
	}
	if result.Spent.Actions == 0 {
		result.Spent.Actions = report.Actions
		result.Spent.States = report.States
	}
	claimed := map[int]string{}
	pending := make([]MutationAttribution, len(sealed))
	candidates := make([][]scoredFinding, len(sealed))
	for i, mutation := range sealed {
		pending[i] = inspectMutation(report, mutation)
		candidates[i] = linkableFindings(report, mutation, pending[i].Activated)
	}
	order := make([]int, len(sealed))
	for i := range sealed {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return specificity(sealed[order[i]].Link) > specificity(sealed[order[j]].Link)
	})
	for _, i := range order {
		best := scoredFinding{index: -1}
		for _, item := range candidates[i] {
			if _, taken := claimed[item.index]; taken {
				continue
			}
			if item.score > best.score {
				best = item
			}
		}
		if best.index >= 0 && pending[i].Activated {
			claimed[best.index] = sealed[i].ID
			pending[i].Linked = []ExploreFinding{best.finding}
			if pending[i].RunID == "" {
				pending[i].RunID = report.Campaign
			}
			if best.finding.Evidence != "" {
				pending[i].OracleEvidence = appendUnique(pending[i].OracleEvidence, best.finding.Evidence)
			}
			if best.step >= 0 && best.step < len(report.Steps) && report.Steps[best.step].RAW != "" {
				pending[i].OracleEvidence = appendUnique(pending[i].OracleEvidence, report.Steps[best.step].RAW)
			}
			pending[i].Verdict = FaultDetected
			pending[i].Reason = "FAIL " + best.finding.Rule + " is linked to " + sealed[i].ID + " by independent oracle facts"
		}
	}
	for i, mutation := range sealed {
		item := pending[i]
		if item.Verdict == "" {
			item.Verdict = mutationVerdict(item, report, mutation)
		}
		if item.Verdict == FaultDetected && !item.Activated {
			item.Verdict = FaultFalsePositive
			item.Reason = "a FAIL was claimed without an activated mutation"
		}
		if item.Verdict == FaultSurvived {
			item.Verdict = FaultFalseNegative
		}
		result.Counts[item.Verdict]++
		result.Attributions = append(result.Attributions, item)
	}
	return result
}

func inspectMutation(report ExploreReport, mutation SealedMutation) MutationAttribution {
	item := MutationAttribution{
		MutationID:     mutation.ID,
		Category:       mutation.Category,
		InjectSite:     mutation.InjectSite,
		Sequence:       []string{},
		OracleEvidence: []string{},
		Linked:         []ExploreFinding{},
	}
	entered := false
	for _, step := range report.Steps {
		if !stepMatchesActivationAction(step, mutation.Activation) {
			continue
		}
		entered = true
		item.Sequence = append(item.Sequence, string(step.Candidate.Kind)+":"+step.Candidate.Action+":"+step.Candidate.RequestID)
		if step.RAW != "" {
			item.OracleEvidence = appendUnique(item.OracleEvidence, step.RAW)
		}
		if activatedByStep(step, report, mutation.Activation) {
			item.Activated = true
			item.RunID = firstNonEmpty(item.RunID, report.Campaign)
		}
	}
	if !item.Activated && !entered {
		item.Reason = "trigger conditions were never entered"
	}
	if !item.Activated && entered {
		item.Reason = "the trigger ran but the fault signature was not observed"
	}
	return item
}

func stepMatchesActivationAction(step ExploreStep, spec MutationActivation) bool {
	if spec.Action != "" && step.Candidate.Action != spec.Action {
		return false
	}
	if spec.Role != "" && actorName(step) != spec.Role && step.Candidate.Role != spec.Role {
		return false
	}
	return spec.Action != "" || spec.Role != "" || spec.GuardFalse != ""
}

func activatedByStep(step ExploreStep, report ExploreReport, spec MutationActivation) bool {
	changed := step.After.State != step.Before.State || measurementGrew(step.Before, step.After, "records")
	if spec.ObservedState != "" && step.After.State != spec.ObservedState {
		return false
	}
	if spec.Changed && !changed {
		return false
	}
	if spec.GuardFalse != "" {
		value, ok := step.Before.Conditions[spec.GuardFalse]
		if !ok || value {
			return false
		}
		if !changed {
			return false
		}
	}
	if spec.SameRequestIDGrowth && !sameRequestGrew(step, report) {
		return false
	}
	if spec.DistinctRequestIDs > 0 && distinctRequestIDs(step, report) < spec.DistinctRequestIDs {
		return false
	}
	if spec.OverCapacity && !overCapacity(step.After) {
		return false
	}
	if spec.ObservedState == "" && !spec.Changed && !spec.SameRequestIDGrowth && spec.DistinctRequestIDs == 0 && spec.GuardFalse == "" && !spec.OverCapacity {
		return false
	}
	return true
}

func linkableFindings(report ExploreReport, mutation SealedMutation, activated bool) []scoredFinding {
	out := []scoredFinding{}
	index := 0
	for stepIndex, step := range report.Steps {
		for _, finding := range step.Findings {
			current := index
			index++
			if finding.Kind == "PASS" || finding.Kind == "RISK" {
				continue
			}
			if mutation.Link.Kind != "" && finding.Kind != mutation.Link.Kind {
				continue
			}
			if mutation.Link.Rule != "" && finding.Rule != mutation.Link.Rule {
				continue
			}
			if mutation.Link.Action != "" && step.Candidate.Action != mutation.Link.Action {
				continue
			}
			if mutation.Link.ObservedState != "" && step.After.State != mutation.Link.ObservedState {
				continue
			}
			if mutation.Link.GuardFalse != "" {
				value, ok := step.Before.Conditions[mutation.Link.GuardFalse]
				if !ok || value {
					continue
				}
			}
			if mutation.Link.SameRequestID && !sameRequestGrew(step, report) {
				continue
			}
			if mutation.Link.DistinctIDs && distinctRequestIDs(step, report) < 2 {
				continue
			}
			score := specificity(mutation.Link)
			if activated {
				score += 10
			}
			out = append(out, scoredFinding{step: stepIndex, finding: finding, index: current, score: score})
		}
	}
	return out
}

func specificity(link MutationLink) int {
	score := 0
	if link.SameRequestID {
		score += 8
	}
	if link.DistinctIDs {
		score += 8
	}
	if link.ObservedState != "" {
		score += 6
	}
	if link.GuardFalse != "" {
		score += 6
	}
	if link.Action != "" {
		score += 2
	}
	if link.Rule != "" {
		score += 1
	}
	return score
}

func mutationVerdict(item MutationAttribution, report ExploreReport, mutation SealedMutation) FaultVerdict {
	if item.Activated && len(item.Linked) > 0 {
		return FaultDetected
	}
	if !item.Activated && item.Reason == "trigger conditions were never entered" {
		return FaultUnreachable
	}
	if !item.Activated {
		return FaultUnreachable
	}
	if oracleMissing(report) {
		return FaultInconclusive
	}
	return FaultSurvived
}

func oracleMissing(report ExploreReport) bool {
	for _, step := range report.Steps {
		if step.After.State == "" || step.RAW == "" {
			return true
		}
		for _, finding := range step.Findings {
			if finding.Kind == "BLOCKED" {
				return true
			}
		}
	}
	return len(report.Steps) == 0
}

func sameRequestGrew(step ExploreStep, report ExploreReport) bool {
	id := step.Candidate.RequestID
	if id == "" {
		return false
	}
	if step.Candidate.Kind == kindRepeat && measurementGrew(step.Before, step.After, "records") {
		return true
	}
	minRecords, maxRecords, seen := -1, -1, 0
	for _, item := range report.Steps {
		if item.Candidate.RequestID != id && !callHasRequest(item, id) {
			continue
		}
		seen++
		if value, ok := factInt(item.After, "records"); ok {
			if minRecords < 0 || value < minRecords {
				minRecords = value
			}
			if value > maxRecords {
				maxRecords = value
			}
		}
		if value, ok := factInt(item.Before, "records"); ok {
			if minRecords < 0 || value < minRecords {
				minRecords = value
			}
		}
	}
	return seen >= 1 && maxRecords > minRecords && (step.Candidate.Kind == kindRepeat || seen > 1 || measurementGrew(step.Before, step.After, "records") && step.Candidate.Kind == kindRepeat)
}

func callHasRequest(step ExploreStep, id string) bool {
	for _, call := range step.Calls {
		if call.RequestID == id {
			return true
		}
	}
	return false
}

func distinctRequestIDs(step ExploreStep, report ExploreReport) int {
	ids := map[string]bool{}
	if step.Candidate.RequestID != "" {
		ids[step.Candidate.RequestID] = true
	}
	for _, call := range step.Calls {
		if call.RequestID != "" {
			ids[call.RequestID] = true
		}
	}
	if step.Candidate.Kind == kindConcurrent {
		return len(ids)
	}
	for _, item := range report.Steps {
		if item.Candidate.Action != step.Candidate.Action {
			continue
		}
		if item.Candidate.RequestID != "" {
			ids[item.Candidate.RequestID] = true
		}
		for _, call := range item.Calls {
			if call.RequestID != "" {
				ids[call.RequestID] = true
			}
		}
	}
	return len(ids)
}

func actorName(step ExploreStep) string {
	if step.After.Role != "" {
		return step.After.Role
	}
	return step.Candidate.Role
}

func factInt(facts IndependentFacts, name string) (int, bool) {
	if facts.Values == nil {
		return 0, false
	}
	raw, ok := facts.Values[name]
	if !ok || !json.Valid(raw) {
		return 0, false
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false
	}
	return n, true
}

func measurementGrew(before, after IndependentFacts, name string) bool {
	left, lok := factInt(before, name)
	right, rok := factInt(after, name)
	return lok && rok && right > left
}

func overCapacity(facts IndependentFacts) bool {
	records, rok := factInt(facts, "records")
	capacity, cok := factInt(facts, "capacity")
	return rok && cok && records > capacity
}

func seatBlindMutations() []SealedMutation {
	return []SealedMutation{
		{ID: "B-STATE", Category: "illegal_transition", InjectSite: "claimLocked.skipReview", Rule: "R-CLAIM", Activation: MutationActivation{Action: "claim", ObservedState: "enrolled"}, Link: MutationLink{Rule: "R-CLAIM", Kind: "FAIL", ObservedState: "enrolled"}},
		{ID: "B-AUTH", Category: "unauthorized_access", InjectSite: "handleForce.skipAdmin", Rule: "R-NO-FORCE", Activation: MutationActivation{Action: "force", Role: "STUDENT", Changed: true}, Link: MutationLink{Rule: "R-NO-FORCE", Kind: "FAIL", Action: "force"}},
		{ID: "B-IDEM", Category: "idempotency", InjectSite: "claimLocked.duplicateRecords", Rule: "invariant.capacity", Activation: MutationActivation{Action: "claim", SameRequestIDGrowth: true}, Link: MutationLink{Rule: "invariant.capacity", Kind: "FAIL", SameRequestID: true}},
		{ID: "B-RACE", Category: "race_oversell", InjectSite: "raceClaim.staleOccupancy", Rule: "invariant.capacity", Activation: MutationActivation{Action: "claim", DistinctRequestIDs: 2, OverCapacity: true}, Link: MutationLink{Rule: "invariant.capacity", Kind: "FAIL", DistinctIDs: true}},
		{ID: "B-VALID", Category: "missing_server_validation", InjectSite: "claimLocked.acceptWhenClosed", Rule: "R-CLAIM", Activation: MutationActivation{Action: "claim", GuardFalse: "course_open", Changed: true}, Link: MutationLink{Rule: "R-CLAIM", Kind: "FAIL", GuardFalse: "course_open"}},
	}
}

func formatAttribution(result BlindCampaignResult) string {
	return fmt.Sprintf("detected=%d survived=%d unreachable=%d inconclusive=%d fp=%d fn=%d",
		result.Counts[FaultDetected], result.Counts[FaultSurvived], result.Counts[FaultUnreachable],
		result.Counts[FaultInconclusive], result.Counts[FaultFalsePositive], result.Counts[FaultFalseNegative])
}
