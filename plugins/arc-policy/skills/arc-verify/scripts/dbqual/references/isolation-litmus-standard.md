# Isolation and anomaly qualification standard

Do not equate ACID with “all concurrency problems solved.” The qualification contract must name the anomaly that is forbidden and the mechanism that prevents or surfaces it.

Minimum portfolio when applicable:

```text
lost update / stale read-modify-write
write skew
check-then-act race
unique/exclusion conflict under concurrent key choice
phantom/range business rule
read-modify-write across multiple rows
concurrent UPSERT/MERGE ordering
predicate/range contention under SERIALIZABLE
```

For PostgreSQL, Repeatable Read and Serializable can produce `40001`; the application must retry the **complete** transaction logic. Serializable can prevent anomalies by aborting a transaction rather than making all schedules commit successfully.

A litmus test should record:

```text
isolation level
exact initial state
barrier/interleaving
per-session reads/writes
commit/abort outcome
SQLSTATEs
final invariant
whether retry was applied
```

The built-in `isolation_litmus_runner.py` provides standard write-skew and stale read-modify-write probes. Application-specific concurrency remains the stronger evidence.
