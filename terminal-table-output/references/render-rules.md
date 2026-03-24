# Terminal Table Render Rules

本文件承载 `terminal-table-output` 的细则；`SKILL.md` 仅保留触发、流程与质量门禁。

## Decide Whether To Use A Table

- 仅在信息天然适合二维结构、且字段短小重复时使用表格。
- 典型适配：比较、状态汇总、短 inventory、紧凑矩阵。
- 不要把解释性正文、错误细节、文件路径、URL、代码、JSON、长注释硬塞入表格。
- 如果只有一部分适合表格，先给一个紧凑摘要表，再在表格后用列表补充长说明。

## Render Rules

- 所有表格都放在 fenced code block，info string 必须是 `text`。
- 使用 Unicode 盒线字符绘制边框与分隔线。
- 除非用户明确要求 Markdown tables，否则不要输出 Markdown 表。
- 以**显示宽度**而不是字符个数对齐列宽。
- 中文和其他全角字符按宽度 `2` 计算。
- 英文字母、数字、半角符号按宽度 `1` 计算。
- 右侧用空格补齐列宽，不能使用 tab。
- 每一行总宽度必须完全一致，且左右边界闭合。

## Width And Long Content

- 默认优先控制在约 `80` 个半角列以内。
- 如果表格会过宽，先减少列数、缩短表头或把长说明移出表格。
- 必要时可在单元格内手动换行，但必须保持外框完整。
- 如果必须截断，显式加 `...`。
- 如果仍无法稳定对齐，直接放弃表格，改用列表或正文。

## Output Priority

- 可读性优先于密度。
- 宁可少列，也不要产出一张宽而乱的表。
- 对齐稳定优先于信息“全塞进去”。

## Composition Rule

- 这是**输出范式**，不是领域技能。
- 它可以与 `arc:exec`、`arc:decide`、`arc:audit`、`arc:gate`、`arc:build` 等技能组合使用。
- 组合时仅影响聊天呈现；磁盘工件、报告文件和程序输出格式保持原样。
