---
name: arc:fix
description: "故障修复：基于失败证据定位根因，实施修复并回归验证。"
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

## Context Search

- Use `.ai-code-index/search.sh "query"` first to locate the failure path, related tests, ownership boundaries, and likely root-cause files.
- If the index is missing or stale, run `.ai-code-index/reindex.sh`.
- Use `.ai-code-index/struct-search.sh` for suspicious code shapes and `.ai-code-index/symbols.sh` for definitions.
- Use `rg` only for exact error strings, stack frames, config values, new files, or fallback when the index is insufficient.

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

## SQL Failure Checks

- When a bug involves SQL writes, distinguish database execution success from business success. `err == nil` does not prove that an `UPDATE` or `DELETE` matched the intended row.
- For resource-targeted writes, inspect the command tag and treat `RowsAffected() == 0` as not found or conflict according to the business rule.

```go
tag, err := store.Exec(ctx, `
    DELETE FROM feedbacks
    WHERE id = $1::uuid
`, id)
if err != nil {
    return err
}
if tag.RowsAffected() == 0 {
    return ErrFeedbackNotFound
}
```

- If a write needs to return the updated row, prefer `UPDATE ... RETURNING ...` with `QueryRow(...).Scan(...)` over `Exec` followed by a separate read.
- Check SQL fixes for parameterization, explicit column lists, `NULL` handling, stable pagination order, soft-delete filters, timestamp consistency, transaction boundaries, and state-transition guards in `WHERE`.
- Translate database errors and 0-row write results into precise business errors instead of reporting success or leaking raw database details.

## Code Rot Gates

Full catalog: [`docs/code-rot-taxonomy.md`](../../docs/code-rot-taxonomy.md). Failures most often root-cause into family D (data layer) and family E (error/state); the `SQL Failure Checks` section above is the data-layer arm of this.

- Suspect the data layer first (#5, #25, #26): an N+1 loop, a missing `LIMIT`, or a leading-wildcard `LIKE '%x%'` on an unindexed column is a common latency/timeout root cause.
- Check soft-delete consistency (#3): a "ghost row" or wrong count is often one query that forgot the `deleted_at IS NULL` filter the others apply.
- Hunt swallowed exceptions (#13): an empty `catch`/`except: pass` hiding the real error is a frequent reason a failure is invisible. Surface it before patching.
- Treat flaky-under-load as a race (#27): guard read-modify-write with a transaction, optimistic lock, or state-encoded `WHERE`, then check affected rows — do not just retry.
- Verify the state machine (#7): an "impossible" state usually means an undeclared transition; confirm the legal transition set rather than special-casing the symptom.
- Execution integrity (#34, #35, #36): re-run the failing check after patching (`Arc/scripts/verify-project.sh`); leave no placeholder or half-applied fix and report honestly (`check-placeholders.sh`, `check-completion.sh`); never bulk-`sed` a fix across the tree without a rollback checkpoint (`check-destructive.sh`).

## Expert Standards

- Severity is described using `SEV` or an equivalent impact scale when relevant.
- Root cause analysis uses `5 Whys` or comparable causal reasoning for non-trivial failures.
- Complex incidents may use a lightweight `Fault Tree` to avoid single-cause tunnel vision.
- Communication is compatible with a `Blameless Postmortem` style.
- Maintain a `Mandatory Hypothesis` before editing significant code.
- Watch for `Rationalization Watch`: do not reinterpret failure evidence to fit an easy fix.

## Scripts & Commands

No dedicated Arc runtime scripts. Use `.ai-code-index/` for repository context search, then project-native tests, logs, build commands, browser automation, and observability tools.

## Red Flags

- Fixing without reading the error or reproduction evidence.
- Changing tests to pass without validating behavior.
- Broad rewrites for localized failures.
- Declaring success without rerunning the relevant check.
- Patching a symptom while an empty `catch` still hides the real cause (#13).
- Retrying a flaky path instead of guarding the underlying race (#27).

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
