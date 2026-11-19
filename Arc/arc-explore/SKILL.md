---
name: arc-explore
version: 3.0.0
description: >-
  Read-only code intelligence for a local repository: locate definitions (代码在哪, 搜代码),
  extract symbol skeletons, list callers (谁调用), trace call chains (调用链), scan cross-layer
  SQL/ORM/route/topic impact (影响面), and map architecture (架构地图). Maintains the arc Zoekt
  index (arc-idx: arc idx doctor, index, daemon). Not for edits (use arc-code), review (use arc-audit),
  or tests (use arc-verify).
---

# arc-explore

Locate code, extract signature skeletons, trace callers and map cross-layer impact with reproducible `arc idx` evidence; edit nothing except index state.

## Non-trigger

- Edit/fix/refactor code → `arc-code`; review/AppSec → `arc-audit`; tests/storylines → `arc-verify`; PRD/DAG/closure → `arc-plan`.

## Goal

Cite `file:line` and the exact `arc idx` query for every claim; in impact mode output SQL/ORM/route/topic couplings plus the 8-row checklist.

## Inputs

- `question` (required); `scope` (default repo root); `seeds` (symbol/table/route/error); `depth` (default 3); `mode` (`locate|trace|impact|map|index-ops`).

## Intent Router

| Condition | Mode | Read / Run |
|---|---|---|
| Change verbs (改/删/重命名/rename/remove) or 影响面 / entity coupling | `impact` | [code-intel.md](../_shared/code-intel.md) §4; `arc idx impact <entity> --prod-only` |
| Callers, call chain or data flow (谁调用, 调用链) | `trace` | `arc idx symbol` → `arc idx search --prod-only` → `arc idx ast` |
| Locate files, types, signatures or literals (代码在哪, 搜代码) | `locate` | `arc idx skeleton <dir>` → `arc idx symbol` → `arc idx search --prod-only` |
| Module layers or architecture map (架构地图) | `map` | [architecture-map.md](references/architecture-map.md) |
| Index health, stale shards, daemon or `.arc/arc.db` read | `index-ops` | `arc idx doctor --format json`; [arc-db.md](../_shared/arc-db.md) |

## Workflow

1. Run `arc idx doctor --format json`. If stale or missing shards, run `arc idx index --quiet` once (git-blob incremental).
2. Start with `arc idx skeleton <dir>` to read signatures/line ranges without dumping function bodies.
3. Execute the matched Intent Router mode:
   - `locate`: `skeleton` → `symbol --exact` → `search --prod-only --context 3` → `ast`.
   - `trace`: definition → production callers per hop up to `depth` or entry point; list unexpanded frontier nodes.
   - `impact`: run `arc idx impact <entity> --prod-only --format json` to extract SQL FKs/status/amount columns, ORM structs, routes, async topics and candidate `world_flags`, then close all 8 rows of [code-intel.md](../_shared/code-intel.md) §4.
   - `map`: `arc idx stats` + `arc idx files` + [architecture-map.md](references/architecture-map.md).
4. Read the enclosing function/type at every hit; mark unresolved dynamic dispatch `dynamic — confirm at runtime`.
5. Report per [contract.md](../_shared/contract.md) §9 with queries, `file:line`, coverage limits and next owner (`arc-plan` / `arc-code` / `arc-verify`).

## Tool Strategy

- **Preferred**: `arc idx skeleton`, `arc idx impact`, `arc idx symbol`, `arc idx search --prod-only`, `arc idx ast`, `arc idx files`, `arc idx stats`.
- **Forbidden**: guessing paths/symbols, calling `zoekt`/`ctags`/`ast-grep` directly, dumping full files when `skeleton` suffices, editing source/docs.
- **Fallback**: if `arc` remains unhealthy after 1 rebuild, use host search labeled `fallback:<tool>`.

## Decision Rules

- IF the prompt contains a change verb → select `impact` mode even if it also asks where code lives.
- IF hits > 200 → narrow by `--prod-only`, `--profile`, directory or `ast`; label `sampled` only after 3 narrowing passes.
- IF impact touches public API, DB schema, auth, money, concurrency or ≥3 packages → mark T2 and route to `arc-plan` before `arc-code`.

## Verification

| Claim | Evidence |
|---|---|
| Location / hop | Read `file:line` + exact `arc idx` command |
| Cross-layer impact | `arc idx impact` JSON/table + 8-row checklist |
| "not found" | Full widening ladder queries with zero hits |

## Failure Recovery

| Failure | Action | Stop |
|---|---|---|
| Index stale/missing | `arc idx index --quiet` (1 retry) | Still unhealthy → `fallback:<tool>` + doctor JSON |
| Zero hits | Widen exact → substring → `files` | Report `not found` with all queries |
| Daemon stuck | `arc idx daemon restart` (1 retry) | Report log tail |

## Completion Criteria

- [ ] `arc idx doctor` preflight line recorded; mode and scope stated.
- [ ] Every claim cites `file:line` and its reproducible `arc idx` query.
- [ ] Impact mode includes `arc idx impact` couplings and 8-row checklist; zero source edits made.
