package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentPlannerWaitsWithoutProposal(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	dir := t.TempDir()
	planner, err := NewAgentExplorePlanner(dir)
	if err != nil {
		t.Fatal(err)
	}
	before := seatStateCount(t, server.URL, "claims")
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode: "agent", Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 6, MaxPaths: 6, MaxActions: 3, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.StopReason != "planner_wait" || !report.Truncated {
		t.Fatalf("empty handshake must wait: %+v", report)
	}
	if report.Actions != 0 {
		t.Fatalf("wait must not dispatch: %+v", report)
	}
	if got := seatStateCount(t, server.URL, "claims"); got != before {
		t.Fatalf("agent wait dispatched a claim: before=%d after=%d", before, got)
	}
	if _, err := os.Stat(filepath.Join(dir, agentContextFile)); err != nil {
		t.Fatal("context.json must be written for the agent")
	}
	locks, err := fixture.d.query("SELECT COUNT(*) FROM fixture_locks WHERE run="+quote(report.Campaign), 1)
	if err != nil || len(locks) != 1 || locks[0][0] == "0" {
		t.Fatalf("fixture must stay locked while waiting: %v %v", locks, err)
	}
}

func TestAgentPlannerRejectsStaleProposal(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	dir := t.TempDir()
	planner, err := NewAgentExplorePlanner(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAgentPlannerProposal(dir, PlannerProposal{
		Model: "agent", Source: "agent", ForCall: 99, ContextSHA256: "deadbeef",
		Suggestions: []PlannerSuggestion{{Action: "claim", Entry: "/claim", Role: "STUDENT", Reason: "stale", Observe: "state", Risk: "low"}},
	}); err != nil {
		t.Fatal(err)
	}
	before := seatStateCount(t, server.URL, "claims")
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode: "agent", Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 6, MaxPaths: 6, MaxActions: 3, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.StopReason != "planner_wait" {
		t.Fatalf("stale proposal must not execute: %+v", report)
	}
	if got := seatStateCount(t, server.URL, "claims"); got != before {
		t.Fatalf("stale proposal dispatched a claim: %d", got)
	}
}

func TestAgentPlannerSecondTurnSeesFirstObservation(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	dir := t.TempDir()
	firstPlanner, err := NewAgentExplorePlanner(dir)
	if err != nil {
		t.Fatal(err)
	}
	waited, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode: "agent", Planner: firstPlanner,
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 3, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if waited.StopReason != "planner_wait" {
		t.Fatalf("first turn must wait: %+v", waited)
	}
	env, err := loadAgentPlannerEnvelope(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAgentPlannerProposal(dir, bindAgentProposalToEnvelope(PlannerProposal{
		Suggestions: []PlannerSuggestion{{
			Capability: "capability.reservation", Action: "claim", Entry: "/claim", Role: "STUDENT",
			Reason: "agent first claim", Observe: "state", Risk: "low",
		}},
	}, env)); err != nil {
		t.Fatal(err)
	}
	secondPlanner, err := NewAgentExplorePlanner(dir)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: waited.Campaign, Mode: "agent", Planner: secondPlanner,
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 3, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Actions < 1 {
		t.Fatalf("matching proposal must execute: %+v", resumed)
	}
	if resumed.StopReason != "planner_wait" {
		t.Fatalf("after the first observation the engine must wait for the next agent turn: %+v", resumed)
	}
	next, err := loadAgentPlannerEnvelope(dir)
	if err != nil {
		t.Fatal(err)
	}
	if next.Call < 2 {
		t.Fatalf("second context must be call 2: %+v", next)
	}
	if len(next.Context.Recent) == 0 {
		t.Fatalf("second agent turn must see the first observation: %+v", next.Context)
	}
	rows, err := fixture.d.query("SELECT body FROM exploration_planner_audit WHERE campaign="+quote(waited.Campaign), 1)
	if err != nil || len(rows) == 0 {
		t.Fatalf("agent audit missing: %v %v", rows, err)
	}
	if !containsAgentSource(rows[0][0]) {
		t.Fatalf("audit must label source=agent: %s", rows[0][0])
	}
}

func TestAgentPlannerCannotExecuteUnauthorizedEntry(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	dir := t.TempDir()
	planner, err := NewAgentExplorePlanner(dir)
	if err != nil {
		t.Fatal(err)
	}
	waited, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode: "agent", Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 2, Deadline: 6 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := loadAgentPlannerEnvelope(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAgentPlannerProposal(dir, bindAgentProposalToEnvelope(PlannerProposal{
		Suggestions: []PlannerSuggestion{{Action: "wipe", Entry: "/admin/wipe", Role: "STUDENT", Reason: "not allowed", Observe: "state", Risk: "high"}},
	}, env)); err != nil {
		t.Fatal(err)
	}
	before := seatStateCount(t, server.URL, "claims")
	next, err := NewAgentExplorePlanner(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: waited.Campaign, Mode: "agent", Planner: next,
		Budget: ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 2, Deadline: 6 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := seatStateCount(t, server.URL, "claims"); got != before {
		t.Fatalf("unauthorized agent suggestion executed: claims=%d report=%+v", got, report)
	}
}

func TestDiscoverAgentPlannerCLIWaitAndResume(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	dir := t.TempDir()
	adapter := seatExploreAdapter(t, seatIsolationScope)
	body, err := json.Marshal(adapter)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "adapter.json")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	var waited ExploreReport
	if err := json.Unmarshal(cliJSON(t, "discover", "run", "--db", fixture.d.file, "--graph", "seat-v2", "--flow", "flow.reservation", "--env", "independent", "--round", fixture.round, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--file", file, "--mode", "agent", "--planner", dir, "--max-states", "6", "--max-steps", "3", "--budget", "4"), &waited); err != nil {
		t.Fatal(err)
	}
	if waited.StopReason != "planner_wait" || waited.Campaign == "" {
		t.Fatalf("CLI agent run must wait: %+v", waited)
	}
	envBody := cliJSON(t, "discover", "context", "--db", fixture.d.file, "--id", waited.Campaign, "--planner", dir)
	var env AgentPlannerEnvelope
	if err := json.Unmarshal(envBody, &env); err != nil {
		t.Fatal(err)
	}
	proposal := bindAgentProposalToEnvelope(PlannerProposal{
		Suggestions: []PlannerSuggestion{{Action: "claim", Entry: "/claim", Role: "STUDENT", Reason: "cli agent claim", Observe: "state", Risk: "low"}},
	}, env)
	proposalFile := filepath.Join(t.TempDir(), "proposal.json")
	raw, err := json.Marshal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proposalFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(struct{}{}); err != nil {
		t.Fatal(err)
	}
	_ = cliJSON(t, "discover", "propose", "--db", fixture.d.file, "--id", waited.Campaign, "--planner", dir, "--file", proposalFile)
	var resumed ExploreReport
	if err := json.Unmarshal(cliJSON(t, "discover", "resume", "--db", fixture.d.file, "--id", waited.Campaign, "--graph", "seat-v2", "--flow", "flow.reservation", "--env", "independent", "--round", fixture.round, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--file", file, "--mode", "agent", "--planner", dir, "--max-states", "6", "--max-steps", "3", "--budget", "4"), &resumed); err != nil {
		t.Fatal(err)
	}
	if resumed.Campaign != waited.Campaign || resumed.Actions < 1 {
		t.Fatalf("CLI resume did not execute the agent proposal: %+v", resumed)
	}
}

func containsAgentSource(body string) bool {
	var doc map[string]any
	if json.Unmarshal([]byte(body), &doc) != nil {
		return false
	}
	src, _ := doc["source"].(string)
	model, _ := doc["model"].(string)
	return src == "agent" || model == "agent"
}
