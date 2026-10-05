---
name: arc:build
version: 1.2.3
description: >
  Implements scoped code changes and verifies them. Use when the user says 写代码, 开发功能,
  落地改动, implement a known scheme, or deliver a scoped change. Not for unknown-failure
  repair or requirement clarification.
---
# arc:build

## When to Use

Deliver a scoped implementation, refactor, migration, or documentation change. Use
`arc:clarify` for unresolved acceptance and `arc:fix` for failure-driven repair.
Preserve the user's existing authorization, explicit scope, and unrelated edits.

## Intent Router

| Condition | Read / use |
|---|---|
| Every implementation | [Execution truth](../../docs/execution-truth.md): scope, completion, think-before-coding, simplicity, surgical edits |
| Bug, regression, or failure-driven optimization | **Evidence-first root-cause repair** in [docs/execution-truth.md](../../docs/execution-truth.md), then `arc:fix` |
| Repository discovery | `arc:idx` before reading or editing unfamiliar files |
| Backend code / architecture | `arc:arch`, including its ponytail preflight |
| Frontend, mobile, desktop, mini-program | `arc:frontend` for platform defaults and interaction verification |
| API behavior or documentation | [Markdown API contract](references/api-contract.md) before dependent code |
| Large / cross-session / tracked implementation | `arc:sdlc`; ordinary multi-step work uses an in-session plan |
| Business invariants, transaction/retry effects, recovery evidence | `arc:reliability` defines claims; `arc:test` executes bindings |
| Useful, supported same-session model swap | `arc:prewalk` after the first production edit |
| Existing `.lark.json` or explicitly requested Lark work | `arc:docs` for all remote records |

Load only the applicable references. A route adds its constraints; it does not
expand scope, authorize an external action, or require re-asking for an authorized one.

## Red Lines

- No behavioral repair from an unconfirmed issue or root-cause hypothesis; collect
  bounded, behavior-preserving evidence first. Never mask a defect with retries,
  swallowed errors, weakened tests, temporary flags, or a partial symptom patch.
- No invented domain keys, wrong environment/branch, or opportunistic adjacent work.
- No JSON task/API specification replacing the required Markdown contract.
- No success from checkboxes, ability matrices, or second-hand claims: require code,
  project gates, and reachable behavior; name failed or unavailable verification.
- No Lark creation when neither `.lark.json` nor explicit user activation exists.
  All active Lark task/status writes go through `arc:docs`.
- No plan-only cold handoff as a substitute for `arc:prewalk` trajectory transfer.

## Workflow

1. Establish the requested outcome, acceptance, current surface and scope. Read the
   applicable execution-truth gates. Resolve discoverable facts locally; ask only
   for a material decision or missing input that blocks the authorized work.
2. Use `arc:idx` to inspect owners, analogous paths, contracts, tests and dependencies.
   Distinguish a new implementation from a repair before editing. When diagnosing,
   confirm the failure, architecture ownership, business contract and complete fix
   boundary under the evidence-first gate.
3. Apply `arc:sdlc` task tiering. For tracked work, keep task cards and progress current;
   split actual backend/frontend packages with independent acceptance. Read `arc:arch`
   before backend edits and `arc:frontend` for frontend decisions. Align changed APIs
   in Markdown (default `docs/02-API记录/`); Apifox is an explicitly requested copy.
4. Implement the smallest complete change. Reuse existing code before adding anything.
   Preserve contracts, error semantics and security boundaries. Empty collections are
   successful results with pagination metadata; distinguish not-found, permission,
   validation, network/server failure and processing states. Validate user-controlled
   ownership, roles, prices/amounts, identifiers, SQL and sort fields server-side.
5. Run the relevant native checks and verify the requested behavior. For repairs,
   cover the reproducer and affected sibling invariant. Check business success for
   data writes. Stop additional testing when required evidence and gates pass.
6. Update affected durable docs and tracked status. If Lark is active, read `.lark.json`
   and hand `arc:docs` the feature title, owner, status, requirement, changed files,
   verification, lifecycle link and `updated_at`; `task_base` must be updated or its
   blocker explicit before claiming that tracked feature complete.

## Delivery Evidence

Report the implemented behavior, changed files, checks and their results, skipped
checks with reasons, and remaining risks. Include model-swap notes or Lark links
only when applicable. Missing verification remains a limitation, never a PASS.
