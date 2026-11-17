package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type businessChainServer struct {
	mu                     sync.Mutex
	orderSeq               int
	refundSeq              int
	currentOrder           string
	currentRefund          string
	state                  string
	notifyPolls            int
	notifyAfterPolls       int
	illegalAsyncState      string
	neverConvergeAsync     bool
	omitOrderExtract       bool
	corruptRefundAccount   bool
	providerActionsCalled  int
	refundActionsCalled    int
	observedCallbackOrder  string
	observedRefundOrder    string
	sessionCookieAccepted  bool
}

func newBusinessChainServer(t *testing.T) (*businessChainServer, *httptest.Server) {
	t.Helper()
	srv := &businessChainServer{
		state:            "S-INITIAL",
		notifyAfterPolls: 2,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		srv.mu.Lock()
		defer srv.mu.Unlock()

		actorRole := "ROLE-CALLER"
		actorPrincipal := "principal-caller"
		switch r.Header.Get("X-Actor-Key") {
		case "provider-secret":
			actorRole = "ROLE-PROVIDER"
			actorPrincipal = "principal-provider"
		case "worker-secret":
			actorRole = "ROLE-WORKER"
			actorPrincipal = "principal-worker"
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/reset":
			srv.currentOrder = ""
			srv.currentRefund = ""
			srv.state = "S-INITIAL"
			srv.notifyPolls = 0
			fmt.Fprint(w, `{"reset":true}`)

		case r.Method == http.MethodPost && r.URL.Path == "/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "state_sid", Value: "sess-caller-001", Path: "/"})
			w.Header().Set("X-Caller-Session", "sess-hdr-001")
			fmt.Fprint(w, `{"logged_in":true,"session_label":"caller-session"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/payments":
			c, err := r.Cookie("state_sid")
			if err != nil || c.Value != "sess-caller-001" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"caller session cookie missing"}`)
				return
			}
			srv.sessionCookieAccepted = true
			srv.orderSeq++
			orderID := fmt.Sprintf("P-%04d", 1000+srv.orderSeq)
			srv.currentOrder = orderID
			srv.state = "S-PAY-CONFIRMED"
			if srv.omitOrderExtract {
				fmt.Fprint(w, `{"status":"created","stored_account":"acct-alpha"}`)
				return
			}
			fmt.Fprintf(w, `{"payment_order_id":%q,"stored_account":"acct-alpha","state":"S-PAY-CONFIRMED"}`, orderID)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/callbacks/v1/payments/") && strings.HasSuffix(r.URL.Path, "/paid"):
			if r.Header.Get("X-Actor-Key") != "provider-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"provider auth missing"}`)
				return
			}
			srv.providerActionsCalled++
			orderID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/callbacks/v1/payments/"), "/paid")
			srv.observedCallbackOrder = orderID
			if orderID != srv.currentOrder || srv.currentOrder == "" {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":"payment order not found"}`)
				return
			}
			srv.state = "S-PAID"
			fmt.Fprintf(w, `{"payment_order_id":%q,"state":"S-PAID"}`, orderID)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/worker/v1/notify/"):
			if r.Header.Get("X-Actor-Key") != "worker-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"worker auth missing"}`)
				return
			}
			orderID := strings.TrimPrefix(r.URL.Path, "/worker/v1/notify/")
			if orderID != srv.currentOrder {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":"payment order not found"}`)
				return
			}
			srv.notifyPolls = 0
			srv.state = "S-NOTIFYING"
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, `{"payment_order_id":%q,"state":"S-NOTIFYING"}`, orderID)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/payments/") && strings.HasSuffix(r.URL.Path, "/refunds"):
			c, err := r.Cookie("state_sid")
			if err != nil || c.Value != "sess-caller-001" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"caller session cookie missing"}`)
				return
			}
			srv.refundActionsCalled++
			orderID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/payments/"), "/refunds")
			srv.observedRefundOrder = orderID
			if orderID != srv.currentOrder {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":"original payment not found"}`)
				return
			}
			srv.refundSeq++
			refundID := fmt.Sprintf("R-%04d", 2000+srv.refundSeq)
			srv.currentRefund = refundID
			srv.state = "S-REFUNDED"
			storedAcct := "acct-alpha"
			if srv.corruptRefundAccount {
				storedAcct = "acct-tampered"
			}
			fmt.Fprintf(w, `{"refund_order_id":%q,"payment_order_id":%q,"stored_account":%q,"state":"S-REFUNDED"}`, refundID, orderID, storedAcct)

		case r.Method == http.MethodGet && (r.URL.Path == "/oracle/baseline" || strings.HasPrefix(r.URL.Path, "/oracle/payments")):
			if r.URL.Path != "/oracle/baseline" && strings.HasPrefix(r.URL.Path, "/oracle/payments/") {
				orderID := strings.TrimPrefix(r.URL.Path, "/oracle/payments/")
				if srv.currentOrder != "" && orderID != srv.currentOrder {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"error":"unknown payment order"}`)
					return
				}
			}
			if srv.state == "S-NOTIFYING" {
				srv.notifyPolls++
				if srv.illegalAsyncState != "" {
					srv.state = srv.illegalAsyncState
				} else if !srv.neverConvergeAsync && srv.notifyPolls >= srv.notifyAfterPolls {
					srv.state = "S-NOTIFIED"
				}
			}
			storedAcct := "acct-alpha"
			if srv.state == "S-REFUNDED" && srv.corruptRefundAccount {
				storedAcct = "acct-tampered"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":%q,"scope":"pay-chain-fixture","conditions":{"C-ACCOUNT-ACTIVE":true},"measurements":{"stored_account":%q,"original_account":"acct-alpha","payment_order_id":%q,"refund_order_id":%q}}`,
				srv.state, actorPrincipal, actorRole, storedAcct, srv.currentOrder, srv.currentRefund)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":"route not found"}`)
		}
	}))
	t.Cleanup(server.Close)
	return srv, server
}

func businessChainModelJSON() string {
	return `{
  "schema_version": 2,
  "id": "pay-chain-v3",
  "system": "SYS-PAY-CHAIN",
  "scope": "Synthetic multi-actor payment-callback-notify-refund model for arc-verify business chain tests",
  "nodes": [
    {"id":"SYS-PAY-CHAIN","kind":"system","name":"支付转发业务链系统"},
    {"id":"D-TRADE","kind":"domain","name":"交易转发域"},
    {"id":"D-ASYNC","kind":"domain","name":"异步回调通知域"},
    {"id":"CAP-PAY","kind":"capability","name":"创建支付"},
    {"id":"CAP-CALLBACK","kind":"capability","name":"渠道异步回调"},
    {"id":"CAP-DELIVERY","kind":"capability","name":"终态通知投递"},
    {"id":"CAP-REFUND","kind":"capability","name":"基于原单退款"},
    {"id":"BND-PAY","kind":"boundary","name":"转发边界"},
    {"id":"ROLE-CALLER","kind":"role","name":"业务方"},
    {"id":"ROLE-PROVIDER","kind":"role","name":"上游渠道"},
    {"id":"ROLE-WORKER","kind":"role","name":"通知工作者"},
    {"id":"A-PAY-CREATE","kind":"action","name":"创建支付单"},
    {"id":"A-CB-PAID","kind":"action","name":"渠道成功回调"},
    {"id":"A-NOTIFY-DISPATCH","kind":"action","name":"投递终态通知"},
    {"id":"A-REFUND-CREATE","kind":"action","name":"提交原单退款"},
    {"id":"C-ACCOUNT-ACTIVE","kind":"condition","name":"路由账户激活"},
    {"id":"S-INITIAL","kind":"state","name":"初始待交易"},
    {"id":"S-PAY-CONFIRMED","kind":"state","name":"支付单已创建"},
    {"id":"S-PAID","kind":"state","name":"渠道已支付"},
    {"id":"S-NOTIFYING","kind":"state","name":"通知投递中"},
    {"id":"S-NOTIFIED","kind":"state","name":"通知已终态送达"},
    {"id":"S-REFUNDED","kind":"state","name":"退款已完成"}
  ],
  "contracts": [
    {"subject":"CAP-PAY","description":"Create payment order","inputs":["caller_session","account"],"outputs":["payment_order_id"],"source":"document","source_ref":"spec/pay.md:10"},
    {"subject":"CAP-CALLBACK","description":"Accept provider paid callback","inputs":["payment_order_id"],"outputs":["paid_status"],"source":"document","source_ref":"spec/cb.md:20"},
    {"subject":"CAP-DELIVERY","description":"Deliver async notification","inputs":["payment_order_id"],"outputs":["notified"],"source":"document","source_ref":"spec/notify.md:30"},
    {"subject":"CAP-REFUND","description":"Refund referencing original payment order","inputs":["payment_order_id"],"outputs":["refund_order_id"],"source":"document","source_ref":"spec/refund.md:40"}
  ],
  "condition_subjects": [
    {"condition":"C-ACCOUNT-ACTIVE","subject_kind":"account","subject":"ROLE-CALLER","attribute":"merchant_account_enabled","operator":"eq","expected":"true","description":"Caller forwarding account must be active"}
  ],
  "edges": [
    {"id":"c-sys-trade","from":"SYS-PAY-CHAIN","to":"D-TRADE","kind":"contains"},
    {"id":"c-sys-async","from":"SYS-PAY-CHAIN","to":"D-ASYNC","kind":"contains"},
    {"id":"c-trade-pay","from":"D-TRADE","to":"CAP-PAY","kind":"contains"},
    {"id":"c-trade-refund","from":"D-TRADE","to":"CAP-REFUND","kind":"contains"},
    {"id":"c-async-cb","from":"D-ASYNC","to":"CAP-CALLBACK","kind":"contains"},
    {"id":"c-async-deliv","from":"D-ASYNC","to":"CAP-DELIVERY","kind":"contains"},
    {"id":"own-pay","from":"BND-PAY","to":"CAP-PAY","kind":"owns"},
    {"id":"own-cb","from":"BND-PAY","to":"CAP-CALLBACK","kind":"owns"},
    {"id":"own-deliv","from":"BND-PAY","to":"CAP-DELIVERY","kind":"owns"},
    {"id":"own-refund","from":"BND-PAY","to":"CAP-REFUND","kind":"owns"},
    {"id":"imp-pay","from":"CAP-PAY","to":"A-PAY-CREATE","kind":"implements"},
    {"id":"imp-cb","from":"CAP-CALLBACK","to":"A-CB-PAID","kind":"implements"},
    {"id":"imp-deliv","from":"CAP-DELIVERY","to":"A-NOTIFY-DISPATCH","kind":"implements"},
    {"id":"imp-refund","from":"CAP-REFUND","to":"A-REFUND-CREATE","kind":"implements"},
    {"id":"srv-caller","from":"ROLE-CALLER","to":"D-TRADE","kind":"serves"},
    {"id":"srv-provider","from":"ROLE-PROVIDER","to":"D-ASYNC","kind":"serves"},
    {"id":"srv-worker","from":"ROLE-WORKER","to":"D-ASYNC","kind":"serves"},
    {"id":"req-pay","from":"C-ACCOUNT-ACTIVE","to":"A-PAY-CREATE","kind":"requires"},
    {"id":"req-cb","from":"C-ACCOUNT-ACTIVE","to":"A-CB-PAID","kind":"requires"},
    {"id":"req-notify","from":"C-ACCOUNT-ACTIVE","to":"A-NOTIFY-DISPATCH","kind":"requires"},
    {"id":"req-refund","from":"C-ACCOUNT-ACTIVE","to":"A-REFUND-CREATE","kind":"requires"},
    {"id":"at-pay","from":"A-PAY-CREATE","to":"S-PAY-CONFIRMED","kind":"transitions"},
    {"id":"at-cb","from":"A-CB-PAID","to":"S-PAID","kind":"transitions"},
    {"id":"at-deliv","from":"A-NOTIFY-DISPATCH","to":"S-NOTIFIED","kind":"transitions"},
    {"id":"at-refund","from":"A-REFUND-CREATE","to":"S-REFUNDED","kind":"transitions"},
    {"id":"TR-PAY","from":"S-INITIAL","to":"S-PAY-CONFIRMED","kind":"transitions_to"},
    {"id":"TR-CB","from":"S-PAY-CONFIRMED","to":"S-PAID","kind":"transitions_to"},
    {"id":"TR-NOTIFY","from":"S-PAID","to":"S-NOTIFIED","kind":"transitions_to"},
    {"id":"TR-REFUND","from":"S-NOTIFIED","to":"S-REFUNDED","kind":"transitions_to"},
    {"id":"v-identity","from":"T-FLOW-PAY-REFUND","to":"INV-ORIGINAL-IDENTITY","kind":"verifies"}
  ],
  "rules": [
    {"id":"R-PAY-CREATE","action":"A-PAY-CREATE","actor":"ROLE-CALLER","entry":"/api/v1/payments","oracle":"pg-independent-store","conditions":["C-ACCOUNT-ACTIVE"],"expected":"S-PAY-CONFIRMED","rejected":"UNCHANGED","source":"document","description":"Create payment under caller session"},
    {"id":"R-CB-PAID","action":"A-CB-PAID","actor":"ROLE-PROVIDER","entry":"/callbacks/v1/payments/{id}/paid","oracle":"pg-independent-store","conditions":["C-ACCOUNT-ACTIVE"],"expected":"S-PAID","rejected":"UNCHANGED","source":"document","description":"Provider marks payment paid"},
    {"id":"R-NOTIFY","action":"A-NOTIFY-DISPATCH","actor":"ROLE-WORKER","entry":"/worker/v1/notify/{id}","oracle":"pg-independent-store","conditions":["C-ACCOUNT-ACTIVE"],"expected":"S-NOTIFIED","rejected":"UNCHANGED","source":"document","description":"Worker finishes notification delivery"},
    {"id":"R-REFUND","action":"A-REFUND-CREATE","actor":"ROLE-CALLER","entry":"/api/v1/payments/{id}/refunds","oracle":"pg-independent-store","conditions":["C-ACCOUNT-ACTIVE"],"expected":"S-REFUNDED","rejected":"UNCHANGED","source":"document","description":"Caller refunds created payment retaining original account"}
  ],
  "transitions": [
    {"edge":"TR-PAY","action":"A-PAY-CREATE","rule":"R-PAY-CREATE","preconditions":{"C-ACCOUNT-ACTIVE":true},"outcome":"success"},
    {"edge":"TR-CB","action":"A-CB-PAID","rule":"R-CB-PAID","preconditions":{"C-ACCOUNT-ACTIVE":true},"outcome":"success"},
    {"edge":"TR-NOTIFY","action":"A-NOTIFY-DISPATCH","rule":"R-NOTIFY","preconditions":{"C-ACCOUNT-ACTIVE":true},"outcome":"success"},
    {"edge":"TR-REFUND","action":"A-REFUND-CREATE","rule":"R-REFUND","preconditions":{"C-ACCOUNT-ACTIVE":true},"outcome":"success"}
  ],
  "flows": [
    {"id":"FLOW-PAY-CALLBACK-NOTIFY-REFUND","capability":"CAP-PAY","capabilities":["CAP-PAY","CAP-CALLBACK","CAP-DELIVERY","CAP-REFUND"],"initial":"S-INITIAL"}
  ],
  "invariants": [
    {"id":"INV-ORIGINAL-IDENTITY","name":"后续退款保持原账户身份","scope":["CAP-PAY","CAP-REFUND"],"oracle":"pg-independent-store","observation_boundary":"after refund creation referencing payment_order_id","checks":[{"left":"stored_account","operator":"eq","right":"original_account"}],"source":"document","source_ref":"spec/refund.md:45"}
  ],
  "tests": [
    {
      "id":"T-FLOW-PAY-REFUND",
      "name":"跨角色跨能力业务链：登录会话→创建支付(提取订单ID)→渠道回调→异步等待通知→原单退款",
      "rule":"R-REFUND",
      "capability":"CAP-PAY",
      "kind":"SEQUENCE",
      "flow":"FLOW-PAY-CALLBACK-NOTIFY-REFUND",
      "steps":["TR-PAY","TR-CB","TR-NOTIFY","TR-REFUND"],
      "expected":"S-REFUNDED",
      "actors":[
        {"alias":"provider","role":"ROLE-PROVIDER"},
        {"alias":"worker","role":"ROLE-WORKER"}
      ],
      "source":"document",
      "source_ref":"spec/e2e.md:1"
    }
  ],
  "risks": []
}`
}

func businessChainManifest(id string) RunnerManifest {
	callerOracle := OracleAdapter{
		ID:            "pg-independent-store",
		Boundary:      "after refund creation referencing payment_order_id",
		Call:          AdapterCall{Kind: adapterHTTP, Method: http.MethodGet, Path: "/oracle/baseline", SuccessStatuses: []int{200}},
		StatePath:     "/state",
		PrincipalPath: "/principal",
		RolePath:      "/role",
		ScopePath:     "/scope",
		Conditions:    map[string]string{"C-ACCOUNT-ACTIVE": "/conditions/C-ACCOUNT-ACTIVE"},
		Measurements:  map[string]string{"stored_account": "/measurements/stored_account", "original_account": "/measurements/original_account"},
	}
	orderOracle := func(headerEnvRef string) []OracleAdapter {
		perOrder := callerOracle
		perOrder.Call.Path = "/oracle/payments/{{var.payment_order_id}}"
		baseline := callerOracle
		baseline.ID = "pg-baseline-store"
		if headerEnvRef != "" {
			perOrder.Call.HeaderRefs = map[string]string{"X-Actor-Key": headerEnvRef}
			baseline.Call.HeaderRefs = map[string]string{"X-Actor-Key": headerEnvRef}
		}
		return []OracleAdapter{perOrder, baseline}
	}
	return RunnerManifest{
		SchemaVersion:  1,
		ID:             id,
		Graph:          "pay-chain-v3",
		Test:           "T-FLOW-PAY-REFUND",
		IsolationScope: "pay-chain-fixture",
		TimeoutMS:      10000,
		StepTimeoutMS:  3000,
		Parallelism:    1,
		Setup: []RecoveryStep{
			{
				Actor: "primary",
				Call: AdapterCall{
					Kind:            adapterHTTP,
					Method:          http.MethodPost,
					Path:            "/auth/login",
					SessionScope:    "caller",
					RetainSession:   true,
					SuccessStatuses: []int{200},
					Extract: []ExtractRule{
						{Name: "state_sid_label", Source: "response_body", Path: "/session_label", EntityKind: "caller_session", EntityAlias: "caller_session", OwnerActor: "ROLE-CALLER", SessionScope: "caller"},
						{Name: "state_sid_ref", Source: "state_cookie", Path: "state_sid", SessionScope: "caller", Required: true, Redact: true},
					},
				},
			},
		},
		Steps: []RunnerStep{
			{
				Node:       "A-PAY-CREATE",
				Transition: "TR-PAY",
				Actor:      "primary",
				Call: AdapterCall{
					Kind:            adapterHTTP,
					Method:          http.MethodPost,
					Path:            "/api/v1/payments",
					SessionScope:    "caller",
					SuccessStatuses: []int{200},
					Extract: []ExtractRule{
						{Name: "payment_order_id", Source: "response_body", Path: "/payment_order_id", EntityKind: "payment_order", EntityAlias: "current_payment", OwnerActor: "ROLE-CALLER", Required: true},
					},
				},
				Oracles: []OracleAdapter{callerOracle},
			},
			{
				Node:       "A-CB-PAID",
				Transition: "TR-CB",
				Actor:      "provider",
				Call: AdapterCall{
					Kind:            adapterHTTP,
					Method:          http.MethodPost,
					Path:            "/callbacks/v1/payments/{{var.payment_order_id}}/paid",
					HeaderRefs:      map[string]string{"X-Actor-Key": "env://ARC_TEST_PROVIDER_KEY"},
					SuccessStatuses: []int{200},
				},
				Oracles: orderOracle("env://ARC_TEST_PROVIDER_KEY"),
			},
			{
				Node:       "A-NOTIFY-DISPATCH",
				Transition: "TR-NOTIFY",
				Actor:      "worker",
				Call: AdapterCall{
					Kind:            adapterHTTP,
					Method:          http.MethodPost,
					Path:            "/worker/v1/notify/{{var.payment_order_id}}",
					HeaderRefs:      map[string]string{"X-Actor-Key": "env://ARC_TEST_WORKER_KEY"},
					SuccessStatuses: []int{202},
				},
				Oracles: orderOracle("env://ARC_TEST_WORKER_KEY"),
				AsyncWait: &WaitContract{
					OracleID:                  "pg-independent-store",
					UntilState:                "S-NOTIFIED",
					AllowedIntermediateStates: []string{"S-PAID", "S-NOTIFYING"},
					MaxWaitMS:                 1500,
					PollIntervalMS:            25,
					StableWindowMS:            25,
					RuleSourceRef:             "spec/notify.md:30",
				},
			},
			{
				Node:       "A-REFUND-CREATE",
				Transition: "TR-REFUND",
				Actor:      "primary",
				Call: AdapterCall{
					Kind:            adapterHTTP,
					Method:          http.MethodPost,
					Path:            "/api/v1/payments/{{var.payment_order_id}}/refunds",
					SessionScope:    "caller",
					SuccessStatuses: []int{200},
					Extract: []ExtractRule{
						{Name: "refund_order_id", Source: "response_body", Path: "/refund_order_id", EntityKind: "refund_order", EntityAlias: "current_refund", OwnerActor: "ROLE-CALLER", Required: true},
					},
				},
				Oracles: orderOracle(""),
				Checks: []PropertyCheck{
					{Left: "stored_account", Operator: "eq", Right: "original_account"},
				},
			},
		},
		Recovery: &FixtureRecovery{
			IsolationScope: "pay-chain-fixture",
			Steps: []RecoveryStep{
				{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/reset", SuccessStatuses: []int{200}}},
			},
			Oracle:     callerOracle,
			State:      "S-INITIAL",
			Conditions: ConditionAssignment{"C-ACCOUNT-ACTIVE": true},
			Checks:     []PropertyCheck{{Left: "stored_account", Operator: "eq", Right: "original_account"}},
		},
	}
}

func setupBusinessChainFixture(t *testing.T, targetURL string, manifest RunnerManifest) (executionFixture, string) {
	t.Helper()
	graph, err := loadGraph(writeBusinessModel(t, businessChainModelJSON()))
	if err != nil {
		t.Fatalf("load businessChainModel: %v", err)
	}
	fixture := prepareExecutionFixture(t, graph, targetURL)
	t.Setenv("ARC_TEST_PROVIDER_KEY", "provider-secret")
	t.Setenv("ARC_TEST_WORKER_KEY", "worker-secret")
	if _, err := fixture.d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: "fixture.caller", Kind: "account", State: "ACTIVE", Refs: map[string]string{"role": "ROLE-CALLER", "principal": "principal-caller", "environment": "independent"}},
		{ID: "fixture.provider", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_TEST_PROVIDER_KEY", Refs: map[string]string{"role": "ROLE-PROVIDER", "principal": "principal-provider", "environment": "independent"}},
		{ID: "fixture.worker", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_TEST_WORKER_KEY", Refs: map[string]string{"role": "ROLE-WORKER", "principal": "principal-worker", "environment": "independent"}},
		{ID: "fixture.business", Kind: "business", State: "S-INITIAL", Refs: map[string]string{"environment": "independent", "scope": "pay-chain-fixture"}},
	}}); err != nil {
		t.Fatal(err)
	}
	fixture.account = "fixture.caller"
	fixture.fixture = "fixture.business"
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "chain-runner.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		t.Fatal(err)
	}
	cliJSON(t, "runner", "--db", fixture.d.file, "--file", manifestPath)
	var roundRes struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cliJSON(t, "round", "--db", fixture.d.file, "--graph", "pay-chain-v3", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &roundRes); err != nil {
		t.Fatal(err)
	}
	fixture.round = roundRes.ID
	if _, err := fixture.d.plan("pay-chain-v3", 128); err != nil {
		t.Fatal(err)
	}
	return fixture, manifestPath
}

func runBusinessChain(t *testing.T, fixture executionFixture, runnerID string) ExecuteResult {
	t.Helper()
	res, err := fixture.d.execute(context.Background(), ExecuteRequest{
		Task:        "pay-chain-v3/T-FLOW-PAY-REFUND",
		Runner:      runnerID,
		Round:       fixture.round,
		Environment: "independent",
		Commit:      fixture.commit,
		Root:        fixture.root,
		Config:      fixture.config,
		Account:     fixture.account,
		Fixture:     fixture.fixture,
		Accounts:    map[string]string{"provider": "fixture.provider", "worker": "fixture.worker"},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	return res
}

func TestBusinessChainMultiActorExtractSessionAsyncAndExplain(t *testing.T) {
	srv, server := newBusinessChainServer(t)
	fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("chain-e2e-runner"))
	result := runBusinessChain(t, fixture, "chain-e2e-runner")
	if result.Verdict != "PASS" || result.Recovery.Verdict != "PASS" {
		t.Fatalf("expected PASS execute + recovery, got %+v", result)
	}
	if !srv.sessionCookieAccepted || srv.observedCallbackOrder != "P-1001" || srv.observedRefundOrder != "P-1001" {
		t.Fatalf("dynamic entity/session did not flow across actors: %+v", srv)
	}

	explained, err := fixture.d.explainFacts("", result.Run, "", explainAllSteps)
	if err != nil {
		t.Fatalf("explainFacts failed: %v", err)
	}
	if explained["offline_db_complete"] != true {
		t.Fatalf("expected offline_db_complete=true, got %v", explained["offline_db_complete"])
	}
	steps := explained["steps"].([]ExplainStep)
	if len(steps) != 6 { // setup + 4 sequence steps + recovery
		t.Fatalf("expected 6 lifecycle step_facts (setup + 4 actions + recovery), got %d", len(steps))
	}
	entities := explained["entities"].([]EntityBinding)
	foundPay, foundRefund := false, false
	for _, ent := range entities {
		if ent.Alias == "current_payment" && ent.ID == "P-1001" {
			foundPay = true
		}
		if ent.Alias == "current_refund" && ent.ID == "R-2001" {
			foundRefund = true
		}
	}
	if !foundPay || !foundRefund {
		t.Fatalf("expected P-1001 and R-2001 entity bindings in DB, got %+v", entities)
	}
	// Check step 3 recorded bounded AsyncWait observation and assertions
	if len(steps[3].Waits) != 1 || steps[3].Waits[0].TimedOut || steps[3].Waits[0].ViolatedIntermediate || steps[3].Waits[0].FinalState != "S-NOTIFIED" {
		t.Fatalf("expected converged async wait on step 3, got %+v", steps[3].Waits)
	}
	if len(steps[4].Actions) != 1 || !strings.Contains(steps[4].Actions[0].ResolvedPath, "P-1001") || steps[4].Actions[0].TemplateSHA256 == "" {
		t.Fatalf("expected resolved path & template digest on step 4, got %+v", steps[4].Actions)
	}

	// Query by --graph and --node through CLI
	nodeOut := cliJSON(t, "explain", "--db", fixture.d.file, "--graph", "pay-chain-v3", "--node", "A-REFUND-CREATE")
	if !strings.Contains(string(nodeOut), "R-2001") || !strings.Contains(string(nodeOut), "ROLE-CALLER") {
		t.Fatalf("explain by graph+node missing expected details: %s", nodeOut)
	}

	// Copy ONLY SQLite DB to an isolated temp directory (ZERO raw/ directory) and verify offline self-containment
	rawDBBytes, err := os.ReadFile(fixture.d.file)
	if err != nil {
		t.Fatal(err)
	}
	// Ensure no raw/ leaks into isolated copy directory
	isoDir := t.TempDir()
	isoDBPath := filepath.Join(isoDir, "offline.db")
	if err := os.WriteFile(isoDBPath, rawDBBytes, 0600); err != nil {
		t.Fatal(err)
	}
	isoDB, err := openDB(isoDBPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer isoDB.close()

	isoExplain, err := isoDB.explainFacts("", result.Run, "", explainAllSteps)
	if err != nil || isoExplain["offline_db_complete"] != true {
		t.Fatalf("offline copied DB explain failed: %+v %v", isoExplain, err)
	}
	isoTrace, err := isoDB.trace(result.Run)
	if err != nil || isoTrace.Status != "PASS" || len(isoTrace.Steps) != 4 {
		t.Fatalf("offline copied DB trace failed: %+v %v", isoTrace, err)
	}
	if err := isoDB.qualifyModelRun(result.Run); err != nil {
		t.Fatalf("offline copied DB qualifyModelRun failed: %v", err)
	}
	assetsDir := filepath.Join("workbench", "dist")
	if _, statErr := os.Stat(assetsDir); statErr == nil {
		snapOut := filepath.Join(t.TempDir(), "snap")
		if _, err := isoDB.writeSnapshot(SnapshotRequest{Graph: "pay-chain-v3", Round: fixture.round, Assets: assetsDir, Out: snapOut}); err != nil {
			t.Fatalf("offline copied DB snapshot failed: %v", err)
		}
	}
	// Security sanity: persisted facts_json must not leak raw secret values
	if strings.Contains(string(nodeOut), "provider-secret") || strings.Contains(string(nodeOut), "worker-secret") || strings.Contains(string(nodeOut), "sess-caller-001") {
		t.Fatalf("explain output leaked secret or raw session value: %s", nodeOut)
	}
}

func TestBusinessChainNegativeSensitivity(t *testing.T) {
	t.Run("missing_required_extract", func(t *testing.T) {
		srv, server := newBusinessChainServer(t)
		srv.omitOrderExtract = true
		fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("chain-missing-extract"))
		res := runBusinessChain(t, fixture, "chain-missing-extract")
		if res.Verdict != "BLOCKED" || srv.providerActionsCalled != 0 {
			t.Fatalf("missing required extract should block before step 2: %+v providerCalled=%d", res, srv.providerActionsCalled)
		}
	})

	t.Run("async_wait_illegal_intermediate_state", func(t *testing.T) {
		srv, server := newBusinessChainServer(t)
		srv.illegalAsyncState = "S-DEAD-LETTER"
		fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("chain-illegal-async"))
		res := runBusinessChain(t, fixture, "chain-illegal-async")
		if res.Verdict != "FAIL" || srv.refundActionsCalled != 0 {
			t.Fatalf("illegal async intermediate state should FAIL immediately before step 4: %+v refundCalled=%d", res, srv.refundActionsCalled)
		}
		tr, err := fixture.d.trace(res.Run)
		if err != nil || tr.FirstDivergence != "3" {
			t.Fatalf("expected first_divergence=3, got %+v err=%v", tr, err)
		}
		exp, err := fixture.d.explainFacts("", res.Run, "", 3)
		if err != nil {
			t.Fatal(err)
		}
		steps := exp["steps"].([]ExplainStep)
		if len(steps) != 1 || steps[0].Verdict != "FAIL" || !steps[0].FirstDivergence {
			t.Fatalf("explain step 3 did not record first_divergence FAIL: %+v", steps)
		}
	})

	t.Run("async_wait_timeout", func(t *testing.T) {
		srv, server := newBusinessChainServer(t)
		srv.neverConvergeAsync = true
		m := businessChainManifest("chain-timeout-async")
		m.Steps[2].AsyncWait.MaxWaitMS = 120
		fixture, _ := setupBusinessChainFixture(t, server.URL, m)
		res := runBusinessChain(t, fixture, "chain-timeout-async")
		if res.Verdict != "FAIL" || srv.refundActionsCalled != 0 {
			t.Fatalf("async timeout should FAIL cleanly before step 4: %+v refundCalled=%d", res, srv.refundActionsCalled)
		}
	})

	t.Run("tampered_entity_identity_after_step", func(t *testing.T) {
		srv, server := newBusinessChainServer(t)
		srv.corruptRefundAccount = true
		fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("chain-tampered-acct"))
		res := runBusinessChain(t, fixture, "chain-tampered-acct")
		if res.Verdict != "FAIL" {
			t.Fatalf("corrupted stored_account should fail invariant/check, got %+v", res)
		}
	})

	t.Run("tampered_resolved_path_fails_verification", func(t *testing.T) {
		_, server := newBusinessChainServer(t)
		fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("chain-tampered-tpl"))
		res := runBusinessChain(t, fixture, "chain-tampered-tpl")
		if res.Verdict != "PASS" {
			t.Fatalf("expected initial PASS, got %+v", res)
		}
		contract, err := fixture.d.runTaskContract(res.Run)
		if err != nil {
			t.Fatal(err)
		}
		step4, hash4, err := fixture.d.workbenchEvidence(res.Run, "action", 4)
		if err != nil {
			t.Fatal(err)
		}
		_ = hash4
		// Mutate ResolvedPath without changing TemplateSHA256 or RequestSHA256: verifier must catch mismatch
		step4.Model.Actions[0].ResolvedPath = "/api/v1/payments/P-9999/refunds"
		verdict, reason, _, err := fixture.d.evaluateModelObservation(contract, step4, 4, step4.Model.Actions[0].RequestSHA256)
		if err == nil && verdict == "PASS" {
			t.Fatalf("tampered ResolvedPath was not rejected: verdict=%s reason=%s", verdict, reason)
		}
	})

	t.Run("explore_unifies_into_step_facts", func(t *testing.T) {
		_, server := newBusinessChainServer(t)
		fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("chain-explore"))
		adapter := ExploreAdapter{
			SchemaVersion:  1,
			Graph:          "pay-chain-v3",
			IsolationScope: "pay-chain-fixture",
			TimeoutMS:      8000,
			StepTimeoutMS:  2000,
			Parallelism:    1,
			Accounts:       map[string]string{"ROLE-CALLER": fixture.account},
			Oracle: OracleAdapter{
				ID:            "pg-independent-store",
				Boundary:      "after explore action",
				Call:          AdapterCall{Kind: adapterHTTP, Method: http.MethodGet, Path: "/oracle/baseline", SuccessStatuses: []int{200}},
				StatePath:     "/state",
				PrincipalPath: "/principal",
				RolePath:      "/role",
				ScopePath:     "/scope",
				Conditions:    map[string]string{"C-ACCOUNT-ACTIVE": "/conditions/C-ACCOUNT-ACTIVE"},
				Measurements:  map[string]string{"stored_account": "/measurements/stored_account", "original_account": "/measurements/original_account"},
			},
			Recovery: &FixtureRecovery{
				IsolationScope: "pay-chain-fixture",
				Steps:          []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/reset", SuccessStatuses: []int{200}}}},
				Oracle: OracleAdapter{
					ID:            "pg-independent-store",
					Boundary:      "after reset",
					Call:          AdapterCall{Kind: adapterHTTP, Method: http.MethodGet, Path: "/oracle/baseline", SuccessStatuses: []int{200}},
					StatePath:     "/state",
					PrincipalPath: "/principal",
					RolePath:      "/role",
					ScopePath:     "/scope",
					Conditions:    map[string]string{"C-ACCOUNT-ACTIVE": "/conditions/C-ACCOUNT-ACTIVE"},
					Measurements:  map[string]string{"stored_account": "/measurements/stored_account", "original_account": "/measurements/original_account"},
				},
				State:      "S-INITIAL",
				Conditions: ConditionAssignment{"C-ACCOUNT-ACTIVE": true},
				Checks:     []PropertyCheck{{Left: "stored_account", Operator: "eq", Right: "original_account"}},
			},
			Entries: []ExploreEntry{
				{
					Action: "A-PAY-CREATE",
					Entry:  "/api/v1/payments",
					Method: http.MethodPost,
					Roles:  []string{"ROLE-CALLER"},
				},
			},
		}
		report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
			Graph: "pay-chain-v3", Flow: "FLOW-PAY-CALLBACK-NOTIFY-REFUND", Environment: "independent", Round: fixture.round,
			Root: fixture.root, Config: fixture.config, Commit: fixture.commit, Account: fixture.account, Fixture: fixture.fixture,
			Adapter: adapter,
			Budget:  ExploreRuntimeBudget{MaxStates: 6, MaxPaths: 4, MaxActions: 2, Deadline: 8 * time.Second},
		})
		if err != nil || report.Actions == 0 {
			t.Fatalf("exploration failed: %+v err=%v", report, err)
		}
		verdicts, err := fixture.d.query("SELECT verdict FROM exploration_actions WHERE campaign="+quote(report.Campaign)+" ORDER BY seq LIMIT 10", 1)
		if err != nil || len(verdicts) == 0 {
			t.Fatalf("exploration_actions missing: %v", err)
		}
		for _, v := range verdicts {
			if v[0] != "PASS" && v[0] != "FAIL" && v[0] != "BLOCKED" {
				t.Fatalf("exploration_actions.verdict stored non-verdict value %q", v[0])
			}
		}
		exp, err := fixture.d.explainFacts("", report.Campaign, "", explainAllSteps)
		if err != nil {
			t.Fatal(err)
		}
		if steps := exp["steps"].([]ExplainStep); len(steps) == 0 || steps[0].SourceKind != "explore" {
			t.Fatalf("explore steps did not unify into step_facts: %+v", exp)
		}
	})
}
