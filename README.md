# Arc Skills

跨运行时、可解耦的技能（Skill）仓库。  
本仓库统一采用 `arc:xxx` 命名空间，当前保留 17 个核心编排技能。

## 当前状态

- 已完成从单一工具耦合到运行时无关编排（`dispatch_job / collect_job`）的迁移。
- 所有 `arc:*` 技能已统一融合结构：`Quick Contract` / `Announce` / `The Iron Law` / `Workflow` / `Quality Gates` / `Red Flags`。
- 路由文档已形成三层：矩阵、决策树、阶段视图 + 单页速查。
- 所有 `SKILL.md` 的 frontmatter `description` 已统一为中文。

## Arc 技能清单（17）

| Skill | 目录 | 作用 |
|---|---|---|
| `arc:agent` | `agent/` | 统一入口，做需求理解、Skill 路由与多 Agent 调度 |
| `arc:refine` | `refine/` | 需求澄清与上下文补齐，输出可执行提示 |
| `arc:deliberate` | `deliberate/` | 多视角审议与反驳收敛，用于高风险决策 |
| `arc:estimate` | `estimate/` | round 模型估算、风险校准与并行波次规划 |
| `arc:implement` | `implement/` | 方案落地为代码变更，附验证与交接产物 |
| `arc:review` | `review/` | 企业级多维诊断与改进路线图 |
| `arc:gate` | `gate/` | CI/CD 质量门禁判定（阈值 + 豁免） |
| `arc:simulate` | `simulate/` | 用户路径 E2E 验证与 UI 证据沉淀 |
| `arc:triage` | `triage/` | 基于失败工件做根因定位、修复与复验 |
| `arc:loop` | `loop/` | 重启服务 + 多轮 fail-fix-retest 闭环 |
| `arc:cartography` | `cartography/` | 分层 codemap 生成与增量刷新 |
| `arc:uml` | `uml/` | 基于项目证据生成 14 类 UML 图谱（需 E-R 时按陈氏画法） |
| `arc:init` | `init/` | CLAUDE 索引维护自动路由（full/update） |
| `arc:init:full` | `init-full/` | CLAUDE 索引全量重建 |
| `arc:init:update` | `init-update/` | CLAUDE 索引增量更新 |
| `arc:ip-audit` | `ip-audit/` | 软著/专利可行性审查与风险评估 |
| `arc:ip-docs` | `ip-docs/` | 基于审查交接起草 IP 申请材料 |

## 我该用哪个 Skill？

| 你现在的情况 | 直接用 |
|---|---|
| 不知道该选哪个 | `arc:agent` |
| 需求说不清、约束不完整 | `arc:refine` |
| 方案有争议、风险高 | `arc:deliberate` |
| 方案已定，开始改代码 | `arc:implement` |
| 想评估项目健康度/PR 风险 | `arc:review` |
| 准备合并或发布，做门禁 | `arc:gate` |
| 需要真实用户路径 E2E 验证 | `arc:simulate` |
| E2E 失败或线上故障排查 | `arc:triage` |
| 修复后必须重启并多轮回归 | `arc:loop` |
| 刚接手陌生仓库，先看结构 | `arc:cartography` |
| 需要系统建模图（类图/时序图/部署图等） | `arc:uml` |
| 要维护 CLAUDE 索引（不想判断 full/update） | `arc:init` |

## 收敛结果

- 已取消独立“能力子技能”，全部能力并入核心流程，避免重复入口与职责分散。
- `arc:score` 对外入口已收敛到 `arc:gate`；内部保留 `score/` 模块负责量化评分，`arc:gate` 负责门禁判定。
- `arc:agent`：统一承接仓库摸底与任务拆解路由。
- `arc:implement`：统一承接契约先行、迁移、重构与文档同步交付。
- `arc:review`：统一承接测试策略、性能回归、安全基线与 PR 评审结论。
- `arc:triage`：统一承接 simulate 失败、flaky 与恢复闭环。
- `arc:gate`：统一承接发布前 Go/No-Go 与门禁判定。

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
- 禁止遗留耦合关键字（如 `Task(`、`subagent_type` 等）

## 快速开始

```bash
# 命名到命令映射
# arc:agent -> arc agent
# arc:init:full -> arc init full
# arc:init:update -> arc init update

# 推荐入口
arc agent

# 常用链路
arc refine
arc deliberate
arc implement
arc uml

# 质量链路
# `arc gate` 会编排触发 `score/` 评分模块
arc gate
arc review

# E2E 链路
arc simulate
arc triage
arc loop

# 初始化链路
arc init
arc init full
arc init update
```

## 约定

详见 `CLAUDE.md`。
