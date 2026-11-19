# Build validation — v4.0.0 Final

Package: `industrial-database-release-qualification-skill 4.0.0`

## Framework validation

| Check | Result |
|---|---:|
| Python compile | PASS |
| Unit tests | PASS — 43/43 |
| Example TOML validation | PASS |
| Parameter corpus self-test | PASS |
| Evidence report Markdown/HTML generation | PASS |
| Baseline promotion/comparison self-test | PASS |
| Synthetic canary evaluator example | PASS |

The unit suite covers plan/cardinality interpretation, pgbench parsing/full failure counts, property-test SQL binding, write-EXPLAIN safety, lock-order cycles, same-snapshot wait-for cycle semantics, workload-replay dependencies, bootstrap regression repeatability, maintenance policy, G0-G31 strict completeness, release-gate selector/count semantics, schema-integrity policy and final report sections.

## Fail-closed rehearsal

The full configured pipeline was deliberately run with `DATABASE_URL` removed.

```text
pipeline return code: 2
Evidence files:       27
PASS:                  1
FAIL:                  2
BLOCKED:              22
NOT_RUN:               2
release gate:        FAIL
Markdown report: generated
HTML report:     generated
```

This is the expected result. Missing PostgreSQL/tooling did **not** become zero-valued metrics or PASS. The single PASS was a database-independent static check.

## Real PostgreSQL integration status

**BLOCKED in this artifact-generation environment.**

The environment does not provide:

```text
psql
pgbench
pg_amcheck
pg_verifybackup
psycopg
Docker / Podman
real PostgreSQL test instance
```

Therefore this document does **not** claim any real application/database result for:

```text
TPS / P95 / P99
real lock waits or deadlock observation
40001 / 40P01 retry behavior
migration lock impact
replica/CDC behavior
restore/PITR
failover/failback
```

Those become PASS/FAIL only when the Skill runs against an isolated, production-representative project environment.

## Interpretation

`Framework self-test PASS` means the verification machinery itself passed its hermetic checks. It is not equivalent to `database release qualification PASS`. Expert pre-release manifests intentionally require project-bound evidence and must fail closed when those bindings are absent.

## Local modification 2026-10-08 — fail-closed run binding

Two release-gate false PASS paths were reproduced offline and closed:

| Reproducer | Before | After |
|---|---|---|
| Leftover PASS evidence from an earlier run, gate run on that directory | `PASS`, exit 0 | `FAIL` (`missing_run_manifest` / `stale_evidence`), exit 2 |
| `regression_compare.py` with an empty baseline (0 comparisons, no manifest) | `PASS`, exit 0 | `BLOCKED`, exit 2 |

Changes: `run_suite.py` archives earlier evidence-directory contents to `superseded/` and writes
`run-manifest.json` (run id, HEAD, digests of changed/untracked paths, config digests); every
record carries `run_id`; `release_gate.py` recomputes the candidate fingerprint (`--root`) and
accepts only this run's records; unreadable files and duplicate ids fail instead of being skipped
(also in `qualification_coverage_gate.py`); `regression_compare.py` is `BLOCKED` on a missing
baseline or manifest, missing environment snapshot, a baseline workload absent from the current
run, or zero effective comparisons; `baseline_promote.py` promotes only the gated run;
`canary_evaluator.py` is `BLOCKED` with no metric rules; `pgbench_runner.py` parses only the logs
of the current invocation.

| Check | Result |
|---|---:|
| Unit tests | PASS — 61/61 (18 new in `tests/test_run_binding.py`) |
| `selftest.py` | PASS, including the new `fail_closed_controls` negative controls |
| Sensitivity: new tests against the pre-change CLI scripts | 6 assertion failures (BLOCKED paths), 8 errors (old gate lacks run binding) |
| Rehearsal without `DATABASE_URL`, run twice in a temporary git repository | 26 evidence files, all stamped with the current `run_id`; first run archived to `superseded/`; gate `FAIL` only for `bad_status_or_count` and `metric_gate` |

Real PostgreSQL integration remains **BLOCKED** (no test instance in this environment).

## Local modification 2026-10-08 — static SQL gate

`sql_static_audit.py` (evidence `sql.static`, config `[sql_static]`, run by run_suite before the
schema gates) parses and lints SQL with SQLFluff in the configured dialect and lints migrations
with Squawk. Parse, lex and template errors (`PRS`, `LXR`, `TMP`), `fail_on` rule hits and
unexcluded Squawk findings FAIL; a missing tool, usage error, missing dialect, empty glob or a file
absent from the tool report is BLOCKED.

| Check | Result |
|---|---:|
| Unit tests | PASS — 72/72 run, 1 real-tool test skipped without `SQLVERIFY_SQLFLUFF_CMD` |
| Real-tool test (`uvx sqlfluff==4.4.0`, `uvx --from squawk-cli==2.68.0 squawk`) | PASS: `PRS` caught in a broken file, `AM04` gated, `require-concurrent-index-creation` raised on a migration |
| `selftest.py` | PASS |

## Local modification 2026-10-14 — gates that check nothing

Two more false PASS paths were reproduced through the real CLI with a run-bound `FAIL` record present:

| Reproducer | Before | After |
|---|---|---|
| `release_gate.py` with `{}`, empty sections, a misspelled section (`required_evidnce`) | `PASS`, exit 0 | `FAIL` (`empty_policy` / `invalid_policy`), exit 2 |
| `release_gate.py` with a misspelled selector field (`min_cont`) | field ignored, default `min_count` used | `FAIL` (`invalid_policy`), exit 2 |
| `release_gate.py` with a section given as an object instead of a list | traceback, exit 1, no report | `FAIL` (`invalid_policy`), exit 2, report written |
| `qualification_coverage_gate.py` with `{}`, `claims: []` or a misspelled `claim` key | `PASS`, exit 0 | `FAIL` ("declares no claims"), exit 2 |

Changes: `release_gate.py` checks the policy against `schemas/gate-policy.schema.json` (sections and
selector fields outside the schema, sections that are not lists) and requires at least one
`required_evidence` or `metric_rule` before evaluating; `qualification_coverage_gate.py` requires at
least one claim.

| Check | Result |
|---|---:|
| Unit tests | PASS — 75 run, 1 real-tool test skipped (2 new in `tests/test_run_binding.py`) |
| `selftest.py` | PASS |
| Sensitivity: new tests against the pre-change scripts | 7 assertion failures, 1 error (crash) across 8 sub-cases |
| Example policies (`examples/gate-policy*.json`) against the schema check | all conform |

