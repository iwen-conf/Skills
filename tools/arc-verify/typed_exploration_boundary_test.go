package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func typedExplorationFixture(t *testing.T) (executionFixture, ExploreRequest, *atomic.Int64) {
	t.Helper()
	_, target := newRuntimeBusinessTarget(t, true)
	requests := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		target.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	fixture := prepareExecutionFixture(t, runtimeBusinessGraph(t), server.URL)
	fixture.openRound(t)
	body, err := os.ReadFile(fixtureManifest(t, "exploration-boundary", "R1/POSITIVE"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest RunnerManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	request := ExploreRequest{
		Graph: "course-v2", Flow: "flow.enrollment", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit, Account: fixture.account, Fixture: fixture.fixture,
		Adapter: ExploreAdapter{SchemaVersion: 1, Graph: "course-v2", IsolationScope: "course-fixture", TimeoutMS: 8000, StepTimeoutMS: 800, Parallelism: 1,
			Oracle: manifest.Steps[0].Oracles[0], Recovery: manifest.Recovery, Accounts: map[string]string{"STUDENT": fixture.account},
			Entries: []ExploreEntry{{Action: "enroll", Entry: "/enroll", Method: "POST", Roles: []string{"STUDENT"}}}},
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 1, Deadline: 8 * time.Second},
	}
	return fixture, request, requests
}

func assertTypedExplorationBlocked(t *testing.T, report ExploreReport, err error, requests *atomic.Int64) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "business_contract") || report.StopReason != "blocked" || len(report.Blocked) != 1 || report.Blocked[0].Kind != "BLOCKED" || report.Actions != 0 || len(report.Steps) != 0 || requests.Load() != 0 {
		t.Fatalf("typed model reached the legacy online interpreter: stop=%s actions=%d steps=%d requests=%d err=%v", report.StopReason, report.Actions, len(report.Steps), requests.Load(), err)
	}
	if !strings.Contains(err.Error(), "contract check") || !strings.Contains(err.Error(), "execute") || !strings.Contains(err.Error(), "run-chain") {
		t.Fatalf("blocked explorer omitted the supported execution route: %v", err)
	}
}

func TestTypedExplorationRunBlocksBeforeRequestsOrActions(t *testing.T) {
	for _, mode := range []string{"heuristic", "deterministic"} {
		t.Run(mode, func(t *testing.T) {
			fixture, request, requests := typedExplorationFixture(t)
			request.Mode = mode
			report, err := fixture.d.runExploration(t.Context(), request)
			assertTypedExplorationBlocked(t, report, err, requests)
			for _, table := range []string{"exploration_state", "exploration_actions", "fixture_locks"} {
				rows, err := fixture.d.query("SELECT COUNT(*) FROM "+table, 1)
				if err != nil || len(rows) != 1 || rows[0][0] != "0" {
					t.Fatalf("blocked new exploration changed %s: %v %v", table, rows, err)
				}
			}
		})
	}
}

func seedTypedExplorationCheckpoint(t *testing.T, fixture executionFixture, request ExploreRequest) ExploreCheckpoint {
	t.Helper()
	const campaign = "X-typed-history"
	binding, err := fixture.d.exploreBinding(request.Graph, request.Environment, request.Commit, request.Account, request.Fixture, request.Root, request.Config, request.Adapter.Oracle.ID, "explore/"+campaign)
	if err != nil {
		t.Fatal(err)
	}
	_, _, ids, err := fixture.d.exploreAccounts(request.Adapter, binding, request.Account, request.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{
		func() error { return fixture.d.persistExploreBinding(campaign, binding) },
		func() error { return fixture.d.lockExploreFixtures(campaign, ids) },
		func() error { return fixture.d.attachExploreReceipt(campaign, request.Round) },
	} {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	modelSHA, err := fixture.d.graphFingerprint(request.Graph)
	if err != nil {
		t.Fatal(err)
	}
	state := ExploreCheckpoint{Campaign: campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round,
		ModelSHA: modelSHA, SourceCommit: request.Commit, Status: exploreInterrupted, Account: request.Account, Fixture: request.Fixture,
		Mode: "deterministic", Budget: request.Budget.normalized(), Binding: binding,
		World:  ExplorePersistedWorld{State: "pending", Role: "STUDENT", Principal: "synthetic-STUDENT", Account: request.Account, Conditions: map[string]bool{"login": true}},
		Report: ExploreReport{Campaign: campaign, Graph: request.Graph, Flow: request.Flow, Environment: request.Environment, Round: request.Round},
	}
	if err := fixture.d.saveExploreCheckpoint(state); err != nil {
		t.Fatal(err)
	}
	state, err = fixture.d.loadExploreCheckpoint(campaign)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestTypedExplorationResumeBlocksBeforeClaimOrRequests(t *testing.T) {
	for _, entry := range []string{"dispatcher", "direct_resume"} {
		t.Run(entry, func(t *testing.T) {
			fixture, request, requests := typedExplorationFixture(t)
			state := seedTypedExplorationCheckpoint(t, fixture, request)
			request.Campaign, request.Mode = state.Campaign, "deterministic"
			before, _ := json.Marshal(state)
			var report ExploreReport
			var err error
			if entry == "dispatcher" {
				report, err = fixture.d.runExploration(t.Context(), request)
			} else {
				report, err = fixture.d.resumeExploration(t.Context(), request)
			}
			assertTypedExplorationBlocked(t, report, err, requests)
			after, err := fixture.d.loadExploreCheckpoint(state.Campaign)
			if err != nil {
				t.Fatal(err)
			}
			afterBytes, _ := json.Marshal(after)
			if !sameJSONValue(before, afterBytes) {
				t.Fatalf("rejecting typed dispatch rewrote or claimed historical state: before fence=%d after fence=%d", state.Fence, after.Fence)
			}
			rows, err := fixture.d.query("SELECT COUNT(*) FROM exploration_actions WHERE campaign="+quote(state.Campaign), 1)
			if err != nil || len(rows) != 1 || rows[0][0] != "0" {
				t.Fatalf("blocked resume recorded an attempted action: %v %v", rows, err)
			}
		})
	}
}
