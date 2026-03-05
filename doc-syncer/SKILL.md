---
name: "doc-syncer"
description: "当代码变更可能影响文档时使用，自动识别文档影响并生成可追溯更新草稿。"
---

# doc-syncer

## Overview

保障“代码变更—文档更新”同步闭环。

**核心原则：代码改了，文档要有可追溯更新。**

## Announce

开始时明确说明：  
“我正在使用 `doc-syncer`，将先做文档影响分析，再输出更新草稿与缺口。”

## When to Use

- 功能已完成准备合并
- API/配置/流程发生变更
- 发布前核对文档一致性

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `change_scope` | string[] | 是 | 代码变更文件或模块范围 |
| `doc_roots` | string[] | 否 | 文档目录（如 docs/, README, ADR） |
| `release_context` | string | 否 | 发布说明上下文 |

## Dependencies

- **编排契约**（推荐）：遵循 `docs/orchestration-contract.md`，支持多运行时文档同步流程。

## Outputs

- 文档影响分析清单
- 必改/建议改项
- 更新草稿与待确认点
- 文档同步完成度报告
- 发布文档缺口预警
- 代码变更到文档更新的可追溯映射

## The Iron Law

```
NO MERGE FOR API/BEHAVIOR CHANGE WITHOUT DOC IMPACT CHECK
```

接口或行为变更若无文档影响结论，不得判定“文档已同步”。

## Workflow

### Phase 1: 影响识别
1. 基于变更范围定位受影响文档
2. 按类型分类（API/配置/流程/运行说明）
3. 标注必须更新项与建议更新项

### Phase 2: 草稿生成
1. 生成最小必要更新草稿
2. 保持现有文档风格与术语一致
3. 标注“需人工确认”内容

### Phase 3: 可追溯核对
1. 建立“代码变更 → 文档条目”映射
2. 标注未覆盖变更
3. 输出同步完成度与剩余动作

### Phase 4: 发布前确认
1. 输出发布文档缺口预警
2. 提供最终核对清单
3. 给出“可发布文档状态”结论

## Quality Gates

- API/配置变更必须有对应文档更新项
- 草稿不得臆造未实现能力
- 每条文档建议必须可追溯到代码变更

## Red Flags（必须停下重做）

- 只更新 README，忽略接口/运维文档
- 草稿包含未落地能力或猜测内容
- 变更影响项无责任人
- 发布前没有文档状态结论
