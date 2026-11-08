# arc.db — repository-local resource, state and change protocol

> **Unified CLI in two stages.** The new `arc` 0.1.0 entrypoint already routes
> code-index commands to `arc-idx` and `arc verify ...` to `arc-verify`.
> It is a **compatibility bridge**, not a merged database. The `arc file/task/change/
> update/db/sync` resource commands and the shared `arc.db` state engine below
> are **proposals**, not operational features. Check `arc capabilities --json`
> before use; never infer a passed gate, migrated store or supported command.

## 1. Purpose and authority

`arc.db` is the **filename** of the planned project-local state store:
`<repo>/.arc/arc.db`. The public CLI is **`arc`**; the current `arc-idx`
and `arc-verify` executables are transitional backends. The end-state is one
CLI, one versioned project-local database and separate internal engines for code
indexing, workflow state and business verification. Keep `arc:explore` read-only;
mutations belong to the authorized work owner and plugin runtime.

- **Git/worktree is authoritative for source bytes and committed revisions.**
  File content, AST and symbol results must be rederived from the actual tree;
  a database row never overrides the bytes on disk.
- **After explicit migration, arc.db owns resource IDs, task state, event history,
  the business verification graph, execution rounds and evidence references.**
  Until cutover, existing task Markdown, `arc-policy` receipts and the
  `arc-verify` graph ledger remain authoritative; no automatic dual-write.
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

## 3. One public CLI: `arc`

The **already implemented bridge** routes current commands unchanged:

```bash
arc capabilities --json
arc doctor --format json
arc search 'lang:go Payment' --format json
arc verify gate --graph GRAPH-ID
arc verify trace --run RUN-ID
```

These forms require the corresponding legacy engine on `PATH`; `arc verify`
currently retains the old `.arc/verify/data.db` default. No database migration
has occurred. `arc-policy task` is still a separate policy gate.

**Proposed resource interface** (NOT implemented):

Grammar: `arc <resource> <action> [id] [typed flags]`. Use stable JSON
envelopes, typed IDs and distinct errors for stale data and invalid transitions.

```bash
# proposed; capability must advertise arc_db_resources=true before use
arc db status --format json
arc file show internal/payment/service.go --format json
arc task create --title "修复支付并发" --format json
arc task transition TASK-001 --to running --expected-version 2
arc change collect --task TASK-001 --worktree . --source hook
arc change impact CHANGE-001 --format json
arc update --changed-only --task TASK-001 --source hook
arc run record --task TASK-001 --revision <git-sha>
arc task verify TASK-001 --revision <git-sha>
arc event list --task TASK-001 --format json
arc sync reconcile --source git --format json
```

- `file update` records a verified filesystem observation; it never fabricates
  source bytes or replaces an editor. `update` is a bounded reconcile
  dispatcher, not a magic "task done" command.
- `task update` changes allowed fields; `task transition` checks state guards.
  `change verify` checks change-linked evidence. Actual business graph checks
  remain under `arc verify` and its existing ledger until explicit migration.
- `arc capabilities --json` currently advertises `arc_db_resources=false`
  and `unified_database=false`. After migration, these features require a
  schema-version handshake, not merely a new executable name.

## 4. State machines and invariants

- **Task:** `draft → ready → running → verifying → done`. `running` may become
  `blocked`, `blocked → ready|running`, `verifying → running` on failure, and
  `done → reopened` only by an explicit permitted transition. An edit alone
  must **never** set `done`.
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

Suggested logical schema (not a migration yet):

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
- Prefer a **single local writer/daemon** with CLI/plugin calls over local
  IPC; allow many readers and short bounded transactions. Do not impose
  whole-project serial execution or long transaction locks.
- The existing verification engine uses local Turso. Choose and **test** one
  compatible storage driver for the merged schema (Turso, SQLite or another
  engine) before migration; do not presume SQLite WAL or Turso multiprocess
  semantics. A single-writer daemon or proven cross-process locking is required.
  Verification's frozen graph, RAW evidence, gate rules and identity checks
  must survive cutover without semantic changes.
- Ignore `.arc/arc.db` and its journal/WAL files in Git; for collaboration
  define replayable exported events or migrations, not Git merges of a
  binary database. No credentials or full private source by default.

## 6. Plugin/Hook reconciliation

The existing `arc-policy` PreToolUse/PostToolUse/Stop hooks are the integration
points, **not** proof that `arc update` is implemented.

1. **Pre-tool:** observe current task, authorization, worktree and baseline.
   Keep current `arc-policy` guard behavior; do not require a nonexistent DB.
2. **Post-tool:** confirm actual changed paths and content hashes; submit one
   idempotent change event or mark it pending. Debounce edits into a bounded
   ChangeSet; do not reindex the entire repository on each write.
3. **Reconcile:** compare against Git and filesystem truth; update only
   affected file/symbol/dependency records and impacted task/contract edges.
   Mark derived docs and stale evidence as needing review rather than
   blindly rewriting authored decisions.
4. **Stop/commit gate:** verify events are reconciled and evidence refers
   to the exact current revision. Do not auto-close tasks. Keep existing
   `arc-policy` and `arc verify` completion gates authoritative.
5. **Recovery:** replay pending events after restart, rescan Git changes
   when hooks were skipped, and report `sync_pending` / `conflict` / `stale`
   explicitly. A missing event never means "no change".

## 7. Migration and acceptance

The current verification engine uses `.arc/verify/data.db` (local Turso).
`arc-policy` retains independent task receipts; `aitask` owns cross-agent
orchestration. Target migration is **one** `.arc/arc.db` that also includes
business graph, approved rules, tasks/attempts/rounds, RAW evidence references
and revision-bound gates. Do not rename files, silently import or dual-write.
First back up and fingerprint the legacy data, map stable graph/rule/run IDs,
migrate under schema versioning, compare full gate results against the old
ledger, then switch `arc verify` and the policy gate together. Keep rollback
and an explicit compatibility-read period. One owner per state domain.

Delivery stages: (1) public `arc` command bridge (implemented);
(2) unified schema + verified graph-ledger migration and rollback;
(3) file/task/change/event state engine, plugin hooks and incremental replay;
(4) design/contract/invariant impact graph and production hardening.

Required acceptance scenarios: repeated PostToolUse event; two agents editing
the same task from different worktrees; optimistic version conflict; crash
between state commit and index update; offline/restart replay; change without
a plugin event; branch checkout invalidating index/evidence; unauthorized
transition; failed test never producing `done`; schema rollback. Report what
was actually tested. Until those pass, treat this document as design guidance,
not an operational guarantee.
