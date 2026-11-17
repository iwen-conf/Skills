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

type campaignStateTarget struct {
	instanceID                          string
	mu                                  sync.Mutex
	active, stored, flipAfterCheckpoint bool
	readyReads, preparations            int
	actions                             []string
}

func newCampaignStateTarget(t *testing.T, active, flip bool) (*campaignStateTarget, *httptest.Server) {
	t.Helper()
	target := &campaignStateTarget{instanceID: newID("synthetic-instance"), active: active, flipAfterCheckpoint: flip, actions: []string{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target.mu.Lock()
		defer target.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/activate":
			target.active = true
			target.preparations++
			fmt.Fprint(w, `{"code":"ACTIVATED"}`)
		case "/enroll":
			if !campaignStateRequestHasInstance(t, r, target.instanceID) {
				w.WriteHeader(409)
				fmt.Fprint(w, `{}`)
				return
			}
			target.actions = append(target.actions, "enroll")
			if !target.active {
				w.WriteHeader(401)
				fmt.Fprint(w, `{"code":"INACTIVE"}`)
				return
			}
			target.stored = true
			target.readyReads = 0
			fmt.Fprint(w, `{"code":"OK"}`)
		case "/clear":
			if !campaignStateRequestHasInstance(t, r, target.instanceID) {
				w.WriteHeader(409)
				fmt.Fprint(w, `{}`)
				return
			}
			target.actions = append(target.actions, "clear")
			target.stored = false
			fmt.Fprint(w, `{"code":"CLEARED"}`)
		case "/reset":
			target.actions = append(target.actions, "unexpected-restore")
			target.stored = false
			fmt.Fprint(w, `{}`)
		case "/oracle":
			state, enrolled := "pending", 0
			if target.stored {
				state, enrolled = "ready", 1
				target.readyReads++
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":1,"active":%t,"stored":%t,"entity_id":%q}`, state, enrolled, target.active, target.stored, target.instanceID)
			// An external account event happens after the first child's target
			// read and retained-checkpoint read. The next probe must see it.
			if target.flipAfterCheckpoint && target.stored && target.readyReads == 2 {
				target.active = false
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return target, server
}

func campaignCycleGraph(t *testing.T, preparation bool) Graph {
	t.Helper()
	g := campaignTestGraph(t, preparation)
	for i := range g.BusinessContract.Subjects {
		if g.BusinessContract.Subjects[i].ID == "order" {
			g.BusinessContract.Subjects[i].RequiresBinding = true
			g.BusinessContract.Subjects[i].Attributes = append(g.BusinessContract.Subjects[i].Attributes, BusinessAttribute{Name: "id", Type: "string"})
		}
	}
	g.Nodes = append(g.Nodes, Node{ID: "clear", Kind: "action", Name: "清除隔离报名实例"})
	g.Edges = append(g.Edges, Edge{ID: "requires-clear-login", From: "login", To: "clear", Kind: "requires"}, Edge{ID: "implements-clear", From: "enrollment.submit", To: "clear", Kind: "implements"}, Edge{ID: "clears-pending", From: "clear", To: "pending", Kind: "transitions"}, Edge{ID: "clear-flow", From: "ready", To: "pending", Kind: "transitions_to"})
	g.Rules = append(g.Rules, Rule{ID: "R2", Action: "clear", Actor: "STUDENT", Entry: "/clear", Oracle: "fixture-independent-store", Conditions: []string{"login"}, Expected: "pending", Rejected: "UNCHANGED", Source: "document", Description: "Synthetic fixture clear for state-cycle acceptance"})
	g.Transitions = append(g.Transitions, TransitionContract{Edge: "clear-flow", Action: "clear", Rule: "R2", Preconditions: ConditionAssignment{"login": true}, Outcome: branchSuccess})
	active := businessPredicate("eq", "account", "active", "boolean", "true")
	for _, enabled := range []bool{true, false} {
		when, id := active, "clear-active"
		if !enabled {
			when, id = businessNot(active), "clear-inactive"
		}
		g.BusinessContract.Branches = append(g.BusinessContract.Branches, BusinessBranch{ID: id, Action: "clear", Rule: "R2", Flow: "flow.enrollment", Transition: "clear-flow", FromStates: []string{"ready"}, When: when, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "cleared", State: "pending", Denied: businessBool(false), Predicate: businessPredicate("eq", "order", "stored", "boolean", "false"), NextAllowed: []string{"enroll"}}}}, Source: "document", SourceRef: "synthetic-campaign-state-cycle", Confirmed: true})
	}
	for i := range g.BusinessContract.Branches {
		if g.BusinessContract.Branches[i].Action == "enroll" {
			g.BusinessContract.Branches[i].Flow = "flow.enrollment"
			g.BusinessContract.Branches[i].Expected.Outcomes[0].NextAllowed = []string{"clear"}
		}
	}
	return g
}

func campaignCheckpointManifest(t *testing.T, id, branch string, preparation bool) RunnerManifest {
	t.Helper()
	m := campaignTestManifest(t, id, preparation)
	m.Steps[0].SubjectBindings["order"] = "entity.instance.id"
	m.Steps[0].Oracles[0].Measurements["order.id"] = "/entity_id"
	m.Steps[0].Call.Body = json.RawMessage(fmt.Sprintf(`{"variant":%q,"entity":"{{var.instance_id}}"}`, id))
	m.Steps[0].Call.Extract = []ExtractRule{{Name: "instance_id", Source: "oracle", OracleID: "fixture-independent-store", Path: "/entity_id", EntityKind: "enrollment_instance", EntityAlias: "instance", Required: true}}
	m.Recovery.Mode = recoveryCheckpoint
	m.Recovery.Steps = []RecoveryStep{}
	if strings.HasPrefix(branch, "clear-") {
		m.Test = "R2/POSITIVE"
		m.Steps[0].Node = "clear"
		m.Steps[0].Transition = "clear-flow"
		m.Steps[0].Call.Path = "/clear"
		m.Preparation = nil
		m.Recovery.State = "pending"
		m.Recovery.Checks = []PropertyCheck{{Left: "enrolled", Operator: "eq", Value: json.RawMessage(`0`)}}
	} else {
		m.Recovery.State = "ready"
		m.Recovery.Checks = []PropertyCheck{{Left: "enrolled", Operator: "eq", Value: json.RawMessage(`1`)}}
	}
	m.Steps[0].BusinessBranch = branch
	return m
}

func TestBusinessCampaignExploresActualStateCycleAcrossCheckpointChildren(t *testing.T) {
	target, server := newCampaignStateTarget(t, false, false)
	g := campaignCycleGraph(t, true)
	// Deliberately reverse state order: clear is listed first but cannot run
	// in pending. The second enrollment variant becomes relevant only after a
	// real clear has independently returned the instance to pending.
	f, request := newCampaignTestFixture(t, g, server.URL, campaignCheckpointManifest(t, "clear", "clear-active", false), campaignCheckpointManifest(t, "enroll-a", "accept", true), campaignCheckpointManifest(t, "enroll-b", "accept", true))
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || result.Status != "COMPLETED_IN_SCOPE" || len(result.Visits) != 3 {
		t.Fatalf("state frontier did not complete: %+v %v", result, err)
	}
	if got := strings.Join(target.actions, ","); got != "enroll,clear,enroll" {
		t.Fatalf("executed a fixed variant order instead of actual-state frontier: %s", got)
	}
	if target.preparations != 1 || result.Spent.RecoveryActions != 0 || result.Spent.Actions != 4 || result.Visits[0].VariantID != "v2" || result.Visits[1].VariantID != "v1" || result.Visits[2].VariantID != "v3" {
		t.Fatalf("checkpoint/preparation/variant scope wrong: %+v", result)
	}
	if result.Visits[0].Before.State != "pending" || result.Visits[1].Before.State != "ready" || result.Visits[2].Before.State != "pending" || result.Visits[0].BeforeStateKey == result.Visits[2].BeforeStateKey {
		t.Fatal("cycle context was collapsed or inferred from expected states")
	}
	for _, visit := range result.Visits {
		if visit.Before.Bindings["order"] != target.instanceID || visit.After.Bindings["order"] != target.instanceID || len(visit.Entities) != 1 || visit.Entities[0].ID != target.instanceID {
			t.Fatalf("checkpoint child did not observe the same entity ID: %+v", visit)
		}
	}
	loaded, err := f.d.loadBusinessCampaign(result.ID)
	if err != nil || loaded.EvidenceStatus != "PASS" || !loaded.Coverage.SelectedVariantsCovered {
		t.Fatalf("cycle evidence not sound: %+v %v", loaded, err)
	}
}

func TestBusinessCampaignExternalEventReprobesConditionsAtSameState(t *testing.T) {
	target, server := newCampaignStateTarget(t, true, true)
	g := campaignCycleGraph(t, false)
	f, request := newCampaignTestFixture(t, g, server.URL, campaignCheckpointManifest(t, "clear-active", "clear-active", false), campaignCheckpointManifest(t, "clear-inactive", "clear-inactive", false), campaignCheckpointManifest(t, "enroll", "accept", false))
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || len(result.Visits) != 2 || result.Visits[0].VariantID != "v3" || result.Visits[1].VariantID != "v2" || result.Status == "COMPLETED_IN_SCOPE" {
		t.Fatalf("stale after state/guard selected next action: %+v %v", result, err)
	}
	if result.Visits[0].After.State != "ready" || string(result.Visits[0].After.Values["account.active"]) != "true" || result.Visits[1].Before.State != "ready" || string(result.Visits[1].Before.Values["account.active"]) != "false" {
		t.Fatal("external condition change at identical state was not independently observed")
	}
	if strings.Join(target.actions, ",") != "enroll,clear" {
		t.Fatal(target.actions)
	}
}

func TestBusinessCampaignCyclesConsumeGlobalActionAndStateBudgets(t *testing.T) {
	for _, kind := range []string{"actions", "states"} {
		t.Run(kind, func(t *testing.T) {
			target, server := newCampaignStateTarget(t, true, false)
			f, request := newCampaignTestFixture(t, campaignCycleGraph(t, false), server.URL, campaignCheckpointManifest(t, "clear", "clear-active", false), campaignCheckpointManifest(t, "enroll-a", "accept", false), campaignCheckpointManifest(t, "enroll-b", "accept", false))
			if kind == "actions" {
				request.Budget.MaxActions = 2
			} else {
				request.Budget.MaxStates = 7
			}
			result, err := f.d.runBusinessCampaign(context.Background(), request)
			if err != nil || result.Status != "BUDGET" || result.Spent.Actions > request.Budget.MaxActions || result.Spent.States > request.Budget.MaxStates || len(target.actions) > 2 || result.Coverage.SelectedVariantsCovered {
				t.Fatalf("cycle exceeded scope: %+v %v", result, err)
			}
		})
	}
}

func campaignStateRequestHasInstance(t *testing.T, r *http.Request, want string) bool {
	t.Helper()
	var body struct {
		Entity string `json:"entity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Entity != want {
		t.Errorf("target request did not use the independently observed instance ID: got=%q want=%q err=%v", body.Entity, want, err)
		return false
	}
	return true
}

func TestBusinessCampaignVerifiedNextAllowedRestrictsOtherwiseReadyBranch(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	target.wrongOutcome = true // explicitly model a successful no-change operation
	g := campaignTestGraph(t, false)
	g.Nodes = append(g.Nodes, Node{ID: "clear", Kind: "action", Name: "允许但受后继限制的动作"})
	g.Edges = append(g.Edges, Edge{ID: "implements-clear", From: "enrollment.submit", To: "clear", Kind: "implements"}, Edge{ID: "requires-clear-login", From: "login", To: "clear", Kind: "requires"}, Edge{ID: "clears-pending", From: "clear", To: "pending", Kind: "transitions"}, Edge{ID: "clear-flow", From: "pending", To: "pending", Kind: "transitions_to"})
	g.Rules = append(g.Rules, Rule{ID: "R2", Action: "clear", Actor: "STUDENT", Entry: "/clear", Oracle: "fixture-independent-store", Conditions: []string{"login"}, Expected: "pending", Rejected: "UNCHANGED", Source: "document", Description: "Synthetic successor restriction"})
	g.Transitions = append(g.Transitions, TransitionContract{Edge: "clear-flow", Action: "clear", Rule: "R2", Preconditions: ConditionAssignment{"login": true}, Outcome: branchSuccess})
	g.BusinessContract.Branches[0].Expected.Outcomes[0].State = "pending"
	g.BusinessContract.Branches[0].Expected.Outcomes[0].Predicate = businessPredicate("eq", "order", "stored", "boolean", "false")
	g.BusinessContract.Branches[0].Expected.Outcomes[0].NextAllowed = []string{"enroll"}
	g.BusinessContract.Branches = append(g.BusinessContract.Branches, BusinessBranch{ID: "clear", Action: "clear", Rule: "R2", FromStates: []string{"pending"}, When: businessPredicate("eq", "account", "active", "boolean", "true"), Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "cleared", State: "pending", Denied: businessBool(false), Predicate: businessPredicate("eq", "order", "stored", "boolean", "false")}}}, Source: "document", SourceRef: "synthetic-next-allowed", Confirmed: true})
	enroll := campaignTestManifest(t, "enroll", false)
	clear := campaignTestManifest(t, "clear", false)
	clear.Test = "R2/POSITIVE"
	clear.Steps[0].BusinessBranch = "clear"
	clear.Steps[0].Node = "clear"
	clear.Steps[0].Transition = "clear-flow"
	clear.Steps[0].Call.Path = "/clear"
	f, request := newCampaignTestFixture(t, g, server.URL, enroll, clear)
	request.Budget.MaxActions = 4
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || result.Status != "BUDGET" || len(result.Visits) != 2 || target.requests != 2 || result.Coverage.SelectedVariantsCovered {
		t.Fatalf("verified successor restriction was ignored: %+v %v", result, err)
	}
	for _, visit := range result.Visits {
		if visit.VariantID != "v1" {
			t.Fatalf("disallowed but otherwise READY action executed: %+v", visit)
		}
	}
	if len(result.Frontier) != 2 || result.Frontier[1].Eligibility != "NEXT_DISALLOWED" {
		t.Fatalf("frontier lost explicit restriction: %+v", result.Frontier)
	}
}
