# Cross-Agent Collaboration Protocol

Applies identically to **Claude Code**, **Codex**, **Grok Build**, and **Gemini CLI**. Switching agents mid-feature must require zero chat copy-paste.

## 1. Single Source of Truth

1. **Git Worktree**: Authoritative for source bytes, branches, and diffs.
2. **Project Ledger (`.arc/arc.db` via `arc` + `.arc/handoff/`)**: Authoritative for task state (`draft -> ready -> running -> verifying -> done`), optimistic CAS versions (`--expected-version`), frozen contracts, and verification receipts.
3. **Zero Memory Drift**: Private chat history or `.magi/sessions` never overrides `.arc/arc.db` or `git status`.

## 2. Mandatory Entry Sequence

1. Run `arc-wave status` (or `arc capabilities --json && arc db status --repo .` when `.arc/arc.db` exists) before broad file scanning.
2. Resume active tasks from recorded `write_scope`, CAS `version`, and the last failing gate command—never redo verified phases.

## 3. Complexity Routing

- **L0 Direct (`<= 3` files, no public API / SQL schema / auth change)**: `arc-code` -> `gate.sh check` -> `arc update --changed-only` -> done.
- **L1 Standard (Module feature or bugfix)**: freeze contract -> `arc-code` -> `arc-verify` -> `arc task complete`.
- **L2 Multi-Agent (Cross-module, schema migration, or parallel worktrees)**:
  1. **Wave 0**: Freeze OpenAPI / Protobuf / SQL schema contracts first.
  2. **Wave Derivation**: Pass flat code anchors (`symbol` / `path`) to `arc-wave plan` + `arc task create`—never hand-write static ETA DAG YAML.
  3. **Execution**: Disjoint `write_scope` waves run in isolated `git worktree` directories; overlapping tasks coalesce or serialize automatically.

## 4. Handoff & Completion Invariants

- **Switching Agents Mid-Task**: Run `arc update --changed-only` and `arc-wave handoff --task <ID> --agent <name> --note "..."` so the next agent inherits exact `HEAD`, `write_scope`, and gate logs.
- **Anti-Self-Grading**: Prefer a different agent family for `arc-verify` / `arc-audit` than the `arc-code` author.
- **Done Definition**: `gate.sh check` PASS + `arc index flush --repo .` + revision-bound `arc task complete --id <ID> --expected-version <N>`.
