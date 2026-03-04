---
name: "arc:refine"
description: 当用户提出模糊问题、缺少上下文或需求不清晰时使用，通过项目级探索进行澄清和补充
---

# Question Refiner

## Overview

系统性地补充用户问题的上下文信息。通过扫描项目的 CLAUDE.md 多级索引，结合交互式提问，将模糊需求转化为结构化、可执行的增强 prompt。

## When to Use

- 用户描述的问题缺少技术上下文（如"帮我优化性能"而不说哪个模块）
- 问题的解决方向涉及多个技术栈，但用户未明确优先级
- 项目有多个子模块，需要确定问题所属的上下文边界
- 用户的需求表述过于笼统，需要拆解为具体维度

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

**CRITICAL: The following behaviors are FORBIDDEN in arc:refine execution:**

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
- **Orphaned Output**: Not writing enhanced-prompt.md to `.arc/deliberate/` — breaks arc:deliberate consumption
## Integration

此 Skill 完成后，建议继续调用 `arc:deliberate` Skill 进行多Agent审议。

如果在细化过程中发现共享产物失效：
- CLAUDE 索引失效 → 触发 `arc:init:update`（必要时 `arc:init:full`）
- codemap 失效 → 触发 `arc:cartography` 更新
- score/review 数据失效 → 触发 `arc:score` / `arc:review` 更新

```
问题细化完成。增强 prompt 已写入：
.arc/deliberate/<task-name>/context/enhanced-prompt.md

可继续调用 /arc:deliberate 进行多Agent审议。
```
