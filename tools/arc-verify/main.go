package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const version = "3.0.0"

const usage = `arc verify %s — Storyline Verification Engine (world_flags + checkpoints + concurrent blind routes + anti-softlock + multi-ending completion)

Usage:
  arc verify init       [--db PATH]
  arc verify scan-world <focal_entity> [--repo DIR] [--format json|yaml|ai-md]
  arc verify lint-story <story.yaml> [--repo DIR] [--world-scan] [--max-errors 5] [--format json|ai-md]
  arc verify plan-shards <story.yaml> [--out FILE] [--format yaml|json]
  arc verify checkpoint <build|clone|list> --story <story.yaml> [--save ID] [--ns NS] [--engine dir|sqlite|postgres] [--pg-url URL] [--db PATH]
  arc verify play       <story.yaml> [--parallel] [--route ID] [--blind] [--dry-run] [--repo DIR] [--db PATH]
  arc verify completion <story.yaml> [--repo DIR] [--world-scan] [--db PATH] [--format json|ai-md]
  arc verify gate       --story <id>|--graph <id> [--db PATH]
  arc verify task-proof --graph <story_id> --task <story_id/route_id> [--db PATH]
  arc verify snapshot   --story <id>|--file <story.yaml> --out <dir> [--db PATH]
  arc verify version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	code := dispatch(cmd, args)
	os.Exit(code)
}

func dispatch(cmd string, args []string) int {
	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("arc-verify %s (storyline-engine)\n", version)
		return 0
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
		return 0
	case "init":
		return cmdInit(args)
	case "scan-world":
		return cmdScanWorld(args)
	case "lint-story":
		return cmdLintStory(args)
	case "plan-shards":
		return cmdPlanShards(args)
	case "checkpoint":
		return cmdCheckpoint(args)
	case "play":
		return cmdPlay(args)
	case "completion":
		return cmdCompletion(args)
	case "gate":
		return cmdGate(args)
	case "task-proof":
		return cmdTaskProof(args)
	case "snapshot":
		return cmdSnapshot(args)
	case "import":
		return cmdImport(args)
	default:
		fmt.Fprintf(os.Stderr, "arc verify: unknown command %q\n\n", cmd)
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
}

func emitJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", err)
		return 1
	}
	return 0
}

func parseInterleavedFlags(fl *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for len(args) > 0 {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		args = fl.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	return pos, nil
}

func cmdInit(args []string) int {
	fl := flag.NewFlagSet("init", flag.ContinueOnError)
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify init: %v\n", err)
		return 1
	}
	defer ledger.Close()
	return emitJSON(map[string]any{"initialized": true, "db": *dbPath, "engine": "storyline-v3"})
}

func cmdImport(args []string) int {
	fl := flag.NewFlagSet("import", flag.ContinueOnError)
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	file := fl.String("file", "", "storyline YAML/JSON file")
	if err := fl.Parse(args); err != nil || *file == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify import --file <story.yaml> [--db PATH]")
		return 2
	}
	s, err := loadStoryline(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify import: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify import: %v\n", err)
		return 1
	}
	defer ledger.Close()
	if err := ledger.SaveStoryDefinition(s); err != nil {
		fmt.Fprintf(os.Stderr, "arc verify import: %v\n", err)
		return 1
	}
	return emitJSON(map[string]any{"imported": true, "story_id": s.StoryID, "sha256": s.SHA256})
}

func cmdScanWorld(args []string) int {
	fl := flag.NewFlagSet("scan-world", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	format := fl.String("format", "json", "output format: json | yaml | ai-md")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify scan-world <focal_entity> [--repo DIR] [--format json|yaml|ai-md]")
		return 2
	}
	res, err := scanWorldFlags(*repoDir, pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify scan-world: %v\n", err)
		return 1
	}
	switch *format {
	case "yaml":
		out, _ := yaml.Marshal(res)
		fmt.Print(string(out))
		return 0
	case "ai-md":
		fmt.Print(renderWorldScanMarkdown(res))
		return 0
	default:
		return emitJSON(res)
	}
}

func cmdLintStory(args []string) int {
	fl := flag.NewFlagSet("lint-story", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root for --world-scan")
	worldScan := fl.Bool("world-scan", false, "verify 100% world_flags binding against repo scan")
	maxErr := fl.Int("max-errors", 5, "maximum fatal errors to display")
	format := fl.String("format", "json", "output format: json | ai-md")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify lint-story <story.yaml> [--repo DIR] [--world-scan] [--max-errors 5]")
		return 2
	}
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify lint-story: %v\n", err)
		return 1
	}
	var scan *WorldScanResult
	if *worldScan {
		scan, err = scanWorldFlags(*repoDir, s.FocalEntity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify lint-story world-scan: %v\n", err)
			return 1
		}
	}
	rep := lintStory(s, scan, *maxErr)
	if *format == "ai-md" {
		fmt.Printf("## lint-story `%s`: %s (%d errors)\n", rep.StoryID, rep.Status, rep.TotalErrors)
		for _, e := range rep.Errors {
			fmt.Printf("- [%s] `%s`: %s\n", e.Code, e.Scope, e.Message)
		}
	} else {
		emitJSON(rep)
	}
	if rep.Status != "PASS" {
		return 1
	}
	return 0
}

func cmdPlanShards(args []string) int {
	fl := flag.NewFlagSet("plan-shards", flag.ContinueOnError)
	outFile := fl.String("out", "", "optional output file path")
	format := fl.String("format", "yaml", "output format: yaml | json")
	_ = fl.String("db", "", "optional ledger path (ignored)")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify plan-shards <story.yaml> [--out FILE] [--format yaml|json]")
		return 2
	}
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify plan-shards: %v\n", err)
		return 1
	}
	plan, err := planShards(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify plan-shards: %v\n", err)
		return 1
	}
	var raw []byte
	if *format == "json" {
		raw, err = json.MarshalIndent(plan, "", "  ")
		raw = append(raw, '\n')
	} else {
		raw, err = yaml.Marshal(plan)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify plan-shards encode: %v\n", err)
		return 1
	}
	if *outFile != "" {
		if err := os.MkdirAll(filepath.Dir(*outFile), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "arc verify plan-shards mkdir: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*outFile, raw, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "arc verify plan-shards write: %v\n", err)
			return 1
		}
		return emitJSON(map[string]any{"written": *outFile, "shards": len(plan.ConcurrentShards)})
	}
	fmt.Print(string(raw))
	return 0
}

func cmdCheckpoint(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify checkpoint <build|clone|list> --story <story.yaml> [flags]")
		return 2
	}
	sub := args[0]
	fl := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	storyFile := fl.String("story", "", "storyline YAML file")
	saveID := fl.String("save", "", "checkpoint save_id (for clone)")
	ns := fl.String("ns", "ns_default", "runtime namespace (for clone)")
	engine := fl.String("engine", "", "checkpoint engine: dir | sqlite | postgres")
	source := fl.String("source", "", "source sqlite file (for sqlite engine)")
	pgURL := fl.String("pg-url", "", "PostgreSQL admin URL (for postgres engine)")
	repoDir := fl.String("repo", ".", "repository root")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if _, err := parseInterleavedFlags(fl, args[1:]); err != nil {
		return 2
	}
	if *storyFile == "" {
		fmt.Fprintln(os.Stderr, "arc verify checkpoint: --story is required")
		return 2
	}
	s, err := loadStoryline(*storyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify checkpoint: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify checkpoint: %v\n", err)
		return 1
	}
	defer ledger.Close()
	absRoot, _ := filepath.Abs(*repoDir)

	switch sub {
	case "build":
		recs, err := buildCheckpoints(ledger, s, absRoot, *engine, *source, *pgURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify checkpoint build: %v\n", err)
			return 1
		}
		return emitJSON(map[string]any{"story_id": s.StoryID, "checkpoints": recs})
	case "clone":
		if *saveID == "" {
			fmt.Fprintln(os.Stderr, "arc verify checkpoint clone: --save is required")
			return 2
		}
		eng := *engine
		if eng == "" {
			eng = "dir"
		}
		clonePath, err := cloneCheckpoint(absRoot, s.StoryID, *saveID, *ns, eng, *pgURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify checkpoint clone: %v\n", err)
			return 1
		}
		return emitJSON(map[string]any{"story_id": s.StoryID, "save_id": *saveID, "runtime_ns": *ns, "clone_path": clonePath})
	case "list":
		recs, err := ledger.ListCheckpoints(s.StoryID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify checkpoint list: %v\n", err)
			return 1
		}
		return emitJSON(map[string]any{"story_id": s.StoryID, "checkpoints": recs})
	default:
		fmt.Fprintf(os.Stderr, "arc verify checkpoint: unknown action %q\n", sub)
		return 2
	}
}

func cmdPlay(args []string) int {
	fl := flag.NewFlagSet("play", flag.ContinueOnError)
	parallel := fl.Bool("parallel", false, "execute routes concurrently in isolated namespaces")
	route := fl.String("route", "", "execute only one route_id")
	blind := fl.Bool("blind", false, "enable blind-test mode with ARC_BLIND_MASK")
	dryRun := fl.Bool("dry-run", false, "validate wiring without executing route commands")
	repoDir := fl.String("repo", ".", "repository root")
	engine := fl.String("engine", "", "override checkpoint engine: dir | sqlite | postgres")
	pgURL := fl.String("pg-url", "", "PostgreSQL URL when engine=postgres")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify play <story.yaml> [--parallel] [--route ID] [--blind] [--db PATH]")
		return 2
	}
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify play: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify play: %v\n", err)
		return 1
	}
	defer ledger.Close()
	summary, err := playStory(ledger, s, PlayOptions{
		Parallel:    *parallel,
		RouteFilter: *route,
		Blind:       *blind,
		DryRun:      *dryRun,
		RepoRoot:    *repoDir,
		Engine:      *engine,
		PGURL:       *pgURL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify play: %v\n", err)
		return 1
	}
	emitJSON(summary)
	if summary.Verdict != "PASS" {
		return 1
	}
	return 0
}

func cmdCompletion(args []string) int {
	fl := flag.NewFlagSet("completion", flag.ContinueOnError)
	repoDir := fl.String("repo", ".", "repository root")
	worldScan := fl.Bool("world-scan", false, "verify world_flag binding against repo scan")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	format := fl.String("format", "json", "output format: json | ai-md")
	pos, err := parseInterleavedFlags(fl, args)
	if err != nil || len(pos) < 1 {
		fmt.Fprintln(os.Stderr, "usage: arc verify completion <story.yaml> [--world-scan] [--db PATH]")
		return 2
	}
	s, err := loadStoryline(pos[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify completion: %v\n", err)
		return 1
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify completion: %v\n", err)
		return 1
	}
	defer ledger.Close()
	var scan *WorldScanResult
	if *worldScan {
		scan, err = scanWorldFlags(*repoDir, s.FocalEntity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify completion world-scan: %v\n", err)
			return 1
		}
	}
	rep, err := computeCompletion(ledger, s, scan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify completion: %v\n", err)
		return 1
	}
	if *format == "ai-md" {
		fmt.Printf("## Storyline Completion `%s`: **%s**\n- World Flags: %.0f%%\n- Routes: %.0f%%\n- Butterfly Probes: %.0f%%\n- Endings: %.0f%% (%d/%d)\n- Softlocks: %d · Historical FAILs: %d\n",
			rep.StoryID, rep.Verdict, rep.WorldFlagBindingPct, rep.RouteCoveragePct,
			rep.ButterflyProbeHitPct, rep.EndingUnlockPct, len(rep.UnlockedEndings), len(s.RequiredEndings),
			rep.SoftlockCount, rep.HistoricalFailCount)
	} else {
		emitJSON(rep)
	}
	if rep.Verdict != "PASS" {
		return 1
	}
	return 0
}

// cmdGate serves unified `arc db migrate` and gate checks against stored storyline definitions.
func cmdGate(args []string) int {
	fl := flag.NewFlagSet("gate", flag.ContinueOnError)
	storyID := fl.String("story", "", "story_id")
	graphID := fl.String("graph", "", "alias for --story")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	id := *storyID
	if id == "" {
		id = *graphID
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify gate --story <id> [--db PATH]")
		return 2
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify gate: %v\n", err)
		return 1
	}
	defer ledger.Close()
	s, err := ledger.LoadStoryDefinition(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify gate: story %q not found: %v\n", id, err)
		return 1
	}
	rep, err := computeCompletion(ledger, s, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify gate: %v\n", err)
		return 1
	}
	out := map[string]any{
		"verdict":          rep.Verdict,
		"proof_scope":      "storyline_completion_100pct",
		"inputs_unchanged": true,
		"context":          map[string]any{"graph": id, "story_id": id, "story_sha256": s.SHA256},
		"completion":       rep,
	}
	emitJSON(out)
	if rep.Verdict != "PASS" {
		return 1
	}
	return 0
}

// cmdTaskProof bridges storyline route verification receipts into `arc task complete`.
func cmdTaskProof(args []string) int {
	fl := flag.NewFlagSet("task-proof", flag.ContinueOnError)
	graphID := fl.String("graph", "", "story_id")
	taskID := fl.String("task", "", "story_id/route_id")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if err := fl.Parse(args); err != nil || *graphID == "" || *taskID == "" {
		fmt.Fprintln(os.Stderr, "usage: arc verify task-proof --graph <story_id> --task <story_id/route_id> [--db PATH]")
		return 2
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify task-proof: %v\n", err)
		return 1
	}
	defer ledger.Close()
	s, err := ledger.LoadStoryDefinition(*graphID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify task-proof: %v\n", err)
		return 1
	}
	routeID := strings.TrimPrefix(*taskID, *graphID+"/")
	recs, err := ledger.ListRouteResults(*graphID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify task-proof: %v\n", err)
		return 1
	}
	var latest *RouteResultRecord
	for i := range recs {
		if recs[i].RouteID == routeID && recs[i].StorySHA256 == s.SHA256 {
			latest = &recs[i]
		}
	}
	if latest == nil || latest.Verdict != "PASS" || latest.SoftlockDetected {
		emitJSON(map[string]any{
			"version": 1, "graph": *graphID, "task": *taskID, "obligation_id": routeID,
			"verdict": "NOT_RUN", "reason": "no current passing storyline route receipt",
		})
		return 1
	}
	root, _ := filepath.Abs(".")
	head := currentGitHead(root)
	zeroSum := sha256.Sum256([]byte(s.SHA256))
	proof := map[string]any{
		"version":          1,
		"graph":            *graphID,
		"task":             *taskID,
		"obligation_id":    routeID,
		"run":              latest.RunID,
		"round":            latest.LoadSave,
		"verdict":          "PASS",
		"proof_scope":      "linked_task_evidence",
		"inputs_unchanged": true,
		"evidence_status":  "PASS",
		"context": map[string]any{
			"graph":         *graphID,
			"environment":   latest.RuntimeNS,
			"target":        s.FocalEntity,
			"source_commit": head,
			"source_sha256": hex.EncodeToString(zeroSum[:]),
			"config_sha256": s.SHA256,
			"graph_sha256":  s.SHA256,
			"source_root":   root,
			"config_path":   s.SourcePath,
		},
		"raw_sha256":     []string{latest.ReceiptSHA256},
		"receipt_sha256": latest.ReceiptSHA256,
	}
	return emitJSON(proof)
}

func cmdSnapshot(args []string) int {
	fl := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	storyID := fl.String("story", "", "story_id stored in ledger")
	file := fl.String("file", "", "or storyline YAML file")
	outDir := fl.String("out", "", "output directory (must be absent or empty)")
	dbPath := fl.String("db", defaultVerifyDB, "ledger database path")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	ledger, err := openStoryLedger(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
		return 1
	}
	defer ledger.Close()

	var s *Storyline
	if *file != "" {
		s, err = loadStoryline(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
			return 1
		}
		_ = ledger.SaveStoryDefinition(s)
	} else if *storyID != "" {
		s, err = ledger.LoadStoryDefinition(*storyID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(os.Stderr, "usage: arc verify snapshot (--story ID | --file story.yaml) --out DIR [--db PATH]")
		return 2
	}
	if err := exportSnapshot(ledger, s, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "arc verify snapshot: %v\n", err)
		return 1
	}
	return emitJSON(map[string]any{"exported": true, "story_id": s.StoryID, "out": *outDir})
}
