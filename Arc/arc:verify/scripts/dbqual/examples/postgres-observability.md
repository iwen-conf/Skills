# pg_stat_statements setup (benchmark/staging)

`pg_stat_statements` is recommended for per-query attribution.

In PostgreSQL configuration:

```conf
shared_preload_libraries = 'pg_stat_statements'
compute_query_id = auto
```

Restart PostgreSQL after changing `shared_preload_libraries`, then in the benchmark database:

```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

Optional for IO timing evidence:

```conf
track_io_timing = on
```

Do not change production configuration solely to satisfy the Skill without an operational review.
