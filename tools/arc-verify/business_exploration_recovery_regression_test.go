package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestBusinessCampaignPreparationRequiresReachableTargetPreconditions(t *testing.T) {
	for _, mode := range []string{"unmapped", "annotation_only", "mapped_unsatisfied", "mapped_repair"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			active, loggedIn, stored := false, false, false
			preparations, targets, restores := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/activate":
					preparations++
					active = true
					loggedIn = mode == "mapped_repair" || mode == "annotation_only"
					fmt.Fprint(w, `{"code":"ACTIVATED"}`)
				case "/enroll":
					targets++
					stored = true
					fmt.Fprint(w, `{"code":"OK"}`)
				case "/reset":
					restores++
					active, loggedIn, stored = false, false, false
					fmt.Fprint(w, `{}`)
				default:
					state, enrolled := "pending", 0
					if stored {
						state, enrolled = "ready", 1
					}
					fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":%t,"enrolled":%d,"capacity":1,"active":%t,"stored":%t}`, state, loggedIn, enrolled, active, stored)
				}
			}))
			defer server.Close()
			g := campaignTestGraph(t, true)
			m := campaignTestManifest(t, "reachable-login", true)
			m.Recovery.Conditions["login"] = false
			if mode != "unmapped" {
				g.BusinessContract.Subjects[0].Attributes = append(g.BusinessContract.Subjects[0].Attributes, BusinessAttribute{Name: "logged_in", Type: "boolean"})
				if mode == "annotation_only" {
					g.ConditionSubjects = []ConditionSubject{{Condition: "login", SubjectKind: "account", Subject: "account", Attribute: "logged_in", Operator: "eq", Expected: "true"}}
				} else {
					m.Steps[0].Oracles[0].Measurements["account.logged_in"] = "/login"
				}
				if mode != "mapped_unsatisfied" {
					g.BusinessContract.Preparation[0].Effects = append(g.BusinessContract.Preparation[0].Effects, PreparationEffect{Subject: "account", Attribute: "logged_in", Value: json.RawMessage(`true`)})
				}
			}
			f, request := newCampaignTestFixture(t, g, server.URL, m)
			c, err := f.d.runBusinessCampaign(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := f.d.loadBusinessCampaign(c.ID)
			if err != nil || loaded.EvidenceStatus != "PASS" {
				t.Fatalf("candidate cannot be independently replayed: status=%s reason=%s err=%v", loaded.Status, loaded.Reason, err)
			}
			if mode == "mapped_repair" {
				if c.Status != "COMPLETED_IN_SCOPE" || loaded.Status != "COMPLETED_IN_SCOPE" || preparations != 1 || targets != 1 || restores != 1 || len(c.Visits) != 1 || c.Frontier[0].Eligibility != "PREPARABLE" {
					t.Fatalf("explicit same-reader typed repair was blocked: status=%s reason=%s prep=%d targets=%d restores=%d", c.Status, c.Reason, preparations, targets, restores)
				}
				return
			}
			if c.Status != "BLOCKED" || preparations != 0 || targets != 0 || restores != 0 || c.Spent.Dispatches != 0 || len(c.Visits) != 0 || c.Frontier[0].Eligibility == "PREPARABLE" || !strings.Contains(c.Frontier[0].Reason, "login") {
				t.Fatalf("unrepairable precondition dispatched writes: status=%s candidate=%+v prep=%d target=%d restore=%d spent=%+v", c.Status, c.Frontier[0], preparations, targets, restores, c.Spent)
			}
			// The same guard must reject a claimed PREPARABLE projection during
			// independent history review, not only in the live scheduler.
			candidate := c.Frontier[0]
			candidate.Eligibility = "PREPARABLE"
			if err := f.d.verifyBusinessCampaignCandidate(c, g, candidate, c.HistorySHA256); err == nil {
				t.Fatal("offline verification accepted an unrepairable PREPARABLE claim")
			}
		})
	}
}

func TestBusinessCampaignPreparedConditionsCannotBorrowAnotherObservation(t *testing.T) {
	g := campaignTestGraph(t, true)
	g.BusinessContract.Subjects[0].Attributes = append(g.BusinessContract.Subjects[0].Attributes, BusinessAttribute{Name: "logged_in", Type: "boolean"})
	m := campaignTestManifest(t, "condition-identity", true)
	m.Steps[0].Oracles[0].Measurements["account.logged_in"] = "/login"
	obligation, err := validateManifest(g, m)
	if err != nil {
		t.Fatal(err)
	}
	base := ModelObservation{Oracles: []OracleObservation{{Actor: "primary", ActionIndex: 1, ID: m.Steps[0].Oracles[0].ID, Before: IndependentFacts{Conditions: ObservedConditions{"login": false}, Values: map[string]json.RawMessage{"account.logged_in": json.RawMessage(`false`)}}}}}
	before := BusinessObservation{State: "pending", Bindings: map[string]string{"account": "same-account"}, Values: map[string]json.RawMessage{"account.logged_in": json.RawMessage(`false`)}, Sources: map[string][]string{"account.logged_in": {m.Steps[0].Oracles[0].ID}}}
	predicted := preparationWithEffects(before, []PreparationEffect{{Subject: "account", Attribute: "logged_in", Value: json.RawMessage(`true`)}})
	if err := businessCampaignPreparedPreconditions(g, obligation, m.Steps[0], base, before, predicted); err != nil {
		t.Fatalf("same actor/reader/response mapping should be usable: %v", err)
	}
	for _, field := range []string{"actor", "action", "oracle", "missing_actual", "unknown_prediction", "changed_subject", "conflicting_response"} {
		t.Run(field, func(t *testing.T) {
			obs, _ := cloneBusinessJSON(base)
			projection, _ := cloneBusinessJSON(predicted)
			switch field {
			case "actor":
				obs.Oracles[0].Actor = "other-account"
			case "action":
				obs.Oracles[0].ActionIndex = 2
			case "oracle":
				obs.Oracles[0].ID = "other-reader-with-the-same-pointer"
			case "missing_actual":
				delete(obs.Oracles[0].Before.Conditions, "login")
			case "unknown_prediction":
				projection.Values["account.logged_in"] = json.RawMessage(`null`)
			case "changed_subject":
				projection.Bindings["account"] = "other-account"
			case "conflicting_response":
				obs.Oracles[0].Before.Values["account.logged_in"] = json.RawMessage(`true`)
			}
			if err := businessCampaignPreparedPreconditions(g, obligation, m.Steps[0], obs, before, projection); err == nil {
				t.Fatalf("precondition repair borrowed %s instead of its complete scoped observation", field)
			}
		})
	}
}

func TestBusinessCampaignChildRecordWriteFailurePreservesRecoverableProjection(t *testing.T) {
	for _, verdict := range []string{"PASS", "FAIL", "BLOCKED"} {
		t.Run(verdict, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			target.wrongOutcome = verdict == "FAIL"
			f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, campaignTestManifest(t, "record-write-failure", false))
			hooks := &businessCampaignHooks{afterIntent: func(BusinessCampaign, *businessCampaignExecution) error {
				if verdict == "BLOCKED" {
					target.mu.Lock()
					target.active = false
					target.mu.Unlock()
				}
				return f.d.tx([]string{`CREATE TRIGGER fail_child_record BEFORE UPDATE OF status ON business_campaign_dispatches WHEN NEW.status='RECORDED' BEGIN SELECT RAISE(ABORT,'injected child record write failure'); END`})
			}}
			c, err := f.d.runBusinessCampaignWithHooks(context.Background(), request, hooks)
			if err != nil {
				t.Fatalf("STOP itself could not be persisted: %v", err)
			}
			rows, err := f.d.query("SELECT run_id,status FROM business_campaign_dispatches WHERE campaign_id="+quote(c.ID), 2)
			if err != nil || len(rows) != 1 || rows[0][1] != "STARTED" {
				t.Fatalf("missing unresolved original dispatch: %v %v", rows, err)
			}
			child := rows[0][0]
			facts, err := f.d.verifiedRunFacts(child)
			if err != nil || facts.EvidenceStatus != "PASS" || !facts.OfflineComplete {
				t.Fatalf("test needs independently verifiable original child RAW: evidence=%s complete=%t err=%v", facts.EvidenceStatus, facts.OfflineComplete, err)
			}
			if c.Status != "UNKNOWN" || c.EvidenceStatus != "BLOCKED" || len(c.Visits) != 0 || c.Variants[0].Visits != 0 || c.Variants[0].Passes != 0 || c.Variants[0].LastRun != "" || c.Spent.Actions != 0 || c.Spent.ReservedActions != 2 || c.LastActual != nil || c.LastRun != "" || c.FirstDivergence != "" {
				t.Fatalf("failed transaction leaked a mutable child projection into STOP: %+v", c)
			}
			loaded, err := f.d.loadBusinessCampaign(c.ID)
			if err != nil || loaded.Status != "UNKNOWN" || loaded.EvidenceStatus != "BLOCKED" || len(loaded.Variants) != 1 || len(loaded.Visits) != 0 {
				t.Fatalf("rolled-back transaction poisoned checked history: status=%s reason=%s err=%v", loaded.Status, loaded.Reason, err)
			}
			if err := f.d.tx([]string{"DROP TRIGGER fail_child_record"}); err != nil {
				t.Fatal(err)
			}
			resumed, err := f.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: c.ID})
			status := map[string]string{"PASS": "COMPLETED_IN_SCOPE", "FAIL": "DIVERGED", "BLOCKED": "BLOCKED"}[verdict]
			requests := 1
			if verdict == "BLOCKED" {
				requests = 0
			}
			if err != nil || resumed.Status != status || resumed.EvidenceStatus != "PASS" || len(resumed.Visits) != 1 || resumed.Visits[0].Run != child || resumed.Visits[0].Status != verdict || resumed.Spent.ReservedActions != 0 || target.requests != requests {
				t.Fatalf("original child was lost, replayed or misclassified: status=%s evidence=%s visits=%+v targets=%d err=%v", resumed.Status, resumed.EvidenceStatus, resumed.Visits, target.requests, err)
			}
			loaded, err = f.d.loadBusinessCampaign(c.ID)
			if err != nil || loaded.Status != status || loaded.EvidenceStatus != "PASS" || len(loaded.Visits) != 1 || loaded.Visits[0].Run != child || (verdict == "PASS") != loaded.Coverage.SelectedVariantsCovered {
				t.Fatalf("reconciled child is not independently verifiable: status=%s reason=%s err=%v", loaded.Status, loaded.Reason, err)
			}
			if verdict != "PASS" && loaded.FirstDivergence != child {
				t.Fatalf("recovery erased the original divergence: %+v", loaded)
			}
		})
	}
}

func TestBusinessCampaignCheckedLoadRejectsNumericLexemeReplacement(t *testing.T) {
	for _, pair := range [][2]string{{"100", "100.0"}, {"100", "1e2"}, {"9007199254740993", "9007199254740992"}} {
		t.Run(pair[0]+"_to_"+pair[1], func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			g := campaignTestGraph(t, false)
			g.BusinessContract.Subjects = append(g.BusinessContract.Subjects, BusinessSubject{ID: "request", Kind: "environment", Attributes: []BusinessAttribute{{Name: "amount", Type: "integer", Encoding: "json_i64", SourceRef: "test:input"}}})
			g.BusinessContract.Branches[0].When = BusinessPredicate{Op: "and", Args: []BusinessPredicate{g.BusinessContract.Branches[0].When, businessPredicate("eq", "request", "amount", "integer", pair[0])}}
			m := campaignTestManifest(t, "lexical-boundary", false)
			m.Steps[0].InputMeasurements = map[string]string{"request.amount": "/amount"}
			m.Steps[0].Call.Body = json.RawMessage(`{"amount":` + pair[0] + `}`)
			f, request := newCampaignTestFixture(t, g, server.URL, m)
			c, err := f.d.runBusinessCampaign(context.Background(), request)
			if err != nil || c.Status != "COMPLETED_IN_SCOPE" || target.requests != 1 {
				t.Fatalf("control campaign failed: status=%s reason=%s requests=%d err=%v", c.Status, c.Reason, target.requests, err)
			}
			checked, err := f.d.loadBusinessCampaign(c.ID)
			if err != nil || checked.EvidenceStatus != "PASS" {
				t.Fatalf("original numeric tokens were not preserved through child commit: %+v %v", checked, err)
			}
			rewriteCampaignEventBodies(t, f.d, c.ID, func(snapshot *BusinessCampaign) {
				for i := range snapshot.Visits {
					if snapshot.Visits[i].Before != nil {
						snapshot.Visits[i].Before.Values["request.amount"] = json.RawMessage(pair[1])
					}
				}
			})
			if _, err := f.d.readBusinessCampaign(c.ID); err != nil {
				t.Fatalf("mutation was not a self-consistent event hash chain: %v", err)
			}
			changed, err := f.d.loadBusinessCampaign(c.ID)
			if err != nil || changed.Status != "BLOCKED" || changed.EvidenceStatus != "BLOCKED" || !strings.Contains(changed.Reason, "projection differs from verified child-run facts") {
				t.Fatalf("numeric token substitution bypassed independent child comparison: %+v %v", changed, err)
			}
		})
	}
}
