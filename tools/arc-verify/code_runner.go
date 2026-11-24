package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runCodeVerifyPlan 按装配计划执行必选及可选的代码级测试技术并持久化不可变账本收据
// 请求体
// ledger 验证账本实例指针
// plan 待执行的代码测试计划指针
// opts 运行时控制选项
// 响应体
// summary 本次完整执行汇总报告与门禁评估
// err 计划参数非法、无可运行项或账本写入错误
func runCodeVerifyPlan(ledger *StoryLedger, plan *CodeVerifyPlan, opts CodeRunOptions) (*CodeRunSummary, error) {
	// 1. 解析并归一化仓库根目录与当前源码修订指纹，并持久化计划定义
	if plan == nil || strings.TrimSpace(plan.PlanID) == "" {
		return nil, errors.New("code verify plan and plan_id are required")
	}
	repoRoot := opts.RepoRoot
	if strings.TrimSpace(repoRoot) == "" {
		repoRoot = plan.RepoRoot
	}
	absRoot := canonicalRepoRoot(repoRoot)
	plan.RepoRoot = absRoot
	if plan.PlanSHA256 == "" {
		plan.PlanSHA256 = computeCodePlanSHA256(plan)
	}
	if err := ledger.SaveCodePlan(plan); err != nil {
		return nil, err
	}

	// 2. 初始化工具链探针缓存（非DryRun）并筛选本批次待执行的测试技术列表
	var cache *toolProbeCache
	if !opts.DryRun {
		cache = buildToolProbeCache(plan.Profile.Languages)
	}
	selected, err := selectRunnableTechniques(plan, opts, cache)
	if err != nil {
		return nil, err
	}
	runID := newCodeRunID()
	startedAt := nowUTC()
	sourceCommit := currentGitHead(absRoot)
	sourceTreeSHA := sourceWorktreeDigestExcluding(absRoot, ledger.path)

	// 3. 逐项执行测试、解析产物指标并追加带修订绑定的不可变收据
	results := make([]CodeResultRecord, 0, len(selected))
	passedN, failedN, blockedN := 0, 0, 0
	for _, tech := range selected {
		rec := executeCodeTechnique(plan, tech, runID, absRoot, sourceCommit, sourceTreeSHA, opts, cache)
		if err := ledger.AppendCodeResult(rec); err != nil {
			return nil, err
		}
		results = append(results, rec)
		switch Verdict(rec.Verdict) {
		case VerdictPass:
			passedN++
		case VerdictFail:
			failedN++
		default:
			blockedN++
		}
	}

	// 4. 汇总运行批次总判定并写入 code_verify_runs 表
	runVerdict := string(VerdictPass)
	if failedN > 0 {
		runVerdict = string(VerdictFail)
	} else if blockedN > 0 {
		runVerdict = string(VerdictBlocked)
	}
	runRec := CodeRunRecord{
		RunID:            runID,
		PlanID:           plan.PlanID,
		PlanSHA256:       plan.PlanSHA256,
		RepoRoot:         absRoot,
		SourceCommit:     sourceCommit,
		SourceTreeSHA256: sourceTreeSHA,
		MaxTier:          string(plan.MaxTier),
		Verdict:          runVerdict,
		RequiredTotal:    len(selected),
		PassedCount:      passedN,
		FailedCount:      failedN,
		BlockedCount:     blockedN,
		StartedAt:        startedAt,
		FinishedAt:       nowUTC(),
	}
	if err := ledger.RecordCodeRun(runRec); err != nil {
		return nil, err
	}
	histFails, err := ledger.CountHistoricalCodeFails(plan.PlanID)
	if err != nil {
		return nil, err
	}
	gateRep, err := computeCodeGate(ledger, plan.PlanID, runID, plan.MaxTier, false)
	if err != nil {
		return nil, err
	}
	return &CodeRunSummary{
		Run:                 runRec,
		Results:             results,
		HistoricalFailCount: histFails,
		Gate:                gateRep,
	}, nil
}

// executeCodeTechnique 执行单项目标代码测试技术、落盘原始产物并生成含修订签名的不可变收据
// 请求体
// plan 所属代码测试计划指针
// tech 当前计划条目
// runID 运行批次ID
// repoRoot 已归一化的仓库根目录绝对路径
// sourceCommit 当前仓库Git提交哈希
// sourceTreeSHA 当前工作树内容摘要
// opts 运行控制选项
// cache 工具探针缓存指针
// 响应体
// rec 含内容与产物双重SHA256签名的执行收据
func executeCodeTechnique(
	plan *CodeVerifyPlan,
	tech PlannedTechnique,
	runID, repoRoot, sourceCommit, sourceTreeSHA string,
	opts CodeRunOptions,
	cache *toolProbeCache,
) CodeResultRecord {
	// 1. 构造含源码修订绑定的基础收据与产物目录路径
	artDir := filepath.Join(repoRoot, ".arc", "verify", "code", runID, tech.TechniqueID)
	effPriority := string(tech.EffectivePriority)
	if effPriority == "" {
		effPriority = string(tech.Priority)
	}
	rec := CodeResultRecord{
		RunID:            runID,
		PlanID:           plan.PlanID,
		PlanSHA256:       plan.PlanSHA256,
		TechniqueID:      tech.TechniqueID,
		Priority:         effPriority,
		Language:         tech.Language,
		ToolName:         tech.ToolName,
		ExecutedCmd:      tech.Command,
		ArtifactPath:     filepath.ToSlash(filepath.Join(".arc", "verify", "code", runID, tech.TechniqueID)),
		RepoRoot:         repoRoot,
		SourceCommit:     sourceCommit,
		SourceTreeSHA256: sourceTreeSHA,
		CreatedAt:        nowUTC(),
	}

	// 2. 拦截 DryRun、空命令、缺失工具或缺失夹具状态（诚实判 BLOCKED，绝不伪造 PASS）
	if opts.DryRun {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = "dry-run only; no real commands executed"
		rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
		return rec
	}
	if strings.TrimSpace(tech.Command) == "" {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = "no executable command configured for technique " + tech.TechniqueID
		rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
		return rec
	}
	if !tech.ToolAvailable {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = nonEmpty(tech.MissingPrerequisite, "missing tool: "+tech.ToolName)
		if tech.InstallHint != "" {
			rec.MissingPrerequisite += " (install: " + tech.InstallHint + ")"
		}
		rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
		return rec
	}
	if liveOK, liveReason := verifyLiveCommandReady(tech.Command, tech, cache); !liveOK {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = liveReason
		if tech.InstallHint != "" {
			rec.MissingPrerequisite += " (install: " + tech.InstallHint + ")"
		}
		rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
		return rec
	}
	if tech.HarnessRequired && !tech.HarnessReady && !opts.AllowUncheckedHarness {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = nonEmpty(tech.MissingPrerequisite, "missing required test harness for "+tech.TechniqueID)
		rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
		return rec
	}

	// 3. 创建产物目录并以带超时上下文运行真实测试命令
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = "create artifact dir: " + err.Error()
		rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
		return rec
	}
	timeoutSec := opts.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", tech.Command)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"ARC_CODE_RUN_ID="+runID,
		"ARC_CODE_PLAN_ID="+plan.PlanID,
		"ARC_CODE_TECHNIQUE="+tech.TechniqueID,
		"ARC_CODE_ARTIFACT_DIR="+artDir,
	)
	t0 := time.Now()
	outBytes, runErr := cmd.CombinedOutput()
	rec.DurationMS = time.Since(t0).Milliseconds()
	rawOut := strings.TrimSpace(string(outBytes))
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			rec.ExitCode = exitErr.ExitCode()
		} else {
			rec.ExitCode = 1
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			rawOut = strings.TrimSpace(rawOut + fmt.Sprintf("\n[arc-verify timeout after %ds]", timeoutSec))
		}
	}

	// 4. 解析测试度量载荷、判定零用例/未测量阻断状态与门槛结果并落盘不可变产物
	metrics := parseTechniqueOutput(tech.TechniqueID, tech.Language, tech.ToolName, rawOut, artDir)
	rec.Metrics = metrics
	rec.RawOutput = truncateCodeOutput(rawOut, 64<<10)

	if isZeroTestsResult(tech.TechniqueID, rec.ExitCode, metrics, rawOut) {
		rec.Verdict = string(VerdictBlocked)
		rec.MissingPrerequisite = "zero tests ran in target repository"
	} else {
		verdict, reason := evaluateCodeFailureCriteria(tech.FailureCriteria, rec.ExitCode, metrics)
		rec.Verdict = string(verdict)
		if verdict == VerdictBlocked {
			rec.MissingPrerequisite = reason
		} else {
			rec.FailureReason = reason
		}
	}

	_ = os.WriteFile(filepath.Join(artDir, "output.log"), []byte(rawOut+"\n"), 0o644)
	if mJSON, mErr := json.MarshalIndent(metrics, "", "  "); mErr == nil {
		_ = os.WriteFile(filepath.Join(artDir, "metrics.json"), append(mJSON, '\n'), 0o644)
	}
	rec.ArtifactSHA256 = hashPath(artDir)
	rec.ReceiptSHA256 = computeCodeResultSHA256(rec)
	return rec
}
