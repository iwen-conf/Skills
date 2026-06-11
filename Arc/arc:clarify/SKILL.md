---
name: arc:clarify
description: "Requirement clarification; hand active Lark requirements, task_base, and lifecycle updates to arc:docs."
---

# arc:clarify

## Overview

`arc:clarify` turns vague work into an executable task brief. It does not implement, plan a whole project, or write Lark resources directly.

## Quick Contract

- **Trigger**: Scope, context, constraints, or acceptance criteria are unclear.
- **Inputs**: User request, workdir, known context, relevant files, optional `.lark.json`.
- **Outputs**: Task brief with scope, constraints, assumptions, questions, and success criteria.
- **Quality Gate**: A downstream engineer can start without guessing.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:docs` only when Lark is active and clarified requirements must update Lark Docx, Base, `task_base`, or `.lark.json.lifecycle[]`.
- Use `arc:build` once the task is executable.
- Use `arc:frontend` when the clarified scope includes Web, mobile, desktop, multi-vendor mini-program stack, UI architecture, routing, server state, client state, forms, or theme decisions.
- Use `arc:audit` for read-only assessment.
- Use `aitask` only for cross-agent ownership or coordination.

## Context Search

- MUST inspect available local context before asking broad questions.
- MUST use `.ai-code-index/search.sh` first for repository context; use `rg` only for narrow exact follow-up.
- If `.lark.json` exists, MUST read it before finalizing the brief.

## Announce

Begin by stating clearly:
"I am using `arc:clarify` to turn the request into an executable task brief."

## The Iron Law

```text
NO EXECUTION WITHOUT CONTEXT, CONSTRAINTS, AND SUCCESS CRITERIA.
NO LARK REQUIREMENT UPDATE OUTSIDE arc:docs.
NO LARK-ACTIVE FEATURE REQUIREMENT WITHOUT task_base ROW.
```

## Hard Constraints

- MUST separate facts, assumptions, and open questions.
- MUST ask only blocking questions.
- MUST define in-scope and out-of-scope work.
- MUST use `Given-When-Then` when behavior must be tested.
- MUST record `arc:frontend` platform default stack as the frontend assumption unless the user explicitly specifies another stack: Web = React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod; mobile = React Native + Expo + TypeScript + NativeWind + Zustand + TanStack Query + Expo Router; desktop = Tauri 2 + Web stack; multi-vendor mini-program = Taro 4 + React + TypeScript + Zustand.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST hand off Lark-active feature requirements to `arc:docs` so `task_base` is created or updated before delivery starts.
- NEVER expand the requested surface speculatively.

## Workflow

1. Restate the goal in concrete terms.
2. Inspect repository and `.lark.json` context when available.
3. Define scope, constraints, assumptions, and success criteria.
4. Ask only remaining blocking questions.
5. Produce the brief.
6. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with requirement, acceptance criteria, task owner/status, and resource keys to update.

## Quality Gates

- Brief includes `Context`, `Task`, `Scope`, `Constraints`, `Assumptions`, `Open Questions`, and `Success Criteria`.
- Ambiguous terms are resolved or explicitly open.
- Acceptance criteria are testable.
- Lark requirement and `task_base` updates are linked through `.lark.json` via `arc:docs` only when Lark is active.

## Expert Standards

- Requirements quality follows `IEEE 29148`.
- User stories should satisfy `INVEST` when relevant.
- Behavioral acceptance criteria use `Given-When-Then`.

## Scripts & Commands

No dedicated runtime scripts. Use `.ai-code-index/` for repository search and exact file reads for follow-up.

## Red Flags

- Asking many questions before reading available context.
- Hiding assumptions inside the task.
- Vague success criteria.
- Updating Lark Base, `task_base`, Task, or Docx without `arc:docs`.

## When to Use

- **Preferred Trigger**: Requirements are vague or execution-critical context is missing.
- **Typical Scenario**: The user asks for a change but scope or verification is unclear.
- **Boundary Tip**: If the task is already clear, proceed to the appropriate execution skill.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `request` | string | yes | Original user request |
| `workdir` | string | no | Target project directory |
| `known_context` | string | no | Existing constraints or files |

## Outputs

```text
Task Brief
- Context
- Task
- Scope
- Constraints
- Assumptions
- Open Questions
- Success Criteria
- Lark / .lark.json / task_base handoff, if applicable
```
