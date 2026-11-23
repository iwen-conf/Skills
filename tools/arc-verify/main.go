package main

import (
	"fmt"
	"os"
)

const version = "3.1.0"

const usage = `arc verify %s — 6-Dimension Storyline & Full-Business E2E Verification Engine

Usage:
  arc verify init         [--db PATH]
  arc verify scan-surface [--repo DIR] [--format json|yaml|ai-md]
  arc verify scan-world   <focal_entity> [--repo DIR] [--format json|yaml|ai-md]
  arc verify lint-story   <story.yaml> [--repo DIR] [--world-scan] [--max-errors 5] [--format json|ai-md]
  arc verify plan-shards  <story.yaml> [--out FILE] [--format yaml|json]
  arc verify checkpoint   <build|clone|list> --story <story.yaml> [--save ID] [--ns NS] [--engine dir|sqlite|postgres] [--pg-url URL] [--db PATH]
  arc verify play         <story.yaml> [--parallel] [--route ID] [--blind] [--dry-run] [--repo DIR] [--db PATH]
  arc verify completion   <story.yaml> [--repo DIR] [--world-scan] [--db PATH] [--format json|ai-md]
  arc verify explain      --story <id>|--graph <id> [--run ID] [--route ID] [--db PATH] [--format json|ai-md]
  arc verify gate         --story <id>|--graph <id> [--db PATH]
  arc verify task-proof   --graph <story_id> --task <story_id/route_id> [--db PATH]
  arc verify snapshot     --story <id>|--file <story.yaml> --out <dir> [--db PATH]
  arc verify version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	os.Exit(dispatch(cmd, args))
}

// dispatch 将命令行一级子命令分发至对应的验证控制器函数
// 请求体
// cmd 子命令名称
// args 子命令参数切片
// 响应体
// code 进程退出状态码
func dispatch(cmd string, args []string) int {
	// 1. 按闭合子命令集合分发至对应命令入口
	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("arc-verify %s (storyline-6d-engine)\n", version)
		return 0
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
		return 0
	case "init":
		return cmdInit(args)
	case "scan-surface":
		return cmdScanSurface(args)
	case "scan-world":
		return cmdScanWorld(args)
	case "lint-story":
		return cmdLintStory(args)
	case "plan-shards":
		return cmdPlanShards(args)
	case "checkpoint":
		return cmdCheckpoint(args)
	case "play":
		return cmdPlay(args)
	case "completion":
		return cmdCompletion(args)
	case "explain":
		return cmdExplain(args)
	case "gate":
		return cmdGate(args)
	case "task-proof":
		return cmdTaskProof(args)
	case "snapshot":
		return cmdSnapshot(args)
	case "import":
		return cmdImport(args)
	default:
		fmt.Fprintf(os.Stderr, "arc verify: unknown command %q\n\n", cmd)
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
}
