---
name: "arc:context"
description: "上下文预算治理与恢复：当用户提到“分析日志/看 build 输出/读大 JSON/浏览器快照/context window/context mode/恢复任务状态/resume this work”时触发；用于把大输出留在文件或检索层、生成高信号工作集，并在长会话或新会话中按需恢复。"
---

# arc:context — context-budget governance and recovery

## Overview
`arc:context` distills the durable part of the `context-mode` project into an `arc:*` workflow for coding sessions that need to survive two related failure modes:

1. Large logs, snapshots, docs, and tool output flood the conversation.
2. Long sessions lose continuity after compaction, interruption, or handoff.

Instead of replaying whole transcripts or dumping raw payloads, this skill keeps data in files or retrieval artifacts, returns only the smallest useful slice, and writes a compact task packet for restore-safe continuation.

This adaptation is intentionally explicit and repository-native. It reuses `.arc/context-hub/index.json` when available, favors verifiable file and artifact references over hidden memory, and lets downstream skills reopen only the sources they need.

Read [`references/context-mode-adaptation.md`](./references/context-mode-adaptation.md) when you need the upstream project summary and adaptation rationale. Read [`references/data-handling-playbook.md`](./references/data-handling-playbook.md) when you need concrete source-by-source handling paths.

## Quick Contract
- **Trigger**: The task is dominated by high-volume output, repeated retrieval, context-window pressure, or a need to hand off / restore work safely.
- **Inputs**: `project_path`, `task_name`, `mode`, objective/current status, candidate entrypoints, `data_sources`, and any reusable `.arc/context-hub` artifacts.
- **Outputs**: `context-brief.md`, `working-set.md`, `context-plan.md`, `compact-findings.md`, `recovery-manifest.json`, `restore-checklist.md`, and `handoff-notes.md`.
- **Quality Gate**: Every item in the packet must point to verifiable sources, mark freshness, and keep raw data outside the main conversation unless the output is guaranteed-small.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix
- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Tip** of this skill `## When to Use` shall prevail.

## Announce
Begin by stating:
> "I am using `arc:context` to control context budget first, then build or restore a bounded task packet before continuing the work."

## Teaming Requirement

- Every run should clarify `Owner`, `Executor`, and `Reviewer` responsibilities, even when the same agent fills all three roles.
- In a single-agent environment, explicitly separate what is known, what still needs verification, and what the next session should do first.

## The Iron Law
```text
SAVE FIRST, SEARCH SECOND, QUOTE LAST.
NO SNAPSHOT WITHOUT A RESUME PATH.
```

## Workflow
1. **Identify the pressure point**: decide whether the task needs `prime`, `analyze`, `snapshot`, or `restore` mode.
   - `prime`: establish a fresh task packet before deep work starts.
   - `analyze`: process large output or repeated retrieval without flooding the main conversation.
   - `snapshot`: compress the current session before context loss, handoff, or interruption.
   - `restore`: reopen a prior packet and rebuild the minimum viable context to continue.
2. **Reuse shared context first**: read `.arc/context-hub/index.json` and any relevant `.arc/<skill>/...` artifacts before scanning the repository again.
3. **Choose the narrowest viable data path**:
   - guaranteed-small output -> direct read
   - large one-shot inspection -> save/process in file or `sandbox` workflow
   - repeated lookup -> retrieval/search workflow (for example `FTS5` / `BM25` style indexing when supported)
4. **Define the working set**: select the smallest high-signal set of files, symbols, commands, artifacts, and queries required for the next step. Do not turn the packet into a repository dump.
5. **Write the packet**: capture the objective, current status, blockers, assumptions, open questions, next actions, file/artifact pointers, and data-handling path in explicit task artifacts.
6. **Mark freshness and trust level**: for each referenced artifact, note whether it is fresh, stale, or unknown. Prefer `needs-refresh` over pretending the context is current.
7. **Restore lazily**: when resuming, reopen only the required files and artifacts from the packet, then verify the next action before continuing implementation or debugging.
8. **Hand off cleanly**: if another skill takes over, the packet must tell it what to open first, what not to trust yet, and which query or decision is still pending.

## Quality Gates
- `working-set.md` must stay bounded to the next decision or execution slice; do not stuff full transcripts into it.
- `context-plan.md` must state whether the task uses direct-read, file-first, retrieval-first, or restore-first handling.
- Raw logs, snapshots, fetched docs, and similar bulk sources must stay in files or retrieval artifacts unless the output is proven small.
- `recovery-manifest.json` must contain verifiable repo paths or artifact references, not vague summaries with no source pointers.
- Freshness must be explicit for reused context-hub artifacts; stale artifacts must be marked for refresh instead of silently trusted.
- Every snapshot or restore must end with at least one concrete `next_action` and one explicit `open_question` or `no-open-questions` statement.
- `compact-findings.md` must return identifiers, counts, paths, error families, or exact retrieval anchors instead of replaying raw payloads.
- Handoff notes must distinguish `observed facts`, `assumptions`, and `pending verification`.

## Expert Standards
- Treat `context budget` as a first-class engineering constraint, not a presentation preference.
- Prefer `sandbox` or file-backed processing so only conclusions, counts, and exact anchors reach the conversation.
- When exact recall matters, use `FTS5` / `BM25` style retrieval or an equivalent indexed search pattern instead of replaying full source documents.
- Plan explicitly for `compaction`: write resumable task state, active-file lists, and follow-up queries before long or data-heavy loops.
- Use `tool-backed context` instead of transcript replay: point the next session back to files, artifacts, and commands that can be reopened and checked.
- Keep a bounded `working set` that only covers the next meaningful execution step, not the entire repository or every prior thought.
- Maintain a structured `recovery manifest` so resumption is deterministic, inspectable, and diff-friendly.
- Prefer `lazy restore`: rehydrate only the minimum files and artifacts needed now, then pull additional context on demand.
- Respect the session `token budget` by compressing status into objective, blockers, assumptions, next actions, and source pointers.
- When shared context exists, prefer reusing `.arc/context-hub` artifacts over building ad hoc memory silos that downstream skills cannot inspect.

## Scripts & Commands
- Runtime main command: `arc context`
- Workspace scaffolding: `python3 Arc/arc:context/scripts/scaffold_context_session.py --project-path <project_path> --task-name <task_name> --mode <prime|analyze|snapshot|restore>`
- Upstream adaptation notes: `Arc/arc:context/references/context-mode-adaptation.md`
- Data-source playbook: `Arc/arc:context/references/data-handling-playbook.md`
- Recommended restore flow: reopen `restore/recovery-manifest.json`, then `context/working-set.md`, then the listed entrypoints and artifact paths.

## Red Flags
- Treating `arc:context` as a generic note dump instead of a bounded recovery packet.
- Reading full logs, diffs, fetched docs, or browser trees into context without first testing a bounded path.
- Copying long transcripts into artifacts rather than pointing to files, commands, and generated products.
- Restoring from stale context without marking it or triggering `arc:init` / `arc:cartography` refresh when needed.
- Using this skill when the user really needs solution design (`arc:decide`), implementation (`arc:build`), or verification (`arc:e2e` / `arc:fix`) rather than context management.
- Hiding blockers or uncertain assumptions so the next session cannot tell what is fact versus guesswork.

## Mandatory Linkage (cannot be fought alone)

1. If repository-level indexes or cached artifacts are stale or missing, refresh via `arc:init` before trusting the packet.
2. If the task still lacks repository structure awareness, pull `arc:cartography` outputs into the working set instead of summarizing the repo from memory.
3. When the packet is being created for implementation or debugging continuation, hand off to `arc:build`, `arc:fix`, or `arc:e2e` after the restore step rather than trying to complete those jobs inside `arc:context`.
4. If the user only says "I need to continue this work" but the skill boundary is still unclear, let `arc:exec` orchestrate the follow-up after the packet is prepared.
5. If the objective itself is underspecified, call `arc:clarify` before freezing a context packet around the wrong task.

## When to Use
- **Preferred Trigger**: The user mentions large logs, build/test output, browser snapshots, fetched docs, context overflow, session restart, task handoff, or explicitly asks for a compact context packet.
- **Typical Scenario**: Large-output debugging, codebase/documentation mining, multi-day debugging, interrupted implementation, switching from one agent/session to another, or preserving task state before the context window gets noisy.
- **Boundary Tip**: Use `arc:init` for repository indexing, `arc:cartography` for repository maps, and `arc:build` / `arc:fix` / `arc:e2e` for doing the actual implementation or repair work after context is ready.

## Input Arguments
| Parameter | Type | Required | Description |
|---|---|---|---|
| `project_path` | string | yes | Absolute path to the target project root. |
| `task_name` | string | yes | Stable task identifier used for the context packet directory. |
| `mode` | enum | no | `prime` / `analyze` / `snapshot` / `restore`; default `prime`. |
| `objective` | string | no | Current task objective or desired resume outcome. |
| `entrypoints` | array | no | Key files, docs, commands, or artifact paths that the next session should reopen first. |
| `data_sources` | array | no | Logs, files, URLs, browser artifacts, or command outputs that need bounded handling. |
| `context_budget` | integer | no | Approximate token budget for the compressed packet; default `1200`. |
| `output_dir` | string | no | Override output path; default `<project_path>/.arc/context/<task_name>/`. |

## Outputs
```text
<project>/.arc/context/<task_name>/
├── plan/
│   └── context-plan.md
├── sources/
├── retrieval/
│   └── search-queries.md
├── findings/
│   └── compact-findings.md
├── context/
│   ├── context-brief.md
│   └── working-set.md
├── restore/
│   ├── recovery-manifest.json
│   └── restore-checklist.md
└── handoff/
    └── handoff-notes.md
```
