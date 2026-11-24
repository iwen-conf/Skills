package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// cmdPlay 执行分支故事线路由验证并落盘不可变六维步骤收据
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdPlay(args []string) int {
	// 1. 解析并发、路由过滤、盲测与数据库标志
	fl := flag.NewFlagSet("play", flag.ContinueOnError)
	parallel := fl.Bool("parallel", false, "execute routes concurrently in isolated namespaces")
	route := fl.String("route", "", "execute only one route_id")
	blind := fl.Bool("blind", false, "enable blind-test mode with ARC_BLIND_MASK")
	dryRun := fl.Bool("dry-run", false, "validate wiring without executing route commands")
	repoDir := fl.String("repo", ".", "repository root")
	engine := fl.String("engine", "", "override checkpoint engine: dir | sqlite | postgres")
	pgURL := fl.String("pg-url", "", "PostgreSQL URL when engine=postgres")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify play <story.yaml> [--parallel] [--route ID] [--blind] [--db PATH]")
		return 2
	}
	// 2. 加载故事线并打开验证账本执行路由推演
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify play: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify play: %v\n", err)
		return 1
	}
	defer ledger.Close()
	summary, err := playStory(ledger, s, PlayOptions{
		Parallel:    *parallel,
		RouteFilter: *route,
		Blind:       *blind,
		DryRun:      *dryRun,
		RepoRoot:    *repoDir,
		Engine:      *engine,
		PGURL:       *pgURL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify play: %v\n", err)
		return 1
	}
	// 3. 输出执行摘要并在非PASS时返回失败退出码
	_ = emitRawJSON(json.MarshalIndent(summary, "", "  "))
	if Verdict(summary.Verdict) != VerdictPass {
		return 1
	}
	return 0
}

// cmdCompletion 核验世界状态标志、路由、蝴蝶探针与结局四项100%完成度门禁
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdCompletion(args []string) int {
	// 1. 解析故事线文件与可选仓库耦合复核标志
	fl := flag.NewFlagSet("completion", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	worldScan := fl.Bool("world-scan", false, "verify world_flag binding against repo scan")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	format := fl.String("format", "json", "output format: json | ai-md")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify completion <story.yaml> [--world-scan] [--db PATH]")
		return 2
	}
	// 2. 加载故事线定义、账本及可选world-scan结果计算完成度
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify completion: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify completion: %v\n", err)
		return 1
	}
	defer ledger.Close()
	var scan *WorldScanResult
	if *worldScan {
		scan, err = scanWorldFlags(*repoDir, s.FocalEntity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify completion world-scan: %v\n", err)
			return 1
		}
	}
	rep, err := computeCompletion(ledger, s, scan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify completion: %v\n", err)
		return 1
	}
	// 3. 按格式输出完成度报告并根据判定结果返回退出码
	if *format == "ai-md" {
		fmt.Printf("## Storyline Completion `%s`: **%s**\n- World Flags: %.0f%%\n- Routes: %.0f%%\n- Butterfly Probes: %.0f%%\n- Endings: %.0f%% (%d/%d)\n- Softlocks: %d · Historical FAILs: %d\n",
			rep.StoryID, rep.Verdict, rep.WorldFlagBindingPct, rep.RouteCoveragePct,
			rep.ButterflyProbeHitPct, rep.EndingUnlockPct, len(rep.UnlockedEndings), len(s.RequiredEndings),
			rep.SoftlockCount, rep.HistoricalFailCount)
	} else {
		_ = emitRawJSON(json.MarshalIndent(rep, "", "  "))
	}
	if Verdict(rep.Verdict) != VerdictPass {
		return 1
	}
	return 0
}

// cmdExplain 渲染六维 Expected-vs-Actual 预期与实际对比审计台账
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdExplain(args []string) int {
	// 1. 解析故事线ID、批次过滤与输出格式参数
	fl := flag.NewFlagSet("explain", flag.ContinueOnError)
	storyID := fl.String("story", "", "story_id")
	graphID := fl.String("graph", "", "alias for --story")
	runID := fl.String("run", "", "optional run_id (defaults to latest run)")
	routeID := fl.String("route", "", "optional route_id filter")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	format := fl.String("format", "json", "output format: json | ai-md")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	id := *storyID
	if id == "" {
		id = *graphID
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify explain --story <id> [--run ID] [--route ID] [--db PATH] [--format json|ai-md]")
		return 2
	}
	// 2. 从验证账本生成六维审计报告并输出
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify explain: %v\n", err)
		return 1
	}
	defer ledger.Close()
	rep, err := explainStory(ledger, id, *runID, *routeID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify explain: %v\n", err)
		return 1
	}
	if *format == "ai-md" {
		fmt.Print(renderExplainMarkdown(rep))
		return 0
	}
	return emitRawJSON(json.MarshalIndent(rep, "", "  "))
}

// cmdGate 执行供统一数据库迁移与门禁校验消费的结构化完成度判定
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdGate(args []string) int {
	// 1. 解析目标故事线ID与账本路径
	fl := flag.NewFlagSet("gate", flag.ContinueOnError)
	storyID := fl.String("story", "", "story_id")
	graphID := fl.String("graph", "", "alias for --story")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	id := *storyID
	if id == "" {
		id = *graphID
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify gate --story <id> [--db PATH]")
		return 2
	}
	// 2. 加载账本内存储的故事线定义计算完成度
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify gate: %v\n", err)
		return 1
	}
	defer ledger.Close()
	s, err := ledger.LoadStoryDefinition(id)
	if err != nil {
		if plan, pErr := ledger.LoadCodePlan(id); pErr == nil && plan != nil {
			rep, cErr := computeCodeGate(ledger, plan.PlanID, "", plan.MaxTier, false)
			if cErr != nil {
				fmt.Fprintf(os.Stderr, "arc verify gate: %v\n", cErr)
				return 1
			}
			_ = emitRawJSON(json.MarshalIndent(rep, "", "  "))
			if Verdict(rep.Verdict) != VerdictPass {
				return 1
			}
			return 0
		}
		fmt.Fprintf(os.Stderr, "arc verify gate: story %q not found: %v\n", id, err)
		return 1
	}
	rep, err := computeCompletion(ledger, s, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify gate: %v\n", err)
		return 1
	}
	// 3. 输出具名 gateResponse 结构体并根据判定返回退出码
	_ = emitRawJSON(json.MarshalIndent(gateResponse{
		Verdict:         rep.Verdict,
		ProofScope:      "storyline_completion_100pct",
		InputsUnchanged: true,
		Context: gateContextInfo{
			storyRefBase: storyRefBase{StoryID: id},
			Graph:        id,
			StorySHA256:  s.SHA256,
		},
		Completion: rep,
	}, "", "  "))
	if Verdict(rep.Verdict) != VerdictPass {
		return 1
	}
	return 0
}

// cmdTaskProof 导出绑定路由收据与六维步骤轨迹哈希的任务完成证明
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdTaskProof(args []string) int {
	// 1. 解析图ID、任务ID、验证义务与账本路径
	fl := flag.NewFlagSet("task-proof", flag.ContinueOnError)
	graphID := fl.String("graph", "", "story_id or plan_id")
	taskID := fl.String("task", "", "story_id/route_id or plan_id/technique_id")
	obligation := fl.String("obligation", "", "optional route_id, technique_id, or all")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil || *graphID == "" || *taskID == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify task-proof --graph <story_id> --task <story_id/route_id> [--obligation ID] [--db PATH]")
		return 2
	}
	// 2. 加载账本并按故事线或代码测试计划分发证明校验
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify task-proof: %v\n", err)
		return 1
	}
	defer ledger.Close()
	s, err := ledger.LoadStoryDefinition(*graphID)
	if err != nil {
		if codeExit, handled := tryEmitCodeTaskProofWithObligation(ledger, *graphID, *taskID, *obligation); handled {
			return codeExit
		}
		fmt.Fprintf(os.Stderr, "arc verify task-proof: %v\n", err)
		return 1
	}
	routeID := strings.TrimSpace(*obligation)
	if routeID == "" {
		routeID = strings.TrimPrefix(*taskID, *graphID+"/")
	}
	return emitStoryTaskProof(ledger, s, *graphID, *taskID, routeID)
}

// cmdSnapshot 导出只读静态HTML与JSON六维流程图工作台
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdSnapshot(args []string) int {
	// 1. 解析故事线来源与输出目录标志
	fl := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	storyID := fl.String("story", "", "story_id stored in ledger")
	file := fl.String("file", "", "or storyline YAML file")
	outDir := fl.String("out", "", "output directory (must be absent or empty)")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
		return 1
	}
	defer ledger.Close()
	// 2. 只读加载YAML文件或账本内故事线定义（绝不隐式写回账本）
	var s *Storyline
	if *file != "" {
		s, err = loadStoryline(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
			return 1
		}
	} else if *storyID != "" {
		s, err = ledger.LoadStoryDefinition(*storyID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(os.Stderr, "usage: arc verify snapshot (--story ID | --file story.yaml) --out DIR [--db PATH]")
		return 2
	}
	// 3. 导出静态HTML与JSON快照并输出具名响应
	if err := exportSnapshot(ledger, s, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
		return 1
	}
	return emitRawJSON(json.MarshalIndent(snapshotExportResponse{
		storyRefBase: storyRefBase{StoryID: s.StoryID},
		Exported:     true,
		Out:          *outDir,
	}, "", "  "))
}

// tryEmitCodeTaskProof 尝试为代码级验证计划及测试技术义务导出原生任务完成证明
// 请求体
// ledger 验证账本实例指针
// graphID 计划ID
// taskID 完整验证任务ID
// 响应体
// exitCode 进程退出状态码
// handled 是否匹配并处理了代码验证计划
func tryEmitCodeTaskProof(ledger *StoryLedger, graphID, taskID string) (exitCode int, handled bool) {
	// 1. 委托含修订绑定与跨提交校验的通用证明导出函数
	return tryEmitCodeTaskProofWithObligation(ledger, graphID, taskID, "")
}
