package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type runtimeBusinessTarget struct {
	mu                                                           sync.Mutex
	active, initialActive, stored                                bool
	unknownActive, wrongResponse, wrongOutcome, lyingPreparation bool
	requests, preparations                                       int
	preFrozen                                                    bool
	ledger                                                       *db
}

func runtimeBusinessGraph(t *testing.T) Graph {
	t.Helper()
	g := referenceModel(t)
	active := businessPredicate("eq", "account", "active", "boolean", "true")
	status := func(code string) BusinessExpectedCheck {
		return BusinessExpectedCheck{ID: "response-status", Kind: "response", Oracle: "response-capture", SourceRef: "test-spec:response", Predicate: businessPredicate("eq", "transport", "status_code", "integer", code)}
	}
	g.BusinessContract = &BusinessContract{Version: 1,
		Subjects: []BusinessSubject{
			{ID: "account", Kind: "account", RequiresBinding: true, Attributes: []BusinessAttribute{{Name: "active", Type: "boolean"}}},
			{ID: "order", Kind: "entity", Attributes: []BusinessAttribute{{Name: "stored", Type: "boolean"}}},
			{ID: "transport", Kind: "environment", Attributes: []BusinessAttribute{{Name: "status_code", Type: "integer"}, {Name: "denied", Type: "boolean"}, {Name: "exit_code", Type: "integer"}}},
			{ID: "reply", Kind: "environment", Attributes: []BusinessAttribute{{Name: "code", Type: "string"}}},
		},
		Branches: []BusinessBranch{
			{ID: "accept", Action: "enroll", Rule: "R1", FromStates: []string{"pending"}, When: active, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "created", State: "ready", Denied: businessBool(false), Predicate: businessPredicate("eq", "order", "stored", "boolean", "true"), Checks: []BusinessExpectedCheck{status("200")}}}}, Source: "document", SourceRef: "test-spec:active", Confirmed: true},
			{ID: "reject", Action: "enroll", Rule: "R1", FromStates: []string{"pending"}, When: businessNot(active), Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "unchanged", State: "UNCHANGED", Denied: businessBool(true), Predicate: businessPredicate("eq", "order", "stored", "boolean", "false"), Checks: []BusinessExpectedCheck{status("401")}}}}, Source: "document", SourceRef: "test-spec:inactive", Confirmed: true},
		},
	}
	return g
}

func newRuntimeBusinessTarget(t *testing.T, active bool) (*runtimeBusinessTarget, *httptest.Server) {
	t.Helper()
	target := &runtimeBusinessTarget{active: active, initialActive: active}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target.mu.Lock()
		defer target.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/activate":
			target.preparations++
			if !target.lyingPreparation {
				target.active = true
			}
			fmt.Fprint(w, `{"code":"ACTIVATED"}`)
		case "/enroll":
			target.requests++
			if target.ledger != nil {
				rows, err := target.ledger.query("SELECT COUNT(*) FROM business_expectation_captures WHERE status='READY' AND expectation_sha256<>''", 1)
				target.preFrozen = err == nil && len(rows) == 1 && rows[0][0] != "0"
			}
			if !target.active {
				code := http.StatusUnauthorized
				if target.wrongResponse {
					code = http.StatusConflict
				}
				w.WriteHeader(code)
				fmt.Fprint(w, `{"code":"ACCOUNT_INACTIVE"}`)
				return
			}
			if !target.wrongOutcome {
				target.stored = true
			}
			fmt.Fprint(w, `{"code":"OK"}`)
		case "/reset":
			target.stored = false
			target.active = target.initialActive
			fmt.Fprint(w, `{}`)
		default:
			state, enrolled := "pending", 0
			if target.stored {
				state, enrolled = "ready", 1
			}
			activeJSON := fmt.Sprint(target.active)
			if target.unknownActive {
				activeJSON = "null"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":1,"active":%s,"stored":%t}`, state, enrolled, activeJSON, target.stored)
		}
	}))
	t.Cleanup(server.Close)
	return target, server
}

func setupRuntimeBusiness(t *testing.T, g Graph, target *runtimeBusinessTarget, server *httptest.Server, branch string, change func(*RunnerManifest)) executionFixture {
	t.Helper()
	fixture := prepareExecutionFixture(t, g, server.URL)
	target.ledger = &fixture.d
	manifest := rewriteFixtureManifest(t, "typed-business", "R1/POSITIVE", func(m *RunnerManifest) {
		step := &m.Steps[0]
		step.BusinessBranch = branch
		step.SubjectBindings = map[string]string{"account": "actor.primary.principal"}
		step.ResponseMeasurements = map[string]string{"reply.code": "/code"}
		step.Call.RejectionStatuses = []int{401, 409, 422}
		step.Oracles[0].Measurements["account.active"] = "/active"
		step.Oracles[0].Measurements["order.stored"] = "/stored"
		if change != nil {
			change(m)
		}
	})
	fixture.registerAndRound(t, manifest)
	return fixture
}

func executeRuntimeBusiness(t *testing.T, fixture executionFixture) ExecuteResult {
	t.Helper()
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "typed-business", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRuntimeBusinessFreezesBeforeRequestAndPersistsActual(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", nil)
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" || !target.preFrozen || target.requests != 1 {
		t.Fatalf("execution lacked durable pre-action expectation: %+v target=%+v", result, target)
	}
	facts, err := fixture.d.verifiedRunFacts(result.Run)
	if err != nil || facts.EvidenceStatus != "PASS" {
		t.Fatalf("facts not independently verifiable: %+v %v", facts, err)
	}
	for _, fact := range facts.Facts {
		if fact.Seq != 1 {
			continue
		}
		if len(fact.BusinessExpectations) != 1 {
			t.Fatalf("missing frozen/actual comparison: %+v", fact)
		}
		entry := fact.BusinessExpectations[0]
		if entry.Decision.Frozen == nil || entry.Decision.Frozen.Before.State != "pending" || entry.After == nil || entry.After.State != "ready" || entry.Comparison == nil || entry.Comparison.Status != "PASS" {
			t.Fatalf("business before/expected/actual lost: %+v", entry)
		}
		if string(entry.After.Values["transport.status_code"]) != "200" || string(entry.After.Values["reply.code"]) != `"OK"` {
			t.Fatalf("actual response measurements missing: %+v", entry.After)
		}
	}
}

func TestRuntimeBusinessWrongActualCannotChooseAnotherExpected(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	target.wrongOutcome = true
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", nil)
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "FAIL" || target.requests != 1 {
		t.Fatalf("wrong actual was accepted: %+v", result)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil || raw.Model.BusinessExpectations[0].Comparison.Branch != "accept" {
		t.Fatalf("frozen branch changed: %+v %v", raw, err)
	}
}

func TestRuntimeBusinessTransportRejectionMustMatchExactStatus(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		t.Run(fmt.Sprint(wrong), func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, false)
			target.wrongResponse = wrong
			fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "reject", nil)
			result := executeRuntimeBusiness(t, fixture)
			want := "PASS"
			if wrong {
				want = "FAIL"
			}
			if result.Verdict != want || target.requests != 1 {
				t.Fatalf("HTTP rejection identity was collapsed to denied=true: want=%s got=%+v", want, result)
			}
		})
	}
}

func TestRuntimeBusinessNoRequestOnUnknownGapOverlapOrWrongPlannedBranch(t *testing.T) {
	for _, scenario := range []string{"unknown", "gap", "overlap", "wrong_planned_branch"} {
		t.Run(scenario, func(t *testing.T) {
			active := scenario == "overlap" || scenario == "unknown"
			target, server := newRuntimeBusinessTarget(t, active)
			g := runtimeBusinessGraph(t)
			branch, wantReason := "", ""
			switch scenario {
			case "unknown":
				target.unknownActive = true
				wantReason = "BLOCKED"
			case "gap":
				g.BusinessContract.Branches = g.BusinessContract.Branches[:1]
				wantReason = "SPECIFICATION_GAP"
			case "overlap":
				duplicate := g.BusinessContract.Branches[0]
				duplicate.ID = "also-accept"
				g.BusinessContract.Branches = append(g.BusinessContract.Branches, duplicate)
				wantReason = "MODEL_ERROR"
			case "wrong_planned_branch":
				branch = "accept"
				wantReason = "planned business branch"
			}
			fixture := setupRuntimeBusiness(t, g, target, server, branch, nil)
			result := executeRuntimeBusiness(t, fixture)
			if result.Verdict != "BLOCKED" || target.requests != 0 || !strings.Contains(result.Reason, wantReason) {
				t.Fatalf("unready branch issued a request or lost its reason: %+v requests=%d", result, target.requests)
			}
			raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
			if err != nil || len(raw.Model.Actions) != 0 || len(raw.Model.BusinessExpectations) != 1 {
				t.Fatalf("unattempted decision evidence missing: %+v %v", raw, err)
			}
		})
	}
}

func TestRuntimeBusinessExplicitAllowedOutcomeIsNotRejectedByLegacySummary(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	target.wrongOutcome = true
	g := runtimeBusinessGraph(t)
	expected := &g.BusinessContract.Branches[0].Expected
	expected.Mode, expected.Reason = "allowed_outcomes", "test-spec: either synchronous persistence or explicitly accepted pending outcome"
	alternative := expected.Outcomes[0]
	alternative.ID, alternative.State = "accepted-pending", "pending"
	alternative.Predicate = businessPredicate("eq", "order", "stored", "boolean", "false")
	expected.Outcomes = append(expected.Outcomes, alternative)
	fixture := setupRuntimeBusiness(t, g, target, server, "accept", nil)
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" {
		t.Fatalf("an explicitly declared outcome was overridden by the legacy summary: %+v", result)
	}
}

func TestRuntimeBusinessAutomaticPreparationUsesIndependentActual(t *testing.T) {
	for _, scenario := range []string{"prepared", "lying_adapter", "missing_adapter"} {
		t.Run(scenario, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, false)
			target.lyingPreparation = scenario == "lying_adapter"
			g := runtimeBusinessGraph(t)
			g.Nodes = append(g.Nodes, Node{ID: "activate", Kind: "action", Name: "激活测试账户"})
			g.Edges = append(g.Edges, Edge{ID: "impl-activate", From: "enrollment.submit", To: "activate", Kind: "implements"})
			active := businessPredicate("eq", "account", "active", "boolean", "true")
			g.BusinessContract.Preparation = []PreparationAction{{ID: "activate-account", Action: "activate", Requires: businessNot(active), Ensures: active, Effects: []PreparationEffect{{Subject: "account", Attribute: "active", Value: json.RawMessage(`true`)}}, Source: "document", SourceRef: "test-spec:activate"}}
			fixture := setupRuntimeBusiness(t, g, target, server, "accept", func(m *RunnerManifest) {
				observer := m.Steps[0]
				observer.BusinessBranch = ""
				adapter := RunnerStep{Node: "activate", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/activate", SuccessStatuses: []int{200}}}
				m.Preparation = &RunnerPreparation{Goal: active, MaxSteps: 4, MaxStates: 16, Observer: observer, Adapters: map[string]RunnerStep{"activate-account": adapter}}
				if scenario == "missing_adapter" {
					m.Preparation.Adapters = map[string]RunnerStep{}
				}
			})
			result := executeRuntimeBusiness(t, fixture)
			want, calls := "PASS", 1
			if scenario != "prepared" {
				want, calls = "BLOCKED", 0
			}
			if result.Verdict != want || target.requests != calls {
				t.Fatalf("preparation prediction was confused with actual state: %+v requests=%d prep=%d", result, target.requests, target.preparations)
			}
			raw, _, err := fixture.d.workbenchEvidence(result.Run, "setup", 0)
			if err != nil || raw.Model.Preparation == nil || raw.Model.Preparation.TargetReady != (scenario == "prepared") {
				t.Fatalf("preparation execution/readback not durable: %+v %v", raw, err)
			}
			if scenario == "lying_adapter" && (len(raw.Model.Preparation.Steps) != 1 || raw.Model.Preparation.Steps[0].After == nil || raw.Model.Preparation.Steps[0].Ensures.Value != businessFalse) {
				t.Fatalf("false setup success was not independently refuted: %+v", raw.Model.Preparation)
			}
		})
	}
}

func TestRuntimeBusinessMissingPreActionReceiptCannotQualify(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", nil)
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" {
		t.Fatal(result)
	}
	if err := fixture.d.tx([]string{"DELETE FROM business_expectation_captures WHERE run=" + quote(result.Run)}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.qualifyModelRun(result.Run); err == nil {
		t.Fatal("post-action RAW was accepted without its durable pre-action expectation")
	}
	if facts, err := fixture.d.verifiedRunFacts(result.Run); err == nil {
		t.Fatalf("snapshot accepted an expectation without its original receipt: %+v", facts)
	}
}
