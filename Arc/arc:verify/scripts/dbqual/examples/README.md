# Runnable reference example — v4 final

Use an isolated PostgreSQL database whose name matches the configured safe pattern (`*_bench`, `*_test`, or `sqlverify` in the sample).
Use the approved remote instance; a loopback address below denotes its SSH tunnel, not a local
server. Do not install or start PostgreSQL on the workstation. Replace example credentials and
ports with the project's credential reference and authorized tunnel.

```bash
export DATABASE_URL='postgresql://postgres:postgres@127.0.0.1:5432/sqlverify'
psql "$DATABASE_URL" -f examples/schema.sql
psql "$DATABASE_URL" -f examples/seed.sql
```

Quick repository example:

```bash
python3 scripts/run_suite.py \
  --config examples/sql-verification.toml \
  --root . \
  --timeout-s "${SQLVERIFY_TIMEOUT_S:?set the whole-suite budget from the task contract}" \
  --evidence-dir .sqlverify/evidence \
  --gate-policy examples/gate-policy-pr.json \
  --report-dir .sqlverify/report
```

The compact TOML demonstrates dataset oracles, EXPLAIN parameter classes, property testing, lock-order analysis, deliberate deadlock detection, production no-deadlock contract, isolation litmus, seeded contention, pgbench mix, overload/recovery, replay and migration rehearsal.
SQLVERIFY_TIMEOUT_S is a positive finite task budget, not a universal SLO. Evidence/report
directories must be dedicated owned outputs; the suite rejects candidate/input overlap before
archiving, retains partial BLOCKED evidence on a deadline/start failure and stops scheduling on
timeout. Report and baseline promotion revalidate the gated run; an all-n/a manifest proves no
runtime claim and returns NOT_RUN. A command-suite PASS does not replace a release gate.

The **expert pre-release manifest/policy is intentionally stricter than the compact sample configuration**. It requires project-bound evidence such as idempotency/ambiguous-result, soak, history/model checking and a trusted baseline. If those bindings are absent, the expert gate must fail. This is deliberate: an example package must not manufacture PASS evidence for infrastructure it does not own.

Additional fragments show how to bind:

```text
retry-idempotency.example.toml.fragment
logical-replication.example.toml.fragment
replica-consistency.example.toml.fragment
backup-restore.example.toml.fragment
failover.example.toml.fragment
compatibility.example.toml.fragment
history-check.example.toml.fragment
longrun-profile.toml.fragment
fault-scenario.example.json
canary-metrics.example.json
canary-policy.example.json
```

Thresholds are examples, not universal SLOs. Replace them with budgets derived from service SLOs, business consequence, production distribution and characterized benchmark-runner variance. Tight regression gates should use stable runners and provenance-checked baselines.
