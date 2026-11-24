package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// cmdCode 将 arc verify code 二级命令分发至目录、计划、执行、解释、门禁与工作台快照入口
// 请求体
// args 子命令参数切片
// 响应体
// code 进程退出状态码
func cmdCode(args []string) int {
	// 1. 检查二级子命令并分发到对应处理函数
	if len(args) == 0 {
		printCodeUsage()
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "help", "--help", "-h":
		printCodeUsage()
		return 0
	case "catalog":
		return cmdCodeCatalog(rest)
	case "plan":
		return cmdCodePlan(rest)
	case "run":
		return cmdCodeRun(rest)
	case "explain":
		return cmdCodeExplain(rest)
	case "gate":
		return cmdCodeGate(rest)
	case "snapshot":
		return cmdCodeSnapshot(rest)
	default:
		fmt.Fprintf(os.Stderr, "arc verify code: unknown action %q\n\n", sub)
		printCodeUsage()
		return 2
	}
}

// cmdCodeCatalog 查询并筛选26项代码级测试技术目录与8项覆盖率细分矩阵
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdCodeCatalog(args []string) int {
	// 1. 解析过滤参数与输出格式标志
	fl := flag.NewFlagSet("code catalog", flag.ContinueOnError)
	lang := fl.String("lang", "", "filter by language: rust | go | typescript | python | c_cpp")
	tierStr := fl.String("tier", "", "filter max priority tier: P0 | P1 | P2 | P3")
	tech := fl.String("technique", "", "filter by single technique ID")
	withCov := fl.Bool("with-coverage", true, "include 8 code coverage metric definitions")
	format := fl.String("format", "json", "output format: json | yaml | ai-md")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	maxTier := TierUnspecified
	if strings.TrimSpace(*tierStr) != "" {
		parsed, ok := parsePriorityTier(*tierStr)
		if !ok {
			fmt.Fprintf(os.Stderr, "arc verify code catalog: invalid --tier %q (want P0|P1|P2|P3)\n", *tierStr)
			return 2
		}
		maxTier = parsed
	}
	// 2. 按条件筛选目录并按指定格式渲染输出
	items := filterCodeTechniques(*lang, maxTier, *tech)
	covMetrics := allCoverageMetrics()
	switch *format {
	case "ai-md":
		fmt.Print(renderCatalogMarkdown(items, covMetrics, *withCov))
		return 0
	case "yaml":
		resp := codeCatalogResponse{
			Version:              1,
			TotalTechniques:      len(items),
			TotalCoverageMetrics: len(covMetrics),
			Techniques:           items,
		}
		if *withCov {
			resp.CoverageMetrics = covMetrics
		}
		raw, err := yaml.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify code catalog yaml: %v\n", err)
			return 1
		}
		fmt.Print(string(raw))
		return 0
	default:
		resp := codeCatalogResponse{
			Version:              1,
			TotalTechniques:      len(items),
			TotalCoverageMetrics: len(covMetrics),
			Techniques:           items,
		}
		if *withCov {
			resp.CoverageMetrics = covMetrics
		}
		return emitRawJSON(json.MarshalIndent(resp, "", "  "))
	}
}

// cmdCodePlan 分析仓库结构风险并自动编译26项代码级测试装配计划
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdCodePlan(args []string) int {
	// 1. 解析仓库路径、层级阈值、工具探测开关与持久化标志
	fl := flag.NewFlagSet("code plan", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	lang := fl.String("lang", "", "override primary language")
	tierStr := fl.String("max-tier", "P1", "target priority tier: P0 | P1 | P2 | P3")
	planID := fl.String("plan-id", "", "custom plan_id")
	strict := fl.Bool("strict", false, "promote applicable harness-based techniques in max-tier to REQUIRED")
	allApp := fl.Bool("all-applicable", false, "mark all applicable techniques REQUIRED regardless of tier")
	noProbe := fl.Bool("no-probe", false, "skip local tool probing")
	saveDB := fl.Bool("save", false, "persist generated plan into verification ledger")
	outFile := fl.String("out", "", "optional YAML or JSON file path to export plan")
	format := fl.String("format", "json", "output format: json | yaml | ai-md")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil {
		return 2
	}
	if len(pos) > 0 && *repoDir == "." {
		*repoDir = pos[0]
	}
	maxTier, ok := parsePriorityTier(*tierStr)
	if !ok {
		fmt.Fprintf(os.Stderr, "arc verify code plan: invalid --max-tier %q\n", *tierStr)
		return 2
	}
	// 2. 剖析仓库并编译代码级测试计划
	plan, err := buildCodeVerifyPlan(*repoDir, CodePlanOptions{
		PlanID:               *planID,
		Language:             *lang,
		MaxTier:              maxTier,
		Strict:               *strict,
		IncludeAllApplicable: *allApp,
		ProbeTools:           !*noProbe,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code plan: %v\n", err)
		return 1
	}
	// 3. 按需持久化至账本或导出计划文件
	if *saveDB {
		ledger, lErr := openStoryLedger(*dbPath)
		if lErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code plan db: %v\n", lErr)
			return 1
		}
		if sErr := ledger.SaveCodePlan(plan); sErr != nil {
			_ = ledger.Close()
			fmt.Fprintf(os.Stderr, "arc verify code plan save: %v\n", sErr)
			return 1
		}
		_ = ledger.Close()
	}
	if *outFile != "" {
		rawOut, encErr := marshalCodePlanByExt(plan, *outFile)
		if encErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code plan encode: %v\n", encErr)
			return 1
		}
		if mkErr := os.MkdirAll(filepath.Dir(*outFile), 0o755); mkErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code plan mkdir: %v\n", mkErr)
			return 1
		}
		if wErr := os.WriteFile(*outFile, rawOut, 0o644); wErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code plan write: %v\n", wErr)
			return 1
		}
	}
	// 4. 按指定格式输出计划内容
	switch *format {
	case "ai-md":
		fmt.Print(renderCodePlanMarkdown(plan))
		return 0
	case "yaml":
		raw, mErr := yaml.Marshal(plan)
		if mErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code plan yaml: %v\n", mErr)
			return 1
		}
		fmt.Print(string(raw))
		return 0
	default:
		return emitRawJSON(json.MarshalIndent(plan, "", "  "))
	}
}

// cmdCodeRun 执行代码级测试计划中的必选或指定技术并记录不可变账本收据与产物
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码（全量执行按门禁、子集执行按批次结果判定PASS返回0）
func cmdCodeRun(args []string) int {
	// 1. 解析计划来源、显式仓库标志、技术筛选与执行控制标志
	fl := flag.NewFlagSet("code run", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	planFile := fl.String("plan", "", "optional saved YAML/JSON plan file or stored plan_id")
	planID := fl.String("plan-id", "", "optional plan_id")
	lang := fl.String("lang", "", "override primary language")
	tierStr := fl.String("max-tier", "P1", "target priority tier: P0 | P1 | P2 | P3")
	techCSV := fl.String("technique", "", "comma-separated technique IDs to run")
	incRec := fl.Bool("include-recommended", false, "also execute RECOMMENDED items")
	strict := fl.Bool("strict", false, "strict harness planning")
	dryRun := fl.Bool("dry-run", false, "evaluate prerequisites as BLOCKED without executing commands")
	allowUnchecked := fl.Bool("allow-unchecked-harness", false, "run even when static harness check is negative")
	timeoutSec := fl.Int("timeout", 300, "per-technique command timeout seconds")
	format := fl.String("format", "json", "output format: json | ai-md")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil {
		return 2
	}
	repoExplicit := false
	fl.Visit(func(f *flag.Flag) {
		if f.Name == "repo" {
			repoExplicit = true
		}
	})
	if len(pos) > 0 && !repoExplicit && *planFile == "" {
		*repoDir = pos[0]
		repoExplicit = true
	}
	maxTier, ok := parsePriorityTier(*tierStr)
	if !ok {
		fmt.Fprintf(os.Stderr, "arc verify code run: invalid --max-tier %q\n", *tierStr)
		return 2
	}

	// 2. 先打开账本以支持按 plan_id 回退查询已入库计划，同时保留原 plan.RepoRoot
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code run db: %v\n", err)
		return 1
	}
	defer ledger.Close()
	plan, runRepoRoot, err := resolveCodePlanForRun(ledger, *planFile, *planID, *repoDir, repoExplicit, *lang, maxTier, *strict)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code run: %v\n", err)
		return 1
	}

	// 3. 执行筛选后的测试技术集合并汇总账本结果与门禁状态
	var techs []string
	for _, part := range strings.Split(*techCSV, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			techs = append(techs, trimmed)
		}
	}
	summary, err := runCodeVerifyPlan(ledger, plan, CodeRunOptions{
		Techniques:            techs,
		IncludeRecommended:    *incRec,
		DryRun:                *dryRun,
		AllowUncheckedHarness: *allowUnchecked,
		TimeoutSec:            *timeoutSec,
		RepoRoot:              runRepoRoot,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code run: %v\n", err)
		return 1
	}
	if *format == "ai-md" {
		rep := &CodeExplainReport{
			PlanID:              plan.PlanID,
			PlanSHA256:          plan.PlanSHA256,
			RepoRoot:            plan.RepoRoot,
			PrimaryLanguage:     plan.PrimaryLanguage,
			LatestRun:           &summary.Run,
			Plan:                plan,
			Results:             summary.Results,
			HistoricalFailCount: summary.HistoricalFailCount,
			Gate:                summary.Gate,
		}
		fmt.Print(renderCodeExplainMarkdown(rep))
	} else {
		_ = emitRawJSON(json.MarshalIndent(summary, "", "  "))
	}
	if len(techs) == 0 {
		if summary.Gate == nil || Verdict(summary.Gate.Verdict) != VerdictPass {
			return 1
		}
		return 0
	}
	if Verdict(summary.Run.Verdict) != VerdictPass {
		return 1
	}
	return 0
}

// cmdCodeExplain 渲染代码级测试执行台账、八项覆盖率明细与三大易忽略测试状态
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdCodeExplain(args []string) int {
	// 1. 解析计划ID、批次ID与格式参数
	fl := flag.NewFlagSet("code explain", flag.ContinueOnError)
	planID := fl.String("plan", "", "plan_id (defaults to latest code plan)")
	graphID := fl.String("graph", "", "alias for --plan")
	storyID := fl.String("story", "", "alias for --plan")
	runID := fl.String("run", "", "optional run_id (defaults to latest run)")
	format := fl.String("format", "json", "output format: json | ai-md")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	id := nonEmpty(*planID, nonEmpty(*graphID, *storyID))
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code explain: %v\n", err)
		return 1
	}
	defer ledger.Close()
	// 2. 生成代码测试台账并输出
	rep, err := explainCodeVerify(ledger, id, *runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code explain: %v\n", err)
		return 1
	}
	if *format == "ai-md" {
		fmt.Print(renderCodeExplainMarkdown(rep))
		return 0
	}
	return emitRawJSON(json.MarshalIndent(rep, "", "  "))
}

// cmdCodeGate 核验代码级测试计划按指定优先级与三大易忽略测试的完成门禁
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码（仅PASS返回0）
func cmdCodeGate(args []string) int {
	// 1. 解析计划ID、门禁层级与三大易忽略测试强制开关
	fl := flag.NewFlagSet("code gate", flag.ContinueOnError)
	planID := fl.String("plan", "", "plan_id (defaults to latest code plan)")
	graphID := fl.String("graph", "", "alias for --plan")
	storyID := fl.String("story", "", "alias for --plan")
	runID := fl.String("run", "", "optional run_id")
	tierStr := fl.String("min-tier", "", "minimum required tier: P0 | P1 | P2 | P3")
	reqOverlooked := fl.Bool("require-overlooked", false, "require mutation, property, and fuzz checks when applicable")
	format := fl.String("format", "json", "output format: json | ai-md")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	minTier := TierUnspecified
	if strings.TrimSpace(*tierStr) != "" {
		parsed, ok := parsePriorityTier(*tierStr)
		if !ok {
			fmt.Fprintf(os.Stderr, "arc verify code gate: invalid --min-tier %q\n", *tierStr)
			return 2
		}
		minTier = parsed
	}
	id := nonEmpty(*planID, nonEmpty(*graphID, *storyID))
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code gate: %v\n", err)
		return 1
	}
	defer ledger.Close()
	if id == "" {
		latest, lErr := ledger.LatestCodePlan()
		if lErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code gate: no code plan found in ledger: %v\n", lErr)
			return 1
		}
		id = latest.PlanID
	}
	// 2. 计算门禁报告并按结果返回退出码
	rep, err := computeCodeGate(ledger, id, *runID, minTier, *reqOverlooked)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code gate: %v\n", err)
		return 1
	}
	if *format == "ai-md" {
		fmt.Print(renderCodeGateMarkdown(rep))
	} else {
		_ = emitRawJSON(json.MarshalIndent(rep, "", "  "))
	}
	if Verdict(rep.Verdict) != VerdictPass {
		return 1
	}
	return 0
}

// cmdCodeSnapshot 导出自包含的26项代码级测试工作台HTML与JSON静态快照
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdCodeSnapshot(args []string) int {
	// 1. 解析计划ID、批次ID与强制输出目录
	fl := flag.NewFlagSet("code snapshot", flag.ContinueOnError)
	planID := fl.String("plan", "", "plan_id (defaults to latest code plan)")
	graphID := fl.String("graph", "", "alias for --plan")
	storyID := fl.String("story", "", "alias for --plan")
	runID := fl.String("run", "", "optional run_id")
	outDir := fl.String("out", "", "output directory (must be absent or empty)")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	id := nonEmpty(*planID, nonEmpty(*graphID, *storyID))
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code snapshot: %v\n", err)
		return 1
	}
	defer ledger.Close()
	if id == "" {
		latest, lErr := ledger.LatestCodePlan()
		if lErr != nil {
			fmt.Fprintf(os.Stderr, "arc verify code snapshot: %v\n", lErr)
			return 1
		}
		id = latest.PlanID
	}
	// 2. 导出静态HTML与JSON快照并输出具名响应
	if err := exportCodeSnapshot(ledger, id, *runID, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "arc verify code snapshot: %v\n", err)
		return 1
	}
	return emitRawJSON(json.MarshalIndent(codeSnapshotExportResponse{
		PlanID:   id,
		RunID:    *runID,
		Exported: true,
		Out:      *outDir,
	}, "", "  "))
}
