---
name: arc:fix
description: "Failure repair with root cause; hand active Lark incidents, risks, follow-ups, task_base, and lifecycle to arc:docs."
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

## Routing Matrix

- Use `arc:clarify` only if the failure report lacks enough evidence to begin.
- Use `arc:build` when the work is planned feature delivery, not repair.
- Use `arc:docs` only when Lark is active for incident docs, risk Base, `task_base` feature status, follow-up Task, meeting records, or `.lark.json.lifecycle[]`.
- Use `arc:audit` after repair for broader read-only review.

## Context Search

- MUST inspect the failure signal before editing.
- MUST use `.ai-code-index/search.sh` first for related code paths and tests.
- MUST use exact search for error strings, stack frames, logs, and config keys.
- If `.lark.json` exists, MUST read it before final incident handoff.

## Announce

Begin by stating clearly:
"I am using `arc:fix` to inspect the failure, identify root cause, patch it, and verify the result."

## The Iron Law

```text
NO FIX WITHOUT ROOT CAUSE OR EXPLICIT UNCERTAINTY.
NO LARGE REPAIR WITHOUT CURRENT LOCAL TASK DOCS.
NO DEBUGGING WITHOUT PERSISTED EVIDENCE WHEN LOGS CAN BE CAPTURED.
NO SUCCESS CLAIM WITHOUT RERUNNING THE FAILING CHECK OR NAMING THE BLOCKER.
NO LARK INCIDENT UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE FEATURE FIX WITHOUT task_base UPDATE.
```

## Hard Constraints

- MUST preserve failure evidence.
- MUST capture runnable or observable failures into local log/evidence files before large edits. Use paths such as `.arc/artifacts/<task>/logs/` or `tmp/logs/`.
- MUST state a concrete hypothesis before significant edits.
- MUST patch the smallest safe surface.
- MUST apply `arc:task-doc-progress-conventions` before code edits for large, multi-step, cross-module, or tracked repair work; task docs must be generated from the latest project state and updated immediately when project files, scope, assumptions, failure evidence, or status change.
- MUST apply `arc:project-architecture-conventions` before code edits, including its default backend architecture, DIP, helper extraction, and ponytail preflight rules.
- MUST rerun the failing path when feasible.
- MUST route all Lark incident/risk/task updates through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST update `task_base` through `arc:docs` when Lark is active and a fix changes a tracked feature or user-visible flow.
- NEVER change tests only to make them pass.
- NEVER broad-rewrite a localized failure.
- NEVER hide uncertainty behind a confident root cause.

## Workflow

1. Capture failure, expected behavior, and reproduction path.
2. Reproduce or inspect the failing path and persist available logs, command output, browser console, network traces, screenshots, or stack traces to a local evidence file.
3. Search the saved evidence for exact error strings, request IDs, stack frames, network failures, and config keys.
4. Form and test a root-cause hypothesis.
5. For large, multi-step, cross-module, or tracked repair work, apply `arc:task-doc-progress-conventions` before code edits and keep local task status current as evidence or project state changes.
6. Apply `arc:project-architecture-conventions` before code edits; stop and report if ponytail is required but unavailable or conflicting.
7. Patch the smallest safe surface.
8. Rerun the failing check plus focused regressions, saving verification output when useful.
9. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with incident summary, severity, root cause, changed feature/flow, verification, task status, and follow-up tasks.

## Quality Gates

- Fix targets cause, not only symptom.
- Large, multi-step, cross-module, or tracked repair work has current local task docs, detailed subtasks, and synchronized progress status from `arc:task-doc-progress-conventions`.
- Runnable failures have persisted sanitized log/evidence files, or the reason evidence could not be captured is explicit.
- Fix preserves DIP and default backend architecture responsibilities when backend architecture applies, unless the failure is explicitly caused by migrating toward them.
- Verification covers the original failure.
- Residual risk and rollback/monitoring notes are explicit for risky changes.
- Data-layer fixes check rows affected, transaction boundaries, state guards, soft-delete filters, and query bounds when relevant.
- Incident records and affected `task_base` rows are linked through `.lark.json` only when Lark is active.

## Expert Standards

- Severity is described with `SEV` or an equivalent impact scale when relevant.
- Root cause uses `5 Whys` or equivalent causal reasoning.
- Complex incidents may use a lightweight `Fault Tree`.
- Communication remains compatible with `Blameless Postmortem`.
- Maintain a `Mandatory Hypothesis`; apply `Rationalization Watch` against easy but unsupported fixes.

## Scripts & Commands

Use project-native tests, logs, build commands, browser tooling, and observability. Persist command output and browser/runtime evidence to local files, then use `.ai-code-index/` for context and exact search for failure strings.

## Red Flags

- Fixing before reading the error.
- Debugging only by rereading code while runnable logs, browser console output, or command output were available but not captured.
- Fixing by adding concrete infrastructure dependencies into business services.
- Treating a retry as root cause.
- Fixing from stale task docs or leaving local repair progress inconsistent with the actual project state.
- Swallowing exceptions or masking logs.
- Declaring success without verification.
- Updating Lark incident resources directly instead of through `arc:docs`.
- Fixing a Lark-active tracked feature while the `task_base` row is missing or stale.

## When to Use

- **Preferred Trigger**: There is concrete failure evidence or reproducible broken behavior.
- **Typical Scenario**: CI failure, runtime exception, regression, flaky path, broken flow, or incident follow-up.
- **Boundary Tip**: If there is no failure evidence, use `arc:build`.

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
