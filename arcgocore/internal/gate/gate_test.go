package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestRunGateCheckAndExecute(t *testing.T) {
	project := t.TempDir()
	scoreDir := filepath.Join(project, ".arc", "score", filepath.Base(project))
	mustJSON(t, filepath.Join(scoreDir, "score", "overall-score.json"), map[string]any{
		"score":       95.0,
		"by_severity": map[string]any{"critical": 0, "high": 1},
	})
	mustJSON(t, filepath.Join(scoreDir, "analysis", "smell-report.json"), map[string]any{
		"violations": []map[string]any{{"category": "code_quality"}},
	})

	result, err := RunGateCheck(project, scoreDir, "strict", "")
	if err != nil {
		t.Fatalf("RunGateCheck: %v", err)
	}
	if result.Status != "pass" {
		t.Fatalf("expected pass, got %+v", result)
	}

	executed, jsonPath, summaryPath, err := Execute(Options{ProjectPath: project, ScoreDir: scoreDir, Mode: "strict", OutputDir: filepath.Join(project, "out")})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if executed.Status != "pass" {
		t.Fatalf("expected pass, got %+v", executed)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("missing json output: %v", err)
	}
	if _, err := os.Stat(summaryPath); err != nil {
		t.Fatalf("missing summary output: %v", err)
	}
}

func TestFindScoreDirFromContextHub(t *testing.T) {
	project := t.TempDir()
	scoreDir := filepath.Join(project, ".arc", "score", filepath.Base(project))
	handoffPath := filepath.Join(scoreDir, "handoff", "review-input.json")
	mustJSON(t, handoffPath, map[string]any{"ok": true})
	mustJSON(t, filepath.Join(scoreDir, "score", "overall-score.json"), map[string]any{"score": 91})
	mustJSON(t, filepath.Join(project, ".arc", "context-hub", "index.json"), map[string]any{
		"artifacts": []map[string]any{{
			"producer_skill": "arc:score",
			"path":           filepath.ToSlash(strings.TrimPrefix(handoffPath, project+string(filepath.Separator))),
			"expires_at":     "2099-01-01T00:00:00Z",
		}},
	})
	found := findScoreDirFromContextHub(project)
	if found != scoreDir {
		t.Fatalf("findScoreDirFromContextHub = %q, want %q", found, scoreDir)
	}
}

func BenchmarkGenerateSummary(b *testing.B) {
	result := GateResult{Status: "pass", Mode: "strict", OverallScore: 92.5, Violations: []Violation{{Rule: "high_issues_threshold", Severity: "warn", Actual: 2, Threshold: 5}}, CheckedAt: "2026-03-19T00:00:00Z"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateSummary(result)
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
