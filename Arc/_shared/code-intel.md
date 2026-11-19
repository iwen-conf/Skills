# Code Intelligence Protocol (`arc idx`)

Unified local code intelligence via the compiled Go binary `arc` (`tools/arc` → `tools/arc-idx`: Zoekt + Universal Ctags + ast-grep + AST Skeleton + Cross-Layer Impact Scanner).

## 1. Preflight (Once per Session)

```bash
arc idx doctor --format json
```
- If `stale_index` is true or shards are missing, run `arc idx index --quiet` once (uses Git blob hashes in `manifest.json` to skip unchanged files in milliseconds; `--force` forces full rebuild).
- Never call `ctags`, `zoekt`, or `ast-grep` directly when `arc` is healthy.

## 2. Core High-Signal Commands

| Command | Purpose | When to use |
|---|---|---|
| `arc idx skeleton <paths...> [--all] [--format ai-md\|json]` | Public types, interfaces, structs, functions, SQL tables & line ranges **without bodies** (~80% token savings) | First look at any module/directory before reading full files |
| `arc idx impact <entity> [--prod-only] [--format ai-md\|json]` | Cross-layer reverse coupling scan: SQL tables/FKs/status/amount cols, ORM models, HTTP routes, async MQ topics & candidate `world_flags` | Before schema/API edits, refactoring, or authoring `*.story.yaml` |
| `arc idx symbol <name> [--kind K] [--lang L] [--exact]` | Exact symbol definitions & signatures | Jump to definitions |
| `arc idx search <query> [--prod-only] [--profile P] [--context 3]` | Authority-ranked search (`prod > contract > test > doc`) via Zoekt | Find callers, literals, error strings, SQL fragments |
| `arc idx ast --lang <L> '<pattern>' [paths...]` | Structural AST query via ast-grep | Find risky shapes or exact call patterns |
| `arc idx files [pattern] [--profile P]` / `arc idx stats` | Candidate file inventory & LOC breakdown | Architecture mapping & hotspot detection |

## 3. Query Ladder (Stop at First Sufficient Rung)

1. `arc idx skeleton <dir>` — understand package surface without flooding context.
2. `arc idx symbol <name> --exact` — jump directly to definition.
3. `arc idx impact <entity> --prod-only` — map all cross-layer DB/ORM/route/topic couplings.
4. `arc idx search '<query>' --prod-only --context 3` — trace production callers (docs/tests deprioritized).
5. `arc idx ast --lang <L> '<pattern>'` — structural cross-check.
6. Host fallback (`rg` / `grep`) ONLY when `arc` is unavailable after 1 repair; label evidence `fallback:<tool>`.

## 4. Eight-Row Impact Checklist (For Signature / Schema / Contract Changes)

1. Definition (`symbol`)
2. Production callers (`search --prod-only` + `ast`)
3. Constructors / writers / serializers
4. Readers / deserializers / JSON tags
5. API contract & client consumers
6. Database schema / migrations / foreign keys (`impact`)
7. Async events / queues / background jobs (`impact`)
8. Tests & storyline `world_flags`

## 5. Widening & Evidence Rules

- Zero hits: relax exact → case-insensitive → stem/substring → `files` → report `not found (queries: …)`. Never invent paths, columns or symbols.
- Every claim cites `file:line` plus the exact `arc idx` command executed.
- Dynamic dispatch / reflection items are labeled `dynamic — confirm at runtime`.
