# db gates: schema, query and plan (G3–G6)

Entry point and verdict mapping: [../db.md](../db.md). Contract:
[../../../_shared/contract.md](../../../_shared/contract.md). Script paths are relative to the
package root; `C` = `.sqlverify/sql-verification.toml`, `F` = an evidence file under
`$ART/logs/dbqual-evidence/`. Waivers: [gates-release.md](gates-release.md). All runners below
refuse a database whose name misses `safe_database_pattern` (`BLOCKED`).

## G3 — Schema and durable data contract

**Question.** Are durable invariants enforced and checked independently of the code under test?

**Check** whichever the change touches: business uniqueness, primary keys, foreign keys, supporting
FK indexes when the workload needs them, `NOT VALID` constraints, invalid indexes,
sequence/identity exhaustion (also G16 `max_sequence_utilization`), partition and default-partition
growth, orphan reconciliation, `CHECK` constraints, schema assertions after backfill/migration.

**Config `[schema_integrity]`:** `enabled` (default true; false → `NOT_RUN`, exit 0),
`require_no_invalid_indexes`, `require_no_unvalidated_constraints`, `require_no_orphaned_sequences`
(flags default false), `max_tables_without_primary_key`, `max_unindexed_foreign_keys`,
`max_default_partition_rows` (unset = unchecked; sample 0/0/0), `[[schema_integrity.assertion]]`
read-only `{id, sql, op, expected}`; `op` ∈ `== != >= > <= <`.

**Config `[[invariant]]`:** `id`, `sql` or `sql_file`, `op`, `expected`, `description`,
`statement_timeout_ms` (default 10000). Invariants are the independent end-state oracle reused by
concurrency, workload and migration gates.

```bash
python3 scripts/schema_integrity_audit.py --config C --out F   # id schema.integrity, kind schema_integrity
python3 scripts/invariant_audit.py --config C --root . --out F # id invariant.audit, kind invariant_audit
```

**PASS**: no flag or limit violated, all assertions/invariants hold. **FAIL**: any violation.
**BLOCKED**: missing DSN, unsafe database, query error.

**References:** [schema-relational-integrity-standard](../../scripts/dbqual/references/schema-relational-integrity-standard.md),
[data-lifecycle-integrity-standard](../../scripts/dbqual/references/data-lifecycle-integrity-standard.md).

## G4 — Query correctness and parameter corpus

**Named classes first** (fuzz expands them, never replaces them): zero/one/many result, hot/cold
key, small/large tenant, recent/history range, NULL/boundary value, high fan-out, pagination
boundary, incident-derived case.

**Then property/metamorphic cases:** result count equals an independent `COUNT` oracle; equivalent
queries return equal multisets; a narrow predicate is a subset of the broad one; ordering and
pagination are unique under declared snapshot semantics; aggregation conservation; equivalence with
a reference implementation. Persist the seed and minimized failing inputs.

**Corpus.** `python3 scripts/parameter_corpus.py .sqlverify/params/<spec>.json --out <corpus>.json`.
Spec: `seed` (default 1), `fixed_cases[]` `{name, params, source}` emitted first, `parameters[]`
`{name, values, weights?}`, `random_runs` seeded weighted draws (sample `examples/property-corpus-spec.json`).

**Config `[[property_test]]`:** `id`, `kind` ∈ `exact_multiset` | `count_matches_rows` | `subset`,
`query_file`, `oracle_query_file`, `corpus_spec`, `param_style` (`pg_dollar` default | `psycopg`),
`max_cases` (default corpus size), `statement_timeout_ms` 5000, `max_failure_examples` 20.
Read-only.

```bash
python3 scripts/query_property_runner.py --config C --property ID --root . --out F   # property.<id>, kind query_property
```

**PASS**: every case satisfies the property. **FAIL**: any counterexample (recorded with params,
up to `max_failure_examples`). **BLOCKED**: DSN/safety/runner error. Add each counterexample as a
named fixed case before fixing.

**References:** [parameter-fuzzing](../../scripts/dbqual/references/parameter-fuzzing.md).

## G5 — EXPLAIN and resource diagnostics

**Capture** JSON EXPLAIN for critical parameter classes in the test environment:

```sql
EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS, FORMAT JSON, TIMING FALSE, SUMMARY TRUE)
```

`explain_capture.py` adds `MEMORY TRUE` on PostgreSQL 18 when `explain_memory` (default true) and
rolls back after execution. Write statements are refused unless `allow_write_explain=true`
(approval; isolated database only).

**Inspect:** node shape, estimated vs actual rows per loop, cumulative processed rows, temp spill,
shared/local reads/hits/writes, WAL, sort/hash batches, rows removed by filter, nested-loop
amplification, parallel workers, planning/execution diagnostics. Never ban Seq Scan or Nested Loop
universally; gate the business consequence.

**Config `[[query]]`:** `id`, `sql_file`, `param_style`, `allow_write_explain`, `explain_memory`,
`statement_timeout_ms` (default 30000); `[query.gates]` `max_exec_ms`, `max_estimate_error_ratio`
(q-error on per-loop actual rows), `max_temp_written_blocks`, `forbid_node_types`;
`[[query.parameter_set]]` `name` + `params_file`.

```bash
python3 scripts/explain_capture.py --config C --query QID --parameter-set PS --root . --out F   # explain.<qid>.<ps>, kind explain
python3 scripts/plan_analyzer.py F [--baseline BASELINE_EXPLAIN.json]                        # q-error, temp/shared/WAL sums, node types
```

**PASS**: every configured `[query.gates]` limit holds. **FAIL**: limit exceeded or forbidden node
present. **BLOCKED**: DSN/safety/runner error. Sample manifest requires `min_pass` 2 (two parameter
classes). EXPLAIN time is instrumented: confirm regressions with G12 latency.

**References:** [query-planner-and-statistics-standard](../../scripts/dbqual/references/query-planner-and-statistics-standard.md),
[observability-and-statistics](../../scripts/dbqual/references/observability-and-statistics.md).

## G6 — Planner statistics and prepared-plan sensitivity

**Test** parameter-sensitive statements under: named hot/cold classes, custom plan, generic plan,
auto plan-cache behavior when reproducible, fresh vs deliberately stale statistics when that is a
known failure mode, extended-statistics candidates for correlated columns. Use PostgreSQL 18
`EXPLAIN (GENERIC_PLAN)` where the target supports it.

**Config `[[plan_sensitivity]]`:** `id`, `query` (a `[[query]]` id), `param_types` (required, simple
type names, e.g. `["bigint", "timestamptz", "integer"]`), `parameter_sets` (names),
`max_generic_custom_cost_ratio` (default 10.0; sample 5.0), `warn_on_plan_change` (default true).
The runner uses `PREPARE` with `plan_cache_mode` = `force_custom_plan` / `force_generic_plan`.

```bash
python3 scripts/plan_sensitivity_runner.py --config C --case ID --root . --out F   # plan-sensitivity.<id>, kind plan_sensitivity
```

**PASS**: generic/custom estimated-cost ratio ≤ limit for every set. **FAIL**: ratio exceeded.
Plan-shape change is a warning when `warn_on_plan_change`. **BLOCKED**: DSN/safety, missing
`param_types`, runner error.

**References:** [query-planner-and-statistics-standard](../../scripts/dbqual/references/query-planner-and-statistics-standard.md).
