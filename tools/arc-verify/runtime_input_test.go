package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func configureInputMeasurements(t *testing.T, step *RunnerStep, measurements map[string]string) {
	t.Helper()
	body, _ := json.Marshal(step)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	fields["input_measurements"], _ = json.Marshal(measurements)
	body, _ = json.Marshal(fields)
	if err := json.Unmarshal(body, step); err != nil {
		t.Fatal(err)
	}
}

func runtimeInputGraph(t *testing.T, amount string) Graph {
	t.Helper()
	g := runtimeBusinessGraph(t)
	g.BusinessContract.Subjects = append(g.BusinessContract.Subjects, BusinessSubject{ID: "request", Kind: "environment", Attributes: []BusinessAttribute{{Name: "amount", Type: "integer"}, {Name: "currency", Type: "string"}, {Name: "merchant_no", Type: "string"}}})
	g.BusinessContract.Branches[0].When = BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		g.BusinessContract.Branches[0].When,
		businessPredicate("eq", "request", "amount", "integer", amount),
	}}
	return g
}

func TestRuntimeInputUsesResolvedRequestBeforeFreezing(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeInputGraph(t, "100"), target, server, "accept", func(m *RunnerManifest) {
		m.Steps[0].Call.Body = json.RawMessage(`{"amount":100,"currency":"USD","merchant_no":"{{run.id}}"}`)
		configureInputMeasurements(t, &m.Steps[0], map[string]string{"request.amount": "/amount", "request.currency": "/currency", "request.merchant_no": "/merchant_no"})
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" || target.requests != 1 || !target.preFrozen {
		t.Fatalf("actual request fields were not frozen before dispatch: %+v requests=%d", result, target.requests)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	before := raw.Model.BusinessExpectations[0].Decision.Frozen.Before
	if len(raw.Model.Requests) != 1 || raw.Model.BusinessExpectations[0].Request == nil || !requestMatchesAction(raw.Model.Requests[0], raw.Model.Actions[0]) {
		t.Fatalf("frozen request is not tied to the actual emitted operation: %+v", raw.Model)
	}
	wantRun, _ := json.Marshal(result.Run)
	if string(before.Values["request.amount"]) != "100" || string(before.Values["request.currency"]) != `"USD"` || !sameJSONValue(before.Values["request.merchant_no"], wantRun) || len(before.Sources["request.amount"]) != 1 || before.Sources["request.amount"][0] != "request-capture" {
		t.Fatalf("before facts did not bind the resolved runtime body: %+v", before)
	}
	facts, err := fixture.d.verifiedRunFacts(result.Run)
	if err != nil || facts.EvidenceStatus != "PASS" {
		t.Fatalf("request capture could not be independently replayed: %+v %v", facts, err)
	}
}

func TestRuntimeInputInvalidActualBlocksBeforeRequest(t *testing.T) {
	for _, input := range []string{`{"amount":99}`, `{"currency":"USD"}`, `{"amount":"100"}`} {
		t.Run(input, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			fixture := setupRuntimeBusiness(t, runtimeInputGraph(t, "100"), target, server, "accept", func(m *RunnerManifest) {
				m.Steps[0].Call.Body = json.RawMessage(input)
				configureInputMeasurements(t, &m.Steps[0], map[string]string{"request.amount": "/amount"})
			})
			result := executeRuntimeBusiness(t, fixture)
			if result.Verdict != "BLOCKED" || target.requests != 0 {
				t.Fatalf("invalid actual input reached the target: %+v requests=%d", result, target.requests)
			}
			raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
			if err != nil || len(raw.Model.Actions) != 0 || len(raw.Model.BusinessExpectations) != 1 {
				t.Fatalf("unattempted input decision lost its evidence: %+v %v", raw, err)
			}
		})
	}
}

func TestRuntimeInputMutationAfterFreezeCannotDispatch(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	step := RunnerStep{Node: "enroll", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: "POST", Path: "/enroll", Body: json.RawMessage(`{"amount":"{{var.amount}}"}`), SuccessStatuses: []int{200}}, InputMeasurements: map[string]string{"request.amount": "/amount"}}
	sess := newExecSession("run-input-freeze", []ParticipantBinding{{Alias: "primary", Principal: "synthetic-STUDENT"}})
	sess.vars["amount"] = ExtractedBinding{Name: "amount", Value: "100"}
	frozen, err := captureRuntimeRequest(step, 1, sess)
	if err != nil {
		t.Fatal(err)
	}
	sess.vars["amount"] = ExtractedBinding{Name: "amount", Value: "200"}
	_, err = performCallWithRequestCapture(context.Background(), step.Call, RunBinding{Identity: ExecutionIdentity{Target: server.URL}}, Fixture{}, step.Actor, sess, &frozen)
	if err == nil || target.requests != 0 || !strings.Contains(err.Error(), "request was not sent") {
		t.Fatalf("changed request escaped its frozen precondition: %v requests=%d", err, target.requests)
	}
}

func TestRuntimeInputReplayRejectsTamperedOrOtherActorValues(t *testing.T) {
	step := RunnerStep{Node: "enroll", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: "POST", Path: "/enroll", Body: json.RawMessage(`{"amount":100}`), SuccessStatuses: []int{200}}, InputMeasurements: map[string]string{"request.amount": "/amount"}}
	for _, corrupt := range []string{"value", "actor", "resolved_body", "request_hash"} {
		t.Run(corrupt, func(t *testing.T) {
			capture, err := captureRuntimeRequest(step, 1, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateRuntimeRequest(step, 1, capture); err != nil {
				t.Fatalf("control capture invalid: %v", err)
			}
			switch corrupt {
			case "value":
				capture.Values["request.amount"] = json.RawMessage(`999`)
			case "actor":
				capture.Actor = "other-account"
			case "resolved_body":
				capture.ResolvedInput = json.RawMessage(`{"amount":999}`)
			case "request_hash":
				capture.RequestSHA256 = strings.Repeat("0", 64)
			}
			if err := validateRuntimeRequest(step, 1, capture); err == nil {
				t.Fatal("altered request evidence was accepted")
			}
		})
	}
}

func TestRuntimeInputOracleCannotSupplyInventedRequest(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeInputGraph(t, "1"), target, server, "accept", func(m *RunnerManifest) {
		m.Steps[0].Call.Body = json.RawMessage(`{"amount":0}`)
		m.Steps[0].Oracles[0].Measurements["request.amount"] = "/capacity"
		configureInputMeasurements(t, &m.Steps[0], map[string]string{"request.amount": "/amount"})
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "BLOCKED" || target.requests != 0 || !strings.Contains(result.Reason, "request") {
		t.Fatalf("oracle constants masqueraded as actual request input: %+v requests=%d", result, target.requests)
	}
}
