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

const version = "0.2.0"

const usage = "arc %s — unified Arc CLI with local state database\n\n" +
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
	"  arc db <status|init|migrate>     Explicit local database lifecycle\n" +
	"  arc task <create|show|transition> [flags]  Versioned task state\n" +
	"  arc file update --path FILE     Record code file observation\n" +
	"  arc change <collect|show>       Collect changed source files\n" +
	"  arc update --changed-only       Collect Git changes idempotently\n" +
	"  arc capabilities --json        Detect supported backends and capabilities\n" +
	"  arc version                    Print the unified entrypoint version\n\n" +
	"Resource writes require an explicitly initialized/migrated .arc/arc.db.\n" +
	"Verification completion stays fail-closed; arc task done is not permitted yet.\n"

var indexCommands = map[string]bool{
	"index": true, "reindex": true, "search": true, "symbol": true,
	"symbols": true, "ast": true, "struct": true, "files": true,
	"stats": true, "doctor": true, "watch": true, "daemon": true,
}
var proposedCommands = map[string]bool{
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
	active := false
	if root, err := repositoryRoot("."); err == nil {
		if manifest, e := readManifest(root); e == nil && manifest != nil { active = true }
	}
	engines := make(map[string]any, 2)
	supported := make(map[string]bool, 2)
	for _, tool := range []string{"arc-idx", "arc-verify"} {
		p, err := resolveBackend(tool)
		supported[tool] = err == nil
		engines[tool] = map[string]any{"available": err == nil, "path": p}
	}
	return map[string]any{
		"name": "arc", "protocol_version": 1, "version": version,
		"mode": "local_state_and_legacy_backends",
		"code_index": supported["arc-idx"], "business_verify": supported["arc-verify"],
		"arc_db_resources": true, "unified_database": active,
		"arc_db_active": active, "arc_db_schema_version": arcSchemaVersion,
		"resource_commands": []string{"db", "file update", "task create", "task show", "task transition", "change collect", "change show", "update"},
		"engines": engines,
	}
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Printf(usage, version)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("arc %s — local state + legacy index/verify engines\n", version)
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
	switch args[0] {
	case "db", "task", "file", "change", "update":
		var err error
		if args[0] == "db" { err = dbCommand(args[1:]) } else { err = resourceCommand(args[0], args[1:]) }
		if err != nil { fmt.Fprintf(os.Stderr, "arc %s: %v\n", args[0], err); return 2 }
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
	forward := func() int {
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
	if args[0] == "verify" {
		root, err := repositoryRoot(".")
		if err != nil { fmt.Fprintln(os.Stderr, "arc verify:", err); return 2 }
		m, err := readManifest(root)
		if err != nil { fmt.Fprintln(os.Stderr, "arc verify:", err); return 2 }
		if m != nil && !isArcDBArgument(selected.argv) {
			selected.argv = append(selected.argv[:1], append([]string{"--db",statePath(root)},selected.argv[1:]...)...)
		}
		if m != nil {
			code := 2
			if err := withArcLock(root,func() error{ code=forward(); return nil }); err != nil {
				fmt.Fprintln(os.Stderr,"arc verify:",err); return 2
			}
			return code
		}
	}
	return forward()
}

func main() { os.Exit(run(os.Args[1:])) }
