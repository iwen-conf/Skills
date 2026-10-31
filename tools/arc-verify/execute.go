package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ExecuteRequest struct {
	Task        string            `json:"task"`
	Runner      string            `json:"runner"`
	Round       string            `json:"round"`
	Environment string            `json:"environment"`
	Commit      string            `json:"commit"`
	Root        string            `json:"root"`
	Config      string            `json:"config"`
	Account     string            `json:"account"`
	Fixture     string            `json:"fixture"`
	Accounts    map[string]string `json:"accounts,omitempty"`
}

type ExecuteResult struct {
	Run      string          `json:"run"`
	Round    string          `json:"round"`
	Verdict  string          `json:"verdict"`
	Reason   string          `json:"reason"`
	Steps    int             `json:"steps"`
	Recovery RecoveryOutcome `json:"recovery"`
}

func writeRunnerRAW(d db, run string, index int, value StepRAW) (string, []byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", nil, err
	}
	var parsed any
	if err := decodeUniqueJSON(body, &parsed); err != nil {
		return "", nil, err
	}
	if hasSensitiveKey(parsed) {
		return "", nil, errors.New("captured evidence contains credential-like data")
	}
	directory := filepath.Join(filepath.Dir(d.file), "raw", run)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", nil, err
	}
	path := filepath.Join(directory, fmt.Sprintf("%03d-%s.json", index, newID("capture")))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", nil, err
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		return "", nil, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		return "", nil, err
	}
	if err := d.captureModelRAW(run, index, body); err != nil {
		return "", nil, err
	}
	return path, body, nil
}

func actionRecord(step RunnerStep, participant ParticipantBinding, output AdapterOutput) ActionObservation {
	return ActionObservation{Node: step.Node, Transition: step.Transition, Actor: step.Actor, Principal: participant.Principal, RequestSHA256: output.RequestSHA256, ResponseSHA256: output.ResponseSHA256, StatusCode: output.StatusCode, ExitCode: output.ExitCode, Denied: output.Denied, Started: output.Started, Completed: output.Completed}
}

func (d db) execute(parent context.Context, request ExecuteRequest) (ExecuteResult, error) {
	result := ExecuteResult{Round: request.Round, Verdict: "BLOCKED", Recovery: RecoveryOutcome{Verdict: "NOT_RUN", Reason: "execution has not started", Actions: []ActionObservation{}}}
	if request.Task == "" || request.Runner == "" || request.Round == "" || request.Environment == "" || request.Commit == "" || request.Account == "" || request.Fixture == "" || request.Root == "" || request.Config == "" {
		return result, errors.New("execute needs task, registered runner, round, environment, candidate, account and fixture")
	}
	runner, err := d.registeredRunner(request.Runner)
	if err != nil {
		return result, err
	}
	model, err := d.model(runner.Manifest.Graph)
	if err != nil {
		return result, err
	}
	obligation, err := validateManifest(model, runner.Manifest)
	if err != nil {
		return result, err
	}
	if obligation.Task.ID != request.Task {
		return result, errors.New("runner belongs to another frozen test")
	}
	binding, err := d.startBinding(request.Task, request.Environment, request.Commit, request.Account, request.Fixture, request.Root, request.Config)
	if err != nil {
		return result, err
	}
	participants, err := d.participantBindings(obligation.Definition, binding, request.Accounts)
	if err != nil {
		return result, err
	}
	fixture, _, err := d.bindingFixture(binding.Identity.Fixture)
	if err != nil {
		return result, err
	}
	if fixture.Refs["scope"] != runner.Manifest.IsolationScope || runner.Manifest.Recovery != nil && runner.Manifest.Recovery.State != fixture.State {
		return result, errors.New("runner isolation or recovery target does not match the registered fixture")
	}
	record := ModelRunRecord{Round: request.Round, Runner: request.Runner, RunnerSHA256: runner.SHA256, CodeSHA256: runner.CodeSHA256, Participants: participants, Recovery: result.Recovery}
	accounts := map[string]Fixture{}
	actors := map[string]ParticipantBinding{}
	for _, participant := range participants {
		account, _, err := d.bindingFixture(participant.Account)
		if err != nil {
			return result, err
		}
		accounts[participant.Alias] = account
		actors[participant.Alias] = participant
	}
	run, err := d.start(request.Task, binding, record)
	if err != nil {
		return result, err
	}
	result.Run = run
	record.Run = run
	manifest := runner.Manifest
	total := time.Duration(manifest.TimeoutMS) * time.Millisecond
	deadline := time.Now().Add(total)
	actionCtx, cancelAction := context.WithTimeout(parent, total*2/3)
	defer cancelAction()
	call := func(ctx context.Context, operation AdapterCall, alias string) (AdapterOutput, error) {
		single, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		defer cancel()
		return performCall(single, operation, binding, accounts[alias])
	}
	baseline := []OracleObservation{}
	setupObservation := ModelObservation{Node: "fixture.prepare", Round: record.Round, RunnerSHA256: runner.SHA256, CodeSHA256: runner.CodeSHA256, Input: json.RawMessage(`{}`), Participants: participants, Oracles: []OracleObservation{}, Actions: []ActionObservation{}}
	setupFailed := false
	for _, participant := range participants {
		oracle := manifest.Steps[0].Oracles[0]
		for _, step := range manifest.Steps {
			if step.Actor == participant.Alias {
				oracle = step.Oracles[0]
				break
			}
		}
		ctx, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		facts, hash, readErr := readOracle(ctx, oracle, binding, accounts[participant.Alias], manifest.IsolationScope)
		cancel()
		if readErr != nil {
			result.Reason = "fixture isolation: " + readErr.Error()
			setupFailed = true
			break
		}
		baseline = append(baseline, OracleObservation{Actor: participant.Alias, ID: oracle.ID, Boundary: oracle.Boundary, Before: facts, After: facts, BeforeSHA256: hash, AfterSHA256: hash})
	}
	isolationVerified := !setupFailed
	setupObservation.Oracles = baseline
	if isolationVerified {
		for _, operation := range manifest.Setup {
			output, callErr := call(actionCtx, operation.Call, operation.Actor)
			setupObservation.Actions = append(setupObservation.Actions, actionRecord(RunnerStep{Node: "fixture.prepare", Actor: operation.Actor}, actors[operation.Actor], output))
			if callErr != nil || output.Denied {
				result.Reason = "fixture preparation was rejected"
				if callErr != nil {
					result.Reason = "fixture preparation: " + callErr.Error()
				}
				setupFailed = true
				break
			}
		}
	}
	setupRAW := StepRAW{Identity: binding.Identity, Phase: "setup", Run: run, Step: -1, Expected: "ISOLATED", Observed: "ISOLATED", Preconditions: map[string]bool{}, Oracle: binding.Oracle, Runner: request.Runner, Model: &setupObservation}
	if setupFailed {
		setupRAW.Observed, setupRAW.ExitCode, setupObservation.Error = "UNKNOWN", 1, result.Reason
	}
	setupPath, setupBody, setupErr := writeRunnerRAW(d, run, -1, setupRAW)
	if setupErr != nil {
		setupFailed, result.Reason = true, "record fixture preparation: "+setupErr.Error()
	} else {
		record.SetupEvidence, record.SetupSHA256 = setupPath, digestBytes(setupBody)
		if err := d.updateModelRun(record); err != nil {
			setupFailed, result.Reason = true, "persist fixture preparation: "+err.Error()
		}
	}
	if !setupFailed {
		for index, batch := range runnerBatches(manifest) {
			last := batch[len(batch)-1]
			expected := obligation.Definition.Expected
			values := map[string]bool(obligation.Definition.Preconditions)
			x := indexModel(model)
			judged := stepRule(x, obligation, last.Transition)
			if last.Transition != "" {
				for _, transition := range model.Transitions {
					if transition.Edge == last.Transition {
						values = transition.Preconditions
						expected = x.edges[transition.Edge].To
						if transition.Outcome == branchRejection && x.rules[transition.Rule].Rejected == "UNCHANGED" {
							expected = "UNCHANGED"
						}
					}
				}
			}
			observation := ModelObservation{Node: last.Node, Transition: last.Transition, Round: record.Round, RunnerSHA256: runner.SHA256, CodeSHA256: runner.CodeSHA256, Input: json.RawMessage(`{}`), Participants: participants, Oracles: []OracleObservation{}, Actions: []ActionObservation{}}
			if len(last.Call.Body) > 0 {
				observation.Input = last.Call.Body
			}
			raw := StepRAW{Identity: binding.Identity, Run: run, Step: index + 1, Expected: expected, Observed: "UNKNOWN", Preconditions: map[string]bool{}, Before: "UNOBSERVED", After: "UNOBSERVED", Oracle: binding.Oracle, Runner: request.Runner, ExitCode: 0, Model: &observation}
			for _, step := range batch {
				var stepFacts *IndependentFacts
				for _, oracle := range step.Oracles {
					ctx, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
					facts, hash, readErr := readOracle(ctx, oracle, binding, accounts[step.Actor], manifest.IsolationScope)
					cancel()
					if readErr != nil {
						observation.Error = "before action: " + readErr.Error()
						raw.ExitCode = 1
						break
					}
					observation.Oracles = append(observation.Oracles, OracleObservation{Actor: step.Actor, ID: oracle.ID, Boundary: oracle.Boundary, Before: facts, BeforeSHA256: hash})
					if oracle.ID == judged.Oracle {
						copy := facts
						stepFacts = &copy
					}
					if oracle.ID == judged.Oracle && step.Actor == last.Actor {
						raw.Preconditions = map[string]bool(facts.Conditions)
						raw.Before, _ = factsHash(facts)
					}
				}
				if raw.ExitCode != 0 {
					break
				}
				if stepFacts == nil || !observedConditions(map[string]bool(stepFacts.Conditions), values, judged.Conditions) {
					raw.ExitCode = 1
					observation.Error = "participant preconditions do not match the frozen test; no batch action was attempted"
					break
				}
				if step.Transition != "" && stepFacts.State != x.edges[step.Transition].From {
					raw.ExitCode = 1
					observation.Error = "participant state differs from its transition source; no batch action was attempted"
					break
				}
			}
			if raw.ExitCode == 0 && !observedConditions(raw.Preconditions, values, judged.Conditions) {
				raw.ExitCode = 1
				observation.Error = "actual preconditions do not match the frozen test; action was not attempted"
			}
			if raw.ExitCode == 0 && last.Transition != "" {
				for _, oracle := range observation.Oracles {
					if oracle.ID == judged.Oracle && oracle.Actor == last.Actor && oracle.Before.State != x.edges[last.Transition].From {
						raw.ExitCode = 1
						observation.Error = "actual state differs from the declared transition source"
					}
				}
			}
			if raw.ExitCode == 0 {
				type actionResult struct {
					output AdapterOutput
					err    error
				}
				outputs := make([]actionResult, len(batch))
				semaphore := make(chan struct{}, manifest.Parallelism)
				var workers sync.WaitGroup
				for position, step := range batch {
					workers.Add(1)
					go func(position int, step RunnerStep) {
						defer workers.Done()
						select {
						case semaphore <- struct{}{}:
							defer func() { <-semaphore }()
						case <-actionCtx.Done():
							outputs[position].err = actionCtx.Err()
							return
						}
						outputs[position].output, outputs[position].err = call(actionCtx, step.Call, step.Actor)
					}(position, step)
				}
				workers.Wait()
				for position, item := range outputs {
					observation.Actions = append(observation.Actions, actionRecord(batch[position], actors[batch[position].Actor], item.output))
					if item.err != nil {
						raw.ExitCode = 1
						observation.Error = "action: " + item.err.Error()
					}
				}
				if len(outputs) == 1 {
					raw.Denied = outputs[0].output.Denied
				}
				for position := range observation.Oracles {
					item := &observation.Oracles[position]
					var adapter OracleAdapter
					for _, step := range batch {
						if step.Actor == item.Actor {
							for _, candidate := range step.Oracles {
								if candidate.ID == item.ID {
									adapter = candidate
									break
								}
							}
						}
					}
					ctx, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
					facts, hash, readErr := readOracle(ctx, adapter, binding, accounts[item.Actor], manifest.IsolationScope)
					cancel()
					if readErr != nil {
						raw.ExitCode = 1
						observation.Error = "after action: " + readErr.Error()
						continue
					}
					item.After, item.AfterSHA256 = facts, hash
					if item.ID == judged.Oracle && item.Actor == last.Actor {
						raw.After, _ = factsHash(facts)
						raw.Observed = facts.State
						if expected == "UNCHANGED" && raw.Before == raw.After {
							raw.Observed = "UNCHANGED"
						}
					}
				}
			}
			path, _, err := writeRunnerRAW(d, run, index+1, raw)
			if err != nil {
				result.Reason = err.Error()
				break
			}
			stepResult, err := d.step(run, raw.Expected, raw.Observed, path)
			if err != nil {
				result.Reason = "record action: " + err.Error()
				break
			}
			result.Steps++
			result.Verdict = fmt.Sprint(stepResult["verdict"])
			if result.Verdict != "PASS" {
				result.Reason = observation.Error
				if result.Reason == "" {
					result.Reason = "independent business state or property contradicted the frozen expectation"
				}
				break
			}
		}
	}
	cancelAction()
	if result.Steps != len(runnerBatches(manifest)) || result.Verdict != "PASS" {
		rows, err := d.query("SELECT status FROM runs WHERE id="+quote(run), 1)
		if err != nil {
			return result, err
		}
		if len(rows) == 1 && rows[0][0] == "RUNNING" {
			reason := result.Reason
			if reason == "" {
				reason = "registered execution did not complete"
			}
			if _, err := d.abort(run, reason); err != nil {
				return result, err
			}
			result.Verdict = "BLOCKED"
		}
	}
	recoveryCtx, cancelRecovery := context.WithDeadline(context.WithoutCancel(parent), deadline)
	defer cancelRecovery()
	result.Recovery = d.restoreFixture(recoveryCtx, binding, record, manifest, accounts, actors, fixture, baseline, isolationVerified)
	record.Recovery = result.Recovery
	record.Completed = true
	if err := d.updateModelRun(record); err != nil {
		return result, err
	}
	if result.Verdict == "PASS" {
		if result.Recovery.Verdict != "PASS" {
			result.Verdict = "BLOCKED"
			result.Reason = "fixture recovery failed: " + result.Recovery.Reason
			if _, err := d.abort(run, result.Reason); err != nil {
				return result, err
			}
		} else {
			if _, err := d.finish(run); err != nil {
				result.Verdict = "BLOCKED"
				result.Reason = "final qualification: " + err.Error()
				if _, abortErr := d.abort(run, result.Reason); abortErr != nil {
					return result, abortErr
				}
			} else {
				result.Reason = "registered operations, independent properties and external fixture restoration verified"
			}
		}
	} else if result.Recovery.Verdict == "PASS" {
		ids, err := d.runFixtureIDs(run, binding)
		if err != nil {
			return result, err
		}
		statements := []string{}
		for _, id := range ids {
			statements = append(statements, fmt.Sprintf("DELETE FROM fixture_locks WHERE fixture=%s AND run=%s", quote(id), quote(run)))
		}
		if err := d.transition(statements); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (d db) restoreFixture(ctx context.Context, binding RunBinding, record ModelRunRecord, manifest RunnerManifest, accounts map[string]Fixture, actors map[string]ParticipantBinding, fixture Fixture, baseline []OracleObservation, isolationVerified bool) (result RecoveryOutcome) {
	result = RecoveryOutcome{Verdict: "BLOCKED", Reason: "fixture recovery has not been observed", Actions: []ActionObservation{}}
	model := ModelObservation{Node: "fixture.restore", Round: record.Round, RunnerSHA256: record.RunnerSHA256, CodeSHA256: record.CodeSHA256, Input: json.RawMessage(`{}`), Participants: record.Participants, Actions: []ActionObservation{}, Oracles: []OracleObservation{}}
	raw := StepRAW{Identity: binding.Identity, Phase: "fixture", Run: record.Run, Step: 0, Expected: fixture.State, Observed: "UNKNOWN", Preconditions: map[string]bool{}, Oracle: binding.Oracle, Runner: record.Runner, ExitCode: 1, Model: &model}
	defer func() {
		model.Actions = result.Actions
		if result.Verdict != "PASS" {
			model.Error = result.Reason
		}
		path, body, err := writeRunnerRAW(d, record.Run, 0, raw)
		if err != nil {
			result.Verdict, result.Safe = "BLOCKED", false
			result.Reason += "; capture recovery: " + err.Error()
			return
		}
		result.Evidence, result.SHA256 = path, digestBytes(body)
	}()
	if !isolationVerified {
		result.Reason = "recovery writes refused because participant identity or fixture isolation was not verified"
		return result
	}
	protocol := manifest.Recovery
	if protocol == nil {
		for _, before := range baseline {
			oracle := manifest.Steps[0].Oracles[0]
			for _, step := range manifest.Steps {
				if step.Actor == before.Actor {
					oracle = step.Oracles[0]
					break
				}
			}
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
			facts, hash, err := readOracle(callCtx, oracle, binding, accounts[before.Actor], manifest.IsolationScope)
			cancel()
			if err != nil {
				result.Reason = "read-only checkpoint: " + err.Error()
				return result
			}
			before.After, before.AfterSHA256 = facts, hash
			model.Oracles = append(model.Oracles, before)
			beforeHash, err := factsHash(before.Before)
			if err != nil {
				result.Reason = err.Error()
				return result
			}
			afterHash, err := factsHash(facts)
			if err != nil || beforeHash != afterHash || facts.State != fixture.State {
				result.Reason = "read-only checkpoint changed the independent fixture snapshot"
				return result
			}
			if before.Actor == "primary" {
				raw.Observed, raw.Preconditions, raw.Before, raw.After = facts.State, map[string]bool(facts.Conditions), beforeHash, afterHash
			}
		}
		raw.SafeToContinue, raw.ExitCode = true, 0
		result.Verdict, result.Reason, result.Safe, result.State = "PASS", "read-only fixture unchanged under independent before/after observation", true, fixture.State
		return result
	}
	for _, step := range protocol.Steps {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		output, err := performCall(callCtx, step.Call, binding, accounts[step.Actor])
		cancel()
		result.Actions = append(result.Actions, actionRecord(RunnerStep{Node: "fixture.restore", Actor: step.Actor}, actors[step.Actor], output))
		if err != nil || output.Denied {
			if err != nil {
				result.Reason = "recovery operation: " + err.Error()
			} else {
				result.Reason = "recovery operation was rejected"
			}
			return result
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
	facts, hash, err := readOracle(callCtx, protocol.Oracle, binding, accounts["primary"], manifest.IsolationScope)
	cancel()
	if err != nil {
		result.Reason = "recovery readback: " + err.Error()
		return result
	}
	result.State = facts.State
	model.Oracles = append(model.Oracles, OracleObservation{Actor: "primary", ID: protocol.Oracle.ID, Boundary: protocol.Oracle.Boundary, Before: facts, After: facts, BeforeSHA256: hash, AfterSHA256: hash})
	raw.Observed, raw.Preconditions = facts.State, map[string]bool(facts.Conditions)
	if facts.State != fixture.State || !observedConditions(facts.Conditions, protocol.Conditions, nil) {
		result.Reason = "fixture state or conditions were not restored"
		return result
	}
	for _, outcome := range evaluateProperties(protocol.Checks, facts.Values) {
		if outcome.Verdict != "PASS" {
			result.Reason = "recovery property: " + outcome.Reason
			return result
		}
	}
	raw.SafeToContinue, raw.ExitCode = true, 0
	result.Verdict, result.Reason, result.Safe = "PASS", "isolated fixture restored and independently read back", true
	return result
}
