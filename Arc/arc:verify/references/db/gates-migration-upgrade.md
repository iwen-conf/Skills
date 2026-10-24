# db gates: migration and upgrade (G18, G25)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `C` = `.sqlverify/sql-verification.toml`, `F` = evidence file under
`$ART/logs/dbqual-evidence/`. Waivers: [gates-release.md](gates-release.md).

## G18 — Migration, DDL, backfill and cutover

**Rehearse:** preconditions, DDL lock probe, `lock_timeout` / `statement_timeout`, apply duration
and blocking, old/new application compatibility, expand-contract phases, dual write,
resumable/throttled backfill, independent reconciliation, dual-read/cutover, rollback or
forward-fix, post-cutover cleanup, WAL/replica/soak impact (G15, G17, G20). Success means the
online protocol works, not that a DDL file executed. PR lanes run at least the precheck when the
schema changes; pre-release runs the full rehearsal. The static precheck is Squawk on
`[sql_static].migration_paths` (G4 static prerequisite): it flags avoidable blocking DDL and missing
timeouts before any rehearsal; only the rehearsal measures real locks and duration.

**Config `[[migration_rehearsal]]`:**

| Key | Default | Meaning |
|---|---|---|
| `up_sql_file`, `down_sql_file` | — | paths relative to `--root`; down SQL usually drops objects (approval) |
| `transactional` | true | false runs in autocommit (`CREATE INDEX CONCURRENTLY` style) |
| `pre_assertion`, `post_assertion`, `rollback_assertion` | [] | read-only `{id, sql, op, expected, params?}` |
| `lock_probe_sql` | — | a blocker transaction runs it and holds for `lock_probe_hold_s` (5) while up SQL is tried under `lock_probe_timeout_ms` (250), then rolled back |
| `expect_lock_timeout` | false | true → probe must hit `55P03` (proves the lock guard fires) |
| `require_nonblocking` | false | true → probe hitting `55P03` fails |
| `statement_timeout_ms` / `apply_lock_timeout_ms` | 60000 / 5000 | apply guards |
| `max_apply_ms` | — | apply duration budget |
| `verify_rollback` / `rollback_required` | true / false | run down SQL + rollback assertions; require a down file |

```bash
python3 scripts/migration_rehearsal.py --config C --migration ID --root . --out F   # migration.<id>, kind migration_rehearsal
```

**PASS**: pre/post/rollback assertions hold, probe matches the declared expectation, apply within
budget, rollback verified when required. **FAIL**: any violation or runner error after preflight.
**BLOCKED**: missing DSN or unsafe database. Online-protocol steps that the runner cannot express
(dual write, backfill throttling, mixed-version compatibility, cutover, reconciliation) need
project harness evidence (G14 replay, G11/G24 commands, invariants) bound in the manifest.

**References:** [migration-and-recovery-standard](../../scripts/dbqual/references/migration-and-recovery-standard.md),
[data-lifecycle-integrity-standard](../../scripts/dbqual/references/data-lifecycle-integrity-standard.md).

## G25 — Version, extension and locale compatibility

For a major upgrade or platform change: `pg_upgrade --check` or provider precheck, extension
versions, client/driver compatibility, collation-version drift, schema/generated-expression
behavior, workload replay (G14), plan/performance comparison (G5, G12, G28), backup/rollback path
(G22–G23), replication transition (G19–G20). The audit is one evidence layer, not the whole gate.

**Config `[[compatibility_audit]]`** (sample `examples/compatibility.example.toml.fragment`):
`dsn_env` (default `[database].dsn_env`; sample `TARGET_DATABASE_URL`), `safe_database_pattern`,
`expected_major`, `expected_extensions` `{name = "version" | "*"}`,
`require_no_collation_version_mismatch` true, `check_command` argv (e.g. a project wrapper for
`pg_upgrade --check`; `command_timeout_s` 1800), `smoke_command` list of argv (`smoke_timeout_s` 300).
Commands run with cwd `--root` against the project-provided target; never start a target server
locally.

```bash
python3 scripts/compatibility_audit.py --config C --case ID --root . --out F   # compatibility.<id>, kind compatibility_audit
```

**PASS**: major matches, required extensions present at expected versions, no collation version
mismatch (when required), check and smoke commands exit 0. **FAIL**: any violation. **BLOCKED**:
missing target DSN, unsafe database name, runner error.

**References:** [upgrade-compatibility-standard](../../scripts/dbqual/references/upgrade-compatibility-standard.md).
