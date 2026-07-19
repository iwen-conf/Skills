---
name: arc:idx
description: Use .ai-code-index for local search, symbols, profiles, files, stats, refresh, and diagnostics.
---

# AI Code Index

## Overview

`arc:idx` defines the architecture and usage of the **High-Performance AI Context Engine**. 
*DEPRECATION NOTICE:* The legacy `.ai-code-index/*.sh` bash scripts are deprecated. They were too slow, fragile, and outputted formats hostile to AI consumption.

The modern standard dictates that the project MUST expose a robust, highly concurrent, compiled CLI (e.g., written in Go/Rust, such as `arc-idx`). **MCP (Model Context Protocol) is STRICTLY BANNED** for this specific context engine, as the user's hardware handles high-concurrency CLI execution optimally. This CLI engine is responsible for maintaining persistent memory caching, AST-level code parsing (via Tree-sitter), and delivering strictly structured context payloads (JSON or Markdown code blocks) directly to the AI.

Default behavior: Agents MUST use the high-performance CLI (`arc-idx`) as their primary context discovery tool. Fallback to `rg` or `find` ONLY when the index is unavailable or explicitly corrupted.

## Quick Contract

- **Trigger**: The user asks about `.ai-code-index`, code search coverage, stale indexes, local search workflow, symbol lookup, structural search, file discovery, or code inventory.
- **Inputs**: Project path, target scope/profile, search intent, optional language/symbol/pattern, freshness concern, and available local CLI tools.
- **Outputs**: A working search sequence, index structure guidance, commands to refresh/diagnose, and any missing tool or stale-profile risks.
- **Quality Gate**: Agent uses `.ai-code-index/search.sh`, `symbols.sh`, `files.sh`, `struct-search.sh`, or `stats.sh` before broad `rg`, and runs `doctor.sh` when results look incomplete.
- **Decision Tree**: Use this skill for local code context discovery; use `arc:build`, `arc:fix`, or `arc:audit` for the implementation, repair, or review that follows.

## Architecture & Interface

The Context Engine (compiled CLI) must provide the following unified capabilities, abstracting away underlying complexity (Zoekt, Tree-sitter, etc.) from the AI:

1. **Unified Query Endpoint**: A single command/tool (e.g., `arc-idx query --symbol "UserService"`) instead of scattered scripts.
2. **AI-Native Formatting**: MUST support `--format json` or `--format ai-md`. Terminal color codes and unstructured text dumps are strictly forbidden. Output must contain file paths, precise line numbers, and complete structural blocks (e.g., full struct definition + methods).
3. **High-Concurrency & Caching**: The engine must leverage the host's hardware (e.g., multi-threading in Go/Rust) and maintain a live or highly optimized incremental cache (e.g., via `watchexec` or `fsnotify`).
4. **AST Precision**: Use Tree-sitter or LSP-backend for structural queries, replacing outdated regex or basic `ctags`.

### SDLC Integration (arc:sdlc)
The Context Engine is tightly coupled with `arc:sdlc`. When creating tasks in `docs/03-执行任务/`, the planning agent MUST include a `<Context-Queries>` section that explicitly lists the `arc-idx` queries the execution agent must run before coding.

Use profiles to keep search broad enough for context but narrow enough to avoid noise.

| profile | intended contents |
|---|---|
| `code` | default active code surfaces plus light docs and meta files |
| `backend` | backend service code |
| `pc` | active PC reader, author, and admin frontends |
| `android` | active Android app source |
| `docs` | stable requirements/API docs, excluding task dumps and profiler output |
| `docs-full` | complete docs tree |
| `meta` | root docs/config, scripts, AGENTS.md, and `.ai-code-index` docs/scripts |
| `all` | code plus complete docs, still using ignore rules |
| `ref` | reference projects or vendor sources, on demand only |

For a new repository, configure the profile paths in the `arc-idx` config file (e.g., `.arc-idx.yaml`). Do not assume these profile names map to the same directories across projects.

## Workflow & Execution

1. **Task Handoff**: The execution agent reads `docs/03-执行任务/Txx.md` and executes the pre-defined `<Context-Queries>`.
2. **Context Expansion**: If more context is needed, the agent runs specific structural queries (e.g., finding all implementations of an interface, or usages of a struct) via the unified CLI.
3. **Execution**: With precise, non-polluted context loaded into the LLM window, the agent writes the code.

## Commands (Conceptual Specification)

*Note: The exact CLI binary name (`arc-idx`) may vary per project implementation, but the semantic interface remains standard.*

```bash
# Broad text search with AI formatting
arc-idx search "RecordReadingProgress" --profile backend --format json

# Precise AST symbol retrieval
arc-idx symbol "UserService" --kind struct --format ai-md

# Cross-reference / Usages
arc-idx references "UserService.Login" --format json

# Structural / AST match
arc-idx ast --lang go 'func (s *Service) RecordReadingProgress($$$) { $$$ }'

# Stats / Health
arc-idx stats --profile code
```

## Auto Refresh & Daemon Lifecycle

Because `arc-idx` operates as a high-concurrency background daemon (leveraging `fsnotify`/`watchexec` for sub-millisecond retrieval), **Lifecycle and Memory Management are critical**:

### 1. Daemon Control Commands
The CLI MUST expose strict lifecycle controls so the AI (or developer) can manage the engine:
```bash
arc-idx daemon start    # Boots the background indexer
arc-idx daemon stop     # Gracefully shuts down the indexer and clears RAM
arc-idx daemon status   # Reports memory footprint, uptime, and index state
arc-idx daemon restart  # Hard resets the daemon and clears memory leaks
```

### 2. Anti-Memory Leak & Resource Constraints
The engine MUST implement explicit safeguards against memory leaks and zombie processes:
- **Idle Hibernation (TTL)**: If no queries are received for a configurable threshold (e.g., 10 minutes), the daemon MUST automatically terminate or flush its AST cache to disk to release RAM.
- **Memory Hard Limits**: The binary must enforce a maximum memory footprint (e.g., via `GOMEMLIMIT` in Go, or a custom allocator in Rust). If the cache exceeds the threshold, it must trigger LRU (Least Recently Used) eviction.
- **Zombie Prevention**: The daemon must maintain a PID file (e.g., `.arc-idx.pid`). If the parent IDE/terminal process dies, the daemon must detect the orphaned state and self-terminate.
- **Garbage Collection**: Stale file shards and outdated AST trees must be periodically swept from memory during incremental updates.

The AI does NOT need to run manual `reindex` scripts, but the AI SHOULD proactively run `arc-idx daemon stop` when a massive refactor task is completely finished.

## Quality Gates

- The local indexer MUST NOT be a collection of scattered bash scripts, and MUST NOT use MCP. It MUST be a compiled CLI (Go/Rust).
- The engine MUST implement PID tracking, memory eviction (LRU), and idle timeouts to prevent memory leaks.
- Output MUST be deterministic and strictly formatted (`--format json` or `--format ai-md`).
- Queries MUST return associated context (e.g., querying a struct returns its methods).
- `arc:sdlc` tasks MUST pre-define required queries for the execution agent.
- Agents explicitly report when they used `rg` because the index could not cover the target.

## Red Flags

- Agent runs whole-repo `rg` repeatedly while the high-performance indexer is present.
- Outputting terminal color codes, messy logs, or unformatted text to the AI context.
- Fallback to legacy `search.sh` or manual `ctags` parsing.
- Using regex to find complex code structures instead of relying on the indexer's AST/Tree-sitter capabilities.

## When to Use

- **Preferred Trigger**: The user mentions `.ai-code-index`, local code search, index coverage, stale search results, symbol lookup, ast-grep, Zoekt, ctags, profile search, code inventory, or replacing repeated `rg`.
- **Typical Scenario**: Starting work in a large repo, improving agent search behavior, adding project profiles, diagnosing incomplete results, or documenting index structure and commands.
- **Boundary Tip**: Use this skill for discovery and maintenance of local search tooling. Use the relevant Arc lifecycle skill for the actual code change, repair, audit, or documentation work that follows.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Repository root containing or needing `.ai-code-index/` |
| `profile` | string | no | Search scope such as `code`, `backend`, `pc`, `android`, `docs`, `all`, or project-specific profile |
| `query` | string | no | Text, regex, symbol, route, config key, or intent to search |
| `language` | string | no | Language for structural or symbol search |
| `pattern` | string | no | ast-grep pattern when structural search is needed |
| `freshness` | string | no | Concern such as missing results, stale paths, changed layout, or old profile |

## Outputs

```text
Code Index Handoff
- Profile used
- Commands run or recommended
- Search/symbol/file/stat findings
- Refresh or doctor result
- Missing Rust/Go helper tools
- Stale path or coverage risks
- Reason for any rg fallback
```
