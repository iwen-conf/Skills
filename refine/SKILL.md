---
name: "arc:clarify"
description: "需求模糊或约束不全时使用：补齐上下文并生成可直接执行的高质量提示。"
---

# Question Refiner

## Overview

系统性地补充用户问题的上下文信息。通过扫描项目的 CLAUDE.md 多级索引，结合交互式提问，将模糊需求转化为结构化、可执行的增强 prompt。

## Quick Contract

- **Trigger**：用户需求表达模糊、上下文不足、验收标准缺失。
- **Inputs**：原始问题、项目索引上下文、澄清问答结果。
- **Outputs**：结构化增强 prompt（Context/Task/Constraints/Success Criteria）。
- **Quality Gate**：下游执行前必须通过 `## Quality Gates` 的可执行性检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:clarify`，先补齐上下文与约束，再生成可执行 prompt。”

## The Iron Law

```
NO EXECUTION PROMPT WITHOUT CONTEXT, CONSTRAINTS, AND SUCCESS CRITERIA
```

未形成上下文、约束与验收标准前，不得进入执行阶段。

## Workflow

1. 优先读取共享索引与 CLAUDE/codemap 上下文。
2. 对比用户原始问题做缺口分析。
3. 通过关键问题澄清不确定项。
4. 生成结构化增强 prompt 并写入共享目录。

## Quality Gates

- 增强 prompt 必须包含 Context/Task/Constraints/Success Criteria。
- 关键术语必须统一并可被下游复用。
- 澄清问题应最少且高价值，避免噪声追问。
- 明确标注上下文来源与新鲜度。

## Red Flags

- 未做上下文扫描就直接改写需求。
- 只复述用户原话，没有补充可执行信息。
- 缺少成功判定标准却交付给实现阶段。
- 关键约束冲突未显式提示。

## When to Use

- **首选触发**：需求模糊或上下文不足，需要转成可执行任务描述。
- **典型场景**：模块边界不清、约束缺失、验收标准未定义。
- **边界提示**：多方案争议需论证时用 `arc:decide`，需求已清晰则直接下游执行。

## Core Pattern

### Step 1: 项目索引扫描

先读取共享上下文索引，再按优先级降级：

1. **优先读取** `.arc/context-hub/index.json`，查找可复用产物：
   - `CLAUDE.md` 层级索引
   - `codemap.md`（arc:cartography）
   - 最近的 review/score/implement handoff
2. **若索引缺失或失效**，再扫描项目根目录及子目录收集 CLAUDE.md：

```bash
# 扫描当前项目的所有 CLAUDE.md
find . -name "CLAUDE.md" -type f
```

3. **若仍不足**，最后回退到 ace-tool 源码语义搜索。

### Step 2: 上下文提取

优先从共享索引产物提取，再补充 CLAUDE.md 层级信息：

- **根级 CLAUDE.md**: 项目愿景、技术栈总览、模块关系
- **模块级 CLAUDE.md**: 特定模块的架构、编码规范、依赖关系
- **codemap.md**: 目录职责、模块边界、跨目录调用关系
- **review/score/handoff**: 已识别风险点、质量瓶颈、改动上下文

使用 Grep/Read 读取相关内容，构建项目上下文画像。

### Step 3: 差距分析

对比用户原始问题与项目上下文，识别缺失维度：

| 缺失维度 | 示例 |
|---------|------|
| 技术边界 | 前端/后端/全栈？具体模块？ |
| 约束条件 | 性能要求？兼容性要求？时间限制？ |
| 依赖影响 | 改动是否涉及其他模块？是否需要迁移？ |
| 验收标准 | 怎么算完成？如何验证成功？ |

### Step 4: 交互式细化

使用 `AskUserQuestion` 向用户提出 1-4 个关键问题。每个问题应：

- 基于差距分析的发现
- 提供 2-4 个选项，并允许用户自定义补充
- 问题表述清晰，避免专业术语

### Step 5: Prompt 合成

组装结构化增强 prompt，包含四个部分：

```markdown
## Context
<项目上下文摘要，包括相关模块、技术栈、规范>

## Task
<用户的原始任务描述>

## Constraints
<识别到的约束条件>

## Success Criteria
<如何验证任务完成>
```

### Step 6: 输出

将增强后的完整 prompt 写入共享目录：

```
<workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md
```

## Quick Reference

| 阶段 | 工具 | 输出 |
|------|------|------|
| 扫描 | Read + Glob/Find + ace-tool | 共享索引/CLAUDE.md/源码上下文 |
| 提取 | Read | 项目上下文摘要 |
| 分析 | 人工 | 差距清单 |
| 提问 | AskUserQuestion | 用户回答 |
| 合成 | Write | enhanced-prompt.md |

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:clarify execution:**

### Scanning Anti-Patterns

- **Skip-Scan-Ask**: Jumping straight to questions without reading CLAUDE.md hierarchy first — questions lack context
- **Cache Ignorance**: Not checking `.arc/context-hub/index.json` when valid cache exists — wastes prior work
- **Stale Context Usage**: Using expired cache (24h+) without verification — causes incorrect assumptions

### Question Anti-Patterns

- **Question Overload**: Asking 5+ questions at once — overwhelming, prioritize to 1-3 key questions
- **Technical Jargon**: Using developer terms with non-technical users — use business language
- **Leading Questions**: Phrasing questions to bias toward a specific answer — neutral phrasing required
- **No Custom Option**: Forcing users to pick from predefined options only — always allow free-form input

### Analysis Anti-Patterns

- **Literal Interpretation**: Taking user's words at face value without identifying implicit assumptions
- **Scope Skipping**: Not clarifying module boundaries when project has multiple sub-systems
- **Constraint Blindness**: Not identifying performance, compatibility, or timeline constraints

### Output Anti-Patterns

- **Missing Success Criteria**: Enhanced prompt without verification checklist — how to know it's done?
- **Orphaned Output**: Not writing enhanced-prompt.md to `.arc/deliberate/` — breaks arc:decide consumption
## Integration

此 Skill 完成后，建议继续调用 `arc:decide` Skill 进行多Agent审议。

如果在细化过程中发现共享产物失效：
- CLAUDE 索引失效 → 触发 `arc:init:update`（必要时 `arc:init:full`）
- codemap 失效 → 触发 `arc:cartography` 更新
- score/review 数据失效 → 触发 `score` 模块刷新（由 `arc:release` 编排触发）/ `arc:audit` 更新

```
问题细化完成。增强 prompt 已写入：
.arc/deliberate/<task-name>/context/enhanced-prompt.md

可继续调用 `arc decide` 进行多Agent审议。
```
