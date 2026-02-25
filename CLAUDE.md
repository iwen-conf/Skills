# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 变更记录 (Changelog)

| 时间 | 操作 |
|------|------|
| 2026-02-24T16:30:00 | arc:init 三模型协作生成模块级 CLAUDE.md（simulate/triage/loop/review/deliberate/init/agent/refine） |
| 2026-02-24 | 初始版本，定义 Skill 清单、依赖链、架构和约定 |

---

## What This Repo Is

A collection of Claude Code Skills (custom slash-command plugins) under the **`arc:`** namespace. Each top-level directory is a self-contained Skill defined by a `SKILL.md` frontmatter file. Skills are invoked via `/arc:<name>` in Claude Code and extend the agent with specialized workflows.

## Skill Inventory

| Directory | Skill Name | Invoke | Purpose |
|-----------|-----------|--------|---------|
| `agent/` | arc:agent | `/arc:agent` | 智能调度 agent，分析用户需求后选择合适的 arc: skill，通过 oh-my-opencode Agent 系统调度执行任务 |
| `simulate/` | arc:simulate | `/arc:simulate` | 通过 `agent-browser` 模拟真实用户进行 E2E 浏览器测试，生成含截图的结构化报告 |
| `triage/` | arc:triage | `/arc:triage` | 分析 arc:simulate 的失败报告，定位根因、修复缺陷、执行回归验证 |
| `loop/` | arc:loop | `/arc:loop` | 管理 tmux 会话启动/重启服务，循环执行 arc:simulate 直到 PASS 或达到迭代上限 |
| `refine/` | arc:refine | `/arc:refine` | 扫描 CLAUDE.md 层级索引，为模糊的用户 prompt 补充项目上下文 |
| `deliberate/` | arc:deliberate | `/arc:deliberate` | 多 Agent 多视角审议，使用 OpenSpec 生成结构化计划 |
| `review/` | arc:review | `/arc:review` | 按企业级七维度框架（ISO/IEC 25010 + TOGAF）深度评审软件项目，多 Agent 对抗式分析，输出诊断报告与改进路线图 |
| `init/` | arc:init | `/arc:init` | 多 Agent 协作生成项目层级式 CLAUDE.md 索引体系，深度扫描项目结构后输出根级+模块级 CLAUDE.md |

## 模块文档索引

每个 Skill 目录下都有模块级 CLAUDE.md 文件，提供详细的模块职责、入口与启动、对外接口等信息：

| 模块 | 文档 | 核心内容 |
|------|------|---------|
| simulate/ | [CLAUDE.md](./simulate/CLAUDE.md) | E2E 测试执行、脚本接口、报告产物结构 |
| triage/ | [CLAUDE.md](./triage/CLAUDE.md) | 缺陷分析流程、Fix Packet 结构、回归验证 |
| loop/ | [CLAUDE.md](./loop/CLAUDE.md) | tmux 服务管理、配置模型、迭代回归流程 |
| review/ | [CLAUDE.md](./review/CLAUDE.md) | 七维度评审框架、多Agent对抗、改进路线图 |
| deliberate/ | [CLAUDE.md](./deliberate/CLAUDE.md) | 四阶段审议流程、OpenSpec 集成、共识报告模型 |
| init/ | [CLAUDE.md](./init/CLAUDE.md) | 五阶段生成流程、CLAUDE.md 结构规范、校验体系 |
| agent/ | [CLAUDE.md](./agent/CLAUDE.md) | 调度决策树、执行预览、多Agent任务分配 |
| refine/ | [CLAUDE.md](./refine/CLAUDE.md) | CLAUDE.md 索引扫描、差距分析、Prompt 增强 |

## Skill Dependency Chain

```
arc:agent ────┬─▶ arc:init         (项目初始化)
              ├─▶ arc:refine       (问题细化)
              │     └─▶ arc:deliberate
              ├─▶ arc:review       (项目评审)
              ├─▶ arc:simulate     (E2E 测试)
              │     └─▶ arc:triage
              │           └─▶ arc:loop
              └─▶ Task API dispatch (category/subagent routing)

arc:init  (独立运行；输出的 CLAUDE.md 被 arc:refine 消费)
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

- **搜索约定**：
  - 禁止使用 Grep/Bash grep/ripgrep 等低效方式搜索代码
  - 项目外部搜索：使用 `Exa` MCP 服务（搜索文档、OSS、GitHub 等）
  - 项目内部搜索：使用 `Ace-tool_search_context` MCP 服务（语义搜索代码库）
- **Markdown table alignment** is enforced: header, separator, and data rows must have identical column counts. Always validate with `check_artifacts.py --strict` after generating/editing Markdown reports.
- **Reports directory (`reports/`)** contains plaintext secrets (by design, for developer reproducibility). It is gitignored and must never be committed.
- **`run_id` format**: `YYYY-MM-DD_HH-mm-ss_<short>`, optionally suffixed with `_iterNN` in retest loops.
- **Screenshot naming**: `s<4-digit-step>_<slug>.png` (e.g., `s0007_after-submit.png`).
- **Agent 调用约定**：
  - **Task API**: `Task(category="<domain>", load_skills=[...], description="...", prompt="...", run_in_background=true/false)`
  - **Subagent API**: `Task(subagent_type="<agent>", load_skills=[...], description="...", prompt="...", run_in_background=true/false)`
  - **可用 Category**: `visual-engineering` | `ultrabrain` | `deep` | `artistry` | `quick` | `unspecified-low` | `unspecified-high` | `writing`
  - **可用 Subagent**: `explore`(代码搜索) | `librarian`(文档搜索) | `oracle`(架构咨询) | `metis`(预分析) | `momus`(质量审查)
  - **Session 延续**: 每次 Task() 返回 session_id，用 `session_id="<id>"` 延续多轮对话
- **Working directory for arc:agent**: `.arc/agent/` (调度记录在此目录)。
- **Working directory for arc:deliberate**: `.arc/deliberate/<task-name>/` (inside the target project).
- **Working directory for arc:review**: `.arc/review/<project-name>/` (inside the target project). 严禁修改被评审项目的源代码，只在此目录下产出评审文件。
- **Working directory for arc:init**: `.arc/init/` (inside the target project). 工作文件在此目录下，最终 CLAUDE.md 输出到项目目录树中。只写 CLAUDE.md，不改源码。
- **OpenSpec integration**: arc:deliberate 的 Phase 3 使用 [OpenSpec](https://github.com/Fission-AI/OpenSpec) CLI（`openspec`）生成结构化计划。OpenSpec 在 `.arc/deliberate/<task-name>/` 内初始化，artifact 写入 `openspec/changes/<task-name>/` 下。工作流：`openspec init` → `openspec new change` → `openspec instructions` → `openspec validate` → `openspec archive`。
