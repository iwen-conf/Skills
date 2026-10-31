package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func startEnrollSUT(t *testing.T, faults string) (string, string) {
	t.Helper()
	return startEnrollSUTStore(t, filepath.Join(t.TempDir(), "store.json"), faults)
}

func startEnrollSUTStore(t *testing.T, store, faults string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "enroll-sut")
	addrFile := filepath.Join(dir, "addr")
	build := exec.Command("go", "build", "-o", bin, "./testdata/enroll-sut")
	if body, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build enroll-sut: %v %s", err, body)
	}
	command := exec.Command(bin)
	command.Env = append(os.Environ(), "ENROLL_SUT_STORE="+store, "ENROLL_SUT_ADDR_FILE="+addrFile, "ENROLL_SUT_LISTEN=127.0.0.1:0", "ENROLL_SUT_FAULTS="+faults)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if body, err := os.ReadFile(addrFile); err == nil && len(body) > 0 {
			return string(body), store
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("enroll-sut did not publish its listen address")
	return "", ""
}

func TestIndependentEnrollProcessBusinessChain(t *testing.T) {
	url, _ := startEnrollSUT(t, "")
	if resp, err := http.Get(url + "/oracle"); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	fixture := prepareSeatFixture(t, url)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.register(t, writeSeatManifest(t, "claim-login", "R-CLAIM/NEGATIVE/login", "primary", []RunnerStep{seatStep("claim", "", "/claim", "claim-1", "primary")}, seatRecovery(false, true, false)))
	fixture.register(t, writeSeatManifest(t, "no-force", "R-NO-FORCE/NEGATIVE/is_admin", "primary", []RunnerStep{seatStep("force", "", "/force", "force-1", "primary")}, seatRecovery(true, true, false)))
	fixture.register(t, writeSeatManifest(t, "sequence", "test.sequence.claim-approve", "admin", []RunnerStep{
		seatStep("claim", "claim-flow", "/claim", "seq-1", "claimant"),
		seatStep("approve", "approve-flow", "/approve", "seq-1", "primary"),
	}, seatRecovery(true, true, true)))
	fixture.register(t, writeSeatManifest(t, "idempotent", "test.idempotency.duplicate-claim", "primary", []RunnerStep{
		seatStep("claim", "claim-flow", "/claim", "dup-1", "primary"),
		seatStep("claim", "", "/claim", "dup-1", "primary"),
	}, seatRecovery(true, true, false)))
	fixture.register(t, writeSeatManifest(t, "concurrent", "test.concurrency.last-seat", "primary", []RunnerStep{
		func() RunnerStep {
			step := seatStep("claim", "claim-flow", "/claim", "race-1", "primary")
			step.ParallelGroup = "last-seat"
			return step
		}(),
		func() RunnerStep {
			step := seatStep("claim", "claim-flow", "/claim", "race-2", "contender")
			step.ParallelGroup = "last-seat"
			return step
		}(),
	}, seatRecovery(true, true, false)))
	fixture.openRound(t)

	normal := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if normal.Verdict != "PASS" || normal.Recovery.Verdict != "PASS" {
		t.Fatalf("independent process normal claim: %+v", normal)
	}
	deniedForce := fixture.run(t, "no-force", "R-NO-FORCE/NEGATIVE/is_admin", "", nil)
	if deniedForce.Verdict != "PASS" {
		t.Fatalf("cross-role force: %+v", deniedForce)
	}
	sequence := fixture.run(t, "sequence", "test.sequence.claim-approve", "fixture.admin", map[string]string{"claimant": "fixture.student"})
	if sequence.Verdict != "PASS" {
		t.Fatalf("student then registrar sequence: %+v", sequence)
	}
	duplicate := fixture.run(t, "idempotent", "test.idempotency.duplicate-claim", "", nil)
	if duplicate.Verdict != "PASS" {
		t.Fatalf("duplicate request: %+v", duplicate)
	}
	race := fixture.run(t, "concurrent", "test.concurrency.last-seat", "", map[string]string{"contender": "fixture.contender"})
	if race.Verdict != "PASS" {
		t.Fatalf("last-seat on independent process: %+v", race)
	}
	explore, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 4, Deadline: 8 * time.Second},
	})
	if err != nil || explore.Actions == 0 {
		t.Fatalf("discover run against the independent process: %+v %v", explore, err)
	}
	writeExploreArtifact(t, "reports/enroll-e2e.json", map[string]any{
		"target": url, "process": "testdata/enroll-sut", "not": "isolated_seat_test httptest",
		"normal": normal, "denied_force": deniedForce, "sequence": sequence, "duplicate": duplicate, "race": race,
		"explore": explore,
	})
}

func TestIndependentEnrollPersistenceAndFailedRecovery(t *testing.T) {
	store := filepath.Join(t.TempDir(), "shared.json")
	url, _ := startEnrollSUTStore(t, store, "")
	resp, err := http.Post(url+"/claim", "application/json", strings.NewReader(`{"request_id":"persist-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	body, err := os.ReadFile(store)
	if err != nil || !strings.Contains(string(body), "persist-1") {
		t.Fatalf("independent process did not persist the claim: %v %s", err, body)
	}
	urlRestart, _ := startEnrollSUTStore(t, store, "")
	oracle, err := http.Get(urlRestart + "/oracle")
	if err != nil {
		t.Fatal(err)
	}
	observed, _ := io.ReadAll(oracle.Body)
	oracle.Body.Close()
	if !strings.Contains(string(observed), `"state":"pending"`) {
		t.Fatalf("restarted process lost persisted state: %s", observed)
	}
	url2, _ := startEnrollSUT(t, "reset_broken")
	fixture2 := prepareSeatFixture(t, url2)
	fixture2.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture2.openRound(t)
	broken := fixture2.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if broken.Verdict != "BLOCKED" || classifyRecovery(broken.Recovery, 1) == RecoveryRestored {
		t.Fatalf("failed restore on the independent process must stay BLOCKED: %+v", broken)
	}
	locks, err := fixture2.d.query("SELECT COUNT(*) FROM fixture_locks", 1)
	if err != nil || locks[0][0] == "0" {
		t.Fatalf("locks released after failed independent restore: %v %v", locks, err)
	}
}

func TestIndependentEnrollFaultIsBlockedNotPass(t *testing.T) {
	url, _ := startEnrollSUT(t, "hang")
	fixture := prepareSeatFixture(t, url)
	fixture.register(t, writeSeatManifest(t, "timeout", "R-CLAIM/POSITIVE", "primary", []RunnerStep{seatStep("claim", "claim-flow", "/claim", "hang-1", "primary")}, seatRecovery(true, true, false)))
	fixture.openRound(t)
	timeout := fixture.run(t, "timeout", "R-CLAIM/POSITIVE", "", nil)
	if timeout.Verdict != "BLOCKED" {
		t.Fatalf("hung independent process is not a product PASS: %+v", timeout)
	}
}

func TestIndependentEnrollRetestKeepsHistoricFail(t *testing.T) {
	url, _ := startEnrollSUT(t, "skip_review")
	fixture := prepareSeatFixture(t, url)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	failed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if failed.Verdict != "FAIL" {
		t.Fatalf("skip-review on independent process: %+v", failed)
	}
	if _, err := http.Post(url+"/debug/faults", "text/plain", strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.d.retry("seat-v2/R-CLAIM/POSITIVE", "fixture independently restored", failed.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	passed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if passed.Verdict != "PASS" || passed.Run == failed.Run {
		t.Fatalf("retest must open a new PASS without covering the FAIL: fail=%s pass=%+v", failed.Run, passed)
	}
	var packet HandoffPacket
	if err := json.Unmarshal(cliJSON(t, "handoff", "--db", fixture.d.file, "--to", "arc-code", "--graph", "seat-v2", "--run", failed.Run), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Run != failed.Run || packet.Status != "FAIL" {
		t.Fatalf("later PASS overwrote the historic FAIL: %+v", packet)
	}
}

func TestIndependentEnrollExploreModeContrast(t *testing.T) {
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "reservation.claim", Action: "force", Entry: "/force", Role: "STUDENT",
		Reason: "students may still reach force after a claim", Observe: "state", Risk: "high",
	}}}
	run := func(mode string, planner ExplorePlanner, seed int) (ExploreReport, BlindCampaignResult) {
		t.Helper()
		url, _ := startEnrollSUT(t, "skipReview,unauthorizedForce,duplicateRecords")
		fixture := prepareSeatFixture(t, url)
		fixture.openRound(t)
		report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
			Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
			Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
			Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
			Mode: mode, Planner: planner,
			Budget: ExploreRuntimeBudget{MaxStates: 12, MaxPaths: 12, MaxActions: 6, Deadline: 12 * time.Second, Seed: seed},
		})
		if err != nil {
			t.Fatal(err)
		}
		return report, AttributeBlindCampaign(report, seatBlindMutations(), ExploreRuntimeBudget{MaxActions: 6})
	}
	deterministic, detAttr := run("deterministic", nil, 1)
	heuristic, heurAttr := run("heuristic", nil, 2)
	assisted, aiAttr := run("ai", planner, 3)
	writeExploreArtifact(t, "reports/enroll-mode-contrast.json", map[string]any{
		"deterministic": map[string]any{"campaign": deterministic.Campaign, "actions": deterministic.Actions, "counts": detAttr.Counts, "spent": detAttr.Spent},
		"heuristic":     map[string]any{"campaign": heuristic.Campaign, "actions": heuristic.Actions, "counts": heurAttr.Counts, "spent": heurAttr.Spent, "unspecified": len(heuristic.Unspecified)},
		"ai":            map[string]any{"campaign": assisted.Campaign, "actions": assisted.Actions, "counts": aiAttr.Counts, "spent": aiAttr.Spent, "planner_calls": planner.Calls},
		"im_mongo":      "iso28886 authorized; see reports/im-mongo-iso28886.json",
	})
	if deterministic.Mode != "deterministic" || heuristic.Mode != "heuristic" || assisted.Mode != "ai" {
		t.Fatalf("mode labels %s %s %s", deterministic.Mode, heuristic.Mode, assisted.Mode)
	}
	if deterministic.Actions == 0 || heuristic.Actions == 0 || assisted.Actions == 0 {
		t.Fatal("each isolated enroll-sut mode must execute live actions")
	}
}
