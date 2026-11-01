package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUnownedCheckpointCannotOverwriteNewWorker(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-unowned-bypass"
	seedExploreCampaign(t, fixture.d, campaign)
	first, err := fixture.d.claimExploreCampaign(campaign, "worker-old")
	if err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	second, err := fixture.d.claimExploreCampaign(campaign, "worker-new")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-new"}, second); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.saveExploreCheckpoint(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-old", ClaimedBy: first.Worker, Fence: first.Fence}); err == nil {
		t.Fatal("unowned checkpoint write must not cover a claimed campaign")
	}
	state, err := fixture.d.loadExploreCheckpoint(campaign)
	if err != nil || state.Checkpoint != "CK-new" || state.ClaimedBy != "worker-new" {
		t.Fatalf("old worker unowned write overwrote the new owner: %+v %v", state, err)
	}
}

func TestExpiredLeaseRejectsOwnedCheckpointBeforeTakeover(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-lease-expire"
	seedExploreCampaign(t, fixture.d, campaign)
	lease, err := fixture.d.claimExploreCampaign(campaign, "worker-slow")
	if err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-late"}, lease); err == nil {
		t.Fatal("an expired lease must not commit even before another worker takes over")
	}
}

func TestExpiredLeaseCannotDispatchAnotherAction(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-lease-dispatch"
	seedExploreCampaign(t, fixture.d, campaign)
	lease, err := fixture.d.claimExploreCampaign(campaign, "worker-dispatch")
	if err != nil {
		t.Fatal(err)
	}
	state := ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning}
	drive := &exploreDrive{lease: lease, state: &state, candidate: CandidateAction{Action: "claim", Kind: kindDeclared, RequestID: "req-1"}}
	if err := fixture.d.markExploreAction(drive, actionPlanned, IndependentFacts{State: "idle"}); err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.markExploreAction(drive, actionDispatching, IndependentFacts{State: "idle"}); err == nil {
		t.Fatal("expired lease must stop dispatching external requests")
	}
}

func TestAbortPreventsRunningWorkerCheckpoint(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-abort-race"
	seedExploreCampaign(t, fixture.d, campaign)
	lease, err := fixture.d.claimExploreCampaign(campaign, "worker-run")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.d.abortExploreCampaign(campaign, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.saveExploreCheckpoint(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-zombie", ClaimedBy: lease.Worker, Fence: lease.Fence}); err == nil {
		t.Fatal("aborted campaign must not accept an unowned running overwrite")
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-late"}, lease); err == nil {
		t.Fatal("running worker must not commit after abort")
	}
	state, err := fixture.d.loadExploreCheckpoint(campaign)
	if err != nil || state.Status != exploreAborted {
		t.Fatalf("abort must remain the terminal status: %+v %v", state, err)
	}
}

func TestResumeWithStaleFenceCannotRewrite(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-stale-resume"
	seedExploreCampaign(t, fixture.d, campaign)
	old, err := fixture.d.claimExploreCampaign(campaign, "worker-old")
	if err != nil {
		t.Fatal(err)
	}
	state := ExploreCheckpoint{
		Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, ClaimedBy: old.Worker, Fence: old.Fence,
		Actions: []ExploreActionRecord{{
			Candidate:     CandidateAction{Action: "", Kind: kindRecover, RequestID: "idle"},
			Status:        actionDispatching,
			SideEffecting: false,
			Before:        IndependentFacts{State: "idle"},
		}},
	}
	if err := fixture.d.saveExploreCheckpointOwned(state, old); err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	newer, err := fixture.d.claimExploreCampaign(campaign, "worker-new")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-owner"}, newer); err != nil {
		t.Fatal(err)
	}
	stale := state
	stale.Checkpoint = "CK-stale-resume"
	if err := fixture.d.reconcileUnknownActions(context.Background(), RunBinding{}, ExploreAdapter{}, map[string]Fixture{}, &stale, old); err == nil {
		t.Fatal("reconcile with a stale fencing token must not rewrite the campaign")
	}
	loaded, err := fixture.d.loadExploreCheckpoint(campaign)
	if err != nil || loaded.Checkpoint != "CK-owner" || loaded.ClaimedBy != "worker-new" {
		t.Fatalf("stale reconcile overwrote the new owner: %+v %v", loaded, err)
	}
}

func TestTwoConnectionsCannotDoubleClaim(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-two-conn"
	seedExploreCampaign(t, fixture.d, campaign)
	left, err := openDB(fixture.d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer left.close()
	right, err := openDB(fixture.d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer right.close()
	start := make(chan struct{})
	var wins int32
	var workers sync.WaitGroup
	workers.Add(2)
	for _, item := range []struct {
		conn   db
		worker string
	}{{left, "conn-left"}, {right, "conn-right"}} {
		go func(conn db, worker string) {
			defer workers.Done()
			<-start
			if _, err := conn.claimExploreCampaign(campaign, worker); err == nil {
				atomic.AddInt32(&wins, 1)
			}
		}(item.conn, item.worker)
	}
	close(start)
	workers.Wait()
	if atomic.LoadInt32(&wins) != 1 {
		t.Fatalf("two connections must not both own one campaign, winners=%d", wins)
	}
}

func TestIndependentProcessesClaimOnlyOnce(t *testing.T) {
	if os.Getenv("ARC_VERIFY_HELPER") == "claim" {
		os.Exit(runExploreClaimHelper())
	}
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-proc-claim"
	seedExploreCampaign(t, fixture.d, campaign)
	path := fixture.d.file
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan string, 2)
	var workers sync.WaitGroup
	spawn := func(worker string) {
		defer workers.Done()
		<-start
		cmd := exec.Command(os.Args[0], "-test.run=TestIndependentProcessesClaimOnlyOnce", "-test.count=1")
		cmd.Env = append(os.Environ(),
			"ARC_VERIFY_HELPER=claim",
			"ARC_VERIFY_DB="+path,
			"ARC_VERIFY_CAMPAIGN="+campaign,
			"ARC_VERIFY_WORKER="+worker,
		)
		body, err := cmd.CombinedOutput()
		if err != nil {
			results <- "err:" + worker + ":" + err.Error() + ":" + string(body)
			return
		}
		results <- strings.TrimSpace(string(body))
	}
	workers.Add(2)
	go spawn("proc-left")
	go spawn("proc-right")
	close(start)
	workers.Wait()
	close(results)
	won := 0
	logs := []string{}
	for line := range results {
		logs = append(logs, line)
		if strings.Contains(line, "CLAIM_OK") {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("exactly one independent process may claim, winners=%d logs=%q", won, logs)
	}
	reopened, err := openDB(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	rows, err := reopened.query("SELECT claimed_by FROM exploration_state WHERE campaign="+quote(campaign), 1)
	if err != nil || len(rows) != 1 || rows[0][0] == "" {
		t.Fatalf("ledger must keep one process owner: %+v %v", rows, err)
	}
}

func runExploreClaimHelper() int {
	path := os.Getenv("ARC_VERIFY_DB")
	campaign := os.Getenv("ARC_VERIFY_CAMPAIGN")
	worker := os.Getenv("ARC_VERIFY_WORKER")
	var last error
	for i := 0; i < 8; i++ {
		d, err := openDB(path, false)
		if err != nil {
			last = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_, err = d.claimExploreCampaign(campaign, worker)
		d.close()
		if err == nil {
			os.Stdout.WriteString("CLAIM_OK\n")
			return 0
		}
		last = err
		if !strings.Contains(err.Error(), "claimed") && !strings.Contains(strings.ToLower(err.Error()), "busy") && !strings.Contains(strings.ToLower(err.Error()), "lock") {
			os.Stdout.WriteString("CLAIM_DENY:" + err.Error() + "\n")
			return 0
		}
		if strings.Contains(err.Error(), "already claimed") {
			os.Stdout.WriteString("CLAIM_DENY\n")
			return 0
		}
		time.Sleep(50 * time.Millisecond)
	}
	os.Stdout.WriteString("CLAIM_DENY:" + last.Error() + "\n")
	return 0
}

func TestSameRequestIDKeepsSeparateAttempts(t *testing.T) {
	first := ExploreActionRecord{
		AttemptID: "AT-1",
		Candidate: CandidateAction{Action: "claim", RequestID: "same-req"},
		Status:    actionRecorded,
		Before:    IndependentFacts{State: "idle"},
	}
	second := ExploreActionRecord{
		AttemptID: "AT-2",
		Candidate: CandidateAction{Action: "claim", RequestID: "same-req"},
		Status:    actionDispatching,
		Before:    IndependentFacts{State: "idle"},
	}
	out := upsertExploreAction(upsertExploreAction(nil, first), second)
	if len(out) != 2 {
		t.Fatalf("the same Request ID must keep both attempts, got %+v", out)
	}
	if out[0].AttemptID != "AT-1" || out[0].Status != actionRecorded {
		t.Fatalf("first attempt history was overwritten: %+v", out)
	}
}

func TestObservedCrashWindowIsReconciledWithoutReplay(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-observed"
	seedExploreCampaign(t, fixture.d, campaign)
	lease, err := fixture.d.claimExploreCampaign(campaign, "worker-obs")
	if err != nil {
		t.Fatal(err)
	}
	state := ExploreCheckpoint{
		Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, ClaimedBy: lease.Worker, Fence: lease.Fence,
		Actions: []ExploreActionRecord{{
			AttemptID:     "AT-obs",
			Candidate:     CandidateAction{Action: "claim", Kind: kindDeclared, RequestID: "obs-1"},
			Status:        actionObserved,
			SideEffecting: true,
			Before:        IndependentFacts{State: "idle"},
			Step:          &ExploreStep{RAW: "", SHA256: ""},
		}},
	}
	if err := fixture.d.saveExploreCheckpointOwned(state, lease); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.reconcileUnknownActions(context.Background(), RunBinding{}, ExploreAdapter{}, map[string]Fixture{}, &state, lease); err != nil {
		t.Fatal(err)
	}
	if len(state.Actions) != 1 || state.Actions[0].Status != actionBlocked {
		t.Fatalf("OBSERVED without RAW must become BLOCKED, not skipped or replayed: %+v", state.Actions)
	}
	if state.Actions[0].Status == actionPlanned || state.Actions[0].Status == actionDispatching {
		t.Fatal("observed crash must not return the action to a dispatchable state")
	}
}

func TestEvidenceGateEmptyReportIsNotPass(t *testing.T) {
	verdict, reason := evaluateEvidenceGate(EvidenceGateInput{
		Report:         ExploreReport{StopReason: "complete"},
		Binding:        RunBinding{},
		ExpectedCommit: "abc",
		ExpectedTarget: "http://127.0.0.1:9",
	})
	if verdict == gatePass || verdict == gateUnspec {
		t.Fatalf("zero-step report must not PASS: %s %s", verdict, reason)
	}
}

func TestOfficialGateRejectsBrokenExploreEvidence(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	before, err := d.gate("academic-v1")
	if err != nil || before["verdict"] != "PASS" {
		t.Fatalf("fixture must start from a passing academic-v1 gate: %+v %v", before, err)
	}
	if err := d.saveExploreCheckpoint(ExploreCheckpoint{
		Campaign: "X-empty", Graph: "academic-v1", Status: exploreCompleted,
		Report: ExploreReport{Campaign: "X-empty", Graph: "academic-v1", StopReason: "complete", Steps: nil},
	}); err != nil {
		t.Fatal(err)
	}
	after, err := d.gate("academic-v1")
	if err == nil && after["verdict"] == "PASS" {
		t.Fatalf("official gate ignored a complete campaign with no steps: %+v", after)
	}
	if after["evidence_verdict"] == nil && after["evidence_gate"] == nil {
		t.Fatalf("official gate must surface explore evidence evaluation: %+v", after)
	}
	if err := run([]string{"gate", "--db", d.file, "--graph", "academic-v1"}); err == nil {
		t.Fatal("arc-verify gate CLI must exit non-zero on broken explore evidence")
	}
}

func TestCoverageDoesNotTrustRAWPresenceAlone(t *testing.T) {
	d := isolatedLedger(t, sample())
	raw := filepath.Join(t.TempDir(), "step.json")
	if err := os.WriteFile(raw, []byte(`{"identity":{"graph":"academic-v1"},"run":"X-1","step":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	report := ExploreReport{
		Campaign:   "X-raw-only",
		Graph:      "academic-v1",
		StopReason: "complete",
		Steps: []ExploreStep{{
			Candidate: CandidateAction{Action: "submit", Scope: "R1"},
			RAW:       raw,
			SHA256:    digestBytes([]byte(`{"identity":{"graph":"academic-v1"},"run":"X-1","step":1}`)),
			Findings:  []ExploreFinding{{Kind: "FAIL", Rule: "R1", Reason: "confirmed violation"}},
		}},
		RuleFails: []ExploreFinding{{Kind: "FAIL", Rule: "R1"}},
	}
	if err := d.persistExploration(report); err != nil {
		t.Fatal(err)
	}
	cover, err := d.explorationCoverage("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	item := cover["R1"]
	if item.Trusted {
		t.Fatalf("RAW+SHA256 without official evidence verification must not be trusted: %+v", item)
	}
	if item.EvidenceTrust == "VERIFIED" {
		t.Fatalf("business FAIL with unbound RAW is not VERIFIED: %+v", item)
	}
	if item.BusinessVerdict == "PASS" {
		t.Fatalf("a confirmed rule FAIL must not become a business PASS: %+v", item)
	}
}

func TestPlannerAuditIsPersisted(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Action: "claim", Entry: "/claim", Role: "STUDENT", Reason: "probe capacity", Observe: "state",
	}}}
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget:  ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 1, MaxPlannerCalls: 1, Deadline: 6 * time.Second},
		Mode:    "ai",
		Planner: planner,
	})
	if err != nil && report.Campaign == "" {
		t.Fatal(err)
	}
	if planner.Calls < 1 {
		t.Fatal("scripted planner must be invoked so the audit path is exercised")
	}
	rows, err := fixture.d.query("SELECT id FROM exploration_planner_audit WHERE campaign="+quote(report.Campaign), 1)
	if err != nil || len(rows) == 0 {
		t.Fatalf("planner audit must be persisted, not discarded: rows=%v err=%v report=%+v", rows, err, report)
	}
}

func TestBudgetFieldsStopExpansion(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 4, MaxDepth: 1, MaxEvidenceBytes: 1, Deadline: 6 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.StopReason == "complete" || !report.Truncated {
		t.Fatalf("depth or evidence budget must truncate: %+v", report)
	}
	if report.Spent.EvidenceKind == "" && report.Spent.RSSKind == "" {
		body, _ := json.Marshal(report.Spent)
		t.Fatalf("spent must distinguish MemStats from process RSS: %s", body)
	}
}
