package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// cmdInit 初始化Storyline六维验证账本数据库并返回具名响应
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdInit(args []string) int {
	// 1. 解析账本数据库路径标志
	fl := flag.NewFlagSet("init", flag.ContinueOnError)
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		return 2
	}
	// 2. 打开并创建Storyline账本表结构
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify init: %v\n", err)
		return 1
	}
	defer ledger.Close()
	// 3. 输出具名初始化成功响应
	return emitRawJSON(json.MarshalIndent(initResponse{Initialized: true, DB: *dbPath, Engine: "storyline-v3.1-6d"}, "", "  "))
}

// cmdImport 解析并导入Storyline声明文件到验证账本
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdImport(args []string) int {
	// 1. 解析文件路径与账本数据库参数
	fl := flag.NewFlagSet("import", flag.ContinueOnError)
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	file := fl.String("file", "", "storyline YAML/JSON file")
	pos, err := parseInterleavedFlags(fl, args)
	if err == nil && *file == "" && len(pos) > 0 {
		*file = pos[0]
	}
	if err != nil || *file == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify import --file <story.yaml> [--db PATH]")
		return 2
	}
	// 2. 加载故事线定义并持久化写入账本
	s, err := loadStoryline(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify import: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify import: %v\n", err)
		return 1
	}
	defer ledger.Close()
	if err := ledger.SaveStoryDefinition(s); err != nil {
		fmt.Fprintf(os.Stderr, "arc verify import: %v\n", err)
		return 1
	}
	// 3. 输出嵌入storyRefBase的具名导入响应
	return emitRawJSON(json.MarshalIndent(importResponse{
		storyRefBase: storyRefBase{StoryID: s.StoryID},
		Imported:     true,
		SHA256:       s.SHA256,
	}, "", "  "))
}

// cmdScanSurface 扫描全仓HTTP/RPC/数据表/前端路由/异步队列与测试面清单
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdScanSurface(args []string) int {
	// 1. 解析仓库根目录与输出格式标志
	fl := flag.NewFlagSet("scan-surface", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	format := fl.String("format", "json", "output format: json | yaml | ai-md")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	if _, err := parseInterleavedFlags(fl, args); err != nil {
		fmt.Fprintln(os.Stderr, "usage: arc verify scan-surface [--repo DIR] [--format json|yaml|ai-md]")
		return 2
	}
	// 2. 执行全项目业务测试面扫描并按格式渲染输出
	res, err := scanProjectSurface(*repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify scan-surface: %v\n", err)
		return 1
	}
	switch *format {
	case "yaml":
		out, _ := yaml.Marshal(res)
		fmt.Print(string(out))
		return 0
	case "ai-md":
		fmt.Print(renderSurfaceScanMarkdown(res))
		return 0
	default:
		return emitRawJSON(json.MarshalIndent(res, "", "  "))
	}
}

// cmdScanWorld 扫描与核心业务实体耦合的跨表与异步 world_flags 候选集
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdScanWorld(args []string) int {
	// 1. 解析核心实体位置参数与输出格式标志
	fl := flag.NewFlagSet("scan-world", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	format := fl.String("format", "json", "output format: json | yaml | ai-md")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify scan-world <focal_entity> [--repo DIR] [--format json|yaml|ai-md]")
		return 2
	}
	// 2. 扫描关联状态字段并按请求格式输出
	res, err := scanWorldFlags(*repoDir, pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify scan-world: %v\n", err)
		return 1
	}
	switch *format {
	case "yaml":
		out, _ := yaml.Marshal(res)
		fmt.Print(string(out))
		return 0
	case "ai-md":
		fmt.Print(renderWorldScanMarkdown(res))
		return 0
	default:
		return emitRawJSON(json.MarshalIndent(res, "", "  "))
	}
}

// cmdLintStory 校验Storyline图的DAG合法性、反软锁逃生约束与写范围隔离
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdLintStory(args []string) int {
	// 1. 解析故事线路径、可选world-scan开关与错误截断上限
	fl := flag.NewFlagSet("lint-story", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root for --world-scan")
	worldScan := fl.Bool("world-scan", false, "verify 100% world_flags binding against repo scan")
	maxErr := fl.Int("max-errors", 5, "maximum fatal errors to display")
	format := fl.String("format", "json", "output format: json | ai-md")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify lint-story <story.yaml> [--repo DIR] [--world-scan] [--max-errors 5]")
		return 2
	}
	// 2. 加载故事线定义并按需采集仓库耦合扫描结果
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify lint-story: %v\n", err)
		return 1
	}
	var scan *WorldScanResult
	if *worldScan {
		scan, err = scanWorldFlags(*repoDir, s.FocalEntity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify lint-story world-scan: %v\n", err)
			return 1
		}
	}
	// 3. 执行静态规则审查并输出报告
	rep := lintStory(s, scan, *maxErr)
	if *format == "ai-md" {
		fmt.Printf("## lint-story `%s`: %s (%d errors)\n", rep.StoryID, rep.Status, rep.TotalErrors)
		for _, e := range rep.Errors {
			fmt.Printf("- [%s] `%s`: %s\n", e.Code, e.Scope, e.Message)
		}
	} else {
		_ = emitRawJSON(json.MarshalIndent(rep, "", "  "))
	}
	if Verdict(rep.Status) != VerdictPass {
		return 1
	}
	return 0
}

// cmdPlanShards 将分支路由编译为互不干扰的并发盲测分片计划
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdPlanShards(args []string) int {
	// 1. 解析故事线文件与可选落盘输出路径
	fl := flag.NewFlagSet("plan-shards", flag.ContinueOnError)
	outFile := fl.String("out", "", "optional output file path")
	format := fl.String("format", "yaml", "output format: yaml | json")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify plan-shards <story.yaml> [--out FILE] [--format yaml|json]")
		return 2
	}
	// 2. 加载故事线并生成分片规划及序列化内容
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify plan-shards: %v\n", err)
		return 1
	}
	plan, err := planShards(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify plan-shards: %v\n", err)
		return 1
	}
	var raw []byte
	if *format == "json" {
		raw, err = json.MarshalIndent(plan, "", "  ")
		raw = append(raw, '\n')
	} else {
		raw, err = yaml.Marshal(plan)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify plan-shards encode: %v\n", err)
		return 1
	}
	// 3. 若指定 --out 则写入文件并返回具名落盘响应，否则直接打印至标准输出
	if *outFile != "" {
		if err := os.MkdirAll(filepath.Dir(*outFile), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "arc verify plan-shards mkdir: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*outFile, raw, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "arc verify plan-shards write: %v\n", err)
			return 1
		}
		return emitRawJSON(json.MarshalIndent(planShardsFileResponse{Written: *outFile, Shards: len(plan.ConcurrentShards)}, "", "  "))
	}
	fmt.Print(string(raw))
	return 0
}

// cmdCheckpoint 管理检查点快照的拓扑构建、分片克隆与列表读取
// 请求体
// args 命令行参数切片
// 响应体
// code 进程退出状态码
func cmdCheckpoint(args []string) int {
	// 1. 解析二级动作名称与检查点配置参数
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify checkpoint <build|clone|list> --story <story.yaml> [flags]")
		return 2
	}
	sub := args[0]
	fl := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	storyFile := fl.String("story", "", "storyline YAML file")
	saveID := fl.String("save", "", "checkpoint save_id (for clone)")
	ns := fl.String("ns", "ns_default", "runtime namespace (for clone)")
	engine := fl.String("engine", "", "checkpoint engine: dir | sqlite | postgres")
	source := fl.String("source", "", "source sqlite file (for sqlite engine)")
	pgURL := fl.String("pg-url", "", "PostgreSQL admin URL (for postgres engine)")
	repoDir := fl.String("repo", ".", "repository root")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args[1:]); err != nil {
		return 2
	}
	if *storyFile == "" {
		fmt.Fprintln(os.Stderr, "arc verify checkpoint: --story is required")
		return 2
	}
	// 2. 加载故事线定义并打开验证账本
	s, err := loadStoryline(*storyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify checkpoint: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify checkpoint: %v\n", err)
		return 1
	}
	defer ledger.Close()
	absRoot, _ := filepath.Abs(*repoDir)
	// 3. 按 build/clone/list 分支执行并输出具名响应结构体
	switch sub {
	case "build":
		recs, err := buildCheckpoints(ledger, s, absRoot, *engine, *source, *pgURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify checkpoint build: %v\n", err)
			return 1
		}
		return emitRawJSON(json.MarshalIndent(checkpointListResponse{storyRefBase: storyRefBase{StoryID: s.StoryID}, Checkpoints: recs}, "", "  "))
	case "clone":
		if *saveID == "" {
			fmt.Fprintln(os.Stderr, "arc verify checkpoint clone: --save is required")
			return 2
		}
		eng := *engine
		if eng == "" {
			eng = "dir"
		}
		clonePath, err := cloneCheckpoint(absRoot, s.StoryID, *saveID, *ns, eng, *pgURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify checkpoint clone: %v\n", err)
			return 1
		}
		return emitRawJSON(json.MarshalIndent(checkpointCloneResponse{
			storyRefBase: storyRefBase{StoryID: s.StoryID},
			SaveID:       *saveID,
			RuntimeNS:    *ns,
			ClonePath:    clonePath,
		}, "", "  "))
	case "list":
		recs, err := ledger.ListCheckpoints(s.StoryID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify checkpoint list: %v\n", err)
			return 1
		}
		return emitRawJSON(json.MarshalIndent(checkpointListResponse{storyRefBase: storyRefBase{StoryID: s.StoryID}, Checkpoints: recs}, "", "  "))
	default:
		fmt.Fprintf(os.Stderr, "arc verify checkpoint: unknown action %q\n", sub)
		return 2
	}
}
