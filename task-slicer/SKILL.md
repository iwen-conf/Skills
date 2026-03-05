---
name: "task-slicer"
description: "当大需求需要拆分为可并行且具依赖关系的小任务时使用。"
---

# task-slicer

## Overview

把“大而全需求”切成“小步快跑任务包”，支持并行推进与风险隔离。

## When to Use

- 任务规模大、协作角色多
- 需要并行开发与阶段交付
- 依赖关系复杂导致排期不稳

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `goal` | string | 是 | 总体目标 |
| `constraints` | string[] | 否 | 时间、资源、质量约束 |
| `team_shape` | string | 否 | 团队能力/人数信息 |

## Outputs

- 任务树（Epic → Story → Task）
- 依赖图（阻塞/可并行）
- 优先级（P0/P1/P2）
- 里程碑与交付节奏

## Workflow

1. 按目标拆分主路径与支线
2. 切分最小可交付单元
3. 标注依赖与并行窗口
4. 计算关键路径并定优先级
5. 产出迭代计划
