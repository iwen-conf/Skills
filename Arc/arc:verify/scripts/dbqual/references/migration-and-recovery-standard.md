# Migration, recovery and ambiguous-result standard

## Schema/data migration rehearsal

For material migrations verify:

- representative pre-state;
- lock behavior under concurrent traffic;
- apply time and statement/lock timeout behavior;
- post-state correctness;
- application compatibility window;
- rollback or forward-fix path;
- backfill correctness and resumability;
- dual-write/dual-read reconciliation when used;
- index validity and constraint state;
- long transaction interaction;
- replica/WAL impact when applicable.

`migration_rehearsal.py` supports apply, a lock probe, post assertions, rollback and rollback assertions on a disposable PostgreSQL environment.

A migration that is expected to require an intrusive lock can still pass rehearsal if the risk is explicit and the rollout plan contains an acceptable maintenance/traffic-control strategy. Do not relabel expected blocking as “nonblocking.”

## Incremental online migration

For high-volume migrations prefer an expand/migrate/contract pattern:

1. introduce compatible schema;
2. begin dual write or compatibility path;
3. backfill incrementally with throttling/checkpoints;
4. continuously reconcile old/new representations;
5. switch reads gradually;
6. observe;
7. remove old path only after confidence and rollback window.

## Ambiguous commit

The dangerous case is not only an explicit database error. A client may lose its connection after COMMIT reached the database but before the response reached the client. Test the application contract under this uncertainty:

- retry with same idempotency key;
- determine transaction outcome when possible;
- ensure no duplicate durable or external side effect;
- reconcile if outcome remains ambiguous.

Use `fault_scenario_runner.py` with a real application integration test and a network fault mechanism such as Toxiproxy in an isolated environment.

## Backup/PITR

For systems where recovery is a release concern, test restore/PITR separately. A successful backup command is not evidence that the backup is restorable. The gate should include restored data invariants and, when relevant, target recovery point/time objectives.
