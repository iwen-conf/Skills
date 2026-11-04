# Database Release Qualification Plan — Expert Final

## 1. Change identity
- Change / release:
- Owners:
- PostgreSQL version(s):
- Topology (primary/replica/CDC/proxy/pooler):
- Risk class: DB-R0 / DB-R1 / DB-R2 / DB-R3 / DB-R4 / DB-R5 / DB-R6
- Rollout strategy:
- Rollback / forward-fix owner:

## 2. Business claims and consequences
| Claim ID | Must be true | Forbidden outcome | Independent oracle | Consequence if violated | Runtime detector |
|---|---|---|---|---|---|

## 3. G0–G31 applicability
Every gate must be YES or NO. NO requires rationale. UNKNOWN is a release-planning defect for material risk.

| Gate | Qualification area | Applies | Required evidence IDs | Rationale if NO |
|---|---|---:|---|---|
| G0 | scope / risk / claims | | | |
| G1 | isolated environment / safety / tool trust | | | |
| G2 | representative dataset | | | |
| G3 | schema integrity | | | |
| G4 | query correctness / property | | | |
| G5 | EXPLAIN / IO / cardinality | | | |
| G6 | generic/custom plan sensitivity | | | |
| G7 | isolation litmus / anomaly | | | |
| G8 | lock-order graph | | | |
| G9 | deterministic concurrency/deadlock | | | |
| G10 | contention schedule exploration | | | |
| G11 | retry/idempotency/ambiguous commit | | | |
| G12 | workload benchmark | | | |
| G13 | overload/recovery | | | |
| G14 | workload replay | | | |
| G15 | soak/resource stability | | | |
| G16 | MVCC/vacuum/XID/slot health | | | |
| G17 | WAL/checkpoint/IO/temp health | | | |
| G18 | migration/backfill/rollback | | | |
| G19 | logical replication/CDC | | | |
| G20 | read replica consistency | | | |
| G21 | physical integrity | | | |
| G22 | backup verification | | | |
| G23 | restore/PITR | | | |
| G24 | failover/failback | | | |
| G25 | version/extension/collation compatibility | | | |
| G26 | DB security boundary | | | |
| G27 | history/model checking | | | |
| G28 | trusted baseline regression | | | |
| G29 | canary/control | | | |
| G30 | production reconciliation / incident regression | | | |
| G31 | evidence completeness / waiver / harness trust | | | |

## 4. Transaction and lock catalog
| Operation | Isolation | Shared resources | Acquisition order | Max txn duration | Retryable SQLSTATE | Idempotency key | Business oracle |
|---|---|---|---|---:|---|---|---|

## 5. Deterministic concurrency portfolio
| Scenario | Forced schedule/barriers | Expected waits | Required SQLSTATE | Forbidden SQLSTATE | Final-state oracle |
|---|---|---|---|---|---|

## 6. Isolation/anomaly contract
| Workflow | Isolation | Anomaly threatened | Expected prevention | Retry contract | Litmus evidence |
|---|---|---|---|---|---|

## 7. Dataset and parameter model
Document:
- scale and growth horizon;
- hot/cold keys and Zipf/skew;
- tenant-size distribution;
- time/status correlation;
- NULL/rare/extreme values;
- fan-out;
- historical incident cases;
- cardinality classes;
- anonymization/synthetic equivalence method.

## 8. Query and planner contract
| Query | Semantic oracle | Parameter classes | P95/P99 SLO | Allowed plan variability | Cardinality-risk threshold |
|---|---|---|---:|---|---:|

## 9. Workload / capacity / recovery contract
| Workload | Arrival model | Concurrency | P95 | P99 | Error/retry budget | Resource budget | Recovery budget |
|---|---|---:|---:|---:|---:|---|---|

## 10. Replay contract
- Trace source/sanitization:
- Timing scale:
- Session identity preserved:
- Transaction boundaries preserved:
- Cross-session dependency model:
- Result digest/oracle:
- Production-only operations removed/rebound:

## 11. Migration/backfill contract
- Expand/contract phases:
- Maximum lock impact:
- Old/new binary compatibility matrix:
- DDL timeout policy:
- Backfill rate / pause / resume:
- Dual-write divergence oracle:
- Reconciliation query:
- Rollback vs forward-fix:

## 12. Replication / CDC / consistency contract
- Read consistency semantics:
- Max visibility lag:
- Read-your-writes requirement:
- Monotonic-read requirement:
- CDC ordering/duplicate contract:
- Conflict handling:
- Slot/WAL retention budget:

## 13. Backup / restore / PITR / HA contract
- Backup source/type:
- Backup verification:
- Restore rehearsal cadence:
- PITR target semantics:
- RPO:
- RTO:
- Failover trigger:
- Failback strategy:
- Business-level restored-state assertions:

## 14. Security contract
- Runtime roles:
- Migration role:
- RLS scope:
- SECURITY DEFINER inventory:
- Extension allow-list:
- Cross-tenant negative tests:
- Backup/audit access boundary:

## 15. Canary and production guardrails
| Metric/invariant | Control baseline | Warning | Pause | Rollback | Observation source |
|---|---:|---:|---:|---:|---|

## 16. Execution profiles
- Developer required evidence:
- PR required evidence:
- Nightly required evidence:
- Pre-release required evidence:
- Post-deploy required evidence:

## 17. Waivers
| Gate/claim | Reason | Owner | Expiry | Containment | Runtime detection | Repair/rollback |
|---|---|---|---|---|---|---|

No missing evidence is PASS. No permanent waiver is assumed safe.
