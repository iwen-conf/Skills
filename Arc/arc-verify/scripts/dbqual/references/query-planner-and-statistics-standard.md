# Query Planner, Statistics and Prepared-Plan Standard

## 1. Query identity includes parameter class

Do not certify `SELECT ... WHERE tenant_id=$1` using one tenant. At minimum classify:

```text
hot / cold key
small / medium / extreme tenant
recent / wide-history range
zero / one / many rows
high / low fan-out join key
NULL / non-NULL branches
common / rare status
incident-derived values
pagination beginning / deep page or cursor boundary
```

## 2. Dataset fidelity

Capture distributions and correlations that affect plans:

- table and index size;
- key cardinality and skew;
- MCV/top-key concentration;
- NULL fraction;
- time recency;
- parent-child fan-out;
- tenant size distribution;
- status distribution;
- cross-column dependence;
- soft-delete/history proportion;
- physical clustering/correlation when material.

Pure uniform random data is usually not representative.

## 3. Planner statistics

A large q-error can be a planner-statistics problem rather than a missing-index problem. Investigate:

- stale ANALYZE statistics;
- low statistics target;
- correlated predicates that need `CREATE STATISTICS` extended statistics;
- expression statistics/indexes where appropriate;
- parameter value classes;
- data distribution changes since baseline.

## 4. EXPLAIN evidence

Prefer JSON evidence and record, as supported by the server version:

```text
ANALYZE
BUFFERS
WAL
SETTINGS
MEMORY
TIMING FALSE when node timing is unnecessary
SUMMARY
```

Interpretation targets:

- estimated vs actual rows (q-error);
- loops and cumulative work;
- scan type/index identity;
- join type/order;
- rows removed by filter;
- temp read/write / sort spill;
- shared reads vs hits;
- WAL generation on writes;
- parallel workers;
- sort/hash memory behavior;
- planning vs execution time.

`EXPLAIN ANALYZE` has instrumentation overhead. Do not treat its execution time as a substitute for a workload benchmark.

## 5. Generic vs custom prepared plans

Production drivers often use prepared/extended protocol. PostgreSQL may choose custom plans for early executions and later use a generic plan when it estimates that to be worthwhile. Parameter-skewed queries can therefore behave differently in production than one-shot EXPLAIN tests.

For sensitive statements:

1. prepare the real statement with declared parameter types;
2. force custom and generic plans;
3. compare plan shape and estimated cost for named parameter classes;
4. separately benchmark using the same client protocol used in production (`prepared`, `extended`, etc.);
5. do not globally force custom plans as a first response—fix data/statistics/query/index design when possible.

## 6. Plan regression policy

Do not fail merely because a node type changed. A plan change is a diagnostic signal. Gate on business-relevant evidence such as:

```text
P95/P99 regression
unacceptable resource amplification
temp spill budget
large q-error
known-dangerous plan for a named parameter class
full scan of a table whose scale makes it unsafe
```

## 7. Query anti-patterns to inspect

- N+1 amplification at the request/job level;
- unbounded list/report queries;
- OFFSET deep pagination when scale makes it expensive;
- missing deterministic ORDER BY for pagination;
- expressions/casts defeating intended index usage;
- non-sargable predicates;
- huge `IN` lists or repeated point queries where a set operation is better;
- unnecessary `SELECT *` on wide/hot tables;
- lock-taking reads in broad scans;
- application-side filtering after excessive row retrieval;
- repeated correlated subqueries when joins/precomputation are more suitable.
