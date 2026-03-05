---
name: "test-matrix-builder"
description: "当需要制定测试策略并补齐覆盖缺口时使用，生成功能/边界/异常测试矩阵。"
---

# test-matrix-builder

## Overview

把测试从“拍脑袋补 case”升级为系统化覆盖矩阵。

**核心原则：覆盖按风险分层，不按感觉补点。**

## Announce

开始时明确说明：  
“我正在使用 `test-matrix-builder`，先构建风险驱动测试矩阵，再落测试实现。”

## When to Use

- 新功能上线前补齐测试
- 复杂业务规则需要覆盖边界
- 回归集过大需重构测试策略

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `feature_scope` | string | 是 | 本次功能范围 |
| `risk_focus` | string[] | 否 | 重点风险（安全、并发、资金、权限等） |
| `test_levels` | string[] | 否 | 需要产出的层级（unit/integration/e2e） |

## Dependencies

- **编排契约**（推荐）：遵循 `docs/orchestration-contract.md`，便于跨运行时复用矩阵产物。

## Outputs

- 功能 × 场景 × 风险矩阵
- 单测/集成/E2E 分层建议
- 必测边界与异常清单
- 覆盖缺口与优先补测项
- 最小回归集建议（高收益低成本）
- 测试执行优先级（P0/P1/P2）

## The Iron Law

```
NO SHIP WITH UNKNOWN HIGH-RISK COVERAGE GAPS
```

高风险路径若无覆盖结论，不得给出“可发布”判断。

## Workflow

### Phase 1: 场景抽取
1. 抽取核心业务主路径
2. 列出关键断言与失败条件
3. 标注高风险节点

### Phase 2: 矩阵构建
1. 对每条路径补齐：正常/边界/异常
2. 增加并发、时序、权限、数据一致性场景（按需）
3. 形成“功能 × 场景 × 风险”矩阵

### Phase 3: 分层映射
1. 将矩阵映射到 unit/integration/e2e
2. 标注每个用例的执行成本与收益
3. 确定 P0/P1/P2 测试优先级

### Phase 4: 回归最小集
1. 选出发布前最小高收益回归集
2. 标注必须阻塞项
3. 输出缺口与补测建议

## Quality Gates

- 每个核心功能至少覆盖：正常/边界/异常
- 高风险路径必须包含集成或 E2E 验证
- 用例描述必须可直接转为测试代码或手工步骤

## Red Flags（必须停下重做）

- 只有 happy path 没有边界/异常
- 单测全绿但关键链路无集成验证
- 用例不能映射到可执行步骤
- 覆盖声明没有风险分级依据
