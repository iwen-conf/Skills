---
name: arc:frontend
version: 1.0.0
description: >
  Delivers frontend and UI work on the default Web, mobile, desktop, and mini-program
  stacks. Use when the user says 前端, UI, 页面, React, 小程序, shadcn, Tailwind, or frontend
  baseline. Not for backend-only API work.
---
# arc:frontend

## Overview

`arc:frontend` handles frontend lifecycle work: baseline, UI implementation, theme tokens, frontend architecture, verification, and frontend progress handoff. It is the canonical default Web and cross-platform frontend stack contract for every domain skill. It does not replace design-only skills or create Lark resources directly.

## Quick Contract

- **Trigger**: The project needs frontend baseline, UI implementation, theme setup, frontend architecture, or frontend progress documentation.
- **Inputs**: Project path, frontend scope, product type, existing stack, verification expectations.
- **Outputs**: Frontend decisions, code/config changes, verification, and optional Lark handoff.
- **Quality Gate**: UI follows the default stack unless explicitly exempted, existing patterns, tokenized styling, accessibility basics, responsive constraints, and verified checks.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Intent Router

| When | Load |
|---|---|
| Stack, tokens, scaffold | [`references/scaffold-commands.md`](references/scaffold-commands.md) / [`references/color-tokens.md`](references/color-tokens.md) |
| UI implementation | this SKILL.md Workflow |
| Unclear scope | `arc:clarify` |
| Large tracked frontend work | `arc:sdlc` |
| Same-session multi-model routing | `arc:prewalk` |
| Backend/API only | `arc:build` + `arc:arch` |
| Lark-active page progress | `arc:docs` |

## Context Search

- MUST inspect existing routes, layouts, components, theme files, API clients, stores, forms, and tests before changing them.
- MUST use `arc-idx search` first; use `arc-idx ast --lang tsx` for React patterns.
- If `.lark.json` exists, MUST read it before major frontend decisions.


## Red Lines

```text
NO FRONTEND CHANGE WITHOUT USER WORKFLOW.
NO LARGE FRONTEND CHANGE WITHOUT CURRENT LOCAL TASK DOCS.
NO MULTI-MODEL COST ROUTING VIA PLAN-POSTCARD COLD HANDOFF.
NO UI DELIVERY WITHOUT DESIGN TOKENS, RESPONSIVE CONSTRAINTS, AND VERIFICATION.
NO LARK FRONTEND UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE FRONTEND FEATURE COMPLETION WITHOUT task_base UPDATE.
```

## Hard Constraints

- MUST apply `arc:sdlc` before code edits for large, multi-page, cross-module, architectural, or tracked frontend work; task docs must be generated from the latest project state and updated immediately when routes, components, APIs, state models, scope, assumptions, or status change.
- MUST apply `arc:prewalk` for same-session multi-model cost routing when the runtime supports mid-session model swap; MUST NOT default to plan-document cold handoff.
- MUST use the default Web frontend stack for new Web frontends unless the user explicitly names another stack: `React 19` + `TypeScript` + `Vite` + `Tailwind CSS` + `shadcn/ui` + `Zustand` for client state + `TanStack Query` for server state + `TanStack Router` for type-safe routing + `React Hook Form` + `Zod`.
- MUST use the default mobile stack for new iOS/Android apps unless the user explicitly names another stack: `React Native` + `Expo` + `TypeScript` + `NativeWind` + `Zustand` + `TanStack Query` + `Expo Router`.
- MUST use the default desktop stack for new Mac/Windows/Linux apps unless the user explicitly names another stack: `Tauri 2` + the default React Web stack, reusing Web UI/state/query/form code where practical.
- MUST use the default mini-program stack for WeChat and other mini-program targets unless the user explicitly names another stack: `Taro 4` + `React` + `TypeScript` + `Zustand`.
- MUST use `wxskills` for WeChat platform APIs, privacy, payment, native component constraints, Skyline, and maintenance of existing native mini-program projects on WeChat; do not treat `wxskills` as a separate default project stack.
- MUST preserve an existing frontend stack unless migration is explicitly requested; new modules in mixed legacy projects use the default stack when they can be isolated.
- MUST NOT substitute same-duty defaults such as React Router, Redux, MobX, SWR, Formik, Yup, Ant Design, MUI, Chakra, or a custom component library unless the user explicitly requires it or the existing project already depends on it and the boundary is documented.
- MUST separate state by responsibility: URL/search params in `TanStack Router`, server cache and mutations in `TanStack Query`, lightweight cross-page client state in `Zustand`, complex form state in `React Hook Form` with `Zod`, and component-only state in React local state.
- MUST make vertical or business-domain skills call this default stack contract for frontend surfaces; business domain differences do not create a new default frontend or cross-platform stack.
- MUST model UI state semantics explicitly: `loading` means a request or transition is pending; `empty` means the request succeeded but returned no usable records/content; `error` means the request or parsing failed; `permission-denied` means the user is not allowed to see or perform the action.
- MUST design `empty` and `error` as different surfaces. Empty states use neutral tone, contextual next action, optional create/reset-filter guidance, and must not use destructive/error styling, error pages, exception boundaries, or failure toasts.
- MUST derive empty state from successful data contracts such as `items.length === 0`, `total === 0`, or an explicit empty flag, not from thrown errors, rejected promises, HTTP 4xx/5xx branches, or generic catch handlers.
- MUST keep styling behind `Design Token` or CSS variable paths.
- MUST implement `Accessibility` basics: semantic controls, labels, focus states, keyboard reachability, and contrast.
- MUST build `Responsive` layouts with explicit constraints; NEVER use viewport-scaled fonts as the layout solution.
- MUST model `RBAC` at route and action level when roles or permissions exist.
- MUST use project-native data/form/state patterns before introducing new libraries.
- MUST persist frontend debugging evidence to local files when investigating runnable UI bugs: browser console output, network failures, runtime errors, screenshots, and relevant reproduction notes should go under `.arc/artifacts/<task>/logs/` or `tmp/logs/`.
- MUST remove temporary `console.log`, `debugger`, alert-based probes, and noisy instrumentation before completion, or convert them into the project's level-gated logger/telemetry pattern.
- MUST verify build/typecheck/lint/tests or report the blocker.
- MUST route all Lark progress, whiteboard, screenshot, or `.lark.json` updates through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST hand off every Lark-active frontend feature/page update to `arc:docs` so `task_base` records title, owner, status, related requirement, changed files, verification, lifecycle link, and `updated_at`.
- MUST NOT call Lark-active frontend work complete until `task_base` is updated or the blocker is explicit.

## Workflow

1. Confirm user workflow, pages, audience, devices, and verification.
2. For large, multi-page, cross-module, architectural, or tracked frontend work, apply `arc:sdlc` before code edits and keep local task status current as routes, components, APIs, or state models change.
3. Inspect existing frontend stack and patterns.
4. Choose route: apply the platform default stack, preserve an existing stack with a documented boundary, or document the explicit user-requested exception.
5. If multi-model routing is available and useful, follow `arc:prewalk` for implementation turns; otherwise oneshot.
6. Implement with tokenized styling, stable layout constraints, accessible states, explicit loading/empty/error/permission branches, and existing data/form/state patterns.
7. For UI bugs, capture browser/runtime evidence to files before broad edits and use exact error strings or request IDs to drive the fix.
8. Run project-native verification.
9. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with pages, decisions, changed files, verification, screenshots/whiteboards, task status, lifecycle link, and resource keys.

## Quality Gates

- No scattered hardcoded palette values.
- Large, multi-page, cross-module, architectural, or tracked frontend work has current local task docs, detailed subtasks, and synchronized progress status from `arc:sdlc`.
- Default-stack projects include the required stack pieces and no same-duty duplicate library.
- Cross-platform projects declare exactly one platform target per deliverable: Web, iOS/Android, Desktop, or Mini Program.
- URL, server, client-global, form, and local state are not mixed into one store.
- No overlapping or overflowing text in expected viewports.
- Loading, empty, error, disabled, and permission-denied states exist when relevant, with empty and error proven to render through different branches and visual treatments.
- Server state is not duplicated into unrelated global stores.
- Frontend bug work has saved, searchable console/network/runtime evidence when the failure is reproducible or observable.
- Temporary debug probes are removed or converted into approved level-gated logging/telemetry before delivery.
- Significant frontend progress and `task_base` are linked through `.lark.json` only when Lark is active.


## Scripts & Commands

Use project-native scripts. For new React 19 + TypeScript + Vite projects, use [`references/scaffold-commands.md`](references/scaffold-commands.md). For palettes and CSS variables, use [`references/color-tokens.md`](references/color-tokens.md).

## Red Flags

- Replacing the stack without a migration request.
- Introducing React Router, Redux, SWR, Formik, Yup, MUI, Ant Design, Chakra, or another same-duty library as a new default.
- Treating a vertical business skill as permission to invent a separate Web, mobile, desktop, or mini-program frontend stack.
- Starting a new WeChat mini-program with native WXML/WXSS/Component as the default stack instead of Taro 4 without explicit user direction.
- Decorative landing page instead of the requested usable UI.
- Implementing from stale frontend task docs or leaving local page/task progress inconsistent with current routes, components, APIs, or state contracts.
- Plan-postcard multi-model handoff that drops Guide trajectory and re-reads the UI surface cold on a cheap model.
- Debugging only from source inspection while browser console, network, or runtime errors could be captured.
- Leaving `console.log`, `debugger`, alert probes, or noisy debug output in delivered frontend code.
- Hardcoded component colors.
- Global store used for server state by convenience.
- Empty search results, first-use pages, zero-count dashboards, or filtered-out lists displayed as error pages, destructive alerts, or failed fetch states.
- Frontend lifecycle progress left out of existing `.lark.json`.
- Completed Lark-active frontend feature missing a current `task_base` row.

## When to Use

- **Preferred Trigger**: The user asks to build, standardize, review, or document frontend lifecycle work.
- **Typical Scenario**: React 19 + TypeScript + Vite setup, React Native + Expo apps, Tauri 2 desktop shells, Taro 4 mini-programs, dashboard/admin UI, TanStack Router/Query architecture, theme setup, forms, responsive pages, frontend handoff.
- **Boundary Tip**: Use `arc:build` for backend/API-only work and `arc:docs` for pure Lark indexing.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `frontend_scope` | string | yes | Page, module, app shell, theme, state layer, or full frontend |
| `platform_target` | string | yes | Web, iOS/Android, Desktop, or Mini Program |
| `product_type` | string | no | Admin console, dashboard, content platform, tool, landing, or other |
| `verification` | string | no | Expected build, lint, test, screenshot, or typecheck |

## Outputs

```text
Frontend Handoff
- Workflow and pages covered
- Stack and token decisions, including default stack or explicit exception
- Platform target: Web / iOS+Android / Desktop / Mini Program
- Files changed
- Responsive/accessibility checks
- Debug evidence/log files, when frontend bug work was performed
- Verification run
- Lark / .lark.json / task_base handoff, if applicable
- Residual risks
```
