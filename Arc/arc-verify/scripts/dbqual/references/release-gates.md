# Release gate design

Do not use one weighted score.

A P0/P1 claim must have its own required evidence. Missing critical evidence is `FAIL`/`BLOCKED`, not "98% passed".

## Absolute budgets

Use them for business SLO constraints:

```text
P95 <= X ms
P99 <= Y ms
error_rate <= Z
unexpected SQLSTATE count == 0
invariant failures == 0
```

## Relative budgets

Use them for stable-runner regression:

```text
P95 regression <= 20%
P99 regression <= 25%
TPS drop <= 15%
```

These percentages are examples, not universal defaults. Tune them to measurement variance and risk.

## Plan gates

Prefer gates on dangerous outcomes such as extreme cardinality estimate error or temp spill. Only forbid a plan node when the specific query contract provides a defensible reason.
