# Arc Skills

跨运行时、可解耦的技能（Skill）仓库。  
本仓库统一采用 `arc:xxx` 命名空间，对外主技能收敛为 13 个。

## 当前状态

- 已完成从单一工具耦合到运行时无关编排（`schedule_task / collect_job`）的迁移。
- 所有 `arc:*` 技能已统一融合结构：`Quick Contract` / `Announce` / `The Iron Law` / `Workflow` / `Quality Gates` / `Red Flags`。
- 路由文档已形成三层：矩阵、决策树、阶段视图 + 单页速查。
- 所有 `SKILL.md` 的 frontmatter `description` 已统一为中文。

## 命名规则（唯一口径）

- 对外与对内统一只使用一套 Skill ID：`arc:*`
- Skill 只按“能力边界”命名，不再同时维护另一套“人类别名”
- 模式参数统一放在 Skill 内部（如 `--mode`），不再把每个模式都当成一级入口

## 对外主技能（13）

| 分组 | Skill ID | 一句话用途 |
|---|---|---|
| 总控 | `arc:exec` | 一句话入口，自动路由与编排 |
| 方案 | `arc:clarify` | 把模糊需求补全为可执行输入 |
| 方案 | `arc:decide` | 高风险方案收敛（可含估算模式） |
| 交付 | `arc:build` | 按方案实施改动并交付验证 |
| 交付 | `arc:cartography` | 生成/刷新 codemap 结构视图 |
| 交付 | `arc:model` | 生成标准 UML（E-R 用陈氏画法） |
| 索引 | `arc:init` | 自动维护索引（full/update 模式内聚） |
| 质量 | `arc:e2e` | 按真实路径执行 E2E 验证 |
| 质量 | `arc:fix` | 基于失败证据定位并修复（可含 retest 模式） |
| 治理 | `arc:audit` | 七维评审 + HTML 可视化报告 + 业务/依赖成熟度 |
| 治理 | `arc:release` | 质量阈值判定与 Go/No-Go |
| 知产 | `arc:ip-check` | 软著/专利可行性与风险评估 |
| 知产 | `arc:ip-draft` | 依据审查结果起草申请材料 |

## 我该用哪个 Skill？

| 你现在的情况 | 直接用 |
|---|---|
| 只想一句话开工：“拉一个团队做这个任务” | `arc:exec` |
| 不知道该选哪个 | `arc:exec` |
| 需求说不清、约束不完整 | `arc:clarify` |
| 方案有争议、风险高 | `arc:decide` |
| 方案已定，开始改代码 | `arc:build` |
| 想评估项目健康度/PR 风险 | `arc:audit` |
| 准备合并或发布，做门禁 | `arc:release` |
| 需要真实用户路径 E2E 验证 | `arc:e2e` |
| E2E 失败或线上故障排查 | `arc:fix` |
| 修复后必须重启并多轮回归 | `arc:fix --mode retest-loop` |
| 刚接手陌生仓库，先看结构 | `arc:cartography` |
| 需要系统建模图（类图/时序图/部署图等） | `arc:model` |
| 要维护索引（不想判断 full/update） | `arc:init` |

## 最简记忆（只记 5 个）

- 不知道用什么：`arc:exec`
- 要改代码交付：`arc:build`
- 要项目体检打分：`arc:audit`
- 要发布门禁：`arc:release`
- 要画 UML：`arc:model`

## 收敛结果

- 入口从 17 个收敛为 13 个主技能，减少重复选择成本。
- `arc:score` 对外入口收敛到 `arc:release`；内部 `score/` 负责量化评分，`arc:release` 负责门禁判定。
- 路由、矩阵、速查与 README 统一采用同一套 Skill ID 口径。

## 路由文档

- 单页速查：`docs/arc-routing-cheatsheet.md`
- 路由矩阵：`docs/arc-routing-matrix.md`
- 决策树：`docs/arc-routing-matrix.md#signal-to-skill-decision-tree`
- 阶段视图：`docs/arc-routing-matrix.md#phase-routing-view`

## 编排与融合文档

- 编排契约：`docs/orchestration-contract.md`
- 融合指南：`docs/fusion-guide.md`
- 执行模式说明：`docs/conductor-pattern.md`

## 质量校验

```bash
python3 scripts/validate_skills.py
```

当前校验会检查（节选）：

- `arc:*` frontmatter 仅允许 `name` + `description`
- 所有 skill 名称必须为 `arc:xxx`
- `description` 必须包含中文
- `arc:*` 必须包含统一结构段落与路由链接
- `When to Use` 必须包含：`首选触发 / 典型场景 / 边界提示`
- 禁止遗留旧调度关键字（旧 API 占位符等）

## 快速开始

```bash
# 命名到命令映射
# arc:exec -> arc exec
# arc:init --mode full -> arc init --mode full
# arc:init --mode update -> arc init --mode update

# 推荐入口
arc exec

# 常用链路
arc clarify
arc decide
arc build
arc model

# 质量链路
# `arc release` 会编排触发 `score/` 评分模块
arc release
arc audit

# E2E 链路
arc e2e
arc fix
arc fix --mode retest-loop

# 初始化链路
arc init
arc init --mode full
arc init --mode update
```

## 约定

详见 `docs/orchestration-contract.md` 与 `docs/arc-routing-matrix.md`。
