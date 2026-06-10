---
name: arc:build
description: "Code delivery with verification; hand active Lark task_base, progress, delivery, and lifecycle to arc:docs."
---

# arc:build

## Overview

`arc:build` implements scoped project changes and verifies them. It does not clarify vague work, fix unknown failures, or update Lark resources directly.

## Quick Contract

- **Trigger**: The task is implementation-ready and files should change.
- **Inputs**: Project path, task, scope, constraints, expected verification.
- **Outputs**: Code changes, verification evidence, risks, and optional Lark handoff.
- **Quality Gate**: The change is minimal, verified, and explainable.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` first if scope or acceptance criteria are unclear.
- Use `arc:fix` when the primary input is a failure, incident, or failing check.
- Use `arc:frontend` for frontend baseline or UI lifecycle work.
- Use `arc:docs` only when Lark is active for delivery notes, `task_base` feature status, progress Base, Drive artifacts, or `.lark.json.lifecycle[]`.
- Use `arc:audit` after delivery for read-only review.

## Context Search

- MUST inspect existing code before editing unfamiliar files.
- MUST use `.ai-code-index/search.sh` first for broad repository context.
- MUST use `.ai-code-index/struct-search.sh` for structural patterns and `.ai-code-index/symbols.sh` for definitions when relevant.
- If `.lark.json` exists, MUST read it before final handoff.

## Announce

Begin by stating clearly:
"I am using `arc:build` to implement the scoped change and verify it."

## The Iron Law

```text
NO CODE CHANGE WITHOUT SCOPE.
NO DELIVERY WITHOUT VERIFICATION OR AN EXPLICIT BLOCKER.
NO LARK DELIVERY UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE TRACKED FEATURE COMPLETION WITHOUT task_base UPDATE.
```

## Hard Constraints

- MUST preserve unrelated user changes.
- MUST edit the smallest viable file set.
- MUST run targeted verification when feasible.
- MUST report failed or skipped verification.
- MUST route all Lark writes through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST hand off to `arc:docs` after every Lark-active tracked feature update so `task_base` records title, owner, status, related requirement, files, verification, lifecycle link, and `updated_at`.
- MUST NOT claim a Lark-active tracked feature is complete until `task_base` is updated or the blocker is explicit.
- NEVER broaden scope opportunistically.
- NEVER suppress type, lint, test, or runtime failures to claim completion.
- NEVER trust user-controlled SQL, payment, ownership, role, amount, sort field, or identifier without server-side validation.

## Workflow

1. Confirm task, scope, and verification target.
2. Search for existing patterns, call sites, tests, and contracts.
3. Edit only the needed files.
4. Run targeted verification; broaden only when risk requires it.
5. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with feature/task title, owner, status, related requirement, files, verification, lifecycle link, and resource keys.
6. Summarize changes, verification, and residual risk.

## Quality Gates

- Requested behavior is implemented without speculative extra surface.
- Existing contracts, names, state shapes, and response envelopes are preserved unless explicitly changed.
- Security-sensitive work checks authz, ownership, server-side amount/price computation, and secret handling.
- Data writes check business success, not just execution success.
- No placeholders, half-migrated call sites, or knowingly broken builds remain.
- Lark delivery status and `task_base` are recorded via `.lark.json` only when Lark is active.

## Expert Standards

- Definition of Done (`DoD`) covers behavior, tests, and documentation.
- Compatibility-impacting changes consider `SemVer`.
- Contract-sensitive changes include a `Contract Test` or equivalent check when practical.
- Reliability-sensitive changes mention `RTO/RPO` only when actually relevant.
- Dependency changes consider `SBOM` and supply-chain risk.

## Scripts & Commands

Use project-native build, lint, test, typecheck, and migration commands. Use `Arc/scripts/verify-project.sh` and related guard scripts only when they fit the target project.

## Red Flags

- Editing before understanding existing patterns.
- Duplicating existing endpoints, helpers, formatters, or constants.
- Adding speculative APIs or states.
- Skipping verification silently.
- Updating Lark delivery resources directly instead of through `arc:docs`.
- Completing a Lark-active feature while the `task_base` row is missing or stale.

## When to Use

- **Preferred Trigger**: The user asks to implement a known change or approved plan.
- **Typical Scenario**: Feature work, refactor, migration, documentation sync, or small automation.
- **Boundary Tip**: Use `arc:fix` for failure-first work and `arc:clarify` for underspecified work.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `task` | string | yes | Implementation goal |
| `scope` | string | no | Expected files or modules |
| `verification` | string | no | Expected test, lint, build, or typecheck |

## Outputs

```text
Build Handoff
- What changed
- Files touched
- Verification run
- Residual risks
- Lark / .lark.json / task_base handoff, if applicable
```
