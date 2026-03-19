package main

import (
	"flag"
	"fmt"
	"os"

	"skills/arcgocore/internal/scaffold"
)

func main() {
	opts := scaffold.BuildRenderOptions{Result: "pass", Summary: "实现完成，详见变更记录。", Verification: "待补充验证命令与结果。", AffectedAreas: "待补充受影响模块。", Risks: "待补充剩余风险。"}
	fs := flag.NewFlagSet("buildrender", flag.ExitOnError)
	fs.StringVar(&opts.CaseDir, "case-dir", "", "Path to .arc/build/<task>")
	fs.StringVar(&opts.TaskName, "task-name", "", "Task name")
	fs.StringVar(&opts.Result, "result", "pass", "pass or fail")
	fs.StringVar(&opts.Summary, "summary", opts.Summary, "summary text")
	fs.StringVar(&opts.Verification, "verification", opts.Verification, "verification text")
	fs.StringVar(&opts.AffectedAreas, "affected-areas", opts.AffectedAreas, "affected areas")
	fs.StringVar(&opts.Risks, "risks", opts.Risks, "risk summary")
	fs.Parse(os.Args[1:])
	if opts.CaseDir == "" || opts.TaskName == "" {
		fmt.Fprintln(os.Stderr, "--case-dir and --task-name are required")
		os.Exit(2)
	}
	caseDir, err := scaffold.RunBuildRender(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Rendered build artifacts in: %s\n", caseDir)
}
