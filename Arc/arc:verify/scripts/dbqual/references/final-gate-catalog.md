# G0-G31 final qualification gate catalog

Every gate has one of two states in the qualification manifest:

```text
applicable = true   -> named Evidence is required
applicable = false  -> a concrete rationale is required
```

Missing is never interpreted as false.

| Gate | Domain | Core release question |
|---|---|---|
| G0 | Scope / ownership | Do we know exactly which DB-dependent behavior changed, who owns the risk and rollback? |
| G1 | Environment / safety | Is the test environment isolated, production-representative and fingerprinted enough for the claim? |
| G2 | Dataset fidelity | Does scale/skew/correlation/fan-out/history resemble the risk-bearing production shape? |
| G3 | Schema / data contract | Are constraints, keys, FKs, partition/default behavior and independent invariants sound? |
| G4 | Query correctness | Do named boundary parameters and property/metamorphic oracles establish result correctness? |
| G5 | Plan / resource diagnostics | Are plans, cardinality, buffers, temp, WAL and amplification understood for critical parameter classes? |
| G6 | Planner/statistics/prepared plans | Are stale stats, extended statistics needs and generic-vs-custom plan sensitivity covered? |
| G7 | Isolation anomalies | Can the chosen isolation level prevent or explicitly surface the anomalies the business cannot tolerate? |
| G8 | Static lock order | Do business operations define a consistent resource acquisition order without obvious cycles? |
| G9 | Deterministic concurrency | Have dangerous interleavings, lock waits and deadlock detection been forced deterministically? |
| G10 | Schedule exploration | Has a seeded portfolio explored additional contention schedules and persisted failing seeds? |
| G11 | Retry / idempotency | Do 40001/40P01/timeouts/ambiguous results retry the whole operation without duplicate durable/external effects? |
| G12 | Business workload benchmark | Are TPS, P50/P95/P99, failures, retries and per-SQL cost measured under a real workload mix? |
| G13 | Overload / backpressure | Where is the failure cliff, is overload graceful, and does the system recover after load falls? |
| G14 | Workload replay | Has a sanitized production-shaped session/timing/transaction trace been replayed with result oracles? |
| G15 | Soak / resource stability | Do long-running workloads avoid connection, lock, transaction, temp, WAL or memory/resource drift? |
| G16 | MVCC / maintenance | Are vacuum, freeze/XID, stale stats, dead tuples, idle transactions and slots within safety budgets? |
| G17 | WAL / checkpoint / I/O | Are write amplification, WAL growth, checkpoints, temp and I/O pressure within operational budgets? |
| G18 | Migration / backfill | Is DDL lock impact, expand-contract compatibility, backfill, reconciliation and rollback/forward-fix rehearsed? |
| G19 | Logical replication / CDC | Are apply workers, conflicts, lag and schema/replica-identity assumptions healthy? |
| G20 | Read replica consistency | Does the application meet its declared read-your-writes/monotonic/freshness contract under lag? |
| G21 | Physical integrity | Are checksums/amcheck/index/heap integrity checks performed at the appropriate cadence? |
| G22 | Backup verification | Is backup integrity checked and provenance/retention known? |
| G23 | Restore / PITR | Can the system actually restore to the required point with verified data and measured RPO/RTO? |
| G24 | Failover / failback | Are failover, fencing/split-brain protections, client recovery and failback rehearsed? |
| G25 | Upgrade / compatibility | Are major-version, extension, collation, protocol and application compatibility risks qualified? |
| G26 | Database security boundary | Are DB roles, privileges, RLS/SECURITY DEFINER/search_path and extension boundaries appropriate? |
| G27 | History / model checking | For high-integrity concurrency, has a generated operation history been checked against an explicit model? |
| G28 | Trusted baseline regression | Is the current evidence statistically and environmentally comparable to a provenance-checked baseline? |
| G29 | Canary / progressive rollout | Are representative, attributable canary metrics baked long enough with automatic stop/rollback rules? |
| G30 | Production reconciliation / incident regression | Can production detect/repair invariant divergence and have escaped incidents become permanent tests? |
| G31 | Evidence/toolchain trust | Did required tests really execute, are observers trustworthy, and is the report complete about BLOCKED/FLAKY gaps? |

## Rule

No gate is universally mandatory. Applicability is driven by the actual change and risk class. The system is deliberately strict about **declaring** applicability so important dimensions cannot disappear because nobody asked the question.
