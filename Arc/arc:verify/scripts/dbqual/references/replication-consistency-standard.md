# Replica, Read-Routing and Consistency Qualification

## 1. Declare the consistency contract first

Examples:

```text
primary-only strong read after commit
async replica eventual visibility <= 2 s P99
session read-your-writes
monotonic reads
bounded staleness
synchronous remote-apply commit
```

Do not call a replica path “consistent” without naming what consistency means to the business operation.

## 2. Measure visibility, not only transport lag

For application-facing tests:

1. write a unique marker through the normal primary/write path;
2. commit;
3. read through the intended replica/read-routing path;
4. measure time until the write becomes visible;
5. once visible, verify monotonicity for the session/route contract;
6. record source/region/replica metadata when the platform exposes it.

WAL/LSN lag is useful diagnostics but application-visible staleness is the real oracle.

## 3. Physical PostgreSQL replication

Capture:

```text
sent/write/flush/replay LSN
write_lag / flush_lag / replay_lag
sync_state
replay conflicts
WAL receiver state
standby query cancellations
```

Remember that the lag fields are measurements of recent WAL handling/visibility delay, not a prediction of catch-up time.

## 4. Logical replication / CDC

Test:

- insert/update/delete propagation;
- schema-change compatibility;
- ordering assumptions;
- duplicate delivery/application idempotency;
- slot lag/WAL retention;
- restart/resubscribe behavior;
- initial snapshot plus ongoing changes;
- reconciliation between source and target.

## 5. Replica reads and Serializable assumptions

PostgreSQL documentation notes that Serializable integrity protection does not extend in the same way to hot standby/logical replicas. Do not use a primary-side Serializable assumption as proof that arbitrary replica reads satisfy the same application-level invariant.

## 6. Cross-region / managed read routing

Managed services may offer session tokens/bookmarks or explicit consistency modes. Treat those tokens as part of the application protocol and test loss/reuse/expiry/routing behavior. For example, Cloudflare D1 publicly documents a Sessions API/bookmark mechanism to provide sequential consistency with read replicas; the lesson is general: **consistency metadata must be preserved by the application if the contract depends on it**.

## 7. Replica qualification gates

Potential gates:

```text
visibility P99 <= contract
zero monotonic-read violations
zero stale reads for read-your-writes contract
replication-slot WAL retention <= budget
replica conflicts/query cancellations <= budget
failover promotion catches up within RPO/RTO
reconciliation mismatch == 0
```
