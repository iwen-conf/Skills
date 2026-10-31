package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverRunSubcommandExists(t *testing.T) {
	err := run([]string{"discover", "run"})
	if err != nil && strings.Contains(err.Error(), "discover <record|list|triage>") && !strings.Contains(err.Error(), "run") {
		t.Fatal("discover run is still missing; autonomous exploration cannot be only discover record")
	}
}

func seatExploreAdapter(t *testing.T, scope string) ExploreAdapter {
	t.Helper()
	oracle := seatOracle()
	oracle.Call.CredentialHeader = "X-Fixture-Actor"
	recovery := seatRecovery(true, true, false)
	recovery.Oracle.Call.CredentialHeader = "X-Fixture-Actor"
	return ExploreAdapter{
		SchemaVersion: 1, Graph: "seat-v2", IsolationScope: scope,
		TimeoutMS: 8000, StepTimeoutMS: 800, Parallelism: 2,
		Oracle: oracle, Recovery: recovery,
		Accounts: map[string]string{"STUDENT": "fixture.student", "ADMIN": "fixture.admin", "CONTENDER": "fixture.contender"},
		Entries: []ExploreEntry{
			{Action: "claim", Entry: "/claim", Method: "POST", Roles: []string{"STUDENT", "CONTENDER"}},
			{Action: "approve", Entry: "/approve", Method: "POST", Roles: []string{"ADMIN"}},
			{Action: "force", Entry: "/force", Method: "POST", Roles: []string{"STUDENT", "ADMIN"}},
		},
	}
}

func TestExploreRunSelectsAndExecutesWithoutHandwrittenPath(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 16, MaxPaths: 16, MaxActions: 6, Deadline: 8 * time.Second, MaxReplans: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Campaign == "" || report.Actions == 0 || len(report.Steps) == 0 {
		t.Fatalf("explorer did not execute any live action: %+v", report)
	}
	if report.StopReason == "" {
		t.Fatal("exploration stopped without a reason")
	}
	if report.Truncated && len(report.Remaining) == 0 {
		t.Fatal("a truncated exploration must name the untried space")
	}
	for _, step := range report.Steps {
		if step.RAW == "" || step.SHA256 == "" || step.Candidate.ID == "" {
			t.Fatalf("executed candidate lost RAW or identity: %+v", step)
		}
		if step.Candidate.Source != sourceRule && step.Candidate.Source != sourceImpl && step.Candidate.Source != sourceInfer {
			t.Fatalf("candidate source must stay explicit: %+v", step.Candidate)
		}
	}
}

func TestExploreRunKeepsInferenceOffTheRuleTable(t *testing.T) {
	target, server := newSeatServer(t)
	target.skipReview = true
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	before, err := fixture.d.query("SELECT COUNT(*) FROM rules WHERE graph='seat-v2'", 1)
	if err != nil {
		t.Fatal(err)
	}
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 4, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := fixture.d.query("SELECT COUNT(*) FROM rules WHERE graph='seat-v2'", 1)
	if err != nil || after[0][0] != before[0][0] {
		t.Fatalf("exploration rewrote the rule table: before=%v after=%v", before, after)
	}
	if report.WithinContract+len(report.Unspecified)+len(report.RuleFails)+len(report.Risks) == 0 {
		t.Fatalf("live skip-review must produce an observation: %+v", report)
	}
	for _, risk := range report.Risks {
		if risk.Kind != "RISK" {
			t.Fatalf("inference leaked out of RISK: %+v", risk)
		}
	}
}

func TestExploreRunCLIUsesAdapterNotHandwrittenRecord(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	body, err := json.Marshal(adapter)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "adapter.json")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	var report ExploreReport
	if err := json.Unmarshal(cliJSON(t, "discover", "run", "--db", fixture.d.file, "--graph", "seat-v2", "--flow", "flow.reservation", "--env", "independent", "--round", fixture.round, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--file", file, "--max-states", "8", "--max-steps", "4", "--budget", "4"), &report); err != nil {
		t.Fatal(err)
	}
	if report.Actions == 0 || report.StopReason == "" {
		t.Fatalf("CLI discover run did not explore: %+v", report)
	}
}

func TestExplorerSourceDoesNotContainSealedDefects(t *testing.T) {
	body, err := os.ReadFile("explore_runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, word := range []string{"ENROLL_SUT_FAULTS", "M-AUTH", "M-RACE", "skipReview", "unauthorizedForce", "raceNoLock", "acceptWhenClosed"} {
		if strings.Contains(src, word) {
			t.Fatalf("explorer source must not know sealed defect token %s", word)
		}
	}
}
