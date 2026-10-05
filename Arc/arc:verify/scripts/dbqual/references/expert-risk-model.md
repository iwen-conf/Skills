# Expert database risk model

Risk class determines the default depth, not the final answer.

## DB-R0 — local read-only

Read-only low-frequency behavior with no schema/index/transaction impact. Typical gates: G0-G6, G31.

## DB-R1 — performance-critical query/index

High-frequency read paths, joins, search/list endpoints, indexes or planner-sensitive work. Add G12, G28; G13 when capacity matters.

## DB-R2 — mutable/concurrent state

Writes, UPSERT/MERGE, row locks, counters, capacity, cancellation races, worker/user races. Add G7-G11, plus workload evidence.

## DB-R3 — schema/data lifecycle

DDL, repartitioning, type changes, backfills, large cleanup, data model cutover. Add G18 and usually G14/G15/G28.

## DB-R4 — high-integrity business truth

Money, ledger, inventory, entitlements, quotas, billing, idempotency, outbox/inbox. Require strong independent invariants, G7-G11, G27 when tractable, G30 reconciliation, and fault/recovery evidence.

## DB-R5 — replication / HA / disaster recovery

Read replicas, logical replication/CDC, failover, backup/PITR, cross-region routing. Add G19-G24 and consistency/RPO/RTO contracts.

## DB-R6 — platform / major upgrade / storage-critical

Major PostgreSQL upgrades, storage/filesystem changes, extension upgrades, locale/collation changes, managed-service migration or DB platform replacement. Require G14, G18, G21-G25, G28-G31 and a progressive rollout plan.

## Escalation signals

Escalate one or more classes when any of these apply:

```text
irreversible write
large blast radius
high cardinality shared state
hot key / hotspot
unknown production distribution
external side effect inside/near transaction
ambiguous commit risk
cross-region or asynchronous replication
online migration with mixed old/new application versions
historical incident in the same mechanism
recovery mechanism never rehearsed
```
