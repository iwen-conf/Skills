package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorldScanFindsEcommerceCouplings(t *testing.T) {
	repo := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("db/001.sql", `
CREATE TABLE wallet_accounts (
	id BIGINT PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id),
	available_cents BIGINT NOT NULL
);
CREATE TABLE fulfillment_orders (
	id BIGINT PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id),
	delivery_status TEXT NOT NULL
);
`)
	mustWrite("internal/settlement/model.go", `package settlement
type SettlementOrder struct {
	UserID int64
	PendingCents int64
}
func Emit(bus Bus, userID int64) { bus.Publish("users.refund.settled", userID) }
`)
	res, err := scanWorldFlags(repo, "users.id")
	if err != nil {
		t.Fatalf("scanWorldFlags: %v", err)
	}
	if len(res.Candidates) < 4 {
		t.Fatalf("expected >=4 candidates, got %+v", res.Candidates)
	}
}

func TestLintStoryDetectsSoftlockMissingFlagsOverlapAndUnreachableEnding(t *testing.T) {
	s := &Storyline{
		StoryID:        "bad_story",
		FocalEntity:    "users.id",
		FrozenReadonly: []string{"api/openapi.yaml"},
		WorldFlags:     map[string]string{"wallet_available": "wallet_accounts.available_cents"},
		Checkpoints: []Checkpoint{
			{SaveID: "save_1", Parent: "root"},
		},
		Routes: []Route{
			{
				RouteID:       "r1",
				LoadSave:      "save_1",
				RuntimeNS:     "ns_same",
				WriteScope:    []string{"tests/shared/**"},
				UnlocksEnding: "END_A",
				Sequence: []RouteStep{
					{Step: 1, Choice: "apply_cancel", ExpectStatus: "COOLING_SUSPENSE"},
				},
			},
			{
				RouteID:       "r2",
				LoadSave:      "save_1",
				RuntimeNS:     "ns_same",
				WriteScope:    []string{"tests/shared/sub_test.go", "api/openapi.yaml"},
				UnlocksEnding: "END_A",
				Sequence: []RouteStep{
					{Step: 1, Choice: "force", ExpectStatus: "PURGED"},
				},
			},
		},
		RequiredEndings: []string{"END_A", "END_UNREACHABLE"},
	}
	scan := &WorldScanResult{
		FocalEntity: "users.id",
		Candidates: []WorldFlagCandidate{
			{Flag: "fulfillment_orders_delivery_status", Source: "fulfillment_orders.delivery_status", File: "db.sql", Line: 5, Kind: "sql_status"},
		},
	}

	rep := lintStory(s, scan, 20)
	if rep.Status != "FAIL" {
		t.Fatalf("expected FAIL, got %+v", rep)
	}
	codes := map[string]bool{}
	for _, e := range rep.Errors {
		codes[e.Code] = true
	}
	for _, want := range []string{"UNBOUND_WORLD_FLAG", "SOFTLOCK_RISK", "DUPLICATE_RUNTIME_NS", "OVERLAPPING_WRITE_SCOPE", "FROZEN_WRITE_VIOLATION", "UNREACHABLE_ENDING"} {
		if !codes[want] {
			t.Fatalf("expected lint code %s, got %+v", want, rep.Errors)
		}
	}

	// Verify maxErrors capping works.
	capped := lintStory(s, scan, 3)
	if len(capped.Errors) != 3 || !capped.Truncated {
		t.Fatalf("expected 3 truncated errors, got len=%d truncated=%v", len(capped.Errors), capped.Truncated)
	}
}

func TestPlayParallelPreservesHistoricalFailAndComputes100PctCompletion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".arc", "verify", "story.db")
	ledger, err := openStoryLedger(dbPath)
	if err != nil {
		t.Fatalf("openStoryLedger: %v", err)
	}
	defer ledger.Close()

	s, err := loadStoryline("examples/ecommerce-cancel.story.yaml")
	if err != nil {
		t.Fatalf("load example storyline: %v", err)
	}
	// First run: inject a failing route to verify historical FAIL is preserved forever.
	failStory := *s
	failStory.Routes = append([]Route(nil), s.Routes...)
	failStory.Routes[0].VerifyCmd = "echo 'SOFTLOCK detected' && exit 1"

	sumFail, err := playStory(ledger, &failStory, PlayOptions{
		Parallel: true,
		Blind:    true,
		RepoRoot: dir,
	})
	if err != nil {
		t.Fatalf("playStory fail run: %v", err)
	}
	if sumFail.Verdict != "FAIL" || sumFail.Completion.Verdict != "FAIL" {
		t.Fatalf("expected FAIL verdict, got run=%s comp=%+v", sumFail.Verdict, sumFail.Completion)
	}
	if sumFail.Completion.SoftlockCount != 1 || sumFail.Completion.HistoricalFailCount != 1 {
		t.Fatalf("expected softlock=1 historicalFail=1, got %+v", sumFail.Completion)
	}

	// Second run: fix route and run all 4 routes concurrently with --parallel --blind.
	sumPass, err := playStory(ledger, s, PlayOptions{
		Parallel: true,
		Blind:    true,
		RepoRoot: dir,
	})
	if err != nil {
		t.Fatalf("playStory pass run: %v", err)
	}
	c := sumPass.Completion
	if c.Verdict != "PASS" || c.RouteCoveragePct != 100 || c.ButterflyProbeHitPct != 100 || c.EndingUnlockPct != 100 || c.WorldFlagBindingPct != 100 {
		t.Fatalf("expected 100%% PASS completion, got %+v", c)
	}
	if c.HistoricalFailCount != 1 {
		t.Fatalf("historical FAIL must be preserved even after later PASS; got %d", c.HistoricalFailCount)
	}

	// Plan shards check.
	plan, err := planShards(s)
	if err != nil {
		t.Fatalf("planShards: %v", err)
	}
	if len(plan.ConcurrentShards) != 4 || len(plan.CheckpointOrder) != 2 {
		t.Fatalf("unexpected shard plan: %+v", plan)
	}

	// Snapshot export check: must contain <meta charset="UTF-8"> in first 1024 bytes.
	snapDir := filepath.Join(dir, "snapshot-out")
	if err := exportSnapshot(ledger, s, snapDir); err != nil {
		t.Fatalf("exportSnapshot: %v", err)
	}
	htmlBytes, err := os.ReadFile(filepath.Join(snapDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	headChunk := string(htmlBytes[:min(1024, len(htmlBytes))])
	if !strings.Contains(headChunk, `<meta charset="UTF-8">`) {
		t.Fatalf("snapshot HTML missing UTF-8 charset in first 1024 bytes")
	}
}

func TestRouteWithoutExecutableCommandIsBlockedNotPass(t *testing.T) {
	dir := t.TempDir()
	ledger, err := openStoryLedger(filepath.Join(dir, "story.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	s := &Storyline{
		StoryID:         "no_cmd_story",
		FocalEntity:     "users.id",
		WorldFlags:      map[string]string{"wallet": "wallet.balance"},
		Checkpoints:     []Checkpoint{{SaveID: "save_1", Parent: "root"}},
		RequiredEndings: []string{"END_1"},
		Routes: []Route{
			{
				RouteID:       "r_empty",
				LoadSave:      "save_1",
				RuntimeNS:     "ns_empty",
				UnlocksEnding: "END_1",
				Sequence:      []RouteStep{{Step: 1, Choice: "noop", ExpectStatus: "OK"}},
			},
		},
	}
	sum, err := playStory(ledger, s, PlayOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Verdict != "BLOCKED" || sum.Completion.Verdict != "BLOCKED" {
		t.Fatalf("route with no executable cmd must be BLOCKED, got run=%s comp=%s", sum.Verdict, sum.Completion.Verdict)
	}
}
