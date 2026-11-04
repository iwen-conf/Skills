# Business SQL parameter fuzzing

Random parameters are useful only after explicit cases and an oracle exist.

## Corpus order

Always preserve these first:

1. semantic boundaries;
2. hot/cold keys;
3. empty/one/many result cardinalities;
4. tenant-size extremes;
5. short/long time windows;
6. NULL/optional dimensions;
7. historical incident cases;
8. then seeded generated cases.

## Oracles

Examples:

- result rows satisfy filter predicates;
- ordering is total and stable under tie conditions;
- keyset pagination has no duplicates/gaps under the declared snapshot model;
- aggregate equals independent recomputation;
- retry/idempotency preserves final state;
- conservation/capacity invariants hold.

## Reproduction

Every generated failure must persist:

```text
seed
exact parameter tuple
query/version hash
data profile
PostgreSQL version/settings
```

Promote useful failures into named fixed cases instead of relying on rediscovery by fuzzing.
