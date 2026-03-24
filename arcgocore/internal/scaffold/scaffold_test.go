package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestRunContextScaffold(t *testing.T) {
	project := t.TempDir()
	mustJSON(t, filepath.Join(project, ".arc", "context-hub", "index.json"), map[string]any{
		"artifacts": []map[string]any{{
			"name":           "skills.index",
			"path":           "skills.index.json",
			"artifact_type":  "skills-registry",
			"producer_skill": "arc-registry",
			"generated_at":   "2026-03-13T00:00:00Z",
			"expires_at":     "2099-01-01T00:00:00Z",
		}},
	})
	caseDir, err := RunContextScaffold(ContextOptions{ProjectPath: project, TaskName: "resume-auth", Mode: "restore", Objective: "Continue auth migration", Entrypoints: []string{filepath.Join(project, "src", "auth.ts")}, ContextBudget: DefaultContextBudget})
	if err != nil {
		t.Fatalf("RunContextScaffold: %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(caseDir, "restore", "recovery-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest["mode"] != "restore" {
		t.Fatalf("unexpected mode: %v", manifest["mode"])
	}
}

func TestBuildAndExecScaffolds(t *testing.T) {
	repoRoot := repoRootFromModule(t)
	oldValue, hadValue := os.LookupEnv("ARC_REPO_ROOT")
	if err := os.Setenv("ARC_REPO_ROOT", repoRoot); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	defer func() {
		if hadValue {
			_ = os.Setenv("ARC_REPO_ROOT", oldValue)
		} else {
			_ = os.Unsetenv("ARC_REPO_ROOT")
		}
	}()

	project := t.TempDir()
	buildDir, err := RunBuildScaffold(BuildScaffoldOptions{ProjectPath: project, TaskName: "ship-feature"})
	if err != nil {
		t.Fatalf("RunBuildScaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(buildDir, "context", "implementation-brief.md")); err != nil {
		t.Fatalf("missing implementation brief: %v", err)
	}
	if _, err := RunBuildRender(BuildRenderOptions{CaseDir: buildDir, TaskName: "ship-feature", Result: "pass"}); err != nil {
		t.Fatalf("RunBuildRender: %v", err)
	}
	execDir, err := RunExecScaffold(ExecScaffoldOptions{Workdir: project, TaskName: "dispatch-work"})
	if err != nil {
		t.Fatalf("RunExecScaffold: %v", err)
	}
	if _, err := RunExecRender(ExecRenderOptions{CaseDir: execDir, TaskName: "dispatch-work", DispatchRows: []string{"deep|[arc-build]|implement backend interface|done|execution/backend.md"}}); err != nil {
		t.Fatalf("RunExecRender: %v", err)
	}
	if _, err := os.Stat(filepath.Join(execDir, "aggregation", "final-summary.md")); err != nil {
		t.Fatalf("missing final summary: %v", err)
	}
}

func BenchmarkRenderDispatchTable(b *testing.B) {
	rows := []dispatchRow{{Profile: "deep", Capabilities: "[arc-build]", Description: "implement backend interface", Status: "done", Output: "execution/backend.md"}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = renderDispatchTable(rows)
	}
}

func mustJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func repoRootFromModule(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
