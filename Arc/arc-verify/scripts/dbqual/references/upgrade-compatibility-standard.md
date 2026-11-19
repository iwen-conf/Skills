# Upgrade and compatibility qualification

For a major PostgreSQL, managed-service, extension, locale or platform change, test more than “server starts.”

Evidence should include as applicable:

```text
pg_upgrade --check or provider equivalent
application driver/protocol compatibility
extension presence/version compatibility
schema and generated expression behavior
collation version drift and reindex implications
prepared-plan behavior
representative workload replay
query-plan/performance comparison
logical/physical replication transition
backup/restore and rollback path
migration/cutover/failback plan
canary/progressive traffic
```

Locale/collation changes are data-ordering/index correctness concerns, not cosmetic environment differences.
