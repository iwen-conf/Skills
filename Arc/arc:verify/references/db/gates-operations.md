# db gates: operations, integrity and security (G16, G17, G21, G26)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `C` = `.sqlverify/sql-verification.toml`, `F` = evidence file under
`$ART/logs/dbqual-evidence/`. Waivers: [gates-release.md](gates-release.md). These audits are
read-only; enabling extensions, statistics settings or restarts is a project-environment change
(permission; restart needs approval).

## G16 — MVCC, vacuum, XID, statistics and retention health

**Audit:** connection utilization, oldest transaction, idle-in-transaction age, database/table XID
age, dead tuple ratio, modifications since ANALYZE, statistics age, invalid indexes/unvalidated
constraints, sequence utilization, replication-slot retained WAL / lost slots, prepared
transactions. XID wraparound and stuck slots are availability risks, not tuning notes.

**Config `[maintenance_audit]`** (`enabled` default true; false → `NOT_RUN`, exit 0). Scalar limits
(unset = unchecked; sample values):

```text
max_connections_utilization       0.80        max_oldest_transaction_s        60
max_idle_in_transaction_s         15          max_database_xid_age            100000000
max_table_xid_age                 100000000   max_dead_tuple_ratio            0.20
max_inactive_slot_retained_bytes  1073741824  max_mod_since_analyze_ratio     0.20
max_statistics_age_s              86400       max_sequence_utilization        0.90
```

Zero rules (true → count must be 0): `require_no_invalid_indexes`,
`require_no_unvalidated_constraints`, `require_no_prepared_transactions`,
`require_no_lost_replication_slots`, `require_no_never_analyzed_tables` (sample false).

```bash
python3 scripts/maintenance_health_audit.py --config C --out F   # maintenance.health, kind maintenance_health
```

**PASS**: all set limits hold. **FAIL**: any exceeded. **BLOCKED**: missing DSN, unsafe database,
query error. Expert policy sample also gates `observed.connections_utilization <= 0.8`.

**References:** [maintenance-vacuum-wal-standard](../../scripts/dbqual/references/maintenance-vacuum-wal-standard.md).

## G17 — WAL, checkpoint and I/O qualification

Capture pre/post and long-run deltas from supported statistics: `pg_stat_database`, `pg_stat_wal`,
`pg_stat_io` (PG16+), `pg_stat_checkpointer`, backend wait events, `track_io_timing` /
`track_wal_io_timing` when deliberately enabled in the project environment.

**Evidence:** `postgres.snapshot.pre` and `postgres.snapshot.post` (kind `postgres_snapshot`, see
[gates-foundation.md](gates-foundation.md)), per-query WAL/temp from G12
`pg_stat_statements_delta`, `db_delta` from G15. Optional by failure model: `pg_wait_sampling`
(wait history), pgBadger (log analysis); installation needs permission.

**Gate.** Project policies may gate WAL amplification, temp spill, checkpoint pressure, storage
latency and disk headroom through `metric_rule`s on those evidence paths. Never invent universal
thresholds. **PASS**: both snapshots PASS and every project metric rule holds. **BLOCKED**: a
snapshot is missing or BLOCKED (missing evidence is not zero WAL).

**References:** [wal-checkpoint-io-standard](../../scripts/dbqual/references/wal-checkpoint-io-standard.md),
[observability-and-statistics](../../scripts/dbqual/references/observability-and-statistics.md).

## G21 — Physical and index integrity

Use page checksums (`pg_checksums` where host access permits), `pg_amcheck`/`amcheck`, index
validity and storage monitoring in the project environment. A clean scan is evidence, not proof
that corruption is impossible. A corruption finding is a blocking incident until understood.

**Config `[physical_integrity]`:** `enabled` (default false → `NOT_RUN`, exit 0), `dsn_env` (default
`[database].dsn_env`), `jobs`, `heapallindexed` false, `checkunique` true, `parent_check` false,
`rootdescend` false, `schema` [], `exclude_schema` [], `install_missing` false (creates the amcheck
extension: permission), `timeout_s` 3600 (sample 1800), `passfile`, `options`. DSN parts reach
pg_amcheck through `PG*` environment variables, not argv.

```bash
python3 scripts/physical_integrity_runner.py --config C --out F   # physical.integrity, kind physical_integrity
```

**PASS**: pg_amcheck exit 0. **FAIL**: nonzero exit (finding). **BLOCKED**: missing DSN,
`pg_amcheck` not installed, runner error. Parent/root/heapallindexed checks need stronger locks or
much more work: pick the least disruptive mode that covers the risk. Cadence: periodic,
pre-release after storage/version/index-access-method changes, post-incident on suspected
corruption, before trusting a recovered clone; not every PR. On corruption: preserve evidence,
distrust benchmark/correctness results from that instance, classify index-only vs heap/storage,
use documented repair/rebuild/recovery, investigate storage/memory/kernel/filesystem/version
history, add a detection control. Never downgrade an integrity error to a warning.

**References:** [physical-integrity-standard](../../scripts/dbqual/references/physical-integrity-standard.md).

## G26 — Database security boundary

Only when security behavior is in scope. **Qualify:** runtime/migration role privileges,
SUPERUSER/CREATEROLE/CREATEDB grants, RLS and FORCE RLS requirements, SECURITY DEFINER
`search_path`, schema ownership/CREATE rights, extension allowlist, cross-tenant assertion queries,
replication/subscription privileges. Use explicit application policy; do not apply RLS to every
table by reflex.

**Config `[security_audit]`** (`enabled` default true when present; false → `NOT_RUN`):
`allowed_superusers` (default `["postgres"]`), `require_no_unexpected_superusers` false,
`allowed_createrole` (default = allowed superusers), `require_no_unexpected_createrole` false,
`require_security_definer_search_path` false, `require_rls` (explicit `schema.table` list; missing
or disabled RLS fails), `require_force_rls` false, `allowed_extensions` + `require_extension_allowlist`
(default true; checked only when the allowlist is non-empty), `[[security_audit.assertion]]` run in
`BEGIN READ ONLY` with `statement_timeout_ms` 10000 (cross-tenant probes).

```bash
python3 scripts/security_boundary_audit.py --config C --out F   # security.boundary, kind security_boundary
```

**PASS**: no enabled rule violated, assertions hold. **FAIL**: violation. **BLOCKED**: missing DSN,
unsafe database, malformed `require_rls` entry, query error. Privilege findings in product
migrations go to `arc:code`; deep application authz review belongs to `arc:audit`.

**References:** [security-boundary-standard](../../scripts/dbqual/references/security-boundary-standard.md),
[security-and-database-boundaries](../../scripts/dbqual/references/security-and-database-boundaries.md).
