# arc — one CLI, project-local state and verification

`arc` 0.4.x is the public entrypoint for source intelligence, project resource state,
and business graph verification. It still delegates code indexing to `arc-idx`
and business graph execution to `arc-verify`, while **resource state uses
the same local Turso database after activation**.

## Build and test

```bash
go -C tools/arc test ./...
go -C tools/arc-verify test ./...
bash scripts/install_arc.sh
arc version
arc capabilities --json
```

The installer publishes only `arc` to the binary directory. The `arc-idx` and
`arc-verify` engines go to the private `<prefix>/libexec/arc` (for
`~/.local/bin` that is `~/.local/libexec/arc`), outside `PATH`, and stale public
copies from earlier installs are removed. `arc` resolves engines from
`libexec/arc`, then beside itself, then `PATH`. The resource engine
uses the existing official `tursogo` driver; no Turso Cloud connection is required.

## Supported commands

| Command | Behavior |
|---|---|
| `arc search`, `symbol`, `ast`, `files`, `stats` | Existing code intelligence |
| `arc index`, `doctor`, `daemon` | Code index maintenance |
| `arc verify <action>` | Business graph model, tests, trace, gate and handoff |
| `arc db status` | JSON status for active `.arc/arc.db` |
| `arc db init` | Initialize a new Turso ledger when no old graph DB exists |
| `arc db migrate --confirm-offline` | Offline copy, backup and gate-parity cutover |
| `arc task create`, `ensure`, `show`, `transition`, `complete` | Versioned tasks; `complete` checks an existing current Arc Policy PASS and index state |
| `arc file update`, `show --path <path>` | Record or inspect versioned content hashes and change events |
| `arc change collect`, `show` | Snapshot a bounded Git change set |
| `arc update --changed-only` | Idempotent post-edit Git reconciliation |
| `arc sync reconcile` | Catch up committed changes missed by hooks, with a durable Git cursor |
| `arc index flush` | Full current Zoekt/ctags rebuild; confirm source outbox batch only after healthy index and stable source snapshot |
| `arc event list` | Bounded, read-only inspection of append-only state events |
| `arc policy state get/put/status/migrate` | Opt-in Turso SSOT for Arc Policy task, history, and immutable receipts |

```bash
arc db status --repo .
arc db init --repo .                 # only without an existing verifier DB
arc task create --title "Repair payment"
arc task ensure --id POLICY-TASK-ID --title "payment (arc-code)"
arc task transition --id TASK-ID --to ready --expected-version 1
arc file update --path internal/payment/service.go
arc file show --path internal/payment/service.go
arc event list --resource task --limit 20
arc index flush --repo .
arc update --changed-only
arc change collect --task TASK-ID
arc sync reconcile                   # also catches commits missed by PostToolUse
arc verify gate --graph GRAPH-ID
```

### Offline migration from the old verifier

The old graph store lives at `.arc/verify/data.db`. A migration is **explicit**:

1. Stop agents and direct `arc-verify` writers; checkpoint the legacy database
   and ensure no live WAL/MVCC sidecars remain.
2. Run `arc db migrate --repo . --confirm-offline`. It copies the source to
   `.arc/backups/`, checks the source SHA-256, creates `.arc/arc.db`,
   adds `arc_*` resource tables and compares the **actual gate output of
   every legacy graph** against the migrated graph.
3. Only if all checks pass, it atomically installs `.arc/arc.json` to mark the
   database active. `arc verify` then selects `.arc/arc.db` by default, and
   `arc-policy` follows the activation manifest unless its configuration
   explicitly pins another database.
4. The legacy database stays available for inspection and recovery. Failed
   migrations do not produce an active manifest and must not be treated as done.

Do **not** manually copy a live WAL database, rename files over an active
ledger, or delete backups. Backups do not justify unverified PASS claims.

### Policy state and receipt physical cutover (Arc 0.4)

After installing **both** the upgraded Go `arc` CLI and Python `arc-policy`
runtime, quiesce any old agents that might write `.arc/policy/state.json`.
The original verification graph must already be active in `.arc/arc.db`.

```bash
arc db status --repo .
arc policy state status --repo .          # legacy before migration
arc policy state migrate --repo . --confirm-offline
arc policy state status --repo .          # mode: arc.db
arc policy state get --repo .             # active task, history and CAS version
arc-policy --cwd . task status            # now reads the same DB
```

`arc policy state migrate` imports the active task, historical tasks and all
legacy PASS/AUTHORIZED receipts in one Turso transaction. The source
`state.json` receives a SHA-256-named copy in `.arc/backups/`. After
successful DB readback, the legacy receipts directory is archived under
`.arc/backups/` and `state.json` is replaced with a directory tombstone:
older plugin builds cannot silently recreate a second writable state file.
An interrupted archive can be finalized by rerunning the offline migration;
newer agents use the DB row as the only authoritative Policy state throughout.
Never manually delete these backups.

The Python runtime accesses Turso through `arc policy state get/put`, not
Python `sqlite3`. The `put` operation requires an expected version and appends
an audit event plus immutable receipt rows transactionally; stale writers
fail closed. This migration does **not** mark tasks verified, change graph
results or run tests. `git commit` and `git push` remain free of per-commit
verification.

Install new host/plugin runtimes **before** migration. An unavailable or
broken DB backend must fail rather than revert to the old JSON. Windows
Policy migration and completion remain fail-closed until the Go lock matches
the Python msvcrt protocol.

### Guarantees and deliberate limits

Resource writes use short SQL transactions and append-only events/outbox rows.
Task completion now rechecks the *live* policy receipt while holding the same
`.arc/policy/task.lock` used by `arc-policy`, then acquires the `arc.db` lock
(policy lock first, database lock second) and commits the CAS transition.
Revoked/replaced receipts and concurrent policy task changes cannot silently
produce `done`. This completion path currently fails closed on Windows until
its policy lock adapter matches Python's `msvcrt` locking protocol.

Task transitions use expected-version CAS. `done` is now available only through
`arc task complete --id <policy-task-id> --expected-version N` (or `arc task
transition --to done`), which checks an **existing current PASS** from
`arc-policy task status`. An `AUTHORIZED` waiver is not a PASS. The completion
and evidence fingerprint commit atomically, and tests are not re-run by `complete`.
Index shards remain independently rebuildable and are not part of `arc.db`.

`arc update` observes the dirty worktree; `arc sync reconcile` additionally
replays missed Git **commits** using `arc_meta.sync_head`. Only file/change events enter the index Outbox; task lifecycle events remain
in the append-only journal without triggering a full rebuild. `arc index flush`
is the real Outbox consumer. It runs the **existing full** `arc-idx` indexer,
checks `doctor` health, source snapshot freshness and concurrent changes before
marking its bounded original event batch delivered. Failures keep events
pending for retry; newer events are not accidentally acknowledged. This is
**batched full indexing**, not yet a true per-file incremental Zoekt writer.

`arc-policy task` remains the authoritative authorization/receipt workflow.
When `.arc/arc.json` is active, Plugin PostToolUse now binds its stable task ID
through idempotent `arc task ensure` and passes `--task` to `arc sync reconcile`.
This creates traceable task identity without prematurely promoting it to
`done`. `arc task complete` additionally requires no pending source index events
and an index snapshot matching the verified source. Run it after `arc index flush`
and a successful `arc-policy task verify`, not on each Git commit.

`arc task show` reports the historical completion and `source_revision_current`,
but does **not** rerun independent runtime evidence. `arc-policy` remains the
current authority for the actual verification receipt; `arc.db` records its
immutable fingerprint and checks in `task.completed` events. Full policy-state
migration and native incremental indexing are future work. Current multi-agent scheduling remains owned by `aitask`.
Unsupported resource commands must not be described as shipped.
