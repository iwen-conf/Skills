# db gates: foundation (G0–G2)

Entry point, paths, approvals and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root (`$DBQ`); other paths to the project root. Waivers never change a status; rules in
[gates-release.md](gates-release.md).

## G0 — Change scope, risk and ownership

**Question.** Which database claim would be dangerous if false, who owns rollout and rollback, and
which risk class applies? Do not begin with tooling.

**Inventory** (record in `templates/database-release-qualification-plan.md` or
`templates/sql-verification-plan.md`):

```text
business operations and callers        SQL/query IDs
read / write / DDL                     transaction boundaries and isolation
shared mutable resources               locks and acquisition order
constraints / indexes / triggers / FKs retry / idempotency / outbox / external effects
migration / compatibility window       replica / CDC / failover path
historical incidents                   rollout / rollback owner
```

**Risk class.** One of DB-R0…DB-R6 from
[expert-risk-model](../../scripts/dbqual/references/expert-risk-model.md); escalate on the signals in
[../db.md](../db.md). The class supplies default gates; the actual change decides applicability.

**Evidence.** No runner. G0 is the written inventory plus the manifest: every G0–G31 claim is
`applicable=true` with selectors or `applicable=false` with `rationale`. The sample manifest binds
G0 to `toolchain.doctor`, `postgres.snapshot.pre` and `postgres.snapshot.post` (identity of the
qualified environment).

**PASS** when the inventory names every item above (or "none" with reason), the risk class and
owner are recorded, and the manifest declares the gates. **FAIL/BLOCKED**: no owner, no claim, or
undeclared gates in a `strict_complete` manifest (coverage gate fails).

## G1 — Environment, isolation and comparability

**Question.** Is the test environment safe for the claim and representative enough to compare?

**Capture:** PostgreSQL major/minor, settings hash, extensions, schema fingerprint, machine/runner
metadata, connection mode and pooler (when a pooler sits in the path), data profile and seed,
workload/script hashes. Destructive, fault and restore tests require explicit isolated-environment
acknowledgement (flag) and, under Arc, user approval in the conversation.

**Environment source.** Only the project-provided test database or CI environment. Never start
PostgreSQL or containers locally; without a DSN the gate is `BLOCKED`.

**Run:**

```bash
python3 scripts/toolchain.py --dsn-env DATABASE_URL --out "$ART/logs/dbqual-evidence/toolchain.doctor.json"
python3 scripts/postgres_snapshot.py --dsn-env DATABASE_URL --id postgres.snapshot.pre \
  --out "$ART/logs/dbqual-evidence/postgres.snapshot.pre.json"
```

- `toolchain.doctor` (kind `toolchain`): required pgbench, psql, psycopg, Python ≥ 3.11 and, when
  `--dsn-env` is given, a `psql` connectivity probe (`current_database()`, `server_version_num`).
  Optional (reported, not required): pgbadger, docker, toxiproxy-cli, sqlancer, java, clojure,
  pg_amcheck, pg_verifybackup, pg_isready. Status is PASS or FAIL only. A psycopg failure prints
  an install hint; installing needs permission and is recorded.
- `postgres.snapshot.pre|post` (kind `postgres_snapshot`): `server_version_num`, settings SHA-256,
  extensions, `pg_stat_database`, `pg_stat_wal`, `pg_stat_checkpointer`, `pg_stat_archiver`,
  `pg_stat_io` (PG16+), replication slots, schema fingerprint SHA-256, host metadata. Missing DSN →
  `BLOCKED`.

**Safety model** (enforced by runners):

1. Database name must match `safe_database_pattern` (default `(_test|_bench|sqlverify)$`).
2. Bypass needs `allow_destructive=true` and `SQLVERIFY_ALLOW_DESTRUCTIVE=1` (Arc: approval).
3. Write `EXPLAIN ANALYZE` stays refused unless `allow_write_explain=true` (Arc: approval).
4. UDF/FDW/trigger external effects escape rollback; use disposable project environments.
5. Fault/restore/failover need `--ack-isolated-environment` (run_suite: `--ack-isolated-destructive`).
6. Never reset shared production statistics to simplify a benchmark.
7. Never enable expensive tracing on a busy production system only for a test.
8. Production traces are sanitized/tokenized; credentials and sensitive payloads never enter artifacts.
9. Restore/failover/fault commands are argv arrays, never shell strings.
10. The canary evaluator never deploys or rolls back.

**PASS** when doctor and both snapshots are PASS and settings/schema/version are comparable with
any baseline used (G28). **Interpretation**: a snapshot drift between pre and post that the change
does not explain invalidates numeric comparisons.

## G2 — Representative dataset fidelity

**Question.** Does the data reproduce the production distribution that drives plans, contention
and correctness? Validate, not merely generate.

**Validate:** row scale, cardinality, hot-key/Zipf skew, tenant size distribution, status/NULL
distribution, time recency/history, parent-child fan-out, cross-column correlation,
soft-delete/history fraction, large text/JSON/array distributions (when such columns exist),
partition distribution, statistics freshness. Random rows without a distribution contract are
insufficient.

**Config:** `[dataset]` with `profile`, `seed`, `manifest` (path relative to `--root`). The manifest
(sample `examples/dataset-manifest.json`) holds `profile`, `seed` and `checks[]` of
`{id, description, sql, op, expected}`; `op` ∈ `== != >= > <= <` (default `==`). Each check is a
read-only SQL oracle such as "top 100 customers own ≥ 45 % of orders".

**Run:**

```bash
python3 scripts/dataset_audit.py --dsn-env DATABASE_URL --manifest .sqlverify/dataset/manifest.json \
  --safe-database-pattern '(_test|_bench|sqlverify)$' --statement-timeout-ms 10000 \
  --out "$ART/logs/dbqual-evidence/dataset.audit.json"
```

Evidence `dataset.audit`, kind `dataset_audit`. **PASS**: every check holds. **FAIL**: any check
misses. **BLOCKED**: missing DSN, unsafe database name, runner error. Loading or reseeding the
dataset is a write to the test database: approval when it drops or truncates.

**References:** [workload-and-data-model](../../scripts/dbqual/references/workload-and-data-model.md),
[data-lifecycle-integrity-standard](../../scripts/dbqual/references/data-lifecycle-integrity-standard.md),
[engineering-basis](../../scripts/dbqual/references/engineering-basis.md).
