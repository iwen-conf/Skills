# Physical and Index Integrity Qualification

Logical application tests do not detect every storage/index corruption mode.

## 1. `amcheck` / `pg_amcheck`

PostgreSQL's `amcheck` facilities verify structural/logical invariants for supported relations and indexes. `pg_amcheck` can run these checks across a database. For routine/live checks, choose the least disruptive mode that addresses the risk; more thorough parent/root/heapallindexed checks can require stronger locks or significantly more work.

## 2. Gate policy

Physical integrity checks are typically:

```text
periodic
pre-release after storage/version/index-access-method changes
post-incident when corruption is suspected
before trusting a recovered clone for critical validation
```

They are not normally run on every PR.

## 3. Corruption response

If corruption is detected:

- preserve evidence;
- stop treating performance/correctness benchmark results from that instance as trustworthy;
- identify whether corruption is index-only or heap/storage-level;
- use documented PostgreSQL repair/rebuild/recovery procedures;
- investigate storage, memory, kernel/filesystem and database version history;
- add an incident regression/detection control.

Never turn an integrity error into an ignored warning just to make a release gate green.
