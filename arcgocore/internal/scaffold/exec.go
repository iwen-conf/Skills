package scaffold

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type ExecScaffoldOptions struct {
	Workdir   string
	TaskName  string
	OutputDir string
}

type ExecRenderOptions struct {
	CaseDir      string
	TaskName     string
	Route        string
	RouteReason  string
	Risk         string
	DispatchRows []string
	NextSteps    []string
}

var execSubdirs = []string{"context", "routing", "preview", "dispatch", "aggregation", "snapshots", "rollback"}

type dispatchRow struct {
	Profile      string
	Capabilities string
	Description  string
	Status       string
	Output       string
}

func RunExecScaffold(opts ExecScaffoldOptions) (string, error) {
	workdir, err := filepath.Abs(expandUser(opts.Workdir))
	if err != nil {
		return "", fmt.Errorf("resolve workdir: %w", err)
	}
	caseDir := opts.OutputDir
	if caseDir == "" {
		caseDir = filepath.Join(workdir, ".arc", "exec", opts.TaskName)
	} else {
		caseDir, err = filepath.Abs(expandUser(caseDir))
		if err != nil {
			return "", fmt.Errorf("resolve output dir: %w", err)
		}
	}
	for _, subdir := range execSubdirs {
		if err := ensureDir(filepath.Join(caseDir, subdir)); err != nil {
			return "", err
		}
	}
	now := isoNow()
	manifest := map[string]any{
		"task_name":   opts.TaskName,
		"created_at":  now,
		"workdir":     workdir,
		"owner_skill": "arc-exec",
		"status":      "initialized",
	}
	if err := writeJSONIfMissing(filepath.Join(caseDir, "manifest.json"), manifest); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "context", "task-brief.md"), strings.Join([]string{
		fmt.Sprintf("# Task Brief: %s", opts.TaskName),
		"",
		fmt.Sprintf("- generated_at: %s", now),
		fmt.Sprintf("- workdir: %s", workdir),
		"- task_description: pending",
		"- constraints: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "routing", "dispatch-log.md"), strings.Join([]string{
		"# Dispatch Log",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		fmt.Sprintf("- created_at: %s", now),
		"- selected_path: pending",
		"- reason: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "preview", "execution-preview.md"), strings.Join([]string{
		"# Execution Preview",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		"- planned_actions:",
		"  - pending",
		"- estimated_risk: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "dispatch", "task-board.md"), strings.Join([]string{
		"# Task Board",
		"",
		"| Wave | capability_profile | capabilities | status | output |",
		"|------|--------------------|--------------|--------|--------|",
		"| 1 | pending | pending | pending | pending |",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "aggregation", "final-summary.md"), strings.Join([]string{
		"# Final Summary",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		"- result: pending",
		"- conflicts: pending",
		"- next_steps: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "rollback", "restore-notes.md"), strings.Join([]string{
		"# Restore Notes",
		"",
		"- snapshot_path: pending",
		"- rollback_command: pending",
		"- trigger_condition: pending",
		"",
	}, "\n")); err != nil {
		return "", err
	}
	return caseDir, nil
}

func RunExecRender(opts ExecRenderOptions) (string, error) {
	caseDir, err := filepath.Abs(expandUser(opts.CaseDir))
	if err != nil {
		return "", fmt.Errorf("resolve case dir: %w", err)
	}
	if err := ensureDir(filepath.Join(caseDir, "routing")); err != nil {
		return "", err
	}
	if err := ensureDir(filepath.Join(caseDir, "dispatch")); err != nil {
		return "", err
	}
	if err := ensureDir(filepath.Join(caseDir, "aggregation")); err != nil {
		return "", err
	}
	rows := make([]dispatchRow, 0, len(opts.DispatchRows))
	for _, row := range opts.DispatchRows {
		rows = append(rows, parseDispatchRow(row))
	}
	if len(rows) == 0 {
		rows = append(rows, parseDispatchRow("pending|[]|pending|pending|pending"))
	}
	nextSteps := opts.NextSteps
	if len(nextSteps) == 0 {
		nextSteps = []string{"补充最终交付说明", "执行回归验证"}
	}
	table := renderDispatchTable(rows)
	now := isoNow()

	routingLog := strings.Join([]string{
		"# Dispatch Log",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		fmt.Sprintf("- generated_at: %s", now),
		fmt.Sprintf("- selected_route: %s", defaultString(opts.Route, "direct-dispatch")),
		fmt.Sprintf("- route_reason: %s", defaultString(opts.RouteReason, "待补充")),
		"",
		"## Dispatch Waves",
		table,
		"",
	}, "\n")
	if err := writeFile(filepath.Join(caseDir, "routing", "dispatch-log.md"), routingLog); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(caseDir, "dispatch", "task-board.md"), strings.Join([]string{"# Task Board", "", table, ""}, "\n")); err != nil {
		return "", err
	}

	summaryLines := []string{
		"# Final Summary",
		"",
		fmt.Sprintf("- task_name: %s", opts.TaskName),
		fmt.Sprintf("- generated_at: %s", now),
		fmt.Sprintf("- route: %s", defaultString(opts.Route, "direct-dispatch")),
		fmt.Sprintf("- risk: %s", defaultString(opts.Risk, "medium")),
		fmt.Sprintf("- dispatch_count: %d", len(rows)),
		"",
		"## Next Steps",
	}
	for index, item := range nextSteps {
		summaryLines = append(summaryLines, fmt.Sprintf("%d. %s", index+1, item))
	}
	summaryLines = append(summaryLines, "")
	if err := writeFile(filepath.Join(caseDir, "aggregation", "final-summary.md"), strings.Join(summaryLines, "\n")); err != nil {
		return "", err
	}

	return caseDir, nil
}

func parseDispatchRow(raw string) dispatchRow {
	parts := strings.Split(raw, "|")
	for len(parts) < 5 {
		parts = append(parts, "")
	}
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return dispatchRow{
		Profile:      defaultString(parts[0], "pending"),
		Capabilities: defaultString(parts[1], "[]"),
		Description:  defaultString(parts[2], "pending"),
		Status:       defaultString(parts[3], "pending"),
		Output:       defaultString(parts[4], "pending"),
	}
}

func renderDispatchTable(rows []dispatchRow) string {
	lines := []string{
		"| Wave | capability_profile | capabilities | description | status | output |",
		"|------|--------------------|--------------|-------------|--------|--------|",
	}
	for index, row := range rows {
		lines = append(lines, fmt.Sprintf("| %d | %s | %s | %s | %s | %s |", index+1, escapeTable(row.Profile), escapeTable(row.Capabilities), escapeTable(row.Description), escapeTable(row.Status), escapeTable(row.Output)))
	}
	return strings.Join(lines, "\n")
}

func escapeTable(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	unquoted := strings.Trim(string(encoded), "\"")
	return strings.ReplaceAll(unquoted, "|", "\\|")
}
