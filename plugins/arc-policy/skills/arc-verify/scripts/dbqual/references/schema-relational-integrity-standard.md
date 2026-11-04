# Schema and relational integrity qualification standard

Schema review is both a correctness and operability gate. A schema that permits impossible business states or creates pathological write/read behavior is not production-ready even when individual SQL tests pass.

## Required questions

- What is the durable identity of each business entity?
- Which uniqueness rules are business truth rather than application convention?
- Which relationships must be enforced by FK, and which are intentionally external/eventual?
- Are FK write/delete paths supported by suitable indexes for the actual workload?
- Are constraints VALID, or is a NOT VALID -> VALIDATE lifecycle explicitly planned?
- Can sequences/identity values approach exhaustion?
- Do partitions cover the intended keyspace, and can the DEFAULT partition silently accumulate unexpected data?
- Are triggers/functions part of the business contract, and are they represented in the schema fingerprint?
- Can nullable/default behavior distinguish “unknown”, “not applicable”, and real zero values correctly?
- Are generated columns/collations/operator classes/version-specific features compatible with upgrades and replicas?

## Gate discipline

Do not universally require every table to have a surrogate primary key or every FK to have an index. The supplied default audit is intentionally strict because OLTP business tables usually benefit from these properties; project policy may explicitly relax them with rationale.

Schema audit results are hypotheses for review when the rule is workload-dependent. Independent business invariants remain authoritative for business correctness.

## Migration relationship

G3 describes the steady-state schema contract. G18 proves the system can safely move from old to new while live traffic, mixed versions and backfill are present. Passing one does not imply the other.
