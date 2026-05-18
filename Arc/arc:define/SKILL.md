---
name: arc:define
description: "项目定义：将模糊想法整理为结构化的项目/产品定义文档（PRD / Blueprint）。"
---

# arc:define

## Overview

`arc:define` is the project-genesis skill. It turns a vague idea or scattered notes into a structured Project Definition Document covering positioning, problem, core concepts, features, objects, flow, differentiators, and keywords. It does not write code, do task-level decomposition, or run any execution; for those, route to other Arc skills.

## Quick Contract

- **Trigger**: The user has a project idea, partial draft, or rough PRD that needs structuring.
- **Inputs**: User-provided idea, target users, known constraints, optional prior notes.
- **Outputs**: A Project Definition Document strictly following [`references/template.md`](references/template.md).
- **Quality Gate**: Every template section is concrete, non-empty, and free of filler; gaps are explicitly listed as open questions.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` after definition when individual tasks need an executable brief.
- Use `arc:build` when a defined feature is ready to be implemented.
- Use `arc:audit` when an existing project's definition or scope needs read-only review.
- Use `aitask` when the genesis work needs cross-agent coordination or persisted memory.

## Context Search

- For brownfield projects, first run `.ai-code-index/search.sh "query"` to surface existing modules, naming, and domain language before drafting the definition.
- If the index is missing or stale, run `.ai-code-index/reindex.sh`.
- Use `.ai-code-index/symbols.sh` to extract the project's real entity names instead of inventing new ones.
- Use `rg` only for narrow exact follow-up, new files, non-indexed files, or fallback when the index is insufficient.

## Announce

Begin by stating clearly:
"I am using `arc:define` to produce a structured project definition document."

## The Iron Law

```text
NO SECTION WITHOUT A CONCRETE CONCEPT.
```

## Workflow

1. Restate the user's idea in one or two sentences to confirm intent.
2. Map the idea onto each section of [`references/template.md`](references/template.md).
3. Fill known sections directly; mark unknowns as open questions rather than guessing.
4. Force each section past filler: positioning ≠ "全方位 / 一站式"; core concept ≠ a feature list.
5. Iterate with the user only on the smallest set of gaps blocking the definition.
6. Deliver the final document in the template's exact section order.

## Quality Gates

- Every section in `template.md` is present, in order, and non-empty.
- "项目定位" is 1–3 sentences and not interchangeable with another project.
- "核心概念" names a metaphor or central abstraction, not a feature.
- "核心功能" lists 3–6 capabilities, each verifiable from the user's perspective.
- "主要对象" lists the real domain entities the system will model.
- "业务流程" is expressible as a short arrow chain (`A → B → C`).
- Open questions are listed separately and do not pollute the body.

## Expert Standards

- Requirements quality follows `IEEE 29148`: complete, consistent, verifiable.
- Concept framing uses a `Domain-Driven Design` lens — name the ubiquitous language.
- Differentiators follow a `Positioning Statement` shape: for *who*, that *needs*, our project *does*, unlike *alternatives*.
- Keywords serve discoverability, not marketing; prefer concrete domain terms over hype.

## Scripts & Commands

No dedicated runtime scripts. Use `.ai-code-index/` for brownfield context, then plain Markdown editing tools. The output is a single document — do not generate side artifacts.

## Red Flags

- Filling sections with "全方位 / 智能化 / 一站式" or other interchangeable phrasing.
- Collapsing 核心概念 into a feature list.
- Inventing entities that contradict an existing codebase when one exists.
- Expanding scope into task decomposition or implementation planning — route to `arc:clarify` / `arc:build` instead.
- Producing a parallel PRD format that diverges from `template.md`.

## When to Use

- **Preferred Trigger**: The user says "梳理一下这个项目 / 写个 PRD / 把想法整理成定义 / 定义新项目".
- **Typical Scenario**: New project genesis, rebooting a stalled idea, aligning multiple contributors on what a project actually is.
- **Boundary Tip**: If the user has a concrete task and only needs scope/constraints, use `arc:clarify`; this skill is for the project, not the task.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `idea` | string | yes | User's project idea or rough description |
| `project_path` | string | no | Existing repository, for brownfield grounding |
| `constraints` | string | no | Known constraints: users, platform, deadline, regulations |
| `prior_notes` | string | no | Earlier drafts, slides, or chat logs to consolidate |

## Outputs

```text
Project Definition Document
- 项目名称
- 项目类型
- 项目定位
- 解决的问题
- 核心概念
- 核心功能
- 主要对象
- 业务流程
- 项目特色
- 项目关键词
- Open Questions (only if any section is unresolved)
```
