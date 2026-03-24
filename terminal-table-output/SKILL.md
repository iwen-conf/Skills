---
name: terminal-table-output
description: 终端盒线表输出范式：当用户要求表格、比较表、矩阵或状态汇总，且内容足够紧凑时，用 Unicode 盒线表替代 Markdown 表。
---

# Terminal Table Output

## Overview

`terminal-table-output` 是一个 generic/fusion skill，负责把**聊天里的紧凑二维信息**渲染为终端风格盒线表，而不是 Markdown 表。

它只负责表现层，不负责领域判断。本 skill 不应改变磁盘工件的原生格式；Markdown、JSON、HTML、日志与代码文件仍保持各自原约定。

详细渲染规则见 [`references/render-rules.md`](references/render-rules.md)。

## Quick Contract

- **Trigger**: 用户明确要求表格/对比表/矩阵/汇总表，或当前输出天然是短字段重复的二维结构。
- **Inputs**: 行列结构明确的紧凑数据；可选列宽约束、截断策略和补充说明。
- **Outputs**: 一个 fenced `text` 代码块中的 Unicode 盒线表；若有溢出说明，可在表格后附短列表。
- **Quality Gate**: 只有在内容能保持紧凑、对齐稳定、边界闭合时才允许输出表格；否则必须降级为非表格结构。
- **Decision Tree**: 先判断“是否天然二维且短字段重复”；若否，直接不用表格。

## Announce

Begin by stating clearly:
"I am using `terminal-table-output` to render a compact box-drawing table for chat delivery."

## The Iron Law

```text
NO TABLE UNLESS THE CONTENT IS NATURALLY TWO-DIMENSIONAL, COMPACT, AND ALIGNABLE.
```

如果内容不是天然二维、不能保持紧凑，或对齐稳定性存疑，就不要输出表格。

## Workflow

1. 判断内容是否天然适合二维呈现，而不是为了“看起来像表格”强行入格。
2. 精简列数、缩短表头，并把长解释、路径、URL、代码、JSON 挪到表格外。
3. 逐列计算显示宽度：
   - 中文和其他全角字符按宽度 `2`
   - 英文、数字、半角符号按宽度 `1`
4. 在 fenced `text` 代码块中用 Unicode 盒线字符渲染完整外框与分隔线。
5. 逐行复核总宽度是否一致；若不一致，继续缩短或放弃表格。

## Quality Gates

- 仅当信息天然是紧凑二维结构时才使用表格。
- 只能在 fenced code block with `text` 中输出，不使用 Markdown table syntax，除非用户明确要求。
- 每一行都必须左右闭合，且总宽度一致。
- 单元格右侧只能用空格补齐，不能使用 tab。
- 若内容过宽，优先减列、缩短表头、手动换行或显式截断为 `...`。
- 文件路径、URL、代码、JSON、长段解释、错误细节不应被强行塞进表格。
- 当稳定对齐无法保证时，必须降级为列表、分组块或正文。

## Red Flags

- 为了“形式统一”把长解释硬塞进表格。
- 混用 Markdown 表和盒线表。
- 行宽不一致、边框未闭合、全角宽度未处理。
- 表格过宽，超出默认聊天区域仍继续堆列。
- 本该用列表或正文，却为了视觉密度误用表格。

## When to Use

- **首选触发**：用户要求表格、比较表、矩阵、状态汇总，或当前答案天然就是短字段重复的二维信息。
- **典型场景**：方案对比、状态盘点、短清单汇总、风险矩阵、紧凑型 inventory。
- **边界提示**：若内容包含长解释、代码、日志、路径、URL、JSON 或稳定对齐存疑，应直接改用列表、分组块或正文。

## Input Arguments

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `headers` | array | yes | 列标题，需保持简短 |
| `rows` | array | yes | 行数据，单元格应尽量短小 |
| `max_width_hint` | integer | no | 目标总宽度提示，默认尽量控制在约 80 半角列以内 |
| `wrap_or_truncate` | enum | no | `wrap` / `truncate`，默认按可读性择优 |
| `notes` | array | no | 不适合入表的补充说明，放在表格后 |

## Outputs

```text
┌──────────┬──────────────┬──────────────┐
│ 维度     │ 选择         │ 说明         │
├──────────┼──────────────┼──────────────┤
│ 输出形式 │ 盒线表       │ 聊天摘要用   │
│ 工件格式 │ 原格式保留   │ 不改落盘结果 │
└──────────┴──────────────┴──────────────┘
```

## Integration

- 与 `arc:*` 组合时，只接管聊天总结、对比与状态盘点的表现层。
- 磁盘工件继续保持原始约定，例如 Markdown、JSON、HTML、text。
- 共享组合规则见 [`../docs/orchestration-contract.md`](../docs/orchestration-contract.md) 与 [`../docs/fusion-guide.md`](../docs/fusion-guide.md)。
