---
name: arc:microcopy
description: "报错文案人话化改写：扫描系统中的错误提示、校验失败文案、空状态和恢复提示，识别堆栈直出、内部错误码、技术术语与责怪式表述，并改写为非技术用户也能理解、可执行、可恢复的提示；当用户说“把报错改成人话/优化错误提示/扫描错误信息/让小白也能看懂报错”时触发。"
---

# arc:microcopy — user-friendly error copy refactoring

## Overview

`arc:microcopy` scans user-visible failure text across product surfaces and rewrites it in plain language for non-technical users. It separates internal diagnostics from customer-facing copy, inventories where confusing messages appear, rewrites each message to explain what happened and what to do next, and keeps debugging anchors such as support codes, structured logs, and trace IDs available to engineers.

## Quick Contract

- **Trigger**: The product exposes technical error text, raw exception wording, or jargon-heavy validation messages that ordinary users cannot understand or act on.
- **Inputs**: `project_path`, rewrite scope, target user profile, optional screenshots/logs/support tickets, and delivery mode (`catalog-only` / `catalog+patch` / `patch+verify`).
- **Outputs**: Message inventory, rewrite rules, source patch notes, and verification evidence under `.arc/microcopy/<task-name>/`.
- **Quality Gate**: Every changed message must be understandable to the target audience, include recovery direction when possible, and preserve engineering debuggability.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Tip** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:microcopy` to inventory user-visible error messages, separate internal diagnostics from user copy, and rewrite failure text in plain language."

## Teaming Requirement

- Every execution should separate `Owner` (target audience and product intent), `Executor` (scan + patch), and `Reviewer` (readability + diagnostics guardrail).
- In a single-agent environment, keep the same decision-execution-review loop explicit before delivery.

## The Iron Law

```
NO USER-FACING ERROR REWRITE WITHOUT PRESERVING DEBUGGABILITY, RECOVERY GUIDANCE, AND AUDIENCE FIT.
```

## Workflow

1. Inventory candidate error copy across UI text, i18n dictionaries, validation rules, toast/snackbar layers, API error mappers, CLI output meant for end users, and support screenshots.
2. Split findings into `user-facing` and `internal-only`. Stack traces, SQL fragments, hostnames, internal exception classes, and raw payload dumps must stay out of end-user copy unless there is an explicit admin/debug audience requirement.
3. For every user-facing message, capture: source path, current wording, trigger condition, blocked user goal, target audience, and whether an error code or support reference must remain visible.
4. Rewrite with `plain language`: say what happened, why the user is blocked if known, and what to do next. Use `actionable guidance` instead of vague advice like "operation failed" or "contact admin" with no context.
5. Match the `user mental model`: use product terms the user already sees in the UI, keep copy `blame-free`, and `avoid jargon` such as exception names, protocol terms, SQL/database phrasing, or framework internals.
6. Patch the true source of the message, not just one rendered location. Prefer shared translation maps, validation catalogs, or API-to-UI translation boundaries when they exist.
7. Verify the affected flow. Re-run targeted validators and user paths so the new text is visible, consistent, and still traceable through support code, structured logging, or correlated diagnostics.
8. If the real problem is product breakage rather than poor wording, stop polishing the symptom and route to `arc:fix` so the defect itself is repaired.

## Quality Gates

- Each changed message must tell the target user what happened in language they can understand.
- Each changed message should include the next best user action, fallback path, or support handoff when recovery is possible.
- User-visible copy must not leak stack traces, SQL details, file paths, tokens, hostnames, or internal class/function names.
- Internal debuggability must remain intact through logs, error IDs, correlation IDs, or preserved machine-readable error codes.
- Similar failure types must use consistent wording across product surfaces.

## Expert Standards

- Use `plain language` first: short sentences, familiar nouns, and verbs that describe the blocked action.
- Preserve the `user mental model`: reuse the product's visible labels, workflow names, and object names rather than backend terminology.
- Provide `actionable guidance`: tell the user whether to retry, check input, wait, refresh, re-authenticate, or contact support with a code.
- Explicitly `avoid jargon`: no exception names, HTTP jargon, storage/SQL internals, framework terms, or cryptic abbreviations unless the audience is clearly technical.
- Keep error copy `blame-free`: do not shame the user or imply incompetence when the system cannot complete the action.

## Scripts & Commands

- Candidate scan command: `rg -n --hidden --glob '!{.git,node_modules,dist,build,.magi}/**' '(error|exception|failed|invalid|denied|timeout|not found|forbidden|toast|snackbar|alert|setError|message)' <project_path>`
- Surface prioritization: inspect shared copy catalogs, i18n files, form validators, API error translation layers, and notification components before editing duplicated call sites.
- Verification: run the repository validators plus the most relevant user flow or targeted tests after patching copy.
- Working directory: write inventory and review outputs to `<project_path>/.arc/microcopy/<task-name>/`.

## Red Flags

- Rewriting internal logs instead of user-facing copy and destroying engineering context.
- Hiding a real product defect behind nicer wording instead of fixing the broken flow.
- Returning generic copy like "操作失败，请重试" without context or next-step guidance.
- Exposing stack traces, infrastructure details, raw payloads, or sensitive tokens to end users.
- Using accusatory or责怪式 wording that makes the user feel at fault for a system limitation.

## When to Use

- **Preferred Trigger**: The system already has user-visible technical error text that non-technical users cannot understand or recover from.
- **Typical Scenario**: Frontend toasts, form validation messages, installer/update errors, admin console alerts, API error mapping, or support escalations caused by opaque wording.
- **Boundary Tip**: Use `arc:fix` when the underlying flow is broken, `arc:clarify` when target audience or tone is still unclear, and `arc:build` once the rewrite scope is clear and code changes are ready to land.

## Input Arguments

| Parameter | Type | Required | Description |
|---|---|---|---|
| `project_path` | string | required | Root path of the product or repository to scan. |
| `task_name` | string | required | Case name used for the working directory under `.arc/microcopy/`. |
| `search_scope` | string | optional | Directory, module, or surface filter; defaults to repo-wide scanning. |
| `user_profile` | string | optional | Target audience, such as customer, operator, finance user, or non-technical admin. |
| `delivery_mode` | string | optional | `catalog-only`, `catalog+patch`, or `patch+verify`. |
| `evidence_paths` | string | optional | Screenshots, logs, support tickets, bug reports, or failing runs that show confusing copy. |

## Outputs

```text
<project_path>/.arc/microcopy/<task-name>/
├── inventory.md
├── rewrite-rules.md
├── patch-notes.md
└── verification.md
```
