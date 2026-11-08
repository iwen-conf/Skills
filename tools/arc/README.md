# arc — one CLI, project-local state and verification

`arc` 0.2.x is the public entrypoint for source intelligence, project resource state,
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

The installer builds `arc`, `arc-idx`, and `arc-verify` into the same binary
directory; `arc` finds sibling binaries before `PATH`. The resource engine
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
| `arc task create`, `ensure`, `show`, `transition` | Versioned workflow tasks; `ensure` binds an existing policy task ID idempotently |
| `arc file update`, `show --path <path>` | Record or inspect versioned content hashes and change events |
| `arc change collect`, `show` | Snapshot a bounded Git change set |
| `arc update --changed-only` | Idempotent post-edit Git reconciliation |
| `arc sync reconcile` | Catch up committed changes missed by hooks, with a durable Git cursor |
| `arc event list` | Bounded, read-only inspection of append-only state events |

```bash
arc db status --repo .
arc db init --repo .                 # only without an existing verifier DB
arc task create --title "Repair payment"
arc task ensure --id POLICY-TASK-ID --title "payment (arc-code)"
arc task transition --id TASK-ID --to ready --expected-version 1
arc file update --path internal/payment/service.go
arc file show --path internal/payment/service.go
arc event list --resource task --limit 20
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

### Guarantees and deliberate limits

Resource writes use short SQL transactions and append-only events/outbox rows.
Task transitions use expected-version CAS; `done` is intentionally unavailable
until independent evidence can be bound to that exact task/revision. Code index
shards remain independently rebuildable and are not part of `arc.db`.

`arc update` observes the dirty worktree; `arc sync reconcile` additionally
replays missed Git **commits** using `arc_meta.sync_head`. Index events stay
`pending` until a future consumer acknowledges them: collecting changes is
not proof that Zoekt/ctags have been incrementally reindexed.

`arc-policy task` remains the authoritative authorization/receipt workflow.
When `.arc/arc.json` is active, Plugin PostToolUse now binds its stable task ID
through idempotent `arc task ensure` and passes `--task` to `arc sync reconcile`.
This creates a traceable resource identity without promoting a task to `done` or
inventing verification results. The lifecycle authorities are not yet merged. Current multi-agent scheduling remains owned by `aitask`.
Unsupported resource commands must not be described as shipped.
