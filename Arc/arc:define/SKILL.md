---
name: arc:define
description: "Project definition and PRD; hand active Lark project-home/Wiki/.lark.json updates to arc:docs."
---

# arc:define

## Overview

`arc:define` turns a rough product or project idea into a concrete definition or PRD. It does not implement, decompose tasks, or create Lark resources directly.

## Quick Contract

- **Trigger**: The user needs a project definition, PRD, blueprint, or structured product concept.
- **Inputs**: Idea, target users, constraints, optional repository, optional prior notes.
- **Outputs**: Project Definition Document plus open questions and optional Lark handoff.
- **Quality Gate**: Every section is concrete, named, and verifiable; unknowns stay explicit.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:docs` only when Lark is active or the user confirms Lark for PRD, project home, Wiki node, or `.lark.json` entry.
- Use `arc:clarify` for task-level acceptance criteria after the project is defined.
- Use `arc:frontend` for Web, mobile, desktop, multi-vendor mini-program stack, UI architecture, theme, routing, state, and form baseline decisions.
- Use `arc:build` only after scope is implementation-ready.
- Use `arc:audit` for read-only review of an existing definition.

## Context Search

- For brownfield work, MUST search existing terms first with `.ai-code-index/search.sh`, then `.ai-code-index/symbols.sh` if entity names matter.
- If `.lark.json` exists, MUST read it before drafting so PRD, owners, requirements, and Wiki links remain consistent.

## Announce

Begin by stating clearly:
"I am using `arc:define` to produce a structured project definition."

## The Iron Law

```text
NO GENERIC PRD.
NO INVENTED DOMAIN TERMS IN A BROWNFIELD PROJECT.
NO PASSIVE LARK PRD OR .lark.json CREATION.
```

## Hard Constraints

- MUST follow [`references/template.md`](references/template.md) section order unless the user explicitly provides another required format.
- MUST use one canonical name per domain concept.
- MUST define frontend surfaces with `arc:frontend` platform default stack unless the user explicitly chooses another stack: Web = React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod; mobile = React Native + Expo + TypeScript + NativeWind + Zustand + TanStack Query + Expo Router; desktop = Tauri 2 + Web stack; multi-vendor mini-program = Taro 4 + React + TypeScript + Zustand.
- MUST mark missing facts as open questions; NEVER fill gaps with filler.
- MUST route through `arc:docs` for all active Lark writes and `.lark.json.resources.prd` updates.
- MUST NOT suggest or create a Lark project space unless the user explicitly asks for Lark/Feishu output or `.lark.json` already exists.
- MUST NOT create `.lark.json` when Lark is inactive.
- NEVER turn project definition into task planning, code design, or implementation.

## Workflow

1. Restate the idea in one concrete sentence.
2. Extract or confirm domain terms, users, constraints, and non-goals.
3. Fill the template with specific content only.
4. List open questions separately.
5. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with the PRD content and intended resource key.

## Quality Gates

- Project positioning is specific to this project.
- Core concept is a central abstraction, not a feature list.
- Core capabilities are 3-6 user-verifiable capabilities.
- Main objects use existing repository vocabulary when a repository exists.
- Business flow is a short ordered path.
- `.lark.json.resources.prd` is updated through `arc:docs` only when Lark is active.

## Expert Standards

- Requirements quality follows `IEEE 29148`.
- Naming uses `Domain-Driven Design` ubiquitous language.
- Differentiation follows a `Positioning Statement`: for whom, what need, what product, why different.

## Scripts & Commands

No dedicated runtime scripts. Use `.ai-code-index/` for brownfield context and plain Markdown for output.

## Red Flags

- Generic positioning such as "all-in-one" or "intelligent platform".
- Two names for one concept.
- Claims that contradict existing code, docs, or `.lark.json`.
- Lark document creation outside `arc:docs`.

## When to Use

- **Preferred Trigger**: The user asks to define a project, write a PRD, or structure a rough idea.
- **Typical Scenario**: New project kickoff, stalled idea reset, or contributor alignment.
- **Boundary Tip**: Use `arc:clarify` for task briefs and `arc:build` for implementation.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `idea` | string | yes | User's rough project idea |
| `project_path` | string | no | Existing repository for grounding |
| `constraints` | string | no | Users, platform, deadline, rules, or non-goals |
| `prior_notes` | string | no | Drafts, slides, chat logs, or existing docs |

## Outputs

```text
Project Definition Document
- Project Name
- Project Type
- Project Positioning
- Problem
- Core Concept
- Core Capabilities
- Main Objects
- Business Flow
- Differentiators
- Keywords
- Open Questions
- Lark / .lark.json handoff, if applicable
```
