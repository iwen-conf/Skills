# Report contract v2

A report is evidence navigation, not marketing prose.

It must distinguish:

- **Measured fact** — directly present in Evidence JSON.
- **Gate decision** — explicit policy applied to evidence.
- **Hypothesis** — plausible explanation still requiring falsification.
- **Recommendation** — proposed engineering action.
- **Limitation** — missing, blocked, flaky, non-comparable or inapplicable evidence.

A report must never turn `Seq Scan`, `Nested Loop`, temp spill, 40P01, 40001 or high P99 into a root-cause conclusion by itself.

## Required sections for full qualification

1. Executive gate result
2. Scope / risk / applicability
3. Environment and baseline comparability
4. Dataset fidelity
5. Query/property correctness
6. Workload latency/TPS/error/retry and per-query cost attribution
7. Query-plan/cardinality/IO/temp/WAL evidence
8. Static lock-order analysis
9. Deterministic concurrency/deadlock/serialization evidence
10. Seeded contention/schedule exploration
11. Production-shaped workload replay and result oracles
12. Overload/failure-cliff/recovery evidence
13. Migration/rollback evidence
14. Fault/recovery evidence where applicable
15. Long-run resource evidence where applicable
16. Statistical baseline regression
17. Blocking findings
18. Non-blocking warnings
19. Reproduction inputs/seeds/hashes
20. BLOCKED/NOT_RUN/FLAKY and other untested risk
21. Canary/reconciliation/rollback controls for pre-release reports

Every blocking finding must point to a stable evidence ID or explicit missing required evidence.

## Completeness wording

A report may be complete as an **audit record** even when qualification failed. It must not say the system is fully verified when an applicable critical gate is BLOCKED or absent.
