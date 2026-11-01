package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentPlannerImplementationRemoved(t *testing.T) {
	if _, err := os.Stat("explore_agent_planner.go"); err == nil {
		t.Fatal("explore_agent_planner.go must be deleted; Agent file handshake is not a product surface")
	}
}

func TestPlannerFromFlagsRejectsAgentMode(t *testing.T) {
	if _, err := plannerFromFlags("agent", t.TempDir()); err == nil {
		t.Fatal("plannerFromFlags must reject --mode agent")
	}
	if _, err := plannerFromFlags("AGENT", ""); err == nil {
		t.Fatal("plannerFromFlags must reject Agent mode without a handshake directory")
	}
	if _, err := plannerFromFlags("ai", ""); err == nil {
		t.Fatal("empty ai mode must not construct a model client")
	}
	planner, err := plannerFromFlags("heuristic", "")
	if err != nil || planner != nil {
		t.Fatalf("heuristic discovery must not require a planner: %v %v", planner, err)
	}
}

func TestDecodePlannerModeDoesNotKeepAgent(t *testing.T) {
	if got := decodePlannerMode("agent"); got == "agent" {
		t.Fatal("decodePlannerMode must not keep agent as a live planner mode")
	}
	if got := decodePlannerMode("agent"); got != "heuristic" {
		t.Fatalf("agent mode must fall back to heuristic, got %q", got)
	}
	if got := decodePlannerMode("ai"); got != "ai" {
		t.Fatalf("scripted ai fixture mode must remain: %q", got)
	}
}

func TestShouldAskPlannerIgnoresAgentMode(t *testing.T) {
	empty := ExploreReport{}
	if shouldAskPlanner("agent", nil, empty, exploreWorld{}) {
		t.Fatal("agent mode must not consult a planner")
	}
	if !shouldAskPlanner("ai", nil, empty, exploreWorld{}) {
		t.Fatal("scripted ai mode still asks the fixture planner on the first turn")
	}
}

func TestDiscoverRejectsAgentHandshakeCommands(t *testing.T) {
	for _, sub := range []string{"context", "propose"} {
		err := run([]string{"discover", sub, "--id", "C1"})
		if err == nil {
			t.Fatalf("discover %s must be removed", sub)
		}
		msg := err.Error()
		if !strings.Contains(msg, "usage: arc-verify discover") {
			t.Fatalf("discover %s must fail as unknown subcommand, got %v", sub, err)
		}
		if strings.Contains(msg, "context") || strings.Contains(msg, "propose") {
			t.Fatalf("usage must not advertise handshake subcommands: %s", msg)
		}
	}
}

func TestDiscoverRunRejectsAgentMode(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "data.db")
	if err := run([]string{"init", "--db", db}); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(dir, "adapter.json")
	if err := os.WriteFile(adapter, []byte(`{"accounts":{},"entries":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{
		"discover", "run",
		"--db", db,
		"--file", adapter,
		"--graph", "g",
		"--flow", "f",
		"--env", "independent",
		"--round", "r1",
		"--account", "a",
		"--fixture", "fx",
		"--commit", "deadbeef",
		"--mode", "agent",
		"--planner", dir,
	})
	if err == nil {
		t.Fatal("discover run --mode agent must fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "agent") {
		t.Fatalf("agent mode rejection must name the removed planner, got %v", err)
	}
}
