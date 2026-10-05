---
name: arc:idx
version: 1.1.1
description: >
  First step of any local repo work: implement, debug, review, 搜代码, 找符号, or locate
  files. Runs ~/.local/bin/arc-idx (zoekt search, ctags symbol, ast-grep ast, files, stats,
  daemon, doctor) before guessing paths. Not rg/find/MCP as primary search. Not for editing code.
---
# arc:idx

## When to Use

First step of any local repository task: locate files, text, definitions, callers,
AST structure or language inventory before reading/editing or claiming absence.
This skill discovers context; use `arc:build`, `arc:fix` or `arc:audit` afterward.

## Intent Router

| Need | Command / route |
|---|---|
| Each repository session | Identity and session preflight below |
| File name / existence | `arc-idx files <pattern> --format json` |
| Text / regex / callers | `arc-idx search '<zoekt query>' --format json` |
| Definition | `arc-idx symbol <name> --exact --format json` |
| Code structure | `arc-idx ast --lang <language> '<pattern>' --format json` |
| Language / size inventory | `arc-idx stats --format json` |
| Missing, suspicious or stale results | `arc-idx doctor --format json` |
| Persistent task cards | `arc:sdlc`; include precise `<Context-Queries>` |

## Red Lines

- Use this compiled `arc-idx` engine for all repository discovery. No primary search
  with rg/grep/git-grep/fd/find, recursive listings, shell/Python/Node tree walks,
  IDE/runtime search, MCP, legacy index scripts, or another index engine.
- Never invoke ctags, ast-grep, zoekt or MCP directly as a fallback.
- No guessed paths/symbols, stale-index trust, raw tag dumps or terminal-color logs
  in context. Discovery must precede Read/edit and claims that a file is missing.
- Missing binary or wrong identity: stop and report path/output; never silently switch.
- Only use `rg`/`fd` when this engine is confirmed unavailable by doctor, the target
  is outside every repository, or corruption remains after index repair. State why.
  A missing/stale index alone requires rebuilding first, not bypassing the engine.

## Identity and Session Preflight

```bash
command -v arc-idx
arc-idx version
arc-idx doctor --format json
arc-idx daemon status
```

The command must resolve to `~/.local/bin/arc-idx` and print exactly:

```text
arc-idx 1.0.0 — local AI context engine (zoekt + ctags + ast-grep)
```

Use the command name directly; only when PATH cannot find it, use the full path and
report that fallback. If doctor is unhealthy/stale or the index is missing, run
`arc-idx index --quiet`. If daemon is not alive, run `arc-idx daemon start`.
An already-running start exits 1 with `already running`; that state is normal.

## Queries

```bash
arc-idx files handler --format json
arc-idx search 'f:\.go$ handler' --context 3 --max 20 --format ai-md
arc-idx search 'lang:go case:yes UserService' --format json
arc-idx search 'sym:UserService' --format json
arc-idx symbol UserService --kind struct --exact --format json
arc-idx ast --lang go 'func ($_ *Service) $NAME($$$) error' --format json
arc-idx stats --format json
```

Use `--format json|ai-md` for search and JSON for files/symbol/AST/stats/doctor.
Use `--profile` only with names reported for this repository; `all` is implicit.
Use zoekt syntax for search (`f:`, `lang:`, `sym:`, `case:yes`, `-exclude`);
`files` is filename matching, not a regex-alternation search. Independent queries
may run together. Read bounded hits, then expand only where evidence requires it.

The engine finds the nearest `.ai-code-index/config.json` above the working path.
Use `--repo <dir>` or enter a child clone; an umbrella directory need not have `.git`.
Run Git state/diff commands with `git -C <clone>` for each actual repository.
If a search reports `engine=rg-fallback`, diagnose/rebuild the index before continuing;
only the explicit fallback conditions above allow relying on it.

## Index Lifecycle

Config and profiles live in `.ai-code-index/config.json`; generated tags live beside
it, and Zoekt shards default to `~/.cache/ai-code-index/zoekt/<repo>/` (overridable).
The fsnotify daemon uses `.ai-code-index/daemon.pid` and a log, an idle TTL of 30 minutes,
a 512 MiB Go memory limit and 800 ms debounce. Let it refresh incrementally; do not
loop manual rebuilds. Use `daemon stop` after a large refactor is fully finished.
Use `daemon status`/`doctor` to diagnose stale PID files or missing updates.

In this Skills repository, engine source is `tools/arc-idx/`; rebuild only when
changing the engine: `go build -o ~/.local/bin/arc-idx ./tools/arc-idx`.

Report relevant paths/symbols, the profile and evidence commands, health/daemon
changes, and any coverage limitation or fallback reason. Task cards must record
exact discovery queries for the executor to run before coding.
