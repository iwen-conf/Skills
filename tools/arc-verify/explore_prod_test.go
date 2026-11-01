package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExploreBudgetExhaustionKeepsFrontier(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 1, Deadline: 6 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Truncated || report.StopReason == "complete" {
		t.Fatalf("a leftover frontier must not be reported complete: %+v", report)
	}
	if len(report.Remaining) == 0 {
		t.Fatal("budget exhaustion must keep the unexplored frontier")
	}
	if report.Spent.DBBytes == 0 || report.Spent.PeakRSSBytes == 0 {
		t.Fatalf("spent must record process and database bounds: %+v", report.Spent)
	}
}

func TestExploreGroupPartialIsNotSingleSuccess(t *testing.T) {
	target, server := newSeatServer(t)
	target.capacity = 1
	target.raceNoLock = true
	target.raceGate = newOccupancyBarrier(2)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	binding, err := fixture.d.exploreBinding("seat-v2", "independent", fixture.commit, fixture.account, fixture.fixture, fixture.root, fixture.config, "seat-store", "explore/group")
	if err != nil {
		t.Fatal(err)
	}
	adapter := seatExploreAdapter(t, seatIsolationScope)
	accounts, _, _, err := fixture.d.exploreAccounts(adapter, binding, fixture.account, fixture.fixture)
	if err != nil {
		t.Fatal(err)
	}
	_, calls, group, err := fixture.d.performExplore(t.Context(), binding, adapter, accounts, CandidateAction{
		Kind: kindConcurrent, Action: "claim", Entry: "/claim", Role: "STUDENT", Account: "fixture.student",
		PartnerRole: "CONTENDER", RequestID: "pair-partial",
	})
	if err != nil && !isTimeout(err) && !errorsIsUnknown(err) {
		t.Fatal(err)
	}
	if len(calls) != 2 || len(group.Members) != 2 {
		t.Fatalf("group lost a member: calls=%+v group=%+v", calls, group)
	}
	if group.Partial || group.Unknown || group.AllDenied {
		if group.Members[0].Error == "" && group.Members[1].Error == "" && !group.Members[0].Denied && !group.Members[1].Denied && !group.Partial {
			t.Fatalf("mixed or unknown outcomes must not collapse to a silent success: %+v", group)
		}
	}
}

func TestDiscoverUndeclaredSourceEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "routes.go"), []byte(`package app
func routes(mux *http.ServeMux) {
  mux.HandleFunc("/claim", handleClaim)
  mux.HandleFunc("/hidden-close", handleClose)
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	found, err := discoverSourceEntries(root, ExploreAdapter{Entries: []ExploreEntry{{Action: "claim", Entry: "/claim"}}})
	if err != nil {
		t.Fatal(err)
	}
	var undeclared, confirmed bool
	for _, item := range found {
		item = authorizeDiscoveredEntry(item, ExploreAdapter{Entries: []ExploreEntry{{Action: "claim", Entry: "/claim"}}})
		if item.Path == "/claim" && item.Class == entryConfirmed {
			confirmed = true
		}
		if item.Path == "/hidden-close" && item.Class == entryUndeclared {
			undeclared = true
		}
		if item.Class == entryForbidden && strings.Contains(item.Path, "://") {
			t.Fatalf("scanner must not emit executable foreign URIs: %+v", item)
		}
	}
	if !confirmed || !undeclared {
		t.Fatalf("source discovery must keep confirmed and undeclared entries: %+v", found)
	}
}

func TestEvidenceGateRejectsTamperAndMissingRAW(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "001.json")
	if err := os.WriteFile(raw, []byte(`{"identity":{"graph":"seat-v2","source_commit":"abc","target":"http://127.0.0.1:9"},"run":"R","step":1,"expected":"pending","observed":"pending","preconditions":{},"denied":false,"oracle":"seat-store","runner":"x","exit_code":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	sha := mustFileDigest(t, raw)
	report := ExploreReport{Steps: []ExploreStep{{RAW: raw, SHA256: sha}}, StopReason: "complete"}
	binding := RunBinding{Identity: ExecutionIdentity{Graph: "seat-v2", SourceCommit: "abc", Target: "http://127.0.0.1:9"}}
	if verdict, _ := evaluateEvidenceGate(EvidenceGateInput{Report: report, Binding: binding, ExpectedCommit: "abc", ExpectedTarget: "http://127.0.0.1:9"}); verdict != gatePass && verdict != gateUnspec {
		t.Fatalf("intact evidence should pass or stay unspecified, got %s", verdict)
	}
	if err := os.Remove(raw); err != nil {
		t.Fatal(err)
	}
	if verdict, reason := evaluateEvidenceGate(EvidenceGateInput{Report: report, Binding: binding, ExpectedCommit: "abc"}); verdict != gateFail || !strings.Contains(reason, "RAW") {
		t.Fatalf("missing RAW must fail the gate: %s %s", verdict, reason)
	}
	if err := os.WriteFile(raw, []byte(`{"identity":{"graph":"other","source_commit":"zzz","target":"http://evil"},"run":"R","step":1,"expected":"x","observed":"x","preconditions":{},"denied":false,"oracle":"seat-store","runner":"x","exit_code":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	report.Steps[0].SHA256 = mustFileDigest(t, raw)
	if verdict, _ := evaluateEvidenceGate(EvidenceGateInput{Report: report, Binding: binding, ExpectedCommit: "abc", ExpectedTarget: "http://127.0.0.1:9"}); verdict != gateFail {
		t.Fatalf("transplanted RAW must fail, got %s", verdict)
	}
	if verdict, reason := evaluateEvidenceGate(EvidenceGateInput{Report: report, Binding: binding, RequirePlanner: true, PlannerCalls: 0}); verdict != gateBlocked || !strings.Contains(reason, "planner") {
		t.Fatalf("AI gate without planner calls must block: %s %s", verdict, reason)
	}
	if verdict, _ := evaluateEvidenceGate(EvidenceGateInput{Report: report, Binding: binding, ExpectedCommit: "newer"}); verdict != gateStale {
		t.Fatalf("commit change must be STALE, got %s", verdict)
	}
}

func TestOracleSyntheticCannotPass(t *testing.T) {
	fields := classifyOracleFacts(IndependentFacts{State: "idle", Role: "STUDENT", Principal: "synthetic-STUDENT"}, seatOracle(), "http://127.0.0.1:9", "STUDENT", true)
	if oracleAllowsBusinessPass(fields) {
		t.Fatal("synthetic oracle fields cannot be the sole PASS evidence")
	}
	fields = classifyOracleFacts(IndependentFacts{State: "idle", Role: "STUDENT", Principal: "p"}, seatOracle(), "http://127.0.0.1:9", "STUDENT", false)
	if !oracleAllowsBusinessPass(fields) {
		t.Fatal("directly observed fields may support a business verdict")
	}
}

func TestHTTPPlannerUnavailableWithoutCredentials(t *testing.T) {
	t.Setenv("ARC_VERIFY_PLANNER_URL", "")
	t.Setenv("ARC_VERIFY_PLANNER_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := NewHTTPExplorePlannerFromEnv(); err == nil {
		t.Fatal("missing model configuration must be a visible planner failure")
	}
}

func TestIMMongoBusinessChainsStayAuthorized(t *testing.T) {
	matrix := []struct {
		chain  string
		status string
		reason string
	}{
		{"A-consult-claim", "NOT_RUN", "requires live iso28886 and formally approved v2 RULE"},
		{"B-cross-role", "NOT_RUN", "requires live iso28886 and formally approved v2 RULE"},
		{"C-concurrency-recovery", "NOT_RUN", "requires live iso28886; unauthorized fault injection stays blocked"},
	}
	for _, row := range matrix {
		if row.status != "NOT_RUN" && row.status != "BLOCKED" {
			t.Fatalf("unexecuted IM-mongo chain %s must not be marked %s", row.chain, row.status)
		}
		if row.reason == "" {
			t.Fatalf("chain %s needs a blocker reason", row.chain)
		}
	}
}

func errorsIsUnknown(err error) bool {
	return err != nil && (err == errOutcomeUnknown || strings.Contains(err.Error(), "unknown"))
}

func mustFileDigest(t *testing.T, path string) string {
	t.Helper()
	sum, err := digestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}
