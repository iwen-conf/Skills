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

扫描项目根目录及子目录，收集所有 CLAUDE.md 文件路径：

```bash
# 扫描当前项目的所有 CLAUDE.md
find . -name "CLAUDE.md" -type f
```

### Step 2: 上下文提取

对于找到的每个 CLAUDE.md，按层级提取关键信息：

- **根级 CLAUDE.md**: 项目愿景、技术栈总览、模块关系
- **模块级 CLAUDE.md**: 特定模块的架构、编码规范、依赖关系

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
- 提供 2-4 个选项 + "Other" 让用户补充
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
<workdir>/.tri-model-deliberation/<task-name>/context/enhanced-prompt.md
```

## Quick Reference

| 阶段 | 工具 | 输出 |
|------|------|------|
| 扫描 | Glob/Find | CLAUDE.md 路径列表 |
| 提取 | Read | 项目上下文摘要 |
| 分析 | 人工 | 差距清单 |
| 提问 | AskUserQuestion | 用户回答 |
| 合成 | Write | enhanced-prompt.md |

## Common Mistakes

1. **跳过扫描直接提问** — 不先了解项目结构，提问会没针对性
2. **提问过多** — 一次问 5+ 个问题会增加用户负担，优先问最关键的 1-3 个
3. **问题过于技术** — 用户可能不懂术语，用业务语言描述
4. **不做差距分析** — 直接按字面理解用户问题，忽略隐含假设

## Integration

此 Skill 完成后，建议继续调用 `tri-model-deliberation` Skill 进行三模型审议。

```
问题细化完成。增强 prompt 已写入：
.tri-model-deliberation/<task-name>/context/enhanced-prompt.md

可继续调用 /tri-model-deliberation 进行三模型审议。
```
