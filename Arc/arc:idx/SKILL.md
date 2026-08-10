---
name: arc:idx
version: 0.2.0
description: Use the arc-idx CLI for local search, symbols, structural queries, profiles, files, stats, refresh, and diagnostics.
---

# AI Code Index

## Overview

`arc:idx` defines the architecture and usage of the **High-Performance AI Context Engine**, implemented as the compiled Go CLI `arc-idx` (source: `tools/arc-idx/` in the Skills repo; installed at `~/.local/bin/arc-idx`; rebuild with `go build -o ~/.local/bin/arc-idx ./tools/arc-idx`).

The legacy `.ai-code-index/*.sh` bash scripts are **removed**, not just deprecated. `arc-idx` embeds the Zoekt index/search engine in-process, drives universal-ctags for symbols, wraps ast-grep for AST queries, and saturates all available cores. **MCP (Model Context Protocol) is STRICTLY BANNED** for this context engine: the user's hardware handles high-concurrency CLI execution optimally, and every query must stay a zero-negotiation subprocess call.

Default behavior: Agents MUST use `arc-idx` as their primary context discovery tool. Fallback to `rg`/`fd` ONLY when the index is unavailable, the target is outside the repo, or `arc-idx doctor` reports corruption — and the fallback reason must be reported.

## Quick Contract

- **Trigger**: The user asks about `.ai-code-index`, code search coverage, stale indexes, local search workflow, symbol lookup, structural search, file discovery, or code inventory.
- **Inputs**: Project path, target scope/profile, search intent, optional language/symbol/pattern, freshness concern.
- **Outputs**: A working `arc-idx` command sequence, index structure guidance, refresh/diagnose commands, and any missing-tool or stale-index risks.
- **Quality Gate**: Agent uses `arc-idx search|symbol|ast|files|stats` before broad `rg`, and runs `arc-idx doctor` when results look incomplete.
- **Decision Tree**: Use this skill for local code context discovery; use `arc:build`, `arc:fix`, or `arc:audit` for the implementation, repair, or review that follows.

## Architecture & Interface

One binary, one repo-local home:

- `~/.local/bin/arc-idx` — the engine (Go, zoekt embedded, `runtime.NumCPU()` parallelism).
- `<repo>/.ai-code-index/config.json` — tracked config: `ignore_dirs`, `ignore_files`, `size_max`, and `profiles` (include/exclude globs, `**` supported).
- `<repo>/.ai-code-index/tags.json` — regenerated ctags symbol table (gitignored).
- `~/.cache/ai-code-index/zoekt/<repo>/` — zoekt shards, outside the working tree (override via `AI_CODE_INDEX_DIR` or `config.json.index_dir`).

Interface guarantees:

1. **Unified Query Endpoint**: every capability is a subcommand of `arc-idx`; no scattered scripts.
2. **AI-Native Formatting**: `--format json | ai-md` everywhere. No terminal color codes, no zoekt log chatter (silenced unless `ARC_IDX_DEBUG=1`), long generated lines truncated at 300 runes.
3. **High-Concurrency & Caching**: zoekt shard build and ctags run concurrently; a full rebuild of a medium repo is ~100ms-class on Apple Silicon, so freshness is cheap.
4. **AST Precision**: `arc-idx ast` delegates to ast-grep (Tree-sitter) for structural queries; `arc-idx symbol` uses universal-ctags with JSON fields; zoekt shards also carry ctags symbols for `sym:` queries.

### SDLC Integration (arc:sdlc)

When creating tasks in `docs/03-执行任务/`, the planning agent MUST include a `<Context-Queries>` section listing the exact `arc-idx` commands the execution agent runs before coding. The execution agent runs them first, expands with structural queries as needed, then codes with a non-polluted context window.

### Profiles

Profiles narrow scope without losing recall. They live in `config.json` per repository — never assume profile names transfer between projects. Typical sets: `code`, `backend`, `pc`, `android`, `docs`, `meta`; `all` always exists implicitly. Unknown profile names fail fast and list what is available.

## Commands (Implemented Interface)

```bash
# Full rebuild: zoekt shards + ctags symbols, concurrent
arc-idx index [--repo DIR] [--quiet]

# Indexed text/regex search (zoekt query syntax: f: lang: sym: case: -negation)
arc-idx search 'f:\.go$ RecordReadingProgress' --profile backend --format json
arc-idx search "sym:UserService" --context 3 --max 20

# Symbol lookup from ctags (kinds: func, struct, class, method, interface, ...)
arc-idx symbol UserService --kind struct --exact --format json

# Structural / AST match via ast-grep
arc-idx ast --lang go 'func ($_ *Service) $NAME($$$) error' --format json

# File discovery and inventory
arc-idx files handler --profile backend
arc-idx stats --profile code --format json

# Health: tools present, shard age vs source mtimes, daemon state, advice
arc-idx doctor --format json
```

Flags may appear before or after positional arguments. `search` falls back to `rg --json` automatically when no shards exist (reported as `"engine": "rg-fallback"`).

## Auto Refresh & Daemon Lifecycle

`arc-idx` ships a real fsnotify-driven incremental indexer with strict lifecycle and memory management:

```bash
arc-idx daemon start     # detached watcher; pid file + log in .ai-code-index/
arc-idx daemon status    # JSON: pid, alive, index age, staleness
arc-idx daemon stop      # SIGTERM, waits, removes pid file
arc-idx daemon restart   # stop + start
arc-idx watch [--ttl 30m] [--mem 512] [--debounce 800ms]   # foreground mode
```

Safeguards (all implemented, all defaults overridable):

- **Idle Hibernation (TTL)**: no file events for `--ttl` (default 30m) → the daemon exits and releases RAM.
- **Memory Hard Limit**: `debug.SetMemoryLimit` (default 512 MiB) keeps the Go GC inside budget.
- **Zombie Prevention**: pid file at `.ai-code-index/daemon.pid`; a watcher re-parented to PID 1 self-terminates; `doctor` flags stale pid files.
- **Debounced Rebuilds**: bursts of file events collapse into one rebuild after `--debounce` quiet time.

The AI does NOT need to run manual reindex loops. Prefer `arc-idx daemon start` at the beginning of a long session and `arc-idx daemon stop` when a massive refactor task is completely finished. For one-shot sessions, `arc-idx index` is fast enough to run before querying.

## Quality Gates

- The local indexer MUST be the compiled `arc-idx` CLI. No bash script collections, no MCP, no TypeScript indexers.
- The engine MUST keep pid tracking, memory limits, and idle TTL active in daemon mode.
- Output MUST be deterministic and strictly formatted (`--format json` or ai-md); zoekt/ctags internals stay out of the AI context.
- Symbol queries return path, line, kind, scope, and signature so a follow-up `Read` is surgical.
- `arc:sdlc` tasks MUST pre-define required `arc-idx` queries for the execution agent.
- Agents explicitly report when they used `rg` because the index could not cover the target.

## Red Flags

- Agent runs whole-repo `rg` repeatedly while `arc-idx` and its index are present.
- Terminal color codes, zoekt log lines, or unformatted dumps in the AI context.
- Resurrecting legacy `search.sh`-style wrappers or parsing raw ctags files by hand.
- Regex fishing for code structure that `arc-idx ast` or `sym:` queries answer precisely.
- A daemon left running with no pid file, or a stale index silently trusted (`doctor` exists; use it).

## When to Use

- **Preferred Trigger**: The user mentions `.ai-code-index`, `arc-idx`, local code search, index coverage, stale search results, symbol lookup, ast-grep, Zoekt, ctags, profile search, code inventory, or replacing repeated `rg`.
- **Typical Scenario**: Starting work in a large repo, improving agent search behavior, adding project profiles, diagnosing incomplete results, or documenting index structure and commands.
- **Boundary Tip**: Use this skill for discovery and maintenance of local search tooling. Use the relevant Arc lifecycle skill for the actual code change, repair, audit, or documentation work that follows.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Repository root containing or needing `.ai-code-index/` |
| `profile` | string | no | Search scope defined in `config.json`, plus implicit `all` |
| `query` | string | no | Zoekt query: text, regex, `f:`, `lang:`, `sym:`, `case:yes` |
| `language` | string | no | Language for structural (`ast`) or symbol search |
| `pattern` | string | no | ast-grep pattern when structural search is needed |
| `freshness` | string | no | Concern such as missing results, stale paths, changed layout, or old profile |

## Outputs

```text
Code Index Handoff
- Profile used
- arc-idx commands run or recommended
- Search/symbol/ast/file/stat findings
- doctor result (index age, staleness, missing tools)
- Daemon state changes (started/stopped)
- Stale path or coverage risks
- Reason for any rg fallback
```
