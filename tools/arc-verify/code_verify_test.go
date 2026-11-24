package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCodeCatalogCoversAll26TechniquesAnd8CoverageMetrics 验证26项代码测试技术与8项覆盖率指标的七维完整规格
func TestCodeCatalogCoversAll26TechniquesAnd8CoverageMetrics(t *testing.T) {
	specs := allCodeTechniques()
	if len(specs) != 26 {
		t.Fatalf("expected exactly 26 code testing techniques, got %d", len(specs))
	}

	seenIdx := map[int]bool{}
	seenID := map[string]bool{}
	tierCounts := map[CodePriorityTier]int{}
	overlooked := map[string]bool{}

	for _, spec := range specs {
		if spec.Index < 1 || spec.Index > 26 || seenIdx[spec.Index] {
			t.Fatalf("invalid or duplicate technique index %d (%s)", spec.Index, spec.ID)
		}
		seenIdx[spec.Index] = true
		if spec.ID == "" || seenID[spec.ID] {
			t.Fatalf("invalid or duplicate technique id %q", spec.ID)
		}
		seenID[spec.ID] = true
		tierCounts[spec.Priority]++
		if spec.TopOverlooked {
			overlooked[spec.ID] = true
			if strings.TrimSpace(spec.OverlookedReason) == "" {
				t.Fatalf("top_overlooked technique %s missing overlooked_reason", spec.ID)
			}
		}

		// Verify all 7 required dimensions are populated.
		if len(spec.Languages) == 0 {
			t.Fatalf("technique %s missing languages", spec.ID)
		}
		if len(spec.TriggerSignals) == 0 {
			t.Fatalf("technique %s missing trigger_signals", spec.ID)
		}
		if len(spec.Tools) == 0 {
			t.Fatalf("technique %s missing tools", spec.ID)
		}
		if len(spec.Metrics) == 0 {
			t.Fatalf("technique %s missing metrics", spec.ID)
		}
		if strings.TrimSpace(spec.FailureCriteria.Summary) == "" {
			t.Fatalf("technique %s missing failure_criteria summary", spec.ID)
		}
		if len(spec.ReportFormats) == 0 {
			t.Fatalf("technique %s missing report_formats", spec.ID)
		}
		if spec.EvidenceStorage.LedgerTable != "code_verify_results" ||
			spec.EvidenceStorage.ArtifactDirTemplate == "" ||
			!spec.EvidenceStorage.ImmutableFailHistory {
			t.Fatalf("technique %s has incomplete evidence_storage: %+v", spec.ID, spec.EvidenceStorage)
		}
	}

	if tierCounts[TierP0] != 6 || tierCounts[TierP1] != 8 || tierCounts[TierP2] != 7 || tierCounts[TierP3] != 5 {
		t.Fatalf("unexpected priority distribution P0..P3: %+v", tierCounts)
	}
	for _, want := range []string{"mutation", "property", "fuzz"} {
		if !overlooked[want] {
			t.Fatalf("expected %q marked top_overlooked, got %+v", want, overlooked)
		}
	}

	covMetrics := allCoverageMetrics()
	if len(covMetrics) != 8 {
		t.Fatalf("expected 8 coverage metrics, got %d", len(covMetrics))
	}
	wantCov := []CoverageMetricKind{CovLine, CovStatement, CovFunction, CovBranch, CovCondition, CovDecision, CovMCDC, CovPath}
	for i, kind := range wantCov {
		if covMetrics[i].ID != kind {
			t.Fatalf("coverage[%d]: got %s, want %s", i, covMetrics[i].ID, kind)
		}
	}
}

// TestCodePlannerAutoSelectsAndRiskPromotesForRustRepo 验证Rust仓库剖析提取真实行号信号并自动对高风险技术提级
func TestCodePlannerAutoSelectsAndRiskPromotesForRustRepo(t *testing.T) {
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

	mustWrite("Cargo.toml", `[package]
name = "ledger_core"
version = "0.1.0"
edition = "2021"
rust-version = "1.78"

[features]
default = ["std"]
std = []
fast-hash = []

[dependencies]
serde = "1.0"
serde_json = "1.0"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
proptest = "1.4"
rstest = "0.18"
insta = "1.34"
criterion = "0.5"
`)
	mustWrite("src/lib.rs", `use std::sync::{Arc, Mutex};
/// Parse untrusted frame
/// `+"```"+`
/// assert_eq!(1 + 1, 2);
/// `+"```"+`
pub fn parse_frame(raw: &[u8]) -> Result<(), String> {
    let _guard = Arc::new(Mutex::new(0u64));
    let _parsed: serde_json::Value = serde_json::from_slice(raw).map_err(|e| e.to_string())?;
    unsafe {
        let ptr = raw.as_ptr();
        let _ = ptr;
    }
    Ok(())
}
`)
	mustWrite("fuzz/fuzz_targets/parse.rs", `pub fn fuzz_entry(_data: &[u8]) {}`)
	mustWrite("benches/bench.rs", `pub fn bench_entry() {}`)
	mustWrite("tests/integration.rs", `#[test] fn end_to_end_parse() {}`)

	plan, err := buildCodeVerifyPlan(repo, CodePlanOptions{
		MaxTier:    TierP0,
		ProbeTools: false,
	})
	if err != nil {
		t.Fatalf("buildCodeVerifyPlan: %v", err)
	}
	if plan.PrimaryLanguage != "rust" {
		t.Fatalf("expected primary language rust, got %s", plan.PrimaryLanguage)
	}
	if len(plan.Profile.RiskSignals) == 0 {
		t.Fatal("expected non-empty risk signals with provenance")
	}
	for _, hit := range plan.Profile.RiskSignals {
		if hit.File == "" || hit.Line <= 0 {
			t.Fatalf("risk signal missing real file:line provenance: %+v", hit)
		}
	}

	byID := map[string]PlannedTechnique{}
	for _, tech := range plan.Techniques {
		byID[tech.TechniqueID] = tech
	}
	// Even with MaxTier=P0, high-risk P1/P2 techniques must promote to REQUIRED!
	for _, promotedID := range []string{"memory_safety", "data_race", "concurrency", "fuzz", "property"} {
		tech, ok := byID[promotedID]
		if !ok || tech.Status != SelectRequired || !tech.PromotedByRisk || tech.EffectivePriority != TierP0 {
			t.Fatalf("expected %s risk-promoted to REQUIRED/P0, got %+v", promotedID, tech)
		}
	}
	if byID["fuzz"].Command != "cargo +nightly fuzz run parse -- -max_total_time=30" {
		t.Fatalf("expected concrete fuzz target 'parse' in command, got %q", byID["fuzz"].Command)
	}
}

// TestCodePlannerAutoSelectsForGoRepo 验证Go项目剖析准确识别并发信号并装配Go生态工具链
func TestCodePlannerAutoSelectsForGoRepo(t *testing.T) {
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
	mustWrite("go.mod", "module example.com/svc\ngo 1.24.0\n")
	mustWrite("store.go", "package svc\nimport (\"encoding/json\"\n\"sync\")\ntype Store struct { mu sync.Mutex }\nfunc Decode(b []byte) error { var v int; return json.Unmarshal(b, &v) }\n")
	mustWrite("store_test.go", "package svc\nimport \"testing\"\nfunc TestDecode(t *testing.T) {}\n")

	plan, err := buildCodeVerifyPlan(repo, CodePlanOptions{MaxTier: TierP0, ProbeTools: false})
	if err != nil {
		t.Fatalf("buildCodeVerifyPlan go: %v", err)
	}
	if plan.PrimaryLanguage != "go" {
		t.Fatalf("expected primary language go, got %s", plan.PrimaryLanguage)
	}
	for _, tech := range plan.Techniques {
		if tech.TechniqueID == "data_race" {
			if tech.Status != SelectRequired || !tech.PromotedByRisk {
				t.Fatalf("expected Go data_race promoted to REQUIRED, got %+v", tech)
			}
			return
		}
	}
	t.Fatal("missing data_race in Go plan")
}

// TestCodeParsersHandleRustGoPythonTSOutputs 验证覆盖率、变异、安全审计与并发原生输出解析器
func TestCodeParsersHandleRustGoPythonTSOutputs(t *testing.T) {
	// 1. cargo-llvm-cov JSON: 5 measured metrics (when branch & mcdc enabled), 3 unsupported.
	llvmOut := `{"data":[{"totals":{"lines":{"count":100,"covered":85,"percent":85.0},"functions":{"count":10,"covered":9,"percent":90.0},"regions":{"count":50,"covered":41,"percent":82.0},"branches":{"count":20,"covered":15,"percent":75.0},"mcdc":{"count":0,"covered":0,"percent":0.0}}}]}`
	covPayload := parseTechniqueOutput("coverage", "rust", "cargo-llvm-cov", llvmOut, "")
	if covPayload.Coverage == nil || covPayload.Coverage.LinePct == nil || *covPayload.Coverage.LinePct != 85.0 ||
		covPayload.Coverage.BranchPct == nil || *covPayload.Coverage.BranchPct != 75.0 ||
		covPayload.Coverage.MCDCPct != nil {
		t.Fatalf("unexpected llvm-cov parse result: %+v", covPayload.Coverage)
	}
	if len(covPayload.Coverage.MeasuredMetrics) != 4 || len(covPayload.Coverage.UnsupportedMetrics) != 4 {
		t.Fatalf("expected 4 measured and 4 unsupported metrics, got measured=%v unsupported=%v",
			covPayload.Coverage.MeasuredMetrics, covPayload.Coverage.UnsupportedMetrics)
	}

	// 2. cargo test multi-crate summation.
	cargoOut := "test result: ok. 12 passed; 0 failed; 1 ignored; 0 measured\ntest result: FAILED. 3 passed; 2 failed; 0 ignored"
	unitPayload := parseTechniqueOutput("unit", "rust", "cargo test", cargoOut, "")
	if unitPayload.TestsPassed != 15 || unitPayload.TestsFailed != 2 || unitPayload.TestsSkipped != 1 {
		t.Fatalf("unexpected cargo test sum: %+v", unitPayload)
	}

	// 3. cargo mutants JSON outcomes + failure criteria check.
	artDir := t.TempDir()
	outcomes := `{"outcomes":[{"summary":"CaughtMutant"},{"summary":"CaughtMutant"},{"summary":"MissedMutant"}]}`
	if err := os.WriteFile(filepath.Join(artDir, "outcomes.json"), []byte(outcomes), 0o644); err != nil {
		t.Fatal(err)
	}
	mutPayload := parseTechniqueOutput("mutation", "rust", "cargo-mutants", "", artDir)
	if mutPayload.MutantsCaught != 2 || mutPayload.MutantsMissed != 1 || mutPayload.MutationScorePct == nil {
		t.Fatalf("unexpected cargo mutants parse: %+v", mutPayload)
	}
	v, reason := evaluateCodeFailureCriteria(CodeFailureCriteria{MinMutationScorePct: 80.0}, 0, mutPayload)
	if v != VerdictFail || !strings.Contains(reason, "mutation_score=66.7%") {
		t.Fatalf("expected mutation score failure, got verdict=%s reason=%q", v, reason)
	}

	// 4. Go tool cover -func + go test -race.
	goCovOut := "pkg/a.go:10:\tFoo\t100.0%\npkg/a.go:20:\tBar\t0.0%\ntotal:\t\t(statements)\t64.5%\n"
	goCovPayload := parseTechniqueOutput("coverage", "go", "go tool cover", goCovOut, "")
	if goCovPayload.Coverage == nil || goCovPayload.Coverage.StatementPct == nil || *goCovPayload.Coverage.StatementPct != 64.5 ||
		goCovPayload.Coverage.FunctionPct == nil || *goCovPayload.Coverage.FunctionPct != 50.0 {
		t.Fatalf("unexpected go cover parse: %+v", goCovPayload.Coverage)
	}
	racePayload := parseTechniqueOutput("data_race", "go", "go test -race", "WARNING: DATA RACE\n--- FAIL: TestRace (0.01s)", "")
	if racePayload.DataRaces != 1 || racePayload.TestsFailed != 1 {
		t.Fatalf("unexpected go race parse: %+v", racePayload)
	}

	// 5. cargo audit JSON + vitest coverage-summary.json + pytest coverage.json + [ARC_CODE_METRIC].
	auditPayload := parseTechniqueOutput("dependency_audit", "rust", "cargo-audit", `{"vulnerabilities":{"found":true,"count":2}}`, "")
	if auditPayload.HighVulns != 2 {
		t.Fatalf("expected 2 vulns from cargo audit, got %+v", auditPayload)
	}
	tsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tsDir, "coverage-summary.json"), []byte(`{"total":{"lines":{"pct":91.2},"statements":{"pct":90.5},"functions":{"pct":88.0},"branches":{"pct":79.5}}}`), 0o644)
	tsCov := parseTechniqueOutput("coverage", "typescript", "vitest", "", tsDir)
	if tsCov.Coverage == nil || tsCov.Coverage.LinePct == nil || *tsCov.Coverage.LinePct != 91.2 || *tsCov.Coverage.BranchPct != 79.5 {
		t.Fatalf("unexpected vitest coverage parse: %+v", tsCov.Coverage)
	}
	pyDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(pyDir, "coverage.json"), []byte(`{"totals":{"percent_covered":83.3,"covered_lines":25,"num_statements":30,"covered_branches":8,"num_branches":10}}`), 0o644)
	pyCov := parseTechniqueOutput("coverage", "python", "pytest-cov", "", pyDir)
	if pyCov.Coverage == nil || pyCov.Coverage.BranchPct == nil || *pyCov.Coverage.BranchPct != 80.0 {
		t.Fatalf("unexpected pytest coverage parse: %+v", pyCov.Coverage)
	}
	customOut := `[ARC_CODE_METRIC]{"tests_passed":42,"mutation_score_pct":92.5}`
	customPayload := parseTechniqueOutput("unit", "rust", "custom", customOut, "")
	if customPayload.TestsPassed != 42 || customPayload.MutationScorePct == nil || *customPayload.MutationScorePct != 92.5 {
		t.Fatalf("unexpected [ARC_CODE_METRIC] parse: %+v", customPayload)
	}
}

// TestCodeRunPreservesHistoricalFailGatesExplainsAndExportsSnapshot 验证代码测试执行保留历史FAIL并完成门禁解释与快照导出
func TestCodeRunPreservesHistoricalFailGatesExplainsAndExportsSnapshot(t *testing.T) {
	dir := t.TempDir()
	ledger, err := openStoryLedger(filepath.Join(dir, ".arc", "verify", "story.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	makePlan := func(covExit int, covLine float64, mutScore float64) *CodeVerifyPlan {
		p := &CodeVerifyPlan{
			PlanID:          "demo_rust_verify",
			RepoRoot:        dir,
			PrimaryLanguage: "rust",
			MaxTier:         TierP1,
			Profile: RepoCodeProfile{
				RepoRoot:        dir,
				PrimaryLanguage: "rust",
				Languages:       []string{"rust"},
				RiskSignals:     []RiskSignalHit{{Signal: "has_critical_core_logic", Evidence: "ledger transfer", File: "src/lib.rs", Line: 12}},
			},
			Techniques: []PlannedTechnique{
				{
					Index: 1, TechniqueID: "unit", ChineseName: "单元测试", EnglishName: "Unit Testing",
					Priority: TierP0, EffectivePriority: TierP0, Status: SelectRequired, Language: "rust", ToolName: "sh",
					ToolAvailable: true, HarnessReady: true,
					Command:         `printf 'test result: ok. 8 passed; 0 failed; 0 ignored\n'`,
					FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "zero failures"},
				},
				{
					Index: 3, TechniqueID: "coverage", ChineseName: "代码覆盖率分析", EnglishName: "Code Coverage",
					Priority: TierP0, EffectivePriority: TierP0, Status: SelectRequired, Language: "rust", ToolName: "cargo-llvm-cov",
					ToolAvailable: true, HarnessReady: true,
					Command:         `printf '{"data":[{"totals":{"lines":{"count":10,"covered":9,"percent":` + strconv.FormatFloat(covLine, 'f', 1, 64) + `},"functions":{"count":2,"covered":2,"percent":100.0},"regions":{"count":10,"covered":9,"percent":90.0}}}]}\n' && exit ` + strconv.Itoa(covExit),
					FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MinLineCoveragePct: 80.0, Summary: "line >= 80%"},
				},
				{
					Index: 6, TechniqueID: "mutation", ChineseName: "变异测试", EnglishName: "Mutation Testing",
					Priority: TierP1, EffectivePriority: TierP1, Status: SelectRequired, TopOverlooked: true, Language: "rust", ToolName: "cargo-mutants",
					ToolAvailable: true, HarnessReady: true,
					Command:         `printf '[ARC_CODE_METRIC]{"mutants_caught":9,"mutants_missed":1,"mutation_score_pct":` + strconv.FormatFloat(mutScore, 'f', 1, 64) + `}\n'`,
					FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MinMutationScorePct: 80.0, Summary: "mutation >= 80%"},
				},
			},
			RequiredCount: 3,
			CreatedAt:     nowUTC(),
		}
		p.PlanSHA256 = computeCodePlanSHA256(p)
		return p
	}

	// Run 1: coverage and mutation fail -> 2 historical FAIL receipts stored.
	failPlan := makePlan(1, 55.0, 60.0)
	sumFail, err := runCodeVerifyPlan(ledger, failPlan, CodeRunOptions{RepoRoot: dir})
	if err != nil {
		t.Fatalf("runCodeVerifyPlan fail run: %v", err)
	}
	if sumFail.Run.Verdict != "FAIL" || sumFail.HistoricalFailCount != 2 || sumFail.Gate.Verdict != "FAIL" {
		t.Fatalf("expected FAIL with 2 historical fails, got run=%+v hist=%d gate=%+v", sumFail.Run, sumFail.HistoricalFailCount, sumFail.Gate)
	}

	// Run 2: passing run -> gate PASS while historical FAIL count remains 2!
	passPlan := makePlan(0, 92.0, 90.0)
	sumPass, err := runCodeVerifyPlan(ledger, passPlan, CodeRunOptions{RepoRoot: dir})
	if err != nil {
		t.Fatalf("runCodeVerifyPlan pass run: %v", err)
	}
	if sumPass.Run.Verdict != "PASS" || sumPass.Gate.Verdict != "PASS" || sumPass.HistoricalFailCount != 2 {
		t.Fatalf("expected PASS with preserved historicalFailCount=2, got run=%+v gate=%+v hist=%d", sumPass.Run, sumPass.Gate, sumPass.HistoricalFailCount)
	}
	for _, r := range sumPass.Results {
		if len(r.ReceiptSHA256) != 64 || len(r.ArtifactSHA256) != 64 {
			t.Fatalf("expected 64-hex hashes on result %+v", r)
		}
		if _, err := os.Stat(filepath.Join(dir, r.ArtifactPath, "metrics.json")); err != nil {
			t.Fatalf("expected artifact metrics.json for %s: %v", r.TechniqueID, err)
		}
	}

	rep, err := explainCodeVerify(ledger, passPlan.PlanID, sumPass.Run.RunID)
	if err != nil {
		t.Fatalf("explainCodeVerify: %v", err)
	}
	md := renderCodeExplainMarkdown(rep)
	if !strings.Contains(md, "NOT_MEASURED") || !strings.Contains(md, "92.0%") {
		t.Fatalf("explain markdown missing coverage breakdown details:\n%s", md)
	}

	snapDir := filepath.Join(dir, "code-snapshot")
	if err := exportCodeSnapshot(ledger, passPlan.PlanID, sumPass.Run.RunID, snapDir); err != nil {
		t.Fatalf("exportCodeSnapshot: %v", err)
	}
	htmlBytes, err := os.ReadFile(filepath.Join(snapDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlBytes[:min(1024, len(htmlBytes))]), `<meta charset="UTF-8">`) {
		t.Fatal("code snapshot HTML missing UTF-8 charset in first 1024 bytes")
	}
}

// TestCodeRunDryRunAndMissingToolAreBlockedNotPass 验证空跑、缺失工具、缺失夹具与零用例均诚实判定为BLOCKED
func TestCodeRunDryRunAndMissingToolAreBlockedNotPass(t *testing.T) {
	dir := t.TempDir()
	ledger, err := openStoryLedger(filepath.Join(dir, "story.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	plan := &CodeVerifyPlan{
		PlanID:          "blocked_cases",
		RepoRoot:        dir,
		PrimaryLanguage: "rust",
		MaxTier:         TierP1,
		Techniques: []PlannedTechnique{
			{
				Index: 1, TechniqueID: "unit", Priority: TierP0, EffectivePriority: TierP0,
				Status: SelectRequired, Language: "rust", ToolName: "sh",
				ToolAvailable: true, HarnessReady: true,
				Command: `printf 'running 0 tests\n'`,
			},
			{
				Index: 4, TechniqueID: "fuzz", Priority: TierP1, EffectivePriority: TierP0,
				Status: SelectRequired, Language: "rust", ToolName: "cargo-fuzz",
				ToolAvailable: true, HarnessRequired: true, HarnessReady: false,
				MissingPrerequisite: "missing fuzz target",
				Command:             `printf 'ok\n'`,
			},
			{
				Index: 6, TechniqueID: "mutation", Priority: TierP1, EffectivePriority: TierP1,
				Status: SelectRequired, Language: "rust", ToolName: "cargo-mutants",
				ToolAvailable: false, InstallHint: "cargo install cargo-mutants",
				Command: `cargo mutants`,
			},
		},
		RequiredCount: 3,
	}
	plan.PlanSHA256 = computeCodePlanSHA256(plan)

	// Dry-run MUST block all techniques.
	drySum, err := runCodeVerifyPlan(ledger, plan, CodeRunOptions{RepoRoot: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if drySum.Run.Verdict != "BLOCKED" || drySum.Run.BlockedCount != 3 {
		t.Fatalf("expected all 3 blocked on dry-run, got %+v", drySum.Run)
	}

	// Live run MUST block zero-tests-ran, missing harness, and missing tool.
	liveSum, err := runCodeVerifyPlan(ledger, plan, CodeRunOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if liveSum.Run.Verdict != "BLOCKED" || liveSum.Run.BlockedCount != 3 {
		t.Fatalf("expected all 3 blocked on live run, got results=%+v", liveSum.Results)
	}
}

// TestCodeVerifyTaskProofIntegration 验证代码级测试计划通过后能签发符合原生完成门禁协议的task-proof
func TestCodeVerifyTaskProofIntegration(t *testing.T) {
	dir := t.TempDir()
	ledger, err := openStoryLedger(filepath.Join(dir, "story.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	plan := &CodeVerifyPlan{
		PlanID:          "rust_proof_plan",
		RepoRoot:        dir,
		PrimaryLanguage: "rust",
		MaxTier:         TierP0,
		Techniques: []PlannedTechnique{
			{
				Index: 1, TechniqueID: "unit", Priority: TierP0, EffectivePriority: TierP0,
				Status: SelectRequired, Language: "rust", ToolName: "sh",
				ToolAvailable: true, HarnessReady: true,
				Command:         `printf 'test result: ok. 3 passed; 0 failed; 0 ignored\n'`,
				FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "zero failures"},
			},
		},
		RequiredCount: 1,
	}
	plan.PlanSHA256 = computeCodePlanSHA256(plan)

	if _, err := runCodeVerifyPlan(ledger, plan, CodeRunOptions{RepoRoot: dir}); err != nil {
		t.Fatal(err)
	}

	// Capture stdout from tryEmitCodeTaskProof for both single technique and 'all'.
	for _, task := range []string{"rust_proof_plan/unit", "rust_proof_plan/all"} {
		oldOut := os.Stdout
		rPipe, wPipe, pErr := os.Pipe()
		if pErr != nil {
			t.Fatal(pErr)
		}
		os.Stdout = wPipe
		exitCode, handled := tryEmitCodeTaskProof(ledger, "rust_proof_plan", task)
		_ = wPipe.Close()
		os.Stdout = oldOut

		buf := make([]byte, 16384)
		n, _ := rPipe.Read(buf)
		_ = rPipe.Close()

		if !handled || exitCode != 0 {
			t.Fatalf("expected handled=true exit=0 for %s, got handled=%v exit=%d out=%s", task, handled, exitCode, string(buf[:n]))
		}
		var proof taskProofResponse
		if err := json.Unmarshal(buf[:n], &proof); err != nil {
			t.Fatalf("unmarshal taskProofResponse: %v (%s)", err, string(buf[:n]))
		}
		if proof.Version != 1 || proof.Verdict != "PASS" || proof.ProofScope != "linked_task_evidence" ||
			!proof.InputsUnchanged || len(proof.ReceiptSHA256) != 64 || len(proof.RAWSHA256) == 0 {
			t.Fatalf("invalid code task-proof response: %+v", proof)
		}
	}
}

// TestCodeVerifyStaleSourceCrossRunStitchingZeroDoctestAndNilMetrics 验证代码证明防修改、跨提交拼装拦截、零用例doctest及空度量阻塞
func TestCodeVerifyStaleSourceCrossRunStitchingZeroDoctestAndNilMetrics(t *testing.T) {
	// 1. Zero tests ran on doctest and nil coverage/mutation metrics MUST evaluate to BLOCKED!
	vCov, reasonCov := evaluateCodeFailureCriteria(CodeFailureCriteria{MinLineCoveragePct: 80.0}, 0, CodeMetricPayload{})
	if vCov != VerdictBlocked || !strings.Contains(reasonCov, "coverage") {
		t.Fatalf("expected BLOCKED for nil coverage metric, got %s (%s)", vCov, reasonCov)
	}
	vMut, reasonMut := evaluateCodeFailureCriteria(CodeFailureCriteria{MinMutationScorePct: 80.0}, 0, CodeMetricPayload{})
	if vMut != VerdictBlocked || !strings.Contains(reasonMut, "mutation_score_pct") {
		t.Fatalf("expected BLOCKED for nil mutation metric, got %s (%s)", vMut, reasonMut)
	}

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger, err := openStoryLedger(filepath.Join(dir, ".arc", "verify", "story.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	// Test doctest with zero tests ran -> BLOCKED.
	docPlan := &CodeVerifyPlan{
		PlanID:          "doctest_zero_plan",
		RepoRoot:        dir,
		PrimaryLanguage: "rust",
		MaxTier:         TierP1,
		Techniques: []PlannedTechnique{
			{
				Index: 25, TechniqueID: "doctest", Priority: TierP1, EffectivePriority: TierP1,
				Status: SelectRequired, Language: "rust", ToolName: "sh",
				ToolAvailable: true, HarnessReady: true,
				Command: `printf 'running 0 tests\ntest result: ok. 0 passed; 0 failed; 0 ignored\n'`,
			},
		},
		RequiredCount: 1,
	}
	docPlan.PlanSHA256 = computeCodePlanSHA256(docPlan)
	docSum, err := runCodeVerifyPlan(ledger, docPlan, CodeRunOptions{RepoRoot: dir})
	if err != nil {
		t.Fatal(err)
	}
	if docSum.Run.Verdict != "BLOCKED" || docSum.Run.BlockedCount != 1 {
		t.Fatalf("expected BLOCKED for zero doctest execution, got %+v results=%+v", docSum.Run, docSum.Results)
	}

	// 2. Modified source invalidates single proof and rejects cross-revision partial stitching on 'all'.
	stitchPlan := &CodeVerifyPlan{
		PlanID:          "stitch_guard_plan",
		RepoRoot:        dir,
		PrimaryLanguage: "rust",
		MaxTier:         TierP1,
		Techniques: []PlannedTechnique{
			{
				Index: 1, TechniqueID: "unit", Priority: TierP0, EffectivePriority: TierP0,
				Status: SelectRequired, Language: "rust", ToolName: "sh",
				ToolAvailable: true, HarnessReady: true,
				Command:         `printf 'test result: ok. 2 passed; 0 failed; 0 ignored\n'`,
				FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "zero failures"},
			},
			{
				Index: 3, TechniqueID: "coverage", Priority: TierP0, EffectivePriority: TierP0,
				Status: SelectRequired, Language: "rust", ToolName: "sh",
				ToolAvailable: true, HarnessReady: true,
				Command:         `printf '[ARC_CODE_METRIC]{"coverage":{"line_pct":95.0}}\n'`,
				FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MinLineCoveragePct: 80.0, Summary: "line >= 80%"},
			},
		},
		RequiredCount: 2,
	}
	stitchPlan.PlanSHA256 = computeCodePlanSHA256(stitchPlan)
	if _, err := runCodeVerifyPlan(ledger, stitchPlan, CodeRunOptions{RepoRoot: dir, Techniques: []string{"unit"}}); err != nil {
		t.Fatal(err)
	}
	// Mutate source before running only 'coverage': 'coverage' is fresh on v2, 'unit' is stale on v1!
	if err := os.WriteFile(srcFile, []byte("package main\nfunc main() { _ = 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCodeVerifyPlan(ledger, stitchPlan, CodeRunOptions{RepoRoot: dir, Techniques: []string{"coverage"}}); err != nil {
		t.Fatal(err)
	}

	captureCodeProof := func(obligation string) (int, taskProofResponse) {
		oldOut := os.Stdout
		rPipe, wPipe, pErr := os.Pipe()
		if pErr != nil {
			t.Fatal(pErr)
		}
		os.Stdout = wPipe
		code, handled := tryEmitCodeTaskProofWithObligation(ledger, "stitch_guard_plan", "stitch_guard_plan/"+obligation, obligation)
		_ = wPipe.Close()
		os.Stdout = oldOut
		if !handled {
			t.Fatalf("expected handled=true for stitch_guard_plan/%s", obligation)
		}
		var buf [16384]byte
		n, _ := rPipe.Read(buf[:])
		_ = rPipe.Close()
		var resp taskProofResponse
		if uErr := json.Unmarshal(buf[:n], &resp); uErr != nil {
			t.Fatalf("unmarshal code proof: %v (%s)", uErr, string(buf[:n]))
		}
		return code, resp
	}

	if code, pCov := captureCodeProof("coverage"); code != 0 || pCov.Verdict != "PASS" || !pCov.InputsUnchanged {
		t.Fatalf("expected fresh single obligation 'coverage' to PASS, got code=%d proof=%+v", code, pCov)
	}
	if code, pUnit := captureCodeProof("unit"); code != 1 || pUnit.Verdict != "NOT_RUN" || pUnit.EvidenceStatus != "STALE" {
		t.Fatalf("expected stale 'unit' to be rejected as NOT_RUN/STALE, got code=%d proof=%+v", code, pUnit)
	}
	if code, pAll := captureCodeProof("all"); code != 1 || pAll.Verdict != "NOT_RUN" || pAll.EvidenceStatus != "STALE" {
		t.Fatalf("expected cross-revision 'all' stitching to be rejected as NOT_RUN/STALE, got code=%d proof=%+v", code, pAll)
	}

	// 3. Verify extended parsers (govulncheck, pip-audit, ruff, pytest, extractOuterJSON noise, Cargo.toml SHA).
	goVuln := parseNativeSecurityAudit("dependency_audit", "{\"osv\":{\"id\":\"IGNORE\"}}\n{\"finding\":{\"osv\":\"GO-2026-0001\"}}\n")
	pipAud := parseNativeSecurityAudit("dependency_audit", "INFO: scanning\n[{\"name\":\"pkg\",\"vulns\":[{\"id\":\"PYSEC-1\"}]}]")
	ruffW := parseNativeStaticWarnings("python", "src/a.py:3:1: F401 unused import\nFound 1 error.\n")
	noisyCov := parseNativeCoverage("rust", "cargo-llvm-cov", "log {not json}\n{\"data\":[{\"totals\":{\"lines\":{\"count\":10,\"covered\":9,\"percent\":90.0}}}]}", "")
	if goVuln.HighVulns != 1 || pipAud.HighVulns != 1 || ruffW.Warnings != 1 || noisyCov.Coverage == nil || noisyCov.Coverage.LinePct == nil || *noisyCov.Coverage.LinePct != 90.0 {
		t.Fatalf("unexpected extended parser results: goVuln=%+v pip=%+v ruff=%+v cov=%+v", goVuln, pipAud, ruffW, noisyCov)
	}
	pA := &CodeVerifyPlan{PlanID: "x", PrimaryLanguage: "rust", MaxTier: TierP1, RepoRoot: "/tmp/a"}
	pB := &CodeVerifyPlan{PlanID: "x", PrimaryLanguage: "rust", MaxTier: TierP1, RepoRoot: "/tmp/b", Profile: RepoCodeProfile{RepoRoot: "/tmp/b", ToolProbes: []ToolProbeStatus{{ToolName: "go"}}}}
	if computeCodePlanSHA256(pA) != computeCodePlanSHA256(pB) {
		t.Fatal("expected deterministic PlanSHA256 independent of RepoRoot and ToolProbes")
	}
}
