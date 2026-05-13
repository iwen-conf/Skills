---
name: arc:fix
description: "故障修复：当已有失败证据、测试失败、线上异常或可复现 bug 时使用；定位根因、修复并回归验证。"
---

# arc:fix

## Overview

`arc:fix` handles failure-first work. It starts from evidence, identifies root cause, applies the smallest safe fix, and verifies the failing path. It does not depend on a dedicated Arc E2E runtime.

## Quick Contract

- **Trigger**: A bug, failed test, incident, regression, or broken user flow has evidence.
- **Inputs**: Failure symptoms, logs, reproduction steps, failing command, screenshots, or affected files.
- **Outputs**: Root cause, fix, verification evidence, and regression risk notes.
- **Quality Gate**: The original failure is explained and the fix is verified or the blocker is explicit.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` first only when the failure report lacks enough evidence to begin.
- Use `arc:build` when the task is feature work rather than failure repair.
- Use `arc:audit` after repair if the user wants broader quality assessment.
- Use project-native tests, logs, browser tooling, or `agent-browser` for reproduction as appropriate.

## Announce

Begin by stating clearly:
"I am using `arc:fix` to reproduce or inspect the failure, identify root cause, patch it, and verify the result."

## The Iron Law

```text
NO FIX WITHOUT ROOT CAUSE OR EXPLICIT UNCERTAINTY.
```

## Workflow

1. Capture the failure signal and expected behavior.
2. Reproduce or inspect the failure path when feasible.
3. Form a concrete root-cause hypothesis tied to files or runtime behavior.
4. Patch the smallest safe surface.
5. Run the failing check again, plus targeted regression checks.
6. Report root cause, fix, verification, and remaining risk.

## Quality Gates

- The fix addresses the cause, not only the symptom.
- The failing path is verified after the change when feasible.
- Test expectation changes are justified by product behavior, not convenience.
- Risky production changes include rollback or monitoring notes.

## Expert Standards

- Severity is described using `SEV` or an equivalent impact scale when relevant.
- Root cause analysis uses `5 Whys` or comparable causal reasoning for non-trivial failures.
- Complex incidents may use a lightweight `Fault Tree` to avoid single-cause tunnel vision.
- Communication is compatible with a `Blameless Postmortem` style.
- Maintain a `Mandatory Hypothesis` before editing significant code.
- Watch for `Rationalization Watch`: do not reinterpret failure evidence to fit an easy fix.

## Scripts & Commands

No dedicated Arc runtime scripts. Use project-native tests, logs, build commands, browser automation, and observability tools.

## Red Flags

- Fixing without reading the error or reproduction evidence.
- Changing tests to pass without validating behavior.
- Broad rewrites for localized failures.
- Declaring success without rerunning the relevant check.

## When to Use

- **Preferred Trigger**: There is concrete failure evidence or a reproducible broken behavior.
- **Typical Scenario**: CI failure, runtime exception, regression, flaky behavior, broken user flow, or incident follow-up.
- **Boundary Tip**: If there is no failure evidence and the work is a planned change, use `arc:build`.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `failure` | string | yes | Error, log, repro, or failing command |
| `expected` | string | no | Expected behavior |
| `verification` | string | no | Command or flow that proves the fix |

## Outputs

```text
Fix Packet
- Failure observed
- Root cause
- Fix applied
- Verification run
- Regression risk
```
