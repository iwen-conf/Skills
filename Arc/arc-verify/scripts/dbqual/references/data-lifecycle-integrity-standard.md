# Schema and data lifecycle integrity standard

A query can be fast while the relational model is accumulating latent operational failure.

Audit when applicable:

```text
primary/unique keys that encode actual business identity
foreign-key referential integrity
supporting indexes for high-impact foreign keys
NOT VALID / invalid indexes left behind after migrations
sequence/identity exhaustion
partition lifecycle and unexpected DEFAULT-partition growth
soft-delete/history accumulation
nullable/default semantics
constraint validation after backfill
large-table rewrite risk
orphaned data and reconciliation queries
```

An “unindexed foreign key” is a risk indicator, not an automatic defect; low-write tables or uncommon parent deletes may not need one. Gate it only when the workload contract justifies the requirement.
