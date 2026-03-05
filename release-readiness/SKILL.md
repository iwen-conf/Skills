---
name: "release-readiness"
description: "当发布前需要一次性核查版本、回滚、监控与公告准备度时使用。"
---

# release-readiness

## Overview

将发布准备标准化为一套可执行清单。

**核心原则：没有回滚与监控，就不算可发布。**

## Announce

开始时明确说明：  
“我正在使用 `release-readiness`，会先给出 Go/No-Go 发布结论与阻塞项。”

## When to Use

- 任意版本上线前
- 多服务联动发布
- 高风险窗口发布

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `release_scope` | string | 是 | 本次发布范围 |
| `env` | string | 是 | 发布环境（staging/prod 等） |
| `change_items` | string[] | 否 | 关键变更项 |
| `rollback_requirements` | string[] | 否 | 回滚前提与时限 |

## Dependencies

- **编排契约**（推荐）：遵循 `docs/orchestration-contract.md`，支持不同运行时统一执行发布检查。

## Outputs

- 发布检查清单（通过/阻塞）
- 回滚策略核对结果
- 监控告警与值班准备度
- 上线公告/变更说明模板
- 风险放行建议（Go/No-Go）
- 发布阻塞项与责任人清单

## The Iron Law

```
NO ROLLBACK, NO RELEASE
```

无法在可接受时间窗内回滚的变更，默认阻塞发布。

## Workflow

### Phase 1: 变更核对
1. 校验版本号、变更单、发布范围一致性
2. 核对依赖变更（配置/迁移/接口兼容）
3. 标注高风险变更项

### Phase 2: 回滚演练检查
1. 校验回滚路径（代码、数据、配置）
2. 校验回滚时限与触发条件
3. 校验负责人和值班可达性

### Phase 3: 观测与告警检查
1. 核对核心链路监控覆盖
2. 核对告警阈值与噪音水平
3. 校验应急响应路径

### Phase 4: 发布决策
1. 给出 Go/No-Go 结论
2. 输出阻塞项、责任人、预计解除时间
3. 生成上线公告与回滚公告模板

## Quality Gates

- 无回滚方案的发布默认判定为阻塞
- 无监控覆盖的核心链路默认判定为高风险
- 结论必须明确 Go/No-Go 与责任人

## Red Flags（必须停下重做）

- 仅“感觉稳定”但无监控数据支持
- 回滚脚本存在但未校验可执行性
- 关键告警无人值守
- 高风险变更未定义触发回滚阈值
