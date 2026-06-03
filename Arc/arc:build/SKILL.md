---
name: arc:build
description: "代码交付：实施代码变更、运行验证并输出结果。"
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
- Use `.ai-code-index/` local search helpers for repository search and context discovery; use `aitask` for cross-agent task ownership.

## Context Search

- Before editing unfamiliar code, use `.ai-code-index/search.sh "query"` to locate relevant modules, flows, and symbols.
- If the index is missing or stale, run `.ai-code-index/reindex.sh`.
- Use `.ai-code-index/struct-search.sh` for call shapes and refactoring targets; use `.ai-code-index/symbols.sh` for definitions.
- Use `rg` only for narrow exact follow-up, new files, non-indexed files, or fallback when the index is insufficient.

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

## SQL Standards

- Use parameterized SQL for all user-controlled values. Do not concatenate user input into SQL strings.
- For dynamic identifiers such as sort fields, table names, or column names, use explicit whitelist mapping instead of passing raw user input.
- Do not assume `Exec` success means business success. `err == nil` only means the statement executed without a database error.
- For business `UPDATE` or `DELETE` statements that target a specific resource, keep the `Exec` return value and check affected rows.

```go
tag, err := store.Exec(ctx, `
    UPDATE feedbacks
    SET status = $2,
        updated_at = $3
    WHERE id = $1::uuid
`, id, status, now)
if err != nil {
    return err
}
if tag.RowsAffected() == 0 {
    return ErrFeedbackNotFound
}
```

- It is acceptable to discard `Exec` results only when 0 affected rows is valid business behavior, such as inserting logs, best-effort statistics updates, idempotent cleanup, initialization SQL, or code paths that have already proven the target exists.
- When updated data is needed after a write, prefer `UPDATE ... RETURNING ...` or `INSERT ... RETURNING ...` with `QueryRow(...).Scan(...)`.
- Avoid `SELECT *` in application code. Select explicit columns and keep the `Scan` order aligned with the SQL column order.
- Handle `NULL` deliberately. Do not conflate SQL `NULL` with Go zero values unless the business model says they are equivalent.
- Use stable ordering for pagination, usually with a deterministic tie-breaker such as `ORDER BY created_at DESC, id DESC`.
- Keep soft-delete filters consistent. Queries, updates, statistics, and details that should ignore deleted rows must include the same `deleted_at IS NULL` style condition.
- Maintain timestamp fields consistently. Use either database time such as `NOW()` or one application-side `now` value per operation; avoid mixing both in one workflow.
- Use transactions when multiple SQL statements must succeed or fail together.
- Encode state transitions in the `WHERE` clause when concurrency matters, then check affected rows.

```sql
UPDATE orders
SET status = 'paid',
    paid_at = $2,
    updated_at = $2
WHERE id = $1
  AND status = 'pending'
```

- Enforce uniqueness and referential integrity in the database with constraints, then translate database errors into business errors such as already exists, not found, invalid reference, or conflict.
- Cast structured parameters explicitly when needed, for example `$1::uuid` or `$2::jsonb`, and validate serialized JSON before passing it to the database.
- Be careful with unconditional `UPDATE` and `DELETE`. Business writes should normally be bounded by primary key, tenant, owner, status, or another explicit scope.

## Code Rot Gates

Full catalog: [`docs/code-rot-taxonomy.md`](../../docs/code-rot-taxonomy.md). At implementation time this skill prevents families A (convention drift), B (redundancy), C (security), E (error/state); the existing `SQL Standards` section above covers most of family D.

- Reuse before writing (#21, #23): search the index for an existing endpoint, module, or formatter before adding one. Do not clone near-identical behavior.
- Build only the requested surface (#32): if one API satisfies the requirement, ship one. No speculative scenario variants.
- One name per concept (#8, #18): reuse the project's existing field names; never introduce `phone` where the code already says `mobile`. Keep request and response field names identical.
- Centralize, do not scatter (#4, #16): reference shared status-code and constant definitions instead of inlining raw `500`/`400` literals or magic values.
- Never swallow errors (#13): no empty `catch`/`except: pass`; handle, wrap with context, or rethrow.
- Authorize every access (#12, #31): check ownership and role in the query and handler; recompute price/amount server-side before payment — never trust client-supplied amounts.
- Secure randomness and no backdoors (#28, #29): CSPRNG for tokens/codes; secrets from config, never hardcoded.
- Confirm state and JSON contracts before coding (#7, #19): use the state set and response envelope fixed by `arc:clarify`; do not invent new shapes mid-implementation.
- Execution integrity (#34, #35, #36): keep the project runnable — verify after meaningful changes (`Arc/scripts/verify-project.sh`); leave no placeholders or half-migrated call sites and report residual work honestly (`check-placeholders.sh`, `check-completion.sh`); never run unreviewed project-wide `sed -i`/`find -exec sed` rewrites — ensure a rollback checkpoint and edit directory-by-directory (`check-destructive.sh`).

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
- Cloning a near-identical endpoint or formatter instead of reusing an existing one (#21, #23).
- Inventing speculative APIs the requirement never asked for (#32).
- Trusting a client-supplied price, amount, or identifier without server-side authorization (#12, #31).

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
