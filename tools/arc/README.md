# arc — unified CLI entrypoint

The public command for Arc engineering work is `arc`. Version 0.1.0 is an
**executable compatibility bridge**, not yet a compiled-in merger of the
index and verification engines.

## Build and smoke test

From the Skills repository:

```bash
go -C tools/arc test ./...
bash scripts/install_arc.sh              # builds arc-idx, arc-verify, then arc
arc version
arc capabilities --json
```

The installer builds and publishes all three binaries; humans and Skills use
only `arc`. The CLI finds sibling backend binaries next to itself before using
`PATH`; this also works with an explicit `/path/to/arc` invocation. `arc-idx`
and `arc-verify` remain internal executable dependencies. `capabilities` reports whether each is available.
`arc` uses `os/exec` argument forwarding with no shell; it preserves stdout,
stderr and backend process exit codes.

## Implemented entrypoints

| Public CLI | Delegates to |
|---|---|
| `arc index`, `arc search`, `arc symbol`, `arc ast`, `arc files`, `arc stats` | `arc-idx` |
| `arc doctor`, `arc daemon`, `arc watch`, `arc reindex` | `arc-idx` |
| `arc verify init`, `arc verify gate`, `arc verify trace`, `arc verify handoff`, etc. | `arc-verify` |
| `arc capabilities --json`, `arc version`, `arc help` | This bridge |

`arc verify` passes **the original verifier flags through unchanged**,
including `--db`. It uses the legacy default `.arc/verify/data.db`.
`arc-policy task verify` is a separate authorization/evidence gate, not
an alias for `arc verify gate`.

## Planned arc.db integration (not implemented)

Read [Arc shared protocol](../../Arc/_shared/arc-db.md). Eventually one
`.arc/arc.db` holds typed resources, task events, code relationships, business
graphs, verification rounds and revision-scoped evidence. Planned commands
include `arc file`, `arc task`, `arc change`, `arc update`, `arc db` and
`arc sync`. **They currently return exit code 2 rather than doing anything.**

The existing `arc-policy` receipts and `.arc/verify/data.db` are **not**
migrated by this bridge. Do not redirect verified projects to the new path,
duplicate writes, edit live DB files or bypass a failed graph gate.

A future version must ship a versioned schema, proof-equivalent graph migration,
backup/rollback, idempotent post-edit events, concurrent-task conflict handling,
and policy-hook cutover before announcing `unified_database=true`.

## Migration policy

1. Use `arc` in new Skill instructions for supported lookup and verification
   commands; keep legacy tools installed until the native engine cutover.
2. Existing plugin runtimes may still call the legacy verifier internally.
   This is an implementation detail, not evidence of a shared database.
3. Preserve direct legacy commands for old deployments while updating plugin
   hooks, package/installer rules and CI acceptance tests.
4. Validate complete real-entry tests on the target Go toolchain before
   replacing system-installed binaries. Never treat this standalone bridge's
   unit tests as full verification of either underlying engine.
