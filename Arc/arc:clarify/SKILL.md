---
name: arc:clarify
description: "需求澄清：将模糊需求转为可执行任务说明。"
---

# arc:clarify

## Overview

`arc:clarify` turns vague requests into executable task briefs. It is intentionally lightweight and does not orchestrate agents, update indexes, or write long-running state. Use local `.ai-code-index/` search helpers for repository context and `aitask` only for task coordination.

## Quick Contract

- **Trigger**: The request is ambiguous, underspecified, or missing acceptance criteria.
- **Inputs**: User request, working directory, known constraints, relevant files or context provided by the user.
- **Outputs**: A concise task brief with context, scope, constraints, assumptions, open questions, and success criteria.
- **Quality Gate**: A downstream engineer can start work without guessing the intended outcome.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:build` after clarification when code changes are ready.
- Use `arc:audit` after clarification when the user wants read-only assessment.
- Use `aitask` instead when the main problem is coordination, ownership, Inbox, memory, or cross-agent workflow.

## Context Search

- When repository context is needed, first use `.ai-code-index/search.sh "query"` after confirming the index is current.
- If there is no local index or results look stale, run `.ai-code-index/reindex.sh`.
- Use `.ai-code-index/struct-search.sh` for syntax patterns and `.ai-code-index/symbols.sh` for definitions.
- Use `rg` only for narrow exact follow-up, new files, non-indexed files, or fallback when the index is insufficient.

## Announce

Begin by stating clearly:
"I am using `arc:clarify` to turn the request into an executable task brief."

## The Iron Law

```text
NO EXECUTION WITHOUT CONTEXT, CONSTRAINTS, AND SUCCESS CRITERIA.
```

## Workflow

1. Restate the user goal in concrete terms.
2. Identify scope boundaries: files, modules, users, environments, and excluded work.
3. Identify constraints: compatibility, security, performance, timeline, style, and verification expectations.
4. Surface assumptions and high-impact unknowns.
5. Ask only the minimum necessary questions if execution would otherwise be risky.
6. Produce a task brief that can feed directly into implementation or audit.

## Code Rot Gates

Full catalog: [`docs/code-rot-taxonomy.md`](../../docs/code-rot-taxonomy.md). Clarification is where two whole-project rot sources are cut off at the root:

- Prune to the minimum surface (#32): translate the request into the smallest API/feature set that satisfies it. If the requirement is one endpoint, the brief specifies one — flag any speculative scenario expansion as out of scope.
- Fix the state contract up front (#7): when the feature has states, enumerate the full state set and legal transitions in the brief so implementation does not let it drift 8 → 10 → 7 later.
- Pin shared contracts (#2, #18, #19): name the canonical field names, the error-code type (`int` vs `string`), and the response envelope shape in the brief so downstream work cannot diverge.

## Quality Gates

- The brief includes `Context`, `Task`, `Scope`, `Constraints`, and `Success Criteria`.
- Ambiguous terms are resolved or explicitly marked as open questions.
- Assumptions are separated from facts.
- The output does not create artificial process overhead.

## Expert Standards

- Requirements quality follows `IEEE 29148`: complete, consistent, verifiable, and traceable.
- User stories should satisfy `INVEST` where relevant.
- Acceptance criteria should use `Given-When-Then` when behavior needs verification.

## Scripts & Commands

No dedicated runtime scripts. Use `.ai-code-index/` for repository search, then normal inspection tools such as `rg`, `find`, and file reads for exact follow-up.

## Red Flags

- Asking many questions before reading available context.
- Producing a vague restatement instead of executable criteria.
- Hiding assumptions inside the task statement.
- Turning a small clarification into a heavy planning ceremony.
- Letting scope balloon into speculative endpoints the requirement never asked for (#32).

## When to Use

- **Preferred Trigger**: Requirements are vague or missing execution-critical context.
- **Typical Scenario**: The user asks for a change but scope, constraints, or verification are unclear.
- **Boundary Tip**: If the user already gave enough detail, skip this skill and proceed directly.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `request` | string | yes | Original user request |
| `workdir` | string | no | Target project directory |
| `known_context` | string | no | Existing constraints or files already provided |

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
```
