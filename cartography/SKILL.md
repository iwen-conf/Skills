---
name: arc:cartography
description: "当需要快速理解陌生仓库或为下游 arc 技能刷新分层 codemap.md 上下文时使用。"
---

# arc:cartography Skill

## Overview

生成可增量维护的分层 `codemap.md`，为 `arc:refine`、`arc:implement`、`arc:review` 等下游技能提供稳定上下文。

## Quick Contract

- **Trigger**：需要快速理解陌生仓库，或下游技能需要最新结构化上下文。
- **Inputs**：仓库根路径、包含/排除规则、是否首次初始化。
- **Outputs**：分层 `codemap.md`、可选 tier JSON、`.arc/context-hub/index.json` 元数据。
- **Quality Gate**：发布前必须通过 `## Quality Gates` 的增量准确性与索引一致性检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:cartography`，先更新 codemap 基线，再交给下游技能复用。”

## The Iron Law

```
NO MAJOR CHANGE WITHOUT CURRENT CODEMAP
```

关键结构变更前，必须先有最新 codemap 作为上下文基线。

## Dependencies

- **编排契约**：必须。遵循 `docs/orchestration-contract.md`，通过运行时适配层并发分派目录分析任务。
- **cartographer.py**：必须。路径为 `<skills_root>/cartography/scripts/cartographer.py`。

## When to Use

- **首选触发**：需要构建或刷新仓库结构地图（`codemap.md`）。
- **典型场景**：首次接手陌生仓库、目录结构大改后同步上下文。
- **边界提示**：需求澄清用 `arc:refine`，代码落地用 `arc:implement`。

## Workflow

### Step 1: 检查状态

优先检查仓库根目录 `.slim/cartography.json`：
- 存在：进入变更检测流程
- 不存在：执行初始化流程

### Step 2: 初始化（仅首次）

```bash
python3 <skills_root>/cartography/scripts/cartographer.py init \
  --root ./ \
  --include "src/**/*.ts" \
  --exclude "**/*.test.ts" --exclude "dist/**" --exclude "node_modules/**"
```

初始化后会生成：
- `.slim/cartography.json`（变更检测基线）
- 相关目录下的 `codemap.md` 占位文件

### Step 3: 增量检测与更新

```bash
python3 <skills_root>/cartography/scripts/cartographer.py changes --root ./
python3 <skills_root>/cartography/scripts/cartographer.py update  --root ./
```

规则：
- 只更新受影响目录的 `codemap.md`
- 未受影响目录不重写

### Step 4: 可选导出分层 JSON

```bash
python3 <skills_root>/cartography/scripts/cartographer.py export --root ./ --tier 1 --output codemap/index.json
python3 <skills_root>/cartography/scripts/cartographer.py export --root ./ --tier 2 --output codemap/context.json
python3 <skills_root>/cartography/scripts/cartographer.py export --root ./ --tier 3 --output codemap/full.json
```

建议：
- Tier 1：快速检索入口
- Tier 2：模块关系分析
- Tier 3：深度实现上下文

### Step 5: 目录并发分析与根图汇总

1. 通过 `dispatch_job(...)` 并发分派各目录分析任务（按 `docs/orchestration-contract.md`）。
2. 每个目录输出/更新本目录 `codemap.md`。
3. 最后聚合根级 `codemap.md`，形成仓库 Atlas 入口。

### Step 6: 发布共享上下文元数据

将根级与目录级 `codemap` 产物写入 `.arc/context-hub/index.json`，每条记录至少包含：
- `path`
- `content_hash`
- `generated_at`
- `ttl_seconds`
- `expires_at`
- `producer_skill=arc:cartography`
- `refresh_skill=arc:cartography`

## Codemap 写作要求

- **Responsibility**：说明目录职责与边界
- **Design Patterns**：标注关键模式和抽象层
- **Data & Control Flow**：描述主要调用链/数据流
- **Integration Points**：列出依赖与被依赖方

## Quality Gates

- 增量流程必须只更新受影响目录。
- 根级与目录级 codemap 必须保持链接可达。
- `.arc/context-hub/index.json` 必须同步刷新元数据。
- tier 导出产物必须与实际结构一致且可消费。

## Red Flags

- 跳过变更检测直接全量重写所有 codemap
- 不区分受影响目录与未变更目录
- 根级 atlas 不更新导致下游入口过期
- 无共享索引发布导致下游无法复用产物

示例（目录级）：

```markdown
# src/services/

## Responsibility
封装业务服务层，组合仓储层与外部网关。

## Design
- 采用 Service + Repository 分层
- 使用策略模式选择不同支付渠道实现

## Flow
1. API Handler 调用 Service
2. Service 完成参数校验与事务编排
3. Repository 持久化并返回领域对象

## Integration
- 被 `src/api/` 调用
- 依赖 `src/repositories/` 与 `src/gateways/`
```
