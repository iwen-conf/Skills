---
name: "requirements-refiner"
description: "当需求在规划或编码前仍模糊不完整时使用，固化目标、范围与验收标准。"
---

# requirements-refiner

## Overview

将“想法级描述”转成“工程可执行定义”，减少返工与误解。

**核心原则：先对齐定义，再进入计划与实现。**

## Announce

开始时明确说明：  
“我正在使用 `requirements-refiner`，先把需求固化为可执行定义。”

## When to Use

- 用户需求表达模糊、存在多义词
- 任务目标与边界不清晰
- 缺少验收标准和失败判定
- 多方协作前需要统一“完成定义”

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `raw_request` | string | 是 | 原始需求描述 |
| `project_context` | string | 否 | 项目背景、技术栈、业务上下文 |
| `constraints` | string[] | 否 | 预算、时间、合规、技术限制 |

## Dependencies

- **编排契约**（推荐）：遵循 `docs/orchestration-contract.md`，可通过任意运行时适配层执行。

## Outputs

- `目标定义`（业务目标 + 技术目标）
- `范围边界`（In Scope / Out of Scope）
- `验收标准`（可验证、可度量）
- `风险清单`（概率/影响/缓解）
- `待澄清问题`（按优先级）
- `执行前置条件`（依赖、假设、阻塞项）

## The Iron Law

```
NO IMPLEMENTATION BEFORE ACCEPTANCE CRITERIA IS EXPLICIT
```

若验收标准仍不可验证，不得进入实现阶段。

## Workflow

### Phase 1: 意图澄清
1. 抽取业务目标与技术目标
2. 标注显性/隐性约束
3. 识别关键术语并统一定义

### Phase 2: 范围定界
1. 列出 In Scope / Out of Scope
2. 标注依赖、前置条件、阻塞项
3. 输出可执行边界说明

### Phase 3: 验收固化
1. 生成可验证验收条目（Given/When/Then 或等价）
2. 标注成功判定与失败判定
3. 识别不可测试项并转为澄清问题

### Phase 4: 风险与决策
1. 输出风险清单（概率/影响/缓解）
2. 形成待澄清问题优先级
3. 给出“是否可进入 planning”的结论

## Quality Gates

- 不允许仅输出笼统建议，必须可执行
- 验收标准必须可观察、可验证
- 每个风险必须带缓解策略
- 所有关键术语必须有明确释义

## Red Flags（必须停下重做）

- “先做起来再说，后面再补验收标准”
- “范围先不定，边做边看”
- “这个需求应该大家都懂”
- “没有成功判定但已经开始排期/编码”
