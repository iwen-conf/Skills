# Refactor and Migration Protocol

Refactor mode runs this protocol for renames, moves, extractions, splits, merges, library or
framework migrations, and shape changes of APIs, DTOs, schemas, events and config keys. A
refactor preserves observable behavior; an intended behavior change is a separate step with its
own test.

## 1. Behavior baseline (before the first edit)

- Run the tests that cover the code to be touched. Save command and result to
  `.arc/artifacts/<task-slug>/logs/baseline-tests.txt`.
- IF the baseline is red THEN report the failing tests and refactor only areas whose tests are
  green; a red area gets a separate repair step with its own fail → pass evidence first.
- Add characterization tests for every touched behavior that lacks coverage: return values,
  error paths, edge values, serialized shape, side effects (rows written, events sent). They
  must pass on the old code. Golden files are allowed for serialized output.
- IF a touched behavior cannot be tested (no test environment) THEN mark that part
  `BLOCKED(<reason>)` and leave it unmoved.

## 2. Impact map

- Fill all 8 rows of the [code-intel](../../_shared/code-intel.md) §4 checklist for every symbol
  that is moved, renamed, re-signatured or changes meaning. No row is skipped.
- IF a symbol has >30 call sites THEN plan a staged migration behind an adapter or deprecation
  wrapper (§6) instead of one diff.
- Reflection, string-keyed registries, DI containers, templates, SQL strings and JSON keys are
  listed as `dynamic — confirm at runtime` and covered by a test.

## 3. Step plan

- Write numbered steps. Each step is one move that compiles alone, for example: add new →
  switch callers in package A → switch package B → delete old.
- Never combine a move with a behavior change or with formatting of untouched code.
- After each step run build or typecheck plus the affected-package tests, and record the result.
  IF red THEN fix within the step or undo own step before the next; 2 attempts per step, then
  stop and report.
- Mechanical edits use language tooling (gopls rename, `gofmt -r`, IDE refactor, codemods with a
  dry run) before regex. A text rewrite spanning >1 directory needs a checkpoint: ask the user
  for a checkpoint commit, or edit one directory at a time and run the step checks between
  directories.
- The tree builds after every step.

## 4. Expand – migrate – contract

Applies to anything another component reads: API route or field, DTO, table or column,
event or message schema, config key, cache key, exported function of a shared package.

| Phase | Do | Exit evidence |
|---|---|---|
| Expand | Add the new shape beside the old: nullable column, optional field, new endpoint or version, new key with fallback read. Old consumers keep working. | old and new tests green |
| Migrate | Move writers, then readers; backfill data (§5); dual-write or read-new-fallback-old during the window. | in-repo search shows no old-path use except the shim; backfill counts match |
| Contract | Remove the old shape, shim and fallback in a separate change after every consumer, including external ones, has moved. | zero old-path usage (search, plus logs or metrics for external consumers) |

- Single-diff exception: every consumer is in this repository, changes in this diff, deploys
  together, and no persisted data or external client depends on the old shape.
- A contract phase that drops columns, tables or data is destructive and needs explicit approval
  (contract §6).

## 5. Data migrations

- One new migration per change with up and down; never edit an applied migration. IF the change
  cannot be reversed without data loss THEN write a forward-fix plan and get approval.
- Keep schema changes and data backfills in separate migrations or jobs.
- Backfills are:
  - **batched**: bounded rows per transaction (default 1,000), keyset pagination on the primary
    key, never `OFFSET`;
  - **idempotent**: safe to rerun (`WHERE new_col IS NULL`, upsert, deterministic transforms);
  - **resumable**: progress checkpoint stored outside the batch;
  - **throttled** on production-size tables.
- PostgreSQL lock rules: `CREATE INDEX CONCURRENTLY` outside a transaction; add `NOT NULL` checks
  and foreign keys as `NOT VALID`, then `VALIDATE CONSTRAINT`; set `lock_timeout` for DDL; a
  column type change or volatile default that rewrites a large table goes through §4 instead.
- Verify with row counts plus checksums or sampled comparisons before and after; save them to the
  task logs.
- Run migrations only in the project's test environment (contract §6); production execution
  belongs to the user or the deploy pipeline.
- T2 migrations: hand rehearsal, rollback rehearsal and lock-impact qualification to `arc:verify`.

## 6. Deprecating old paths

- Mark the old symbol: Go `// Deprecated: use X.`; TypeScript `/** @deprecated use X */`; APIs get
  a deprecation note and removal condition in the Markdown contract
  ([api-contract.md](../../_shared/api-contract.md)).
- The old path delegates to the new implementation; no duplicated logic.
- When external consumers exist, log or count calls on the old path so the contract phase has
  usage evidence.
- Record the removal as a follow-up task for `arc:plan` with its removal condition; no shim stays
  without one.
- IF the user forbids compatibility and the product is unshipped THEN skip the shim and migrate in
  one diff.

## 7. Rollback plan

Write before the first step that affects persisted data or a deployed contract:

- per step: how to revert code (revert commit, flag off), data (down migration, backfill audit
  columns) and config;
- the point of no return (usually the contract phase) and the approval it needs;
- the signals that trigger rollback: error rate, failing check, backfill count mismatch.

## 8. Surface checklist

| Surface | Check |
|---|---|
| Server | handlers, DTOs, validation, services, repositories, DI wiring, cron jobs and consumers |
| Client | API modules, generated clients, types, forms, state caches, routes |
| Tests | unit, integration, fixtures, golden files, mocks regenerated from source |
| Docs | Markdown API contract, `docs/` references to the moved path, deprecation notes |
| Migrations | up and down, backfill job, indexes, seed data |
| Config | env samples, deploy manifests, feature flags, config keys |
| Generated code | regenerated from proto, OpenAPI or sqlc input, never hand-edited |

## 9. Done

- [ ] Baseline tests and characterization tests green with unchanged assertions.
- [ ] A search for the old pattern returns zero hits, or each remaining hit is listed with its
      follow-up task; no half-migrated call sites.
- [ ] Every surface row checked.
- [ ] Rollback plan written for persisted-data or contract changes.
- [ ] `gate.sh check` shows no `FAIL`.
