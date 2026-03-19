package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"skills/arcgocore/internal/cartography"
)

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	command := os.Args[1]
	switch command {
	case "init":
		runInit(os.Args[2:])
	case "changes":
		runChanges(os.Args[2:])
	case "update":
		runUpdate(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Repository root path")
	var includes, excludes, exceptions stringList
	fs.Var(&includes, "include", "Glob patterns for files to include")
	fs.Var(&excludes, "exclude", "Glob patterns for files to exclude")
	fs.Var(&exceptions, "exception", "Explicit file paths to include despite exclusions")
	fs.Parse(args)
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", root)
		os.Exit(1)
	}
	fmt.Printf("Scanning %s...\n", root)
	fmt.Printf("Include patterns: %v\n", []string(includes))
	fmt.Printf("Exclude patterns: %v\n", []string(excludes))
	fmt.Printf("Exceptions: %v\n", []string(exceptions))
	state, folders, err := cartography.Init(root, includes, excludes, exceptions)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Selected %d files\n", len(state.FileHashes))
	fmt.Printf("Created %s/%s\n", cartography.StateDir, cartography.StateFile)
	fmt.Printf("Created %d empty codemap.md files\n", len(folders))
}

func runChanges(args []string) {
	fs := flag.NewFlagSet("changes", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Repository root path")
	fs.Parse(args)
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	state, err := cartography.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if state == nil {
		fmt.Fprintln(os.Stderr, "No cartography state found. Run 'init' first.")
		os.Exit(1)
	}
	changes, _, err := cartography.DetectChanges(root, state)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(cartography.PrintChanges(changes))
}

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Repository root path")
	fs.Parse(args)
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	state, err := cartography.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if state == nil {
		fmt.Fprintln(os.Stderr, "No cartography state found. Run 'init' first.")
		os.Exit(1)
	}
	next, err := cartography.Update(root, state)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Updated %s/%s with %d files\n", cartography.StateDir, cartography.StateFile, len(next.FileHashes))
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	rootFlag := fs.String("root", "", "Repository root path")
	tier := fs.Int("tier", 1, "Detail tier")
	output := fs.String("output", "-", "Output file path")
	fs.Parse(args)
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	state, err := cartography.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if state == nil {
		fmt.Fprintln(os.Stderr, "No cartography state found. Run 'init' first.")
		os.Exit(1)
	}
	payload, err := cartography.Export(root, state, *tier)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cartography.WriteExport(*output, payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *output != "-" {
		entries, _ := payload["entries"].([]map[string]any)
		fmt.Printf("Exported %d entries to %s (tier %d)\n", len(entries), *output, *tier)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  cartographer init --root /path/to/repo --include "src/**/*.ts" --exclude "node_modules/**"
  cartographer changes --root /path/to/repo
  cartographer update --root /path/to/repo
  cartographer export --root /path/to/repo --tier 2 --output codemap.json`)
}
