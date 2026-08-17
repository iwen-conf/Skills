---
name: arc:prewalk
version: 1.0.0
description: >
  Routes same-session work from a frontier Guide to a cheaper Executor after recon, a
  bounded todo, and the first production edit. Use when the user asks for prewalk, 省token,
  多模型, Guide/Executor swap, or same-session cost routing. Not for cold plan-postcard
  handoff across a new session.
---
# arc:prewalk

## Overview

`arc:prewalk` is the default **same-session multi-model cost routing** contract for Arc code work. A frontier **Guide** explores, writes a bounded todo, lands the first production edit, then an **Executor** continues in the **same trajectory**. It does not replace `arc:sdlc` task docs (cross-session/human progress) and does not implement product features by itself—it constrains how `arc:build` / `arc:fix` / `arc:frontend` should route models when the runtime can swap.

Full contract: [`references/prewalk.md`](references/prewalk.md) (same text as monorepo [`docs/prewalk.md`](../../docs/prewalk.md)).

## Quick Contract

- **Trigger**: Implementation or repair will use more than one model class for cost/speed, or the user asks for prewalk / model handoff / cheap executor after planning.
- **Inputs**: Task goal, optional `arc:sdlc` task docs, guide/executor capability profiles, runtime that can keep context across a model swap.
- **Outputs**: Same worktree progress as the parent skill, with an explicit guide→executor swap after first production edit; residual risks if swap or escalate happened.
- **Quality Gate**: Executor never cold-starts from a plan postcard; trajectory + todo survive the swap; planning-only instruction is pruned.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Intent Router

| When | Load |
|---|---|
| Same-session Guide→Executor routing | [`references/prewalk.md`](references/prewalk.md) |
| Unclear scope | `arc:clarify` |
| Large tracked work first | `arc:sdlc` |
| Domain implementation | parent `arc:build` / `arc:fix` / `arc:frontend` |
| Runtime cannot swap models | oneshot Guide, no fake plan-postcard |

## Context Search

- MUST prefer `arc-idx search` / `ast` / `symbol` during the Guide phase so expensive reads are purposeful.
- MUST NOT force the Executor to re-read the entire Guide corpus “to catch up” if the trajectory already contains those tool results.
- If task docs exist, MUST read the active subtask and `进度跟踪表.md` during Guide, then keep them updated after swap.


## Red Lines

```text
NO PLAN-POSTCARD HANDOFF AS THE DEFAULT COST OPTIMIZATION.
NO MODEL SWAP BEFORE FIRST PRODUCTION EDIT + BOUNDED TODO.
NO CONTEXT WIPE AT SWAP.
NO PLANNING INSTRUCTION LEFT IN EXECUTOR CONTEXT.
NO CLAIM OF "SAVED MONEY" IF FRONTIER STILL PAID TO READ EVERYTHING AND EXECUTOR RE-READ IT COLD.
```

First-Edit Swap: Guide orients, writes a bounded todo, lands the first **production code** edit, then Executor inherits the same trajectory. Docs-only or progress-table-only edits do not count.

## Hard Constraints

- MUST treat agent cost as approximately `O(reads)`, not `O(edits)`.
- MUST run Guide with a hidden planning instruction equivalent to: plan deeply → capture plan as a bounded todo with per-item verification → start executing.
- MUST initialize a **bounded** todo (prefer ≤ 12 items; hard stop well below vanity 60-item lists) with a validation step on each item when the runtime supports todos; otherwise use `arc:sdlc` progress rows as the steering surface.
- MUST swap only after the first **production code** edit/write that advances the active subtask (docs-only, progress-table-only, or comment-only does not count).
- MUST prune the hidden planning instruction at swap so the Executor does not stay in “planning identity”.
- MUST preserve tool trajectory, todo/progress state, and the first edit as an in-context example.
- MUST NOT use fixed-turn swap (e.g. “always after turn 4”).
- MUST NOT schedule a fresh Executor `task_ref` whose only memory is a plan document path.
- MUST escalate back to Guide (or oneshot Guide) when Executor loops on the same failure, invents “done”, or cannot recover after a small number of attempts.
- MUST still obey `arc:sdlc` progress gates for tracked work after swap.
- When the runtime cannot swap models mid-session, MUST oneshot on a single profile and MUST NOT fake prewalk via cold plan handoff.

## Workflow

1. Confirm the parent skill (`arc:build` / `arc:fix` / `arc:frontend`) and that scope is executable.
2. If large/tracked: ensure `arc:sdlc` docs and progress table are current (Guide may update docs; that alone is not a swap gate).
3. Start on **Guide** with the hidden planning instruction.
4. Explore with search/read until a concrete approach is grounded in repo evidence.
5. Write/update the bounded todo (and progress table rows for tracked work).
6. Land the **first production edit** that starts the active checklist item.
7. **Swap Gate**: todo present + first production edit landed → switch to **Executor**, prune planning instruction, keep trajectory.
8. Executor continues edits, tests, and todo checkoffs; update `进度跟踪表.md` when tracked.
9. On stuck/loop/premature-done: escalate to Guide briefly, or finish oneshot Guide for the remainder; record the escalate in the handoff summary.
10. Parent skill still owns verification evidence and Lark handoff rules.

## Quality Gates

- Swap happened only at first production edit (or oneshot was chosen for a documented reason).
- Executor context still contains Guide exploration results and the first edit.
- Planning-only instruction is absent after swap.
- Todo/progress steering remains active; Executor is not asked to “re-derive the plan from a postcard”.
- Tracked work still has non-stale `进度跟踪表.md` / subtask `状态`.
- No silent double-read tax: Guide did not dump a plan for a cold Executor to re-ingest as the primary memory.


## Scripts & Commands

No dedicated scripts. Runtime adapters implement mid-session model swap + instruction prune. Local progress uses `arc:sdlc` files and in-harness todos when available.

## Red Flags

- “Opus plans, Flash implements from the plan file.”
- Swapping after N turns with no edit.
- Swapping after only markdown task-doc writes.
- Clearing context and pasting a plan into a new cheap agent.
- 40–60 item todos with batch checkoffs and no verification.
- Claiming cost savings while Guide already read the whole surface at frontier prices and Executor re-reads it cold.
- Leaving “you are planning, do not edit” instructions in Executor context.

## When to Use

- **Preferred Trigger**: Multi-model cost routing for implementation/repair inside one session; user asks for prewalk, cheap executor after recon, or “贵模型只规划/起手”.
- **Typical Scenario**: Medium SWE tasks where recon dominates tokens; Guide orients and cuts once, Executor finishes tests and polish.
- **Boundary Tip**: Skip swap for trivial one-file edits, when the runtime cannot preserve trajectory across models, or when the user requires a single model end-to-end. Task-doc authoring alone is `arc:sdlc`, not prewalk.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `task` | string | yes | Implementation or repair goal |
| `guide_profile` | string | no | Frontier/deep capability profile |
| `executor_profile` | string | no | Cheap/quick capability profile |
| `project_path` | string | no | Repository root |
| `task_docs_path` | string | no | Active `arc:sdlc` task directory when tracked |

## Outputs

```text
Prewalk Handoff
- Guide phase summary (what was read, approach)
- First production edit (files)
- Swap point (or oneshot reason)
- Executor continuation / escalate notes
- Todo / progress status
- Parent skill verification still required
```
