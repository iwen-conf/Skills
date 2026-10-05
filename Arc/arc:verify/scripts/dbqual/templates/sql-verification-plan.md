# SQL Verification Plan

## Scope

- Change / release:
- Business operations affected:
- PostgreSQL versions:
- Risk owner:

## Business claims

| Claim ID | Business rule / SLO | Consequence if wrong | SQL / transaction path | Required evidence |
|---|---|---|---|---|

## Dataset profile

Document row count, skew, hot keys, tenant sizes, recency, correlations, fan-out and the SQL checks that prove the profile exists.

## Query matrix

| Query ID | Parameter case | Expected cardinality | SLO | Plan risks | Evidence ID |
|---|---|---:|---:|---|---|

## Workload matrix

| Workload | Mode | Clients | Rate | Duration | Repetitions | Seed | Budgets |
|---|---|---:|---:|---:|---:|---:|---|

## Concurrency scenarios

| Scenario | Operations | Forced interleaving | Expected SQLSTATE | Final-state oracle |
|---|---|---|---|---|

## Baseline

- Baseline ID/path:
- Environment comparability requirements:
- Allowed regression:

## Release gate

List required evidence IDs and metric rules. Do not use an aggregate score.

## Limitations

State risks that are not covered by this run and why.
