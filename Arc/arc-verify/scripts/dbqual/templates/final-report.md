# Database Release Qualification Report — Expert Final

## 1. Executive release decision
## 2. Change scope, risk class and business claims
## 3. G0–G31 applicability and evidence coverage
## 4. Test environment, isolation, tools and provenance
## 5. PostgreSQL version/settings/schema/dataset comparability
## 6. Dataset fidelity and parameter-space coverage
## 7. Schema integrity and relational design hazards
## 8. Query correctness / property / metamorphic evidence
## 9. EXPLAIN / cardinality / buffers / temp / WAL evidence
## 10. Prepared-plan / generic-custom / parameter sensitivity
## 11. Isolation-level litmus and anomaly evidence
## 12. Static lock-order analysis
## 13. Deterministic concurrency / wait-for graph / deadlock evidence
## 14. Seeded contention schedule exploration
## 15. Retry / idempotency / ambiguous-result evidence
## 16. Workload benchmark matrix — P50/P95/P99/TPS/errors/retries
## 17. pg_stat_statements / IO / WAL / checkpoint attribution
## 18. Overload / failure cliff / recovery qualification
## 19. Production-shape workload replay and result comparison
## 20. Long-run / soak / leak / queue / transaction-age behavior
## 21. MVCC / autovacuum / XID / statistics / slot health
## 22. Migration / backfill / compatibility / rollback rehearsal
## 23. Logical replication / CDC qualification
## 24. Replica visibility / read consistency qualification
## 25. Physical integrity / checksum / amcheck evidence
## 26. Backup artifact verification
## 27. Restore / PITR rehearsal and business-level restore oracle
## 28. Failover / failback / RTO / RPO evidence
## 29. PostgreSQL / extension / collation / upgrade compatibility
## 30. Database security / role / RLS / SECURITY DEFINER evidence
## 31. Transaction history / model-checker evidence
## 32. Statistical trusted-baseline regression
## 33. Canary/control and post-deploy guardrails
## 34. Production reconciliation / incident-regression controls
## 35. Evidence inventory, hashes, seeds, schedules and traces
## 36. Blocking findings
## 37. Non-blocking warnings / observations
## 38. Waivers and expiry
## 39. Reproduction instructions
## 40. Limitations / BLOCKED / NOT_RUN / FLAKY risks
## 41. Remediation hypotheses and owner/action mapping

Rules:
- Separate measured facts from interpretations and remediation hypotheses.
- Missing metrics are not zero.
- Framework self-test is not database qualification evidence.
- A high-risk missing Gate cannot be hidden by aggregate scores.
- Report absolute values, distributions and confidence where relevant; do not rely on averages alone.
