# Post-deploy Database Guardrails — Expert Final

## 1. Canary/control identity
- Release/change:
- Canary cohort / tenant / shard / region:
- Traffic or workload fraction:
- Control cohort:
- Minimum bake window:
- Minimum sample count:
- Automatic pause owner:
- Automatic rollback owner:

## 2. Database SLIs
| SLI | Control | Warning | Pause | Rollback | Source |
|---|---:|---:|---:|---:|---|
| query P95/P99 | | | | | |
| transaction P95/P99 | | | | | |
| lock-wait P95/P99 | | | | | |
| deadlocks / interval | | | | | |
| 40001 / interval | | | | | |
| DB error rate | | | | | |
| retry amplification | | | | | |
| active connections/utilization | | | | | |
| oldest transaction | | | | | |
| idle-in-transaction age | | | | | |
| temp bytes/files | | | | | |
| WAL bytes/rate | | | | | |
| checkpoint pressure | | | | | |
| cache/IO regression | | | | | |
| replica visibility lag | | | | | |
| CDC apply conflict/error | | | | | |
| replication slot retained WAL | | | | | |

## 3. Business reconciliation
| Invariant | Independent observer/query | Expected | Cadence | Pause/Rollback trigger | Repair path |
|---|---|---|---|---|---|

Examples: ledger conservation, inventory non-negative, exactly-once logical effect, enrollment capacity, outbox/inbox convergence, old/new schema equality.

## 4. Migration/backfill guardrails
- old reader / new writer:
- new reader / old writer:
- dual-write divergence:
- backfill throughput:
- backfill error/retry:
- replica lag amplification:
- lock impact:
- pause/resume mechanism:

## 5. Consistency guardrails
- read-your-writes violation counter:
- monotonic-read violation counter:
- stale-read budget:
- CDC duplicate/out-of-order metric:
- failover epoch/term fencing:

## 6. Containment
- Feature flag / kill switch:
- Traffic isolation / cohort removal:
- Rate reduction:
- Retry reduction/disable:
- DB migration rollback or forward-fix:
- Failback procedure:
- Restore/PITR escalation:
- On-call owner:

## 7. Evidence capture on incident
Capture queryid, plan, parameters class (not secrets), SQLSTATE, wait-for graph, transaction IDs where safe, WAL/replication state, release version, feature flags and the smallest reproducible history/schedule.

## 8. Exit criteria
Do not promote canary merely because no alert fired. Require minimum bake/sample plus affirmative business/database evidence. Record promotion decision and evidence IDs.
