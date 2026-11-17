package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func campaignTestGraph(t *testing.T, preparation bool) Graph {
	t.Helper()
	g := runtimeBusinessGraph(t)

	if preparation {
		active := businessPredicate("eq", "account", "active", "boolean", "true")
		g.Nodes = append(g.Nodes, Node{ID: "activate", Kind: "action", Name: "激活隔离测试账号"})
		g.Edges = append(g.Edges, Edge{ID: "impl-activate", From: "enrollment.submit", To: "activate", Kind: "implements"})
		g.BusinessContract.Preparation = []PreparationAction{{ID: "activate-account", Action: "activate", Requires: businessNot(active), Ensures: active, Effects: []PreparationEffect{{Subject: "account", Attribute: "active", Value: json.RawMessage(`true`)}}, Source: "document", SourceRef: "synthetic-campaign-spec"}}
	}
	return g
}

func campaignTestManifest(t *testing.T, id string, preparation bool) RunnerManifest {
	t.Helper()
	body, err := os.ReadFile(fixtureManifest(t, id, "R1/POSITIVE"))
	if err != nil {
		t.Fatal(err)
	}
	var m RunnerManifest
	if err := decodeStrictJSON(body, &m); err != nil {
		t.Fatal(err)
	}
	m.Steps[0].Transition = "submit-flow"
	m.Steps[0].BusinessBranch = "accept"
	m.Steps[0].SubjectBindings = map[string]string{"account": "actor.primary.principal"}
	m.Steps[0].Oracles[0].Measurements["account.active"] = "/active"
	m.Steps[0].Oracles[0].Measurements["order.stored"] = "/stored"
	m.Steps[0].Call.Body = json.RawMessage(fmt.Sprintf(`{"variant":%q}`, id))
	if preparation {
		observer := m.Steps[0]
		observer.BusinessBranch = ""
		active := businessPredicate("eq", "account", "active", "boolean", "true")
		m.Preparation = &RunnerPreparation{Goal: active, MaxSteps: 1, MaxStates: 8, Observer: observer, Adapters: map[string]RunnerStep{"activate-account": {Node: "activate", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/activate", SuccessStatuses: []int{200}}}}}
	}
	return m
}

func newCampaignTestFixture(t *testing.T, g Graph, targetURL string, manifests ...RunnerManifest) (executionFixture, BusinessCampaignRequest) {
	t.Helper()
	f := prepareExecutionFixture(t, g, targetURL)
	if err := f.d.tx(businessCampaignSchema); err != nil {
		t.Fatal(err)
	}
	variants := []BusinessCampaignVariant{}
	for i, m := range manifests {
		if _, err := f.d.registerRunner(m); err != nil {
			t.Fatalf("register %s: %v", m.ID, err)
		}
		variants = append(variants, BusinessCampaignVariant{ID: fmt.Sprintf("v%d", i+1), Branch: m.Steps[0].BusinessBranch, Runner: m.ID})
	}
	round, err := f.d.createRound(g.ID, "independent", f.commit, f.root, f.config)
	if err != nil {
		t.Fatal(err)
	}
	f.round = round.ID
	return f, BusinessCampaignRequest{Graph: g.ID, Flow: "flow.enrollment", Environment: "independent", Round: round.ID, Commit: f.commit, Root: f.root, Config: f.config, Account: f.account, Fixture: f.fixture, Variants: variants, Budget: BusinessCampaignBudget{MaxActions: 16, MaxStates: 64, DeadlineMS: 120000}}
}

func TestBusinessCampaignMultipleRegisteredVariantsAndRealPreparation(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, false)
	g := campaignTestGraph(t, true)
	f, request := newCampaignTestFixture(t, g, server.URL, campaignTestManifest(t, "variant.b", true), campaignTestManifest(t, "variant.a", true))
	target.ledger = &f.d
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || result.Status != "COMPLETED_IN_SCOPE" || !result.Coverage.SelectedVariantsCovered || len(result.Visits) != 2 {
		t.Fatalf("campaign incomplete: status=%s reason=%s visits=%+v frontier=%+v err=%v", result.Status, result.Reason, result.Visits, result.Frontier, err)
	}
	if target.requests != 2 || target.preparations != 2 || !target.preFrozen || result.Spent.TargetActions != 2 || result.Spent.PreparationActions != 2 || result.Spent.RecoveryActions != 2 || result.Spent.Actions != 6 || result.Spent.ReservedActions != 0 {
		t.Fatalf("actual request/preparation/budget facts wrong: campaign=%+v target=%+v", result.Spent, target)
	}
	if result.Visits[0].Run == result.Visits[1].Run || result.Visits[0].Preparation == nil || !result.Visits[0].Preparation.TargetReady || result.Visits[0].Before.Values["account.active"] == nil {
		t.Fatal("campaign did not preserve separate actual prepared child runs")
	}
	loaded, err := f.d.loadBusinessCampaign(result.ID)
	if err != nil || loaded.EvidenceStatus != "PASS" || loaded.Status != "COMPLETED_IN_SCOPE" {
		t.Fatalf("loaded facts are not independently sound: %+v %v", loaded, err)
	}
	if _, err := f.d.retry(result.Variants[0].Task, "ordinary retry must not reopen PASS", "", "", "", ""); err == nil {
		t.Fatal("campaign hook weakened ordinary PASS retry")
	}
}

func TestBusinessCampaignWrongInputOverlapAndMissingAccountIssueZeroTargetRequests(t *testing.T) {
	for _, scenario := range []string{"overlap", "wrong_input", "principal"} {
		t.Run(scenario, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			g := campaignTestGraph(t, false)
			m := campaignTestManifest(t, "variant", false)
			if scenario == "overlap" {
				duplicate := g.BusinessContract.Branches[0]
				duplicate.ID = "ambiguous"
				g.BusinessContract.Branches = append(g.BusinessContract.Branches, duplicate)
			}
			if scenario == "wrong_input" {
				g.BusinessContract.Subjects = append(g.BusinessContract.Subjects, BusinessSubject{ID: "request", Kind: "environment", Attributes: []BusinessAttribute{{Name: "amount", Type: "integer", Encoding: "json_i64", SourceRef: "synthetic-campaign-spec:strict-json-i64"}}})
				g.BusinessContract.Branches[0].When = BusinessPredicate{Op: "and", Args: []BusinessPredicate{g.BusinessContract.Branches[0].When, businessPredicate("eq", "request", "amount", "integer", "100")}}
				m.Steps[0].InputMeasurements = map[string]string{"request.amount": "/amount"}
				m.Steps[0].Call.Body = json.RawMessage(`{"amount":100.0}`)
			}
			f, request := newCampaignTestFixture(t, g, server.URL, m)
			if scenario == "principal" {
				// Keep the frozen account valid but deliberately point its reader
				// at another principal's actual environment.
				if _, err := f.d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "fixture.other", Kind: "account", State: "ACTIVE", Refs: map[string]string{"role": "STUDENT", "principal": "somebody-else", "environment": "independent"}}}}); err != nil {
					t.Fatal(err)
				}
				request.Account = "fixture.other"
				round, err := f.d.createRound(g.ID, "independent", f.commit, f.root, f.config)
				if err != nil {
					t.Fatal(err)
				}
				request.Round = round.ID
			}
			result, err := f.d.runBusinessCampaign(context.Background(), request)
			if err != nil || len(result.Frontier) != 1 || result.Frontier[0].Observation == nil {
				t.Fatalf("test never reached the real campaign probe: %+v %v", result, err)
			}
			if scenario == "wrong_input" && (result.Frontier[0].Decision == nil || result.Frontier[0].Decision.Status != "BLOCKED") {
				t.Fatalf("strict input did not reach typed decision: %+v", result.Frontier[0])
			}
			if scenario == "principal" && !strings.Contains(result.Frontier[0].Reason, "principal differs") {
				t.Fatalf("wrong principal did not reach independent identity read: %+v", result.Frontier[0])
			}
			if target.requests != 0 || target.preparations != 0 || result.Status == "COMPLETED_IN_SCOPE" {
				t.Fatalf("unsafe branch sent target: %+v err=%v count=%d", result, err, target.requests)
			}
		})
	}
}

func TestBusinessCampaignPreparationPredictionCannotAuthorizeTarget(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, false)
	target.lyingPreparation = true
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, true), server.URL, campaignTestManifest(t, "lying", true))
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || result.Status != "BLOCKED" || target.requests != 0 || target.preparations != 1 || len(result.Visits) != 1 || result.Visits[0].Preparation == nil || result.Visits[0].Preparation.TargetReady {
		t.Fatalf("planned state became actual: %+v err=%v requests=%d prep=%d", result, err, target.requests, target.preparations)
	}
}

func TestBusinessCampaignBudgetReservesPreparationAndRecoveryBeforeAnyWrite(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, false)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, true), server.URL, campaignTestManifest(t, "budget", true))
	request.Budget.MaxActions = 2 // target + preparation + restore needs three
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || result.Status != "BUDGET" || target.requests != 0 || target.preparations != 0 || len(result.Visits) != 0 {
		t.Fatalf("budget did not fence all writes: %+v %v", result, err)
	}
}
