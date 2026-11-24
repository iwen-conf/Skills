package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	stepEvidencePrefix   = "[ARC_STEP_EVIDENCE]"
	softlockMarkerPrefix = "[ARC_SOFTLOCK]"
)

// PlayOptions 描述单次Storyline推演执行的控制选项
type PlayOptions struct {
	// Parallel 是否启用多路由并发隔离执行
	Parallel bool
	// RouteFilter 可选单条路由过滤ID
	RouteFilter string
	// Blind 是否启用盲测掩码模式
	Blind bool
	// DryRun 是否仅校验装配而不执行真实命令
	DryRun bool
	// RepoRoot 目标仓库根目录路径
	RepoRoot string
	// Engine 检查点存储引擎覆盖选项
	Engine string
	// PGURL PostgreSQL连接串
	PGURL string
}

// PlaySummary 描述单次Storyline完整推演的汇总报告
type PlaySummary struct {
	// RunID 运行批次唯一ID
	RunID string `json:"run_id"`
	// StoryID 故事线唯一标识
	StoryID string `json:"story_id"`
	// StorySHA256 故事线原文SHA256摘要
	StorySHA256 string `json:"story_sha256"`
	// Verdict 批次总判定结果
	Verdict string `json:"verdict"`
	// BlindMode 是否开启盲测模式
	BlindMode bool `json:"blind_mode"`
	// Parallel 是否开启并发执行
	Parallel bool `json:"parallel"`
	// Checkpoints 构建完成的检查点列表
	Checkpoints []CheckpointRecord `json:"checkpoints"`
	// RouteResults 各分支路由执行收据列表
	RouteResults []RouteResultRecord `json:"route_results"`
	// StepTraces 全量六维步骤轨迹列表
	StepTraces []StepTraceRecord `json:"step_traces,omitempty"`
	// Completion 四维100%完成度报告
	Completion *CompletionReport `json:"completion"`
}

// routeExecutionOutcome 封装单条路由执行产出的路由收据与步骤轨迹集合
type routeExecutionOutcome struct {
	// route 路由级判定收据
	route RouteResultRecord
	// traces 步骤级六维轨迹切片
	traces []StepTraceRecord
}

// playStory 按拓扑序构建检查点并串行或并发执行全部分支路由六维验证
// 请求体
// ledger 验证账本实例指针
// s 待执行的故事线定义指针
// opts 推演控制参数
// 响应体
// summary 推演批次完整汇总报告
// err 静态审查不通过、检查点构建失败或账本持久化错误
func playStory(ledger *StoryLedger, s *Storyline, opts PlayOptions) (*PlaySummary, error) {
	// 1. 执行前置DAG与反软锁静态审查并保存故事线定义
	lint := lintStory(s, nil, 10)
	if Verdict(lint.Status) != VerdictPass {
		return nil, fmt.Errorf("storyline lint failed (%d errors): %s", lint.TotalErrors, lint.Errors[0].Message)
	}
	absRoot := canonicalRepoRoot(opts.RepoRoot)
	sourceCommit := currentGitHead(absRoot)
	sourceTreeSHA := sourceWorktreeDigestExcluding(absRoot, ledger.path)
	if err := ledger.SaveStoryDefinition(s); err != nil {
		return nil, err
	}
	// 2. 按拓扑序构建全量检查点并筛选本次待执行的分支路由
	cpRecords, err := buildCheckpoints(ledger, s, absRoot, opts.Engine, "", opts.PGURL)
	if err != nil {
		return nil, err
	}
	cpEngines := map[string]string{}
	for _, cp := range cpRecords {
		cpEngines[cp.SaveID] = cp.Engine
	}
	var selected []Route
	for _, r := range s.Routes {
		if opts.RouteFilter != "" && r.RouteID != opts.RouteFilter {
			continue
		}
		selected = append(selected, r)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no matching routes to play (filter=%q)", opts.RouteFilter)
	}
	// 3. 在独立命名空间内串行或并发执行选定路由
	runID := newRunID()
	startedAt := nowUTC()
	outcomes := make([]routeExecutionOutcome, len(selected))
	execOne := func(idx int, r Route) {
		outcomes[idx] = executeRoute(s, r, runID, absRoot, cpEngines[r.LoadSave], opts)
	}
	if opts.Parallel && len(selected) > 1 {
		var wg sync.WaitGroup
		for i, r := range selected {
			wg.Add(1)
			go func(idx int, route Route) {
				defer wg.Done()
				execOne(idx, route)
			}(i, r)
		}
		wg.Wait()
	} else {
		for i, r := range selected {
			execOne(i, r)
		}
	}
	// 4. 追加持久化不可变路由收据与六维步骤轨迹并汇总批次总判定
	runVerdict := string(VerdictPass)
	results := make([]RouteResultRecord, 0, len(outcomes))
	var allTraces []StepTraceRecord
	for _, out := range outcomes {
		out.route.RepoRoot = absRoot
		out.route.SourceCommit = sourceCommit
		out.route.SourceTreeSHA256 = sourceTreeSHA
		out.route.ReceiptSHA256 = computeRouteReceiptSHA256(out.route)
		if err := ledger.AppendRouteResult(out.route); err != nil {
			return nil, err
		}
		for _, tr := range out.traces {
			if err := ledger.AppendStepTrace(tr); err != nil {
				return nil, err
			}
			allTraces = append(allTraces, tr)
		}
		results = append(results, out.route)
		if Verdict(out.route.Verdict) == VerdictFail {
			runVerdict = string(VerdictFail)
		} else if Verdict(out.route.Verdict) != VerdictPass && Verdict(runVerdict) == VerdictPass {
			runVerdict = out.route.Verdict
		}
	}
	if err := ledger.RecordRun(runID, s.StoryID, s.SHA256, absRoot, sourceCommit, sourceTreeSHA, opts.Blind, runVerdict, startedAt); err != nil {
		return nil, err
	}
	// 5. 计算全故事线四维完成度并返回汇总报告
	comp, err := computeCompletion(ledger, s, nil)
	if err != nil {
		return nil, err
	}
	return &PlaySummary{
		RunID:        runID,
		StoryID:      s.StoryID,
		StorySHA256:  s.SHA256,
		Verdict:      runVerdict,
		BlindMode:    opts.Blind,
		Parallel:     opts.Parallel,
		Checkpoints:  cpRecords,
		RouteResults: results,
		StepTraces:   allTraces,
		Completion:   comp,
	}, nil
}

// executeRoute 克隆隔离分片快照并编排单条路由的步骤级与路由级六维验证
// 请求体
// s 故事线定义指针
// r 当前待执行路由定义
// runID 运行批次ID
// repoRoot 仓库根目录绝对路径
// engine 检查点存储引擎类型
// opts 推演控制参数
// 响应体
// outcome 包含路由收据与六维步骤轨迹的执行结果
func executeRoute(s *Storyline, r Route, runID, repoRoot, engine string, opts PlayOptions) routeExecutionOutcome {
	// 1. 克隆检查点至路由专属隔离命名空间并初始化跨步骤变量文件
	clonePath, workDir, err := cloneCheckpointWithDir(repoRoot, s.StoryID, r.LoadSave, r.RuntimeNS, engine, opts.PGURL)
	if err != nil {
		return routeExecutionOutcome{
			route: finalizeRouteRecord(s, r, runID, string(VerdictBlocked), false, 0, 0, "", "clone checkpoint failed: "+err.Error()),
		}
	}
	varsFile := filepath.Join(workDir, "arc_vars.json")
	routeVars := FactMap{}
	_ = os.WriteFile(varsFile, []byte("{}\n"), 0o644)
	baseEnv := append(os.Environ(),
		"SHARD_NS="+r.RuntimeNS,
		"ARC_STORY_ID="+s.StoryID,
		"ARC_ROUTE_ID="+r.RouteID,
		"ARC_LOAD_SAVE="+r.LoadSave,
		"ARC_CLONE_PATH="+clonePath,
		"ARC_VARS_FILE="+varsFile,
	)
	if opts.Blind {
		baseEnv = append(baseEnv, "ARC_BLIND_MODE=1", "ARC_BLIND_MASK="+strings.Join(s.BlindMask, ","))
	}
	// 2. 统计探针总数并拦截DryRun或无任何真实执行命令的空壳路由
	probesTotal := 0
	hasStepCmds := false
	for _, st := range r.Sequence {
		probesTotal += len(st.ButterflyProbes)
		if strings.TrimSpace(st.PrecheckCmd) != "" || strings.TrimSpace(st.Cmd) != "" || strings.TrimSpace(st.ProbeCmd) != "" {
			hasStepCmds = true
		}
	}
	if opts.DryRun {
		return routeExecutionOutcome{
			route: finalizeRouteRecord(s, r, runID, string(VerdictBlocked), false, probesTotal, 0, "", "dry-run only; no real commands executed"),
		}
	}
	hasRouteVerifyCmd := strings.TrimSpace(r.VerifyCmd) != ""
	if !hasStepCmds && !hasRouteVerifyCmd {
		traces := make([]StepTraceRecord, 0, len(r.Sequence))
		for _, st := range r.Sequence {
			traces = append(traces, finalizeStepTrace(s, r, st, runID, "", 0, nil, string(VerdictBlocked), "no executable command configured", ""))
		}
		return routeExecutionOutcome{
			route:  finalizeRouteRecord(s, r, runID, string(VerdictBlocked), false, probesTotal, 0, "", "route has no executable verify_cmd or step cmd; real execution required for PASS"),
			traces: traces,
		}
	}
	// 3. 顺序执行步骤级命令与路由级收尾校验命令并汇总最终结局
	var logBuf strings.Builder
	var traces []StepTraceRecord
	probesPassed := 0
	softlock := false
	if hasStepCmds {
		outcome, passed, locked, halted := executeDeclaredSteps(s, r, runID, repoRoot, varsFile, baseEnv, routeVars, probesTotal, &logBuf, &traces)
		probesPassed = passed
		softlock = locked
		if halted {
			return outcome
		}
	}
	if hasRouteVerifyCmd {
		outcome, passed, locked, halted := executeRouteVerifyCommand(s, r, runID, repoRoot, varsFile, baseEnv, routeVars, hasStepCmds, probesTotal, probesPassed, softlock, &logBuf, &traces)
		probesPassed = passed
		softlock = locked
		if halted {
			return outcome
		}
	}
	if softlock {
		return routeExecutionOutcome{
			route:  finalizeRouteRecord(s, r, runID, string(VerdictFail), true, probesTotal, probesPassed, "", logBuf.String()),
			traces: traces,
		}
	}
	return routeExecutionOutcome{
		route:  finalizeRouteRecord(s, r, runID, string(VerdictPass), false, probesTotal, probesPassed, r.UnlocksEnding, logBuf.String()),
		traces: traces,
	}
}
