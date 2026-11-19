# db gates: replication and recovery (G19, G20, G22–G24, fault scenarios)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `C` = `.sqlverify/sql-verification.toml`, `F` = evidence file under
`$ART/logs/dbqual-evidence/`. Waivers: [gates-release.md](gates-release.md).

**Environment.** Every topology here (subscriber, replica, backup store, restored clone, HA pair,
proxy) is project-provided. Never start PostgreSQL, clusters or containers on the workstation; if
the project has no such environment, the gate is `BLOCKED` with that reason. Restore, failover and
fault commands are destructive: explicit approval in the conversation, then
`--ack-isolated-environment` (run_suite: `--ack-isolated-destructive`); without the flag the runner
records `BLOCKED`. Commands are argv arrays (no shell) and run with cwd `--root`, except fault
scenarios (current directory).

## G19 — Logical replication and CDC

**Validate** when PostgreSQL logical replication or CDC is in the path: replica identity
assumptions, subscription workers, apply errors/conflicts, lag/freshness, schema compatibility,
slot retention, initial sync/cutover, consumer duplicate/reorder semantics outside PostgreSQL.
Also insert/update/delete propagation, ordering assumptions, restart/resubscribe, initial snapshot
plus ongoing changes, source/target reconciliation. Pair the audit with application CDC/outbox
oracles (G11, G30).

**Config `[[logical_replication_audit]]`:** `subscriber_dsn_env` (sample `SUBSCRIBER_DATABASE_URL`),
`safe_database_pattern`, `min_active_workers` 1, `max_apply_errors` 0, `max_conflicts` 0 (sum of
`confl_*` in `pg_stat_subscription_stats`), `max_apply_age_ms` (sample 5000).

```bash
python3 scripts/logical_replication_audit.py --config C --case ID --out F   # logical-replication.<id>, kind logical_replication
```

**FAIL**: workers below minimum, apply errors or conflicts above budget, apply age above budget.
**BLOCKED**: missing subscriber DSN, unsafe name, query error.

## G20 — Read-replica consistency

**State the model first:** eventual only, bounded staleness, read-your-writes, monotonic reads,
session consistency, primary fallback after write. Test write on primary, visibility on replica,
monotonicity and the freshness budget. Async replicas can be healthy and still stale: that is an
application contract question. Primary-side SERIALIZABLE does not protect replica reads. Managed
session tokens/bookmarks are part of the protocol: test loss, reuse, expiry and routing.

**Config `[[replica_consistency]]`:** `primary_dsn_env` (default `[database].dsn_env`),
`replica_dsn_env`, `runs` 5 (sample 20), `poll_ms` 25, `max_visibility_ms` 5000 (sample 2000),
`require_monotonic_reads` true, `create_probe_table` true, `probe_schema` `sqlverify_probe`,
`probe_table` `replica_visibility` (writes marker rows on the primary).

```bash
python3 scripts/replica_consistency_runner.py --config C --case ID --out F   # replica.<id>, kind replica_consistency
```

**FAIL**: a marker not visible within budget, max visibility over budget, monotonic-read failure.
**BLOCKED**: missing primary/replica DSN, unsafe name, error. Candidate policy gates: visibility
P99 ≤ contract, zero monotonic-read violations, zero stale reads under read-your-writes, slot WAL
retention ≤ budget, replica conflicts/cancellations ≤ budget, promotion catch-up within RPO/RTO,
reconciliation mismatch = 0.

**References:** [replication-consistency-standard](../../scripts/dbqual/references/replication-consistency-standard.md).

## G22 — Backup integrity and provenance

Verify backup manifests/integrity when available. **Record:** backup ID/time/source,
PostgreSQL/tool version, retention/location, manifest/checksum result, required WAL availability.
G22 never substitutes for G23 (`pg_verifybackup` does not replace a test restore).

Run through the G23 case with `verify_backup = true` (default): `pg_verifybackup [pg_verifybackup_args]
<backup_path>` with `verify_timeout_s` 900; `backup_path` relative to `--root` (sample
`.sqlverify/backups/latest`). Missing path or tool → `BLOCKED`; nonzero exit → `FAIL`.

## G23 — Restore, PITR, RPO and RTO

Actually restore into an isolated project target. **Validate:** server becomes healthy,
schema/data invariants, critical rows/totals, replay/sequence state (when the system depends on
them), PITR target reached, measured data loss (RPO), measured recovery time (RTO), application
smoke against the restored target. Four separate claims: backup integrity, restorability,
recoverability to target/timeline, business truth.

**Record:** backup identifier/timestamp, manifest hash, WAL/archive range, PostgreSQL version,
restore commands, start-to-healthy RTO, recovery target/timeline, invariant results,
row/count/checksum/reconciliation results, smoke tests, cleanup result. PITR markers:
`M1 committed before target => MUST exist`, `M2 committed after target => MUST NOT exist`. RTO and RPO
are measured, never taken from configuration (include download/decompression/WAL replay/bootstrap
time). Periodically sample older backups and timelines. Where safe, inject missing/corrupt backup
pieces, missing WAL, permission or credential expiry, unavailable archive, wrong recovery target.

**Config `[[backup_restore_rehearsal]]`:** `backup_path`, `verify_backup`, `pg_verifybackup_args`,
`restore_command` argv (`restore_command_timeout_s` 1800), `healthcheck_command` argv or
`restored_dsn_env` connect probe, `healthcheck_timeout_s` 180, `healthcheck_poll_s` 2,
`healthcheck_command_timeout_s` 15, `max_rto_s`, `safe_database_pattern`, read-only `assertion`s on
the restored DSN, `cleanup_command` (`cleanup_timeout_s` 300; always attempted).

```bash
python3 scripts/backup_restore_rehearsal.py --config C --case ID --root . --out F --ack-isolated-environment   # backup-restore.<id>
```

**PASS**: verification passes, target healthy, RTO ≤ `max_rto_s`, assertions hold. **FAIL**:
unhealthy target, RTO over budget, assertion failure. **BLOCKED**: no ack, restore command failure,
missing restored DSN, runner error.

**References:** [backup-restore-pitr-standard](../../scripts/dbqual/references/backup-restore-pitr-standard.md),
[migration-and-recovery-standard](../../scripts/dbqual/references/migration-and-recovery-standard.md).

## G24 — Failover, fencing and failback

Rehearse the real topology/provider harness: primary loss, promotion/failover, client
reconnect/pool recovery, write safety, split-brain/fencing assumptions, replication catch-up,
failback/rejoin, RTO/RPO, business invariant during and after failover. A healthcheck returning 200
does not prove there is no duplicate or missing business effect.

**Config `[[failover_rehearsal]]`:** `prepare_command` (`step_timeout_s` 300), `workload_command`
(background) + `workload_warmup_s` 2, `inject_failover_command` (`failover_command_timeout_s` 300),
`healthcheck_command` (`healthcheck_timeout_s` 180, `healthcheck_poll_s` 1,
`healthcheck_command_timeout_s` 10), `max_rto_s`, `oracle_command` list of argv
(`oracle_timeout_s` 120; e.g. no duplicate effects, ledger reconciles), `failback_command`
(`failback_timeout_s` 300), `cleanup_command` (`cleanup_timeout_s` 300).

```bash
python3 scripts/failover_recovery_runner.py --config C --case ID --root . --out F --ack-isolated-environment   # failover.<id>
```

**PASS**: healthy within timeout, RTO ≤ budget, every oracle and failback exits 0. **FAIL**:
otherwise. **BLOCKED**: no ack, prepare or inject command failure, runner error.

## Fault scenarios (G11, G23, G24 evidence)

Network/process faults around the database (connection reset, timeout, proxy faults via a
project Toxiproxy; installing it needs permission). JSON spec (sample
`examples/fault-scenario.example.json`): `id`, `isolated_environment_required` true (required),
`command_timeout_s` 60, `prepare` / `before_oracle` argv lists, `target` argv (background,
`target_timeout_s` 60), `fault_after_s` 1, `inject`, `fault_hold_s` 1, `recover`,
`allowed_target_returncodes` [0], `oracle`, `cleanup` (always runs). Config:
`[[fault_scenario]] id, scenario_file`.

```bash
python3 scripts/fault_scenario_runner.py --scenario-file .sqlverify/faults/<id>.json --out F --ack-isolated-environment   # fault.<id>
```

**PASS**: target exit allowed and every oracle exits 0. **FAIL**: target/oracle failure, phase
command failure, timeout. **BLOCKED**: no ack or `isolated_environment_required` false.
