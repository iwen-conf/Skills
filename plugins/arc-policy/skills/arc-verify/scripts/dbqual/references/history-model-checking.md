# History / model checking for high-integrity concurrency

Deterministic barriers and seeded contention are not exhaustive. For DB-R4/DB-R5 systems, especially replicated/failover-sensitive state, consider a history/model checker when the state model is tractable.

Suitable targets include:

```text
account/ledger transfers
inventory reservation
quota/capacity
idempotent command processing
leader/failover-sensitive writes
read-after-write / monotonic-read contracts
```

A useful history records at least:

```text
logical operation ID
client/session ID
invoke/completion times
input
observed output
commit/abort/unknown result
retry/idempotency key
relevant version/token/bookmark
```

The checker should evaluate an explicit model, not merely search logs for errors. Jepsen/Elle-style dependency analysis is appropriate when its model matches the system. A detected anomaly is strong evidence that a claim is false; a finite test run is not proof that no anomaly exists.

If an external checker is used, preserve as release Evidence:

```text
checker/tool version
model/spec version
history hash
seed/nemesis/fault schedule
result
minimal counterexample when available
raw checker artifact
```

Do not run distributed fault tooling against production merely to satisfy a gate. Use an isolated, production-representative environment.
