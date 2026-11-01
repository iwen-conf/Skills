package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestObservedWithoutRAWCannotReturnToFrontier(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-obs-frontier"
	seedExploreCampaign(t, fixture.d, campaign)
	lease, err := fixture.d.claimExploreCampaign(campaign, "worker-frontier")
	if err != nil {
		t.Fatal(err)
	}
	candidate := CandidateAction{ID: "C-claim", Action: "claim", Kind: kindDeclared, Source: sourceRule, Entry: "/claim", Role: "STUDENT", RequestID: "obs-frontier"}
	state := ExploreCheckpoint{
		Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, ClaimedBy: lease.Worker, Fence: lease.Fence,
		World:    ExplorePersistedWorld{State: "idle"},
		Frontier: []CandidateAction{candidate, {ID: "C-approve", Action: "approve", Kind: kindDeclared, Entry: "/approve", Role: "ADMIN", RequestID: "later"}},
		Actions: []ExploreActionRecord{{
			AttemptID: "AT-obs-frontier", Candidate: candidate, Status: actionObserved, SideEffecting: true,
			Before: IndependentFacts{State: "idle"}, RequestID: candidate.RequestID,
			Step: &ExploreStep{Candidate: candidate, RAW: ""},
		}},
	}
	if err := fixture.d.saveExploreCheckpointOwned(state, lease); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.reconcileUnknownActions(context.Background(), RunBinding{}, ExploreAdapter{}, map[string]Fixture{}, &state, lease); err != nil {
		t.Fatal(err)
	}
	if len(state.Actions) != 1 || state.Actions[0].AttemptID != "AT-obs-frontier" {
		t.Fatalf("first attempt must be kept: %+v", state.Actions)
	}
	if !state.Actions[0].ReplayForbidden || !state.Actions[0].EvidenceIncomplete {
		t.Fatalf("observed-without-RAW must be sealed unreplayable: %+v", state.Actions[0])
	}
	if state.Actions[0].Status != actionBlocked {
		t.Fatalf("status must be blocked, got %s", state.Actions[0].Status)
	}
	for _, item := range state.Frontier {
		if item.RequestID == candidate.RequestID || item.ID == candidate.ID || item.Action == "claim" && item.Role == "STUDENT" {
			t.Fatalf("observed-without-RAW action returned to frontier: %+v", state.Frontier)
		}
	}
	tried := map[string]bool{}
	for _, key := range state.Tried {
		tried[key] = true
	}
	if !tried[candidateKey(candidate, "idle")] && !tried[candidate.ID] && !tried["request:"+candidate.RequestID] {
		t.Fatalf("sealed action must enter the tried set: %+v", state.Tried)
	}
}

func TestObservedWithoutRAWBlocksCampaignResume(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	before := seatStateCount(t, server.URL, "claims")
	campaign := "X-obs-block-resume"
	if err := seedObservedWithoutRAWCampaign(t, fixture, campaign, "AT-obs-block", "obs-block"); err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	_, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: campaign,
		Budget:   ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 4, Deadline: 8 * time.Second},
	})
	if err == nil {
		t.Fatal("resume must refuse to continue a campaign with observed-without-RAW side effects")
	}
	if !errors.Is(err, errUnreplayableSideEffects) && !strings.Contains(err.Error(), "unreplayed") && !strings.Contains(err.Error(), "observed without RAW") {
		t.Fatalf("resume error must name the unreplayable hold: %v", err)
	}
	if got := seatStateCount(t, server.URL, "claims"); got != before {
		t.Fatalf("blocked resume dispatched another claim: before=%d after=%d", before, got)
	}
	state, loadErr := fixture.d.loadExploreCheckpoint(campaign)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !state.UnsafeResume {
		t.Fatalf("campaign must stay on an unsafe-resume hold: %+v", state)
	}
	if state.Status == exploreCompleted || state.Status == exploreRunning && len(state.Report.Steps) > 0 && state.Report.StopReason == "complete" {
		t.Fatalf("held campaign must not complete: %+v", state)
	}
}

func TestObservedMissingRAWKeepsFixtureLocked(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-obs-lock"
	if err := seedObservedWithoutRAWCampaign(t, fixture, campaign, "AT-obs-lock", "obs-lock"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.lockExploreFixtures(campaign, []string{fixture.fixture}); err != nil {
		t.Fatal(err)
	}
	lease, err := fixture.d.claimExploreCampaign(campaign, "worker-lock")
	if err != nil {
		t.Fatal(err)
	}
	state, err := fixture.d.loadExploreCheckpoint(campaign)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.reconcileUnknownActions(context.Background(), RunBinding{}, ExploreAdapter{}, map[string]Fixture{}, &state, lease); err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	_, _ = fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: "R-lock",
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: campaign,
		Budget:   ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 2, Deadline: 6 * time.Second},
	})
	rows, err := fixture.d.query("SELECT COUNT(*) FROM fixture_locks WHERE fixture="+quote(fixture.fixture)+" AND run="+quote(campaign), 1)
	if err != nil || len(rows) != 1 || rows[0][0] != "1" {
		t.Fatalf("fixture lock must remain until human recovery: %v %v", rows, err)
	}
}

func TestRecordedAttemptSurvivesRepeatedRequestID(t *testing.T) {
	first := ExploreActionRecord{
		AttemptID: "AT-keep", RequestID: "same-req",
		Candidate: CandidateAction{Action: "claim", RequestID: "same-req"},
		Status:    actionRecorded, RAW: "/tmp/raw.json", RAWSHA: "abc",
		Before: IndependentFacts{State: "idle"}, After: IndependentFacts{State: "assigned"},
	}
	downgrade := first
	downgrade.Status = actionDispatching
	downgrade.RAW = ""
	downgrade.After = IndependentFacts{}
	out := upsertExploreAction([]ExploreActionRecord{first}, downgrade)
	if len(out) != 1 || out[0].AttemptID != "AT-keep" || out[0].Status != actionRecorded || out[0].RAW == "" || out[0].After.State != "assigned" {
		t.Fatalf("recorded attempt was overwritten by a later dispatch of the same request: %+v", out)
	}
	retry := ExploreActionRecord{
		AttemptID: "AT-retry", RequestID: "same-req",
		Candidate: CandidateAction{Action: "claim", RequestID: "same-req"},
		Status:    actionDispatching, Before: IndependentFacts{State: "idle"},
	}
	out = upsertExploreAction(out, retry)
	if len(out) != 2 {
		t.Fatalf("a new attempt with the same Request ID must be appended: %+v", out)
	}
	if out[0].AttemptID != "AT-keep" || out[0].Status != actionRecorded {
		t.Fatalf("first recorded attempt must survive the retry: %+v", out)
	}
}

func TestObservedCrashDoesNotDuplicateSideEffects(t *testing.T) {
	runObservedCrashResume(t, "TestObservedCrashDoesNotDuplicateSideEffects")
}

func TestResumeAfterSIGKILLDoesNotRedispatchClaim(t *testing.T) {
	runObservedCrashResume(t, "TestResumeAfterSIGKILLDoesNotRedispatchClaim")
}

func runObservedCrashResume(t *testing.T, name string) {
	t.Helper()
	if os.Getenv("ARC_VERIFY_HELPER") == "explore-observed" {
		os.Exit(runRemainExploreHelper())
	}
	target, server := newSeatServer(t)
	_ = target
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	notify := filepath.Join(t.TempDir(), "observed.notify")
	dbPath := fixture.d.file
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run="+name, "-test.count=1")
	cmd.Env = append(remainHelperEnv(dbPath, fixture, "explore-observed"),
		"ARC_VERIFY_FAULT_WINDOW=after_observed_before_raw",
		"ARC_VERIFY_FAULT_NOTIFY="+notify,
		"ARC_VERIFY_FAULT_HANG_MS=45000",
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(notify); err == nil && seatStateCount(t, server.URL, "claims") >= 1 {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if _, err := os.Stat(notify); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatal("crash window must persist OBSERVED and notify before hang")
	}
	if got := seatStateCount(t, server.URL, "claims"); got < 1 {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatalf("business service never received the first claim: claims=%d", got)
	}
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_, _ = cmd.Process.Wait()
	if got := seatStateCount(t, server.URL, "claims"); got != 1 {
		t.Fatalf("SIGKILL after OBSERVED must leave exactly one side-effect, claims=%d", got)
	}
	reopened, err := openDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	rows, err := reopened.query("SELECT campaign FROM exploration_state LIMIT 1", 1)
	if err != nil || len(rows) != 1 || rows[0][0] == "" {
		t.Fatalf("crashed helper must have persisted a campaign: %v %v", rows, err)
	}
	campaignID := rows[0][0]
	if err := expireExploreLease(reopened, campaignID); err != nil {
		t.Fatal(err)
	}
	beforeResume := seatStateCount(t, server.URL, "claims")
	resumed, resumeErr := reopened.runExploration(context.Background(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Campaign: campaignID,
		Budget:   ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 4, Deadline: 8 * time.Second},
	})
	if resumeErr == nil {
		t.Fatalf("second process must not blindly resume: %+v", resumed)
	}
	if got := seatStateCount(t, server.URL, "claims"); got != beforeResume || got != 1 {
		t.Fatalf("second process redispatched the claim: claims=%d before_resume=%d", got, beforeResume)
	}
	state, err := reopened.loadExploreCheckpoint(campaignID)
	if err != nil {
		t.Fatal(err)
	}
	if classifyBusinessVerdict(state.Report) == "PASS" || state.Report.StopReason == "complete" {
		t.Fatalf("incomplete OBSERVED evidence must not become PASS: %+v", state.Report)
	}
	foundAttempt := false
	for _, rec := range state.Actions {
		if rec.AttemptID == "" {
			t.Fatal("attempt id was dropped")
		}
		if rec.RequestID != "" && rec.Status == actionRecorded && rec.RAW == "" {
			t.Fatalf("missing RAW was forged into a recorded PASS: %+v", rec)
		}
		if rec.ReplayForbidden || rec.Status == actionBlocked || rec.EvidenceIncomplete {
			foundAttempt = true
		}
	}
	if !foundAttempt && len(state.Actions) == 0 {
		t.Fatal("history of the first attempt is missing")
	}
	locks, err := reopened.query("SELECT COUNT(*) FROM fixture_locks WHERE run="+quote(campaignID), 1)
	if err != nil || len(locks) != 1 || locks[0][0] == "0" {
		t.Fatalf("fixture lock was released after incomplete OBSERVED crash: %v %v", locks, err)
	}
}

func seedObservedWithoutRAWCampaign(t *testing.T, fixture seatFixture, campaign, attemptID, requestID string) error {
	t.Helper()
	modelSHA, err := fixture.d.graphFingerprint("seat-v2")
	if err != nil {
		return err
	}
	candidate := CandidateAction{ID: "C-" + requestID, Action: "claim", Kind: kindDeclared, Source: sourceRule, Entry: "/claim", Role: "STUDENT", RequestID: requestID}
	state := ExploreCheckpoint{
		Campaign: campaign, Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		ModelSHA: modelSHA, SourceCommit: fixture.commit, Status: exploreInterrupted,
		Account: fixture.account, Fixture: fixture.fixture,
		World:    ExplorePersistedWorld{State: "idle"},
		Frontier: []CandidateAction{candidate},
		Actions: []ExploreActionRecord{{
			AttemptID: attemptID, Candidate: candidate, Status: actionObserved, SideEffecting: true,
			Before: IndependentFacts{State: "idle"}, RequestID: requestID,
			Step: &ExploreStep{Candidate: candidate, Before: IndependentFacts{State: "idle"}, RAW: ""},
		}},
		Report: ExploreReport{Campaign: campaign, Graph: "seat-v2", Flow: "flow.reservation", StopReason: "interrupted", Mode: "heuristic"},
	}
	return fixture.d.saveExploreCheckpoint(state)
}
