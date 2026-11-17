package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func validateRunnerBusinessBindings(g Graph, step RunnerStep, actors map[string]bool) error {
	if step.BusinessBranch == "" && len(step.SubjectBindings) == 0 && len(step.InputMeasurements) == 0 {
		return nil
	}
	if g.BusinessContract == nil {
		return fmt.Errorf("business_branch, subject_bindings and input_measurements require a declared business contract")
	}
	subjects := map[string]bool{}
	attributes := map[string]bool{}
	for _, subject := range g.BusinessContract.Subjects {
		subjects[subject.ID] = true
		for _, attribute := range subject.Attributes {
			attributes[subject.ID+"."+attribute.Name] = true
		}
	}
	for key := range step.InputMeasurements {
		if !attributes[key] {
			return fmt.Errorf("input measurement %s is not a declared business attribute", key)
		}
	}
	if step.BusinessBranch != "" {
		found := false
		for _, branch := range g.BusinessContract.Branches {
			if branch.ID == step.BusinessBranch && branch.Action == step.Node {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("business_branch %s does not belong to action %s", step.BusinessBranch, step.Node)
		}
	}
	for subject, source := range step.SubjectBindings {
		parts := strings.Split(source, ".")
		if !subjects[subject] || len(parts) != 3 || parts[1] == "" {
			return fmt.Errorf("invalid explicit subject binding %s", subject)
		}
		switch parts[0] {
		case "actor":
			if !actors[parts[1]] || parts[2] != "principal" && parts[2] != "account" {
				return fmt.Errorf("subject %s references an unknown actor identity", subject)
			}
		case "entity":
			if parts[2] != "id" {
				return fmt.Errorf("entity subject %s must bind its actual id", subject)
			}
		case "session":
			if parts[2] != "digest" {
				return fmt.Errorf("session subject %s must bind a non-secret digest", subject)
			}
		default:
			return fmt.Errorf("subject %s has unsupported binding source", subject)
		}
	}
	return nil
}

func runtimeBusinessObservation(flow, primaryOracle string, step RunnerStep, actionIndex int, before bool, observed *ModelObservation) (BusinessObservation, error) {
	result := BusinessObservation{Flow: flow, Values: map[string]json.RawMessage{}, Bindings: map[string]string{}, Sources: map[string][]string{}}
	defaultAliases := map[string]bool{}
	defaultBinding := func(alias, value string) {
		if alias == "" {
			return
		}
		if defaultAliases[alias] {
			// Namespace collisions cannot choose an actor, entity or session by
			// insertion order. Keep this default UNKNOWN until an explicit
			// subject_bindings entry selects the intended namespace below.
			delete(result.Bindings, alias)
			return
		}
		defaultAliases[alias] = true
		if value != "" {
			result.Bindings[alias] = value
		}
	}
	for _, participant := range observed.Participants {
		defaultBinding(participant.Alias, participant.Principal)
	}
	entities, sessions := observed.Entities, observed.Sessions
	if before {
		entities, sessions = observed.BeforeEntities, observed.BeforeSessions
	}
	for _, entity := range entities {
		defaultBinding(entity.Alias, entity.ID)
	}
	for _, session := range sessions {
		if session.Active {
			defaultBinding(session.Scope, session.Digest)
		}
	}
	for subject, source := range step.SubjectBindings {
		// Explicit mappings replace exact-alias defaults, including a missing
		// value. A missing source must remain UNKNOWN, never inherit another ID.
		delete(result.Bindings, subject)
		parts := strings.Split(source, ".")
		if len(parts) != 3 {
			return result, fmt.Errorf("invalid subject binding for %s", subject)
		}
		switch parts[0] {
		case "actor":
			for _, participant := range observed.Participants {
				if participant.Alias == parts[1] {
					if parts[2] == "principal" {
						result.Bindings[subject] = participant.Principal
					} else if parts[2] == "account" {
						result.Bindings[subject] = participant.Account
					}
				}
			}
		case "entity":
			for _, entity := range entities {
				if entity.Alias == parts[1] && parts[2] == "id" {
					result.Bindings[subject] = entity.ID
				}
			}
		case "session":
			for _, session := range sessions {
				if session.Scope == parts[1] && parts[2] == "digest" && session.Active {
					result.Bindings[subject] = session.Digest
				}
			}
		}
	}
	stateFound := false
	for _, item := range observed.Oracles {
		if !oracleForAction(item, step.Actor, actionIndex, observed.EvidenceVersion) {
			continue
		}
		facts := item.After
		if before {
			facts = item.Before
		}
		if item.ID == primaryOracle {
			if stateFound {
				return result, fmt.Errorf("duplicate state oracle for actor %s action %d", step.Actor, actionIndex)
			}
			result.State, stateFound = facts.State, true
		}
		for key, value := range facts.Values {
			if _, input := step.InputMeasurements[key]; input || strings.HasPrefix(key, "request.") {
				return result, fmt.Errorf("independent oracle attempted to supply reserved request fact %s", key)
			}
			if prior, exists := result.Values[key]; exists && !sameJSONValue(prior, value) {
				return result, fmt.Errorf("independent oracle conflict for %s on actor %s", key, step.Actor)
			}
			result.Values[key] = append(json.RawMessage(nil), value...)
			seen := false
			for _, id := range result.Sources[key] {
				seen = seen || id == item.ID
			}
			if !seen {
				result.Sources[key] = append(result.Sources[key], item.ID)
				sort.Strings(result.Sources[key])
			}
		}
	}
	if !stateFound {
		return result, fmt.Errorf("missing independent state oracle for actor %s action %d", step.Actor, actionIndex)
	}
	if len(step.InputMeasurements) > 0 {
		request, err := runtimeRequestFor(step, actionIndex, observed)
		if err != nil {
			return result, err
		}
		for key, value := range request.Values {
			if _, exists := result.Values[key]; exists {
				return result, fmt.Errorf("request measurement %s conflicts with an independent oracle", key)
			}
			result.Values[key] = append(json.RawMessage(nil), value...)
			result.Sources[key] = []string{"request-capture"}
		}
	}
	if !before {
		var action *ActionObservation
		for i := range observed.Actions {
			candidate := &observed.Actions[i]
			if candidate.Actor == step.Actor && candidate.ActionIndex == actionIndex {
				if action != nil {
					return result, fmt.Errorf("duplicate response capture for actor %s action %d", step.Actor, actionIndex)
				}
				action = candidate
			}
		}
		if action == nil {
			return result, fmt.Errorf("missing response capture for actor %s action %d", step.Actor, actionIndex)
		}
		transport := map[string]any{"transport.status_code": action.StatusCode, "transport.denied": action.Denied, "transport.exit_code": action.ExitCode}
		for key, value := range transport {
			if _, exists := result.Values[key]; exists {
				return result, fmt.Errorf("independent oracle attempted to replace reserved response fact %s", key)
			}
			result.Values[key], _ = json.Marshal(value)
			result.Sources[key] = []string{"response-capture"}
		}
		if len(action.ResponseValues) != len(step.ResponseMeasurements) {
			return result, fmt.Errorf("response capture differs from the registered measurement fields")
		}
		for key := range step.ResponseMeasurements {
			value, found := action.ResponseValues[key]
			if _, exists := result.Values[key]; exists || !found {
				return result, fmt.Errorf("response measurement %s is missing or conflicts with an independent oracle", key)
			}
			result.Values[key] = append(json.RawMessage(nil), value...)
			result.Sources[key] = []string{"response-capture"}
		}
	}
	return result, nil
}

func freezeRuntimeBusiness(g Graph, flow, oracle string, step RunnerStep, actionIndex int, observed *ModelObservation) BusinessExpectationObservation {
	entry := BusinessExpectationObservation{Actor: step.Actor, ActionIndex: actionIndex}
	if len(step.InputMeasurements) > 0 {
		request, err := runtimeRequestFor(step, actionIndex, observed)
		if err != nil {
			entry.Decision = BusinessDecision{Status: "BLOCKED", Reason: err.Error(), Action: step.Node}
			return entry
		}
		copy := *request
		entry.Request = &copy
	}
	before, err := runtimeBusinessObservation(flow, oracle, step, actionIndex, true, observed)
	if err != nil {
		entry.Decision = BusinessDecision{Status: "BLOCKED", Reason: err.Error(), Action: step.Node}
		return entry
	}
	entry.Decision = FreezeBusinessExpectation(g, step.Node, before)
	if frozen := entry.Decision.Frozen; entry.Decision.Status == "READY" && frozen != nil {
		if step.BusinessBranch != "" && step.BusinessBranch != frozen.Branch {
			entry.Decision.Status, entry.Decision.Reason = "BLOCKED", fmt.Sprintf("observed branch %s differs from planned business branch %s", frozen.Branch, step.BusinessBranch)
		} else if step.Transition != "" && frozen.Transition != "" && step.Transition != frozen.Transition {
			entry.Decision.Status, entry.Decision.Reason = "BLOCKED", fmt.Sprintf("observed transition %s differs from planned transition %s", frozen.Transition, step.Transition)
		}
	}
	return entry
}

func recordRuntimeBusinessComparisons(g Graph, obligation ModelObligation, batch []RunnerStep, observed *ModelObservation) error {
	x := indexModel(g)
	for i := range observed.BusinessExpectations {
		entry := &observed.BusinessExpectations[i]
		if entry.Decision.Status != "READY" || entry.Decision.Frozen == nil {
			continue
		}
		position := entry.ActionIndex - 1
		if position < 0 || position >= len(batch) || position >= len(observed.Actions) || entry.Actor != batch[position].Actor {
			return fmt.Errorf("business expectation has no matching executed action")
		}
		step := batch[position]
		oracle := stepRule(x, obligation, step.Transition).Oracle
		after, err := runtimeBusinessObservation(obligation.Definition.Flow, oracle, step, entry.ActionIndex, false, observed)
		if err != nil {
			return err
		}
		comparison := CompareBusinessExpectation(g, *entry.Decision.Frozen, after, observed.Actions[position].Denied)
		entry.After, entry.Comparison = &after, &comparison
	}
	return nil
}

func verifyRuntimeBusiness(g Graph, obligation ModelObligation, batch []RunnerStep, observed *ModelObservation) (string, string, []StepAssertion) {
	if observed.EvidenceVersion < runtimeEvidenceVersion || len(observed.BusinessExpectations) != len(batch) {
		return "BLOCKED", "business contract requires a frozen pre-action expectation for every action", nil
	}
	requests := 0
	for _, step := range batch {
		if len(step.InputMeasurements) > 0 {
			requests++
		}
	}
	if len(observed.Requests) != requests {
		return "BLOCKED", "pre-action request capture count differs from the frozen manifest", nil
	}
	verdict, reason := "PASS", "independent observations satisfy the frozen business expectation"
	assertions := make([]StepAssertion, 0, len(batch))
	x := indexModel(g)
	for i, step := range batch {
		entry := observed.BusinessExpectations[i]
		assertion := StepAssertion{Kind: "business_expectation", Name: entry.Decision.Branch, Subject: step.Actor, Operator: "frozen_contract", Verdict: "BLOCKED"}
		if entry.Actor != step.Actor || entry.ActionIndex != i+1 {
			assertion.Reason = "business expectation belongs to another actor or action"
		} else {
			oracle := stepRule(x, obligation, step.Transition).Oracle
			assertion.OracleID = oracle
			// Rebuild solely from the captured BEFORE facts. Never choose an
			// expectation using the actual outcome under examination.
			planned := freezeRuntimeBusiness(g, obligation.Definition.Flow, oracle, step, i+1, observed)
			expectedDecision, _ := json.Marshal(planned.Decision)
			capturedDecision, _ := json.Marshal(entry.Decision)
			expectedRequest, _ := json.Marshal(planned.Request)
			capturedRequest, _ := json.Marshal(entry.Request)
			if !sameJSONValue(expectedRequest, capturedRequest) {
				assertion.Reason = "frozen request differs from the captured pre-action input"
			} else if !sameJSONValue(expectedDecision, capturedDecision) {
				assertion.Reason = "frozen business decision disagrees with independent pre-action facts"
			} else if entry.Decision.Status != "READY" || entry.Decision.Frozen == nil {
				assertion.Reason = entry.Decision.Status + ": " + entry.Decision.Reason
			} else if entry.After == nil || entry.Comparison == nil || i >= len(observed.Actions) {
				assertion.Reason = "business comparison has no captured action or independent after state"
			} else {
				after, err := runtimeBusinessObservation(obligation.Definition.Flow, oracle, step, i+1, false, observed)
				if err != nil {
					assertion.Reason = err.Error()
				} else {
					comparison := CompareBusinessExpectation(g, *entry.Decision.Frozen, after, observed.Actions[i].Denied)
					a, _ := json.Marshal(after)
					b, _ := json.Marshal(entry.After)
					c, _ := json.Marshal(comparison)
					d, _ := json.Marshal(entry.Comparison)
					if !sameJSONValue(a, b) || !sameJSONValue(c, d) {
						assertion.Reason = "recorded business comparison differs from independent after facts"
					} else {
						assertion.Verdict, assertion.Reason = comparison.Status, comparison.Reason
						if assertion.Verdict != "PASS" && assertion.Verdict != "FAIL" {
							assertion.Verdict = "BLOCKED"
						}
						assertion.Actual = string(c)
					}
				}
			}
		}
		if entry.Decision.Frozen != nil {
			body, _ := json.Marshal(entry.Decision.Frozen.Expected)
			assertion.Expected = string(body)
		}
		assertions = append(assertions, assertion)
		if assertion.Verdict == "BLOCKED" || assertion.Verdict == "FAIL" && verdict != "BLOCKED" {
			verdict, reason = assertion.Verdict, assertion.Reason
		}
	}
	return verdict, reason, assertions
}
