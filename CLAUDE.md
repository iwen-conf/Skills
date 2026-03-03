# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 变更记录 (Changelog)

| 时间 | 操作 |
|------|------|
| 2026-03-03 | 新增 arc:score 与 arc:gate，引入量化评分与 CI 门禁能力；增强 arc:review 和 arc:deliberate |
| 2026-02-28 | 新增 arc:init:full 与 arc:init:update，拆分全量初始化与增量更新能力；arc:init 改为智能调度器 |

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
| `implement/` | arc:implement | `/arc:implement` | 将方案落地为工程实现，输出实现计划、执行日志与交接摘要 |
| `review/` | arc:review | `/arc:review` | 按企业级七维度框架（ISO/IEC 25010 + TOGAF）深度评审软件项目，多 Agent 对抗式分析，输出诊断报告与改进路线图 |
KZ|| `init/` | arc:init | `/arc:init` | 智能调度器，自动判断全量(full)或增量(update)模式 |
QB|| `init-full/` | arc:init:full | `/arc:init:full` | 全量生成项目层级式 CLAUDE.md 索引体系，深度扫描+多Agent分析 |
QB|| `init-update/` | arc:init:update | `/arc:init:update` | 增量更新 CLAUDE.md，基于指纹检测变更，仅更新受影响模块 |
| `ip-docs/` | arc:ip-docs | `/arc:ip-docs` | 基于项目上下文与审查结论撰写软著/专利申请文档草稿 |
| `score/` | arc:score | `/arc:score` | 量化评分与 Code Smell 检测，为评审提供量化数据支撑 |
| `gate/` | arc:gate | `/arc:gate` | CI 质量门禁，基于评分数据执行可配置的阻断判定 |

## 模块文档索引

每个 Skill 目录下都有模块级 CLAUDE.md 文件，提供详细的模块职责、入口与启动、对外接口等信息：

| 模块 | 文档 | 核心内容 |
|------|------|---------|
| simulate/ | [CLAUDE.md](./simulate/CLAUDE.md) | E2E 测试执行、脚本接口、报告产物结构 |
| triage/ | [CLAUDE.md](./triage/CLAUDE.md) | 缺陷分析流程、Fix Packet 结构、回归验证 |
| loop/ | [CLAUDE.md](./loop/CLAUDE.md) | tmux 服务管理、配置模型、迭代回归流程 |
| review/ | [CLAUDE.md](./review/CLAUDE.md) | 七维度评审框架、多Agent对抗、改进路线图 |
| deliberate/ | [CLAUDE.md](./deliberate/CLAUDE.md) | 四阶段审议流程、OpenSpec 集成、共识报告模型 |
| implement/ | [CLAUDE.md](./implement/CLAUDE.md) | 方案落地、变更实施、验证与交接产物 |
| init/ | [CLAUDE.md](./init/CLAUDE.md) | 五阶段生成流程、CLAUDE.md 结构规范、校验体系 |
| init-full/ | [CLAUDE.md](./init-full/CLAUDE.md) | 全量初始化流程、深度扫描策略、多Agent协作生成 |
| init-update/ | [CLAUDE.md](./init-update/CLAUDE.md) | 增量更新机制、变更指纹检测、模块级差异更新 |
| agent/ | [CLAUDE.md](./agent/CLAUDE.md) | 调度决策树、执行预览、多Agent任务分配 |
| refine/ | [CLAUDE.md](./refine/CLAUDE.md) | CLAUDE.md 索引扫描、差距分析、Prompt 增强 |
| ip-audit/ | [CLAUDE.md](./ip-audit/CLAUDE.md) | 知识产权可行性审查、风险矩阵、交接产物定义 |
| ip-docs/ | [CLAUDE.md](./ip-docs/CLAUDE.md) | 软著/专利文档草稿写作、模板与交接消费 |
| score/ | [CLAUDE.md](./score/CLAUDE.md) | 量化评分、Code Smell 检测、评分聚合 |
| gate/ | [CLAUDE.md](./gate/CLAUDE.md) | CI 门禁、阈值配置、豁免清单 |

## Skill Dependency Chain

```
arc:agent ────┬─▶ arc:init         (智能调度 → full/update)
              ├─▶ arc:refine       (问题细化)
              │     └─▶ arc:deliberate
              ├─▶ arc:implement    (方案落地实现)
              ├─▶ arc:score        (量化评分) ──▶ arc:review
              │                              └─▶ arc:gate (CI 门禁)
              ├─▶ arc:review       (项目评审)
              ├─▶ arc:ip-audit     (知识产权可行性审查)
              │     └─▶ arc:ip-docs
              ├─▶ arc:simulate     (E2E 测试)
              │     └─▶ arc:triage
              │           └─▶ arc:loop
              └─▶ Task API dispatch (subagent_type routing)

arc:init  (独立运行；输出的 CLAUDE.md 被 arc:refine 消费)
arc:score  (消费 arc:init 产物；输出量化数据给 arc:review 和 arc:gate)
arc:implement  (消费 deliberation/refine 结果；输出实现交接供 review/simulate 使用)
arc:review  (消费 arc:score 量化数据；输出评审报告)
arc:gate  (消费 arc:score 数据；执行 CI 门禁判定)
arc:ip-audit  (优先读取 arc:init/arc:review 产物；输出交接 JSON 给 arc:ip-docs)
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

### ip-audit/scripts/
| Script | What it does |
|--------|-------------|
| `scaffold_audit_case.py` | Creates `.arc/ip-audit/<project>/` scaffold with context, analysis, reports, and handoff dirs |
| `render_audit_report.py` | Renders feasibility report/checklist and outputs `handoff/ip-drafting-input.json` |

### ip-docs/scripts/
| Script | What it does |
|--------|-------------|
| `scaffold_drafting_case.py` | Creates `.arc/ip-docs/<project>/` scaffold with context/copyright/patent/report dirs |
| `render_ip_documents.py` | Renders copyright/patent draft documents from handoff JSON and templates |

### implement/scripts/
| Script | What it does |
|--------|-------------|
| `scaffold_implement_case.py` | Creates `.arc/implement/<task>/` scaffold with context, plan, execution, reports and handoff dirs |
| `render_implementation_report.py` | Renders implementation delivery report and handoff summary |

### score/scripts/
| Script | What it does |
|--------|-------------|
| `scaffold_score_case.py` | Creates `.arc/score/<project>/` scaffold with context, analysis, score and handoff dirs |
| `detect_smell.py` | Code Smell 检测，支持 19 项规则 |
| `grade_bugfix.py` | Bugfix 分级 (A/B/C) + 自动打标 |
| `aggregate_score.py` | 评分聚合，输出综合评分和维度评分 |

### gate/scripts/
| Script | What it does |
|--------|-------------|
| `check_gate.py` | CI 门禁检查，支持 warn/strict/strict_dangerous 模式 |

### review/scripts/
| Script | What it does |
|--------|-------------|
| `integrate_score.py` | 将 arc:score 量化数据集成到 arc:review |

### deliberate/scripts/
| Script | What it does |
|--------|-------------|
| `generate_bdd_seed.py` | 从共识报告生成 BDD 场景种子 |

## Conventions

- **搜索约定**：
  - 禁止使用 Grep/Bash grep/ripgrep 等低效方式搜索代码
  - 项目外部搜索：使用 `Exa` MCP 服务（搜索文档、OSS、GitHub 等）
  - 项目内部搜索：使用 `Ace-tool_search_context` MCP 服务（语义搜索代码库）
- **Markdown table alignment** is enforced: header, separator, and data rows must have identical column counts. Always validate with `check_artifacts.py --strict` after generating/editing Markdown reports.
- **Reports directory (`reports/`)** contains plaintext secrets (by design, for developer reproducibility). It is gitignored and must never be committed.
- **`run_id` format**: `YYYY-MM-DD_HH-mm-ss_<short>`, optionally suffixed with `_iterNN` in retest loops.
- **Screenshot naming**: `s<4-digit-step>_<slug>.png` (e.g., `s0007_after-submit.png`).
- **Agent 调用约定（oh-my-opencode-slim）**：
  - **Task API**: `Task(subagent_type="<agent>", load_skills=[...], description="...", prompt="...", run_in_background=true/false)`
  - **可用 Subagent**: `explorer`(代码搜索) | `librarian`(文档/外部知识) | `oracle`(架构/策略咨询) | `designer`(前端/UI/体验) | `fixer`(实现/修复/重构)
  - **Category 路由**: slim 模式下不作为主路径，优先显式 `subagent_type`
  - **Session 延续**: 每次 Task() 返回 session_id，用 `session_id="<id>"` 延续多轮对话
- **Working directory for arc:agent**: `.arc/agent/` (调度记录在此目录)。
- **Working directory for arc:deliberate**: `.arc/deliberate/<task-name>/` (inside the target project).
- **Working directory for arc:review**: `.arc/review/<project-name>/` (inside the target project). 严禁修改被评审项目的源代码，只在此目录下产出评审文件。
- **Working directory for arc:init**: `.arc/init/` (inside the target project). 工作文件在此目录下，最终 CLAUDE.md 输出到项目目录树中。只写 CLAUDE.md，不改源码。
- **Working directory for arc:implement**: `.arc/implement/<task-name>/` (inside the target project). 记录计划、执行日志、验证与交接，不做无关改动。
- **Working directory for arc:ip-audit**: `.arc/ip-audit/<project-name>/` (inside the target project). 只做审查与交接，不写申请正式文档。
- **Working directory for arc:ip-docs**: `.arc/ip-docs/<project-name>/` (inside the target project). 仅生成可编辑文档草稿，不修改业务源码。
- **OpenSpec integration**: arc:deliberate 的 Phase 3 使用 [OpenSpec](https://github.com/Fission-AI/OpenSpec) CLI（`openspec`）生成结构化计划。OpenSpec 在 `.arc/deliberate/<task-name>/` 内初始化，artifact 写入 `openspec/changes/<task-name>/` 下。工作流：`openspec init` → `openspec new change` → `openspec instructions` → `openspec validate` → `openspec archive`。

## 上下文优先级协议 (Context Priority Protocol)

**为了避免重复扫描代码库，所有 Skill 必须遵循统一的上下文获取优先级：**

### 优先级顺序

1. **优先级 1: `.arc/<skill>/` 缓存上下文**
   - 检查 Skill 特定的缓存文件（如 `project-snapshot.md`）
   - 验证时间戳和文件完整性
   - 如验证通过，直接使用缓存

2. **优先级 2: 项目 CLAUDE.md 层级索引**
   - 扫描：`find . -name "CLAUDE.md" -type f`
   - 读取根级 + 相关模块级 CLAUDE.md
   - 提取：架构、模块、技术栈、入口、接口

3. **优先级 3: 项目源码扫描 (ace-tool MCP)**
   - 触发：缓存和索引均不可用
   - 扫描：ace-tool 语义搜索代码结构
   - 生成：临时快照到 `.arc/<skill>/context/`
   - 建议：提示用户运行 `/arc:init` 生成持久索引

4. **优先级 4: 外部参考搜索 (Exa MCP)**
   - 触发：涉及不熟悉的库/框架
   - 搜索：官方文档、GitHub 示例、最佳实践
   - 缓存：搜索结果到 `.arc/<skill>/external/`

### 缓存验证与自我修复

**验证规则**：
- **使用前验证**：检查文件存在性、时间戳、必需字段
- **使用中验证**：关键信息查找失败（如选择器、API路径）时标记为"可能过期"
- **使用后验证**：任务失败且根因指向缓存错误时触发文档更新

**自我修复流程**：
1. 检测到缓存错误 → 判断错误类型
2. 结构性错误 → 触发 `arc:init` 重新生成
3. 局部错误 → 标记错误位置 → 回退到源码扫描 → 生成补丁到 `.arc/<skill>/patches/`
4. 提示用户：建议运行 `arc:init` 更新

**错误报告位置**：
- arc:simulate: `<run_dir>/context-errors/cache-error-YYYYMMDD-HHMMSS.md`
- arc:triage: `<run_dir>/analysis/context-errors/cache-error-YYYYMMDD-HHMMSS.md`
- 其他 Skill: `.arc/<skill>/context-errors/cache-error-YYYYMMDD-HHMMSS.md`

### 时间戳与过期策略

| 上下文类型 | 新鲜期 | 过期后行为 |
|-----------|--------|-----------|
| `.arc/<skill>/context/` 缓存 | 24h | 重新生成 |
| 项目 CLAUDE.md | 7天 | 提示用户运行 arc:init |
| `.arc/review/` 快照 | 24h | 对比哈希决定是否重新生成 |
| `.arc/init/` 快照 | 7天 | 增量更新 |
| 外部参考缓存 | 30天 | 重新搜索 |

**详细协议文档**：[.arc/context-priority-protocol.md](./.arc/context-priority-protocol.md)

## Agent 使用规范 (Agent Usage Guidelines)

**基于 oh-my-opencode-slim 的 Agent 角色定义与使用边界**

### 核心 Agent 矩阵

> **orchestrator + 5 subagent**：主进程作为 orchestrator 负责拆解和路由，执行侧统一使用 slim 的五个专用 Agent。

| Agent | 角色定位 | 调用方式 | 适用场景 |
|------|---------|---------|---------|
| **orchestrator** | 主控调度员 | 主进程（不通过 Task 调用） | 需求拆解、任务规划、结果整合 |
| **explorer** | 代码库侦察兵 | `Task(subagent_type="explorer", ...)` | 代码搜索、符号定位、依赖拓扑摸排 |
| **librarian** | 外部知识检索 | `Task(subagent_type="librarian", ...)` | 官方文档、社区最佳实践、版本差异检索 |
| **oracle** | 架构与策略顾问 | `Task(subagent_type="oracle", ...)` | 架构决策、复杂问题分析、方案权衡 |
| **designer** | 前端/UI/体验专家 | `Task(subagent_type="designer", ...)` | 页面、组件、交互、视觉与可用性优化 |
| **fixer** | 实施与修复执行者 | `Task(subagent_type="fixer", ...)` | 编码实现、缺陷修复、重构与批量修改 |

### 关键使用边界（CRITICAL）

**❌ 错误使用模式（Anti-Patterns）**

| 错误场景 | 错误调用 | 问题 | 正确调用 |
|---------|---------|------|---------|
| 使用旧版 subagent | `Task(subagent_type="prometheus", ...)` | slim 不支持该 agent | `Task(subagent_type="oracle", ...)` 或 `Task(subagent_type="fixer", ...)` |
| 代码搜索走旧名 | `Task(subagent_type="explore", ...)` | slim 标准名为 `explorer` | `Task(subagent_type="explorer", ...)` |
| 前端任务走 category | `Task(category="visual-engineering", ...)` | slim 推荐显式 agent | `Task(subagent_type="designer", ...)` |
| 通用开发走 category | `Task(category="deep", ...)` | slim 推荐显式 agent | `Task(subagent_type="fixer", ...)` |

**✅ 正确使用模式（Best Practices）**

| 场景 | 应该用哪个 Agent | 调用方式 | 说明 |
|------|----------------|---------|------|
| 代码库侦察 | explorer | `Task(subagent_type="explorer", run_in_background=true, ...)` | 低成本并发搜索 |
| 外部文档检索 | librarian | `Task(subagent_type="librarian", run_in_background=true, ...)` | 官方文档优先 |
| 架构推演 | oracle | `Task(subagent_type="oracle", run_in_background=true, ...)` | 高质量策略分析 |
| 前端实现 | designer | `Task(subagent_type="designer", run_in_background=true, ...)` | 负责 UI/UX 与交互 |
| 后端与通用实现 | fixer | `Task(subagent_type="fixer", run_in_background=true, ...)` | 负责代码落地和修复 |
| 知识产权审查 | oracle + fixer + librarian | 三路并发 `Task(subagent_type=...)` | 技术、实现、合规多视角协作 |

### 典型工作流拓扑

**1. 标准协作拓扑（常规任务）**
```
orchestrator (主控)
  ├─▶ Task(subagent_type="librarian")  - 收集外部文档
  ├─▶ Task(subagent_type="explorer")   - 扫描本地代码
  └─▶ Task(subagent_type="fixer")      - 实施改动并验证
```

**2. 前后端并行拓扑（全栈任务）**
```
orchestrator (主控)
  ├─▶ Task(subagent_type="fixer")      - 后端/通用实现
  ├─▶ Task(subagent_type="designer")   - 前端/UI 实现
  └─▶ Task(subagent_type="oracle")     - 冲突裁决与方案把关
```

**3. 连续失败破局拓扑（失败 ≥ 2 次）**
```
orchestrator (主控)
  └─▶ Task(subagent_type="fixer") 失败重试
      └─▶ Task(subagent_type="oracle") 根因分析与策略修正
```

### 记忆要点

1. **统一使用 slim 五个 subagent**：`explorer/librarian/oracle/designer/fixer`
2. **优先显式 subagent_type**，不再依赖旧 category 路由
3. **探索与文档检索优先并发后台执行**
4. **架构决策由 oracle 兜底**
