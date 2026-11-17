package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func validateResponseMeasurements(step RunnerStep) error {
	if len(step.ResponseMeasurements) > 64 {
		return fmt.Errorf("response measurements exceed 64")
	}
	for key, path := range step.ResponseMeasurements {
		if key == "" || strings.HasPrefix(key, "transport.") || !strings.HasPrefix(path, "/") || hasSensitiveKey(map[string]any{key: nil, path: nil}) {
			return fmt.Errorf("invalid or sensitive response measurement %s", key)
		}
	}
	return nil
}

func captureResponseMeasurements(step RunnerStep, output AdapterOutput) (map[string]json.RawMessage, error) {
	if len(step.ResponseMeasurements) == 0 {
		return nil, nil
	}
	var doc json.RawMessage
	if err := decodeUniqueJSON(output.Body, &doc); err != nil {
		return nil, fmt.Errorf("response measurement JSON: %w", err)
	}
	values := make(map[string]json.RawMessage, len(step.ResponseMeasurements))
	for key, path := range step.ResponseMeasurements {
		value, err := jsonPointer(doc, path)
		if err != nil || len(value) > 65536 {
			return values, fmt.Errorf("response measurement %s is missing or exceeds 64 KiB", key)
		}
		var parsed any
		if err := decodeUniqueJSON(value, &parsed); err != nil || hasSensitiveKey(parsed) {
			return values, fmt.Errorf("response measurement %s is invalid or sensitive", key)
		}
		values[key] = append(json.RawMessage(nil), value...)
	}
	return values, nil
}

func sameJSONValue(left, right json.RawMessage) bool {
	if len(left) == 0 || len(right) == 0 {
		return len(left) == len(right)
	}
	var a, b bytes.Buffer
	if json.Compact(&a, left) != nil || json.Compact(&b, right) != nil {
		return false
	}
	return bytes.Equal(a.Bytes(), b.Bytes())
}

func oracleForAction(item OracleObservation, actor string, actionIndex, version int) bool {
	return item.Actor == actor && (version < runtimeEvidenceVersion || item.ActionIndex == actionIndex)
}

func verifyStepWait(step RunnerStep, seq, actionIndex int, observed *ModelObservation) StepAssertion {
	wait := step.AsyncWait
	assertion := StepAssertion{Kind: "async_wait", Name: wait.OracleID, OracleID: wait.OracleID, Subject: step.Actor, Expected: wait.UntilState, Operator: "async", Verdict: "BLOCKED"}
	if observed.EvidenceVersion < runtimeEvidenceVersion {
		assertion.Reason = "legacy async evidence has no verified actor and action identity"
		return assertion
	}
	var reading *WaitObservation
	for i := range observed.Waits {
		item := &observed.Waits[i]
		if item.Actor == step.Actor && item.Step == seq && item.ActionIndex == actionIndex && item.OracleID == wait.OracleID {
			if reading != nil {
				assertion.Reason = "duplicate async_wait evidence for the same actor and action"
				return assertion
			}
			reading = item
		}
	}
	if reading == nil {
		assertion.Reason = "async_wait evidence missing for this actor and action"
		return assertion
	}
	assertion.Actual = reading.FinalState
	started, startErr := time.Parse(time.RFC3339Nano, reading.Started)
	completed, endErr := time.Parse(time.RFC3339Nano, reading.Completed)
	if reading.Node != step.Node || reading.Transition != step.Transition || reading.RuleSourceRef != wait.RuleSourceRef || reading.MaxWaitMS != wait.MaxWaitMS || reading.PollIntervalMS != wait.PollIntervalMS || reading.StableWindowMS != wait.StableWindowMS || reading.Attempts < 1 || startErr != nil || endErr != nil || completed.Before(started) {
		assertion.Reason = "async_wait identity, bounds or timestamps differ from the frozen contract"
		return assertion
	}
	var oracle *OracleObservation
	for i := range observed.Oracles {
		item := &observed.Oracles[i]
		if item.ID == wait.OracleID && oracleForAction(*item, step.Actor, actionIndex, observed.EvidenceVersion) {
			if oracle != nil {
				assertion.Reason = "duplicate independent oracle for async_wait"
				return assertion
			}
			oracle = item
		}
	}
	if oracle == nil || oracle.After.State != reading.FinalState {
		assertion.Reason = "async_wait final state is not bound to the actor's independent oracle"
		return assertion
	}
	if reading.ViolatedIntermediate {
		assertion.Verdict, assertion.Reason = "FAIL", fmt.Sprintf("async_wait observed illegal intermediate state in %v (rule %s)", reading.ObservedStates, wait.RuleSourceRef)
		return assertion
	}
	if reading.TimedOut {
		assertion.Verdict, assertion.Reason = "FAIL", fmt.Sprintf("async_wait timed out after %dms/%d attempts; final state %s (rule %s)", wait.MaxWaitMS, reading.Attempts, reading.FinalState, wait.RuleSourceRef)
		return assertion
	}
	if !asyncWaitConverged(wait, oracle.After) {
		assertion.Verdict, assertion.Reason = "FAIL", "async_wait final independent readings do not satisfy the declared convergence checks"
		return assertion
	}
	if wait.StableWindowMS > 0 {
		stableSince, err := time.Parse(time.RFC3339Nano, reading.StableSince)
		if err != nil || stableSince.Before(started) || completed.Sub(stableSince) < time.Duration(wait.StableWindowMS)*time.Millisecond {
			assertion.Reason = "async_wait stable window has no complete time evidence"
			return assertion
		}
	}
	assertion.Verdict, assertion.Reason = "PASS", fmt.Sprintf("actor %s converged in %d attempts (rule %s)", step.Actor, reading.Attempts, wait.RuleSourceRef)
	return assertion
}
