# Superpowers 融合指南（已应用到本仓库）

本指南提炼自 `superpowers/` 的高价值模式，并映射到 ARC 与通用 Skills。

## 融合目标

- 保留 ARC 的领域能力
- 吸收 Superpowers 的流程纪律
- 保持运行时无关（不绑定单一 CLI/平台）

## 迁移的核心模式

1. **触发优先描述**
   - frontmatter `description` 优先描述“何时使用”
   - 采用 `Use when ...` 触发式表达

2. **Iron Law（不可违背约束）**
   - 每个关键 Skill 都有一条不可绕过规则
   - 例如：无验收标准不进入实现、无回滚不发布

3. **阶段化流程**
   - 统一 Phase 风格（识别 → 分析 → 执行 → 交付）
   - 降低执行歧义与漏项

4. **Red Flags（反模式止损）**
   - 显式列出常见错误思路
   - 一旦触发，要求回到流程起点

5. **产物契约**
   - 输出明确、可追溯、可被下游消费
   - 强调证据与结论分离

## 适配到本仓库后的约束

- 编排统一使用 `docs/orchestration-contract.md`
- 禁止回退到旧调度字段（`Task(...)`、`subagent_type` 等）
- 技能文档优先保留短小、可执行、可检验结构

## 已重点强化的 Skill（ROI 优先）

1. `requirements-refiner`
2. `repo-onboarding-map`
3. `test-matrix-builder`
4. `release-readiness`
5. `doc-syncer`

## 持续演进建议

- 新建 Skill 时，优先复用本指南结构：
  - `Overview`
  - `When to Use`
  - `Input Arguments`
  - `Outputs`
  - `The Iron Law`
  - `Workflow`
  - `Quality Gates`
  - `Red Flags`
