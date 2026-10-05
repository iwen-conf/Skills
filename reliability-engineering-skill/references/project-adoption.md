# Project adoption standard

Do not copy the entire skill into every repository. Add only the project artifacts that carry durable domain/test knowledge.

Recommended layout when the project benefits from persistent reliability artifacts:

```text
.reliability/
  business/
    enrollment-cancel.toml
    order-lifecycle.toml
  bindings/
    enrollment-cancel.toml
  environments/
    integration.toml
  transactions/
    business-transactions.toml
  regressions/
    incidents.md
  evidence/              # CI artifact / normally not committed
  toolchain.lock.json    # when tool version reproducibility matters
```

For a specific change, create a short change verification plan (issue/PR artifact or `templates/change-verification-plan.md`) referencing stable business rules/scenarios rather than cloning the entire business plan.

## Adoption sequence

1. Use the repository's existing tests/CI/environment first.
2. Add a business plan only for domains whose rules/state/interactions are material enough to justify one.
3. Add executable bindings only for scenarios that should be automated at that layer.
4. Add a sandbox contract only when existing integration infrastructure needs a standard lifecycle adapter.
5. Add transaction catalogs/failpoints only around real shared-state/concurrency risk.
6. Add specialized tools only after a concrete failure model requires them.

The skill should reduce developer friction over time. If the verification architecture costs more to maintain than the incidents it prevents, simplify it.
