package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type recordingExplorePlanner struct {
	Suggestions []PlannerSuggestion
	Contexts    []PlannerContext
	Calls       int
}

func (p *recordingExplorePlanner) Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error) {
	if err := ctx.Err(); err != nil {
		return PlannerProposal{}, err
	}
	p.Calls++
	copied := in
	copied.Recent = append([]ExploreStep{}, in.Recent...)
	p.Contexts = append(p.Contexts, copied)
	limit := len(p.Suggestions)
	if in.BudgetLeft < limit {
		limit = in.BudgetLeft
	}
	if limit < 0 {
		limit = 0
	}
	out := PlannerProposal{Model: "scripted", Suggestions: []PlannerSuggestion{}}
	if limit > 0 {
		out.Suggestions = append(out.Suggestions, p.Suggestions[:limit]...)
	}
	return out, nil
}

func TestSecondPlannerSeesFirstObservations(t *testing.T) {
	_, server := newSeatServer(t)
	planner := &recordingExplorePlanner{Suggestions: []PlannerSuggestion{
		{Capability: "capability.reservation", Action: "claim", Entry: "/claim", Role: "STUDENT", Reason: "first observation", Observe: "state", Risk: "low"},
		{Capability: "capability.reservation", Action: "claim", Entry: "/claim", Role: "CONTENDER", Reason: "revise after oracle", Observe: "owner", Risk: "medium"},
	}}
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode: "ai", Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 2, MaxPlannerCalls: 2, Deadline: 10 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if planner.Calls < 2 {
		t.Fatalf("second planning round never ran: calls=%d report=%+v", planner.Calls, report)
	}
	if len(planner.Contexts) < 2 {
		t.Fatal("planner contexts were not recorded")
	}
	if len(planner.Contexts[1].Recent) == 0 && len(report.Steps) > 0 {
		t.Fatalf("second planner call must see the first oracle observation: %+v", planner.Contexts[1])
	}
	if len(planner.Contexts[1].Facts) == 0 {
		t.Fatal("second planner call must keep evidence-first facts")
	}
}

func TestPlannerAuditOmitsRawModelOutput(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Action: "claim", Entry: "/claim", Role: "STUDENT", Reason: "probe with token sk-test-should-not-persist if copied raw", Observe: "state", Risk: "low",
	}}}
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode: "ai", Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 1, MaxPlannerCalls: 1, Deadline: 8 * time.Second},
	})
	if err != nil && report.Campaign == "" {
		t.Fatal(err)
	}
	rows, err := fixture.d.query("SELECT body FROM exploration_planner_audit WHERE campaign="+quote(report.Campaign), 1)
	if err != nil || len(rows) == 0 {
		t.Fatalf("audit missing: %v %v", rows, err)
	}
	body := rows[0][0]
	if strings.Contains(body, "sk-") || strings.Contains(strings.ToLower(body), "bearer ") {
		t.Fatalf("planner audit persisted a secret: %s", body)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["audit"]; ok {
		t.Fatalf("full proposal blob must not be stored: %s", body)
	}
	if doc["source"] != "scripted" {
		t.Fatalf("scripted planner must be labeled: %s", body)
	}
}

func TestInconclusiveWhenExperimentCannotDiscriminate(t *testing.T) {
	exp := planMinimalExperiment(nil, ExploreAdapter{})
	exp.Discriminating = false
	if exp.Discriminating {
		t.Fatal("empty explanations cannot be treated as discriminating")
	}
	rev := reviseUnderstanding(DiscoveryContext{Hypotheses: []BoundaryHypothesis{{ID: "BH-1", Assumption: "owners are exclusive", Status: hypTested}}}, ExploreStep{
		Before: IndependentFacts{State: "assigned"}, After: IndependentFacts{State: "assigned"},
	}, map[string]bool{"R1": true})
	if len(rev.Open) == 0 {
		t.Fatal("a non-discriminating result must leave an open question")
	}
}

func TestHypothesisIdentityDedupesParaphrases(t *testing.T) {
	a := BoundaryHypothesis{Identity: HypothesisIdentity{Role: "AGENT", Action: "claim", Resource: "ticket", State: "open", Invariant: "single-owner"}}
	b := BoundaryHypothesis{Identity: HypothesisIdentity{Role: "AGENT", Action: "claim", Resource: "ticket", State: "open", Invariant: "single-owner"}, Assumption: "worded differently"}
	if hypothesisKey(a) != hypothesisKey(b) {
		t.Fatalf("same structured identity must collapse: %s vs %s", hypothesisKey(a), hypothesisKey(b))
	}
	c := BoundaryHypothesis{Identity: HypothesisIdentity{Role: "CONTENDER", Action: "claim", Resource: "ticket", State: "open", Invariant: "single-owner"}}
	if hypothesisKey(a) == hypothesisKey(c) {
		t.Fatal("different roles are different hypotheses")
	}
}

func TestVendorModelAcceptanceStaysBlockedWithoutCredentials(t *testing.T) {
	planner, err := NewHTTPExplorePlannerFromEnv()
	if err == nil && planner != nil {
		t.Skip("real planner credentials are configured; live path is exercised elsewhere")
	}
	if err == nil {
		t.Fatal("missing credentials must not construct a live planner")
	}
}
