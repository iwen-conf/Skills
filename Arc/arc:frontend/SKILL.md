---
name: arc:frontend
description: "Frontend engineering; hand active Lark page progress, decisions, screenshots, task_base, and lifecycle to arc:docs."
---

# arc:frontend

## Overview

`arc:frontend` handles frontend lifecycle work: baseline, UI implementation, theme tokens, frontend architecture, verification, and frontend progress handoff. It does not replace design-only skills or create Lark resources directly.

## Quick Contract

- **Trigger**: The project needs frontend baseline, UI implementation, theme setup, frontend architecture, or frontend progress documentation.
- **Inputs**: Project path, frontend scope, product type, existing stack, verification expectations.
- **Outputs**: Frontend decisions, code/config changes, verification, and optional Lark handoff.
- **Quality Gate**: UI follows existing patterns, tokenized styling, accessibility basics, responsive constraints, and verified checks.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:define` if product concept or information architecture is missing.
- Use `arc:clarify` if page scope, target users, or acceptance criteria are unclear.
- Use `arc:build` for the implementation edit path.
- Use `arc:docs` only when Lark is active for frontend milestones, design decisions, page progress, `task_base` feature status, whiteboards, screenshots, or `.lark.json.lifecycle[]`.
- Use `arc:audit` for read-only UI architecture or accessibility review.

## Context Search

- MUST inspect existing routes, layouts, components, theme files, API clients, stores, forms, and tests before changing them.
- MUST use `.ai-code-index/search.sh` first; use `.ai-code-index/struct-search.sh typescript` for React patterns.
- If `.lark.json` exists, MUST read it before major frontend decisions.

## Announce

Begin by stating clearly:
"I am using `arc:frontend` to apply frontend engineering constraints and keep UI decisions traceable."

## The Iron Law

```text
NO FRONTEND CHANGE WITHOUT USER WORKFLOW.
NO UI DELIVERY WITHOUT DESIGN TOKENS, RESPONSIVE CONSTRAINTS, AND VERIFICATION.
NO LARK FRONTEND UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE FRONTEND FEATURE COMPLETION WITHOUT task_base UPDATE.
```

## Hard Constraints

- MUST preserve an existing frontend stack unless migration is explicitly requested.
- MUST keep styling behind `Design Token` or CSS variable paths.
- MUST implement `Accessibility` basics: semantic controls, labels, focus states, keyboard reachability, and contrast.
- MUST build `Responsive` layouts with explicit constraints; NEVER use viewport-scaled fonts as the layout solution.
- MUST model `RBAC` at route and action level when roles or permissions exist.
- MUST use project-native data/form/state patterns before introducing new libraries.
- MUST verify build/typecheck/lint/tests or report the blocker.
- MUST route all Lark progress, whiteboard, screenshot, or `.lark.json` updates through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST hand off every Lark-active frontend feature/page update to `arc:docs` so `task_base` records title, owner, status, related requirement, changed files, verification, lifecycle link, and `updated_at`.
- MUST NOT call Lark-active frontend work complete until `task_base` is updated or the blocker is explicit.

## Workflow

1. Confirm user workflow, pages, audience, devices, and verification.
2. Inspect existing frontend stack and patterns.
3. Choose route: preserve existing stack, apply React baseline for new React work, or document exception for non-React work.
4. Implement with tokenized styling, stable layout constraints, accessible states, and existing data/form/state patterns.
5. Run project-native verification.
6. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with pages, decisions, changed files, verification, screenshots/whiteboards, task status, lifecycle link, and resource keys.

## Quality Gates

- No scattered hardcoded palette values.
- No overlapping or overflowing text in expected viewports.
- Loading, empty, error, disabled, and permission-denied states exist when relevant.
- Server state is not duplicated into unrelated global stores.
- Significant frontend progress and `task_base` are linked through `.lark.json` only when Lark is active.

## Expert Standards

- `Design Token` decisions are centralized and auditable.
- `Accessibility` checks are part of done.
- `Responsive` behavior uses explicit constraints.
- `RBAC` protects routes and action-level controls when applicable.

## Scripts & Commands

Use project-native scripts. For new Vite React projects only, use [`references/scaffold-commands.md`](references/scaffold-commands.md). For palettes and CSS variables, use [`references/color-tokens.md`](references/color-tokens.md).

## Red Flags

- Replacing the stack without a migration request.
- Decorative landing page instead of the requested usable UI.
- Hardcoded component colors.
- Global store used for server state by convenience.
- Frontend lifecycle progress left out of existing `.lark.json`.
- Completed Lark-active frontend feature missing a current `task_base` row.

## When to Use

- **Preferred Trigger**: The user asks to build, standardize, review, or document frontend lifecycle work.
- **Typical Scenario**: React/Vite setup, dashboard/admin UI, routing/state architecture, theme setup, forms, responsive pages, frontend handoff.
- **Boundary Tip**: Use `arc:build` for backend/API-only work and `arc:docs` for pure Lark indexing.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `frontend_scope` | string | yes | Page, module, app shell, theme, state layer, or full frontend |
| `product_type` | string | no | Admin console, dashboard, content platform, tool, landing, or other |
| `verification` | string | no | Expected build, lint, test, screenshot, or typecheck |

## Outputs

```text
Frontend Handoff
- Workflow and pages covered
- Stack and token decisions
- Files changed
- Responsive/accessibility checks
- Verification run
- Lark / .lark.json / task_base handoff, if applicable
- Residual risks
```
