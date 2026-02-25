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
  - **可用 Subagent**: `explore`(代码搜索) | `librarian`(文档搜索) | `oracle`(架构咨询) | `prometheus`(宏观规划) | `metis`(策略审计) | `momus`(代码审查)
  - **Session 延续**: 每次 Task() 返回 session_id，用 `session_id="<id>"` 延续多轮对话
- **Working directory for arc:agent**: `.arc/agent/` (调度记录在此目录)。
- **Working directory for arc:deliberate**: `.arc/deliberate/<task-name>/` (inside the target project).
- **Working directory for arc:review**: `.arc/review/<project-name>/` (inside the target project). 严禁修改被评审项目的源代码，只在此目录下产出评审文件。
- **Working directory for arc:init**: `.arc/init/` (inside the target project). 工作文件在此目录下，最终 CLAUDE.md 输出到项目目录树中。只写 CLAUDE.md，不改源码。
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

**基于 Oh-My-Opencode 框架的 Agent 角色定义与使用边界**

### 核心 Agent 矩阵

| Agent 代号 | 角色定位 | 适用场景 | 底层模型 | 关键技能 |
|-----------|---------|---------|---------|---------|
| **Sisyphus** | 主控调度员/项目经理 | 所有会话的默认入口，负责需求拆解、任务规划与路由 | anthropic/claude-opus-4-6 | ace-tool 项目检索、Exa 外网搜索 |
| **Prometheus** | 宏观规划师/破冰者 | 复杂需求拆解、依赖关系图谱生成、并行执行策略蓝图绘制 | anthropic/claude-opus-4-6 (max) | 交互式需求澄清、甘特图与依赖树构建 |
| **Metis** | 策略专家/算法优化师 | 执行前的策略审计，寻找 Prometheus 计划中的算法漏洞和逻辑盲区 | MiniMax/MiniMax-M2.5 | 计划突变、算法复杂度分析 |
| **Momus** | 严苛的 QA/代码审查员 | 执行极度严苛的代码审查、主动排查隐蔽的安全漏洞、验证代码风格与测试标准 | openai/gpt-5.2 | 诊断输出解析、故障注入模拟 |
| **Oracle** | 架构师/决策顾问 | 复杂系统架构推演、并发竞争条件根因分析、多系统交互权衡策略制定 | openai/gpt-5.1-codex-max (high) | 全代码库只读扫描、诊断日志深度解析 |
| **Librarian** | 知识管理与检索 | 查阅外部长篇官方文档、研读最新 API 迁移手册、检索开源社区最佳实践 | GLM/glm-5 | Exa 全网搜索、Context7 文档摄取、GitHub 仓库爬取 |
| **Explore** | 代码库侦察兵 | 极速踩点遍历深层文件树、执行基于上下文的模糊 grep 搜索、快速映射变量定义与系统依赖拓扑图 | anthropic/claude-haiku-4-5 | ast_grep_search、lsp_workspace_symbols |

### 关键使用边界（CRITICAL）

**❌ 错误使用模式（Anti-Patterns）**

| 错误场景 | 错误调用 | 问题 | 正确调用 |
|---------|---------|------|---------|
| 计划评审 | `Task(subagent_type="momus", prompt="审查这个执行计划...")` | Momus 是代码审查员，不审查计划 | `Task(subagent_type="metis", prompt="审计这个执行计划...")` |
| 需求澄清 | `Task(subagent_type="metis", prompt="澄清用户需求...")` | Metis 是策略审计员，不负责需求澄清 | `Task(subagent_type="prometheus", prompt="澄清用户需求...")` |
| 代码审查 | `Task(subagent_type="prometheus", prompt="审查这段代码...")` | Prometheus 是规划师，不审查代码 | `Task(subagent_type="momus", prompt="审查这段代码...")` |
| 架构决策 | `Task(subagent_type="momus", prompt="评估架构方案...")` | Momus 是 QA，不做架构决策 | `Task(subagent_type="oracle", prompt="评估架构方案...")` |

**✅ 正确使用模式（Best Practices）**

| 场景 | 应该用哪个 Agent | 调用方式 | 说明 |
|------|----------------|---------|------|
| 复杂需求拆解 | Prometheus | `Task(subagent_type="prometheus", load_skills=["arc:deliberate"], prompt="分析需求并生成执行计划...")` | Prometheus 会"采访"开发者以消除需求歧义 |
| 计划质量审计 | Metis | `Task(subagent_type="metis", load_skills=["arc:deliberate"], prompt="审计这个执行计划，寻找算法漏洞...")` | Metis 在执行前进行策略审计 |
| 代码质量审查 | Momus | `Task(subagent_type="momus", prompt="审查这段代码的安全性和代码风格...")` | Momus 执行严苛的代码审查 |
| 架构推演 | Oracle | `Task(subagent_type="oracle", prompt="分析这个并发竞争条件的根因...")` | Oracle 只读咨询，不亲自改代码 |
| 代码库侦察 | Explore | `Task(subagent_type="explore", run_in_background=true, prompt="找到所有认证相关的中间件...")` | Explore 极速前哨，总是后台运行 |
| 外部文档检索 | Librarian | `Task(subagent_type="librarian", run_in_background=true, prompt="查找 React 18 的最新 API 文档...")` | Librarian 打破知识截止日期限制 |

### 典型工作流拓扑

**1. 标准 Ultrawork 交互拓扑**（常规任务）
```
Sisyphus (主控)
  ├─▶ Librarian (后台) - 收集外部 API 文档
  ├─▶ Explore (后台) - 扫描本地代码库
  └─▶ Task(category="visual-engineering") - 执行前端重构
      └─▶ LSP 诊断验证
```

**2. 复杂特性编排管道**（大型系统级特性）
```
Sisyphus (主控)
  └─▶ Prometheus (宏观规划)
      ├─▶ 顾问面试模式 - 澄清需求
      ├─▶ 绘制依赖图谱
      └─▶ Metis (策略审计)
          └─▶ 审查算法漏洞
              └─▶ Momus (质量审查)
                  └─▶ 验证标准
                      └─▶ Atlas (重体力劳动) - 执行大规模重构
```

**3. Oracle 死锁破局机制**（连续失败 2 次后触发）
```
Sisyphus (主控)
  └─▶ Task(category="quick") - 尝试修复 Bug
      └─▶ LSP 诊断失败 (第 1 次)
          └─▶ 重试修复
              └─▶ LSP 诊断失败 (第 2 次)
                  └─▶ 强制挂起
                      └─▶ Oracle (架构顾问)
                          └─▶ 分析根因 + 生成破局指导书
                              └─▶ Sisyphus 执行神谕
```

### 记忆要点

1. **Prometheus = 计划生成**，不是代码审查
2. **Metis = 计划审计**，不是需求澄清
3. **Momus = 代码审查**，不是计划评审
4. **Oracle = 架构咨询**，只读不写
5. **Explore/Librarian = 后台侦察**，总是 `run_in_background=true`

**参考文档**：[多智能体编程框架Agent解析.md](./多智能体编程框架Agent解析.md)
