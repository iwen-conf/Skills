# arc — unified CLI and opt-in project-local state

The public executable is `arc`. Version **0.2.0** keeps the index and
business-verification engines as subprocess backends, while adding a local
**Turso `arc.db`** resource and event store. It is not one statically linked
binary yet. Existing `arc-policy` receipts remain separate.

## Install and test

From the Skills repository:

```bash
go -C tools/arc test -count=1 ./...
go -C tools/arc-verify test -count=1 ./...
bash scripts/install_arc.sh             # builds arc-idx, arc-verify and arc
arc version
arc capabilities --json
```

The installer requires the Go toolchain used by the index backend (currently
Go 1.26.5). `arc` prefers sibling binaries before `PATH`, preserves verifier
arguments and exit codes, and does not invoke a shell.

## New project: initialize one local database

```bash
cd /path/to/repo
arc db status --repo .
arc db init --repo .
arc db status --repo .
arc verify gate --graph EXISTING-GRAPH
```

`arc db init` refuses a second ledger if the legacy
`.arc/verify/data.db` already exists. It uses the existing `arc-verify init`
to create verifier graph tables, then creates namespaced `arc_*` tables.
An explicit `.arc/arc.json` marker activates `.arc/arc.db`. Before activation,
`arc verify` retains its legacy database default.

## Existing project: explicit offline migration

**Stop all direct `arc-verify` processes and background writers before
migration; checkpoint any nonempty legacy WAL.** Never run this against a
live/shared database.

```bash
arc db status --repo .
arc db migrate --repo . --confirm-offline
arc db status --repo .
arc verify gate --graph EXISTING-GRAPH
```

Migration refuses unsafe filesystem paths and nonempty WAL/journal sidecars,
SHA-256 hashes the legacy DB, keeps a checksum-verified copy under
`.arc/backups/verify-<sha-prefix>.db`, and copies the original to
`.arc/arc.db`. It adds `arc_*` tables without overwriting verifier tables.
For **each existing graph**, the old/new `arc-verify gate` outputs and exit
codes must match exactly before the activation manifest is written.

The original `.arc/verify/data.db` is never deleted or modified by migration.
If a stage fails, the new DB remains **inactive**, and the user must inspect
the staged data before any explicit recovery or retry. No implicit overwrite,
forced migration, or data-loss rollback exists.

`arc verify <action>` and the `arc-policy` gate use the unified DB when
activated **unless a project explicitly pins another `--db`**. Old verifier
binaries can still be invoked directly for compatibility; doing so during
active shared-DB writes is unsupported.

## Resource CLI (implemented subset)

```bash
arc task create --title "Repair payment status"
arc task show --id TASK_ID
arc task transition --id TASK_ID --to ready --expected-version 1
arc task transition --id TASK_ID --to running --expected-version 2

arc file update --path internal/payment/service.go --task TASK_ID
arc change collect --task TASK_ID
arc change show --id CHANGE_ID
arc update --changed-only
arc db status
```

**Flags must precede positional IDs** in this first resource parser.
The `task` state machine allows guarded transitions but deliberately
**rejects `done`** until an independently verified, revision-bound completion
gate can bind graph evidence to a development task. There are no implicit
task-completion events.

`arc file update` records content hashes; `arc change collect` takes bounded
Git worktree changes (up to 1,000 files) and persists a deterministic
ChangeSet fingerprint. Repeated collections of the same contents are
idempotent. An event and pending outbox row are transactionally persisted.

The existing `arc-policy` PostToolUse hook runs `arc update --changed-only`
when the repository has an active cutover manifest. Hook failure is surfaced,
not transformed into a verification PASS.

**Important limits:** New `arc_*` task records and the verifier's historical
`tasks` are still distinct logical task domains in the same database. Outbox
consumer/retry, symbols/dependencies/contract impact, doc regeneration and
automatic graph-backed task completion are not implemented. `arc update`
reports `index: pending`; Zoekt/watch indexing continues separately.

## Architecture and safety

- Git files remain the source of truth for source content.
- Local `arc.db` stores typed resource state, change events and the existing
  business verification ledger after explicit cutover.
- Runtime `arc` operations share an OS-level repository lock on Unix; Windows
  uses an exclusive-file fallback. **Direct legacy binaries do not honor this
  lock**, so a single-writer daemon is still required for robust multi-agent
  concurrency.
- Task CAS uses `--expected-version`; event and outbox writes happen in the
  same DB transaction. Automatic event consumers are not yet shipped.
- Do not commit `.arc/arc.db`, its WAL or backups. Keep `.arc/verify/` private.
- The full target model and missing acceptance gates are in
  [Arc shared protocol](../../Arc/_shared/arc-db.md).
