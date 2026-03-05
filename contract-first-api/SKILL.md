---
name: "contract-first-api"
description: "当设计或演进 API 时使用，先定义接口契约并在实现前识别破坏性变更风险。"
---

# contract-first-api

## Overview

以契约作为单一真相源，先约定再编码，降低联调成本。

## When to Use

- 新增 API 或服务间接口
- 多团队并行开发
- 需要严格控制 breaking changes

## Outputs

- 接口契约草案与版本策略
- 兼容性检查结果
- 破坏性变更清单与迁移建议
- 实现与契约一致性核对单

## Workflow

1. 先产出契约草案（字段、语义、错误码、版本）
2. 执行兼容性检查并标注 breaking changes
3. 输出迁移方案与灰度策略
4. 最后核对实现是否与契约一致
