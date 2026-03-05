---
name: "arc:init"
description: "当需要构建或刷新层级 CLAUDE.md 索引并自动选择全量或增量模式时使用。"
---

# arc:init — 智能调度器

## Overview

本 Skill 是 `arc:init` 子系统的**智能调度入口**，根据项目状态自动选择最优工作模式：

- **全量模式** (`arc:init:full`)：首次初始化或强制全量刷新
- **增量模式** (`arc:init:update`)：基于指纹检测，智能增量更新

**用户无需关心选择**：日常使用只需调用本入口，系统自动判断。

## Quick Contract

- **Trigger**：需要建立或刷新项目 CLAUDE 索引体系，但不想手动选择 full/update。
- **Inputs**：`project_path`、可选 `mode`、扫描深度与输出参数。
- **Outputs**：模式决策结果、子技能执行记录与更新后的索引产物。
- **Quality Gate**：执行完成后必须通过 `## Quality Gates` 的路由与产物完整性检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:init`，先判断全量或增量模式，再执行索引生成。”

## The Iron Law

```
NO INDEX UPDATE WITHOUT MODE DECISION AND BASELINE CHECK
```

未完成模式决策与基线校验，不得开始索引写入。

## Workflow

1. 检查指纹与快照状态，确定 full/update 路由。
2. 组装输入参数并下发至对应子 Skill。
3. 追踪执行结果并校验产物完整性。
4. 输出模式选择依据与后续维护建议。

## Quality Gates

- 路由决策必须附条件命中说明。
- 子 Skill 调度参数必须完整可复现。
- 产物必须包含最新元数据与时间戳。
- 失败时必须给出可执行回退路径。

## Red Flags

- 未检查 fingerprints 就强行增量更新。
- 全量/增量判定理由缺失。
- 子 Skill 失败后仍标记初始化完成。
- 输出缺失关键索引文件却继续下游流程。

## When to Use

- **首选触发**：需要自动选择全量或增量来维护 CLAUDE 索引。
- **典型场景**：日常索引维护，但不希望手工判断 `full/update`。
- **边界提示**：明确要全量时用 `arc:init:full`，明确增量时用 `arc:init:update`。

## 工作模式选择

```
用户输入 → 指纹检测 → 路由决策

┌─────────────────────────────────────────┐
│           arc:init (智能调度)            │
└────────────────┬────────────────────────┘
                 │
                 ▼
        ┌────────────────┐
        │ 检查 fingerprints │
        │ (.arc/init/... ) │
        └────────┬────────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
   存在 + 新鲜         不存在/过期
        │                 │
        ▼                 ▼
   arc:init:update   arc:init:full
   (增量模式)          (全量模式)
```

### 自动路由规则

| 条件 | 路由到 | 说明 |
|------|--------|------|
| `module-fingerprints.json` 不存在 | `arc:init:full` | 首次初始化 |
| `module-fingerprints.json` 存在但 git_ref 不匹配 | `arc:init:update` | 检测到 git 变更 |
| `module-fingerprints.json` 存在但过期 (>7天) | `arc:init:update` | 定期更新 |
| 用户显式指定 `mode=full` | `arc:init:full` | 强制全量 |
| 用户显式指定 `mode=update` | `arc:init:update` | 强制增量 |

### 强制路由参数

用户可以通过 prompt 指定模式：

| 参数 | 行为 |
|------|------|
| `force full` / `全量` / `重新初始化` | 强制路由到 `arc:init:full` |
| `增量` / `更新` / `只更新变更` | 强制路由到 `arc:init:update` |

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 项目根目录绝对路径 |
| `project_name` | string | 否 | 项目名称 |
| `depth_level` | string | 否 | 扫描深度：`shallow` / `standard` / `deep` |
| `max_module_depth` | number | 否 | 模块最大深度，默认 3 |
| `language` | string | 否 | 输出语言：`zh-CN` / `en` |
| `output_dir` | string | 否 | 工作目录 |

> 注意：本入口不直接执行扫描和分析，而是调度子 Skill。

## Execution Flow

### Step 0: 模式检测

1. **检查 `module-fingerprints.json` 是否存在**
   - 路径：`<project_path>/.arc/init/context/module-fingerprints.json`
   
2. **检查快照新鲜度**
   - 生成时间 < 7 天：视为有效基线
   - 生成时间 >= 7 天：提示过期，建议更新

3. **检查 Git 变更**
   - 对比 `fingerprints.git_ref` 与当前 HEAD
   - 不一致：存在未同步变更

4. **检查用户显式指定**
   - prompt 中包含强制模式关键词

### Step 1: 路由决策

根据检测结果做出路由决策：

```python
if user_force_mode == "full":
    route_to("arc:init:full")
elif user_force_mode == "update":
    route_to("arc:init:update")
elif not fingerprints_exist:
    route_to("arc:init:full")  # 首次
elif fingerprints_stale or git_changed:
    route_to("arc:init:update")  # 增量
else:
    route_to("arc:init:update")  # 默认增量
```

### Step 2: 执行调度

使用 **Dispatch API** 调度对应的子 Skill：

```typescript
// 调度 arc:init:full
dispatch_job(
  lane: "unspecified-high",
  capabilities: ["arc:init:full"],
  prompt: `执行全量初始化...

  项目路径: <project_path>
  深度级别: <depth_level>
  语言: <language>
  ...`,
  execution_mode: "foreground"
)

// 或调度 arc:init:update
dispatch_job(
  lane: "unspecified-high",
  capabilities: ["arc:init:update"],
  prompt: `执行增量更新...

  项目路径: <project_path>
  深度级别: <depth_level>
  语言: <language>
  ...`,
  execution_mode: "foreground"
)
```

### Step 3: 结果聚合

1. 收集子 Skill 执行结果
2. 汇总变更统计
3. 输出最终报告
4. 发布 `CLAUDE.md` 产物元数据到 `.arc/context-hub/index.json`（路径、哈希、过期时间、刷新入口）
5. 若共享索引不存在，则先初始化最小索引结构再写入本次产物

## Sub-Skills

### arc:init:full

全量初始化模式。适用于：
- 首次初始化项目索引
- 强制全量刷新
- 指纹文件损坏需要重建

**执行内容**：
- 深度扫描项目结构
- 多 Agent 协作分析（oracle + deep + momus）
- 交叉审阅
- 全量生成 CLAUDE.md
- 生成指纹基线

### arc:init:update

增量更新模式。适用于：
- 日常维护更新
- 部分模块变更后同步
- 定期索引刷新

**执行内容**：
- 变更检测（指纹对比 + git diff）
- 选择性 Agent 分析
- 增量生成 CLAUDE.md
- 更新指纹基线

## Quick Usage

| 场景 | 调用方式 |
|------|----------|
| 首次初始化 | `arc-runtime run arc:init` → 自动路由到 full |
| 日常更新 | `arc-runtime run arc:init` → 自动路由到 update |
| 强制全量 | `arc-runtime run arc:init:full` |
| 强制增量 | `arc-runtime run arc:init:update` |
| 查看帮助 | `arc-runtime run arc:init --help` |

## 状态反馈示例

```
[Arc Init] 项目: my-project
=== 模式检测 ===
  ├── 指纹文件: 存在 (生成于 2026-02-25)
  ├── Git 变更: abc1234 → def5678 (15 个文件变更)
  └── 路由决策: 增量更新 (arc:init:update)

=== 增量更新 ===
  ├── 变更检测... [完成]
  │   ├── 新增模块: 2
  │   ├── 修改模块: 5
  │   └── 删除模块: 1
  ├── Agent 分析... [完成]
  │   ├── 新增模块完整分析: 2 个模块
  │   └── 修改模块浅层分析: 5 个模块
  ├── 增量生成... [完成]
  │   ├── 新建 CLAUDE.md: 2
  │   ├── 更新 CLAUDE.md: 7
  │   └── 删除 CLAUDE.md: 1
  └── 验证... [通过]

总计更新 10 个 CLAUDE.md 文件。
```

## 与其他 Skill 的关系

```
arc:init (本 Skill - 智能调度)
 ├── arc:init:full (全量初始化)
 └── arc:init:update (增量更新)

下游消费者:
arc:refine / arc:deliberate / arc:implement / arc:simulate / arc:review
  ↑ 通过 `.arc/context-hub/index.json` 读取 CLAUDE 索引产物

依赖关系:
arc:init:update 需要 arc:init:full 生成的指纹基线

发布关系:
arc:init 必须将本次生成/更新的 CLAUDE.md 元数据发布到共享索引
```

## 故障排除

| 情况 | 解决方案 |
|------|----------|
| 指纹文件不存在 | 提示运行 `arc:init` 或 `arc:init:full` |
| 指纹文件损坏 | 提示运行 `arc:init:full` 重建 |
| Git 仓库不存在 | 仅支持 git 仓库 |
| 子 Skill 执行失败 | 查看 `.arc/init/` 工作目录日志 |

## 调用示例

```bash
# 首次初始化（自动路由到 full）
arc-runtime run arc:init /path/to/new-project

# 日常更新（自动路由到 update）
arc-runtime run arc:init /path/to/existing-project

# 强制全量
arc-runtime run arc:init:full /path/to/project

# 强制增量
arc-runtime run arc:init:update /path/to/project
```
