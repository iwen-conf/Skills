---
name: arc:fix
version: 1.1.3
description: >
  Diagnoses and repairs failures from logs, stack traces, failing tests, or incidents. Use
  when the user says 报错, 修复, 故障, bug, 根因, incident, or failing test. Not for greenfield
  feature delivery.
---
# arc:fix

## When to Use

Repair a bug, incident, regression, crash or failing flow supported by observable
evidence. A suspected failure starts with investigation; an implementation-ready
feature belongs to `arc:build`. Continue authorized diagnosis autonomously.

## Intent Router

| Condition | Read / use |
|---|---|
| Every diagnosis / repair | **Evidence-first root-cause repair** in [docs/execution-truth.md](../../docs/execution-truth.md) |
| Error strings, request IDs, stack frames, owners and sibling paths | `arc:idx` exact text, symbol and AST discovery |
| Unclear expected contract | `arc:clarify` for the unresolved decision |
| Backend fix | `arc:arch`, including ponytail preflight and DIP |
| Frontend failure / browser evidence | `arc:frontend` and the existing browser tooling |
| Large / cross-session / tracked repair campaign | `arc:sdlc`; ordinary repairs use an in-session plan |
| Transaction, duplicate-effect, retry or recovery incident | `arc:reliability` for invariant evidence and prevention controls |
| Regression execution | `arc:test` |
| Useful, supported same-session model swap | `arc:prewalk` after a grounded first production fix |
| Existing `.lark.json` or explicit Lark activation | `arc:docs` for incident, risk and task records |

Read applicable scope, completion and surgical-edit gates in execution-truth too.
Do not load unrelated platforms or repeat an already sufficient investigation.

## Red Lines

- No behavioral repair until both the failure and root cause are confirmed.
- Trace owning architecture and shared invariants, then the real business contract;
  do not patch only a symptom when the cause affects sibling paths.
- No swallowed errors, test-only green, weakened assertions, blanket retries,
  compatibility shims, temporary package flags or invented domain IDs as a repair.
- No debugging solely from code when runnable logs or runtime evidence can be captured.
- No diagnosis on the wrong environment/branch or success from stale docs/matrices.
- No large tracked repair with stale task documents or plan-only cold model handoff.
- No Lark writes outside `arc:docs`, or implicit creation without Lark activation.

## Workflow

1. Record observed/expected behavior, target environment and branch, and a reproducible
   command or observable failing path. Re-check a claimed prior fix on current code.
2. Preserve sanitized evidence before substantial edits: command stdout/stderr, logs,
   traces, screenshots, browser console and network errors as applicable. Use an
   ignored location such as `.arc/artifacts/<task>/logs/`; never persist secrets.
3. Search exact failure details with `arc:idx`. Trace ownership, data/control flow,
   state transitions, dependencies and shared invariants; compare with the actual
   business contract. State a hypothesis and seek evidence that can disprove it.
4. Name the root cause, affected siblings and complete repair boundary. If evidence
   remains insufficient, continue bounded investigation or add behavior-preserving
   instrumentation/reproducer coverage; label it investigation, not a fix. Report
   the specific missing evidence when further progress needs user/environment input.
5. Apply `arc:sdlc` tiering; keep tracked state current. Use `arc:arch` before backend
   edits and the existing frontend rules for UI changes. Repair the complete owning
   boundary while preserving unrelated changes. Keep DIP and server-side validation;
   inspect rows affected, transactions, state guards, soft deletes and query bounds
   when data-layer behavior matters.
6. Rerun the original failing check and focused sibling regressions. Preserve results;
   hand transaction/recovery incidents to `arc:reliability` for real-boundary business
   evidence and prevention/detection/recovery controls. A missing target is BLOCKED,
   not success. Give rollback/monitoring notes for risky changes.
7. Synchronize affected docs and tracked tasks. If Lark is active, read `.lark.json`
   and use `arc:docs` to record incident severity, cause, changed feature, verification,
   follow-ups and the affected `task_base` row; report any update blocker explicitly.

## Delivery Evidence

Use the repair packet from execution-truth:
`Observed → Expected → Surface → Evidence → Hypothesis → Root cause → Affected siblings → Complete fix boundary → Verification → Residual risk`.

Link saved evidence. Distinguish confirmed results from uncertainty and unexecuted
checks; the presence of a patch does not establish that the failing path is repaired.
