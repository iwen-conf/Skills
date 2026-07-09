---
name: arc:code-index
description: Use .ai-code-index for local search, symbols, profiles, files, stats, refresh, and diagnostics.
---

# AI Code Index

## Overview

Use this skill when a repository has, needs, or should maintain a project-local `.ai-code-index/` toolchain. The index is local only and is optimized for AI agents that need faster, broader code discovery than repeated whole-repo `rg`.

Default behavior: search the local index first, then use symbol and structural tools, then use profile-aware file discovery. Use `rg` only for small confirmation, unindexed generated trees, protected/reference trees, or index fallback.

## Quick Contract

- **Trigger**: The user asks about `.ai-code-index`, code search coverage, stale indexes, local search workflow, symbol lookup, structural search, file discovery, or code inventory.
- **Inputs**: Project path, target scope/profile, search intent, optional language/symbol/pattern, freshness concern, and available local CLI tools.
- **Outputs**: A working search sequence, index structure guidance, commands to refresh/diagnose, and any missing tool or stale-profile risks.
- **Quality Gate**: Agent uses `.ai-code-index/search.sh`, `symbols.sh`, `files.sh`, `struct-search.sh`, or `stats.sh` before broad `rg`, and runs `doctor.sh` when results look incomplete.
- **Decision Tree**: Use this skill for local code context discovery; use `arc:build`, `arc:fix`, or `arc:audit` for the implementation, repair, or review that follows.

## Structure

Project root layout:

```text
.ai-code-index/
├── README.md
├── lib.sh
├── manifest.yaml
├── reindex.sh
├── search.sh
├── symbols.sh
├── struct-search.sh
├── files.sh
├── stats.sh
├── install-tools.sh
├── doctor.sh
├── repository-map.md
├── index/              # generated Zoekt shards, gitignored
├── tags                # generated ctags symbols, gitignored
├── .last-profile       # generated profile marker, gitignored
├── .last-indexed-at    # generated freshness marker, gitignored
└── .reindex.lock/      # transient refresh lock, gitignored
```

Core responsibilities:

- `lib.sh`: shared paths, profiles, ignore rules, file helpers, auto-refresh checks, and query filters.
- `reindex.sh`: rebuild Zoekt shards and Universal Ctags symbols; write `.last-profile` and `.last-indexed-at`.
- `search.sh`: run Zoekt search with profile filters and auto-refresh.
- `symbols.sh`: query ctags symbols with optional profile, language, and kind filters; auto-refresh when needed.
- `struct-search.sh`: run ast-grep (`sg`) for AST patterns.
- `files.sh`: profile-aware file discovery; prefer Rust `fd` / `fdfind`, fall back to `find`.
- `stats.sh`: profile-aware code inventory; prefer Go `scc`, then Rust `tokei`, then a minimal shell fallback.
- `install-tools.sh`: install recommended Rust/Go helper CLIs through Homebrew where appropriate.
- `doctor.sh`: diagnose tools, paths, stale shards, profile coverage, and smoke queries.
- `repository-map.md`: human-readable map of profile scopes and important roots.

## Profiles

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

For a new repository, define the actual paths in `lib.sh`. Do not assume these profile names map to the same directories across projects.

## Workflow

1. Start with `.ai-code-index/doctor.sh --profile <profile>` when the index is new, stale, suspicious, or recently migrated.
2. Use `.ai-code-index/search.sh --profile <profile> "<query>"` for broad intent, strings, routes, config keys, and call-site discovery.
3. Use `.ai-code-index/symbols.sh <Name> --profile <profile> [--lang <lang>] [--kind <kind>]` for definitions and symbol inventories.
4. Use `.ai-code-index/files.sh --profile <profile> [--ext <ext>] "<path-or-name-hint>"` to locate candidate files before opening many paths.
5. Use `.ai-code-index/struct-search.sh <lang> '<ast-pattern>' [paths...]` before refactors or when finding code shapes.
6. Use `.ai-code-index/stats.sh --profile <profile> [--json]` before broad audits, ownership reviews, or migration sizing.
7. Run `.ai-code-index/reindex.sh --profile <profile>` after changing profile paths, ignore rules, generated docs, or major file layouts.
8. Fall back to `rg` only for small confirmations, generated files, protected/reference trees, or cases where the index explicitly cannot cover the target.

## Commands

```bash
.ai-code-index/doctor.sh --profile code
.ai-code-index/reindex.sh --profile code
.ai-code-index/search.sh --profile backend "RecordReadingProgress file:go"
.ai-code-index/symbols.sh RecordReadingProgress --lang go --profile backend
.ai-code-index/struct-search.sh go 'func (s *Service) RecordReadingProgress($$$) ($$$) { $$$ }' backend/internal/usecase/novel/service.go
.ai-code-index/files.sh --profile backend --ext go 'controller'
.ai-code-index/stats.sh --profile code
.ai-code-index/install-tools.sh --dry-run
AI_CODE_INDEX_AUTO_REINDEX=0 .ai-code-index/search.sh --profile code "frozen query"
```

## Tooling

Core tools:

- `zoekt` / `zoekt-index`: local full-text and regex search.
- `sg` from ast-grep: structural search.
- Universal Ctags: symbol definitions.

Preferred helper CLIs are Rust or Go tools:

- Rust: `fd` / `fdfind`, `tokei`, `jaq`, `sd`
- Go: `scc`, `yq`, `dasel`

Do not add new Python or TypeScript CLI dependencies to the code-index workflow. Existing repository validation scripts may still use the repository's own language stack; this rule is for `.ai-code-index` helper tooling.

## Auto Refresh

`search.sh` and `symbols.sh` should refresh automatically when:

- no Zoekt shards or ctags file exist;
- the last profile does not cover the requested profile;
- source/config files are newer than `.last-indexed-at`;
- `.ai-code-index` scripts or AGENTS instructions changed;
- old shards point at removed paths.

Disable auto-refresh only for intentionally frozen comparisons:

```bash
AI_CODE_INDEX_AUTO_REINDEX=0 .ai-code-index/search.sh --profile code "query"
```

## Quality Gates

- The project has a `.ai-code-index/README.md` describing profiles, generated files, refresh, and agent search order.
- `doctor.sh --profile code` reports missing tools, missing paths, stale indexes, and smoke-query failures clearly.
- Default profile excludes high-noise generated, vendor, build, cache, task dump, and reference directories.
- `reindex.sh` is idempotent and safe to rerun.
- `search.sh` and `symbols.sh` either return current results or fail loudly with refresh/tool guidance.
- Profile names fail fast when unknown.
- Generated artifacts are gitignored and never treated as source of truth.
- Agents explicitly report when they used `rg` because the index could not cover the target.

## Red Flags

- Agent runs whole-repo `rg` repeatedly while `.ai-code-index/search.sh` is present and healthy.
- Search results miss active backend/frontend/mobile roots because profile paths are stale.
- Old shard names or old paths still appear after a layout migration.
- `ctags` is not Universal Ctags or symbol lookup silently returns partial results.
- `doctor.sh` is missing, stale, or does not perform smoke queries.
- `.ai-code-index/index/`, `.ai-code-index/tags`, or freshness markers are committed.
- New helper tooling requires Python, Node, TypeScript, or a remote indexing service.

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
