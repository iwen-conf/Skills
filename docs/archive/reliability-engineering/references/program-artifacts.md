## 2. Standard work products

For initial framework adoption define four logical artifacts; a scoped change reuses existing artifacts and creates only missing bindings/evidence. Read-only review does not initialize files.

### A. Business verification plan

Describes **what must be true**:

```text
objective/scope
actors and authority
business rules
state model when applicable
side-effect semantics when applicable
primary journeys
expected outcomes
forbidden outcomes
exception/retry/cancellation semantics
cross-business interactions
compatibility/time/migration dimensions when applicable
business oracles
release-required scenarios
```

Use [templates/business-test-plan.md](../templates/business-test-plan.md) and, for machine checking, a TOML plan based on [examples/business-test-plan.toml](../examples/business-test-plan.toml).

Validate:

```bash
python3 "$SKILL_ROOT/scripts/business_test_plan.py" validate --plan business-test-plan.toml
```

Render for review:

```bash
python3 "$SKILL_ROOT/scripts/business_test_plan.py" render \
  --plan business-test-plan.toml \
  --out business-test-plan.md
```

### B. Execution bindings

Describes **how this repository exercises a business scenario** without contaminating business rules with implementation detail.

Examples:

```text
HTTP/gRPC request
background job invocation
real SQL assertion
message/event observation
device command/harness
repository test binary
```

Use [examples/business-scenario-bindings.toml](../examples/business-scenario-bindings.toml) as a pattern.

Validate:

```bash
python3 "$SKILL_ROOT/scripts/business_scenario_runner.py" validate \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml
```

Run a scenario:

```bash
python3 "$SKILL_ROOT/scripts/business_scenario_runner.py" run \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml \
  --scenario S-002 \
  --evidence-out .reliability/evidence/S-002.json
```

### C. Environment contract

Describes how tests run in an isolated, production-representative environment:

```text
bootstrap
reset
healthcheck
teardown
traffic/data isolation
required environment variables
```

Prefer the project's existing Compose/Testcontainers/dev-stack. The skill is an adapter, not a parallel platform.

Use [examples/sandbox.toml](../examples/sandbox.toml) as a pattern.

Example:

```bash
python3 "$SKILL_ROOT/scripts/sandbox_runner.py" \
  --contract sandbox.toml \
  --root . \
  --evidence-out .reliability/evidence/sandbox.json \
  -- python3 <skill>/scripts/business_scenario_runner.py run ...
```

The runner never invokes a shell and always attempts teardown.

### D. Evidence

Evidence is produced by actual execution, not by plan existence.

Record, as applicable:

```text
scenario/claim ID
tool and version
exact command
seed/schedule/corpus
status
elapsed time
observed state/results
failure output
reproduction instructions
```

Release-required business scenarios must be traceable to evidence or an explicit manual/analysis procedure.

Cross-check plan -> binding -> evidence:

```bash
python3 "$SKILL_ROOT/scripts/business_verification_gate.py" \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml \
  --evidence-dir .reliability/evidence \
  --require-evidence
```

This gate produces no aggregate quality score. A missing required P0/P1 scenario cannot be hidden by many passing low-risk tests.

## 2.5 Project adoption and per-change planning

Do not make every feature author rewrite the full domain test plan. Keep stable business rules/journeys in durable project artifacts, and create a small change verification plan for each material change.

Recommended persistent layout is documented in [references/project-adoption.md](../references/project-adoption.md). Use [templates/change-verification-plan.md](../templates/change-verification-plan.md) for a PR/feature/release when useful.

The per-change plan should answer only:

```text
which business rules changed
which existing journeys/states/side effects can observe the change
which scenarios are required for this change
which extra probes are new
what evidence blocks release
what rollout/rollback controls apply
```

This keeps the system maintainable: stable domain knowledge is reused, while change-specific work remains small.

