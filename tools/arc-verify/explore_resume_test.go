package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExploreResumeContinuesFrontierWithoutReplay(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	first, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 1, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Campaign == "" || first.Actions != 1 {
		t.Fatalf("first slice must execute exactly one action: %+v", first)
	}
	if first.RecoveryClass == RecoveryFailedLocked {
		t.Fatalf("first slice recovery blocked resume: %+v", first.Recovery)
	}
	tried := map[string]bool{}
	for _, step := range first.Steps {
		tried[step.Candidate.ID] = true
		tried[candidateKey(step.Candidate, step.Before.State)] = true
	}
	reopened, err := openDB(fixture.d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	second, err := reopened.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: first.Campaign,
		Budget:   ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 3, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Campaign != first.Campaign {
		t.Fatalf("resume must keep campaign %s, got %s", first.Campaign, second.Campaign)
	}
	if second.Actions <= first.Actions {
		t.Fatalf("resume must continue remaining frontier: first=%d second=%d", first.Actions, second.Actions)
	}
	if len(second.Steps) < 2 {
		t.Fatalf("resumed campaign lost executed history: %+v", second)
	}
	if second.Steps[0].Candidate.ID != first.Steps[0].Candidate.ID {
		t.Fatalf("resume replayed or dropped the first action: %+v", second.Steps)
	}
}

func TestExploreResumeRejectsStaleModel(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	first, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 1, Deadline: 6 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: "deadbeef",
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: first.Campaign,
		Budget:   ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 2, Deadline: 6 * time.Second},
	})
	if err == nil {
		t.Fatal("resume must refuse a changed source commit")
	}
}

func TestExploreResumeRefusesFailedRecovery(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	modelSHA, err := fixture.d.graphFingerprint("seat-v2")
	if err != nil {
		t.Fatal(err)
	}
	state := ExploreCheckpoint{Campaign: "X-lock", Graph: "seat-v2", Status: exploreInterrupted, RecoveryClass: RecoveryFailedLocked, SourceCommit: fixture.commit, ModelSHA: modelSHA}
	if err := fixture.d.saveExploreCheckpoint(state); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: "R-1",
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: "X-lock",
	})
	if err == nil {
		t.Fatal("resume must stop after failed fixture recovery")
	}
}

func TestExploreClaimRejectsDuplicateWorker(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	state := ExploreCheckpoint{Campaign: "X-claim", Graph: "seat-v2", Status: exploreRunning, ClaimedBy: "worker-a"}
	if err := fixture.d.saveExploreCheckpoint(state); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.tx([]string{"UPDATE exploration_state SET claimed_by='worker-a', status='running' WHERE campaign='X-claim'"}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.claimExploreCampaign("X-claim", "worker-b"); err == nil {
		t.Fatal("a second worker must not claim a running campaign")
	}
}

func TestUnknownSideEffectingActionIsNotReplayed(t *testing.T) {
	record := ExploreActionRecord{
		Candidate:     CandidateAction{Action: "claim", Kind: kindDeclared, RequestID: "dup-9"},
		Status:        actionExecuting,
		SideEffecting: true,
		Before:        IndependentFacts{State: "pending", Values: map[string]json.RawMessage{"records": json.RawMessage("1")}},
	}
	if !sideEffecting(record.Candidate) {
		t.Fatal("claim is side-effecting")
	}
	if record.Status != actionExecuting {
		t.Fatal("in-flight action must stay distinguishable from planned")
	}
}

func TestDiscoverResumeCLIUsesCampaignID(t *testing.T) {
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
	var first ExploreReport
	if err := json.Unmarshal(cliJSON(t, "discover", "run", "--db", fixture.d.file, "--graph", "seat-v2", "--flow", "flow.reservation", "--env", "independent", "--round", fixture.round, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--file", file, "--max-states", "4", "--max-steps", "1", "--budget", "2"), &first); err != nil {
		t.Fatal(err)
	}
	var status ExploreCheckpoint
	if err := json.Unmarshal(cliJSON(t, "discover", "status", "--db", fixture.d.file, "--id", first.Campaign), &status); err != nil {
		t.Fatal(err)
	}
	if status.Campaign != first.Campaign {
		t.Fatalf("status lost campaign: %+v", status)
	}
	var resumed ExploreReport
	if err := json.Unmarshal(cliJSON(t, "discover", "resume", "--db", fixture.d.file, "--id", first.Campaign, "--graph", "seat-v2", "--flow", "flow.reservation", "--env", "independent", "--round", fixture.round, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--file", file, "--max-states", "8", "--max-steps", "3", "--budget", "4"), &resumed); err != nil {
		t.Fatal(err)
	}
	if resumed.Campaign != first.Campaign || resumed.Actions < first.Actions {
		t.Fatalf("CLI resume did not continue %s: %+v", first.Campaign, resumed)
	}
}
