package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRemainEvidenceRanOnHost60(t *testing.T) {
	id := collectHostIdentity()
	verdict := "BLOCKED"
	if id.On60 {
		verdict = "PASS"
	}
	writeRemainArtifact(t, "reports/host60.json", map[string]any{"verdict": verdict, "host": id})
	if host60Required() && !id.On60 {
		t.Fatalf("ARC_VERIFY_REQUIRE_HOST60=1 but this process is not on 192.168.0.60: %+v", id)
	}
}

func TestHost60LockRefusesLan50(t *testing.T) {
	if err := refuseOffHost60("http://192.168.1.50:28886"); err == nil {
		t.Fatal("host-lock 60 must refuse lan50 writes")
	}
	if err := refuseOffHost60("http://192.168.1.50:28885"); err == nil {
		t.Fatal("host-lock 60 must refuse shared 28885")
	}
	if err := refuseOffHost60("http://127.0.0.1:28886"); err != nil {
		t.Fatalf("loopback on 60 must stay allowed: %v", err)
	}
	if err := refuseOffHost60("http://192.168.0.60:28886"); err != nil {
		t.Fatalf("192.168.0.60 must stay allowed: %v", err)
	}
}

func TestSIGKILLDuringAcceptedClaimCountsOnce(t *testing.T) {
	if os.Getenv("ARC_VERIFY_HELPER") == "explore-kill" {
		os.Exit(runRemainExploreHelper())
	}
	target, server := newSeatServer(t)
	target.hangAfterAccept = 45 * time.Second
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	dbPath := fixture.d.file
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestSIGKILLDuringAcceptedClaimCountsOnce", "-test.count=1")
	cmd.Env = remainHelperEnv(dbPath, fixture, "explore-kill")
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(12 * time.Second)
	var claims int
	for time.Now().Before(deadline) {
		claims = seatStateCount(t, server.URL, "claims")
		if claims >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if claims < 1 {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatalf("accepted claim never incremented before SIGKILL: claims=%d", claims)
	}
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_, _ = cmd.Process.Wait()
	if got := seatStateCount(t, server.URL, "claims"); got != 1 {
		t.Fatalf("SIGKILL after accept must leave exactly one side-effect, claims=%d", got)
	}
}

func TestSIGKILLAfterCommandObservedCountsOnce(t *testing.T) {
	if os.Getenv("ARC_VERIFY_HELPER") == "explore-notify" {
		os.Exit(runRemainExploreHelper())
	}
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	notify := filepath.Join(t.TempDir(), "crash.notify")
	dbPath := fixture.d.file
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestSIGKILLAfterCommandObservedCountsOnce", "-test.count=1")
	cmd.Env = append(remainHelperEnv(dbPath, fixture, "explore-notify"),
		"ARC_VERIFY_FAULT_WINDOW=after_command_observed",
		"ARC_VERIFY_FAULT_NOTIFY="+notify,
		"ARC_VERIFY_FAULT_HANG_MS=45000",
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(notify); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(notify); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatal("crash window must write a notify file before hanging")
	}
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_, _ = cmd.Process.Wait()
	if got := seatStateCount(t, server.URL, "claims"); got != 1 {
		t.Fatalf("SIGKILL after the command returned must leave one claim, claims=%d", got)
	}
}

func TestEntryDiscoverAuthorizeExecuteOracle(t *testing.T) {
	target, server := newSeatServer(t)
	denied := prepareSeatFixture(t, server.URL)
	writeHiddenRoute(t, denied.root)
	denied.openRound(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	found, err := discoverSourceEntries(denied.root, adapter)
	if err != nil {
		t.Fatal(err)
	}
	var hidden DiscoveredEntry
	for _, item := range found {
		item = authorizeDiscoveredEntry(item, adapter)
		if item.Path == "/hidden-close" {
			hidden = item
		}
	}
	if hidden.Class != entryUndeclared {
		t.Fatalf("hidden-close must start undeclared: %+v found=%+v", hidden, found)
	}
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "capability.reservation", Action: "hidden_close", Entry: "/hidden-close",
		Role: "STUDENT", Reason: "probe undeclared close", Observe: "hidden_closes", Risk: "high",
	}}}
	if _, err := denied.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: denied.round,
		Root: denied.root, Config: denied.config, Commit: denied.commit,
		Account: denied.account, Fixture: denied.fixture, Adapter: adapter,
		Mode: "ai", Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 6, MaxPaths: 6, MaxActions: 2, Deadline: 8 * time.Second, MaxPlannerCalls: 2},
	}); err != nil && !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "budget") && !strings.Contains(err.Error(), "authorized") {
		t.Fatal(err)
	}
	if target.hiddenCloses != 0 {
		t.Fatalf("undeclared entry must not execute before authorize: hiddenCloses=%d", target.hiddenCloses)
	}
	allowed := prepareSeatFixture(t, server.URL)
	writeHiddenRoute(t, allowed.root)
	allowed.openRound(t)
	adapter.Entries = []ExploreEntry{{Action: "hidden_close", Entry: "/hidden-close", Method: "POST", Roles: []string{"STUDENT"}}}
	authorized := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "capability.reservation", Action: "hidden_close", Entry: "/hidden-close",
		Role: "STUDENT", Reason: "authorized undeclared close", Observe: "hidden_closes", Risk: "medium",
	}}}
	report, err := allowed.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: allowed.round,
		Root: allowed.root, Config: allowed.config, Commit: allowed.commit,
		Account: allowed.account, Fixture: allowed.fixture, Adapter: adapter,
		Mode: "ai", Planner: authorized,
		Budget: ExploreRuntimeBudget{MaxStates: 6, MaxPaths: 6, MaxActions: 3, Deadline: 8 * time.Second, MaxPlannerCalls: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.hiddenCloses < 1 {
		t.Fatalf("authorized hidden-close must execute: hiddenCloses=%d report=%+v", target.hiddenCloses, report)
	}
	if seatStateCount(t, server.URL, "hidden_closes") < 1 {
		t.Fatal("independent oracle /state must observe hidden_closes after execute")
	}
}

func TestThreeModeBlindCompareReportsHonestStats(t *testing.T) {
	target, server := newSeatServer(t)
	target.skipReview = true
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "capability.reservation", Action: "claim", Entry: "/claim",
		Role: "STUDENT", Reason: "scripted contrast fixture, not host Agent reasoning", Observe: "enrolled", Risk: "medium",
	}}}
	runMode := func(mode string, used ExplorePlanner) ExploreReport {
		t.Helper()
		fixture := prepareSeatFixture(t, server.URL)
		fixture.openRound(t)
		report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
			Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
			Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
			Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
			Mode: mode, Planner: used,
			Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 4, Deadline: 10 * time.Second, Seed: 3, MaxPlannerCalls: 2},
		})
		if err != nil {
			t.Fatalf("%s: %v %+v", mode, err, report)
		}
		if report.Actions == 0 {
			t.Fatalf("%s executed nothing", mode)
		}
		return report
	}
	reports := map[string]ExploreReport{
		"deterministic": runMode("deterministic", nil),
		"heuristic":     runMode("heuristic", nil),
		"ai":            runMode("ai", planner),
	}
	calls := map[string]int{"deterministic": 0, "heuristic": 0, "ai": planner.Calls}
	rows := contrastExploreModes(seatBlindMutations(), reports, calls)
	if len(rows) != 3 {
		t.Fatalf("need three mode rows: %+v", rows)
	}
	var ai ModeContrastRow
	for _, row := range rows {
		if row.Injected == 0 && row.Reachable+row.Triggered+row.Detected+row.NotExecuted == 0 {
			t.Fatalf("%s contrast dropped Injected/Reachable/Triggered/Detected/NotExecuted: %+v", row.Mode, row)
		}
		if row.Mode == "ai" {
			ai = row
		}
	}
	if ai.PlannerCalls < 1 {
		t.Fatalf("scripted AI contrast must record planner calls: %+v", ai)
	}
	if strings.Contains(ai.Notes, "planner_calls=0") {
		t.Fatalf("honest AI notes must not claim zero planner calls: %+v", ai)
	}
	if !strings.Contains(strings.ToLower(ai.Notes+planner.Suggestions[0].Reason), "scripted") {
		t.Fatalf("AI comparison row must identify the scripted fixture rather than host Agent reasoning: %+v", ai)
	}
	writeRemainArtifact(t, "reports/three-mode-contrast.json", map[string]any{
		"rows": rows, "planner": "scripted", "injected": skipReviewMutations(),
	})
}

func TestIndependentRunnerProcessStampsPID(t *testing.T) {
	if os.Getenv("ARC_VERIFY_HELPER") == "execute" {
		os.Exit(runRemainExecuteHelper())
	}
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	dbPath := fixture.d.file
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestIndependentRunnerProcessStampsPID", "-test.count=1")
	cmd.Env = remainHelperEnv(dbPath, fixture, "execute")
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated execute: %v %s", err, body)
	}
	helperPID := 0
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "HELPER_PID=") {
			helperPID, _ = strconv.Atoi(strings.TrimPrefix(line, "HELPER_PID="))
		}
	}
	if helperPID == 0 || helperPID == os.Getpid() {
		t.Fatalf("execute helper must be a distinct process: helper=%d parent=%d out=%s", helperPID, os.Getpid(), body)
	}
	reopened, err := openDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	found := false
	err = filepath.WalkDir(filepath.Join(filepath.Dir(dbPath), "raw"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return walkErr
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var step StepRAW
		if json.Unmarshal(raw, &step) != nil {
			return nil
		}
		if step.RunnerPID == helperPID {
			found = true
		}
		if step.RunnerPID == os.Getpid() {
			t.Fatalf("RAW runner_pid must not be the parent test process: %s", path)
		}
		if step.RunnerPID == 0 {
			t.Fatalf("execute RAW missing runner_pid: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("no RAW recorded helper pid %d: %s", helperPID, body)
	}
}

func writeHiddenRoute(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "routes.go"), []byte(`package app
func routes(mux *http.ServeMux) {
  mux.HandleFunc("/claim", handleClaim)
  mux.HandleFunc("/hidden-close", handleClose)
}
`), 0600); err != nil {
		t.Fatal(err)
	}
}

func remainHelperEnv(dbPath string, fixture seatFixture, helper string) []string {
	return append(os.Environ(),
		"ARC_VERIFY_HELPER="+helper,
		"ARC_VERIFY_DB="+dbPath,
		"ARC_VERIFY_ROOT="+fixture.root,
		"ARC_VERIFY_CONFIG="+fixture.config,
		"ARC_VERIFY_COMMIT="+fixture.commit,
		"ARC_VERIFY_ACCOUNT="+fixture.account,
		"ARC_VERIFY_FIXTURE="+fixture.fixture,
		"ARC_VERIFY_ROUND="+fixture.round,
	)
}

func runRemainExploreHelper() int {
	d, err := openDB(os.Getenv("ARC_VERIFY_DB"), false)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return 1
	}
	defer d.close()
	adapter := remainSeatAdapter()
	adapter.Entries = []ExploreEntry{{Action: "claim", Entry: "/claim", Method: "POST", Roles: []string{"STUDENT", "CONTENDER"}}}
	if os.Getenv("ARC_VERIFY_HELPER") == "explore-kill" || os.Getenv("ARC_VERIFY_HELPER") == "explore-observed" {
		adapter.StepTimeoutMS = 60000
		adapter.TimeoutMS = 60000
	}
	req := ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: os.Getenv("ARC_VERIFY_ROUND"),
		Root: os.Getenv("ARC_VERIFY_ROOT"), Config: os.Getenv("ARC_VERIFY_CONFIG"), Commit: os.Getenv("ARC_VERIFY_COMMIT"),
		Account: os.Getenv("ARC_VERIFY_ACCOUNT"), Fixture: os.Getenv("ARC_VERIFY_FIXTURE"), Adapter: adapter,
		Mode:   "deterministic",
		Budget: ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 1, Deadline: 50 * time.Second},
		Fault:  remainFaultFromEnv(),
	}
	if _, err := d.runExploration(context.Background(), req); err != nil {
		os.Stderr.WriteString("explore helper: " + err.Error() + "\n")
	}
	return 0
}

func runRemainExecuteHelper() int {
	d, err := openDB(os.Getenv("ARC_VERIFY_DB"), false)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return 1
	}
	defer d.close()
	if _, err := d.plan("seat-v2", 128); err != nil {
		os.Stderr.WriteString(err.Error())
		return 1
	}
	result, err := d.execute(context.Background(), ExecuteRequest{
		Task: "seat-v2/R-CLAIM/POSITIVE", Runner: "claim-positive", Round: os.Getenv("ARC_VERIFY_ROUND"),
		Environment: "independent", Commit: os.Getenv("ARC_VERIFY_COMMIT"), Root: os.Getenv("ARC_VERIFY_ROOT"),
		Config: os.Getenv("ARC_VERIFY_CONFIG"), Account: os.Getenv("ARC_VERIFY_ACCOUNT"), Fixture: os.Getenv("ARC_VERIFY_FIXTURE"),
		Accounts: map[string]string{},
	})
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return 1
	}
	os.Stdout.WriteString("HELPER_PID=" + strconv.Itoa(os.Getpid()) + "\n")
	os.Stdout.WriteString("RUN=" + result.Run + "\n")
	return 0
}

func remainFaultFromEnv() ExploreFault {
	hang := time.Duration(0)
	if ms, err := strconv.Atoi(os.Getenv("ARC_VERIFY_FAULT_HANG_MS")); err == nil && ms > 0 {
		hang = time.Duration(ms) * time.Millisecond
	}
	return ExploreFault{Window: os.Getenv("ARC_VERIFY_FAULT_WINDOW"), Notify: os.Getenv("ARC_VERIFY_FAULT_NOTIFY"), HangWindow: hang}
}

func remainSeatAdapter() ExploreAdapter {
	oracle := seatOracle()
	oracle.Call.CredentialHeader = "X-Fixture-Actor"
	recovery := seatRecovery(true, true, false)
	recovery.Oracle.Call.CredentialHeader = "X-Fixture-Actor"
	return ExploreAdapter{
		SchemaVersion: 1, Graph: "seat-v2", IsolationScope: seatIsolationScope,
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

func seatStateCount(t *testing.T, base, field string) int {
	t.Helper()
	resp, err := http.Get(strings.TrimRight(base, "/") + "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return 0
	}
	switch value := doc[field].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func skipReviewMutations() []SealedMutation {
	return []SealedMutation{seatBlindMutations()[0]}
}

func writeRemainArtifact(t *testing.T, rel string, value any) {
	t.Helper()
	path := filepath.Join("..", "..", ".arc", "artifacts", "verify-remain-close", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
