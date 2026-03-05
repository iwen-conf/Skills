---
name: "arc:model"
description: "需要基于项目实际代码与配置输出 UML 图时使用：按适用性产出 14 类 UML 图；如需 E-R 图，必须使用陈氏画法。"
---

# arc:model — 项目 UML 图谱生成

## Overview

`arc:model` 用于从真实项目证据（代码、配置、接口、部署与业务流程）生成 UML 图谱。  
本技能强调“按实际情况选择图型”，不是机械地把 14 类图全部画一遍。
所有 UML 图必须符合 UML 标准记法（建议对齐 UML 2.5.1）；若项目需要 E-R 图，必须使用**陈氏画法**。

## Quick Contract

- **Trigger**：需要建立可追溯的系统建模图谱，辅助沟通、评审、交接或架构演进。
- **Inputs**：项目路径、关注图型、业务场景、部署环境、输出格式。
- **Outputs**：图型适用性矩阵、UML 图文件（固定 Mermaid `.mmd`）、图目录与证据映射；如适用则额外输出陈氏 E-R 图。
- **Quality Gate**：交付前必须通过 `## Quality Gates` 的证据一致性与图间一致性检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:model`，先做图型适用性判定，再输出基于证据的 UML 图谱。”

## The Iron Law

```
NO DIAGRAM WITHOUT EVIDENCE, NO RELATION WITHOUT TRACEABILITY
NO ER DIAGRAM WITHOUT CHEN NOTATION
```

没有证据不要画图，没有可追溯关系不要连线。

## Workflow

1. 扫描项目上下文（代码结构、配置、API、业务流程、部署信息）。
2. 输出 14 图型适用性矩阵（产出/暂缓/不适用 + 理由 + 证据）。
3. 生成适用图型的 UML 文件与索引目录（需要数据建模时追加陈氏 E-R 图）。
4. 执行图间一致性校验并输出交付说明。

## Quality Gates

- 每张图必须给出对应证据（`file:line`、配置路径或接口定义）。
- “不适用”图型必须给出明确理由，不得留空。
- 不同图之间的核心实体命名必须一致（模块名、服务名、领域对象名）。
- 部署、组件、配置三类图的关系必须互相可映射。
- UML 图的语义与记法必须符合标准（关系类型、可见性、生命周期语义不可混用）。
- 如输出 E-R 图，必须使用陈氏画法（实体矩形、联系菱形、属性椭圆、多值双椭圆、弱实体双矩形）。

## Red Flags

- 不看项目证据就直接套模板画图。
- 14 图型全部强制产出，忽略项目实际规模与阶段。
- E-R 图使用乌鸦脚或类图符号冒充陈氏画法。
- 只有图没有文字说明，无法解释建模依据。
- 图与代码命名严重不一致，无法用于交接。

## Context Budget（避免 Request too large）

- 只抽取关键目录、关键配置、关键流程，不粘贴整仓代码。
- 每张图的证据片段控制在 5-20 行关键片段。
- 复杂系统分域输出，避免单图过载。

## When to Use

- **首选触发**：需要系统化 UML 图谱来说明架构、交互、部署或业务流程。
- **典型场景**：新成员 onboarding、架构评审、技术尽调、发布前知识沉淀、跨团队对齐。
- **边界提示**：只需仓库结构先用 `arc:cartography`；只需质量诊断先用 `arc:audit`。

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 项目根目录绝对路径 |
| `project_name` | string | 否 | 项目标识；默认从路径推导 |
| `diagram_types` | array | 否 | 指定图型列表；默认按适用性自动选择 |
| `business_scenarios` | array | 否 | 关键业务场景（用于用例/活动/时序建模） |
| `deployment_targets` | array | 否 | 部署目标（如 k8s/ecs/vm/on-prem） |
| `render_format` | string | 否 | 固定为 `mermaid`（强制） |
| `include_er` | boolean | 否 | 是否输出 E-R 图（输出时必须为陈氏画法），默认自动判定 |
| `depth_level` | string | 否 | `quick` / `standard` / `deep`，默认 `standard` |
| `output_dir` | string | 否 | 默认 `<project_path>/.arc/uml/` |

## Diagram Catalog（14 类）

1. 类图（class）
2. 对象图（object）
3. 组件图（component）
4. 部署图（deployment）
5. 包图（package）
6. 复合结构图（composite-structure）
7. 配置文件图（configuration，扩展视图）
8. 用例图（use-case）
9. 活动图（activity）
10. 状态机图（state-machine）
11. 序列图（sequence）
12. 通信图（communication）
13. 交互概述图（interaction-overview）
14. 时间图（timing）

> 详见 `references/diagram-catalog.md` 的证据映射与适用条件。

## Notation Standards

- UML 图：遵循 UML 标准语义，并使用 Mermaid 语法表达（建议语义基线 UML 2.5.1）。
- E-R 图：使用 Mermaid 语法表达陈氏画法；禁止使用 Crow's Foot / IDEF1X 代替。
- 判定细则见 `references/notation-standards.md`。

## Dependencies

* **编排契约**: 必须。遵循 `docs/orchestration-contract.md`，通过运行时适配层执行。
* **ace-tool (MCP)**: 必须。用于扫描代码结构、模块依赖、接口与实现证据。
* **Exa MCP**: 可选。用于补充 UML 建模规范或框架约定。
* **Mermaid**: 必须。所有图必须使用 Mermaid 语法输出。
* **辅助脚本**: `scripts/scaffold_uml_pack.py`，用于初始化图文件骨架。

## Instructions（执行流程）

### Phase 1: 项目建模侦察

1. 扫描项目模块、实体、接口、配置、部署与业务流程证据。
2. 生成 `context/project-snapshot.md` 与初始证据清单。

### Phase 2: 图型适用性判定

1. 对 14 图型逐一判定：`required` / `recommended` / `not-applicable`。
2. 判定是否需要 E-R 图（数据建模场景通常 `required`）。
3. 产出 `diagram-plan.md`，包含图型、判定结果、理由、证据来源。

### Phase 3: 图谱产出

1. 运行脚本初始化骨架（可选）：
   ```bash
   python3 scripts/scaffold_uml_pack.py --output-dir <output_dir> --types all
   ```
2. 如需要 E-R 图，初始化陈氏 E-R 骨架：
   ```bash
   python3 scripts/scaffold_uml_pack.py --output-dir <output_dir> --types all --include-er-chen
   ```
3. 对 `required` 与 `recommended` 图型写入完整关系与注释。
4. 生成 `diagram-index.md` 作为交付目录。

### Phase 4: 一致性校验

1. 检查实体命名一致性（类、组件、服务、节点）。
2. 检查跨图一致性（组件图 ↔ 部署图 ↔ 配置图）。
3. 如有 E-R 图，检查是否严格陈氏画法。
4. 输出 `validation-summary.md` 与后续维护建议。

## Outputs

```text
<project_path>/.arc/uml/<project-name>/
├── context/
│   └── project-snapshot.md
├── diagram-plan.md
├── diagram-index.md
├── validation-summary.md
└── diagrams/
    ├── class.mmd
    ├── object.mmd
    ├── component.mmd
    ├── deployment.mmd
    ├── package.mmd
    ├── composite-structure.mmd
    ├── configuration.mmd
    ├── use-case.mmd
    ├── activity.mmd
    ├── state-machine.mmd
    ├── sequence.mmd
    ├── communication.mmd
    ├── interaction-overview.mmd
    ├── timing.mmd
    └── er-chen.mmd                 # 可选：陈氏 E-R 图
```

## Anti-Patterns

- 画“好看图”但不连到真实代码与配置。
- 把配置文件图当成纯截图，不表达配置之间关系。
- 时序图只写 happy path，忽略异常与超时分支。
- E-R 图未使用陈氏画法符号或基数表达不规范。
- 输出图谱但没有维护策略，导致很快过期。
