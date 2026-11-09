package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestCommandRouting(t *testing.T) {
	cases := []struct {
		args []string
		tool string
		forward []string
		wantError bool
	}{
		{[]string{"search", "hello", "--format", "json"}, "arc-idx", []string{"search", "hello", "--format", "json"}, false},
		{[]string{"idx", "search", "hello", "--format", "json"}, "arc-idx", []string{"search", "hello", "--format", "json"}, false},
		{[]string{"idx", "daemon", "status"}, "arc-idx", []string{"daemon", "status"}, false},
		{[]string{"idx"}, "", nil, true},
		{[]string{"idx", "--format", "json"}, "", nil, true},
		{[]string{"idx", "flush"}, "", nil, true},
		{[]string{"idx", "task", "create"}, "", nil, true},
		{[]string{"verify", "gate", "--graph", "g1"}, "arc-verify", []string{"gate", "--graph", "g1"}, false},
		{[]string{"verify", "discover", "list", "--graph", "g1"}, "arc-verify", []string{"discover", "list", "--graph", "g1"}, false},
		{[]string{"verify"}, "", nil, true},
		{[]string{"task", "create"}, "", nil, true}, // handled by local resource router, not delegation
		{[]string{"update"}, "", nil, true},
		{[]string{"huh"}, "", nil, true},
	}
	for _, c := range cases {
		got, err := selectCommand(c.args)
		if (err != nil) != c.wantError { t.Fatalf("%v: error=%v", c.args, err) }
		if c.wantError { continue }
		if got.executable != c.tool || !reflect.DeepEqual(got.argv, c.forward) {
			t.Fatalf("%v: got %+v", c.args, got)
		}
	}
}

func TestCapabilitiesAdvertiseImplementedResourcesNotUnmigratedDB(t *testing.T) {
	got := capabilities()
	if got["arc_db_resources"] != true { t.Fatal("resource engine should be reported implemented") }
	if got["arc_db_schema_version"] != arcSchemaVersion { t.Fatal("schema version not reported") }
	// Unified state is advertised only when the current repository has a
	// validated migration marker, not when a second database merely exists.
	if got["arc_db_active"] == false && got["unified_database"] != false {
		t.Fatal("must not advertise inactive DB as unified")
	}
}

func TestHelpAndVersion(t *testing.T) {
	if run([]string{"--help"}) != 0 || run([]string{"version"}) != 0 {
		t.Fatal("help/version failed")
	}
	if !strings.Contains(usage, "arc verify") || !strings.Contains(usage,"arc task") {
		t.Fatal("missing commands from help")
	}
}

func TestLegacyBackendArgumentAndExitForwarding(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("shell-script backend test is Unix-only") }
	for _, tc := range []struct{
		name string
		binary string
		args []string
		expected string
	}{
		{"source", "arc-idx", []string{"search", "lang:go Foo", "--format", "json"}, "search\nlang:go Foo\n--format\njson\n"},
		{"verification", "arc-verify", []string{"verify", "gate", "--graph", "G1"}, "gate\n--graph\nG1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			capture := filepath.Join(dir, "args.txt")
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARC_CAPTURE\"\nexit 17\n"
			if err := os.WriteFile(filepath.Join(dir, tc.binary), []byte(script), 0700); err != nil { t.Fatal(err) }
			t.Setenv("PATH", dir)
			t.Setenv("ARC_CAPTURE", capture)
			if code := run(tc.args); code != 17 { t.Fatalf("exit code %d, want 17", code) }
			got, err := os.ReadFile(capture)
			if err != nil { t.Fatal(err) }
			if string(got) != tc.expected { t.Fatalf("argv: got %q, want %q", got, tc.expected) }
		})
	}
}

func TestDelegationPreservesBackendExitCode(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("shell stub uses /bin/sh") }
	dir := t.TempDir()
	for _, name := range []string{"arc-idx", "arc-verify"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 19\n"), 0755); err != nil { t.Fatal(err) }
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got := run([]string{"search", "some-text"}); got != 19 {
		t.Fatalf("index exit code: got %d, want 19", got)
	}
	if got := run([]string{"verify", "gate", "--graph", "example"}); got != 19 {
		t.Fatalf("verify exit code: got %d, want 19", got)
	}
}

func TestBackendResolvesFromPrivateLibexec(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("Unix layout only") }
	prefix := t.TempDir()
	bin := filepath.Join(prefix, "bin")
	private := filepath.Join(prefix, "libexec", "arc")
	for _, dir := range []string{bin, private} {
		if err := os.MkdirAll(dir, 0o755); err != nil { t.Fatal(err) }
	}
	backend := filepath.Join(private, "arc-idx")
	if err := os.WriteFile(backend, []byte("#!/bin/sh\n"), 0o755); err != nil { t.Fatal(err) }
	t.Setenv("PATH", t.TempDir())
	got, err := resolveBackendFrom(bin, "arc-idx")
	if err != nil || got != backend { t.Fatalf("got %q, %v; want %q", got, err, backend) }
	sibling := filepath.Join(bin, "arc-idx")
	if err := os.WriteFile(sibling, []byte("#!/bin/sh\n"), 0o755); err != nil { t.Fatal(err) }
	if got, _ := resolveBackendFrom(bin, "arc-idx"); got != backend {
		t.Fatalf("private backend must win over a stale public copy, got %q", got)
	}
}
