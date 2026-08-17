---
name: arc:build
version: 1.0.0
description: >
  Implements scoped code changes and verifies them. Use when the user says 写代码, 开发功能,
  落地改动, implement a known scheme, or deliver a scoped change. Not for unknown-failure
  repair or requirement clarification.
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

## Intent Router

| When | Load |
|---|---|
| Scoped implementation | this SKILL.md Workflow |
| Unclear scope or acceptance | `arc:clarify` |
| Failure/incident/failing check | `arc:fix` |
| Frontend/UI work | `arc:frontend` |
| Same-session multi-model routing | `arc:prewalk` |
| Large tracked work | `arc:sdlc` |
| Backend architecture | `arc:arch` |
| Lark-active delivery notes | `arc:docs` |

## Context Search

- MUST inspect existing code before editing unfamiliar files.
- MUST use `arc-idx search` first for broad repository context.
- MUST use `arc-idx ast` for structural patterns and `arc-idx symbol` for definitions when relevant.
- If `.lark.json` exists, MUST read it before final handoff.


## Red Lines

```text
NO CODE CHANGE WITHOUT SCOPE.
NO LARGE PROJECT CODE CHANGE WITHOUT CURRENT LOCAL TASK DOCS.
NO MULTI-MODEL COST ROUTING VIA PLAN-POSTCARD COLD HANDOFF.
NO DELIVERY WITHOUT VERIFICATION OR AN EXPLICIT BLOCKER.
NO LARK DELIVERY UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE TRACKED FEATURE COMPLETION WITHOUT task_base UPDATE.
NO DONE CLAIM FROM DOCS OR MATRIX ALONE.
NO WORK ON THE WRONG ENV/BRANCH/DEPLOY SURFACE.
```

## Hard Constraints

- MUST preserve unrelated user changes.
- MUST edit the smallest viable file set.
- MUST apply the gates in [`docs/execution-truth.md`](../../docs/execution-truth.md): runtime/env/branch surface, completion definition (code + gate + reachable behavior), scope lock, and no invented domain identity keys.
- MUST name the target surface (local / `.26` / `.31` / production / other) before deploy, package, or production diagnosis; MUST follow that surface's branch/track when the repo has dual tracks.
- MUST honor explicit scope locks (read-only, docs-only, frontend-only, no-restart, explicit non-goals) without opportunistic expansion.
- MUST NOT mark delivery complete from task checkboxes, ability-matrix cells, or second-hand "already fixed" claims without current verification.
- MUST apply `arc:sdlc` before code edits for large, multi-step, cross-module, or tracked work; task docs must be generated from the latest project state and updated immediately when project files, scope, assumptions, or status change.
- MUST apply `arc:prewalk` for same-session multi-model cost routing when the runtime supports mid-session model swap; MUST NOT default to “frontier writes plan only → cheap model cold-starts from the plan file”.
- MUST apply `arc:arch` before writing project code, including its default backend architecture, DIP, helper extraction, and ponytail preflight rules.
- MUST route frontend platform decisions through `arc:frontend`; defaults are Web = React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod, mobile = React Native + Expo + TypeScript + NativeWind + Zustand + TanStack Query + Expo Router, desktop = Tauri 2 + Web stack, mini-program = Taro 4 + React + TypeScript + Zustand, unless explicitly overridden by the user.
- MUST preserve the product-state contract across backend and frontend: empty/no-data is a successful business state, not an error; list/query endpoints return success with an empty collection and pagination metadata, while real failures return typed errors.
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

1. Confirm task, scope, verification target, and target env/branch/deploy surface ([`docs/execution-truth.md`](../../docs/execution-truth.md)).
2. For large, multi-step, cross-module, or tracked work, apply `arc:sdlc` before code edits and keep local task status current as the project changes.
3. Apply `arc:arch` before code edits; stop and report if ponytail is required but unavailable or conflicting.
4. Search for existing patterns, call sites, tests, and contracts.
5. If multi-model routing is available and useful, follow `arc:prewalk`: Guide orients + bounded todo + first production edit, then Executor continues in the same trajectory; otherwise oneshot a single profile.
6. Edit only the needed files.
7. Run targeted verification; broaden only when risk requires it.
8. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with feature/task title, owner, status, related requirement, files, verification, lifecycle link, and resource keys.
9. Summarize changes, verification, residual risk, and any prewalk swap/escalate notes. Mark complete only under the completion definition in execution-truth (code + gate + reachable behavior).

## Quality Gates

- Requested behavior is implemented without speculative extra surface.
- Large, multi-step, cross-module, or tracked work has current local task docs, detailed subtasks, and synchronized progress status from `arc:sdlc`.
- Existing contracts, names, state shapes, and response envelopes are preserved unless explicitly changed.
- Project architecture preserves DIP and the default backend architecture responsibilities from `arc:arch` when backend architecture applies.
- API/service contracts distinguish `empty`, `not found`, `permission denied`, validation failure, network/server failure, and loading/processing states; frontend consumers must not need to infer empty state from an error branch.
- Security-sensitive work checks authz, ownership, server-side amount/price computation, and secret handling.
- Data writes check business success, not just execution success.
- No placeholders, half-migrated call sites, or knowingly broken builds remain.
- Lark delivery status and `task_base` are recorded via `.lark.json` only when Lark is active.


## Scripts & Commands

Use project-native build, lint, test, typecheck, and migration commands. Use `Arc/scripts/verify-project.sh` and related guard scripts only when they fit the target project.

## Red Flags

- Editing before understanding existing patterns.
- Duplicating existing endpoints, helpers, formatters, or constants.
- Putting business logic in controllers, adapters, `main`, or cross-business `helpers`.
- Treating zero rows, empty search results, empty dashboards, or first-use setup as exceptions, failed requests, toast errors, or full-page error states.
- Adding speculative APIs or states.
- Implementing from stale task docs or leaving local progress status inconsistent with the actual project state.
- Building, diagnosing, or packaging against the wrong env/branch/deploy surface or dual-track sibling.
- Claiming done from matrix/docs while the path is unimplemented, gated-failing, or unreachable.
- Expanding past read-only / docs-only / frontend-only / no-restart / explicit non-goals.
- Using plan-document cold handoff as the multi-model “optimization” (double-pay for reads).
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
