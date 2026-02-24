---
name: "arc:deliberate"
description: 当复杂问题需要 Claude、Codex、Gemini 三模型多视角分析并通过迭代讨论达成共识时使用
---

# Tri-Model Deliberation

## Overview

通过共享文件系统作为通信总线，协调 Claude、Codex、Gemini 三个模型进行迭代式协作审议。**每个阶段三模型都必须互相反驳、审阅对方的观点**。

流程分为四个阶段：

1. **歧义检查阶段**：三模型分析需求 → 识别歧义 → 互相反驳 → 用户澄清 → 直到无歧义
2. **审议阶段**：三模型独立提案 → 交叉审阅 → 互相反驳 → 迭代收敛 → 合成共识报告
3. **计划生成阶段**：OpenSpec 生成结构化计划 → 三模型审查反驳 → 定稿可执行计划
4. **执行阶段**：使用 Codex 执行代码实现

## 模型调用方式

**CRITICAL**: 三个模型的调用方式不同：

| 模型 | 调用方式 | 原因 |
|------|---------|------|
| **Claude** | **Task 工具（subagent）** | Claude Code 不能嵌套调用自身，`codeagent-wrapper --backend claude` 会报错 |
| **Codex** | `codeagent-wrapper --backend codex` | 独立进程，CLI 调用 |
| **Gemini** | `codeagent-wrapper --backend gemini` | 独立进程，CLI 调用 |

Claude subagent 调用模板：
```
Task({
  description: "<简短描述>",
  subagent_type: "general-purpose",
  prompt: "<具体任务指令，包含读写文件路径>",
  run_in_background: true,
  mode: "bypassPermissions"
})
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
- 问题涉及多个技术栈，需要后端(Codex) + 前端(Gemini) + 中央(Claude) 协作

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

**按模型分目录**，每个模型的所有阶段产出集中在自己的目录下：

```
<workdir>/.arc/deliberate/<task-name>/
├── context/
│   └── enhanced-prompt.md              # arc:refine 产出
├── claude/                              # Claude 所有阶段产出
│   ├── ambiguity-round-1.md            # 歧义分析（Phase 1）
│   ├── ambiguity-round-N.md
│   ├── proposal-round-1.md             # 提案（Phase 2）
│   ├── proposal-round-N.md
│   ├── critique-round-1.md             # 审阅反驳（Phase 2）
│   ├── critique-round-N.md
│   └── plan-review.md                  # 计划审查（Phase 3）
├── codex/                               # Codex 所有阶段产出
│   ├── ambiguity-round-1.md
│   ├── proposal-round-1.md
│   ├── critique-round-1.md
│   └── plan-review.md
├── gemini/                              # Gemini 所有阶段产出
│   ├── ambiguity-round-1.md
│   ├── proposal-round-1.md
│   ├── critique-round-1.md
│   └── plan-review.md
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

### Step 1.1: 三模型分析 + 搜索

**CRITICAL**: 必须先使用 MCP 工具搜索信息，再进行分析。

1. **搜索项目信息**：使用 `ace-tool` MCP 的 `search_context` 搜索项目代码
2. **搜索外部信息**：使用 `Exa MCP` 的 `web_search_exa` 或 `company_research_exa` 搜索互联网
3. **分析**：并发调用三个模型，分析增强后的 prompt，识别潜在歧义

**三模型并发调用**（在同一消息中发起，`run_in_background: true`）：

**Claude 分析**（subagent）:
```
Task({
  description: "Claude 歧义分析",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "你是架构师角色。分析以下需求的歧义点。
上下文信息：<MCP 搜索结果>
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
列出所有可能存在歧义的地方，包括：边界条件未定义、约束不明确、术语理解可能不同、假设未说明等。
将分析结果写入 <workdir>/.arc/deliberate/<task-name>/claude/ambiguity-round-N.md。"
})
```

**Codex 分析**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend codex - "$(pwd)" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/codex/architect.md
<TASK>
分析以下需求的歧义点。
上下文信息（来自 MCP 搜索）：<MCP 搜索结果>

读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
从后端架构、技术约束、性能要求等角度，列出可能存在歧义的地方。
写入 <workdir>/.arc/deliberate/<task-name>/codex/ambiguity-round-N.md。
</TASK>
OUTPUT: 歧义分析报告
EOF
```

**Gemini 分析**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend gemini - "$(pwd)" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/gemini/architect.md
<TASK>
分析以下需求的歧义点。
上下文信息（来自 MCP 搜索）：<MCP 搜索结果>

读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
从前端交互、用户体验、响应式设计等角度，列出可能存在歧义的地方。
写入 <workdir>/.arc/deliberate/<task-name>/gemini/ambiguity-round-N.md。
</TASK>
OUTPUT: 歧义分析报告
EOF
```

### Step 1.2: 互相反驳歧义分析

**CRITICAL**: 三个模型必须**互相反驳对方识别出的歧义**。

每个模型必须：
1. 读取其他两个模型的歧义分析（如 Claude 读取 `codex/ambiguity-round-N.md` 和 `gemini/ambiguity-round-N.md`）
2. 反驳对方认为的"歧义"（可能不是歧义）
3. 补充对方遗漏的歧义
4. 将反驳内容追加到自己的 `ambiguity-round-N.md`

调用方式同 Step 1.1（Claude 用 subagent，Codex/Gemini 用 codeagent-wrapper）。

### Step 1.3: 聚合歧义

读取三份分析报告，中央大脑 Claude（主进程直接处理，不需 subagent）聚合所有歧义点：

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

**CRITICAL**: 三个模型必须在同一消息中并发发起（`run_in_background: true`）。

**Claude 提案**（subagent）:
```
Task({
  description: "Claude 提案 Round N",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "你是架构师角色（中央大脑、全局优化、用户体验）。
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
给出完整的解决方案提案，仅限纯文本 Markdown 格式，禁止代码块。
将提案写入 <workdir>/.arc/deliberate/<task-name>/claude/proposal-round-N.md。"
})
```

**Codex 提案**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend codex - "$(pwd)" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/codex/architect.md
<TASK>
基于 Codex 视角（后端架构、性能优化、数据库、安全），给出完整的解决方案提案。
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
写入 <workdir>/.arc/deliberate/<task-name>/codex/proposal-round-N.md。
仅限纯文本 Markdown 格式，禁止代码块。
</TASK>
EOF
```

**Gemini 提案**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend gemini - "$(pwd)" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/gemini/architect.md
<TASK>
基于 Gemini 视角（前端交互、UI/UX、响应式设计、组件架构），给出完整的解决方案提案。
读取 <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md。
写入 <workdir>/.arc/deliberate/<task-name>/gemini/proposal-round-N.md。
仅限纯文本 Markdown 格式，禁止代码块。
</TASK>
EOF
```

### Step 2.3: 等待完成

等待三个后台任务完成（subagent 用 TaskOutput，codeagent-wrapper 用 Bash 等待）。

### Step 2.4: 交叉审阅 + 互相反驳

**CRITICAL**: 三个模型必须**互相反驳对方的观点**，不能只是简单审阅。

每个模型必须：
1. 读取其他两个模型的提案
2. **找出对方观点的问题和漏洞**
3. **用论据反驳对方的技术选择**
4. 提出自己的替代方案

**Claude 审阅 Codex + Gemini**（subagent）:
```
Task({
  description: "Claude 审阅反驳 Round N",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "读取以下两份提案：
- <workdir>/.arc/deliberate/<task-name>/codex/proposal-round-N.md
- <workdir>/.arc/deliberate/<task-name>/gemini/proposal-round-N.md
从全局视角反驳 Codex 的后端架构选择和 Gemini 的前端设计选择。找出问题和漏洞，用论据反驳。
将审阅结果写入 <workdir>/.arc/deliberate/<task-name>/claude/critique-round-N.md。"
})
```

**Codex 审阅 Claude + Gemini**（codeagent-wrapper）:
- 读取 `claude/proposal-round-N.md` 和 `gemini/proposal-round-N.md`
- 反驳 Claude 的全局视角选择、反驳 Gemini 的交互设计选择
- 产出：`codex/critique-round-N.md`

**Gemini 审阅 Claude + Codex**（codeagent-wrapper）:
- 读取 `claude/proposal-round-N.md` 和 `codex/proposal-round-N.md`
- 反驳 Claude 的抽象设计、反驳 Codex 的后端实现
- 产出：`gemini/critique-round-N.md`

### Step 2.5: 收敛判定

中央大脑 Claude（主进程直接处理）读取三份 critique：
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

**CRITICAL**: 使用 [OpenSpec](https://github.com/Fission-AI/OpenSpec)（CLI: `openspec`）将共识报告转化为结构化可执行计划，再经三模型审查反驳定稿。

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

**产出文件**（均位于 `openspec/changes/<task-name>/` 下）：
- `proposal.md` — 方案提案（Why / What Changes / Capabilities / Impact）
- `specs/<capability>/spec.md` — 详细规范（ADDED Requirements + Scenarios）
- `design.md` — 架构设计（Context / Goals / Decisions / Risks）
- `tasks.md` — 有序可执行任务（分组 checkbox 格式，可被 `openspec archive` 追踪）

### Step 3.2: OpenSpec 验证

生成完成后，校验 artifact 结构完整性和变更状态：

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec validate --change <task-name>
openspec status --change <task-name>
```

- `validate` 校验 artifact 结构是否符合 schema 要求
- `status` 显示每个 artifact 的完成状态（missing / present）

### Step 3.3: 三模型并发审查计划

OpenSpec 生成计划后，**三个模型并发独立审查**（同一消息，`run_in_background: true`）。

> 以下路径简写 `$CHANGE` 代表 `<workdir>/.arc/deliberate/<task-name>/openspec/changes/<task-name>`。

**Claude 审查计划**（subagent）:
```
Task({
  description: "Claude 审查计划",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "审查以下 OpenSpec 计划文件，从全局架构、整体一致性、任务排序合理性角度进行审查反驳。
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
将审查结果写入 <workdir>/.arc/deliberate/<task-name>/claude/plan-review.md。"
})
```

**Codex 审查计划**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend codex - "$(pwd)" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/codex/architect.md
<TASK>
审查以下 OpenSpec 计划文件，从后端架构、性能、安全、可行性角度进行审查反驳。

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

写入 <workdir>/.arc/deliberate/<task-name>/codex/plan-review.md。
</TASK>
OUTPUT: 计划审查报告
EOF
```

**Gemini 审查计划**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend gemini - "$(pwd)" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/gemini/architect.md
<TASK>
审查以下 OpenSpec 计划文件，从前端交互、UI/UX、组件架构、用户体验角度进行审查反驳。

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

写入 <workdir>/.arc/deliberate/<task-name>/gemini/plan-review.md。
</TASK>
OUTPUT: 计划审查报告
EOF
```

### Step 3.4: 三模型交叉反驳计划审查

**CRITICAL**: 三个模型互相反驳对方的计划审查意见。每个模型读取另外两个的 `plan-review.md`，反驳不合理之处，补充遗漏。

调用方式：Claude 用 subagent，Codex/Gemini 用 codeagent-wrapper，三者并发。

各模型产出覆盖（更新）自己的 `plan-review.md`，追加反驳段落。

### Step 3.5: 定稿计划

中央大脑 Claude（主进程直接处理）综合三份审查报告，修订 OpenSpec artifact 文件：

1. 读取 `claude/plan-review.md`、`codex/plan-review.md`、`gemini/plan-review.md`
2. 根据三方审查修订 `openspec/changes/<task-name>/tasks.md`（确保任务有序、有依赖关系、可执行）
3. 同步更新 `design.md` 和 `specs/` 下的 spec 文件（如有必要）

**`tasks.md` 格式要求**（OpenSpec 标准 checkbox 格式，可被 `openspec archive` 追踪进度）：

```markdown
## 1. <任务分组名>

- [ ] 1.1 <任务描述>
- [ ] 1.2 <任务描述>

## 2. <任务分组名>

- [ ] 2.1 <任务描述>
- [ ] 2.2 <任务描述>
```

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

**CRITICAL**: 计划定稿后，**执行代码必须使用 Codex**。

### Step 4.1: Codex 执行计划

根据定稿计划，使用 codeagent-wrapper 调用 Codex 按 `tasks.md` 逐步执行：

```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --backend codex - "$(pwd)" <<'EOF'
根据 .arc/deliberate/<task-name>/openspec/changes/<task-name>/tasks.md 中的任务列表，按顺序执行代码实现。
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
| 单模型超时 > 10min | 使用 AskUserQuestion 询问用户是否继续用剩余模型 |
| 达到 max_ambiguity_rounds 仍有歧义 | 标记未解决歧义，进入审议阶段 |
| 达到 max_rounds 未收敛 | 强制合成共识报告，标注未解决分歧 |
| openspec 命令失败 | 降级为手动编写 openspec/changes/<task-name>/tasks.md |

## 状态反馈

```
[Tri-Model Deliberation] 任务: <task-name>

=== 阶段 1: 歧义检查 ===
Round 1/3:
  ├── Claude(subagent) 分析... [完成]
  ├── Codex(CLI) 分析... [完成]
  ├── Gemini(CLI) 分析... [完成]
  ├── 聚合歧义... [N 个歧义]
  └── 用户澄清... [进行中]

=== 阶段 2: 审议 ===
Round 1/3:
  ├── Claude(subagent) 提案... [完成]
  ├── Codex(CLI) 提案... [完成]
  ├── Gemini(CLI) 提案... [完成]
  ├── 交叉审阅... [完成]
  └── 收敛判定... [收敛]

=== 阶段 3: 计划生成（OpenSpec） ===
  ├── openspec init + new change... [完成]
  ├── 生成 artifact（proposal→specs→design→tasks）... [完成]
  ├── openspec validate... [通过]
  ├── Claude(subagent) 审查... [完成]
  ├── Codex(CLI) 审查... [完成]
  ├── Gemini(CLI) 审查... [完成]
  ├── 三模型交叉反驳... [完成]
  └── 计划定稿... [完成]

=== 阶段 4: 执行 ===
  ├── Codex 执行... [进行中]
  └── 验证... [待定]
```

## Quick Reference

| 阶段 | 步骤 | 输出路径 |
|------|------|---------|
| 歧义检查 | 三模型分析 → 聚合 → 用户澄清 → 判定 | `(claude\|codex\|gemini)/ambiguity-round-N.md` |
| 审议 | 提案 → 审阅 → 收敛判定 → 共识报告 | `(claude\|codex\|gemini)/proposal-round-N.md`, `convergence/final-consensus.md` |
| 计划生成 | OpenSpec init → 生成 artifact → 验证 → 三模型审查 → 交叉反驳 → 定稿 | `openspec/changes/<task-name>/(proposal\|design\|tasks).md`, `openspec/changes/<task-name>/specs/` |
| 执行 | Codex 按 tasks.md 实现代码 → 归档 | 项目代码 + `openspec archive` |

## 调用方式速查

| 角色 | 调用方式 | 并发支持 |
|------|---------|---------|
| Claude | `Task({ subagent_type: "general-purpose", run_in_background: true })` | subagent 后台 |
| Codex | `Bash({ command: "codeagent-wrapper --lite --backend codex ...", run_in_background: true })` | Bash 后台 |
| Gemini | `Bash({ command: "codeagent-wrapper --lite --backend gemini ...", run_in_background: true })` | Bash 后台 |
| Claude（聚合/定稿） | 主进程直接处理 | — |
