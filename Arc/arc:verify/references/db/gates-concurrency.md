# db gates: concurrency, retry and history (G7–G11, G27)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `C` = `.sqlverify/sql-verification.toml`, `F` = evidence file under
`$ART/logs/dbqual-evidence/`. Waivers: [gates-release.md](gates-release.md). Database runners
refuse names that miss `safe_database_pattern`. Scenario setup that deletes or truncates rows
needs approval.

Assertions in every section below are read-only scalar queries `{id, sql, op, expected}` with `op`
∈ `== != >= > <= <` (default `==`). Steps are `{op = "sql", sql, continue_on_error?}`,
`{op = "barrier", name}` (each barrier exactly once per session) or `{op = "sleep_ms", value}`.

## G7 — Isolation-level anomaly qualification

**Question.** Not "is it ACID?" but "which anomaly is forbidden, and does the chosen isolation level
or locking forbid it?"

**Cases:** stale read-modify-write / lost update, write skew, check-then-act, phantom/range
invariant, concurrent key generation unique conflict, multi-row invariant, UPSERT/MERGE
concurrency. Use standard litmus plus application-specific scenarios (G9). Record SQLSTATE and the
final invariant. SERIALIZABLE may satisfy the contract by aborting one transaction with `40001`;
then G11 retry evidence is mandatory.

**Config `[[isolation_litmus]]`:** `id`, `type` = `write_skew` (default) | `lost_update_detection`,
`isolation` = `read committed` | `repeatable read` | `serializable` (default), `timeout_s` 10,
`statement_timeout_ms` 5000, `min_final_on_call` (write skew), `expected_final_value` (lost update),
`allowed_sqlstates`, `required_sqlstates`, `max_serialization_failures`. The runner creates and
resets its own schema `sqlverify_litmus`.

```bash
python3 scripts/isolation_litmus_runner.py --config C --case ID --out F   # isolation.<id>, kind isolation_litmus
```

**PASS**: final invariant holds, every required SQLSTATE seen, none outside allowed, `40001` count
≤ budget. **FAIL**: any of those broken or unsupported isolation. **BLOCKED**: DSN/safety/error.

**References:** [isolation-litmus-standard](../../scripts/dbqual/references/isolation-litmus-standard.md),
[isolation-anomaly-catalog](../../scripts/dbqual/references/isolation-anomaly-catalog.md).

## G8 — Static lock-order design

Maintain a business transaction/resource catalog (`[lock_order_catalog] path = …`; JSON
`{schema_version: 1, operations: [{id, description, lock_order: [...]}]}`) and find order cycles.

```bash
python3 scripts/lock_order_analyzer.py --catalog .sqlverify/transactions/lock-order.json --out F   # lock-order.analysis
```

**PASS**: no cycle. **FAIL**: cycle (or runner error). `--allow-cycles` records cycles as warnings
only; never use it for a release gate. This is a design linter, not proof of deadlock freedom:
row/key order, indexes, triggers, FKs, predicates, UPSERT/MERGE and plans add dynamic waits (G9).

## G9 — Deterministic concurrency and deadlock scenarios

Force named interleavings with barriers/failpoints. **Evidence:** per-session steps and elapsed
time, SQLSTATE counts, `pg_stat_activity` wait state, `pg_blocking_pids()` wait-for edges,
same-snapshot wait-for cycle detection, lock-wait tail, final independent invariants.

Keep two types:

```text
deadlock-detector-selftest -> deliberately observe 40P01 and a real simultaneous wait cycle
production-contract test   -> normally zero avoidable 40P01 (policy metric observed.sqlstate_counts.40P01 <= 0)
```

**Config `[[concurrency_scenario]]`** (≥ 2 `session` tables with `name`, `step`, optional per-session
`allowed_sqlstates`/`required_sqlstates`; ≥ 1 `assertion`): `timeout_s` 15, `observer_sample_ms` 20,
`statement_timeout_ms` 15000, `lock_timeout_ms` 0, `deadlock_timeout_ms` 0 (unset),
`require_observer` true, `require_lock_wait` false, `require_wait_cycle` false,
`required_sqlstates`, `allowed_sqlstates`, `expected_deadlock_count`,
`expected_serialization_failure_count`, `max_deadlock_count`, `max_serialization_failure_count`,
`max_lock_wait_ms`. Observed: `sqlstate_counts`, `assertions_passed`, `max_lock_wait_ms`,
`wait_for_edges`, `wait_for_cycles`.

**Config `[[transaction_scenario]]`** (same shape, older layer): `timeout_s` 15,
`observer_sample_ms` 50, `statement_timeout_ms` max(1000, timeout_s×1000), `lock_timeout_ms` 0,
`require_observer` true, `require_lock_wait` false, `assertion_timeout_ms` 5000.

```bash
python3 scripts/concurrency_scenario_runner.py --config C --scenario ID --root . --out F   # concurrency.<id>
python3 scripts/transaction_scenario_runner.py --config C --scenario ID --root . --out F   # txn.<id>
```

**FAIL** when: a session hangs past `timeout_s`; the observer fails or captures no target samples
(with `require_observer`); required lock wait or wait cycle not seen; SQLSTATEs missing/unexpected
(global or per session); expected counts differ; max counts or lock wait exceeded; any assertion
fails. **PASS** otherwise. A silent observer is never PASS. Sample manifest: `min_pass` 2
concurrency + 1 transaction scenario.

**References:** [concurrency-deadlock-standard](../../scripts/dbqual/references/concurrency-deadlock-standard.md),
[transaction-and-lock-testing](../../scripts/dbqual/references/transaction-and-lock-testing.md).

## G10 — Seeded contention and schedule exploration

Repeat reproducible jitter/interleavings to find cases not named in G9. **Capture:** seed,
iteration, schedule/delays, SQLSTATEs, observer samples, final invariant, minimal failure artifact.
Promote each valuable discovery to a deterministic G9 regression.

**Config `[[contention_fuzz]]`:** `seed` 1, `runs` 100, `jitter_ms` 20, `statement_timeout_ms` 5000,
`iteration_timeout_s` 10, `max_deadlocks` 0, `fail_on_deadlock` true, `fail_on_serialization`
false, `allowed_sqlstates`, `max_failure_examples` 20, `setup_sql`, `reset_sql`, sessions
`{name, sql = [...]}`, assertions.

```bash
python3 scripts/contention_fuzz_runner.py --config C --fuzz ID --root . --out F   # fuzz.<id>, kind contention_fuzz
```

**FAIL**: any captured failing schedule (persisted with `schedule_ms`), `40P01` > `max_deadlocks`,
or runner error. **BLOCKED**: missing DSN or unsafe database. Policy samples gate
`observed.sqlstate_counts.40P01 <= 0`.

## G11 — Retry, idempotency and ambiguous result

`40001` retries the complete transaction decision logic; `40P01` retries when the application
contract permits. **Test whole-operation ambiguity:** failure before begin, failure during work,
serialization/deadlock abort, response lost after commit, connection reset around commit, process
restart before acknowledgment, retry racing the original, same key + same payload, same key +
conflicting payload, retry amplification/backoff/jitter. Oracle durable and external effects, not
exception codes.

**Config `[[retry_idempotency]]`:** `operation_command` argv (project driver), `attempts` ≥ 2
(default 2), `idempotency_key` (else random) exported via `idempotency_key_env`
(default `SQLVERIFY_IDEMPOTENCY_KEY`), `allowed_returncodes` `[0]`, `retry_delay_ms` 0,
`prepare_command`, `command_timeout_s` 60, `fault_command` + `fault_timeout_s` 60 (between attempts;
needs `--ack-isolated-environment`, else `BLOCKED`), assertions (sample: exactly one logical
charge). Commands run with cwd `--root`.

```bash
python3 scripts/retry_idempotency_runner.py --config C --case ID --root . --out F [--ack-isolated-environment]   # retry.<id>
```

**PASS**: every attempt exits in `allowed_returncodes`, fault command succeeds, assertions hold.
**FAIL** otherwise. Fault injection needs approval. Network ambiguity scenarios:
[gates-replication-recovery.md](gates-replication-recovery.md) (fault scenarios).

**References:** [idempotency-ambiguous-commit-standard](../../scripts/dbqual/references/idempotency-ambiguous-commit-standard.md),
[client-pooler-timeout-standard](../../scripts/dbqual/references/client-pooler-timeout-standard.md).

## G27 — Generated history and model checking

For DB-R4/DB-R5 concurrency where examples and fuzz are insufficient, capture operation histories
and check them against an explicit model with a project checker or Jepsen/Elle-style tooling
(installs need permission). **Persist:** history, model/version, fault schedule, checker
command/version, seed, counterexample/minimized history. Finite tests prove an anomaly exists;
they never prove all anomalies absent.

**Config `[[history_check]]`:** `history_file` (relative to `--root`), `checker_command` argv with
`{history}` placeholder, `expected_returncode` 0, `require_stdout_contains` list, `timeout_s` 900,
`model`, `checker_version` (pin it).

```bash
python3 scripts/history_checker_adapter.py --config C --case ID --root . --out F   # history.<id>, kind history_check
```

**PASS**: exit code as expected and every required stdout marker present. **FAIL**: otherwise.
**BLOCKED**: history file missing or checker error.

**References:** [history-model-checking](../../scripts/dbqual/references/history-model-checking.md).
