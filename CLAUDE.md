# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repo Is

A collection of Claude Code Skills (custom slash-command plugins) under the **`arc:`** namespace. Each top-level directory is a self-contained Skill defined by a `SKILL.md` frontmatter file. Skills are invoked via `/arc:<name>` in Claude Code and extend the agent with specialized workflows.

## Skill Inventory

| Directory | Skill Name | Invoke | Purpose |
|-----------|-----------|--------|---------|
| `simulate/` | arc:simulate | `/arc:simulate` | 通过 `agent-browser` 模拟真实用户进行 E2E 浏览器测试，生成含截图的结构化报告 |
| `triage/` | arc:triage | `/arc:triage` | 分析 arc:simulate 的失败报告，定位根因、修复缺陷、执行回归验证 |
| `loop/` | arc:loop | `/arc:loop` | 管理 tmux 会话启动/重启服务，循环执行 arc:simulate 直到 PASS 或达到迭代上限 |
| `refine/` | arc:refine | `/arc:refine` | 扫描 CLAUDE.md 层级索引，为模糊的用户 prompt 补充项目上下文 |
| `deliberate/` | arc:deliberate | `/arc:deliberate` | 三模型（Claude/Codex/Gemini）多视角审议，通过文件总线与 `codeagent-wrapper` CLI 协作，使用 OpenSpec 生成结构化计划 |
| `review/` | arc:review | `/arc:review` | 按企业级七维度框架（ISO/IEC 25010 + TOGAF）深度评审软件项目，三模型对抗式分析，输出诊断报告与改进路线图 |

## Skill Dependency Chain

```
arc:refine
  └─▶ arc:deliberate  (consumes enhanced-prompt.md)

arc:simulate
  └─▶ arc:triage        (consumes run_dir reports/failures)
        └─▶ arc:loop  (orchestrates fix→restart→retest cycle)

arc:review  (独立运行，不依赖其他 Skill)
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
- **Working directory for arc:review**: `.arc/review/<project-name>/` (inside the target project). 严禁修改被评审项目的源代码，只在此目录下产出评审文件。
- **OpenSpec integration**: arc:deliberate 的 Phase 3 使用 [OpenSpec](https://github.com/Fission-AI/OpenSpec) CLI（`openspec`）生成结构化计划。OpenSpec 在 `.arc/deliberate/<task-name>/` 内初始化，artifact 写入 `openspec/changes/<task-name>/` 下。工作流：`openspec init` → `openspec new change` → `openspec instructions` → `openspec validate` → `openspec archive`。
