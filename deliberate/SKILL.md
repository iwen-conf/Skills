---
name: "arc:deliberate"
description: 当复杂问题需要多 Agent 多视角分析并通过迭代讨论达成共识时使用
---

# Multi-Agent Deliberation

## Overview

通过共享文件系统作为通信总线，协调多个专业 Agent（oracle/deep/momus）进行迭代式协作审议。**每个阶段各 Agent 都必须互相反驳、审阅对方的观点**。

流程分为四个阶段：

1. **歧义检查阶段**：多 Agent 分析需求 → 识别歧义 → 互相反驳 → 用户澄清 → 直到无歧义
2. **审议阶段**：多 Agent 独立提案 → 交叉审阅 → 互相反驳 → 迭代收敛 → 合成共识报告
3. **计划生成阶段**：OpenSpec 生成结构化计划 → 多 Agent 审查反驳 → 定稿可执行计划
4. **执行阶段**：使用 Task(category="deep") 执行代码实现

## Agent 调用方式

**CRITICAL**: 所有任务通过 oh-my-opencode 的 Task() API 调度：

| Agent 角色 | 调用方式 | 用途 |
|------|---------|------|
| **oracle** | `Task(subagent_type="oracle", load_skills=["arc:deliberate"], ...)` | 架构分析、设计评审（只读高质量推理） |
| **deep** | `Task(category="deep", load_skills=[...], ...)` | 深度工程分析、方案提案、代码执行 |
| **momus** | `Task(subagent_type="momus", load_skills=["arc:deliberate"], ...)` | 质量审查、完整性验证、计划评估 |
| **metis** | `Task(subagent_type="metis", load_skills=["arc:refine"], ...)` | 需求预分析、歧义检测 |
| **explore** | `Task(subagent_type="explore", load_skills=[], run_in_background=true, ...)` | 代码库搜索（廉价、后台） |
| **librarian** | `Task(subagent_type="librarian", load_skills=[], run_in_background=true, ...)` | 外部文档搜索（廉价、后台） |

通用 Task 调用模板：
```
Task(
  category: "<category>",              // 或 subagent_type: "<agent>"
  load_skills: ["arc:deliberate", ...], // 装备相关 skill
  description: "<简短描述>",
  prompt: "<具体任务指令，包含读写文件路径>",
  run_in_background: true               // 并发执行
)
```

## MCP 工具使用

### 搜索外部信息（互联网）
使用 **Exa MCP** 进行网络搜索：
- 搜索最新技术文档、最佳实践
- 搜索开源项目、库的使用案例
- 搜索行业标准和安全规范

### 搜索项目信息
使用 **ace-tool MCP** 进行语义搜索：
- 搜索项目代码结构
- 搜索现有实现模式
- 搜索项目规范（CLAUDE.md）

## When to Use

- 复杂技术决策需要多视角验证（架构、性能、安全、兼容性）
- 单个模型的方案可能有盲点，需要交叉审阅
- 用户需要可解释的决策而非黑盒结论
- 问题涉及多个技术领域，需要 oracle（架构）+ deep（工程）+ momus（质量）协作

## Core Pattern

### 输入参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `task_name` | string | 是 | 任务名称，用于目录命名 |
| `workdir` | string | 是 | 工作目录绝对路径 |
| `enhanced_prompt_path` | string | 否 | 增强 prompt 路径，默认读取 `.arc/deliberate/<task-name>/context/enhanced-prompt.md` |
| `max_rounds` | number | 否 | 最大审议迭代轮次，默认 3 |
| `max_ambiguity_rounds` | number | 否 | 最大歧义检查轮次，默认 3 |

### 目录结构

**按 Agent 角色分目录**，每个 Agent 的所有阶段产出集中在自己的目录下：

```
<workdir>/.arc/deliberate/<task-name>/
├── context/
│   └── enhanced-prompt.md              # arc:refine 产出
├── agents/
│   ├── oracle/                          # oracle Agent 所有阶段产出（架构视角）
│   │   ├── ambiguity-round-1.md            # 歧义分析（Phase 1）
│   │   ├── proposal-round-1.md             # 提案（Phase 2）
│   │   ├── critique-round-1.md             # 审阅反驳（Phase 2）
│   │   └── plan-review.md                  # 计划审查（Phase 3）
│   ├── deep/                            # deep Agent 所有阶段产出（工程视角）
│   │   ├── ambiguity-round-1.md
│   │   ├── proposal-round-1.md
│   │   ├── critique-round-1.md
│   │   └── plan-review.md
│   └── momus/                           # momus Agent 所有阶段产出（质量视角）
│       ├── ambiguity-round-1.md
│       ├── proposal-round-1.md
│       ├── critique-round-1.md
│       └── plan-review.md
├── convergence/
│   └── round-N-summary.md              # 收敛判定摘要
└── openspec/                            # OpenSpec 工作空间（Phase 3）
    ├── changes/
    │   ├── <task-name>/                 # openspec new change 创建
    │   │   ├── .openspec.yaml
    │   │   ├── proposal.md              # 方案提案
    │   │   ├── specs/                   # 详细规范
    │   │   │   └── <capability>/spec.md
    │   │   ├── design.md                # 架构设计
    │   │   └── tasks.md                 # 有序可执行任务
    │   └── archive/
    └── specs/
```

---

## Phase 1: 歧义检查（Ambiguity Check）

### Step 1.1: 多 Agent 分析 + 搜索

**CRITICAL**: 必须先使用 MCP 工具搜索信息，再进行分析。

1. **搜索项目信息**：使用 `ace-tool` MCP 的 `search_context` 搜索项目代码
2. **搜索外部信息**：使用 `Exa MCP` 的 `web_search_exa` 或 `company_research_exa` 搜索互联网
3. **分析**：并发调用多个 Agent，分析增强后的 prompt，识别潜在歧义

**多 Agent 并发调用**（在同一消息中发起，`run_in_background: true`）：

**oracle 分析**（架构视角）:
```
Task(
  subagent_type: "oracle",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "oracle 歧义分析",
  prompt: "你是架构师角色。分析以下需求的歧义点。
上下文信息：<MCP 搜索结果>
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
列出所有可能存在歧义的地方，包括：边界条件未定义、约束不明确、术语理解可能不同、假设未说明等。
将分析结果写入 <workdir>/.arc/deliberate/<task-name>/agents/oracle/ambiguity-round-N.md。"
)
```

**deep 分析**（工程视角）:
```
Task(
  category: "deep",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "deep 歧义分析",
  prompt: "你是后端架构师。分析以下需求的歧义点。
上下文信息（来自 MCP 搜索）：<MCP 搜索结果>
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
从后端架构、技术约束、性能要求等角度，列出可能存在歧义的地方。
写入 <workdir>/.arc/deliberate/<task-name>/agents/deep/ambiguity-round-N.md。"
)
```

**momus 分析**（质量视角）:
```
Task(
  subagent_type: "momus",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "momus 歧义分析",
  prompt: "你是质量与用户体验分析师。分析以下需求的歧义点。
上下文信息（来自 MCP 搜索）：<MCP 搜索结果>
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
从用户体验、完整性、可维护性等角度，列出可能存在歧义的地方。
写入 <workdir>/.arc/deliberate/<task-name>/agents/momus/ambiguity-round-N.md。"
)
```

### Step 1.2: 互相反驳歧义分析

**CRITICAL**: 各 Agent 必须**互相反驳对方识别出的歧义**。

每个模型必须：
1. 读取其他 Agent 的歧义分析（如 oracle 读取 `agents/deep/ambiguity-round-N.md` 和 `agents/momus/ambiguity-round-N.md`）
2. 反驳对方认为的"歧义"（可能不是歧义）
3. 补充对方遗漏的歧义
4. 将反驳内容追加到自己的 `ambiguity-round-N.md`

调用方式同 Step 1.1（oracle 用 `Task(subagent_type="oracle")`，deep 用 `Task(category="deep")`，momus 用 `Task(subagent_type="momus")`）。

### Step 1.3: 聚合歧义

读取各份分析报告，主进程直接处理（不需 subagent）聚合所有歧义点：

```markdown
# 歧义汇总 (Round N)

## 技术歧义
- <歧义1描述>
- <歧义2描述>

## 边界条件歧义
- <歧义描述>

## 假设歧义
- <歧义描述>

## 需要澄清的问题
1. <问题1>
2. <问题2>
```

### Step 1.4: 用户澄清

使用 `AskUserQuestion` 向用户呈现歧义清单，让用户逐个澄清：

```markdown
## 歧义澄清

在继续审议前，请澄清以下问题：

### 问题 1
<歧义描述>
- [选项A]
- [选项B]
- [用户补充]

### 问题 2
...
```

### Step 1.5: 收敛判定

- **有歧义未澄清**：回到 Step 1.1，进入下一轮歧义检查
- **无歧义或已全部澄清**：进入 Phase 2（审议阶段）

---

## Phase 2: 审议阶段（Deliberation）

### Step 2.1: 目录脚手架

确保 `claude/`、`codex/`、`gemini/`、`convergence/` 目录已创建。

### Step 2.2: 并发派发提案 (每轮)

**CRITICAL**: 各 Agent 必须在同一消息中并发发起（`run_in_background: true`）。

**oracle 提案**（架构视角）:
```
Task(
  subagent_type: "oracle",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "oracle 提案 Round N",
  prompt: "你是架构师角色（全局视角、架构设计、技术选型）。
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
给出完整的解决方案提案，仅限纯文本 Markdown 格式，禁止代码块。
将提案写入 <workdir>/.arc/deliberate/<task-name>/agents/oracle/proposal-round-N.md。"
)
```

**deep 提案**（工程视角）:
```
Task(
  category: "deep",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "deep 提案 Round N",
  prompt: "你是后端架构师（后端架构、性能优化、数据库、安全）。
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
给出完整的解决方案提案，仅限纯文本 Markdown 格式，禁止代码块。
写入 <workdir>/.arc/deliberate/<task-name>/agents/deep/proposal-round-N.md。"
)
```

**momus 提案**（质量视角）:
```
Task(
  subagent_type: "momus",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "momus 提案 Round N",
  prompt: "你是质量与用户体验分析师（UI/UX、用户体验、响应式设计、可维护性）。
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
给出完整的解决方案提案，仅限纯文本 Markdown 格式，禁止代码块。
写入 <workdir>/.arc/deliberate/<task-name>/agents/momus/proposal-round-N.md。"
)
```

### Step 2.3: 等待完成

等待各 Agent 后台任务完成（使用 `background_output(task_id="...")` 收集结果）。

### Step 2.4: 交叉审阅 + 互相反驳

**CRITICAL**: 各 Agent 必须**互相反驳对方的观点**，不能只是简单审阅。

每个 Agent 必须：
1. 读取其他两个 Agent 的提案
2. **找出对方观点的问题和漏洞**
3. **用论据反驳对方的技术选择**
4. 提出自己的替代方案

**oracle 审阅 deep + momus**（用 `Task(subagent_type="oracle", session_id="<复用上轮 session>", ...)`）:
- 读取 `agents/deep/proposal-round-N.md` 和 `agents/momus/proposal-round-N.md`
- 从架构视角反驳 deep 的工程选择、momus 的质量要求
- 产出：`agents/oracle/critique-round-N.md`

**deep 审阅 oracle + momus**（用 `Task(category="deep", session_id="<复用上轮 session>", ...)`）:
- 读取 `agents/oracle/proposal-round-N.md` 和 `agents/momus/proposal-round-N.md`
- 从工程视角反驳 oracle 的架构设计、momus 的体验要求
- 产出：`agents/deep/critique-round-N.md`

**momus 审阅 oracle + deep**（用 `Task(subagent_type="momus", session_id="<复用上轮 session>", ...)`）:
- 读取 `agents/oracle/proposal-round-N.md` 和 `agents/deep/proposal-round-N.md`
- 从质量视角反驳 oracle 的抽象设计、deep 的工程实现
- 产出：`agents/momus/critique-round-N.md`

### Step 2.5: 收敛判定

主进程直接读取各 Agent 的 critique：
- **无分歧**：收敛，合成共识报告，写入 `convergence/final-consensus.md`
- **有分歧**：生成 `convergence/round-N-summary.md`，进入下一轮

### Step 2.6: 共识报告

收敛或达到最大轮次后，合成 `convergence/final-consensus.md`：

```markdown
# 共识报告: <任务名>

## 问题背景
<原始问题描述>

## 解决方案概述
<一句话方案摘要>

## 方案详述
<各方观点的综合>

## 风险与缓解
<识别到的风险及应对>

## 未解决分歧（如有）
<达成的共识>
<未达成的问题及原因>

## 结论
<最终建议>
```

---

## Phase 3: 计划生成阶段（Plan Generation）

**CRITICAL**: 使用 [OpenSpec](https://github.com/Fission-AI/OpenSpec)（CLI: `openspec`）将共识报告转化为结构化可执行计划，再经多 Agent 审查反驳定稿。

OpenSpec 采用 spec-driven 工作流，按 `proposal → specs → design → tasks` 顺序生成 artifact，每个 artifact 都有依赖关系和结构化模板。

### Step 3.0: OpenSpec 初始化

在审议工作空间内初始化 OpenSpec 并创建变更：

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec init --tools none
openspec new change <task-name>
```

产出目录结构：
```
openspec/
├── changes/
│   ├── <task-name>/
│   │   └── .openspec.yaml   # schema: spec-driven
│   └── archive/
└── specs/
```

### Step 3.1: 按序生成 4 个 artifact

对每个 artifact（proposal → specs → design → tasks），执行以下流程：

1. **获取 OpenSpec 结构化指令**：
```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec instructions <artifact> --change <task-name>
```
`openspec instructions` 输出包含 `<instruction>`（写作指南）、`<template>`（结构模板）和 `<output>`（目标写入路径）。

2. **Claude subagent 执行写入**：将 OpenSpec 指令 + `convergence/final-consensus.md` 的内容一起发给 Claude subagent，由其按模板填充并写入指定路径。

**Claude 生成 proposal**（subagent，每个 artifact 单独调用）:
```
Task({
  description: "OpenSpec proposal 生成",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "基于共识报告生成 OpenSpec proposal。
读取以下文件：
- <workdir>/.arc/deliberate/<task-name>/convergence/final-consensus.md

按照以下 OpenSpec 指令填写 proposal：
<此处粘贴 openspec instructions proposal 的完整输出>

将结果写入 <output> 标签指定的路径。"
})
```

依次对 `specs`、`design`、`tasks` 重复上述流程。每个 artifact 必须在前置依赖完成后再生成（`openspec instructions` 会通过 `<warning>` 标签提示缺失依赖）。

**生成 `tasks` artifact 时的额外要求**：Claude subagent 必须为每个 task 标注 AI 执行预估耗时 `[~Xmin]`，参考以下基准：

| 复杂度 | 预估耗时 | 示例 |
|--------|---------|------|
| 简单（单文件修改、配置调整、CRUD） | ~1-2min | 修改一个配置项、添加一个字段 |
| 中等（新增模块/接口、跨 2-3 文件） | ~3-5min | 新增 API endpoint、添加中间件 |
| 高（架构变更、跨模块重构） | ~5-15min | 重构鉴权体系、迁移数据模型 |
| 超大（新系统、大量文件联动） | ~15-30min | 新增微服务、全面重构 |

**产出文件**（均位于 `openspec/changes/<task-name>/` 下）：
- `proposal.md` — 方案提案（Why / What Changes / Capabilities / Impact）
- `specs/<capability>/spec.md` — 详细规范（ADDED Requirements + Scenarios）
- `design.md` — 架构设计（Context / Goals / Decisions / Risks）
- `tasks.md` — 有序可执行任务（分组 checkbox 格式 + AI 耗时标注，可被 `openspec archive` 追踪）

### Step 3.2: OpenSpec 验证

生成完成后，校验 artifact 结构完整性和变更状态：

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec validate --change <task-name>
openspec status --change <task-name>
```

- `validate` 校验 artifact 结构是否符合 schema 要求
- `status` 显示每个 artifact 的完成状态（missing / present）

### Step 3.3: 多 Agent 并发审查计划

OpenSpec 生成计划后，**多 Agent 并发独立审查**（同一消息，`run_in_background: true`）。

> 以下路径简写 `$CHANGE` 代表 `<workdir>/.arc/deliberate/<task-name>/openspec/changes/<task-name>`。

**oracle 审查计划**（架构视角）:
```
Task(
  subagent_type: "oracle",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "oracle 审查计划",
  prompt: "你是架构师角色。审查以下 OpenSpec 计划文件，从全局架构、整体一致性、任务排序合理性角度进行审查反驳。
读取以下文件：
- $CHANGE/proposal.md
- $CHANGE/specs/ 下所有 spec.md
- $CHANGE/design.md
- $CHANGE/tasks.md
审查要求：
1. 指出计划中的逻辑问题和风险
2. 反驳不合理的任务排序或依赖关系
3. 检查计划与共识报告的一致性
4. 给出修改建议
将审查结果写入 <workdir>/.arc/deliberate/<task-name>/agents/oracle/plan-review.md。"
)
```

**deep 审查计划**（工程视角）:
```
Task(
  category: "deep",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "deep 审查计划",
  prompt: "你是后端架构师。审查以下 OpenSpec 计划文件，从后端架构、性能、安全、可行性角度进行审查反驳。
读取以下文件：
- $CHANGE/proposal.md
- $CHANGE/specs/ 下所有 spec.md
- $CHANGE/design.md
- $CHANGE/tasks.md
审查要求：
1. 指出计划中的技术问题和风险
2. 反驳不合理的任务排序或依赖关系
3. 补充遗漏的后端相关任务
4. 给出修改建议
写入 <workdir>/.arc/deliberate/<task-name>/agents/deep/plan-review.md。"
)
```

**momus 审查计划**（质量视角）:
```
Task(
  subagent_type: "momus",
  load_skills: ["arc:deliberate"],
  run_in_background: true,
  description: "momus 审查计划",
  prompt: "你是前端与交互设计师。审查以下 OpenSpec 计划文件，从前端交互、UI/UX、组件架构、用户体验角度进行审查反驳。
读取以下文件：
- $CHANGE/proposal.md
- $CHANGE/specs/ 下所有 spec.md
- $CHANGE/design.md
- $CHANGE/tasks.md
审查要求：
1. 指出计划中的前端/交互问题
2. 反驳不合理的设计选择
3. 补充遗漏的前端相关任务
4. 给出修改建议
写入 <workdir>/.arc/deliberate/<task-name>/agents/momus/plan-review.md。"
)
```

### Step 3.4: 多 Agent 交叉反驳计划审查

**CRITICAL**: 各 Agent 互相反驳对方的计划审查意见。每个 Agent 读取另外两个的 `plan-review.md`，反驳不合理之处，补充遗漏。

调用方式：oracle 用 `Task(subagent_type="oracle", session_id="<复用>")`，deep 用 `Task(category="deep", session_id="<复用>")`，momus 用 `Task(subagent_type="momus", session_id="<复用>")`，三者并发。

各 Agent 产出覆盖（更新）自己的 `plan-review.md`，追加反驳段落。

### Step 3.5: 定稿计划

主进程直接处理，综合各份审查报告，修订 OpenSpec artifact 文件：

1. 读取 `agents/oracle/plan-review.md`、`agents/deep/plan-review.md`、`agents/momus/plan-review.md`
2. 根据各方审查修订 `openspec/changes/<task-name>/tasks.md`（确保任务有序、有依赖关系、可执行）
3. 同步更新 `design.md` 和 `specs/` 下的 spec 文件（如有必要）

**`tasks.md` 格式要求**（OpenSpec 标准 checkbox 格式 + AI 耗时标注，可被 `openspec archive` 追踪进度）：

```markdown
## 1. <任务分组名>

- [ ] 1.1 <任务描述> [~2min]
- [ ] 1.2 <任务描述> [~5min]

## 2. <任务分组名>

- [ ] 2.1 <任务描述> [~3min]
- [ ] 2.2 <任务描述> [~10min]
```

> 时间标注基于 AI agent 执行速度（非人工），参考 Step 3.1 中的耗时基准表。

### Step 3.6: 最终验证与归档

```bash
cd <workdir>/.arc/deliberate/<task-name>
# 最终验证
openspec validate --change <task-name>
openspec status --change <task-name>
# 执行完成后归档（可选，在 Phase 4 完成后执行）
openspec archive <task-name>
```

---

## Phase 4: 执行阶段（Execution）

**CRITICAL**: 计划定稿后，使用 `Task(category="deep")` 执行代码实现。

### Step 4.1: Agent 执行计划

根据定稿计划，使用 deep Agent 按 `tasks.md` 逐步执行：

```
Task(
  category: "deep",
  load_skills: ["arc:deliberate"],
  description: "执行审议计划",
  prompt: "根据 .arc/deliberate/<task-name>/openspec/changes/<task-name>/tasks.md 中的任务列表，按顺序执行代码实现。
同时参考：
- .arc/deliberate/<task-name>/openspec/changes/<task-name>/design.md（架构设计）
- .arc/deliberate/<task-name>/openspec/changes/<task-name>/specs/（规范约束）
工作目录：<workdir>
仅输出实现结果，不要询问确认。
EOF
```

### Step 4.2: 验证

执行完成后，验证代码是否符合 `tasks.md` 中描述的产出要求。

### Step 4.3: 归档变更

所有任务完成后，归档 OpenSpec 变更：

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec archive <task-name>
```

---

## 超时与降级

| 情况 | 处理 |
|------|------|
| Agent 超时 > 10min | 使用 AskUserQuestion 询问用户是否继续等待或切换其他 Agent |
| 达到 max_ambiguity_rounds 仍有歧义 | 标记未解决歧义，进入审议阶段 |
| 达到 max_rounds 未收敛 | 强制合成共识报告，标注未解决分歧 |
| openspec 命令失败 | 降级为手动编写 openspec/changes/<task-name>/tasks.md |

## 状态反馈

```
[Multi-Agent Deliberation] 任务: <task-name>

=== 阶段 1: 歧义检查 ===
Round 1/3:
  ├── oracle(subagent) 分析... [完成]
  ├── deep(category) 分析... [完成]
  ├── momus(subagent) 分析... [完成]
  ├── 聚合歧义... [N 个歧义]
  └── 用户澄清... [进行中]

=== 阶段 2: 审议 ===
Round 1/3:
  ├── oracle 提案... [完成]
  ├── deep 提案... [完成]
  ├── momus 提案... [完成]
  ├── 交叉审阅... [完成]
  └── 收敛判定... [收敛]

=== 阶段 3: 计划生成（OpenSpec） ===
  ├── openspec init + new change... [完成]
  ├── 生成 artifact（proposal→specs→design→tasks）... [完成]
  ├── openspec validate... [通过]
  ├── oracle 审查... [完成]
  ├── deep 审查... [完成]
  ├── momus 审查... [完成]
  ├── 多Agent交叉反驳... [完成]
  └── 计划定稿... [完成]

=== 阶段 4: 执行 ===
  ├── deep Agent 执行... [进行中]
  └── 验证... [待定]
```

## Quick Reference

| 阶段 | 步骤 | 输出路径 |
|------|------|---------|
| 歧义检查 | 多Agent分析 → 聚合 → 用户澄清 → 判定 | `agents/(oracle\|deep\|momus)/ambiguity-round-N.md` |
| 审议 | 提案 → 审阅 → 收敛判定 → 共识报告 | `agents/(oracle\|deep\|momus)/proposal-round-N.md`, `convergence/final-consensus.md` |
| 计划生成 | OpenSpec init → 生成 artifact → 验证 → 多Agent审查 → 交叉反驳 → 定稿 | `openspec/changes/<task-name>/(proposal\|design\|tasks).md`, `openspec/changes/<task-name>/specs/` |
| 执行 | deep Agent 按 tasks.md 实现代码 → 归档 | 项目代码 + `openspec archive` |

## 调用方式速查

| 角色 | 调用方式 | 并发支持 |
|------|---------|---------|
| oracle | `Task(subagent_type="oracle", load_skills=["arc:deliberate"], run_in_background=true, ...)` | 后台异步 |
| deep | `Task(category="deep", load_skills=["arc:deliberate"], run_in_background=true, ...)` | 后台异步 |
| momus | `Task(subagent_type="momus", load_skills=["arc:deliberate"], run_in_background=true, ...)` | 后台异步 |
| 聚合/定稿 | 主进程直接处理 | — |
