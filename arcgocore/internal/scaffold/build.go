package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BuildScaffoldOptions struct {
	ProjectPath string
	TaskName    string
	OutputDir   string
}

type BuildRenderOptions struct {
	CaseDir       string
	TaskName      string
	Result        string
	Summary       string
	Verification  string
	AffectedAreas string
	Risks         string
}

var buildSubdirs = []string{"context", "plan", "execution", "reports", "handoff"}

func RunBuildScaffold(opts BuildScaffoldOptions) (string, error) {
	projectPath, err := filepath.Abs(expandUser(opts.ProjectPath))
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	caseDir := opts.OutputDir
	if caseDir == "" {
		caseDir = filepath.Join(projectPath, ".arc", "build", opts.TaskName)
	} else {
		caseDir, err = filepath.Abs(expandUser(caseDir))
		if err != nil {
			return "", fmt.Errorf("resolve output dir: %w", err)
		}
	}
	for _, subdir := range buildSubdirs {
		if err := ensureDir(filepath.Join(caseDir, subdir)); err != nil {
			return "", err
		}
	}
	now := isoNow()
	if err := writeIfMissing(filepath.Join(caseDir, "context", "implementation-brief.md"), strings.Join([]string{
		fmt.Sprintf("# Implementation Brief: %s", opts.TaskName),
		"",
		fmt.Sprintf("- generated_at: %s", now),
		fmt.Sprintf("- project_path: %s", projectPath),
		"- input_source: pending",
		"- notes: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "plan", "implementation-plan.md"), strings.Join([]string{
		"# Implementation Plan",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		"- goal: pending",
		"- scope: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "execution", "execution-log.md"), strings.Join([]string{
		"# Execution Log",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		fmt.Sprintf("- created_at: %s", now),
		"",
	}, "\n")); err != nil {
		return "", err
	}
	return caseDir, nil
}

func RunBuildRender(opts BuildRenderOptions) (string, error) {
	caseDir, err := filepath.Abs(expandUser(opts.CaseDir))
	if err != nil {
		return "", fmt.Errorf("resolve case dir: %w", err)
	}
	repoRoot := os.Getenv("ARC_REPO_ROOT")
	if repoRoot == "" {
		return "", fmt.Errorf("ARC_REPO_ROOT is required")
	}
	templatesDir := filepath.Join(repoRoot, "Arc", "arc-build", "templates")
	execTemplate, err := os.ReadFile(filepath.Join(templatesDir, "execution-report.md.tpl"))
	if err != nil {
		return "", fmt.Errorf("read execution template: %w", err)
	}
	summaryTemplate, err := os.ReadFile(filepath.Join(templatesDir, "change-summary.md.tpl"))
	if err != nil {
		return "", fmt.Errorf("read change summary template: %w", err)
	}

	reportsDir := filepath.Join(caseDir, "reports")
	handoffDir := filepath.Join(caseDir, "handoff")
	if err := ensureDir(reportsDir); err != nil {
		return "", err
	}
	if err := ensureDir(handoffDir); err != nil {
		return "", err
	}

	verification := defaultString(opts.Verification, "待补充验证命令与结果。")
	values := map[string]string{
		"task_name":      opts.TaskName,
		"generated_at":   isoNow(),
		"result":         defaultString(opts.Result, "pass"),
		"summary":        defaultString(opts.Summary, "实现完成，详见变更记录。"),
		"verification":   verification,
		"affected_areas": defaultString(opts.AffectedAreas, "待补充受影响模块。"),
		"risks":          defaultString(opts.Risks, "待补充剩余风险。"),
		"what_changed":   "- 待补充关键改动文件\n- 待补充核心实现点",
		"why":            "- 对齐实现目标与范围",
		"validation":     fmt.Sprintf("- %s", verification),
		"next_steps":     "1. 执行 arc-audit\n2. 执行 arc-e2e（如涉及UI流程）",
	}
	executionReport := renderTemplate(string(execTemplate), values)
	changeSummary := renderTemplate(string(summaryTemplate), values)

	if err := writeFile(filepath.Join(reportsDir, "execution-report.md"), executionReport); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(handoffDir, "change-summary.md"), changeSummary); err != nil {
		return "", err
	}
	return caseDir, nil
}

func renderTemplate(template string, values map[string]string) string {
	output := template
	for key, value := range values {
		output = strings.ReplaceAll(output, "{{"+key+"}}", value)
	}
	return output
}
