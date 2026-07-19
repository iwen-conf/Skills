// arc-idx is the high-performance local AI context engine required by arc:idx.
//
// It replaces the legacy .ai-code-index/*.sh scripts with a single compiled
// CLI that embeds the Zoekt index/search engine, drives universal-ctags for
// symbols, wraps ast-grep for structural queries, and always emits
// AI-consumable output (--format json | ai-md). No MCP, no color codes.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

const version = "1.0.0"

const usage = `arc-idx %s — local AI context engine (zoekt + ctags + ast-grep)

Usage:
  arc-idx index    [--repo DIR]                        rebuild zoekt shards + ctags symbols
  arc-idx search   <query> [--profile P] [--max N] [--context N] [--format json|ai-md]
  arc-idx symbol   <name>  [--kind K] [--lang L] [--exact] [--format json|ai-md]
  arc-idx ast      <pattern> --lang L [--paths ...]   [--format json|ai-md]
  arc-idx files    [pattern] [--profile P]             list candidate files
  arc-idx stats    [--profile P]                       language/LOC inventory
  arc-idx doctor                                       tool + index health report
  arc-idx watch    [--ttl MIN] [--mem MB]              foreground incremental indexer
  arc-idx daemon   start|stop|status|restart           background indexer lifecycle
  arc-idx version

Query syntax for search follows zoekt: 'foo bar', 'f:\.go$ handler', 'sym:UserService', 'lang:go case:yes Foo'.
All commands run from anywhere inside the repository; the repo root and
.ai-code-index/config.json are discovered automatically.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	// Query commands must emit only their own structured output; zoekt's
	// internal std-log chatter stays visible for indexing/daemon commands
	// and whenever ARC_IDX_DEBUG is set.
	switch cmd {
	case "index", "reindex", "watch", "daemon":
	default:
		if os.Getenv("ARC_IDX_DEBUG") == "" {
			log.SetOutput(io.Discard)
		}
	}

	var err error
	switch cmd {
	case "index", "reindex":
		err = cmdIndex(args)
	case "search":
		err = cmdSearch(args)
	case "symbol", "symbols":
		err = cmdSymbol(args)
	case "ast", "struct":
		err = cmdAst(args)
	case "files":
		err = cmdFiles(args)
	case "stats":
		err = cmdStats(args)
	case "doctor":
		err = cmdDoctor(args)
	case "watch":
		err = cmdWatch(args)
	case "daemon":
		err = cmdDaemon(args)
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, "arc-idx: unknown command %q\n\n", cmd)
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc-idx %s: %v\n", cmd, err)
		os.Exit(1)
	}
}
