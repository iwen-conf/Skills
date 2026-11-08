# arc.db — repository-local resource, state and change protocol

> **Design contract, not a shipped feature.** Current `arc-idx` (1.0.0) only provides
> `index`, `search`, `symbol`, `ast`, `files`, `stats`, `doctor`, `watch` and `daemon`.
> All new CLI forms below are **proposed interfaces**. Never invoke them, assert
> that `arc.db` exists, or skip existing Arc gates until a tested implementation and
> an explicit feature/capability handshake are available.

## 1. Purpose and authority

`arc.db` is the **filename** of the planned project-local state store:
`<repo>/.arc/arc.db`. It is not a new Arc skill or a replacement for the
`arc-idx` executable. Keep `arc:explore` read-only; state mutations belong to
the already authorized work owner and plugin runtime.

- **Git/worktree is authoritative for source bytes and committed revisions.**
  File content, AST and symbol results must be rederived from the actual tree;
  a database row never overrides the bytes on disk.
- **arc.db becomes authoritative for resource IDs, relationships, accepted
  transitions and event history only after explicit migration and cutover.**
  Before cutover, existing task Markdown, `arc-policy` receipts, and the
  `arc-verify` graph ledger retain their current authority.
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
| Quality | `test`, `check`, `evidence`, `risk` | Evidence/claim binding, expiry and verdict |
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

## 3. Proposed CLI surface (NOT currently executable)

Grammar: `arc-idx <resource> <action> [id] [typed flags]`.
Use stable JSON result envelopes, typed IDs, explicit `--repo`, and distinct
exit codes for invalid transition, version conflict, not found and stale data.

```bash
# proposed; do not run against arc-idx 1.0.0
arc-idx db status --format json
arc-idx file show internal/payment/service.go --format json
arc-idx task create --title "修复支付并发" --format json
arc-idx task transition TASK-001 --to running --expected-version 2
arc-idx change collect --task TASK-001 --worktree . --source hook
arc-idx change impact CHANGE-001 --format json
arc-idx update --changed-only --task TASK-001 --source hook
arc-idx run record --task TASK-001 --revision <git-sha>
arc-idx task verify TASK-001 --revision <git-sha>
arc-idx event list --task TASK-001 --format json
arc-idx sync reconcile --source git --format json
```

- `file update` records a verified filesystem observation; it never fabricates
  source bytes or replaces an editor. `update` is a bounded reconcile
  dispatcher, not a magic "task done" command.
- `task update` changes allowed fields; `task transition` checks state guards.
  `change verify` checks change-linked evidence. Actual business graph checks
  remain with `arc-verify` until an explicit migration.
- A future `capabilities`/protocol-version handshake must advertise
  `arc_db` support and schema version before any plugin uses these commands.
  Do not infer support solely from the binary's name.

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
- Begin with a validated SQLite WAL implementation; evaluate the Turso
  engine and driver independently before replacing storage. Never assume
  multi-process concurrency, remote sync or cloud persistence is available.
- Ignore `.arc/arc.db` and its journal/WAL files in Git; for collaboration
  define replayable exported events or migrations, not Git merges of a
  binary database. No credentials or full private source by default.

## 6. Plugin/Hook reconciliation

The existing `arc-policy` PreToolUse/PostToolUse/Stop hooks are the integration
points, **not** proof that `arc-idx update` is implemented.

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
   `arc-policy` and `arc-verify` completion gates authoritative.
5. **Recovery:** replay pending events after restart, rescan Git changes
   when hooks were skipped, and report `sync_pending` / `conflict` / `stale`
   explicitly. A missing event never means "no change".

## 7. Migration and acceptance

The present repository uses `.arc/verify/data.db` (local Turso business graph)
and `arc-policy` state/receipts independently; `aitask` handles cross-agent
orchestration. **Do not rename these files, silently import them, dual-write
them, or change their verification authority** merely because `arc.db` is
documented. An opt-in, tested migration must map IDs and evidence versions,
compare old/new reads, support rollback, and cut over exactly one owner at a time.

Delivery stages: (1) local schema + file/task/change/event reads and safe
reconciliation; (2) plugin hooks, replay and revision-scoped evidence;
(3) design/contract/invariant impact graph, then tested storage alternatives.

Required acceptance scenarios: repeated PostToolUse event; two agents editing
the same task from different worktrees; optimistic version conflict; crash
between state commit and index update; offline/restart replay; change without
a plugin event; branch checkout invalidating index/evidence; unauthorized
transition; failed test never producing `done`; schema rollback. Report what
was actually tested. Until those pass, treat this document as design guidance,
not an operational guarantee.
