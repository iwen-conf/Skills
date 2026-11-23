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

func TestSurfaceScanInventoriesFullProjectEntryPoints(t *testing.T) {
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
	mustWrite("internal/api/router.go", `package api
func Register(r Router) {
	r.POST("/api/v1/orders", createOrder)
	r.GET("/api/v1/orders/:id", getOrder)
}
`)
	mustWrite("proto/payment.proto", `syntax = "proto3";
service PaymentService {
  rpc ConfirmPayment (ConfirmReq) returns (ConfirmResp);
}
`)
	mustWrite("db/001_init.sql", `CREATE TABLE orders (id BIGINT PRIMARY KEY, status TEXT NOT NULL);`)
	mustWrite("app/checkout/page.tsx", `export default function CheckoutPage() { return null; }`)
	mustWrite("internal/worker/consumer.go", `package worker
func Start(q Queue) { q.Subscribe("orders.paid", handlePaid) }
`)
	mustWrite("tests/e2e/checkout.spec.ts", `// e2e spec`)
	mustWrite("stories/orders.story.yaml", `story_id: orders`)

	surf, err := scanProjectSurface(repo)
	if err != nil {
		t.Fatalf("scanProjectSurface: %v", err)
	}
	if surf.Summary.HTTPAPIs != 2 || surf.Summary.GRPCRPCs != 1 || surf.Summary.DataEntities != 1 ||
		surf.Summary.UIRoutes != 1 || surf.Summary.AsyncWorkers != 1 || surf.Summary.ExistingTests != 1 || len(surf.StoryFiles) != 1 {
		t.Fatalf("unexpected surface summary: %+v", surf.Summary)
	}
	md := renderSurfaceScanMarkdown(surf)
	if !strings.Contains(md, "POST /api/v1/orders") || !strings.Contains(md, "ConfirmPayment") {
		t.Fatalf("unexpected surface markdown:\n%s", md)
	}
}

func TestLintStoryDetectsSoftlockInvalidBehaviorMissingFlagsOverlapAndUnreachableEnding(t *testing.T) {
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
					{Step: 1, Choice: "apply_cancel", ExpectedBehavior: "UNKNOWN_KIND", ExpectStatus: "COOLING_SUSPENSE"},
				},
			},
			{
				RouteID:       "r2",
				LoadSave:      "save_1",
				RuntimeNS:     "ns_same",
				WriteScope:    []string{"tests/shared/sub_test.go", "api/openapi.yaml"},
				UnlocksEnding: "END_A",
				Sequence: []RouteStep{
					{Step: 1, Choice: "force", ExpectedBehavior: "FORCE_CASCADE", ExpectStatus: "PURGED"},
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
	for _, want := range []string{
		"UNBOUND_WORLD_FLAG", "INVALID_EXPECTED_BEHAVIOR", "SOFTLOCK_RISK",
		"DUPLICATE_RUNTIME_NS", "OVERLAPPING_WRITE_SCOPE", "FROZEN_WRITE_VIOLATION", "UNREACHABLE_ENDING",
	} {
		if !codes[want] {
			t.Fatalf("expected lint code %s, got %+v", want, rep.Errors)
		}
	}

	capped := lintStory(s, scan, 3)
	if len(capped.Errors) != 3 || !capped.Truncated {
		t.Fatalf("expected 3 truncated errors, got len=%d truncated=%v", len(capped.Errors), capped.Truncated)
	}
}

func TestPlayParallelPreservesHistoricalFailVerifies6DTracesAndExplain(t *testing.T) {
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

	// Second run: run all 4 routes concurrently with 6D [ARC_STEP_EVIDENCE] + cross-step var extraction.
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
	if len(sumPass.StepTraces) != 5 {
		t.Fatalf("expected 5 step traces across 4 routes, got %d", len(sumPass.StepTraces))
	}

	// Verify cross-step extracted variable `settlement_tx=STX-88` propagated into step 2 actual_input!
	explainRep, err := explainStory(ledger, s.StoryID, sumPass.RunID, "route_in_transit_softlock_guard")
	if err != nil {
		t.Fatalf("explainStory: %v", err)
	}
	if len(explainRep.Routes) != 1 || len(explainRep.Routes[0].Steps) != 2 {
		t.Fatalf("expected 1 route with 2 steps in explain, got %+v", explainRep.Routes)
	}
	step2 := explainRep.Routes[0].Steps[1]
	if gotTx := step2.ActualInput["settlement_tx"]; gotTx != "STX-88" {
		t.Fatalf("expected cross-step extracted settlement_tx=STX-88 in step 2 actual_input, got %v", gotTx)
	}
	md := renderExplainMarkdown(explainRep)
	if !strings.Contains(md, "STX-88") || !strings.Contains(md, "ASYNC_HOLD_ESCAPE") {
		t.Fatalf("explain markdown missing expected 6D details:\n%s", md)
	}

	// Snapshot export check: must contain <meta charset="UTF-8"> in first 1024 bytes + step_traces.
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

func TestSixDimensionSensitivityCatchesProbeMismatchForbiddenEffectAndPreconditionBlock(t *testing.T) {
	dir := t.TempDir()
	ledger, err := openStoryLedger(filepath.Join(dir, "story.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	makeBase := func(step RouteStep) *Storyline {
		return &Storyline{
			StoryID:         "six_dim_sensitivity",
			FocalEntity:     "orders.id",
			WorldFlags:      map[string]string{"wallet_balance": "wallets.balance"},
			Checkpoints:     []Checkpoint{{SaveID: "save_1", Parent: "root"}},
			RequiredEndings: []string{"END_1"},
			Routes: []Route{{
				RouteID:       "r1",
				LoadSave:      "save_1",
				RuntimeNS:     "ns_r1",
				UnlocksEnding: "END_1",
				Sequence:      []RouteStep{step},
			}},
		}
	}

	// Case 1: Command exits 0, but actual_probes mismatches expected butterfly_probes -> kernel MUST FAIL!
	sProbeMismatch := makeBase(RouteStep{
		Step:             1,
		Choice:           "pay_order",
		ExpectedBehavior: "ALLOW_TRANSITION",
		ExpectStatus:     "PAID",
		ButterflyProbes:  map[string]any{"wallet_balance": 6800},
		Cmd:              `printf '[ARC_STEP_EVIDENCE]{"step":1,"actual_status":"PAID","actual_probes":{"wallet_balance":9999}}\n'`,
	})
	sum1, err := playStory(ledger, sProbeMismatch, PlayOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sum1.Verdict != "FAIL" || len(sum1.StepTraces) != 1 || !strings.Contains(sum1.StepTraces[0].DiffReason, "butterfly_probe[wallet_balance] mismatch") {
		t.Fatalf("expected FAIL due to butterfly_probe mismatch, got verdict=%s traces=%+v", sum1.Verdict, sum1.StepTraces)
	}

	// Case 2: Command exits 0, but forbidden_effect observed -> kernel MUST FAIL!
	sForbidden := makeBase(RouteStep{
		Step:             1,
		Choice:           "duplicate_pay",
		ExpectedBehavior: "IDEMPOTENT_REPLAY",
		ExpectStatus:     "PAID",
		ForbiddenEffects: []string{"DOUBLE_DEBIT"},
		Cmd:              `printf '[ARC_STEP_EVIDENCE]{"step":1,"actual_status":"PAID","observed_effects":["DOUBLE_DEBIT"]}\n'`,
	})
	sum2, err := playStory(ledger, sForbidden, PlayOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sum2.Verdict != "FAIL" || !strings.Contains(sum2.StepTraces[0].DiffReason, "forbidden_effect triggered") {
		t.Fatalf("expected FAIL due to forbidden_effect, got verdict=%s traces=%+v", sum2.Verdict, sum2.StepTraces)
	}

	// Case 3: Precheck command fails -> kernel MUST classify as BLOCKED (env/setup gap), NOT product FAIL!
	sPrecheckBlocked := makeBase(RouteStep{
		Step:             1,
		Choice:           "pay_order",
		ExpectedBehavior: "ALLOW_TRANSITION",
		Preconditions:    map[string]any{"gateway_up": true},
		PrecheckCmd:      "exit 1",
		Cmd:              "echo should_not_run",
		ExpectStatus:     "PAID",
	})
	sum3, err := playStory(ledger, sPrecheckBlocked, PlayOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sum3.Verdict != "BLOCKED" || sum3.StepTraces[0].Verdict != "BLOCKED" {
		t.Fatalf("expected BLOCKED when precheck_cmd fails, got run=%s step=%+v", sum3.Verdict, sum3.StepTraces[0])
	}

	// Case 4: Checkpoint assert_cmd fails -> buildCheckpoints MUST fail before running routes!
	sBadCheckpoint := makeBase(RouteStep{Step: 1, Choice: "noop", Cmd: "echo ok", ExpectStatus: "OK"})
	sBadCheckpoint.Checkpoints[0].AssertCmd = "exit 1"
	if _, err := playStory(ledger, sBadCheckpoint, PlayOptions{RepoRoot: dir}); err == nil || !strings.Contains(err.Error(), "assert_cmd failed") {
		t.Fatalf("expected checkpoint assert_cmd failure, got err=%v", err)
	}

	// Case 5: Passing output mentioning "without softlock" or "TestSoftlockGuard" (exit 0) MUST NOT false-positive!
	sBenignSoftlockWord := makeBase(RouteStep{
		Step:             1,
		Choice:           "verify_guard",
		ExpectedBehavior: "ALLOW_TRANSITION",
		ExpectStatus:     "OK",
		Cmd:              `echo "PASS: TestInTransitSoftlockGuard — cooling revert verified without softlock"`,
	})
	sum5, err := playStory(ledger, sBenignSoftlockWord, PlayOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sum5.Verdict != "PASS" || sum5.RouteResults[0].SoftlockDetected {
		t.Fatalf("benign 'without softlock' on exit 0 must PASS without softlock, got %+v", sum5.RouteResults[0])
	}

	// Case 6: Structured softlock=true in [ARC_STEP_EVIDENCE] even on exit 0 MUST FAIL with SoftlockDetected=true!
	sStructuredSoftlock := makeBase(RouteStep{
		Step:             1,
		Choice:           "stuck_cooling",
		ExpectedBehavior: "ASYNC_HOLD_ESCAPE",
		ExpectStatus:     "CANCEL_ABORTED_WITHDRAW_UNLOCKED",
		AntiSoftlock:     "can_withdraw == true",
		Cmd:              `printf '[ARC_STEP_EVIDENCE]{"step":1,"actual_status":"COOLING_SUSPENSE","softlock":true}\n'`,
	})
	sum6, err := playStory(ledger, sStructuredSoftlock, PlayOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if sum6.Verdict != "FAIL" || !sum6.RouteResults[0].SoftlockDetected {
		t.Fatalf("structured softlock=true must FAIL with SoftlockDetected=true, got %+v", sum6.RouteResults[0])
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
