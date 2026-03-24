# Skill 融合指南（已应用到本仓库）

本指南提炼外部优秀 Skill 体系中的高价值模式，并映射到 ARC 核心 Skills。

## 融合目标

- 保留 ARC 的领域能力
- 吸收成熟 Skill 体系的流程纪律
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

6. **输出范式可复用**
   - 领域 Skill 与表现层 Skill 分离
   - 允许把“怎么展示”收敛为可组合的 generic skill，而不是把同一套排版规则复制到每个 Skill
   - 当前仓库示例：`terminal-table-output`，用于把聊天中的紧凑二维摘要渲染为终端盒线表

## 适配到本仓库后的约束

- 编排统一使用 `docs/orchestration-contract.md`
- 禁止回退到旧调度字段（旧 API 占位符等）
- 技能文档优先保留短小、可执行、可检验结构

## 已重点强化的 Skill（ROI 优先）

1. `arc:exec`
2. `arc:audit`
3. `arc:gate`
4. `arc:build`
5. `arc:fix`

## 共享输出范式

- `terminal-table-output`
  - 角色：共享聊天输出范式，不承担领域决策
  - 适用：比较、状态汇总、矩阵、短库存表
  - 不适用：长解释、代码、URL、文件路径、JSON、长日志
  - 组合原则：只改变聊天表现层，不改变工件落盘格式

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
- 若 Skill 主要提供输出风格而不是领域能力，优先做成 generic/fusion skill，并通过共享文档或关键领域 Skill 进行组合接入。
