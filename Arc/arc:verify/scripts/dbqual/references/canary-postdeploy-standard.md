# Canary and post-deploy database guardrails

Pre-production qualification reduces uncertainty; it does not eliminate production-only risk.

A database-sensitive canary should define before rollout:

```text
candidate/control population
minimum traffic/sample volume
minimum bake duration
attributable database metrics
business invariant/reconciliation metrics
pause threshold
rollback threshold
owner and maximum decision latency
```

Useful signals include:

```text
API/business success rate
DB P95/P99 and pool wait
40001 / 40P01 / lock timeout / statement timeout
lock-wait tail
connection utilization
WAL/temp/checkpoint changes
replica lag/read consistency failures
migration divergence
duplicate effects / illegal state transitions
reconciliation mismatch count
```

Do not automatically rollback an irreversible schema/data change unless rollback semantics have been proven. In those cases the containment action may be feature disable, traffic stop, dual-read fallback or forward-fix.

`canary_evaluator.py` compares control/canary metric samples and requires minimum bake/sample evidence. It is intentionally an evaluator, not a production deployment controller.
