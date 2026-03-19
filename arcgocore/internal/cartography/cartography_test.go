package cartography

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPatternMatcher(t *testing.T) {
	matcher, err := NewPatternMatcher([]string{"node_modules/", "dist/", "*.log", "src/**/*.ts"})
	if err != nil {
		t.Fatalf("NewPatternMatcher: %v", err)
	}
	cases := map[string]bool{
		"node_modules/foo.js":        true,
		"vendor/node_modules/bar.js": true,
		"dist/main.js":               true,
		"src/dist/output.js":         true,
		"error.log":                  true,
		"logs/access.log":            true,
		"src/index.ts":               true,
		"src/utils/helper.ts":        true,
		"README.md":                  false,
		"tests/test.py":              false,
	}
	for input, expected := range cases {
		if got := matcher.Matches(input); got != expected {
			t.Fatalf("Matches(%q) = %v, want %v", input, got, expected)
		}
	}
}

func TestSelectFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src"))
	mustMkdir(t, filepath.Join(root, "node_modules"))
	mustWrite(t, filepath.Join(root, "src", "index.ts"), "code")
	mustWrite(t, filepath.Join(root, "src", "index.test.ts"), "test")
	mustWrite(t, filepath.Join(root, "node_modules", "foo.js"), "dep")
	mustWrite(t, filepath.Join(root, "package.json"), "{}")

	selected, err := SelectFiles(root, []string{"src/**/*.ts", "package.json"}, []string{"**/*.test.ts", "node_modules/"}, nil, nil)
	if err != nil {
		t.Fatalf("SelectFiles: %v", err)
	}
	expected := []string{"package.json", "src/index.ts"}
	if len(selected) != len(expected) {
		t.Fatalf("SelectFiles len = %d, want %d (%v)", len(selected), len(expected), selected)
	}
	for index, item := range expected {
		if selected[index] != item {
			t.Fatalf("SelectFiles[%d] = %q, want %q", index, selected[index], item)
		}
	}
}

func TestComputeHashes(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	mustWrite(t, filePath, "test content")
	if got := ComputeFileHash(filePath); got != "9473fdd0d880a43c21b7778d34872157" {
		t.Fatalf("ComputeFileHash = %q", got)
	}
	folderHash := ComputeFolderHash("src", map[string]string{"src/a.ts": "hash-a", "src/b.ts": "hash-b", "tests/test.ts": "hash-test"})
	if folderHash == "" {
		t.Fatal("ComputeFolderHash returned empty hash")
	}
}

func TestInitUpdateExport(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src"))
	mustWrite(t, filepath.Join(root, "src", "main.go"), "package main\nfunc main() {}\n")

	state, folders, err := Init(root, nil, []string{".git/**"}, nil)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected folders to be created")
	}
	loaded, err := LoadState(root)
	if err != nil || loaded == nil {
		t.Fatalf("LoadState = %v, %v", loaded, err)
	}
	payload, err := Export(root, &state, 2)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if payload["tier"] != 2 {
		t.Fatalf("unexpected tier: %v", payload["tier"])
	}
	mustWrite(t, filepath.Join(root, "src", "main.go"), "package main\nfunc main() { println(1) }\n")
	changes, _, err := DetectChanges(root, loaded)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}
	if len(changes.Modified) != 1 {
		t.Fatalf("expected 1 modified file, got %+v", changes)
	}
}

func BenchmarkPatternMatcher(b *testing.B) {
	matcher, err := NewPatternMatcher([]string{"src/**/*.go", "**/*.md", "vendor/**"})
	if err != nil {
		b.Fatal(err)
	}
	inputs := []string{"src/main.go", "docs/readme.md", "vendor/mod/file.go", "test/test.txt"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = matcher.Matches(inputs[i%len(inputs)])
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
