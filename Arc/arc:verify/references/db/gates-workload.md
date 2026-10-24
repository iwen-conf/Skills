# db gates: workload, capacity and baseline (G12–G15, G28)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `C` = `.sqlverify/sql-verification.toml`, `D` = `$ART/logs/dbqual-evidence`, `F` =
an evidence file in `D`. Waivers: [gates-release.md](gates-release.md). Load runners refuse
databases that miss `safe_database_pattern` and report `BLOCKED`.

## Performance discipline

No universal slow-SQL threshold. OLTP triage bands, never release policy:

```text
< 50 ms     often healthy, but frequency/resource cost can still dominate
50-100 ms   inspect high-frequency and tail behavior
100-300 ms  inspect plan/distribution on interactive paths
300-500 ms  strong slow-query candidate
> 500 ms    strong triage signal for interactive OLTP
> 1 s       normally high priority unless explicitly analytical/batch
```

Derive budgets from the service SLO and query role. Always separate execution latency, lock wait,
pool wait, network/RPC, queue/schedule lag, retry time and end-to-end business latency. Absolute
budgets (P95, P99, error rate, unexpected SQLSTATE = 0, invariant failures = 0) and relative
budgets go in the gate policy; no weighted score. A noisy shared CI runner is not a precision lab.

## G12 — Business workload benchmark

Custom pgbench scripts or project load drivers with real operation weights. **Declare:** weighted
operations, clients/threads, warmup/duration, repetitions, protocol simple/extended/prepared, base
seed + derived repetition seeds, arrival rate when SLO/load shape matters, latency limit and retry
policy. **Measure:** TPS, P50/P95/P99, schedule lag / scheduled latency, full success/failure/retry
counts, per-script results, `pg_stat_statements` delta, IO/temp/WAL attribution. A 5 ms SQL executed
100k times can matter more than a 2 s report executed once.

**Config `[[workload]]`:** `id`, `scripts = [{path, weight}]` (weight default 1), `clients` list
(one evidence per value), `threads` (default = clients), `duration_s`, `warmup_s` 0 (warmup logs at
sampling 0.01), `repetitions`, `protocol` `prepared` (default) | `extended` | `simple`,
`random_seed` 1 (rep seed = base + rep), `rate_tps`, `latency_limit_ms`, `max_tries` 1,
`log_sampling_rate` 1.0, `progress_s` 10, `capture_pg_stat_statements` true,
`require_pg_stat_statements` false, `capture_query_text` false, `command_timeout_extra_s` 120.

Runs `pgbench -n -c -j -T -M -r -P -l --log-prefix --failures-detailed --verbose-errors
--random-seed=…` plus `--rate`, `--latency-limit`, `--max-tries`, `--sampling-rate` when set. Raw
logs: `D/raw/<workload>/<workload>.c<N>.r<rep>/pgbench_*` (fixed by the script).

```bash
python3 scripts/pgbench_runner.py --config C --workload ID --root . --evidence-dir D   # bench.<id>.c<N>, kind pgbench
python3 scripts/pgbench_log.py D/raw/<workload>/<run>/pgbench_* [--rate] [--retries]   # re-parse raw logs
```

Observed: median TPS, median of per-repetition P50/P95/P99, max, schedule lag, exact
`failure_rate`, retries, `tps_cv`, `pg_stat_statements_delta`. **PASS**: every repetition exits 0.
**FAIL**: any pgbench failure or missing repetition. **BLOCKED**: no DSN, pgbench/psql missing,
unsafe database, warmup or required `pg_stat_statements` unavailable. Latency/failure budgets are
policy `metric_rule`s (samples: c16 P95 ≤ 100 ms, P99 ≤ 200 ms; c64 `failure_rate` ≤ 0.001). Enabling
`pg_stat_statements` changes server config: permission and approval for the restart.

## G13 — Overload, backpressure and recovery

Run `nominal -> ramp -> expected capacity -> beyond cliff -> recovery at nominal`. **Answer:** where
queueing turns nonlinear, what fails first, whether rejection is graceful, retry amplification,
lock/pool saturation, whether P99/TPS recover after demand falls, recovery time.

**Config `[[overload_profile]]`:** `id`, `workload` (a `[[workload]]` id), `clients`,
`nominal_phase`, `recovery_phase`, `max_recovery_p99_ratio` 1.25, `min_recovery_tps_ratio` 0.90;
`[[overload_profile.phase]]` `name`, `kind`, `rate_tps`, `duration_s` (default 30),
`max_failure_rate` (default 1.0), `max_p99_ms`. Sample: 50 → 100 → 250 → 50 TPS with failure budgets
0.001 / 0.01 / 0.10 / 0.001.

```bash
python3 scripts/overload_recovery_runner.py --config C --profile ID --root . --evidence-dir D   # overload.<id>; raw D/raw/overload.<id>/
```

**FAIL**: a phase exceeds its failure rate or P99; named phases missing; recovery P99 / nominal P99
> `max_recovery_p99_ratio` or recovery TPS / nominal TPS < `min_recovery_tps_ratio`. **BLOCKED**:
preflight or runner error.

**References:** [workload-replay-and-overload](../../scripts/dbqual/references/workload-replay-and-overload.md).

## G14 — Production-shaped workload replay

Sanitized traces preserve logical session, relative time, BEGIN/COMMIT/ROLLBACK, session order,
cross-session dependencies, parameters/tokens, rowcount/scalar/result digest where stable.
Application-level harness: no Oracle RAT kernel-level fidelity. Replay complements pgbench, never
replaces targeted tests.

**Trace JSONL:** `id`, `session`, `offset_ms`, `op` (`sql` default | `begin` | `commit` |
`rollback`), `sql`, `params`, `after` (dependency ids), `fetch` (`one` | `all`), `expected_rowcount`,
`expected_scalar`, `expected_result_sha256`, `tag`. **Config `[[workload_replay]]`:** `id`,
`trace_file`, `time_scale` 1.0, `statement_timeout_ms` 30000, `dependency_timeout_s` 30,
`max_result_rows` 10000, `max_runtime_s` (default max(30, trace span × scale + 30)),
`allowed_sqlstates`.

```bash
python3 scripts/workload_replay.py --config C --replay ID --root . --out F   # replay.<id>, kind workload_replay
```

**FAIL**: runtime exceeded, unexpected SQLSTATE, fatal session/dependency error, any oracle
mismatch. **BLOCKED**: DSN/safety/trace load error. Traces from production must be sanitized
before they enter the repository or artifacts.

## G15 — Soak and long-run stability

Short benchmarks miss accumulating failure. Observe over a representative period: connections and
pool wait, idle-in-transaction, transaction age, lock waiters, rollback/deadlock counters, temp
files/bytes, WAL growth, replication lag, memory/CPU/FDs via project telemetry, bloat/dead
tuples/statistics drift, queue/backlog.

**Config `[[longrun_profile]]`** (`templates/longrun-profile.toml.fragment`): `id`, `duration_s`
(required), `sample_s` 5, `dsn_env`, `max_idle_in_tx` 0, `max_lock_waiters`, `max_xact_age_ms`,
`command` argv (wrapped workload; runs in the current directory).

```bash
python3 scripts/longrun_monitor.py --dsn-env DATABASE_URL --duration-s 3600 --sample-s 5 --out F \
  --max-idle-in-tx 0 --max-lock-waiters 5 --max-xact-age-ms 60000 -- ./bin/steady-workload
```

Evidence id is always `longrun.resource` (kind `longrun_monitor`); run_suite names the file
`longrun.<id>.json`, so select by kind when several profiles exist. **FAIL**: a peak exceeds a
limit, the wrapped command fails or overruns, or runner error. **BLOCKED**: missing DSN or unsafe
database. Memory/CPU/FD/queue signals come from project telemetry and are attached as extra
evidence.

## G28 — Trusted baseline regression

A baseline needs provenance and cryptographic file hashes; compare only comparable environments;
use uncertainty, not one noisy point.

```bash
python3 scripts/baseline_promote.py --evidence-dir D --gate "$ART/reports/dbqual/release-gate.json" \
  --destination .sqlverify/baselines/<name> --root .
python3 scripts/regression_compare.py --current-dir D --baseline-dir .sqlverify/baselines/<name> \
  --out D/regression.baseline.json --max-p95-regression 0.20 --max-p99-regression 0.25 \
  --max-tps-drop 0.15 --max-explain-time-regression 0.50
```

- Promotion refuses a non-PASS gate, a gate whose `run_id` differs from the evidence directory's
  run, a directory mixing runs, and a non-empty destination; writes `baseline-manifest.json`
  (SHA-256 per file, git metadata, `run_id`). `--allow-nonpass` is an emergency override recorded in the
  manifest: treat it as a waiver, never as a trusted baseline. Never auto-bless a slower run.
- Compare (`regression.baseline`, kind `regression_compare`; run_suite uses the default thresholds
  shown): manifest hash mismatch → FAIL. PostgreSQL major, settings hash, schema hash, dataset
  profile/seed or workload definition/script hashes differ → FAIL unless
  `--allow-environment-drift`. Host arch/CPU change → warning.
- Nothing compared is never PASS. **BLOCKED**: baseline directory or `baseline-manifest.json`
  absent; current directory without a run manifest or holding another run's records; a
  `postgres.snapshot` missing on either side or `dataset.audit` on one side (unless drift is
  allowed); a baseline pgbench/EXPLAIN id absent from the current run; zero effective comparisons
  (`observed.effective_comparisons`: pairs with a computable delta).
- With ≥ 3 repetitions on both sides, bootstrap CIs on median deltas: FAIL only when the CI low
  exceeds the P95/P99 limit or the TPS CI high is below −limit; a point-only breach is a warning.
  EXPLAIN execution-time regression > 0.50 → FAIL (confirm with workload latency before tuning).

**References:** [release-gates](../../scripts/dbqual/references/release-gates.md),
[ci-operating-model](../../scripts/dbqual/references/ci-operating-model.md),
[observability-and-statistics](../../scripts/dbqual/references/observability-and-statistics.md).
