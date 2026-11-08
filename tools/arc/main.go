// arc is the stable CLI entry point for Arc code intelligence and verification.
// Until engine/store consolidation is complete, it delegates to the legacy binaries.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "0.1.0"

const usage = "arc %s — unified Arc CLI (compatibility bridge)\n\n" +
	"Usage:\n" +
	"  arc index [flags]               Update source index via arc-idx\n" +
	"  arc search <query> [flags]      Search indexed source\n" +
	"  arc symbol <name> [flags]       Find symbols\n" +
	"  arc ast <pattern> [flags]       Structural code search\n" +
	"  arc files [pattern] [flags]     Enumerate candidate files\n" +
	"  arc stats [flags]               Indexed-source statistics\n" +
	"  arc doctor [flags]              Diagnose index health\n" +
	"  arc daemon <action> [flags]     Manage code indexer\n" +
	"  arc verify <action> [flags]     Existing business graph/evidence engine\n" +
	"  arc capabilities --json        Detect supported backends and capabilities\n" +
	"  arc version                    Print the unified entrypoint version\n\n" +
	"New arc.db resource commands (file/task/change/update/db/sync) are not yet\n" +
	"implemented. The bridge preserves current arc-policy and arc-verify stores.\n"

var indexCommands = map[string]bool{
	"index": true, "reindex": true, "search": true, "symbol": true,
	"symbols": true, "ast": true, "struct": true, "files": true,
	"stats": true, "doctor": true, "watch": true, "daemon": true,
}
var proposedCommands = map[string]bool{
	"db": true, "file": true, "task": true, "change": true, "update": true,
	"sync": true, "project": true, "repo": true, "workspace": true,
	"worktree": true, "module": true, "dependency": true, "requirement": true,
	"design": true, "contract": true, "schema": true, "decision": true,
	"invariant": true, "plan": true, "run": true, "artifact": true,
	"test": true, "check": true, "evidence": true, "risk": true,
	"agent": true, "session": true, "context": true, "event": true,
	"policy": true, "snapshot": true,
}

type route struct {
	executable string
	argv []string
}

func selectCommand(args []string) (route, error) {
	if len(args) == 0 { return route{}, errors.New("missing subcommand") }
	name := args[0]
	if indexCommands[name] {
		return route{executable: "arc-idx", argv: args}, nil
	}
	if name == "verify" {
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return route{}, errors.New("usage: arc verify <arc-verify command> [flags]")
		}
		return route{executable: "arc-verify", argv: args[1:]}, nil
	}
	if proposedCommands[name] {
		return route{}, fmt.Errorf("arc %s is a proposed arc.db resource interface, not implemented by this bridge", name)
	}
	return route{}, fmt.Errorf("unknown arc command %q", name)
}

// resolveBackend prefers the sibling built by scripts/install_arc.sh, then PATH.
// It makes a standalone bin directory usable even when not yet on PATH.
func resolveBackend(name string) (string, error) {
	self, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(self), name)
		if runtime.GOOS == "windows" { candidate += ".exe" }
		if st, statErr := os.Stat(candidate); statErr == nil && st.Mode().IsRegular() &&
			(runtime.GOOS == "windows" || st.Mode().Perm() & 0111 != 0) {
			return candidate, nil
		}
	}
	return exec.LookPath(name)
}

func capabilities() map[string]any {
	engines := make(map[string]any, 2)
	supported := make(map[string]bool, 2)
	for _, tool := range []string{"arc-idx", "arc-verify"} {
		p, err := resolveBackend(tool)
		supported[tool] = err == nil
		engines[tool] = map[string]any{"available": err == nil, "path": p}
	}
	return map[string]any{
		"name": "arc", "protocol_version": 1, "version": version,
		"mode": "compatibility_bridge",
		"code_index": supported["arc-idx"], "business_verify": supported["arc-verify"],
		"arc_db_resources": false, "unified_database": false, "engines": engines,
	}
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Printf(usage, version)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("arc %s — compatibility bridge (arc-idx + arc-verify)\n", version)
		return 0
	}
	if args[0] == "capabilities" {
		if len(args) != 2 || args[1] != "--json" {
			fmt.Fprintln(os.Stderr, "arc capabilities: only --json is supported")
			return 2
		}
		_ = json.NewEncoder(os.Stdout).Encode(capabilities())
		return 0
	}
	selected, err := selectCommand(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "arc:", err)
		return 2
	}
	binary, err := resolveBackend(selected.executable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc: backend %s not found alongside arc or on PATH; use scripts/install_arc.sh\n", selected.executable)
		return 127
	}
	cmd := exec.Command(binary, selected.argv...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) { return exited.ExitCode() }
		fmt.Fprintf(os.Stderr, "arc: backend %s failed: %v\n", selected.executable, err)
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:])) }
