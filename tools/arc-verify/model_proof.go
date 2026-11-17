package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

func runnerBatches(manifest RunnerManifest) [][]RunnerStep {
	result := [][]RunnerStep{}
	for _, step := range manifest.Steps {
		if step.ParallelGroup != "" && len(result) > 0 && result[len(result)-1][0].ParallelGroup == step.ParallelGroup {
			result[len(result)-1] = append(result[len(result)-1], step)
		} else {
			result = append(result, []RunnerStep{step})
		}
	}
	return result
}

func factsHash(facts IndependentFacts) (string, error) {
	body, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	return digestBytes(body), nil
}

func observedConditions(actual map[string]bool, expected map[string]bool, required []string) bool {
	for _, id := range required {
		if _, found := actual[id]; !found {
			return false
		}
	}
	for id, value := range expected {
		observed, found := actual[id]
		if !found || observed != value {
			return false
		}
	}
	return true
}

func (d db) captureModelRAW(run string, index int, raw []byte) error {
	hash := digestBytes(raw)
	return d.transition([]string{fmt.Sprintf("INSERT INTO model_captures(run,idx,sha256) SELECT %s,%d,%s FROM model_runs WHERE run=%s", quote(run), index, quote(hash), quote(run))})
}

func (d db) modelCaptureExists(run string, index int, hash string) error {
	rows, err := d.query(fmt.Sprintf("SELECT sha256 FROM model_captures WHERE run=%s AND idx=%d AND sha256=%s", quote(run), index, quote(hash)), 1)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("model RAW was not captured by the registered execution engine")
	}
	return nil
}

func (d db) modelObservationVerdict(contract runContract, raw StepRAW, index int, hash string) (string, error) {
	verdict, _, _, err := d.evaluateModelObservation(contract, raw, index, hash)
	return verdict, err
}

func (d db) evaluateModelObservation(contract runContract, raw StepRAW, index int, hash string) (string, string, []StepAssertion, error) {
	verdict, reason, assertions, err := d.evaluateModelObservationLocal(contract, raw, index, hash)
	if err != nil || verdict != "PASS" || contract.Model.BusinessContract == nil || index <= 1 {
		return verdict, reason, assertions, err
	}
	policy, err := d.precedingBusinessSuccessors(contract, raw.Run, index)
	if err == nil {
		runner, runnerErr := d.registeredRunner(contract.Receipt.Runner)
		if runnerErr != nil {
			err = runnerErr
		} else {
			err = checkBusinessSuccessors(policy, runnerBatches(runner.Manifest)[index-1])
		}
	}
	if err != nil {
		reason = "ordered business successor evidence: " + err.Error()
		assertions = append(assertions, StepAssertion{Kind: "business_successor", Name: "next_allowed", Verdict: "BLOCKED", Reason: reason})
		return "BLOCKED", reason, assertions, nil
	}
	return verdict, reason, assertions, nil
}

func (d db) evaluateModelObservationLocal(contract runContract, raw StepRAW, index int, hash string) (string, string, []StepAssertion, error) {
	assertions := []StepAssertion{}
	if contract.Model == nil || contract.Obligation == nil || contract.Receipt == nil || raw.Model == nil {
		return "BLOCKED", "model observation has no frozen execution context", assertions, errors.New("model observation has no frozen execution context")
	}
	if err := d.modelCaptureExists(raw.Run, index, hash); err != nil {
		return "BLOCKED", err.Error(), assertions, err
	}
	receipt := contract.Receipt
	observed := raw.Model
	if observed.Round != receipt.Round || observed.RunnerSHA256 != receipt.RunnerSHA256 || observed.CodeSHA256 != receipt.CodeSHA256 || !reflect.DeepEqual(observed.Participants, receipt.Participants) {
		return "BLOCKED", "model RAW round, runner or participants mismatch", assertions, errors.New("model RAW round, runner or participants mismatch")
	}
	runner, err := d.registeredRunner(receipt.Runner)
	if err != nil {
		return "BLOCKED", err.Error(), assertions, err
	}
	batches := runnerBatches(runner.Manifest)
	if index < 1 || index > len(batches) {
		return "BLOCKED", "model step index is outside the registered execution", assertions, errors.New("model step index is outside the registered execution")
	}
	batch := batches[index-1]
	final := batch[len(batch)-1]
	x := indexModel(*contract.Model)
	definition := contract.Obligation.Definition
	judged := stepRule(x, *contract.Obligation, final.Transition)
	participantConditions, denied := map[string]bool(definition.Preconditions), definition.Denied
	if definition.Kind == testSequence {
		participantConditions = x.contracts[final.Transition].Preconditions
		denied = x.contracts[final.Transition].Outcome == branchRejection
	}
	if observed.Node != final.Node || observed.Transition != final.Transition {
		return "BLOCKED", "model node or transition differs from its registered operation", assertions, errors.New("model node or transition differs from its registered operation")
	}
	if raw.ExitCode != 0 {
		reason := observed.Error
		if reason == "" {
			reason = fmt.Sprintf("step runner exited with code %d", raw.ExitCode)
		}
		for _, entry := range observed.BusinessExpectations {
			assertions = append(assertions, StepAssertion{Kind: "business_precondition", Name: entry.Decision.Action, Subject: entry.Actor, Verdict: "BLOCKED", Reason: entry.Decision.Status + ": " + entry.Decision.Reason})
		}
		return "BLOCKED", reason, assertions, nil
	}
	if len(observed.Actions) != len(batch) {
		return "BLOCKED", "actual operation count differs from the registered batch", assertions, errors.New("actual operation count differs from the registered batch")
	}
	if contract.Model.BusinessContract != nil {
		if err := d.verifyBusinessExpectationCaptures(contract.Graph, raw.Run, index, observed.BusinessExpectations); err != nil {
			return "BLOCKED", err.Error(), []StepAssertion{{Kind: "business_expectation", Name: "pre_action_receipt", Verdict: "BLOCKED", Reason: err.Error()}}, nil
		}
	}
	participants := map[string]ParticipantBinding{}
	for _, participant := range receipt.Participants {
		participants[participant.Alias] = participant
	}
	if err := validateRuntimeOracleScope(batch, observed, participants, runner.Manifest.IsolationScope); err != nil {
		return "BLOCKED", err.Error(), assertions, err
	}
	for position, action := range observed.Actions {
		step := batch[position]
		stepJudged := stepRule(x, *contract.Obligation, step.Transition)
		stepConds := participantConditions
		if definition.Kind == testSequence && step.Transition != "" {
			stepConds = x.contracts[step.Transition].Preconditions
		}
		participant, found := participants[step.Actor]
		if observed.EvidenceVersion >= runtimeEvidenceVersion && action.ActionIndex != position+1 {
			return "BLOCKED", "actual action index differs from its frozen batch position", assertions, errors.New("actual action index differs from its frozen batch position")
		}
		if !found || action.Node != step.Node || action.Transition != step.Transition || action.Actor != step.Actor || action.Principal != participant.Principal || action.Started == "" || action.Completed == "" || len(action.RequestSHA256) != 64 || len(action.ResponseSHA256) != 64 {
			return "BLOCKED", "actual action identity or capture is incomplete", assertions, errors.New("actual action identity or capture is incomplete")
		}
		templateBytes, err := json.Marshal(step.Call)
		if err != nil {
			return "BLOCKED", "frozen adapter cannot be marshaled", assertions, err
		}
		templateDigest := digestBytes(templateBytes)
		if action.TemplateSHA256 != "" {
			if action.TemplateSHA256 != templateDigest {
				return "BLOCKED", "actual action template differs from its frozen adapter", assertions, errors.New("actual action template differs from its frozen adapter")
			}
			renderedCall, err := applySubstitutionsToCall(step.Call, action.Substitutions)
			if err != nil {
				return "BLOCKED", "actual action substitutions invalid: " + err.Error(), assertions, fmt.Errorf("actual action substitutions invalid: %w", err)
			}
			renderedBytes, err := json.Marshal(renderedCall)
			if err != nil || digestBytes(renderedBytes) != action.RequestSHA256 {
				return "BLOCKED", "actual rendered action input differs from its frozen adapter", assertions, errors.New("actual rendered action input differs from its frozen adapter")
			}
			if action.ResolvedPath != renderedCall.Path || !sameJSONValue(action.ResolvedInput, renderedCall.Body) {
				return "BLOCKED", "resolved action display differs from the verified request", assertions, errors.New("resolved action display differs from the verified request")
			}
		} else if templateDigest != action.RequestSHA256 {
			return "BLOCKED", "actual action input differs from its frozen adapter", assertions, errors.New("actual action input differs from its frozen adapter")
		}
		if step.Call.Kind == adapterHTTP {
			known := false
			actionDenied := false
			for _, status := range step.Call.SuccessStatuses {
				if status == action.StatusCode {
					known = true
				}
			}
			for _, status := range step.Call.RejectionStatuses {
				if status == action.StatusCode {
					known = true
					actionDenied = true
				}
			}
			if !known || actionDenied != action.Denied {
				return "BLOCKED", "actual action status contradicts the registered transport contract", assertions, errors.New("actual action status contradicts the registered transport contract")
			}
		}
		if observed.EvidenceVersion >= runtimeEvidenceVersion {
			if len(step.InputMeasurements) > 0 {
				request, err := runtimeRequestFor(step, position+1, observed)
				if err != nil || !requestMatchesAction(*request, action) {
					return "BLOCKED", "actual request differs from the pre-action frozen input", assertions, errors.New("actual request differs from the pre-action frozen input")
				}
			}
			if len(action.ResponseValues) != len(step.ResponseMeasurements) {
				return "BLOCKED", "actual response measurements differ from the registered capture fields", assertions, errors.New("actual response measurements differ from the registered capture fields")
			}
			for key := range step.ResponseMeasurements {
				if value, found := action.ResponseValues[key]; !found || !json.Valid(value) {
					return "BLOCKED", "actual response measurement is missing or invalid", assertions, errors.New("actual response measurement is missing or invalid")
				}
			}
		}
		if step.Call.Kind == adapterCommand {
			accepted := false
			for _, code := range step.Call.SuccessExitCodes {
				accepted = accepted || code == action.ExitCode
			}
			if !accepted {
				return "BLOCKED", "actual command exit differs from its frozen adapter", assertions, errors.New("actual command exit differs from its frozen adapter")
			}
		}
		foundOracle := false
		for _, item := range observed.Oracles {
			if oracleForAction(item, step.Actor, position+1, observed.EvidenceVersion) && item.ID == stepJudged.Oracle {
				foundOracle = true
				if item.Before.Principal != participant.Principal || item.After.Principal != participant.Principal || item.Before.Role != participant.Role || item.After.Role != participant.Role || item.Before.Scope != runner.Manifest.IsolationScope || item.After.Scope != runner.Manifest.IsolationScope || !observedConditions(map[string]bool(item.Before.Conditions), stepConds, stepJudged.Conditions) {
					return "BLOCKED", "participant identity, scope or observed preconditions contradict the frozen contract", assertions, errors.New("participant identity, scope or observed preconditions contradict the frozen contract")
				}
			}
		}
		if !foundOracle {
			return "BLOCKED", "participant independent oracle is missing", assertions, errors.New("participant independent oracle is missing")
		}
	}
	var oracle *OracleObservation
	for _, item := range observed.Oracles {
		if item.ID == judged.Oracle && oracleForAction(item, final.Actor, len(batch), observed.EvidenceVersion) {
			copy := item
			oracle = &copy
			break
		}
	}
	if oracle == nil || oracle.Before.Principal != participants[final.Actor].Principal || oracle.After.Principal != participants[final.Actor].Principal {
		return "BLOCKED", "independent rule oracle is missing or belongs to another principal", assertions, errors.New("independent rule oracle is missing or belongs to another principal")
	}
	before, err := factsHash(oracle.Before)
	if err != nil {
		return "BLOCKED", err.Error(), assertions, err
	}
	after, err := factsHash(oracle.After)
	if err != nil {
		return "BLOCKED", err.Error(), assertions, err
	}
	if before != raw.Before || after != raw.After || !reflect.DeepEqual(map[string]bool(oracle.Before.Conditions), raw.Preconditions) {
		return "BLOCKED", "action preconditions or before/after snapshot disagree with the independent oracle", assertions, errors.New("action preconditions or before/after snapshot disagree with the independent oracle")
	}
	expected := definition.Expected
	conditions := map[string]bool(definition.Preconditions)
	if final.Transition != "" {
		for _, transition := range contract.Model.Transitions {
			if transition.Edge == final.Transition {
				conditions = transition.Preconditions
				expected = x.edges[transition.Edge].To
				if transition.Outcome == branchRejection && judged.Rejected == "UNCHANGED" {
					expected = "UNCHANGED"
				}
				if oracle.Before.State != x.edges[transition.Edge].From {
					assertions = append(assertions, StepAssertion{Kind: "precondition_state", Name: final.Transition, OracleID: judged.Oracle, Subject: final.Actor, Expected: x.edges[transition.Edge].From, Actual: oracle.Before.State, Operator: "eq", Verdict: "BLOCKED", Reason: "before-action state differs from transition source"})
					return "BLOCKED", "before-action state differs from transition source", assertions, nil
				}
			}
		}
	}
	if raw.Expected != expected {
		return "BLOCKED", "recorded expectation differs from the frozen step contract", assertions, errors.New("recorded expectation differs from the frozen step contract")
	}
	for _, cond := range judged.Conditions {
		actualVal, found := raw.Preconditions[cond]
		wantVal, hasWant := conditions[cond]
		ok := found && (!hasWant || actualVal == wantVal)
		verdict := "PASS"
		reason := "condition observed as declared"
		if !ok {
			verdict = "BLOCKED"
			reason = "observed condition missing or differs from frozen assignment"
		}
		assertions = append(assertions, StepAssertion{Kind: "condition", Name: cond, OracleID: judged.Oracle, Subject: final.Actor, Expected: fmt.Sprintf("%t", wantVal), Actual: fmt.Sprintf("%t (found=%t)", actualVal, found), Operator: "eq", Verdict: verdict, Reason: reason})
	}
	if !observedConditions(raw.Preconditions, conditions, judged.Conditions) {
		return "BLOCKED", "observed preconditions do not match the frozen contract", assertions, nil
	}
	actual := oracle.After.State
	if expected == "UNCHANGED" && before == after {
		actual = "UNCHANGED"
	}
	if actual != raw.Observed {
		return "BLOCKED", "recorded observed state differs from independent state readback", assertions, errors.New("recorded observed state differs from independent state readback")
	}
	stepVerdict := "PASS"
	stepReason := "independent state, preconditions and properties verified"
	setVerdict := func(v, r string) {
		if v == "BLOCKED" || (v == "FAIL" && stepVerdict != "BLOCKED") {
			stepVerdict, stepReason = v, r
		}
	}
	// Evaluate declared async wait observations on any step in the batch.
	for position, step := range batch {
		if step.AsyncWait == nil {
			continue
		}
		assertion := verifyStepWait(step, index, position+1, observed)
		assertions = append(assertions, assertion)
		if assertion.Verdict != "PASS" {
			setVerdict(assertion.Verdict, assertion.Reason)
		}
	}
	// Primary state expectation assertion.
	if contract.Model.BusinessContract != nil {
		businessVerdict, businessReason, businessAssertions := verifyRuntimeBusiness(*contract.Model, *contract.Obligation, batch, observed)
		assertions = append(assertions, businessAssertions...)
		if businessVerdict != "PASS" {
			setVerdict(businessVerdict, businessReason)
		}
	} else if raw.Observed != expected {
		assertions = append(assertions, StepAssertion{Kind: "state", Name: final.Node, OracleID: judged.Oracle, Subject: final.Actor, Expected: expected, Actual: raw.Observed, Operator: "eq", Verdict: "FAIL", Reason: fmt.Sprintf("observed state %s differs from expected %s", raw.Observed, expected)})
		setVerdict("FAIL", fmt.Sprintf("observed state %s differs from expected %s", raw.Observed, expected))
	} else {
		assertions = append(assertions, StepAssertion{Kind: "state", Name: final.Node, OracleID: judged.Oracle, Subject: final.Actor, Expected: expected, Actual: raw.Observed, Operator: "eq", Verdict: "PASS", Reason: "observed state matches expected"})
	}
	// Denial assertion.
	if len(batch) == 1 && contract.Model.BusinessContract == nil {
		if raw.Denied != observed.Actions[0].Denied || denied != raw.Denied {
			assertions = append(assertions, StepAssertion{Kind: "denial", Name: final.Node, OracleID: judged.Oracle, Subject: final.Actor, Expected: fmt.Sprint(denied), Actual: fmt.Sprint(raw.Denied), Operator: "eq", Verdict: "FAIL", Reason: fmt.Sprintf("action denied=%t contradicted expected denied=%t", raw.Denied, denied)})
			setVerdict("FAIL", fmt.Sprintf("action denied=%t contradicted expected denied=%t", raw.Denied, denied))
		} else {
			assertions = append(assertions, StepAssertion{Kind: "denial", Name: final.Node, OracleID: judged.Oracle, Subject: final.Actor, Expected: fmt.Sprint(denied), Actual: fmt.Sprint(raw.Denied), Operator: "eq", Verdict: "PASS", Reason: "denial observation matches contract"})
		}
	}
	if contract.Model.BusinessContract == nil && definition.Denied && expected == "UNCHANGED" && before != after {
		assertions = append(assertions, StepAssertion{Kind: "forbidden_effect", Name: final.Node, OracleID: judged.Oracle, Subject: final.Actor, Expected: before, Actual: after, Operator: "eq", Verdict: "FAIL", Reason: "rejected action mutated independent state snapshot"})
		setVerdict("FAIL", "rejected action mutated independent state snapshot")
	}
	if definition.Kind == testConsistency && index == len(batches) {
		layerV, layerAssertions, err := evaluateLayersWithAssertions(definition.Layers, observed.Oracles, final.Actor, participants[final.Actor].Principal, expected, len(batch), observed.EvidenceVersion)
		assertions = append(assertions, layerAssertions...)
		if layerV != "PASS" {
			reason := "multi-layer consistency check failed"
			if err != nil {
				reason = err.Error()
			}
			setVerdict(layerV, reason)
		}
	}
	// Step-level property checks.
	for position, step := range batch {
		if len(step.Checks) == 0 {
			continue
		}
		mergedValues := map[string]json.RawMessage{}
		for _, item := range observed.Oracles {
			if oracleForAction(item, step.Actor, position+1, observed.EvidenceVersion) {
				for k, v := range item.After.Values {
					if prior, exists := mergedValues[k]; exists && !sameJSONValue(prior, v) {
						assertions = append(assertions, StepAssertion{Kind: "property", Name: k, OracleID: item.ID, Subject: step.Actor, Actual: string(v), Verdict: "BLOCKED", Reason: "independent oracles disagree on a property measurement"})
						setVerdict("BLOCKED", "independent oracles disagree on a property measurement")
						continue
					}
					mergedValues[k] = v
				}
			}
		}
		for _, outcome := range evaluateProperties(step.Checks, mergedValues) {
			check := step.Checks[outcome.Index-1]
			assertions = append(assertions, StepAssertion{Kind: "property", Name: check.Left, Subject: step.Actor, Expected: string(outcome.Right), Actual: string(outcome.Left), Operator: string(check.Operator), Verdict: outcome.Verdict, Reason: outcome.Reason})
			if outcome.Verdict != "PASS" {
				setVerdict(outcome.Verdict, outcome.Reason)
			}
		}
	}
	// Each operation retains its own rule and reader, including nonfinal actors
	// and repeated actor/oracle pairs inside the same concurrent batch.
	for position, step := range batch {
		stepJudged := stepRule(x, *contract.Obligation, step.Transition)
		for _, invariant := range x.invariants[stepJudged.ID] {
			found := false
			for _, item := range observed.Oracles {
				if item.ID == invariant.Oracle && item.Boundary == invariant.ObservationBoundary && oracleForAction(item, step.Actor, position+1, observed.EvidenceVersion) {
					found = true
					for _, outcome := range evaluateProperties(invariant.Checks, item.After.Values) {
						check := invariant.Checks[outcome.Index-1]
						assertions = append(assertions, StepAssertion{Kind: "invariant", Name: invariant.ID + "." + check.Left, OracleID: invariant.Oracle, Subject: item.Actor, Expected: string(outcome.Right), Actual: string(outcome.Left), Operator: string(check.Operator), Verdict: outcome.Verdict, Reason: outcome.Reason})
						if outcome.Verdict != "PASS" {
							setVerdict(outcome.Verdict, outcome.Reason)
						}
					}
				}
			}
			if !found {
				assertions = append(assertions, StepAssertion{Kind: "invariant", Name: invariant.ID, OracleID: invariant.Oracle, Subject: step.Actor, Verdict: "BLOCKED", Reason: "invariant independent oracle observation missing"})
				setVerdict("BLOCKED", "invariant independent oracle observation missing")
			}
		}
	}
	return stepVerdict, stepReason, assertions, nil
}

func evaluateLayersWithAssertions(layers []ObservationLayer, oracles []OracleObservation, actor, principal, expected string, actionIndex, evidenceVersion int) (string, []StepAssertion, error) {
	assertions := make([]StepAssertion, 0, len(layers))
	verdict := "PASS"
	var firstErr error
	for _, layer := range layers {
		var item *OracleObservation
		for position := range oracles {
			if oracles[position].ID == layer.Oracle && oracleForAction(oracles[position], actor, actionIndex, evidenceVersion) {
				if item != nil {
					err := fmt.Errorf("layer %s has duplicate independent observations for the same actor and action", layer.Layer)
					assertions = append(assertions, StepAssertion{Kind: "layer", Name: layer.Layer, OracleID: layer.Oracle, Subject: actor, Expected: expected, Actual: "AMBIGUOUS", Operator: "eq", Verdict: "BLOCKED", Reason: err.Error()})
					return "BLOCKED", assertions, err
				}
				item = &oracles[position]
			}
		}
		if item == nil {
			err := fmt.Errorf("layer %s has no independent observation", layer.Layer)
			assertions = append(assertions, StepAssertion{Kind: "layer", Name: layer.Layer, OracleID: layer.Oracle, Subject: actor, Expected: expected, Actual: "UNOBSERVED", Operator: "eq", Verdict: "BLOCKED", Reason: err.Error()})
			return "BLOCKED", assertions, err
		}
		if item.Before.Principal != principal || item.After.Principal != principal {
			err := fmt.Errorf("layer %s observed another principal", layer.Layer)
			assertions = append(assertions, StepAssertion{Kind: "layer", Name: layer.Layer, OracleID: layer.Oracle, Subject: actor, Expected: principal, Actual: item.After.Principal, Operator: "eq", Verdict: "BLOCKED", Reason: err.Error()})
			return "BLOCKED", assertions, err
		}
		before, err := factsHash(item.Before)
		if err != nil {
			return "BLOCKED", assertions, err
		}
		after, err := factsHash(item.After)
		if err != nil {
			return "BLOCKED", assertions, err
		}
		actual := item.After.State
		if expected == "UNCHANGED" && before == after {
			actual = "UNCHANGED"
		}
		if actual != expected {
			assertions = append(assertions, StepAssertion{Kind: "layer", Name: layer.Layer, OracleID: layer.Oracle, Subject: actor, Expected: expected, Actual: actual, Operator: "eq", Verdict: "FAIL", Reason: fmt.Sprintf("layer %s observed %s, expected %s", layer.Layer, actual, expected)})
			if verdict == "PASS" {
				verdict = "FAIL"
			}
		} else {
			assertions = append(assertions, StepAssertion{Kind: "layer", Name: layer.Layer, OracleID: layer.Oracle, Subject: actor, Expected: expected, Actual: actual, Operator: "eq", Verdict: "PASS", Reason: "layer observation agrees"})
		}
	}
	return verdict, assertions, firstErr
}

// layerVerdict compares every declared observation layer with the frozen
// expectation: a layer without its own observation blocks, a disagreeing
// layer fails, and agreement across all layers passes.
func layerVerdict(layers []ObservationLayer, oracles []OracleObservation, actor, principal, expected string) (string, error) {
	verdict, _, err := evaluateLayersWithAssertions(layers, oracles, actor, principal, expected, 0, 0)
	return verdict, err
}

func (d db) validateModelRecovery(contract runContract) error {
	recovery := contract.Receipt.Recovery
	if recovery.Verdict != "PASS" || !recovery.Safe || recovery.Evidence == "" || len(recovery.SHA256) != 64 {
		return errors.New("external fixture recovery has no valid independent proof")
	}
	runner, err := d.registeredRunner(contract.Receipt.Runner)
	if err != nil {
		return err
	}
	fixture, _, err := d.bindingFixture(contract.Binding.Identity.Fixture)
	if err != nil {
		return err
	}
	protocol := runner.Manifest.Recovery
	mode, expected := fixtureRecoveryMode(protocol), fixtureRecoveryTarget(protocol, fixture)
	if protocol != nil && (mode == recoveryRestore && protocol.State != fixture.State || protocol.IsolationScope != fixture.Refs["scope"]) {
		return errors.New("recovery target or isolation differs from the registered fixture")
	}
	if mode != recoveryRestore && mode != recoveryCheckpoint || recovery.Mode != "" && recovery.Mode != mode || mode == recoveryCheckpoint && (recovery.Mode != mode || len(protocol.Steps) != 0) {
		return errors.New("fixture finalization mode differs from its frozen protocol")
	}
	raw, _, err := d.readRecordedRAWWithFallback(recovery.Evidence, recovery.SHA256, contract.Receipt.Run, expected, expected, 0)
	if err != nil {
		return err
	}
	if err := contract.Binding.matchesRAW(raw); err != nil {
		return err
	}
	if err := d.modelCaptureExists(raw.Run, 0, recovery.SHA256); err != nil {
		return err
	}
	if raw.ExitCode != 0 || !raw.SafeToContinue || raw.Model == nil || raw.Model.Round != contract.Receipt.Round || raw.Model.RunnerSHA256 != contract.Receipt.RunnerSHA256 || raw.Model.CodeSHA256 != contract.Receipt.CodeSHA256 || !reflect.DeepEqual(raw.Model.Participants, contract.Receipt.Participants) {
		return errors.New("recovery observer is incomplete or unsafe")
	}
	if raw.Model.EvidenceVersion >= runtimeEvidenceVersion || mode == recoveryCheckpoint {
		node := "fixture.restore"
		if mode == recoveryCheckpoint {
			node = "fixture.checkpoint"
		}
		if raw.Model.EvidenceVersion < runtimeEvidenceVersion || recovery.Mode != mode || raw.Model.RecoveryMode != mode || raw.Model.Node != node || recovery.State != expected || !sameFactSlice(raw.Model.Actions, recovery.Actions) {
			return errors.New("fixture finalization identity, mode or action capture is incomplete")
		}
	}
	if protocol == nil {
		if !contract.Obligation.Definition.ReadOnly || len(raw.Model.Oracles) != len(contract.Receipt.Participants) || len(raw.Model.Actions) != 0 {
			return errors.New("mutating test cannot reuse a read-only checkpoint")
		}
		for position, participant := range contract.Receipt.Participants {
			item := raw.Model.Oracles[position]
			before, err := factsHash(item.Before)
			if err != nil {
				return err
			}
			after, err := factsHash(item.After)
			if err != nil || before != after || item.Actor != participant.Alias || item.After.Principal != participant.Principal || item.After.Role != participant.Role || item.After.Scope != fixture.Refs["scope"] || item.After.State != fixture.State {
				return errors.New("read-only checkpoint did not preserve each participant's independent fixture snapshot")
			}
		}
		return nil
	}
	if len(raw.Model.Oracles) != 1 || len(raw.Model.Actions) != len(protocol.Steps) {
		return errors.New("recovery operations or oracle are incomplete")
	}
	facts := raw.Model.Oracles[0].After
	if raw.Model.Oracles[0].Actor != "primary" || raw.Model.Oracles[0].ID != protocol.Oracle.ID || raw.Model.Oracles[0].Boundary != protocol.Oracle.Boundary || facts.State != expected || facts.Principal != contract.Binding.Identity.Principal || facts.Role != contract.Binding.Identity.Actor || facts.Scope != fixture.Refs["scope"] || !observedConditions(facts.Conditions, protocol.Conditions, nil) {
		return errors.New("fixture state, identity or conditions do not satisfy the independent " + mode + " proof")
	}
	for _, outcome := range evaluateProperties(protocol.Checks, facts.Values) {
		if outcome.Verdict != "PASS" {
			return errors.New("fixture " + mode + " property failed: " + outcome.Reason)
		}
	}
	return nil
}

func (d db) validateModelCompletion(contract runContract) error {
	raw, _, err := d.readRecordedRAWWithFallback(contract.Receipt.SetupEvidence, contract.Receipt.SetupSHA256, contract.Receipt.Run, "ISOLATED", "ISOLATED", -1)
	if err != nil {
		return fmt.Errorf("fixture preparation capture: %w", err)
	}
	if err := contract.Binding.matchesRAW(raw); err != nil {
		return err
	}
	if err := d.modelCaptureExists(raw.Run, -1, contract.Receipt.SetupSHA256); err != nil {
		return err
	}
	if raw.ExitCode != 0 || raw.Model == nil || raw.Model.Round != contract.Receipt.Round || !reflect.DeepEqual(raw.Model.Participants, contract.Receipt.Participants) || len(raw.Model.Oracles) != len(contract.Receipt.Participants) {
		return errors.New("fixture preparation has no complete isolated participant observation")
	}
	if err := d.validateModelRecovery(contract); err != nil {
		return err
	}
	runner, err := d.registeredRunner(contract.Receipt.Runner)
	if err != nil {
		return err
	}
	if runner.Manifest.Preparation != nil {
		if raw.Model.Preparation == nil {
			return errors.New("automatic preparation has no captured execution and readback")
		}
		preparation := runner.Manifest.Preparation
		request := PreparationRequest{Goal: preparation.Goal, MaxSteps: preparation.MaxSteps, MaxStates: preparation.MaxStates}
		if err := ValidatePreparationExecution(*contract.Model, request, *raw.Model.Preparation); err != nil {
			return fmt.Errorf("automatic preparation evidence: %w", err)
		}
		if err := validateRuntimePreparationIdentityEvidence(*contract.Model, runner.Manifest, raw.Model); err != nil {
			return fmt.Errorf("automatic preparation subject evidence: %w", err)
		}
		if len(raw.Model.Actions) != len(runner.Manifest.Setup)+len(raw.Model.Preparation.Steps) {
			return errors.New("automatic preparation action captures are incomplete")
		}
		for i, actual := range raw.Model.Preparation.Steps {
			adapter, found := preparation.Adapters[actual.Planned.ID]
			if !found || adapter.Node != actual.Planned.Action {
				return errors.New("automatic preparation used an unregistered adapter")
			}
			action := raw.Model.Actions[len(runner.Manifest.Setup)+i]
			if !sameFactSlice([]ActionObservation{action}, []ActionObservation{actual.Receipt.Action}) || action.Actor != adapter.Actor {
				return errors.New("automatic preparation receipt differs from the captured operation")
			}
			template, _ := json.Marshal(adapter.Call)
			rendered, renderErr := applySubstitutionsToCall(adapter.Call, action.Substitutions)
			input, marshalErr := json.Marshal(rendered)
			if renderErr != nil || marshalErr != nil || action.TemplateSHA256 != digestBytes(template) || action.RequestSHA256 != digestBytes(input) || action.ResolvedPath != rendered.Path || !sameJSONValue(action.ResolvedInput, rendered.Body) {
				return errors.New("automatic preparation request differs from its frozen adapter")
			}
		}
	} else if raw.Model.Preparation != nil {
		return errors.New("setup claims automatic preparation absent from the registered manifest")
	}
	rows, err := d.query("SELECT COUNT(*) FROM steps WHERE run="+quote(contract.Receipt.Run), 1)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0][0] != fmt.Sprint(len(runnerBatches(runner.Manifest))) {
		return errors.New("registered execution stopped before all required operations")
	}
	return nil
}

func (d db) qualifyModelRun(run string) error {
	contract, err := d.runTaskContract(run)
	if err != nil {
		return err
	}
	if contract.Model == nil || !contract.Receipt.Completed || contract.Status != "PASS" {
		return errors.New("no completed model PASS run")
	}
	if err := d.validateBinding(contract.Binding, nil); err != nil {
		return err
	}
	if err := d.validateModelCompletion(contract); err != nil {
		return err
	}
	rows, err := d.query("SELECT idx,expected,observed,verdict,evidence_path,evidence_sha256 FROM steps WHERE run="+quote(run)+" ORDER BY idx", 6)
	if err != nil {
		return err
	}
	for index, row := range rows {
		if row[0] != fmt.Sprint(index+1) || row[3] != "PASS" {
			return errors.New("model run contains a failed or missing step")
		}
		raw, _, err := d.readRecordedRAWWithFallback(row[4], row[5], run, row[1], row[2], index+1)
		if err != nil {
			return err
		}
		if err := contract.Binding.matchesRAW(raw); err != nil {
			return err
		}
		verdict, err := d.modelObservationVerdict(contract, raw, index+1, row[5])
		if err != nil {
			return err
		}
		if verdict != "PASS" {
			return errors.New("model RAW no longer passes its independent checks")
		}
	}
	return nil
}

func (d db) validateModelDependencies(g Graph, obligation ModelObligation, round string) error {
	for _, dependency := range obligation.Definition.DependsOn {
		rows, err := d.query("SELECT r.id,r.status FROM runs r JOIN model_runs m ON m.run=r.id WHERE r.task="+quote(g.ID+"/"+dependency)+" AND m.round="+quote(round)+" ORDER BY r.rowid DESC LIMIT 1", 2)
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0][1] != "PASS" {
			return fmt.Errorf("test dependency %s has no PASS in this round", dependency)
		}
		if err := d.qualifyModelRun(rows[0][0]); err != nil {
			return fmt.Errorf("test dependency %s: %w", dependency, err)
		}
	}
	return nil
}
