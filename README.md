# Skills

跨运行时、可解耦的技能（Skill）仓库。

当前仓库按命名空间分为两组：

- `Arc/`：通用工程编排技能，主命名空间为 `arc:*`
- `Lazycat/`：Lazycat 相关发布与运营技能，主命名空间为 `lazycat:*`，由独立仓库 `git@gitea.ezer.heiyu.space:iwen-conf/Lazycat-Skills.git` 作为子模块维护

## 目录结构

```text
Skills/
├── Arc/
│   ├── arc:exec/
│   ├── arc:build/
│   └── ...
├── Lazycat/
│   └── lazycat:ship-app/
├── docs/
├── schemas/
├── scripts/
├── src/
└── tests/
```

## 当前状态

- 已完成从单一工具耦合到运行时无关编排（`schedule_task / collect_job`）的迁移。
- 所有 `arc:*` 技能已统一融合结构：`Quick Contract` / `Announce` / `The Iron Law` / `Workflow` / `Quality Gates` / `Red Flags`。
- 路由文档已形成三层：矩阵、决策树、阶段视图 + 单页速查。
- 所有 `SKILL.md` 的 frontmatter `description` 已统一为中文。
- 新增 `lazycat:*` 命名空间，用于封装 Lazycat 开发者中心、`lpk`、提审与发布闭环。
- `Lazycat/` 已拆分为独立 git 仓库，并通过 `.gitmodules` 挂回当前仓库。

## 命名规则（唯一口径）

- `Arc/` 下技能统一使用 `arc:*`
- `Lazycat/` 下技能统一使用 `lazycat:*`
- 通用辅助或调试 skill 不进入 `Lazycat/` 子模块，避免污染 Lazycat 领域边界
- Skill 只按“能力边界”命名，不再同时维护另一套“人类别名”
- 模式参数统一放在 Skill 内部（如 `--mode`），不再把每个模式都当成一级入口

## Arc 主技能（16）

| 分组 | Skill ID | 一句话用途 |
|---|---|---|
| 总控 | `arc:exec` | 一句话入口，自动路由与编排 |
| 方案 | `arc:clarify` | 把模糊需求补全为可执行输入 |
| 方案 | `arc:decide` | 高风险方案收敛（可含估算模式） |
| 交付 | `arc:build` | 按方案实施改动并交付验证 |
| 交付 | `arc:cartography` | 生成/刷新 codemap 结构视图 |
| 交付 | `arc:uml` | 生成标准 UML（E-R 用陈氏画法） |
| 索引 | `arc:init` | 自动维护索引（full/update 模式内聚） |
| 上下文 | `arc:context` | 生成/恢复任务上下文包与恢复清单 |
| 质量 | `arc:e2e` | 按真实路径执行 E2E 验证 |
| 质量 | `arc:test` | 代码级测试生成（单测/集成/边界/benchmark/fuzz） |
| 质量 | `arc:fix` | 基于失败证据定位并修复（可含 retest 模式） |
| 写作 | `arc:aigc` | 学术/专业文本去模板化润色与作者声线统一 |
| 治理 | `arc:audit` | 七维评审 + HTML 可视化报告 + 业务/依赖成熟度 |
| 治理 | `arc:gate` | 合并/上线门禁判定（Go/No-Go） |
| 知产 | `arc:ip-check` | 软著/专利可行性与风险评估 |
| 知产 | `arc:ip-draft` | 依据审查结果起草申请材料 |

## Lazycat 技能

| 分组 | Skill ID | 一句话用途 |
|---|---|---|
| 发布 | `lazycat:ship-app` | 覆盖懒猫应用从构思、打包、截图、提审到发布后核验的端到端流程 |

## 我该用哪个 Skill？

| 你现在的情况 | 直接用 |
|---|---|
| 只想一句话开工：“拉一个团队做这个任务” | `arc:exec` |
| 不知道该选哪个 | `arc:exec` |
| 要把应用上架到懒猫开发者中心 | `lazycat:ship-app` |
| 需求说不清、约束不完整 | `arc:clarify` |
| 方案有争议、风险高 | `arc:decide` |
| 方案已定，开始改代码 | `arc:build` |
| 想评估项目健康度/PR 风险 | `arc:audit` |
| 准备合并或发布，做门禁 | `arc:gate` |
| 论文/报告太像模板或机器写的，想在保留事实与引用前提下润色 | `arc:aigc` |
| 上下文太长、要切会话或换一个 agent 继续任务 | `arc:context` |
| 需要真实用户路径 E2E 验证 | `arc:e2e` |
| 需要给模块自动生成单测/边界/benchmark/fuzz | `arc:test` |
| E2E 失败或线上故障排查 | `arc:fix` |
| 修复后必须重启并多轮回归 | `arc:fix --mode retest-loop` |
| 刚接手陌生仓库，先看结构 | `arc:cartography` |
| 需要系统建模图（类图/时序图/部署图等） | `arc:uml` |
| 要维护索引（不想判断 full/update） | `arc:init` |

## 最简记忆（只记 5 个）

- 不知道用什么：`arc:exec`
- 要改代码交付：`arc:build`
- 要项目体检打分：`arc:audit`
- 要发布门禁：`arc:gate`
- 要画 UML：`arc:uml`
- 要走懒猫上架链路：`lazycat:ship-app`

## 收敛结果

- 对外主技能现为 16 个，能力边界覆盖总控、方案、交付、上下文、质量、治理、写作与知产。
- 新增 `arc:aigc`，承接学术/专业文本的去模板化润色、两阶段改写与引用保真检查。
- 新增 `arc:context`，承接长任务的上下文压缩、恢复与跨会话交接。
- `arc:score` 对外入口收敛到 `arc:gate`；内部 `score/` 负责量化评分，`arc:gate` 负责门禁判定。
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
- 所有被主校验器索引的 skill 名称必须为 `arc:xxx` 或 `lazycat:xxx`
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
arc context
arc aigc
arc uml

# 质量链路
# `arc gate` 会编排触发 `score/` 评分模块
arc gate
arc audit
arc test

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
