package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ExecuteRequest struct {
	campaign *businessCampaignExecution

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
	stampRunnerIdentity(&value)
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
	return ActionObservation{
		Node: step.Node, Transition: step.Transition, Actor: step.Actor, Principal: participant.Principal,
		TemplateSHA256: output.TemplateSHA256, ResolvedPath: output.ResolvedPath, ResolvedInput: output.ResolvedInput, Substitutions: output.Substitutions,
		RequestSHA256: output.RequestSHA256, ResponseSHA256: output.ResponseSHA256,
		StatusCode: output.StatusCode, ExitCode: output.ExitCode, Denied: output.Denied, Started: output.Started, Completed: output.Completed,
	}
}

func buildConditionReadings(model Graph, actor, oracleID string, oracle OracleAdapter, expected map[string]bool, actual map[string]bool, required []string) []ConditionReading {
	subjectsByCond := map[string][]ConditionSubject{}
	for _, s := range model.ConditionSubjects {
		subjectsByCond[s.Condition] = append(subjectsByCond[s.Condition], s)
	}
	seen := map[string]bool{}
	keys := make([]string, 0, len(required)+len(expected))
	for _, id := range required {
		if !seen[id] {
			seen[id] = true
			keys = append(keys, id)
		}
	}
	extraKeys := make([]string, 0, len(expected))
	for id := range expected {
		if !seen[id] {
			seen[id] = true
			extraKeys = append(extraKeys, id)
		}
	}
	sort.Strings(extraKeys)
	keys = append(keys, extraKeys...)
	readings := make([]ConditionReading, 0, len(keys))
	for _, id := range keys {
		want, hasWant := expected[id]
		if !hasWant {
			want = true
		}
		got, found := actual[id]
		var gotPtr *bool
		if found {
			v := got
			gotPtr = &v
		}
		subjects := subjectsByCond[id]
		if len(subjects) == 0 {
			subjects = []ConditionSubject{{}}
		}
		for _, subj := range subjects {
			readings = append(readings, ConditionReading{
				Condition:      id,
				ObserverActor:  actor,
				SubjectKind:    subj.SubjectKind,
				Subject:        firstNonEmpty(subj.Subject, actor),
				Attribute:      subj.Attribute,
				Operator:       subj.Operator,
				Expected:       subj.Expected,
				AnnotationOnly: subj.Condition != "",
				ExpectedBool:   want,
				ActualBool:     gotPtr,
				OracleID:       oracleID,
				Path:           oracle.Conditions[id],
				Passed:         found && got == want,
			})
		}
	}
	return readings
}

func asyncWaitConverged(wait *WaitContract, facts IndependentFacts) bool {
	if wait.UntilState != "" && facts.State != wait.UntilState {
		return false
	}
	if len(wait.UntilConditions) > 0 && !observedConditions(facts.Conditions, wait.UntilConditions, nil) {
		return false
	}
	if len(wait.UntilChecks) > 0 {
		for _, outcome := range evaluateProperties(wait.UntilChecks, facts.Values) {
			if outcome.Verdict != "PASS" {
				return false
			}
		}
	}
	return true
}

func runStepAsyncWait(ctx context.Context, step RunnerStep, oracle OracleAdapter, binding RunBinding, account Fixture, scope string, sess *execSession) (resultFacts IndependentFacts, resultHash string, resultDoc json.RawMessage, obs WaitObservation, resultErr error) {
	wait := step.AsyncWait
	obs = WaitObservation{
		Actor:          step.Actor,
		Node:           step.Node,
		Transition:     step.Transition,
		OracleID:       wait.OracleID,
		RuleSourceRef:  wait.RuleSourceRef,
		Started:        time.Now().UTC().Format(time.RFC3339Nano),
		MaxWaitMS:      wait.MaxWaitMS,
		PollIntervalMS: wait.PollIntervalMS,
		StableWindowMS: wait.StableWindowMS,
		ObservedStates: []string{},
	}
	defer func() { obs.Completed = time.Now().UTC().Format(time.RFC3339Nano) }()
	allowed := map[string]bool{}
	for _, st := range wait.AllowedIntermediateStates {
		allowed[st] = true
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(wait.MaxWaitMS)*time.Millisecond)
	defer cancel()
	var lastFacts IndependentFacts
	var lastHash string
	var lastDoc json.RawMessage
	var stableSince time.Time
	for {
		obs.Attempts++
		pollTimeout := time.Duration(wait.PollIntervalMS) * time.Millisecond
		if pollTimeout < 250*time.Millisecond {
			pollTimeout = 250 * time.Millisecond
		}
		if remain, ok := waitCtx.Deadline(); ok {
			if r := time.Until(remain); r < pollTimeout {
				pollTimeout = r
			}
		}
		pollCtx, cancelPoll := context.WithTimeout(waitCtx, pollTimeout)
		facts, hash, doc, err := readOracleWithSession(pollCtx, oracle, binding, account, step.Actor, scope, sess)
		cancelPoll()
		if err != nil {
			obs.TimedOut = waitCtx.Err() != nil
			// A declared business window can expire during the next poll as
			// well as between polls. Preserve the last valid independent read
			// and classify both paths consistently. With no valid read, or a
			// parent cancellation, the observation itself remains BLOCKED.
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil && len(lastDoc) > 0 {
				return lastFacts, lastHash, lastDoc, obs, nil
			}
			return lastFacts, lastHash, lastDoc, obs, err
		}
		lastFacts, lastHash, lastDoc = facts, hash, doc
		obs.FinalState = facts.State
		if len(obs.ObservedStates) < 64 && (len(obs.ObservedStates) == 0 || obs.ObservedStates[len(obs.ObservedStates)-1] != facts.State) {
			obs.ObservedStates = append(obs.ObservedStates, facts.State)
		}
		if asyncWaitConverged(wait, facts) {
			if wait.StableWindowMS <= 0 {
				return lastFacts, lastHash, lastDoc, obs, nil
			}
			now := time.Now()
			if stableSince.IsZero() {
				stableSince = now
				obs.StableSince = now.UTC().Format(time.RFC3339Nano)
			}
			if now.Sub(stableSince) >= time.Duration(wait.StableWindowMS)*time.Millisecond {
				return lastFacts, lastHash, lastDoc, obs, nil
			}
		} else {
			stableSince = time.Time{}
			obs.StableSince = ""
			if len(allowed) > 0 && facts.State != wait.UntilState && !allowed[facts.State] {
				obs.ViolatedIntermediate = true
				return lastFacts, lastHash, lastDoc, obs, nil
			}
		}
		if waitCtx.Err() != nil {
			obs.TimedOut = true
			return lastFacts, lastHash, lastDoc, obs, nil
		}
		timer := time.NewTimer(time.Duration(wait.PollIntervalMS) * time.Millisecond)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			obs.TimedOut = true
			return lastFacts, lastHash, lastDoc, obs, nil
		case <-timer.C:
		}
	}
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
	if fixture.Refs["scope"] != runner.Manifest.IsolationScope || runner.Manifest.Recovery != nil && fixtureRecoveryMode(runner.Manifest.Recovery) == recoveryRestore && runner.Manifest.Recovery.State != fixture.State {
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
	run, err := d.startBoundRun(request.Task, binding, request.campaign, record)
	if err != nil {
		return result, err
	}
	result.Run = run
	if request.campaign != nil {
		obligation.Definition.Flow = request.campaign.Intent.Flow
	}
	record.Run = run
	sess := newExecSession(run, participants)
	if request.campaign != nil {
		sess.campaignGuard = func() error {
			return d.validateBusinessCampaignLease(request.campaign.Intent.Campaign, request.campaign.Lease)
		}
	}
	manifest := runner.Manifest
	campaignBound := request.campaign != nil
	total := time.Duration(manifest.TimeoutMS) * time.Millisecond
	deadline := time.Now().Add(total)
	if request.campaign != nil {
		if limit, ok := parent.Deadline(); ok && limit.Before(deadline) {
			deadline = limit
		}
	}
	actionCtx, cancelAction := context.WithTimeout(parent, total*2/3)
	defer cancelAction()
	call := func(ctx context.Context, operation AdapterCall, alias string) (AdapterOutput, error) {
		single, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		defer cancel()
		return performCallWithSession(single, operation, binding, accounts[alias], alias, sess)
	}
	baseline := []OracleObservation{}
	setupObservation := ModelObservation{EvidenceVersion: runtimeEvidenceVersion, Node: "fixture.prepare", Round: record.Round, RunnerSHA256: runner.SHA256, CodeSHA256: runner.CodeSHA256, Input: json.RawMessage(`{}`), Participants: participants, Oracles: []OracleObservation{}, Actions: []ActionObservation{}}
	setupFailed := false
	for _, participant := range participants {
		oracle := manifest.Steps[0].Oracles[0]
		foundPlain := false
		foundAny := false
		baselineSteps := append([]RunnerStep(nil), manifest.Steps...)
		if manifest.Preparation != nil {
			baselineSteps = append([]RunnerStep{manifest.Preparation.Observer}, baselineSteps...)
		}
		for _, step := range baselineSteps {
			if step.Actor == participant.Alias {
				if !foundAny {
					oracle = step.Oracles[0]
					foundAny = true
				}
				for _, cand := range step.Oracles {
					if !strings.Contains(cand.Call.Path, "{{var.") && !strings.Contains(string(cand.Call.Body), "{{var.") {
						oracle = cand
						foundPlain = true
						break
					}
				}
				if foundPlain {
					break
				}
			}
		}
		ctx, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		facts, hash, _, readErr := readOracleWithSession(ctx, oracle, binding, accounts[participant.Alias], participant.Alias, manifest.IsolationScope, sess)
		cancel()
		if readErr != nil {
			result.Reason = "fixture isolation: " + readErr.Error()
			setupFailed = true
			break
		}
		baseline = append(baseline, OracleObservation{Actor: participant.Alias, ID: oracle.ID, Boundary: oracle.Boundary, Before: facts, After: facts, BeforeSHA256: hash, AfterSHA256: hash})
	}
	setupObservation.Oracles = baseline
	if !setupFailed && manifest.Preparation != nil {
		if err := validateRuntimePreparationPreflight(model, manifest, &setupObservation); err != nil {
			result.Reason, setupFailed = "preparation identity preflight: "+err.Error(), true
		}
	}
	isolationVerified := !setupFailed
	if isolationVerified {
		for operationIndex, operation := range manifest.Setup {
			output, callErr := call(actionCtx, operation.Call, operation.Actor)
			action := actionRecord(RunnerStep{Node: "fixture.prepare", Actor: operation.Actor}, actors[operation.Actor], output)
			action.ActionIndex = operationIndex + 1
			setupObservation.Actions = append(setupObservation.Actions, action)
			if callErr != nil || output.Denied {
				result.Reason = "fixture preparation was rejected"
				if callErr != nil {
					result.Reason = "fixture preparation: " + callErr.Error()
				}
				setupFailed = true
				break
			}
			extracted, extErr := applyExtractions(operation.Call.Extract, -1, operationIndex+1, operation.Actor, &output, nil, sess)
			setupObservation.Extractions = append(setupObservation.Extractions, extracted...)
			if extErr != nil {
				result.Reason = "fixture preparation: " + extErr.Error()
				setupFailed = true
				break
			}
		}
	}
	if !setupFailed && manifest.Preparation != nil {
		preparation, scopeRejected := runRuntimePreparation(actionCtx, model, obligation.Definition.Flow, manifest, binding, accounts, actors, sess, &setupObservation)
		setupObservation.Preparation = &preparation
		if scopeRejected && len(setupObservation.Actions) == 0 {
			// No write was attempted. A rejected target identity must not
			// trigger a recovery write to an unrelated subject either.
			isolationVerified = false
		}
		if !preparation.TargetReady {
			setupFailed = true
			result.Reason = "automatic preparation: " + preparation.Status + ": " + preparation.Reason
		}
	}
	setupObservation.Entities = sess.entitySlice()
	setupObservation.Sessions = sess.sessionSlice()
	setupRAW := StepRAW{Identity: binding.Identity, Phase: "setup", Run: run, Step: -1, Expected: "ISOLATED", Observed: "ISOLATED", Preconditions: map[string]bool{}, Oracle: binding.Oracle, Runner: request.Runner, Model: &setupObservation}
	setupVerdict := "PASS"
	if setupFailed {
		setupRAW.Observed, setupRAW.ExitCode, setupObservation.Error = "UNKNOWN", 1, result.Reason
		setupVerdict = "BLOCKED"
	}
	setupPath, setupBody, setupErr := writeRunnerRAW(d, run, -1, setupRAW)
	if setupErr != nil {
		setupFailed, result.Reason = true, "record fixture preparation: "+setupErr.Error()
	} else {
		record.SetupEvidence, record.SetupSHA256 = setupPath, digestBytes(setupBody)
		if err := d.updateModelRun(record); err != nil {
			setupFailed, result.Reason = true, "persist fixture preparation: "+err.Error()
		} else if err := d.persistLifecycleStepFact("setup", binding, run, -1, "fixture.prepare", "ISOLATED", setupRAW.Observed, setupVerdict, result.Reason, setupPath, setupBody, &setupObservation, nil); err != nil {
			setupFailed, result.Reason = true, "persist setup step fact: "+err.Error()
		}
	}
	if !setupFailed {
		var successors BusinessSuccessorPolicy
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
			observation := ModelObservation{EvidenceVersion: runtimeEvidenceVersion, Node: last.Node, Transition: last.Transition, Round: record.Round, RunnerSHA256: runner.SHA256, CodeSHA256: runner.CodeSHA256, Input: json.RawMessage(`{}`), Participants: participants, Oracles: []OracleObservation{}, Actions: []ActionObservation{}}
			observation.BeforeEntities, observation.BeforeSessions = sess.entitySlice(), sess.sessionSlice()
			if len(last.Call.Body) > 0 {
				observation.Input = last.Call.Body
			}
			raw := StepRAW{Identity: binding.Identity, Run: run, Step: index + 1, Expected: expected, Observed: "UNKNOWN", Preconditions: map[string]bool{}, Before: "UNOBSERVED", After: "UNOBSERVED", Oracle: binding.Oracle, Runner: request.Runner, ExitCode: 0, Model: &observation}
			for actionIndex, step := range batch {
				stepJudged := stepRule(x, obligation, step.Transition)
				stepValues := values
				if step.Transition != "" {
					if tc, ok := x.contracts[step.Transition]; ok {
						stepValues = tc.Preconditions
					}
				}
				var stepFacts *IndependentFacts
				var stepOracle OracleAdapter
				beforeDocs := map[oracleIdentity]json.RawMessage{}
				for _, oracle := range step.Oracles {
					ctx, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
					facts, hash, beforeDoc, readErr := readOracleWithSession(ctx, oracle, binding, accounts[step.Actor], step.Actor, manifest.IsolationScope, sess)
					cancel()
					if readErr != nil {
						observation.Error = "before action: " + readErr.Error()
						raw.ExitCode = 1
						break
					}
					beforeDocs[oracleIdentity{ActionIndex: actionIndex + 1, Actor: step.Actor, ID: oracle.ID}] = beforeDoc
					observation.Oracles = append(observation.Oracles, OracleObservation{ActionIndex: actionIndex + 1, Actor: step.Actor, ID: oracle.ID, Boundary: oracle.Boundary, Before: facts, BeforeSHA256: hash})
					if oracle.ID == stepJudged.Oracle {
						copy := facts
						stepFacts = &copy
						stepOracle = oracle
					}
					if oracle.ID == judged.Oracle && step.Actor == last.Actor {
						raw.Preconditions = map[string]bool(facts.Conditions)
						raw.Before, _ = factsHash(facts)
					}
				}
				if raw.ExitCode != 0 {
					break
				}
				if request.campaign != nil {
					extracted, err := rebindBusinessCampaignEntities(step, beforeDocs, sess)
					if err != nil {
						raw.ExitCode, observation.Error = 1, "campaign fresh entity rebinding: "+err.Error()
						break
					}
					observation.Extractions = append(observation.Extractions, extracted...)
					observation.BeforeEntities, observation.BeforeSessions = sess.entitySlice(), sess.sessionSlice()
				}
				if stepFacts != nil {
					observation.Conditions = append(observation.Conditions, buildConditionReadings(model, step.Actor, stepOracle.ID, stepOracle, stepValues, map[string]bool(stepFacts.Conditions), stepJudged.Conditions)...)
				}
				if stepFacts == nil || !observedConditions(map[string]bool(stepFacts.Conditions), stepValues, stepJudged.Conditions) {
					raw.ExitCode = 1
					observation.Error = "participant preconditions do not match the frozen test; no batch action was attempted"
					break
				}
				if step.Transition != "" && stepFacts.State != x.edges[step.Transition].From {
					raw.ExitCode = 1
					observation.Error = "participant state differs from its transition source; no batch action was attempted"
					break
				}
				if model.BusinessContract != nil {
					if len(step.InputMeasurements) > 0 || campaignBound {
						request, err := captureRuntimeRequest(step, actionIndex+1, sess)
						if err != nil {
							raw.ExitCode, observation.Error = 1, "request capture: "+err.Error()+"; no batch action was attempted"
							observation.BusinessExpectations = append(observation.BusinessExpectations, BusinessExpectationObservation{Actor: step.Actor, ActionIndex: actionIndex + 1, Decision: BusinessDecision{Status: "BLOCKED", Action: step.Node, Reason: observation.Error}})
							break
						}
						observation.Requests = append(observation.Requests, request)
					}
					frozen := freezeRuntimeBusiness(model, obligation.Definition.Flow, stepJudged, step, actionIndex+1, &observation)
					observation.BusinessExpectations = append(observation.BusinessExpectations, frozen)
					if frozen.Decision.Status != "READY" {
						raw.ExitCode = 1
						observation.Error = frozen.Decision.Status + ": " + frozen.Decision.Reason + "; no batch action was attempted"
						break
					}
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
			if raw.ExitCode == 0 && model.BusinessContract != nil {
				if err := checkBusinessSuccessors(successors, batch); err != nil {
					raw.ExitCode, observation.Error = 1, err.Error()+"; no batch action was attempted"
				}
			}
			if raw.ExitCode == 0 && request.campaign != nil {
				if err := validateBusinessCampaignTarget(request.campaign, last, &observation); err != nil {
					raw.ExitCode, observation.Error = 1, "campaign target changed after selection: "+err.Error()+"; no target request was sent"
				}
			}
			if model.BusinessContract != nil && len(observation.BusinessExpectations) > 0 {
				if err := d.persistBusinessExpectationCaptures(model.ID, run, index+1, observation.BusinessExpectations); err != nil {
					raw.ExitCode = 1
					observation.Error = "persist pre-action business expectation: " + err.Error() + "; no batch action was attempted"
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
						var request *RequestObservation
						if len(step.InputMeasurements) > 0 || campaignBound {
							var err error
							request, err = runtimeRequestFor(step, position+1, &observation)
							if err != nil {
								outputs[position].err = err
								return
							}
						}
						single, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
						defer cancel()
						outputs[position].output, outputs[position].err = performCallWithRequestCapture(single, step.Call, binding, accounts[step.Actor], step.Actor, sess, request)
					}(position, step)
				}
				workers.Wait()
				for position, item := range outputs {
					action := actionRecord(batch[position], actors[batch[position].Actor], item.output)
					action.ActionIndex = position + 1
					if item.err == nil {
						values, captureErr := captureResponseMeasurements(batch[position], item.output)
						action.ResponseValues = values
						if captureErr != nil {
							item.err = captureErr
						}
					}
					observation.Actions = append(observation.Actions, action)
					if item.err != nil {
						raw.ExitCode = 1
						observation.Error = "action: " + item.err.Error()
						continue
					}
					if !item.output.Denied {
						nonOracleRules := []ExtractRule{}
						for _, ext := range batch[position].Call.Extract {
							if ext.Source != "oracle" {
								nonOracleRules = append(nonOracleRules, ext)
							}
						}
						extracted, extErr := applyExtractions(nonOracleRules, index+1, position+1, batch[position].Actor, &item.output, nil, sess)
						observation.Extractions = append(observation.Extractions, extracted...)
						if extErr != nil {
							raw.ExitCode = 1
							observation.Error = "extract: " + extErr.Error()
						}
					}
				}
				if len(outputs) == 1 {
					raw.Denied = outputs[0].output.Denied
				}
				waitReadings := map[oracleIdentity]struct {
					facts IndependentFacts
					hash  string
					doc   json.RawMessage
				}{}
				if raw.ExitCode == 0 {
					for actionIndex, step := range batch {
						if step.AsyncWait == nil {
							continue
						}
						var waitOracle OracleAdapter
						for _, candidate := range step.Oracles {
							if candidate.ID == step.AsyncWait.OracleID {
								waitOracle = candidate
								break
							}
						}
						facts, hash, doc, waitObs, waitErr := runStepAsyncWait(actionCtx, step, waitOracle, binding, accounts[step.Actor], manifest.IsolationScope, sess)
						waitObs.Step, waitObs.ActionIndex = index+1, actionIndex+1
						observation.Waits = append(observation.Waits, waitObs)
						if waitErr != nil {
							raw.ExitCode = 1
							observation.Error = "async wait: " + waitErr.Error()
							break
						}
						waitReadings[oracleIdentity{ActionIndex: actionIndex + 1, Actor: step.Actor, ID: waitOracle.ID}] = struct {
							facts IndependentFacts
							hash  string
							doc   json.RawMessage
						}{facts: facts, hash: hash, doc: doc}
					}
				}
				oracleDocs := map[oracleIdentity]json.RawMessage{}
				for position := range observation.Oracles {
					item := &observation.Oracles[position]
					key := oracleIdentity{ActionIndex: item.ActionIndex, Actor: item.Actor, ID: item.ID}
					if cached, ok := waitReadings[key]; ok {
						item.After, item.AfterSHA256 = cached.facts, cached.hash
						oracleDocs[key] = cached.doc
						if item.ID == judged.Oracle && item.Actor == last.Actor {
							raw.After, _ = factsHash(cached.facts)
							raw.Observed = cached.facts.State
							if expected == "UNCHANGED" && raw.Before == raw.After {
								raw.Observed = "UNCHANGED"
							}
						}
						continue
					}
					var adapter OracleAdapter
					for actionIndex, step := range batch {
						if actionIndex+1 == item.ActionIndex && step.Actor == item.Actor {
							for _, candidate := range step.Oracles {
								if candidate.ID == item.ID {
									adapter = candidate
									break
								}
							}
						}
					}
					ctx, cancel := context.WithTimeout(actionCtx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
					facts, hash, doc, readErr := readOracleWithSession(ctx, adapter, binding, accounts[item.Actor], item.Actor, manifest.IsolationScope, sess)
					cancel()
					if readErr != nil {
						raw.ExitCode = 1
						observation.Error = "after action: " + readErr.Error()
						continue
					}
					oracleDocs[key] = doc
					item.After, item.AfterSHA256 = facts, hash
					if item.ID == judged.Oracle && item.Actor == last.Actor {
						raw.After, _ = factsHash(facts)
						raw.Observed = facts.State
						if expected == "UNCHANGED" && raw.Before == raw.After {
							raw.Observed = "UNCHANGED"
						}
					}
				}
				if raw.ExitCode == 0 {
					for actionIndex, step := range batch {
						oracleRules := []ExtractRule{}
						for _, ext := range step.Call.Extract {
							if ext.Source == "oracle" {
								oracleRules = append(oracleRules, ext)
							}
						}
						if len(oracleRules) > 0 {
							extracted, extErr := applyExtractions(oracleRules, index+1, actionIndex+1, step.Actor, nil, oracleDocs, sess)
							observation.Extractions = append(observation.Extractions, extracted...)
							if extErr != nil {
								raw.ExitCode = 1
								observation.Error = "extract: " + extErr.Error()
							}
						}
					}
				}
			}
			observation.Entities = sess.entitySlice()
			observation.Sessions = sess.sessionSlice()
			if raw.ExitCode == 0 && model.BusinessContract != nil {
				if err := recordRuntimeBusinessComparisons(model, obligation, batch, &observation); err != nil {
					raw.ExitCode, observation.Error = 1, "business observation: "+err.Error()
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
					if r, ok := stepResult["reason"].(string); ok && r != "" {
						result.Reason = r
					}
				}
				if result.Reason == "" {
					result.Reason = "independent business state or property contradicted the frozen expectation"
				}
				break
			}
			if model.BusinessContract != nil {
				var err error
				successors, err = businessSuccessorsAfter(&observation)
				if err != nil {
					result.Verdict, result.Reason = "BLOCKED", err.Error()
					break
				}
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
	result.Recovery = d.restoreFixtureWithSession(recoveryCtx, binding, record, manifest, accounts, actors, fixture, baseline, isolationVerified, sess)
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
				if result.Recovery.Mode == recoveryCheckpoint {
					result.Reason = "registered operations and independent properties verified; isolated instance retained at its verified checkpoint"
				}
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

func (d db) restoreFixture(ctx context.Context, binding RunBinding, record ModelRunRecord, manifest RunnerManifest, accounts map[string]Fixture, actors map[string]ParticipantBinding, fixture Fixture, baseline []OracleObservation, isolationVerified bool) RecoveryOutcome {
	return d.restoreFixtureWithSession(ctx, binding, record, manifest, accounts, actors, fixture, baseline, isolationVerified, nil)
}

func (d db) restoreFixtureWithSession(ctx context.Context, binding RunBinding, record ModelRunRecord, manifest RunnerManifest, accounts map[string]Fixture, actors map[string]ParticipantBinding, fixture Fixture, baseline []OracleObservation, isolationVerified bool, sess *execSession) (result RecoveryOutcome) {
	mode, expected := fixtureRecoveryMode(manifest.Recovery), fixtureRecoveryTarget(manifest.Recovery, fixture)
	node := "fixture.restore"
	if mode == recoveryCheckpoint {
		node = "fixture.checkpoint"
	}
	result = RecoveryOutcome{Mode: mode, Verdict: "BLOCKED", Reason: "fixture recovery has not been observed", Actions: []ActionObservation{}}
	model := ModelObservation{RecoveryMode: mode, EvidenceVersion: runtimeEvidenceVersion, Node: node, Round: record.Round, RunnerSHA256: record.RunnerSHA256, CodeSHA256: record.CodeSHA256, Input: json.RawMessage(`{}`), Participants: record.Participants, Actions: []ActionObservation{}, Oracles: []OracleObservation{}}
	model.Entities, model.Sessions = sess.entitySlice(), sess.sessionSlice()
	raw := StepRAW{Identity: binding.Identity, Phase: "fixture", Run: record.Run, Step: 0, Expected: expected, Observed: "UNKNOWN", Preconditions: map[string]bool{}, Oracle: binding.Oracle, Runner: record.Runner, ExitCode: 1, Model: &model}
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
		if err := d.persistLifecycleStepFact("recovery", binding, record.Run, 0, node, expected, raw.Observed, result.Verdict, result.Reason, path, body, &model, nil); err != nil {
			result.Verdict, result.Safe = "BLOCKED", false
			result.Reason += "; persist recovery step fact: " + err.Error()
		}
	}()
	if !isolationVerified {
		result.Reason = "fixture finalization refused because participant identity or fixture isolation was not verified"
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
	if mode == recoveryCheckpoint && len(protocol.Steps) != 0 {
		result.Reason = "checkpoint cannot execute restoration operations"
		return result
	}
	for index, step := range protocol.Steps {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		output, err := performCallWithSession(callCtx, step.Call, binding, accounts[step.Actor], step.Actor, sess)
		cancel()
		action := actionRecord(RunnerStep{Node: node, Actor: step.Actor}, actors[step.Actor], output)
		action.ActionIndex = index + 1
		result.Actions = append(result.Actions, action)
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
	facts, hash, _, err := readOracleWithSession(callCtx, protocol.Oracle, binding, accounts["primary"], "primary", manifest.IsolationScope, sess)
	cancel()
	if err != nil {
		result.Reason = "recovery readback: " + err.Error()
		return result
	}
	result.State = facts.State
	model.Oracles = append(model.Oracles, OracleObservation{Actor: "primary", ID: protocol.Oracle.ID, Boundary: protocol.Oracle.Boundary, Before: facts, After: facts, BeforeSHA256: hash, AfterSHA256: hash})
	raw.Observed, raw.Preconditions = facts.State, map[string]bool(facts.Conditions)
	if facts.State != expected || !observedConditions(facts.Conditions, protocol.Conditions, nil) {
		result.Reason = "fixture state or conditions do not match the declared " + mode + " target"
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
	if mode == recoveryCheckpoint {
		result.Reason = "isolated instance retained at an independently verified checkpoint"
	}
	return result
}
