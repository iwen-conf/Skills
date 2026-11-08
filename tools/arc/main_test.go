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
		{[]string{"verify", "gate", "--graph", "g1"}, "arc-verify", []string{"gate", "--graph", "g1"}, false},
		{[]string{"verify", "discover", "list", "--graph", "g1"}, "arc-verify", []string{"discover", "list", "--graph", "g1"}, false},
		{[]string{"verify"}, "", nil, true},
		{[]string{"task", "create"}, "", nil, true},
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

func TestCapabilitiesDoNotAdvertiseUnimplementedResources(t *testing.T) {
	got := capabilities()
	if got["unified_database"] != false || got["arc_db_resources"] != false {
		t.Fatal("must not claim implemented state engine")
	}
}

func TestHelpAndVersion(t *testing.T) {
	if run([]string{"--help"}) != 0 || run([]string{"version"}) != 0 {
		t.Fatal("help/version failed")
	}
	if !strings.Contains(usage, "arc verify") { t.Fatal("missing verification route") }
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
