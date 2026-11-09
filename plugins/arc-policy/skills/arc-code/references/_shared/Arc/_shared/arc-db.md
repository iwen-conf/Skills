# arc.db — repository-local resource, state and change protocol

> **Implementation status (arc 0.4.2).** `arc` routes code intelligence and
> verification to its private engines in `libexec/arc`, and implements
> versioned `arc.db` operations for `db`, `task`, `file`, `change`, `update`
> and `policy state`. One physical project-local Turso DB is created with `arc db init`,
> or the existing graph ledger is copied with `arc db migrate --confirm-offline`
> only after explicit offline confirmation, backup and exact gate parity.
> This is **not** yet one compiled engine, and native per-file index updates and
> some advanced resources remain unimplemented. Check
> `arc capabilities --json`; do not infer a PASS from a data migration.

## 1. Purpose and authority

`arc.db` is the **filename** of the opt-in project-local state store:
`<repo>/.arc/arc.db`. The public CLI is **`arc`**; its code-index and verify
engines are private transitional executables, never called by name. The end-state is one
CLI, one versioned project-local database and separate internal engines for code
indexing, workflow state and business verification. Keep `arc-explore` read-only;
mutations belong to the authorized work owner and plugin runtime.

- **Git/worktree is authoritative for source bytes and committed revisions.**
  File content, AST and symbol results must be rederived from the actual tree;
  a database row never overrides the bytes on disk.
- **After explicit init/migration**, `arc.db` holds source resources and
  verifier graphs, rounds and RAW metadata. `arc policy state migrate
  --confirm-offline` additionally imports Policy active/history state and
  immutable receipts into Turso. Until this separate cutover, Policy JSON
  remains authoritative; after it, the Go DB is the only Policy state source.
- **Completion lock protocol:** `arc task complete` validates the existing
  `arc-policy` PASS, acquires its `task.lock`, compares the live receipt in
  `arc.db` (or the legacy file before Policy cutover),
  and commits under the project DB lock; never call back into the policy CLI
  while holding the locks. Until compatible Windows byte-range locking exists,
  Windows completion is blocked rather than accepted without coordination.
- **Verification evidence is immutable and revision-scoped.** A successful
  index update is not a passed test, and a passed test is not task completion.
- Avoid a second project tracker. The planned task records should replace
  a designated existing task source, not silently mirror it as another SSOT.

## 2. Typed resources and links

| Area | Resources | State semantics |
|---|---|---|
| Identity | `project`, `repo`, `workspace`, `worktree` | Stable IDs and Git/worktree scope |
| Code | `file`, `module`, `symbol`, `dependency`, `index` | Derived facts, content hash and freshness |
| Intent | `requirement`, `design`, `contract`, `schema`, `decision`, `invariant` | Versioned authored facts / constraints |
| Delivery | `plan`, `task`, `change`, `run`, `artifact` | Guarded transitions and ownership |
| Quality | `graph`, `flow`, `rule`, `test`, `check`, `evidence`, `risk` | Business model, graph gate, evidence and verdict |
| Actors | `agent`, `session`, `context` | Actor, scope, provenance; context may be derived |
| Control | `event`, `policy`, `snapshot`, `sync` | Append-only events, checks and replay cursors |

Resources are not automatically tables or state machines. Use typed tables for
durable business state and a narrow typed relation table (`implements`,
`depends_on`, `touches`, `verified_by`, `supersedes`) with enforced endpoint kinds.
`file`/`symbol`/`dependency` are rebuildable observations; `task`/`change`
are workflow aggregates. Do not turn every code symbol into a mutable task.

A `change` connects requirement/plan/task, repository/worktree/base revision,
affected files and symbols, execution run, tests, evidence and decisions.
Avoid relying on ambiguous file paths alone: use repository and worktree IDs,
canonical relative path, Git identity and content hash.

## 3. Implemented CLI (0.4.0) vs future features

```bash
arc capabilities --json
arc doctor --format json
arc search 'lang:go Payment' --format json
arc db status --repo .
arc db init --repo .                       # new project only; no legacy DB
arc db migrate --repo . --confirm-offline   # existing legacy verify DB
arc task create --title "修复支付并发"
arc task ensure --id POLICY-TASK-ID --title "修复支付并发 (arc-code)"
arc task show --id TASK-ID
arc task transition --id TASK-ID --to ready --expected-version 1
arc task complete --id POLICY-TASK-ID --expected-version N
arc policy state status --repo .
arc policy state migrate --repo . --confirm-offline
arc file update --path internal/payment/service.go
arc file show --path internal/payment/service.go
arc event list --resource task --limit 20
arc index flush --repo .                    # real full rebuild + conditional outbox ACK
arc change collect --task TASK-ID
arc change show --id CHANGE-ID
arc update --changed-only
arc sync reconcile                         # recover missed committed edits
arc verify gate --graph GRAPH-ID
```

Flags precede positional IDs in v1. All resource writes require an active
`.arc/arc.json` manifest and `.arc/arc.db`; no silent migration. `arc verify`
uses `arc.db` by default **only after activation**; explicit `--db` still wins.
The plugin graph gate follows the marker when policy has no explicit DB override.

`arc capabilities --json` exposes `arc_db_resources=true`,
`arc_db_schema_version=1`, and `unified_database=true` **only in an active
project**. The code-index and verifier binaries remain separate dependencies.
`arc-policy task verify` remains the completion authorization gate.

**Not yet executable:** native `arc task verify`, `arc change verify/impact`
or `arc design/contract/schema`.
`arc task complete` requires an existing **current PASS** from `arc-policy`;
`AUTHORIZED` never counts as passing independent verification.

`arc update` collects an idempotent worktree ChangeSet. `arc sync reconcile`
also replays commits missed by hooks, maintaining an atomic `arc_meta.sync_head`
cursor. File/change events enter the index outbox; task events remain journal-only.
`arc index flush` batches a **full** Zoekt/ctags rebuild, checks index health
and stable source fingerprints, then atomically acknowledges only the original
batch. Failed/stale builds retain pending events. This is not yet incremental
per-file index writing. The index daemon retains its own watcher.

## 4. State machines and invariants

- **Task:** `draft → ready → running → verifying → done`, with guarded
  blocked/reopened branches. The current implementation enforces CAS and allows `done` only when the
  matching Arc Policy task has a live PASS receipt, the code-index snapshot
  matches that source and the relevant outbox is drained. A completion event
  stores the receipt fingerprint and independent check results. Edits alone
  never complete tasks; `arc task show` labels historical source freshness.
- **Change:** `discovered → collected → reconciled → verified|rejected`.
  A newer source hash creates a new revision and invalidates prior verification;
  do not silently rewrite a verified change record.
- **Index job:** `dirty → indexing → ready|failed`. `ready` is valid only for
  the source fingerprint used to build it.
- **Run:** `queued → running → passed|failed|cancelled`. Keep runner,
  command, exit code, environment, code revision and artifact fingerprints.
- **Evidence:** append-only; mark its claim `stale` when affected source,
  contract, test or environment fingerprints change. Do not rewrite `PASS`.

Every guarded transition requires resource ID, actor, previous state/version,
expected version, event type, timestamp and causation/idempotency key. Reject
impossible edges and ownership/scope violations **before** committing a new state.

## 5. Persistence and concurrency

Current database plus future extensions (only `arc_*` and existing verifier tables ship):

```text
resources / typed resource tables       identity, version, current state
relations                               typed edges and source revisions
changes + change_files                  one logical change and touched paths
business_graphs + nodes + edges         accepted business graph and rules
verification_tasks + rounds + attempts  graph obligations and execution state
runs + evidence                         immutable execution and receipts
events                                  append-only transitions and facts
outbox                                  durable pending derived updates
sync_cursors + snapshots                retry and replay checkpoints
```

- Transactionally write new resource version + `events` + `outbox`.
  Use optimistic compare-and-swap (`WHERE id=? AND version=?`), uniqueness
  on event id/idempotency key, foreign keys and migration versioning.
- Deliver outbox events **at least once**; consumers must be idempotent. A
  failed downstream index or plugin does not roll back an already committed
  source edit or create a fictitious `PASS`.
- The v1 CLI serializes its own local DB operations with a repository lock.
  Legacy direct `arc-verify` invocations do not honor it. A single-owner
  daemon or fully tested cross-process coordination is still required for
  robust multi-agent production writes; do not claim it exists today.
- v1 uses the existing verifier's pinned local `tursogo` 0.8.3 engine.
  The offline migrator copies the legacy file only with no live sidecars,
  saves a SHA-256-verified backup, installs namespaced resource tables and
  compares each old/new graph gate's complete stdout and exit code.
  A failed check **does not activate** the new DB. Direct legacy writes and
  multi-process Turso concurrency remain unsupported cutover risks.
- Ignore `.arc/arc.db` and its journal/WAL files in Git; for collaboration
  define replayable exported events or migrations, not Git merges of a
  binary database. No credentials or full private source by default.

## 6. Plugin/Hook reconciliation

The existing `arc-policy` PostToolUse hook binds the active policy task
with idempotent `arc task ensure`, then runs `arc sync reconcile --task <id>`
for repositories with an active manifest. When the policy has no active task,
reconciliation records changes without an owner. Explicitly read-only shell
calls are skipped; missing CLI or failed writes are reported.

1. **Pre-tool:** observe current task, authorization, worktree and baseline.
   Keep current `arc-policy` guard behavior; do not require a nonexistent DB.
2. **Post-tool:** the implemented hook imports a stable `arc-policy` task identity,
   collects a bounded Git worktree ChangeSet and records file-hash events with
   idempotent fingerprinting. Each mutating call may repeat a Git scan; optimized
   event batching remains future work.
3. **Reconcile:** file hashes and task-linked changes are recorded. The
   `arc index flush` consumer confirms the existing full indexer output before
   acknowledging source events; native per-file incremental indexing,
   dependency invalidation and document projection remain future work.
4. **Stop/delivery gate:** completion claims still require revision-scoped
   verification evidence. Ordinary `git commit` and `git push` never require
   a per-commit verify receipt. Do not auto-close tasks; keep existing
   `arc-policy` and `arc verify` completion gates authoritative.
5. **Recovery:** replay pending events after restart, rescan Git changes
   when hooks were skipped, and report `sync_pending` / `conflict` / `stale`
   explicitly. A missing event never means "no change".

## 7. Migration and acceptance

The current verifier's legacy database is `.arc/verify/data.db`. To migrate,
stop all direct verifier processes, ensure no uncheckpointed sidecars remain,
then run `arc db migrate --confirm-offline`. The original file is untouched;
a content-addressed copy is retained under `.arc/backups/`. Each old/new
graph gate must match before `.arc/arc.json` activates `arc.db`. A failure
leaves an **inactive** target to inspect/recover explicitly; it never
automatically overwrites legacy data. Migration cannot prove that previously
unverified behavior was correct.

Delivery stages: (1) public CLI (implemented); (2) opt-in shared Turso store,
versioned tasks, file/change events and PostToolUse (code implemented,
integration pending); (3) graph-linked task acceptance, outbox/index replay,
multi-agent concurrency hardening, historical migration replays and rollback;
(4) full design/contract/invariant impact graph.

Required acceptance scenarios: repeated PostToolUse event; two agents editing
the same task from different worktrees; optimistic version conflict; crash
between state commit and index update; offline/restart replay; change without
a plugin event; branch checkout invalidating index/evidence; unauthorized
transition; failed test never producing `done`; schema rollback. Report what
was actually tested. Until those pass, treat this document as design guidance,
not an operational guarantee.
