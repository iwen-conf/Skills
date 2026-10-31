package main

import (
	"strings"
	"testing"
	"time"
)

func TestPlannerSuggestionMustPassAllowList(t *testing.T) {
	adapter := ExploreAdapter{
		Accounts: map[string]string{"STUDENT": "fixture.student"},
		Entries:  []ExploreEntry{{Action: "claim", Entry: "/claim", Roles: []string{"STUDENT"}}},
	}
	if _, reason := validatePlannerSuggestion(PlannerSuggestion{Action: "claim", Entry: "/claim", Role: "STUDENT", Reason: "probe last seat"}, adapter, adapter.Accounts, 2); reason != "" {
		t.Fatal(reason)
	}
	if _, reason := validatePlannerSuggestion(PlannerSuggestion{Action: "rm", Entry: "://evil", Role: "STUDENT", Reason: "no"}, adapter, adapter.Accounts, 2); reason == "" {
		t.Fatal("unapproved HTTP or shell-like suggestions must be rejected")
	}
	if _, reason := validatePlannerSuggestion(PlannerSuggestion{Action: "claim", Entry: "/force", Role: "STUDENT", Reason: "mismatch"}, adapter, adapter.Accounts, 2); reason == "" {
		t.Fatal("entry must match an approved adapter action")
	}
	if _, reason := validatePlannerSuggestion(PlannerSuggestion{Action: "claim", Entry: "/claim", Role: "ADMIN", Reason: "wrong role"}, adapter, adapter.Accounts, 2); !strings.Contains(reason, "authorized") && !strings.Contains(reason, "allow") && reason == "" {
		t.Fatal("ADMIN is not on the claim entry")
	}
}

func TestScriptedPlannerDoesNotDecideVerdicts(t *testing.T) {
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "reservation.claim", Action: "force", Entry: "/force", Role: "STUDENT",
		Reason: "students may have a hidden force entry", Observe: "state", Risk: "high",
	}}}
	proposal, err := planner.Suggest(t.Context(), PlannerContext{BudgetLeft: 2})
	if err != nil || len(proposal.Suggestions) != 1 {
		t.Fatalf("scripted planner: %+v %v", proposal, err)
	}
	if proposal.Suggestions[0].Reason == "PASS" {
		t.Fatal("planner output is not a verdict")
	}
}

func TestExploreModeContrastRecordsPlannerSource(t *testing.T) {
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "reservation.claim", Action: "force", Entry: "/force", Role: "STUDENT",
		Reason: "role boundary after a successful claim", Observe: "state", Risk: "high",
	}}}
	run := func(mode string, planner ExplorePlanner) ExploreReport {
		t.Helper()
		_, server := newSeatServer(t)
		fixture := prepareSeatFixture(t, server.URL)
		fixture.openRound(t)
		report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
			Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
			Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
			Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
			Mode: mode, Planner: planner, Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 3, Deadline: 8 * time.Second},
		})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	heuristic := run("heuristic", nil)
	deterministic := run("deterministic", nil)
	assisted := run("ai", planner)
	if deterministic.Mode != "deterministic" || heuristic.Mode != "heuristic" || assisted.Mode != "ai" {
		t.Fatalf("mode labels: %s %s %s", deterministic.Mode, heuristic.Mode, assisted.Mode)
	}
	if !shouldAskPlanner("ai", nil, ExploreReport{Unspecified: []UnspecifiedItem{{Kind: "UNDECLARED_TRANSITION"}}}, exploreWorld{}) {
		t.Fatal("AI mode must consult the planner on unspecified or empty frontier")
	}
	if shouldAskPlanner("deterministic", nil, ExploreReport{}, exploreWorld{}) {
		t.Fatal("deterministic mode must not call the planner")
	}
}
