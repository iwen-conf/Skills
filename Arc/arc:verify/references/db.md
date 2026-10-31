# db mode: PostgreSQL release qualification

Qualify PostgreSQL changes with the bundled dbqual v4.0.0 and Arc run binding (`MANIFEST.txt`).
Official PostgreSQL behavior governs engine semantics; [Arc contract](../../_shared/contract.md)
and this reference govern verdicts and permissions over conflicting standalone package guidance.

## When to use

- Use for changes whose correctness or performance depends on PostgreSQL: SQL, transactions,
  locking, indexes, migrations, data models, retry/idempotency, replication, backups, failover,
  major upgrades, roles/RLS; and for building or checking a G0–G31 manifest, policy or report.
- Not for: other engines (principles transfer, scripts do not), tests without database
  semantics, deployment or rollback (the canary evaluator only evaluates).

## Inputs

| Input | Source | If missing |
|---|---|---|
| Test DSN in the env var named by `database.dsn_env` | project test environment; database name matches `database.safe_database_pattern` (default `(_test\|_bench\|sqlverify)$`) | database gates `BLOCKED: no project test database` |
| Replica / subscriber / upgraded / restored DSN env vars | project topology | G19/G20/G23/G25 `BLOCKED` |
| Target PostgreSQL major | release plan → `compatibility_audit.expected_major` | G1/G25 `BLOCKED` |
| Profile `pr` / `nightly` / `prerelease` / `expert-prerelease` | user or CI lane | `pr`; `prerelease` when a release is named |
| Manifest `.sqlverify/qualification-manifest.json` | step 2 | G31 cannot pass |
| Config `.sqlverify/sql-verification.toml` | step 3 | nothing runs |

## Package and paths

```bash
DBQ="<arc:verify skill dir>/scripts/dbqual"   # package root
ART=".arc/artifacts/<task-slug>"              # project-relative
```

In code blocks, prefix `scripts/`, `examples/`, `templates/`, `schemas/`, `tests/` with `$DBQ/`;
other paths are project-relative. Run from the project root (`python3 "$DBQ/scripts/<name>.py"`):
`fault_scenario_runner.py` and `longrun_profile.command` execute argv in the current directory.
Needs Python ≥ 3.11, psycopg 3, psql and pgbench (doctor requires both); pg_amcheck and
pg_verifybackup for G21–G22.

## Environment rules (Arc contract wins)

1. Never start PostgreSQL or containers on the workstation (docker, podman, pg_ctl, initdb,
   Testcontainers), including for `examples/README.md` (assumes `127.0.0.1:5432/sqlverify`).
   Use the project test DSN; none → `BLOCKED`. `templates/github-actions*.yml` run in CI only.
2. Installing psycopg (`psycopg[binary]>=3.2,<4`), client tools or extensions (`CREATE EXTENSION`,
   `physical_integrity.install_missing`, `pg_stat_statements` per
   `examples/postgres-observability.md`) needs permission; record what was installed where. A
   server restart is a remote restart: approval.
3. Get explicit approval in the current conversation, naming database and commands, before:
   `--ack-isolated-destructive`/`--ack-isolated-environment`; `allow_destructive=true` with
   `SQLVERIFY_ALLOW_DESTRUCTIVE=1`; `allow_write_explain=true`; loading `examples/schema.sql`
   (`DROP TABLE`); migration down SQL; deleting/truncating setup or reset SQL; restore, failover or
   fault commands.
4. Pass `--evidence-dir "$ART/logs/dbqual-evidence" --report-dir "$ART/reports/dbqual"`.
   Baselines stay in `.sqlverify/baselines/<name>`. Owned prior output moves to `superseded/`;
   `run-manifest.json` binds each record. Gates reject other runs or changed candidates.
5. DSNs live only in env vars (evidence redacts them). Credentials, production payloads and
   unsanitized traces never enter configs, traces, evidence or reports.
6. Write `.sqlverify/` configs, fixtures, probes and evidence; report product SQL/schema defects to
   `arc:code`.

## Principles

- Correctness precedes performance; use business outcomes, not HTTP/affected-row/commit signals.
- Missing observations are neither zero nor PASS; zero comparisons prove no regression claim.
  Declare applicability; never substitute an aggregate score.
- PostgreSQL locks/MVCC/WAL/DDL/replication/recovery require the real engine; missing dependency is BLOCKED.
- Characterize project data distribution; named boundaries precede fuzz, always with independent oracles.
- Combine lock order, deterministic interleavings, seeded exploration and history checks. Detector
  selftests do not prove production deadlock freedom; retry the whole contracted operation.
- Capacity includes tail latency, saturation and recovery; replay preserves sessions/transactions.
- Qualify migrations as online protocols and backups by actual restore. Replication uses the
  application's consistency/RPO/RTO contract; maintenance, role/RLS and extension changes need gates.
- Baselines/metrics need provenance and attribution. The harness fails closed, scopes its proof
  and turns escaped incidents into permanent controls. Applicable gate files give the detailed checks.

## Risk classes

| Class | Scope | Default gates |
|---|---|---|
| DB-R0 | local read-only | G0–G6, G31 |
| DB-R1 | performance-critical query/index | + G12, G28; G13 if capacity matters |
| DB-R2 | mutable/concurrent state | + G7–G11, workload evidence |
| DB-R3 | schema/data lifecycle | + G18; usually G14, G15, G28 |
| DB-R4 | money/inventory/quota/billing/idempotency/outbox | strong invariants, G7–G11, G27 if tractable, G30, fault/recovery |
| DB-R5 | replication/HA/DR | + G19–G24, consistency/RPO/RTO contracts |
| DB-R6 | major upgrade/storage/extension/collation/platform | G14, G18, G21–G25, G28–G31, progressive rollout |

Escalate on: irreversible write, large blast radius, hot or high-cardinality shared state, unknown
production distribution, external side effect near a transaction, ambiguous commit, async or
cross-region replication, mixed-version migration, prior incident, never-rehearsed recovery. The
actual change decides applicability.

## Workflow

1. **Claim (G0).** State the claim that is dangerous if false, inventory the change, set the risk
   class. Do not begin with tooling.
2. **Manifest.** Use `examples/qualification-manifest-prerelease.json`: unique G0–G31,
   explicit applicability, PASS-only evidence requirements or non-applicable rationale.
   Pre-release declares all 32 (`strict_complete=true`); selector details are in gates-release.
3. **Config and policy.** Bind SQL, workloads and harnesses in the TOML (sections per gate in
   `db/*.md`; sample `examples/sql-verification.toml`); copy the profile policy. Validate until
   `{"status": "PASS", "errors": []}`:
   `python3 scripts/sqlverify_config.py .sqlverify/sql-verification.toml --root . --json`
   (requires `[project].name`, `[database].dsn_env`, `[database].safe_database_pattern`, `[dataset]`).
4. **Harness trust.** `cd "$DBQ" && python3 scripts/selftest.py && python3 -m unittest discover -s tests -v`.
   Harness PASS ≠ database qualification PASS.
5. **Run** from the project root:

```bash
python3 scripts/run_suite.py --config .sqlverify/sql-verification.toml --root . \
  --timeout-s "${SQLVERIFY_TIMEOUT_S:?set the whole-suite budget from the task contract}" \
  --evidence-dir "$ART/logs/dbqual-evidence" --report-dir "$ART/reports/dbqual" \
  --qualification-manifest .sqlverify/qualification-manifest.json \
  --gate-policy .sqlverify/gate-policy-prerelease.json \
  --baseline-dir .sqlverify/baselines/current   # --ack-isolated-destructive only per rule 3
```

   Require a finite positive task budget. Dedicated owned output directories cannot overlap inputs
   or tracked source; ownership checks precede archives. Timeout/start failures retain BLOCKED
   evidence; expired budgets stop scheduling. Reports and baseline promotion revalidate the run.
   Suite PASS covers executed commands; all-n/a is NOT_RUN. See dbqual `examples/README.md` for
   these runner boundaries, including unqualified Windows process-tree cleanup.

   A step runs only when its config section exists. Order: config check, toolchain, snapshot pre,
   dataset, SQL static, schema, security, lock order, EXPLAIN, properties, invariants, plan sensitivity,
   isolation, txn, concurrency, fuzz, retry, replay, pgbench, overload, migration, maintenance,
   physical, replica, logical replication, compatibility, history, fault, longrun, backup/restore,
   failover, snapshot post, baseline, coverage, release gate, report. `--fail-fast` stops at the
   first nonzero step; otherwise it continues and exits 2. No DSN in `toolchain.doctor` → stop;
   database gates are `BLOCKED`.
6. **Decide** from `release-gate.json`, `qualification.coverage.json` and the evidence files with
   the mapping below; write the report ([db/gates-release.md](db/gates-release.md)).
7. Promote baselines only deliberately from a PASS gate (G28). Post-deploy canary and
   reconciliation, waivers and incident learning: [db/gates-release.md](db/gates-release.md).

## Profiles

| Profile | Adds | Policy template |
|---|---|---|
| local | config check, small-fixture properties, EXPLAIN named params, schema/invariants, lock-order linter, critical deterministic concurrency | none |
| `pr` | G0–G11 per manifest, small G12 where stable, migration precheck on schema change, coverage gate; a shared runner is no precision lab | `examples/gate-policy-pr.json` |
| `nightly` | larger dataset, parameter expansion, contention exploration, full workload matrix, G13, G15, maintenance/physical, scheduled history checks | `examples/gate-policy-nightly.json` |
| `prerelease` | sanitized replay, trusted baseline, migration/backfill/cutover, faults, restore/PITR, replica/failover, upgrade compatibility, longer soak, `strict_complete=true` | `examples/gate-policy-prerelease.json` (= `gate-policy.json`) |
| `expert-prerelease` | + coverage, schema, invariants, isolation, retry, longrun, maintenance, history; fails closed until bound to project IDs | `examples/gate-policy-expert-prerelease.json` |
| post-deploy | canary vs control, reconciliation, lock/deadlock/40001/timeouts, pool and tail latency, replica lag, migration divergence, auto pause | `examples/canary-policy.example.json` |

Template IDs and thresholds are samples, not SLOs. CI lanes: `templates/github-actions-{pr,nightly,prerelease}.yml`.

## Evidence and verdicts

Evidence (`schemas/evidence.schema.json`): `kind`, `id`, `run_id`, `status`, timing, `command`,
`observed`, `violations`, `warnings`, `reproduction` (seeds, hashes, redacted DSN, versions);
stable IDs are contract. Exit 0 on PASS (also disabled schema/security/maintenance/physical audits),
else 2. Apply release-ops verdict semantics: preflight/observer/runner gaps are BLOCKED, valid
contract divergence FAIL. Legacy exception labels must not be reported as confirmed product defects.

| dbqual output | Arc verdict |
|---|---|
| disabled or non-applicable claim | `NOT_RUN` with config/rationale |
| `toolchain.doctor` FAIL only for missing DSN/tools | `BLOCKED: <missing item>` |
| `release-gate.json` PASS, plus coverage PASS when a manifest is used | PASS |
| release gate FAIL citing FAIL evidence, a breached metric on executed evidence, `inconsistent_pass`, `invalid_evidence_selector`, `invalid_policy`, or coverage FAIL from manifest errors | FAIL |
| release gate FAIL whose failures all cite missing/BLOCKED/NOT_RUN evidence or `empty_policy` | `BLOCKED: <evidence IDs>`; not qualified |
| run-binding failures (`stale_evidence`, `candidate_mismatch`, …) | `NOT_RUN (stale)`; rerun run_suite on the current commit |
| reruns of one ID with the same seed/config disagree (no runner emits FLAKY) | FLAKY; keep both evidence dirs |
| run_suite status/exit code, `selftest.py` | pipeline health, never the verdict |

Never edit an evidence status or add BLOCKED/NOT_RUN/FLAKY to a release-blocking `allowed_status`:
that is a waiver. The Arc report (contract §9) carries the dbqual report sections inside
Evidence.

## Gate catalog

| File | Gates |
|---|---|
| [db/gates-foundation.md](db/gates-foundation.md) | G0 scope/risk, G1 environment, G2 dataset |
| [db/gates-schema-query.md](db/gates-schema-query.md) | G3 schema contract, G4 query correctness, G5 EXPLAIN, G6 plan sensitivity |
| [db/gates-concurrency.md](db/gates-concurrency.md) | G7 isolation, G8 lock order, G9 deterministic concurrency, G10 contention, G11 retry/idempotency, G27 history |
| [db/gates-workload.md](db/gates-workload.md) | G12 benchmark, G13 overload, G14 replay, G15 soak, G28 baseline; performance discipline |
| [db/gates-operations.md](db/gates-operations.md) | G16 MVCC/XID, G17 WAL/IO, G21 physical integrity, G26 security |
| [db/gates-migration-upgrade.md](db/gates-migration-upgrade.md) | G18 migration/cutover, G25 version/extension/locale |
| [db/gates-replication-recovery.md](db/gates-replication-recovery.md) | G19 logical replication, G20 replicas, G22 backup, G23 restore/PITR, G24 failover, faults |
| [db/gates-release.md](db/gates-release.md) | G29 canary, G30 reconciliation, G31 completeness; release gate, report, waivers |

Before high-risk work read [final-gate-catalog](../scripts/dbqual/references/final-gate-catalog.md)
and [public-engineering-basis-v4](../scripts/dbqual/references/public-engineering-basis-v4.md); each
gate file links its standards. Chinese guide: `docs/dbqual/README.zh-CN.md` (Skills repository).
