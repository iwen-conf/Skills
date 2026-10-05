# Workload and representative-data standard

## Data is a performance input

Two databases with the same row count can produce radically different plans and contention because of distribution.

A representative profile should record or approximate:

- cardinality per predicate/join key;
- heavy hitters and hot rows;
- tenant/entity size percentiles;
- recency/time-window distribution;
- status/category distribution;
- NULL/empty frequencies;
- parent-child fan-out;
- cross-column correlations;
- history/soft-delete ratios.

Do not copy confidential production data by default. Prefer synthetic data whose statistical shape is derived from approved aggregates.

## Workload modes

### Steady-state SLO mode

Use a realistic arrival rate and verify latency/lag/error budget.

### Capacity sweep

Increase clients/rate in steps until tail latency, schedule lag or errors rise sharply. Record the knee/performance cliff rather than only the maximum TPS point.

### Hotspot contention

Use Zipfian/weighted keys plus fixed seeds to reproduce hot rows/counters.

### Soak

Run long enough to expose bloat, vacuum/checkpoint/WAL/temp/resource accumulation. Soak belongs on nightly/pre-release infrastructure, not every PR.

## Warmup and cache

Default benchmark is steady-state/warm-cache unless the business claim explicitly concerns cold start/cache.

Record warmup duration. Do not fake a cold-cache benchmark by casually dropping OS caches on shared systems.
