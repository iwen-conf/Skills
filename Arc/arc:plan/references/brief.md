# Brief

Turn a vague request into a brief a downstream engineer can start from without guessing.

## Where it goes

| Tier and span | Output |
|---|---|
| T0/T1, one session | Brief in the reply; no file unless the user asks |
| T2, one session | Brief in the reply before the executor's first edit |
| T2 across sessions or people | `tasks` mode; the brief becomes `00-前置约束.md` |

Ordinary multi-step work does not need persistent task documents.

## Rules

1. Inspect local context before asking anything: `arc-idx search` first (fallback per
   [code-intel](../../_shared/code-intel.md) §6), the files the request names, and `.lark.json`
   when it exists.
2. Separate facts (with `file:line` or query evidence), assumptions and open questions; never hide
   an assumption inside a task.
3. Restate the goal in concrete terms. When the request has more than one reading, list each; when a
   materially simpler approach exists, name it.
4. Ask only blocking questions: interface, persisted data, architecture trade-off, or an ambiguity
   that survives the available context. Bug fixes and work with one clearly correct answer proceed
   without questions. At most 3 questions per round, each with a recommended default.
5. Define in-scope and out-of-scope work; never expand the requested surface speculatively.
6. Acceptance is testable: a command with expected result, a test name, or a named manual check.
   Use `Given-When-Then` when behavior must be tested.
7. Preserve the existing frontend stack; never infer a stack migration from clarification. A new
   frontend records required capabilities only.
8. Concrete follow-up work is specific enough to become task-document leaves without hidden
   context.

## Question format

```text
Q1. <question>
    推荐默认：<default> — <one-line reason>
    不回答时：按默认执行，并在简报顶部列为假设。
```

## Template

```markdown
## Context
Current state with evidence (`file:line`, query, user quote).

## Goal
One sentence; observable result.

## Scope
- In: files, directories, symbols, behaviors.
- Out (non-goals): verbatim user non-goals plus excluded adjacent work.

## Constraints
Platform, environment/branch/deploy surface, compatibility, scope tokens.

## Assumptions
Each labeled; the default used when a question went unanswered.

## Open Questions
At most 3 blocking questions, each with a recommended default.

## Acceptance
- `<command>` → expected result
- Given <state>, When <action>, Then <observable outcome>
- Manual: <who> does <steps>, expects <outcome>
```

## Quality gate

- Every heading above is present; ambiguous terms are resolved or listed as open questions.
- Every acceptance line is a command, test name, Given-When-Then, or named manual check.
- Large, cross-session or tracked work names `tasks` mode as the next step before any code edit.

## Lark

When `.lark.json` exists or the user explicitly asked for Lark, hand the requirement, acceptance
criteria, task owner/status and resource keys to `lark-workflow-project-docs`; a Lark-active feature
requirement needs its `task_base` row created or updated before delivery starts. Otherwise create or
request no Lark resource.
