package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func businessJSONHash(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func cloneBusinessJSON[T any](value T) (T, error) {
	var copied T
	body, err := json.Marshal(value)
	if err != nil {
		return copied, err
	}
	err = json.Unmarshal(body, &copied)
	return copied, err
}

func containsBusinessID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// FreezeBusinessExpectation selects from the PRE-action observation only. The
// returned value is detached from Graph and its observation inputs. Persist it
// before sending an action; any other status forbids a strong-contract action.
func FreezeBusinessExpectation(g Graph, action string, before BusinessObservation) BusinessDecision {
	result := BusinessDecision{Status: "BLOCKED", Action: action, Matches: []string{}}
	if g.BusinessContract == nil {
		result.Status, result.Reason = "LEGACY_UNDECLARED", "graph has no deterministic business contract; legacy evidence has its original scope"
		return result
	}
	if err := ValidateBusinessContract(g); err != nil {
		result.Status, result.Reason = "MODEL_ERROR", err.Error()
		return result
	}
	result = selectBusinessBranch(g.BusinessContract, action, before)
	if result.Status != "READY" {
		return result
	}
	var branch BusinessBranch
	for _, candidate := range g.BusinessContract.Branches {
		if candidate.ID == result.Branch {
			branch = candidate
			break
		}
	}
	contractHash, err := businessJSONHash(g)
	if err != nil {
		result.Status, result.Reason = "MODEL_ERROR", err.Error()
		return result
	}
	beforeHash, err := businessJSONHash(before)
	if err != nil {
		result.Reason = "observation cannot be frozen: " + err.Error()
		return result
	}
	copiedBefore, err := cloneBusinessJSON(before)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	expected, err := cloneBusinessJSON(branch.Expected)
	if err != nil {
		result.Status, result.Reason = "MODEL_ERROR", err.Error()
		return result
	}
	frozen := FrozenBusinessExpectation{
		Version: 1, ContractSHA256: contractHash, Branch: branch.ID, Action: action,
		Rule: branch.Rule, Flow: before.Flow, Transition: branch.Transition,
		BeforeSHA256: beforeHash, Before: copiedBefore, Expected: expected,
	}
	frozen.SHA256, err = businessJSONHash(frozen)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	result.Status, result.Reason, result.Branch, result.Frozen = "READY", "exactly one confirmed branch matched; expectation frozen before the action", branch.ID, &frozen
	return result
}

func selectBusinessBranch(contract *BusinessContract, action string, before BusinessObservation) BusinessDecision {
	return selectBusinessBranchIndexed(contract, action, before, businessAttributeIndex(contract))
}

func selectBusinessBranchIndexed(contract *BusinessContract, action string, before BusinessObservation, attributes map[string]businessAttributeInfo) BusinessDecision {
	result := BusinessDecision{Status: "BLOCKED", Action: action, Matches: []string{}}
	if before.State == "" || before.State == "UNKNOWN" {
		result.Reason, result.Unknown = "independent pre-action state is unknown", []string{"state"}
		return result
	}
	for _, branch := range contract.Branches {
		if branch.Action != action {
			continue
		}
		if branch.Flow != "" && before.Flow == "" {
			result.Reason, result.Unknown = "business flow identity is unknown", []string{"flow"}
			return result
		}
		if branch.Flow != "" && branch.Flow != before.Flow || !containsBusinessID(branch.FromStates, before.State) {
			continue
		}
		for _, key := range businessPredicateRefs(branch.When) {
			info := attributes[key]
			if info.subject.RequiresBinding && strings.TrimSpace(before.Bindings[info.subject.ID]) == "" {
				result.Unknown = append(result.Unknown, "binding:"+info.subject.ID)
			}
		}
	}
	if len(result.Unknown) > 0 {
		result.Unknown = uniqueSortedBusiness(result.Unknown)
		result.Reason = "required business subject identity is unknown"
		return result
	}
	matches := []BusinessBranch{}
	unknown := []string{}
	actionKnown := false
	for _, branch := range contract.Branches {
		if branch.Action != action {
			continue
		}
		actionKnown = true
		if branch.Flow != "" && before.Flow != "" && branch.Flow != before.Flow {
			continue
		}
		if before.State != "" && before.State != "UNKNOWN" && !containsBusinessID(branch.FromStates, before.State) {
			continue
		}
		when := evaluateBusinessPredicate(branch.When, before, attributes)
		if when.Value == businessFalse {
			continue
		}
		if before.State == "" || before.State == "UNKNOWN" {
			unknown = append(unknown, "state")
			continue
		}
		if branch.Flow != "" && before.Flow == "" {
			unknown = append(unknown, "flow:"+branch.Flow)
			continue
		}
		if !branch.Confirmed {
			unknown = append(unknown, "confirmation:"+branch.ID)
			continue
		}
		if when.Value == businessUnknown {
			unknown = append(unknown, when.Unknown...)
			continue
		}
		matches = append(matches, branch)
		result.Matches = append(result.Matches, branch.ID)
	}
	sort.Strings(result.Matches)
	result.Unknown = uniqueSortedBusiness(unknown)
	if len(matches) > 1 {
		result.Status, result.Reason = "MODEL_ERROR", "multiple confirmed branches match the same action, flow, prestate and attributes"
		return result
	}
	if len(result.Unknown) > 0 {
		result.Reason = "branch uniqueness cannot be established from known, confirmed independent observations"
		return result
	}
	if len(matches) == 0 {
		result.Status, result.Reason = "SPECIFICATION_GAP", "no confirmed branch matches the observed action, flow, prestate and attributes"
		if !actionKnown {
			result.Reason = "action has no declared deterministic business branch"
		}
		return result
	}
	result.Status, result.Reason, result.Branch = "READY", "exactly one confirmed branch matched", matches[0].ID
	return result
}

func evaluateBusinessCheck(contract *BusinessContract, check BusinessExpectedCheck, after BusinessObservation) BusinessAssertionResult {
	result := BusinessAssertionResult{ID: check.ID, Kind: check.Kind, Oracle: check.Oracle, SourceRef: check.SourceRef}
	unknown := []string{}
	for _, key := range businessPredicateRefs(check.Predicate) {
		if !containsBusinessID(after.Sources[key], check.Oracle) {
			unknown = append(unknown, "oracle:"+check.Oracle+":"+key)
		}
	}
	if len(unknown) > 0 {
		result.Evaluation = unknownBusiness(unknown, "required independent oracle did not supply this assertion's observations")
	} else {
		result.Evaluation = EvaluateBusinessPredicate(contract, check.Predicate, after)
	}
	return result
}

func businessConjunction(values []TruthEvaluation) TruthEvaluation {
	unknown := []string{}
	unknownFound := false
	for _, value := range values {
		if value.Value == businessFalse {
			return businessTruth(false)
		}
		if value.Value == businessUnknown {
			unknownFound = true
			unknown = append(unknown, value.Unknown...)
		}
	}
	if unknownFound {
		return unknownBusiness(unknown, "required expected observation is unknown")
	}
	return businessTruth(true)
}

// CompareBusinessExpectation never chooses a new branch from Actual. It checks
// the recorded pre-action freeze against its original prestate, then compares
// only that fixed outcome or explicitly allowed outcome set and invariants.
func CompareBusinessExpectation(g Graph, frozen FrozenBusinessExpectation, after BusinessObservation, actualDenied bool) BusinessComparison {
	result := BusinessComparison{Status: "BLOCKED", Branch: frozen.Branch, Checks: []BusinessCheckResult{}}
	if g.BusinessContract == nil {
		result.Status, result.Reason = "LEGACY_UNDECLARED", "graph has no deterministic business contract"
		return result
	}
	if frozen.Version != 1 || frozen.SHA256 == "" {
		result.Reason = "missing versioned pre-action expectation"
		return result
	}
	original := FreezeBusinessExpectation(g, frozen.Action, frozen.Before)
	if original.Status != "READY" || original.Frozen == nil || original.Frozen.SHA256 != frozen.SHA256 {
		result.Status, result.Reason = "MODEL_ERROR", "frozen expectation no longer matches its original model and pre-action observation"
		return result
	}
	copyForHash := frozen
	copyForHash.SHA256 = ""
	hash, err := businessJSONHash(copyForHash)
	if err != nil || hash != frozen.SHA256 {
		result.Status, result.Reason = "MODEL_ERROR", "frozen expectation integrity mismatch"
		return result
	}
	if after.Flow != frozen.Before.Flow {
		result.Status, result.Reason = "FAIL", "observed business flow changed during the frozen action"
		return result
	}
	identityUnknown := []string{}
	identityFailed := []string{}
	for _, subject := range g.BusinessContract.Subjects {
		beforeID := frozen.Before.Bindings[subject.ID]
		if beforeID == "" {
			continue
		}
		afterID := after.Bindings[subject.ID]
		if afterID == "" {
			identityUnknown = append(identityUnknown, "binding:"+subject.ID)
		} else if afterID != beforeID {
			identityFailed = append(identityFailed, subject.ID)
		}
	}
	if len(identityFailed) > 0 {
		result.Status, result.Reason = "FAIL", "bound business identity changed: "+strings.Join(uniqueSortedBusiness(identityFailed), ", ")
		return result
	}
	invariants := []TruthEvaluation{}
	if len(identityUnknown) > 0 {
		invariants = append(invariants, unknownBusiness(identityUnknown, "previously bound identity was not independently observed after the action"))
	}
	for _, check := range frozen.Expected.CommonInvariants {
		assertion := evaluateBusinessCheck(g.BusinessContract, check, after)
		result.InvariantChecks = append(result.InvariantChecks, assertion)
		invariants = append(invariants, assertion.Evaluation)
	}
	invariantResult := businessConjunction(invariants)
	matched := []BusinessExpectedOutcome{}
	unknownOutcomes := []string{}
	for _, outcome := range frozen.Expected.Outcomes {
		check := BusinessCheckResult{Outcome: outcome.ID, Predicate: EvaluateBusinessPredicate(g.BusinessContract, outcome.Predicate, after)}
		parts := []TruthEvaluation{check.Predicate}
		if outcome.State != "" {
			target := outcome.State
			if target == "UNCHANGED" {
				target = frozen.Before.State
			}
			if after.State == "" || after.State == "UNKNOWN" {
				parts = append(parts, unknownBusiness([]string{"state"}, "independent poststate is unknown"))
			} else {
				passed := target == after.State
				check.StatePassed = &passed
				parts = append(parts, businessTruth(passed))
			}
		}
		if outcome.Denied != nil {
			passed := *outcome.Denied == actualDenied
			check.DeniedPassed = &passed
			parts = append(parts, businessTruth(passed))
		}
		for _, obligation := range outcome.Checks {
			assertion := evaluateBusinessCheck(g.BusinessContract, obligation, after)
			check.Assertions = append(check.Assertions, assertion)
			parts = append(parts, assertion.Evaluation)
		}
		status := businessConjunction(parts)
		if status.Value == businessTrue {
			matched = append(matched, outcome)
		}
		if status.Value == businessUnknown {
			unknownOutcomes = append(unknownOutcomes, status.Unknown...)
		}
		result.Checks = append(result.Checks, check)
	}
	if invariantResult.Value == businessFalse {
		result.Status, result.Reason = "FAIL", "a frozen common business invariant is false"
		return result
	}
	if len(matched) == 0 && len(unknownOutcomes) == 0 {
		result.Status, result.Reason = "FAIL", "actual result violates every outcome permitted by the pre-action frozen expectation"
		return result
	}
	if invariantResult.Value == businessUnknown || len(matched) == 0 {
		result.Unknown = uniqueSortedBusiness(append(unknownOutcomes, invariantResult.Unknown...))
		result.Reason = "independent observations are insufficient for the frozen expectation"
		return result
	}
	result.Status, result.Reason = "PASS", "actual result satisfies the frozen expected outcome and every common invariant"
	if len(matched) == 1 {
		result.Outcome = matched[0].ID
	}
	// If explicit allowed outcomes overlap, report only next actions allowed by
	// every matching outcome. Actual must never silently widen the next step.
	for _, outcome := range matched {
		result.NextAllowed = result.NextAllowed.Intersect(outcome.NextAllowed)
	}
	sort.Strings(result.NextAllowed)
	if len(matched) > 1 {
		result.Reason = fmt.Sprintf("actual result is within %d explicitly allowed outcomes; next_allowed is their intersection", len(matched))
	}
	return result
}

// FreezeBusinessBranch is for an execution card that names a specific branch.
// It still selects using Actual prestate and blocks, rather than executes, when
// the planned target is different. A caller may never substitute target facts.
func FreezeBusinessBranch(g Graph, branchID string, before BusinessObservation) BusinessDecision {
	if g.BusinessContract == nil {
		return FreezeBusinessExpectation(g, "", before)
	}
	for _, branch := range g.BusinessContract.Branches {
		if branch.ID != branchID {
			continue
		}
		decision := FreezeBusinessExpectation(g, branch.Action, before)
		if decision.Status == "READY" && decision.Branch != branchID {
			decision.Status, decision.Reason, decision.Frozen = "BLOCKED", "observed prestate selects a different branch than the execution card's target", nil
		}
		return decision
	}
	return BusinessDecision{Status: "SPECIFICATION_GAP", Reason: "execution card names an undeclared business branch", Matches: []string{}}
}
