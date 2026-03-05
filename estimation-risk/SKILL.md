---
name: "estimation-risk"
description: "当做排期或资源评估且存在不确定性时使用，输出工时区间、风险等级和缓冲建议。"
---

# estimation-risk

## Overview

以区间估算替代单点拍脑袋，显式管理不确定性。

## When to Use

- 需求刚明确，准备排期
- 方案复杂度较高
- 需要给出风险缓冲依据

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `task_list` | string[] | 是 | 已拆分任务列表 |
| `historical_refs` | string[] | 否 | 历史类似任务 |
| `risk_factors` | string[] | 否 | 依赖、外部系统、人员可用性等 |

## Outputs

- 乐观/基准/保守三档估算
- 风险等级与触发条件
- 缓冲建议（时间/资源）
- 关键不确定性清单

## Workflow

1. 基于任务清单估算基准工作量
2. 识别风险并映射到影响系数
3. 输出区间估算与缓冲建议
4. 标注关键不确定性与复估触发条件
