package main

import (
	"reflect"
	"testing"
)

// TestUnifiedLedgerPreservesVerifierSubcommandPosition 验证统一账本转发时保留子命令与二级动作定位
func TestUnifiedLedgerPreservesVerifierSubcommandPosition(t *testing.T) {
	for _, test := range []struct{ in, want []string }{
		{[]string{"checkpoint", "build", "--story", "cancel.story.yaml"}, []string{"checkpoint", "build", "--db", "/project/.arc/arc.db", "--story", "cancel.story.yaml"}},
		{[]string{"checkpoint", "clone", "--story", "cancel.story.yaml", "--save", "save_1"}, []string{"checkpoint", "clone", "--db", "/project/.arc/arc.db", "--story", "cancel.story.yaml", "--save", "save_1"}},
		{[]string{"code", "plan", "--repo", "."}, []string{"code", "plan", "--db", "/project/.arc/arc.db", "--repo", "."}},
		{[]string{"code", "run", "--max-tier", "P1"}, []string{"code", "run", "--db", "/project/.arc/arc.db", "--max-tier", "P1"}},
		{[]string{"code", "gate"}, []string{"code", "gate", "--db", "/project/.arc/arc.db"}},
		{[]string{"play", "cancel.story.yaml", "--parallel"}, []string{"play", "--db", "/project/.arc/arc.db", "cancel.story.yaml", "--parallel"}},
		{[]string{"completion", "cancel.story.yaml"}, []string{"completion", "--db", "/project/.arc/arc.db", "cancel.story.yaml"}},
		{[]string{"explain", "--story", "ecommerce_account_cancellation", "--format", "ai-md"}, []string{"explain", "--db", "/project/.arc/arc.db", "--story", "ecommerce_account_cancellation", "--format", "ai-md"}},
		{[]string{"scan-surface", "--repo", "."}, []string{"scan-surface", "--db", "/project/.arc/arc.db", "--repo", "."}},
		{[]string{"snapshot", "--story", "ecommerce_account_cancellation", "--out", "out"}, []string{"snapshot", "--db", "/project/.arc/arc.db", "--story", "ecommerce_account_cancellation", "--out", "out"}},
		{[]string{"code"}, []string{"code"}},
		{[]string{"code", "--help"}, []string{"code", "--help"}},
		{[]string{"code", "catalog", "--lang", "rust"}, []string{"code", "catalog", "--lang", "rust"}},
		{[]string{"checkpoint"}, []string{"checkpoint"}},
	} {
		original := append([]string{}, test.in...)
		if actual := withVerifyDatabase(test.in, "/project/.arc/arc.db"); !reflect.DeepEqual(actual, test.want) {
			t.Fatalf("%v -> %v; want %v", test.in, actual, test.want)
		}
		if !reflect.DeepEqual(test.in, original) {
			t.Fatal("routing modified caller arguments")
		}
	}
	if got := extractVerifyRepoArg([]string{"code", "run", "--repo", "/tmp/other"}); got != "/tmp/other" {
		t.Fatalf("extractVerifyRepoArg mismatch: got %q", got)
	}
	if shouldLockVerifyCommand([]string{"play", "story.yaml"}) || shouldLockVerifyCommand([]string{"code", "run"}) {
		t.Fatal("long-running verify execution should not hold global arc.lock")
	}
}
