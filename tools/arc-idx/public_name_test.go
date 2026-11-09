package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The engine is private to the unified arc CLI; everything it prints reaches
// agents through `arc`, so it must never tell them to run the engine by name.
func TestEngineMessagesNameOnlyArc(t *testing.T) {
	if os.Getenv("ARC_IDX_RUN_MAIN") == "1" {
		os.Args = append([]string{"engine"}, strings.Fields(os.Getenv("ARC_IDX_ARGS"))...)
		main()
		return
	}
	repo := t.TempDir()
	for _, args := range []string{"", "help", "bogus", "search", "ast", "daemon bogus", "search zzqqxxnomatch --format ai-md"} {
		cmd := exec.Command(os.Args[0], "-test.run=TestEngineMessagesNameOnlyArc")
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "ARC_IDX_RUN_MAIN=1", "ARC_IDX_ARGS="+args)
		out, _ := cmd.CombinedOutput()
		if strings.Contains(string(out), "arc-idx") {
			t.Errorf("engine %q output names arc-idx:\n%s", args, out)
		}
	}
}
