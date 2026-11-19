# db gates: release, production and evidence (G29–G31)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `D` = `$ART/logs/dbqual-evidence`, `R` = `$ART/reports/dbqual`. This file also holds
the coverage gate, release gate, report contract, waivers and incident learning used by every gate.

## G29 — Canary and progressive rollout

Pre-production qualification reduces uncertainty; it does not remove production-only risk.
**Declare before rollout:** candidate/control population, minimum sample volume, minimum bake time,
representative attributable metrics, business/data reconciliation metric, automatic
pause/rollback/containment thresholds, irreversible-change handling, owner and maximum decision
latency (`templates/post-deploy-database-guardrails.md`). Useful signals: API/business success, DB
P95/P99 and pool wait, 40001/40P01/lock and statement timeouts, lock-wait tail, connection
utilization, WAL/temp/checkpoint change, replica lag/read-consistency failures, migration
divergence, duplicate effects/illegal transitions, reconciliation mismatches. Never auto-rollback an
irreversible schema/data change without proven rollback semantics; contain by feature disable,
traffic stop, dual-read fallback or forward-fix.

**Metrics file:** `{bake_seconds, canary_fraction, control: {metric: [samples]}, canary: {…}}`.
**Policy:** `min_bake_seconds`, `min_canary_fraction`, `min_samples_per_metric` 3,
`bootstrap_samples` 4000, `metric[]` `{name, direction (lower_is_better default | higher_is_better),
max_relative_regression, absolute_max, absolute_min}` (sample: 900 s, 1 %, 5 samples; P99 ≤ +20 %
and ≤ 100 ms, error rate ≤ +50 % and ≤ 0.01, deadlocks/min ≤ 0).

```bash
python3 scripts/canary_evaluator.py --metrics <canary-metrics.json> --policy .sqlverify/canary-policy.json \
  --out D/canary.database.json   # canary.database, kind canary_evaluation
```

**FAIL**: bake or fraction below policy, fewer samples than the minimum, canary median outside an
absolute bound, bootstrap CI of the relative delta entirely beyond the limit. A point-only breach
is a warning. **BLOCKED**: unreadable input or runner error. The evaluator never deploys or rolls
back; metrics come from the project's production telemetry.

**References:** [canary-postdeploy-standard](../../scripts/dbqual/references/canary-postdeploy-standard.md).

## G30 — Production reconciliation and incident regression

For critical truth, define how production detects violations tests missed: ledger/control-total
reconciliation, inventory/capacity mismatch, orphan/duplicate detector, outbox/inbox mismatch,
illegal transition counter, migration divergence, replica consistency probe, backup freshness,
synthetic transaction. Use an oracle independent of the code path it checks.

**Evidence (minimum):** invariant, observer, cadence, detection latency, tolerated mismatch, action
threshold, repair/rollback path. "Dashboard exists" is not evidence. A mismatch needs severity and
escalation owner, containment (pause rollout, feature flag, cohort isolation, retry reduction),
evidence capture before destructive repair, repair/replay/rebuild procedure, audit trail and a
retrospective control. No runner: bind project evidence IDs in the manifest or mark
`applicable=false` with rationale.

**Incident learning.** For every escaped database incident: identify the violated invariant/SLO,
the gate that should have caught it and why evidence missed it; reproduce if feasible; persist the
deterministic scenario/seed/trace/history (`.sqlverify/regressions/incident-corpus/`); add a
prevention, detection, reconciliation or recovery control; prove the control catches the known
failure. A postmortem that only adds prose is incomplete when an engineering control is practical.

**References:** [production-reconciliation-incident-standard](../../scripts/dbqual/references/production-reconciliation-incident-standard.md).

## G31 — Evidence completeness and harness trust

**Confirm before release:** all applicable gates declared, required evidence exists, observers did
not fail silently, PASS evidence contains no violation, seeds/schedules/traces preserved, reports
deterministic, BLOCKED/NOT_RUN/FLAKY visible, harness self-tests pass, real-database integration
status distinguished from harness unit tests, waivers named, scoped and time-bounded.

**Provenance** for release-blocking evidence: commit/build identity, PostgreSQL version,
settings/schema fingerprint, data profile + seed, workload/trace/script hash, tool version,
runner/hardware identity (performance), start/end time, status and raw/minimal failure artifacts.
**Harness trust:** parsers, percentile math, SQLSTATE accounting, plan interpretation, cycle
detection, reports and gate semantics are unit-tested; remove a dependency or credential and confirm
the suite turns BLOCKED/FAIL, never PASS. A failed required observer (lock observer, scraper,
history recorder, reconciliation query) makes the claim BLOCKED even if the workload exits 0.

**Coverage gate** (manifest semantics in [../db.md](../db.md) step 2):

```bash
python3 scripts/qualification_coverage_gate.py --manifest .sqlverify/qualification-manifest.json \
  --evidence-dir D --out D/qualification.coverage.json   # qualification.coverage, kind qualification_coverage
```

FAIL on: invalid or duplicate gate id, non-boolean `applicable`, non-applicable claim without
rationale, applicable claim without requirements, fewer than `min_pass` evidence in
`allowed_status`, a manifest without claims, gates omitted under `strict_complete`, and any evidence-file problem: no
`run-manifest.json`, a record of another run, an unreadable file or a duplicate id. Output is PASS
or FAIL.

**Release gate:**

```bash
python3 scripts/release_gate.py --policy .sqlverify/gate-policy-prerelease.json --evidence-dir D --root . --out R/release-gate.json
```

**Run binding** (before any policy rule): `D/run-manifest.json` written by run_suite holds the run
id and the candidate fingerprint (HEAD, digest of every changed or untracked non-ignored path
except the evidence/report dirs, `.arc/artifacts` and `__pycache__`, config digests). The gate
recomputes the fingerprint at `--root` and accepts only records stamped with that run id. Failure
types: `missing_run_manifest`, `unreadable_run_manifest`, `candidate_unidentified` (not a git work
tree), `candidate_mismatch` (lists the changed paths), `stale_evidence`, `unreadable_evidence`,
`duplicate_evidence_id`. Hand-assembled or copied evidence directories cannot pass; rerun the suite.

Policy (`schemas/gate-policy.schema.json`): `required_evidence[]` `{id | kind | id_prefix,
allowed_status (default ["PASS"]), min_count (default 1)}`; `metric_rule[]` `{evidence_id | id |
kind | id_prefix ("*" = all), path, op (<= default, <, >=, >, ==), threshold, severity (warn →
warning only), min_count}`. Failure types: `invalid_policy` (a key or field outside the schema, a
section that is not a list), `empty_policy` (no assertion: nothing would be checked),
`invalid_evidence_selector`, `missing_evidence`,
`bad_status_or_count`, `metric_evidence_missing`, `metric_evidence_count`, `metric_gate` (missing or
non-numeric value also fails), `inconsistent_pass` (PASS evidence carrying violations), plus the
run-binding types. PASS only with zero failures; exit 0/2. The output records `run_id` and
`candidate`; baseline promotion requires that `run_id`. Design: no weighted score; each P0/P1 claim has its own required
evidence; absolute budgets for SLOs (P95, P99, error rate, unexpected SQLSTATE = 0, invariant
failures = 0); relative budgets tuned to variance (samples 20 %/25 %/15 %); prefer plan-outcome
gates (extreme estimate error, temp spill) over node bans.

## Report contract

```bash
python3 scripts/evidence_report.py --evidence-dir D --gate R/release-gate.json \
  --config .sqlverify/sql-verification.toml --root . --out-md R/sql-verification-report.md --out-html R/sql-verification-report.html
```

Deterministic Markdown + standalone HTML showing proof and gaps. Distinguish measured fact, gate
decision, hypothesis, recommendation and limitation. Full sections: `templates/final-report.md`
(38) or the 21-section contract in
[report-contract](../../scripts/dbqual/references/report-contract.md): executive result,
risk/scope/owner, G0–G31 coverage, environment/baseline comparability, dataset fidelity,
schema/invariants, query/property correctness, EXPLAIN/resources, plan sensitivity, isolation,
lock-order graph, deterministic concurrency, contention schedules, retry/idempotency, benchmark
matrix, `pg_stat_statements` attribution, overload/recovery, replay, soak and maintenance,
WAL/temp/IO, migration/backfill/reconciliation, logical/read replicas, physical integrity,
backup/restore/PITR, failover, upgrade/security, history checks, baseline comparison,
canary/post-deploy, blocking findings (each citing an evidence ID or missing required evidence),
warnings, reproduction seeds/hashes/schedules, BLOCKED/NOT_RUN/FLAKY gaps, limitations and
containment. Per-case review: `templates/sql-case-review.md`.

A report can be complete as an audit record while qualification is FAIL. Never call critical
behavior verified when required evidence is BLOCKED or absent.

**Interpretation discipline:** symptom ≠ fix. Seq Scan and Nested Loop are not bad by definition;
temp spill is not a root cause by definition; high cardinality error ≠ "create index"; 40P01 ≠
PostgreSQL bug; 40001 ≠ application bug; replica lag ≠ data loss; clean pg_amcheck ≠ no corruption;
valid backup manifest ≠ proven restore; high P99 ≠ index problem. Reproduce and falsify hypotheses,
then change the design.

**Retention** (under `$ART`, not committed unless asked): config and policy, workload/query/trace
hashes, evidence JSON, report, baseline manifest, seed/schedule/failure corpus, PostgreSQL
settings/schema fingerprints, CI run id / commit.

## Waivers

A waiver is not PASS and never edits evidence. If organizational policy permits release with a
known FAIL/BLOCKED gate, record outside the PASS state: gate/evidence ID, specific missing or failing
evidence, risk accepted, owner/approver, reason, expiry, blast-radius containment, production
detection, rollback/forward-fix trigger, follow-up issue. No anonymous, permanent or silently
auto-renewed waiver. The Arc report lists waived items under "Not verified" with their verdict.
A corruption finding stays a blocking incident until understood (G21).

**References:** [evidence-provenance-waiver-standard](../../scripts/dbqual/references/evidence-provenance-waiver-standard.md),
[release-gates](../../scripts/dbqual/references/release-gates.md),
[ci-operating-model](../../scripts/dbqual/references/ci-operating-model.md).
