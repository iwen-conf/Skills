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
	if contract.Model == nil || contract.Obligation == nil || contract.Receipt == nil || raw.Model == nil {
		return "BLOCKED", errors.New("model observation has no frozen execution context")
	}
	if err := d.modelCaptureExists(raw.Run, index, hash); err != nil {
		return "BLOCKED", err
	}
	receipt := contract.Receipt
	observed := raw.Model
	if observed.Round != receipt.Round || observed.RunnerSHA256 != receipt.RunnerSHA256 || observed.CodeSHA256 != receipt.CodeSHA256 || !reflect.DeepEqual(observed.Participants, receipt.Participants) {
		return "BLOCKED", errors.New("model RAW round, runner or participants mismatch")
	}
	runner, err := d.registeredRunner(receipt.Runner)
	if err != nil {
		return "BLOCKED", err
	}
	batches := runnerBatches(runner.Manifest)
	if index < 1 || index > len(batches) {
		return "BLOCKED", errors.New("model step index is outside the registered execution")
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
		return "BLOCKED", errors.New("model node or transition differs from its registered operation")
	}
	if raw.ExitCode != 0 {
		return "BLOCKED", nil
	}
	if len(observed.Actions) != len(batch) {
		return "BLOCKED", errors.New("actual operation count differs from the registered batch")
	}
	participants := map[string]ParticipantBinding{}
	for _, participant := range receipt.Participants {
		participants[participant.Alias] = participant
	}
	for position, action := range observed.Actions {
		step := batch[position]
		participant, found := participants[step.Actor]
		if !found || action.Node != step.Node || action.Transition != step.Transition || action.Actor != step.Actor || action.Principal != participant.Principal || action.Started == "" || action.Completed == "" || len(action.RequestSHA256) != 64 || len(action.ResponseSHA256) != 64 {
			return "BLOCKED", errors.New("actual action identity or capture is incomplete")
		}
		input, err := json.Marshal(step.Call)
		if err != nil || digestBytes(input) != action.RequestSHA256 {
			return "BLOCKED", errors.New("actual action input differs from its frozen adapter")
		}
		if step.Call.Kind == adapterHTTP {
			known := false
			denied := false
			for _, status := range step.Call.SuccessStatuses {
				if status == action.StatusCode {
					known = true
				}
			}
			for _, status := range step.Call.RejectionStatuses {
				if status == action.StatusCode {
					known = true
					denied = true
				}
			}
			if !known || denied != action.Denied {
				return "BLOCKED", errors.New("actual action status contradicts the registered transport contract")
			}
		}
		if step.Call.Kind == adapterCommand {
			accepted := false
			for _, code := range step.Call.SuccessExitCodes {
				accepted = accepted || code == action.ExitCode
			}
			if !accepted {
				return "BLOCKED", errors.New("actual command exit differs from its frozen adapter")
			}
		}
		foundOracle := false
		for _, item := range observed.Oracles {
			if item.Actor == step.Actor && item.ID == judged.Oracle {
				foundOracle = true
				if item.Before.Principal != participant.Principal || item.After.Principal != participant.Principal || item.Before.Role != participant.Role || item.After.Role != participant.Role || item.Before.Scope != runner.Manifest.IsolationScope || item.After.Scope != runner.Manifest.IsolationScope || !observedConditions(map[string]bool(item.Before.Conditions), participantConditions, judged.Conditions) {
					return "BLOCKED", errors.New("participant identity, scope or observed preconditions contradict the frozen contract")
				}
			}
		}
		if !foundOracle {
			return "BLOCKED", errors.New("participant independent oracle is missing")
		}
	}
	var oracle *OracleObservation
	for _, item := range observed.Oracles {
		if item.ID == judged.Oracle && item.Actor == final.Actor {
			copy := item
			oracle = &copy
			break
		}
	}
	if oracle == nil || oracle.Before.Principal != participants[final.Actor].Principal || oracle.After.Principal != participants[final.Actor].Principal {
		return "BLOCKED", errors.New("independent rule oracle is missing or belongs to another principal")
	}
	before, err := factsHash(oracle.Before)
	if err != nil {
		return "BLOCKED", err
	}
	after, err := factsHash(oracle.After)
	if err != nil {
		return "BLOCKED", err
	}
	if before != raw.Before || after != raw.After || !reflect.DeepEqual(map[string]bool(oracle.Before.Conditions), raw.Preconditions) {
		return "BLOCKED", errors.New("action preconditions or before/after snapshot disagree with the independent oracle")
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
					return "BLOCKED", nil
				}
			}
		}
	}
	if raw.Expected != expected {
		return "BLOCKED", errors.New("recorded expectation differs from the frozen step contract")
	}
	if !observedConditions(raw.Preconditions, conditions, judged.Conditions) {
		return "BLOCKED", nil
	}
	actual := oracle.After.State
	if expected == "UNCHANGED" && before == after {
		actual = "UNCHANGED"
	}
	if actual != raw.Observed {
		return "BLOCKED", errors.New("recorded observed state differs from independent state readback")
	}
	if raw.Observed != expected {
		return "FAIL", nil
	}
	if len(batch) == 1 && (raw.Denied != observed.Actions[0].Denied || denied != raw.Denied) {
		return "FAIL", nil
	}
	if definition.Denied && expected == "UNCHANGED" && before != after {
		return "FAIL", nil
	}
	if definition.Kind == testConsistency && index == len(batches) {
		if verdict, err := layerVerdict(definition.Layers, observed.Oracles, final.Actor, participants[final.Actor].Principal, expected); verdict != "PASS" {
			return verdict, err
		}
	}
	for _, invariant := range x.invariants[judged.ID] {
		found := false
		for _, item := range observed.Oracles {
			if item.ID == invariant.Oracle && item.Boundary == invariant.ObservationBoundary {
				found = true
				for _, outcome := range evaluateProperties(invariant.Checks, item.After.Values) {
					if outcome.Verdict != "PASS" {
						return outcome.Verdict, nil
					}
				}
			}
		}
		if !found {
			return "BLOCKED", nil
		}
	}
	return "PASS", nil
}

// layerVerdict compares every declared observation layer with the frozen
// expectation: a layer without its own observation blocks, a disagreeing
// layer fails, and agreement across all layers passes.
func layerVerdict(layers []ObservationLayer, oracles []OracleObservation, actor, principal, expected string) (string, error) {
	for _, layer := range layers {
		var item *OracleObservation
		for position := range oracles {
			if oracles[position].ID == layer.Oracle && oracles[position].Actor == actor {
				item = &oracles[position]
			}
		}
		if item == nil {
			return "BLOCKED", fmt.Errorf("layer %s has no independent observation", layer.Layer)
		}
		if item.Before.Principal != principal || item.After.Principal != principal {
			return "BLOCKED", fmt.Errorf("layer %s observed another principal", layer.Layer)
		}
		before, err := factsHash(item.Before)
		if err != nil {
			return "BLOCKED", err
		}
		after, err := factsHash(item.After)
		if err != nil {
			return "BLOCKED", err
		}
		actual := item.After.State
		if expected == "UNCHANGED" && before == after {
			actual = "UNCHANGED"
		}
		if actual != expected {
			return "FAIL", nil
		}
	}
	return "PASS", nil
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
	if protocol != nil && (protocol.State != fixture.State || protocol.IsolationScope != fixture.Refs["scope"]) {
		return errors.New("recovery target or isolation differs from the registered fixture")
	}
	raw, err := readRecordedRAW(recovery.Evidence, recovery.SHA256, contract.Receipt.Run, fixture.State, fixture.State, 0)
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
	if raw.Model.Oracles[0].ID != protocol.Oracle.ID || raw.Model.Oracles[0].Boundary != protocol.Oracle.Boundary || facts.State != fixture.State || facts.Principal != contract.Binding.Identity.Principal || facts.Role != contract.Binding.Identity.Actor || facts.Scope != fixture.Refs["scope"] || !observedConditions(facts.Conditions, protocol.Conditions, nil) {
		return errors.New("fixture state or conditions were not independently restored")
	}
	for _, outcome := range evaluateProperties(protocol.Checks, facts.Values) {
		if outcome.Verdict != "PASS" {
			return errors.New("fixture restoration property failed: " + outcome.Reason)
		}
	}
	return nil
}

func (d db) validateModelCompletion(contract runContract) error {
	raw, err := readRecordedRAW(contract.Receipt.SetupEvidence, contract.Receipt.SetupSHA256, contract.Receipt.Run, "ISOLATED", "ISOLATED", -1)
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
		raw, err := readRecordedRAW(row[4], row[5], run, row[1], row[2], index+1)
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
