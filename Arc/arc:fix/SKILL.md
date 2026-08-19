---
name: arc:fix
version: 1.0.0
description: >
  Diagnoses and repairs failures from logs, stack traces, failing tests, or incidents. Use
  when the user says 报错, 修复, 故障, bug, 根因, incident, or failing test. Not for greenfield
  feature delivery.
---
# arc:fix

## Overview

`arc:fix` repairs evidence-backed failures. It starts from a failing signal, identifies root cause, applies the smallest fix, and verifies the failing path.

## Quick Contract

- **Trigger**: A bug, incident, regression, failing test, crash, or broken flow has evidence.
- **Inputs**: Failure signal, expected behavior, project path, optional verification command.
- **Outputs**: Root cause, fix, verification, risk, and optional Lark incident handoff.
- **Quality Gate**: The original failure is explained and rechecked, or the blocker is explicit.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Intent Router

| When | Load |
|---|---|
| Failure with evidence | this SKILL.md Workflow |
| Suspected issue without evidence | `arc:audit` / `arc:clarify`; no repair edit |
| Confirmed failure requiring repair | this SKILL.md Workflow |
| Unclear acceptance | `arc:clarify` |
| Same-session multi-model routing | `arc:prewalk` |
| Large repair campaign | `arc:sdlc` |
| Backend architecture | `arc:arch` |
| Lark-active incident notes | `arc:docs` |

## Context Search

- MUST load the **Evidence-first root-cause repair** section in [`docs/execution-truth.md`](../../docs/execution-truth.md) before diagnosing or editing.
- MUST inspect the failure signal before editing.
- MUST use `arc-idx search` first for related code paths and tests.
- MUST use exact search for error strings, stack frames, logs, and config keys.
- If `.lark.json` exists, MUST read it before final incident handoff.


## Red Lines

```text
NO BEHAVIORAL REPAIR BEFORE THE FAILURE IS CONFIRMED.
NO FIX WHILE THE ROOT CAUSE IS ONLY A HYPOTHESIS.
NO SYMPTOM PATCH, RETRY, COMPATIBILITY SHIM, OR WEAKENED ASSERTION AS A REPAIR.
ARCHITECTURE TRACE FIRST, BUSINESS CONTRACT SECOND, THEN EDIT.
NO LARGE REPAIR WITHOUT CURRENT LOCAL TASK DOCS.
NO MULTI-MODEL COST ROUTING VIA PLAN-POSTCARD COLD HANDOFF.
NO DEBUGGING WITHOUT PERSISTED EVIDENCE WHEN LOGS CAN BE CAPTURED.
NO SUCCESS CLAIM WITHOUT RERUNNING THE FAILING CHECK OR NAMING THE BLOCKER.
NO LARK INCIDENT UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE FEATURE FIX WITHOUT task_base UPDATE.
NO PAPER-OVER OR TEST-ONLY GREEN.
NO DIAGNOSIS ON THE WRONG ENV/BRANCH/DEPLOY SURFACE.
```

## Hard Constraints

- MUST preserve failure evidence.
- MUST use the **Observed → Expected → Surface → Evidence → Hypothesis → Root cause → Affected siblings → Complete fix boundary → Verification → Residual risk** packet from [`docs/execution-truth.md`](../../docs/execution-truth.md).
- MUST stop without a behavioral repair when the failure cannot be reproduced or the root cause cannot be confirmed; bounded behavior-preserving logs, traces, or reproducer coverage are allowed only to collect missing evidence and MUST NOT be presented as the fix.
- MUST trace the owning architecture boundary and shared invariants before comparing the behavior with the actual business contract.
- MUST inspect analogous call sites/flows governed by the same boundary and add focused regression coverage where the defect could recur.
- MUST capture runnable or observable failures into local log/evidence files before large edits. Use paths such as `.arc/artifacts/<task>/logs/` or `tmp/logs/`.
- MUST state a concrete hypothesis before significant edits.
- MUST patch the smallest safe surface.
- MUST apply the gates in [`docs/execution-truth.md`](../../docs/execution-truth.md): name the failing surface, fix cause not symptom, no matrix-only success, no invented domain keys, no temporary var flags or type-assert paper-overs.
- MUST reproduce or inspect on the same env/branch/deploy surface that produced the signal when dual tracks exist; re-state if production differs from the working tree.
- MUST re-verify claimed prior fixes on current code and evidence before closing residual risk.
- MUST apply `arc:sdlc` before code edits for large, multi-step, cross-module, or tracked repair work; task docs must be generated from the latest project state and updated immediately when project files, scope, assumptions, failure evidence, or status change.
- MUST apply `arc:prewalk` for same-session multi-model cost routing when the runtime supports mid-session model swap; MUST NOT default to plan-document cold handoff after Guide-only recon.
- MUST apply `arc:arch` before code edits, including its default backend architecture, DIP, helper extraction, and ponytail preflight rules.
- MUST rerun the failing path when feasible.
- MUST route all Lark incident/risk/task updates through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST update `task_base` through `arc:docs` when Lark is active and a fix changes a tracked feature or user-visible flow.
- NEVER change tests only to make them pass.
- NEVER broad-rewrite a localized failure.
- NEVER hide uncertainty behind a confident root cause.

## Workflow

1. Load the evidence-first root-cause repair gate and capture the failure, expected behavior, surface, and reproduction path.
2. Reproduce or inspect the failing path and persist available logs, command output, browser console, network traces, screenshots, or stack traces to a local evidence file.
3. If the failure is not confirmed, stop before any behavioral repair and report the evidence gap; add only bounded behavior-preserving instrumentation or reproducer coverage when it is required to collect evidence.
4. Search the saved evidence for exact error strings, request IDs, stack frames, network failures, and config keys.
5. Trace architecture ownership and shared invariants, then compare them with the actual business contract and user-visible semantics.
6. Form and test a root-cause hypothesis; name affected sibling paths and the complete fix boundary.
7. For large, multi-step, cross-module, or tracked repair work, apply `arc:sdlc` before code edits and keep local task status current as evidence or project state changes.
8. Apply `arc:arch` before code edits; stop and report if ponytail is required but unavailable or conflicting.
9. If multi-model routing is available and useful, follow `arc:prewalk` only after a grounded hypothesis: first production fix edit on Guide, then Executor inherits trajectory; otherwise oneshot.
10. Patch the smallest **complete** safe surface; if a shared boundary is the cause, do not constrain the edit to one symptom site.
11. Rerun the failing check plus focused regressions for analogous paths, saving verification output when useful.
12. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with incident summary, severity, root cause, changed feature/flow, verification, task status, and follow-up tasks.

## Quality Gates

- Fix targets cause, not only symptom.
- The original issue and root cause are confirmed before a behavioral repair; an unconfirmed hypothesis is reported, not patched, and any evidence instrumentation is explicitly labeled as investigation.
- Architecture ownership, business semantics, and analogous-path impact are explicit.
- Large, multi-step, cross-module, or tracked repair work has current local task docs, detailed subtasks, and synchronized progress status from `arc:sdlc`.
- Runnable failures have persisted sanitized log/evidence files, or the reason evidence could not be captured is explicit.
- Fix preserves DIP and default backend architecture responsibilities when backend architecture applies, unless the failure is explicitly caused by migrating toward them.
- Verification covers the original failure.
- Residual risk and rollback/monitoring notes are explicit for risky changes.
- Data-layer fixes check rows affected, transaction boundaries, state guards, soft-delete filters, and query bounds when relevant.
- Incident records and affected `task_base` rows are linked through `.lark.json` only when Lark is active.


## Scripts & Commands

Use project-native tests, logs, build commands, browser tooling, and observability. Persist command output and browser/runtime evidence to local files, then use `.ai-code-index/` for context and exact search for failure strings.

## Red Flags

- Fixing before reading the error.
- Debugging only by rereading code while runnable logs, browser console output, or command output were available but not captured.
- Fixing by adding concrete infrastructure dependencies into business services.
- Treating a retry as root cause.
- Fixing from stale task docs or leaving local repair progress inconsistent with the actual project state.
- Diagnosing production on a non-production track/branch, or packaging the wrong dual-track client.
- Closing bugs because docs/matrix say fixed while the failing path still reproduces.
- Paper-overs: tests changed only to pass, swallowed errors, temporary package-level flags, hard-coded epochs/IDs.
- Plan-postcard multi-model handoff that forces the cheap model to re-read everything cold.
- Swallowing exceptions or masking logs.
- Declaring success without verification.
- Updating Lark incident resources directly instead of through `arc:docs`.
- Fixing a Lark-active tracked feature while the `task_base` row is missing or stale.

## When to Use

- **Preferred Trigger**: There is concrete failure evidence or reproducible broken behavior.
- **Typical Scenario**: CI failure, runtime exception, regression, flaky path, broken flow, or incident follow-up.
- **Boundary Tip**: If there is no failure evidence, use `arc:audit` / `arc:clarify`; use `arc:build` only for an implementation-ready change that is not being presented as a repair.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `failure` | string | yes | Error, log, repro, screenshot, or failing command |
| `expected` | string | no | Expected behavior |
| `verification` | string | no | Command or flow proving repair |

## Outputs

```text
Fix Packet
- Failure observed
- Evidence/log files captured
- Root cause
- Fix applied
- Verification run
- Regression risk
- Lark / .lark.json / task_base handoff, if applicable
```
