# CI / release qualification operating model v2

## PR lane

Objective: fast, deterministic evidence for changed claims.

Typical contents:

- config/schema/self-test;
- representative fixture + dataset distribution Oracle;
- static lock-order graph;
- changed query/property cases;
- relevant deterministic concurrency scenarios;
- migration lock probe if DDL changed;
- short representative benchmark where useful;
- claim-based gate.

A noisy shared runner may be suitable for correctness and concurrency evidence but should not be silently treated as a trusted tight performance baseline.

## Nightly lane

Add state-space and capacity exploration:

- all critical parameter classes;
- seeded contention/schedule exploration;
- wider client/rate matrices;
- overload and recovery;
- longer resource monitoring;
- broader generated corpus;
- selected fault tests where the isolated harness is available.

## Pre-release / stable performance lane

Use a controlled environment with known PostgreSQL/settings/data/storage/CPU profile.

Add:

- sanitized production-shaped replay;
- transaction/dependency/result-oracle replay evidence;
- migration apply + rollback/forward-fix rehearsal;
- ambiguous-commit/network/restart/failover tests when applicable;
- longer soak;
- trusted-baseline comparison with repeated runs and uncertainty;
- backup/restore/PITR rehearsal when storage/recovery changes;
- canary/reconciliation/rollback plan.

## Post-deploy lane

Pre-production evidence does not prove absence of unknown failures. High-risk systems should use:

- canary cohorts / gradual exposure;
- query/tail/lock/error SQLSTATE guardrails;
- business invariant reconciliation;
- duplicate-side-effect detection;
- migration dual-state divergence checks;
- automatic pause/rollback where the signal is reliable.

## Baseline governance

Do not automatically bless a slower run as the new baseline. Baseline replacement must be deliberate, traceable, integrity-checked and tied to a known accepted change.

## Evidence retention

Retain enough artifacts to reproduce or explain a release decision:

```text
config and gate policy
exact workload/query/trace hashes
Evidence JSON
report
baseline manifest
seed/schedule/failure corpus
PostgreSQL/settings/schema fingerprints
CI run identifier / commit
```
