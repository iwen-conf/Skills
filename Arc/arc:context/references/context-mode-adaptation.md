# context-mode Adaptation Notes

Source examined on 2026-03-13:
- Repository: <https://github.com/mksglu/context-mode>
- README snapshot: <https://raw.githubusercontent.com/mksglu/context-mode/main/README.md>
- Upstream skill draft: <https://raw.githubusercontent.com/mksglu/context-mode/main/skills/context-mode/SKILL.md>
- Codex-oriented instructions: <https://raw.githubusercontent.com/mksglu/context-mode/main/configs/codex/AGENTS.md>

This reference captures the reusable mechanics from `context-mode` and explains how they were adapted into `arc:context`.

## What the upstream project solves

`context-mode` is a workflow for long-running coding sessions where conversation history becomes noisy, expensive, or unavailable. Its core idea is:

1. prime a task-specific context;
2. keep large raw output out of the main chat whenever possible;
3. save a compact task state before interruption;
4. restore context later from tools and files rather than replaying an entire chat.

The project packages that idea for multiple agent runtimes, including Claude Code and Codex-oriented setups.

## Key upstream commands and configuration

The upstream project combines three layers:

| Layer | Upstream shape |
|---|---|
| MCP server | `context-mode` CLI / server package, exposed as an MCP server in the supported runtimes |
| Runtime integration | hook files and runtime-specific config under `configs/` and `hooks/` |
| Utility operations | session-facing commands such as `ctx stats`, `ctx doctor`, and `ctx upgrade` |

The repository ships runtime-specific files such as:

| Upstream file | Purpose |
|---|---|
| `skills/context-mode/SKILL.md` | Core behavior description for the context workflow. |
| `configs/codex/AGENTS.md` | Codex-specific instructions for using the workflow. |
| `docs/platform-support.md` | Capability comparison across Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, and Codex CLI. |

## What `arc:context` reuses

| Reused pattern | How `arc:context` uses it |
|---|---|
| File-first large-output handling | Logs, snapshots, docs, and tool output stay in files or retrieval artifacts instead of flooding the thread. |
| Search-first retrieval | Repeated questions over the same source should move through indexed lookup instead of repeated full reads. |
| Task-focused context priming | `prime` mode creates a bounded packet before the work expands. |
| Data-heavy analysis mode | `analyze` mode records the handling path and compact findings before downstream execution continues. |
| Compression before interruption | `snapshot` mode saves the minimum restorable state instead of the full transcript. |
| Retrieval-based resumption | `restore` mode points back to files and generated artifacts rather than chat history. |
| Minimal working set | `working-set.md` lists only the next high-signal files, commands, and artifacts. |
| Session budget awareness | The skill treats context as a limited budget, not infinite memory. |

## What `arc:context` does not reuse

| Excluded behavior | Reason |
|---|---|
| Runtime-specific hooks, slash commands, or MCP server wiring | This repo standardizes on portable `arc:*` skills and explicit artifacts. |
| Hidden memory/state stores | The packet must remain inspectable and reusable by downstream skills. |
| Installer-driven platform setup | This repo is a skill collection, not a package installer product. |
| Agent-vendor-specific assumptions | `arc:context` needs a repository-native contract that works with existing `.arc` artifacts. |

## Improvements in this adaptation

`arc:context` intentionally improves on the upstream fit for this repository:

1. It reuses `.arc/context-hub/index.json` as the shared source of artifact pointers instead of inventing a parallel memory silo.
2. It adds an explicit `recovery-manifest.json` contract so restore inputs are machine-readable, diffable, and testable.
3. It formalizes four modes: `prime`, `analyze`, `snapshot`, and `restore`, so large-output control and session recovery live in one bounded skill.
4. It defines clean linkage to `arc:init`, `arc:cartography`, `arc:build`, `arc:fix`, and `arc:exec`, so context management does not absorb unrelated responsibilities.
5. It includes a local scaffolding script that creates durable task artifacts without requiring external installers.

## Recommended restore packet contents

When creating or reviewing an `arc:context` packet, keep these elements explicit:

- objective and current task status
- chosen data-handling path (`direct-read` / `file-first` / `retrieval-first` / `restore-first`)
- working set of files, commands, and artifacts
- recovery manifest with freshness state
- compact findings or query anchors for large-output tasks
- blockers, assumptions, and open questions
- concrete next action for the next session

If any of these are missing, the packet is not ready for handoff.
