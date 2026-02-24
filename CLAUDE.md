# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repo Is

A collection of Claude Code Skills (custom slash-command plugins) under the **`arc:`** namespace. Each top-level directory is a self-contained Skill defined by a `SKILL.md` frontmatter file. Skills are invoked via `/arc:<name>` in Claude Code and extend the agent with specialized workflows.

## Skill Inventory

| Directory | Skill Name | Invoke | Purpose |
|-----------|-----------|--------|---------|
| `simulate/` | arc:simulate | `/arc:simulate` | E2E browser testing via `agent-browser` — simulates real users, produces structured reports with screenshots |
| `triage/` | arc:triage | `/arc:triage` | Triages failures from arc:simulate, locates root cause, applies fix, runs regression |
| `loop/` | arc:loop | `/arc:loop` | Manages tmux sessions for service start/restart, then loops arc:simulate until PASS or max iterations |
| `refine/` | arc:refine | `/arc:refine` | Scans CLAUDE.md hierarchy to enrich vague user prompts with project context |
| `deliberate/` | arc:deliberate | `/arc:deliberate` | Multi-model (Claude/Codex/Gemini) deliberation via file-based message bus and `codeagent-wrapper` CLI |

## Skill Dependency Chain

```
arc:refine
  └─▶ arc:deliberate  (consumes enhanced-prompt.md)

arc:simulate
  └─▶ arc:triage        (consumes run_dir reports/failures)
        └─▶ arc:loop  (orchestrates fix→restart→retest cycle)
```

## Architecture

Each Skill follows the same structure:

- **`SKILL.md`** — The skill definition (frontmatter `name`/`description`/`version` + full instructions). This is the authoritative specification; always read it before modifying a skill.
- **`scripts/`** — Python helper scripts callable from the agent (use `--help` first).
- **`references/`** — Decision trees, runbooks, templates consumed by the skill workflow.
- **`templates/`** / **`examples/`** — Scaffold templates and sample data (simulate only).
- **`assets/`** — Config examples (loop only).

There is no build step. Skills are pure Markdown + Python scripts.

## Key Scripts

All scripts are Python 3 and accept `--help`. No virtual environment is required for core functionality.

### simulate/scripts/
| Script | What it does |
|--------|-------------|
| `scaffold_run.py` | Creates `reports/<run_id>/` directory skeleton with report templates (`--pack e2e` or `--pack full-process`) |
| `compile_report.py` | Compiles `events.jsonl` into `action-log.compiled.md` and `screenshot-manifest.compiled.md`; can update `report.md` in-place |
| `check_artifacts.py` | Quality gate — validates required files, screenshot references, JSONL parsing, Markdown table alignment (`--strict`) |
| `beautify_md.py` | Formats Markdown files via `mdformat` (requires `pip install mdformat`) |
| `new_defect.py` | Generates `failures/failure-XXXX.md` from CLI args |
| `accounts_to_personas.py` | Converts `accounts.jsonc` → personas JSON for replay |

### triage/scripts/
| Script | What it does |
|--------|-------------|
| `triage_run.py` | Best-effort triage summary of a run_dir; outputs `triage.md` / `triage.json` |

### loop/scripts/
| Script | What it does |
|--------|-------------|
| `uxloop_tmux.py` | Starts/restarts services in tmux panes from a JSON config; supports `--reset-window`, `--wait-ready` |
| `uxloop_cleanup.py` | Kills tmux sessions/windows after testing is done |

## Conventions

- **SKILL.md is the source of truth** for each skill's behavior, inputs, outputs, and constraints. Do not contradict it.
- **Markdown table alignment** is enforced: header, separator, and data rows must have identical column counts. Always validate with `check_artifacts.py --strict` after generating/editing Markdown reports.
- **Reports directory (`reports/`)** contains plaintext secrets (by design, for developer reproducibility). It is gitignored and must never be committed.
- **`run_id` format**: `YYYY-MM-DD_HH-mm-ss_<short>`, optionally suffixed with `_iterNN` in retest loops.
- **Screenshot naming**: `s<4-digit-step>_<slug>.png` (e.g., `s0007_after-submit.png`).
- Skills that call external models use `codeagent-wrapper` CLI at `~/.claude/bin/codeagent-wrapper` with `--backend codex` or `--backend gemini`. Claude is invoked via Task subagent (not codeagent-wrapper).
- **Working directory for arc:deliberate**: `.arc/deliberate/<task-name>/` (inside the target project).
