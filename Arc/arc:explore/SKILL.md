---
name: arc:explore
version: 2.2.0
description: >-
  Read-only code intelligence for a local repository: find where code lives (代码在哪, 搜代码),
  locate definitions, list callers (谁调用), trace a call chain (调用链) or data flow, list
  everything a planned change would touch (影响面 / impact analysis), and draw a module or
  architecture map (架构地图). Also maintains the arc-idx / zoekt index (索引维护: doctor, index,
  daemon). Not for edits (use arc:code), read-only risk review (use arc:audit), or tests (use
  arc:verify).
---
# arc:explore

Answer where code lives, who depends on it and what a change would touch, with reproducible
`arc-idx` query evidence; change nothing except index files.

## Non-trigger

- Make the change (implement, rename, refactor, fix, comment) → `arc:code`; it runs code-intel itself.
- Find defects or risks, review a diff, health or security audit → `arc:audit`.
- Write or run tests → `arc:verify`.
- Write a PRD, brief or task document → `arc:plan`.
- Design or review business lifecycle completeness → `arc:plan` model mode; exploration supplies source facts.
- Read one file at a known path → read it directly.
- Third-party library documentation → the host's web or docs tools.
- Feishu/Lark project documents → `lark` (project-docs).

## Goal

Return an answer in which every claim cites a read `file:line` and a reproducible query, and, in
impact mode, an eight-row table that `arc:code` can use as its edit list.

## Inputs

- `question` (required).
- `scope` (optional): paths to search; default the `repo_root` reported by doctor.
- `seeds` (optional): symbol, route, table or column, config key, error string.
- `depth` (optional, trace mode): default 3 hops.
- `mode` (optional): `locate` · `trace` · `impact` · `map` · `index-ops`; default from the Intent Router.

## Intent Router

First matching row wins.

| Condition | Mode | Read |
|---|---|---|
| Question contains 改, 换, 删, 重命名, rename, remove, replace, or asks what a change touches (影响面) | impact | [code-intel §4](../_shared/code-intel.md#4-impact-checklist) |
| Who calls X, call chain, data flow (谁调用, 调用链) | trace | [code-intel §4 rows 1–4](../_shared/code-intel.md#4-impact-checklist) |
| Where a file, definition or string lives (代码在哪, 搜代码, 找符号) | locate | [code-intel §3](../_shared/code-intel.md#3-query-ladder-stop-at-the-first-rung-that-answers-the-question) |
| Module or system structure, entry points, layers (架构地图, 模块梳理) | map | [references/architecture-map.md](references/architecture-map.md) |
| doctor, index, daemon, stale or suspicious results, index config | index-ops | [code-intel §1](../_shared/code-intel.md#1-preflight-once-per-repository-per-session) |

## Workflow

1. Read [contract.md](../_shared/contract.md) and [code-intel.md](../_shared/code-intel.md); run the
   code-intel §1 preflight. Output one line: `arc-idx <version> · repo_root · healthy · index_stale ·
   daemon_alive · repairs run`.
2. Select the mode with the Intent Router. Output: `Mode · scope · depth`.
3. Turn the question into seeds: identifiers, route strings, error strings, table or column names,
   config keys. Output: the seed list.
4. Run the mode:
   - **locate**: walk the [code-intel §3](../_shared/code-intel.md#3-query-ladder-stop-at-the-first-rung-that-answers-the-question)
     ladder and stop at the first rung that answers.
   - **trace**: definition (§4 row 1) → callers (row 2) → callers of each caller, until an entry
     point (route registration, `main`, cron registration, queue consumer, CLI command) or `depth`.
     For data flow, follow the field through rows 3–4 (writes, reads, serialization) layer by layer.
     Output: chains `entry → … → target`, one `file:line` per hop.
   - **impact**: fill all eight §4 rows for every symbol, field, route, column, config key or event
     the change names; cross-check rows 2–4 with one structural and one text query.
   - **map**: follow [architecture-map.md](references/architecture-map.md); label layers against
     [go-backend.md](../_shared/conventions/go-backend.md) or [frontend.md](../_shared/conventions/frontend.md).
   - **index-ops**: run the requested operation from Index operations below; record doctor JSON
     before and after.
5. Open the file at every hit and read the enclosing function or type. Output: the `file:line`
   evidence list.
6. Resolve dynamic dispatch and generated code per [code-intel §5](../_shared/code-intel.md#5-when-a-query-fails).
   Output: the unresolved list, each item marked `dynamic — confirm at runtime`.
7. Write the contract §9 report with the mode output: answer, evidence table in
   [code-intel §7](../_shared/code-intel.md#7-evidence-format) format, impact table, frontier nodes,
   dynamic items, coverage limits, and the next handoff with the exact queries to re-run.

### Index operations

- Status: `arc-idx doctor --format json`; `arc-idx daemon status` (JSON: `pid`, `alive`,
  `index_dir`, `index_age`, `log_file`, `stale_index`).
- Rebuild: `arc-idx index --quiet`, once per repair. While the daemon is alive it reindexes 800 ms
  after file changes; do not loop manual rebuilds.
- Daemon: `arc-idx daemon start|stop|restart|status`. PID file `.ai-code-index/daemon.pid`, log
  `.ai-code-index/daemon.log`; it exits after 30 minutes without file events and runs under a
  512 MiB memory limit. Run `daemon stop` after a large refactor has fully finished.
- Shards: `~/.cache/ai-code-index/zoekt/<repo basename>/`, overridden by `index_dir` in
  `.ai-code-index/config.json` or the `AI_CODE_INDEX_DIR` environment variable.
- Config keys in `.ai-code-index/config.json`: `profiles`, `ignore_dirs`, `ignore_files`,
  `index_exts`, `max_files` (default 50000), `size_max` (default 1 MiB), `index_dir`. Edit it only on
  a user request or a Failure Recovery row below, and report the diff.
- Engine source in the Skills repository is `tools/arc-idx/`. After an engine change and only on user
  request: `go -C tools/arc-idx build -o ~/.local/bin/arc-idx .`, then `arc-idx version`.

## Tool Strategy

- **Preferred**: the `arc-idx` commands in [code-intel §2](../_shared/code-intel.md#2-commands). JSON
  output for counting, `--format ai-md --context 3` for reading hits. Run independent queries in
  parallel.
- **Query order**: locate follows §3; trace runs row 1, then row 2 per hop; impact runs rows 1–8 in
  order; map runs `stats` → `files` → entry-point searches → reads.
- **Fallback**: host-native search only under [code-intel §6](../_shared/code-intel.md#6-fallback-conditions),
  labelled `fallback:<tool>` with the reason.
- **Forbidden**:
  - guessing a path, symbol or route;
  - claiming "no callers" or "unused" from a single query type;
  - natural-language sentences as queries;
  - reading a large directory linearly or listing it recursively instead of `files` and `stats`;
  - calling ctags, ast-grep or zoekt binaries directly, or another index or MCP search as the primary
    engine;
  - pasting `.ai-code-index/tags.json` or unbounded command output into context;
  - editing source, tests, docs or configs other than index files;
  - relying on a search with `"engine": "rg-fallback"` outside code-intel §6.
- **When a query fails**: widen or narrow in the code-intel §5 order, then apply Failure Recovery.

## Decision Rules

- IF the question contains a change verb (改, 换, 删, 重命名, rename, remove, replace) THEN run impact
  mode, even when it also asks where something lives.
- IF a trace reaches `depth` (default 3 hops) before an entry point THEN stop and list every
  unexpanded frontier node with `file:line`; never truncate silently.
- IF a query returns more than 200 hits, or `file_count` equals `--max`, THEN narrow by directory,
  language and word boundary before reading. IF still above 200 after three narrowing passes THEN
  read at most 5 files per directory and label the result `sampled`.
- IF no seed can be extracted from the question THEN run at most 3 single-keyword `files` or `search`
  queries on its nouns; IF they yield none THEN ask the user for one identifier, path or error string.
- IF a hit lies in generated code THEN follow it to the generator input (proto, OpenAPI, sqlc) and
  report both locations.
- IF a hit lies in vendored or third-party code THEN report it and do not expand it.
- IF a structural and a text query disagree THEN re-read the files; trust the AST result except for
  Go selector calls (code-intel §4).
- IF impact touches a public API or DTO, a schema, auth, money, concurrency or 3 or more packages THEN
  mark the change T2 (contract §2) and name `arc:plan` for a brief before `arc:code`.
- IF the user asks to perform the change THEN finish the report and hand off to `arc:code`.

## Verification

| Claim | Evidence |
|---|---|
| Location | read `file:line` plus the query that found it |
| Caller or chain hop | `file:line` per hop plus the query per hop |
| "none" / "not found" | every query run, through code-intel §5 rung 5 |
| Impact | eight rows, each `query → hit count → files` or `none (query …)` |
| Count | `file_count` below `--max`, or the result labelled `sampled` |
| Map | each module row cites a file read; each direction violation cites the import `file:line` |
| Index operation | doctor JSON before and after |

## Failure Recovery

| Failure | Action | Retry budget | Stop condition |
|---|---|---|---|
| `arc-idx` not on PATH | report `command -v arc-idx` output; use code-intel §6 fallback | 0 | fallback evidence labelled |
| Doctor unhealthy or stale | `arc-idx index --quiet`, re-run doctor | 1 | still unhealthy → §6 fallback, report doctor JSON |
| `ctags` or `ast-grep` `MISSING` | ask permission to install; meanwhile use `search` and `files`, mark symbol and AST rows `BLOCKED` with reason | 0 without permission | permission refused → report gap |
| Daemon will not start or stale PID | `arc-idx daemon restart` | 1 | report the `log_file` path and last lines |
| Zero hits | code-intel §5 widening rungs 1–5 | 5 rungs | report `not found` with every query |
| More than 200 hits | narrow (Decision Rules) | 3 passes | sample and label `sampled` |
| `query parse:` error | escape per code-intel §2 | 2 | rewrite as a plain literal term |
| Text after `arc-idx ast: ast-grep:` | simplify the pattern (fewer metavariables, complete syntax) | 2 | use the text query; note the AST gap |
| Hits name files absent from the repository | check doctor `repo_root` and `index_dir`; fix with `--repo` or `<clone>/.ai-code-index/`; if another clone shares the `index_dir` basename, set `index_dir` in config; reindex | 1 | report the collision |
| `index` warns `hit max_files` | report partial coverage; narrow with profiles, or raise `max_files` on user approval | 0 | label results `partial index` |
| Structural and text results contradict | re-read the files (Decision Rules) | 1 | report both readings as unresolved |

## Completion Criteria

- [ ] Preflight line recorded: version, `repo_root`, health, repairs run.
- [ ] Mode, scope and depth stated.
- [ ] Every claim cites a read `file:line` and its query.
- [ ] Evidence table in code-intel §7 format; every "none" lists its queries.
- [ ] Impact mode: eight rows per changed item; rows 2–4 cross-checked.
- [ ] Trace mode: each chain reaches an entry point, or the frontier is listed at the depth limit.
- [ ] Map mode: module table, diagram, dependency-direction table, unresolved items.
- [ ] Dynamic items and coverage limits listed (fallback, sampled, truncated, unindexed paths).
- [ ] Files line reads `none` or lists only index files.
- [ ] Next names the handoff (`arc:code`, `arc:plan` or `arc:audit`) with the queries to re-run.
