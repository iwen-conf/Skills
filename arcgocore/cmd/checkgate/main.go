package main

import (
	"flag"
	"fmt"
	"os"

	"skills/arcgocore/internal/gate"
)

func main() {
	opts := gate.Options{}
	exitCode := false
	flag.StringVar(&opts.ProjectPath, "project", "", "项目根目录")
	flag.StringVar(&opts.ScoreDir, "score-dir", "", "arc:score 输出目录")
	flag.StringVar(&opts.Mode, "mode", "", "门禁模式")
	flag.StringVar(&opts.ConfigPath, "config", "", "配置文件路径")
	flag.StringVar(&opts.OutputDir, "output-dir", ".arc/gate-reports", "输出目录")
	flag.BoolVar(&exitCode, "exit-code", false, "以 exit code 返回结果")
	flag.Parse()
	if opts.ProjectPath == "" {
		fmt.Fprintln(os.Stderr, "--project is required")
		os.Exit(2)
	}
	result, jsonPath, _, err := gate.Execute(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	statusEmoji := "✅"
	resultLabel := "通过"
	if result.Status != "pass" {
		statusEmoji = "❌"
		resultLabel = "失败"
	}
	fmt.Printf("\n%s 门禁%s\n", statusEmoji, resultLabel)
	fmt.Printf("  模式: %s\n", result.Mode)
	fmt.Printf("  评分: %.2f\n", result.OverallScore)
	fmt.Printf("  违规: %d (%d 已豁免)\n", len(result.Violations), result.WhitelistApplied)
	fmt.Printf("\n报告: %s\n", jsonPath)
	if exitCode && result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
}
