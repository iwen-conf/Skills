package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var contextSubdirs = []string{"plan", "sources", "retrieval", "findings", "context", "restore", "handoff"}

type ContextOptions struct {
	ProjectPath   string
	TaskName      string
	Mode          string
	Objective     string
	Entrypoints   []string
	DataSources   []string
	ContextBudget int
	OutputDir     string
}

type contextArtifact struct {
	Name               string `json:"name"`
	Path               string `json:"path"`
	ArtifactType       string `json:"artifact_type"`
	ProducerSkill      string `json:"producer_skill"`
	GeneratedAt        string `json:"generated_at"`
	ExpiresAt          string `json:"expires_at"`
	RefreshSkill       string `json:"refresh_skill"`
	RefreshCommandHint string `json:"refresh_command_hint"`
	Status             string `json:"status"`
}

type contextManifest struct {
	SchemaVersion       string            `json:"schema_version"`
	GeneratedAt         string            `json:"generated_at"`
	ProjectPath         string            `json:"project_path"`
	TaskName            string            `json:"task_name"`
	Mode                string            `json:"mode"`
	Objective           string            `json:"objective"`
	ContextBudget       int               `json:"context_budget"`
	Entrypoints         []string          `json:"entrypoints"`
	DataSources         []string          `json:"data_sources"`
	ContextHubArtifacts []contextArtifact `json:"context_hub_artifacts"`
	Assumptions         []string          `json:"assumptions"`
	OpenQuestions       []string          `json:"open_questions"`
	NextActions         []string          `json:"next_actions"`
}

func RunContextScaffold(opts ContextOptions) (string, error) {
	projectPath, err := filepath.Abs(expandUser(opts.ProjectPath))
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	if opts.ContextBudget == 0 {
		opts.ContextBudget = DefaultContextBudget
	}
	caseDir := opts.OutputDir
	if caseDir == "" {
		caseDir = filepath.Join(projectPath, ".arc", "context", opts.TaskName)
	} else {
		caseDir, err = filepath.Abs(expandUser(caseDir))
		if err != nil {
			return "", fmt.Errorf("resolve output dir: %w", err)
		}
	}
	for _, subdir := range contextSubdirs {
		if err := ensureDir(filepath.Join(caseDir, subdir)); err != nil {
			return "", err
		}
	}

	now := isoNow()
	entrypoints := make([]string, 0, len(opts.Entrypoints))
	for _, item := range opts.Entrypoints {
		entrypoints = append(entrypoints, toRelative(projectPath, item))
	}
	dataSources := make([]string, 0, len(opts.DataSources))
	for _, item := range opts.DataSources {
		dataSources = append(dataSources, toRelative(projectPath, item))
	}
	artifacts := loadContextHubArtifacts(projectPath)
	manifest := contextManifest{
		SchemaVersion:       "1.0.0",
		GeneratedAt:         now,
		ProjectPath:         projectPath,
		TaskName:            opts.TaskName,
		Mode:                opts.Mode,
		Objective:           opts.Objective,
		ContextBudget:       opts.ContextBudget,
		Entrypoints:         entrypoints,
		DataSources:         dataSources,
		ContextHubArtifacts: artifacts,
		Assumptions:         []string{"pending"},
		OpenQuestions:       []string{"pending"},
		NextActions:         []string{"pending"},
	}

	if err := writeIfMissing(filepath.Join(caseDir, "context", "context-brief.md"), renderContextBrief(now, projectPath, opts.TaskName, opts.Mode, opts.Objective)); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "plan", "context-plan.md"), renderContextPlan(opts.Mode, opts.Objective, dataSources)); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "context", "working-set.md"), renderWorkingSet(entrypoints, artifacts)); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "retrieval", "search-queries.md"), renderSearchQueries()); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "findings", "compact-findings.md"), renderCompactFindings()); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "restore", "restore-checklist.md"), renderRestoreChecklist(opts.Mode)); err != nil {
		return "", err
	}
	if err := writeIfMissing(filepath.Join(caseDir, "handoff", "handoff-notes.md"), renderHandoffNotes(now, opts.TaskName)); err != nil {
		return "", err
	}
	if err := writeJSONIfMissing(filepath.Join(caseDir, "restore", "recovery-manifest.json"), manifest); err != nil {
		return "", err
	}

	return caseDir, nil
}

func loadContextHubArtifacts(projectPath string) []contextArtifact {
	indexPath := filepath.Join(projectPath, ".arc", "context-hub", "index.json")
	payload, err := os.ReadFile(indexPath)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}
	items, ok := raw["artifacts"].([]any)
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	artifacts := make([]contextArtifact, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]any)
		if !ok {
			continue
		}
		artifacts = append(artifacts, contextArtifact{
			Name:               asString(value["name"]),
			Path:               asString(value["path"]),
			ArtifactType:       asString(value["artifact_type"]),
			ProducerSkill:      asString(value["producer_skill"]),
			GeneratedAt:        asString(value["generated_at"]),
			ExpiresAt:          asString(value["expires_at"]),
			RefreshSkill:       asString(value["refresh_skill"]),
			RefreshCommandHint: asString(value["refresh_command_hint"]),
			Status:             artifactStatus(asString(value["expires_at"]), now),
		})
	}
	return artifacts
}

func artifactStatus(expiresAt string, now time.Time) string {
	if expiresAt == "" {
		return "unknown"
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "unknown"
	}
	if parsed.Before(now) {
		return "stale"
	}
	return "fresh"
}

func renderContextBrief(now string, projectPath string, taskName string, mode string, objective string) string {
	return strings.Join([]string{
		fmt.Sprintf("# Context Brief: %s", taskName),
		"",
		fmt.Sprintf("- generated_at: %s", now),
		fmt.Sprintf("- project_path: %s", projectPath),
		fmt.Sprintf("- mode: %s", mode),
		fmt.Sprintf("- objective: %s", defaultString(objective, "pending")),
		"- current_status: pending",
		"- blocker_summary: pending",
		"- next_decision: pending",
		"",
	}, "\n")
}

func renderContextPlan(mode string, objective string, dataSources []string) string {
	hint := map[string]string{
		"prime":    "Prime a bounded packet before deep work expands.",
		"analyze":  "Keep large output in files or retrieval artifacts before summarizing.",
		"snapshot": "Compress the current session before interruption or handoff.",
		"restore":  "Reopen only the minimum viable working set and verify freshness.",
	}[mode]
	handlingPath := "bounded packet"
	if mode == "analyze" {
		handlingPath = "file-first / retrieval-first"
	}
	lines := []string{
		"# Context Plan",
		"",
		fmt.Sprintf("- mode: %s", mode),
		fmt.Sprintf("- objective: %s", defaultString(objective, "pending")),
		fmt.Sprintf("- handling_path: %s", handlingPath),
		fmt.Sprintf("- mode_hint: %s", hint),
		"",
		"## Data Sources",
	}
	if len(dataSources) == 0 {
		lines = append(lines, "- pending")
	} else {
		for _, item := range dataSources {
			lines = append(lines, fmt.Sprintf("- `%s`", item))
		}
	}
	lines = append(lines, "", "## Fallback", "- pending", "")
	return strings.Join(lines, "\n")
}

func renderWorkingSet(entrypoints []string, artifacts []contextArtifact) string {
	lines := []string{"# Working Set", "", "## Reopen First"}
	if len(entrypoints) == 0 {
		lines = append(lines, "- pending")
	} else {
		for _, item := range entrypoints {
			lines = append(lines, fmt.Sprintf("- `%s`", item))
		}
	}
	lines = append(lines, "", "## Reusable Artifacts")
	if len(artifacts) == 0 {
		lines = append(lines, "- none detected in `.arc/context-hub/index.json`")
	} else {
		for _, item := range artifacts {
			name := defaultString(item.Name, "unnamed-artifact")
			lines = append(lines, fmt.Sprintf("- `%s` -> `%s` (%s)", name, item.Path, item.Status))
		}
	}
	lines = append(lines, "", "## Open Questions", "- pending", "", "## Next Action", "- pending", "")
	return strings.Join(lines, "\n")
}

func renderRestoreChecklist(mode string) string {
	message := map[string]string{
		"prime":    "Capture the first bounded packet before deep implementation starts.",
		"analyze":  "Persist raw artifacts first and verify the narrowed query plan before summarizing.",
		"snapshot": "Confirm the packet is sufficient for a clean handoff before the current session ends.",
		"restore":  "Reopen only the minimum files and artifacts required for the next step.",
	}[mode]
	return strings.Join([]string{
		"# Restore Checklist",
		"",
		fmt.Sprintf("1. %s", message),
		"2. Read `restore/recovery-manifest.json` and verify freshness markers.",
		"3. Reopen `context/working-set.md` and the listed entrypoints.",
		"4. Separate observed facts from assumptions before resuming changes.",
		"5. Confirm the next action and update the packet if reality has drifted.",
		"",
	}, "\n")
}

func renderSearchQueries() string {
	return strings.Join([]string{
		"# Search Queries",
		"",
		"## Primary Queries",
		"- pending",
		"",
		"## Follow-up Queries",
		"- pending",
		"",
	}, "\n")
}

func renderCompactFindings() string {
	return strings.Join([]string{
		"# Compact Findings",
		"",
		"## Key Signals",
		"- pending",
		"",
		"## Anchors",
		"- pending",
		"",
		"## Next Action",
		"- pending",
		"",
	}, "\n")
}

func renderHandoffNotes(now string, taskName string) string {
	return strings.Join([]string{
		fmt.Sprintf("# Handoff Notes: %s", taskName),
		"",
		fmt.Sprintf("- updated_at: %s", now),
		"- observed_facts: pending",
		"- assumptions: pending",
		"- blockers: pending",
		"- next_action: pending",
		"",
	}, "\n")
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
