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
