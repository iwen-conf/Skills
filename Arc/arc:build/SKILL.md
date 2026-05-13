---
name: arc:build
description: "代码交付：当方案和范围已明确，需要实施代码变更、运行验证并说明结果时使用；不负责总控编排。"
---

# arc:build

## Overview

`arc:build` is the lean implementation skill. It guides concrete code changes, verification, and handoff. It does not own planning infrastructure, indexes, E2E frameworks, or release gates.

## Quick Contract

- **Trigger**: The task is implementation-ready and code or project files should be changed.
- **Inputs**: Clear task, scope, target repository, constraints, and expected verification.
- **Outputs**: Code changes, verification evidence, and a concise change summary.
- **Quality Gate**: The change is scoped, verified, and explainable.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` first if scope or acceptance criteria are unclear.
- Use `arc:fix` instead when the primary input is a failure report or incident.
- Use `arc:audit` after implementation if the user wants a read-only quality review.
- Use `aitask` for cross-agent task ownership or OpenViking memory updates.

## Announce

Begin by stating clearly:
"I am using `arc:build` to implement the scoped change and verify it."

## The Iron Law

```text
NO CODE CHANGE WITHOUT SCOPE AND VERIFICATION.
```

## Workflow

1. Inspect the relevant files and existing patterns.
2. State the implementation approach if the change is non-trivial.
3. Edit the smallest viable set of files.
4. Run targeted verification first, then broader checks when appropriate.
5. Summarize changed files, behavior, verification, and residual risks.

## Quality Gates

- Scope remains tied to the requested outcome.
- Existing user changes are not reverted without explicit instruction.
- Verification commands are run when feasible; failures are reported with cause.
- The final summary distinguishes facts, assumptions, and follow-up risk.

## Expert Standards

- Definition of Done (`DoD`) is explicit for behavior, tests, and documentation.
- Version-impacting changes consider `SemVer` compatibility.
- Contract-sensitive changes include `Contract Test` or equivalent verification when practical.
- Reliability-sensitive changes mention `RTO/RPO` implications when relevant.
- Dependency or packaging changes consider `SBOM`/supply-chain impact.

## Scripts & Commands

No dedicated Arc runtime scripts. Use the project's own build, lint, test, and typecheck commands.

## Red Flags

- Editing before understanding existing patterns.
- Expanding scope opportunistically.
- Skipping verification without saying why.
- Suppressing type, lint, or test failures instead of fixing root causes.

## When to Use

- **Preferred Trigger**: The user asks to implement a known change or approved plan.
- **Typical Scenario**: Feature work, refactor, migration, documentation sync, or small project automation.
- **Boundary Tip**: Use `arc:fix` for failure-first work and `arc:clarify` for underspecified work.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `task` | string | yes | Implementation goal |
| `scope` | string | no | Files/modules expected to change |
| `verification` | string | no | Expected test/lint/build command |

## Outputs

```text
Build Handoff
- What changed
- Files touched
- Verification run
- Residual risks
- Suggested next step, if any
```
