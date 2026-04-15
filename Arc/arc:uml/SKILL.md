---
name: arc:uml
description: "UML 与 Chen E-R 建模：基于代码与文档证据做图型判定，并强制调用 `drawio` skill 生成标准图谱；当用户说“画架构图/UML 建模/sequence diagram/ER 图/activity diagram/drawio”时触发。"
version: 1.0.0
allowed_tools:
  - Bash
  - Read
  - Edit
  - Write
  - Grep
  - Glob
hooks:
  PreToolUse:
    - matcher: Bash
      hooks:
        - type: command
          command: "bash ${ARC_SKILL_DIR}/scripts/check-destructive.sh"
          statusMessage: "Checking for destructive commands..."
---

# arc:uml — 基于证据的 UML / 陈氏 E-R 建模

## Overview

`arc:uml` 负责三件事：

1. 判断当前项目应该画哪些图，不应该画哪些图。
2. 抽取代码、配置、接口、流程和数据库设计中的可追溯证据。
3. 强制调用 `drawio` skill 生成原生 `.drawio` 文件，并在需要时导出 `.drawio.svg`、`.drawio.png`、`.drawio.pdf` 或 `.drawio.jpg`。

从现在开始，`arc:uml` 不再以 Mermaid 作为默认或推荐作图路径（**时序图除外**）。如果当前环境无法使用 `drawio` skill，应当把它视为阻塞项并明确告知用户；除非用户显式要求兼容旧产物，否则不要回退到 Mermaid。

**例外（时序图致命排版预警）**：针对时序图（Sequence Diagram），由于大模型生成原生 `drawio` XML 时极易因无法精确计算连线坐标而导致所有消息箭头在左侧原点重叠堆积（排版完全崩坏），因此时序图**必须**放宽限制：允许并推荐使用 Mermaid 语法（`.mmd`）生成，或在 `drawio` 中使用内嵌的 Mermaid 文本节点（`shape=mermaid`）。切勿让大模型用原生 XML 硬算时序图连线。

如果任务包含 E-R 图，必须使用陈氏画法（Chen Notation），并且数据来源必须是实际的数据库表设计，而不是凭空想象的概念草图。对于活动图、用例图、时序图、类图、部署图等常见图型，默认同时满足：

- `UML 2.5.1 / ISO 19505` 的基本语义要求。
- 中国高校软件工程 / 数据库课程中常见的评图要求。

详细规则按需读取：

- [references/notation-standards.md](./references/notation-standards.md)
- [references/diagram-catalog.md](./references/diagram-catalog.md)
- [references/china-university-diagram-guidelines.md](./references/china-university-diagram-guidelines.md)

## Quick Contract

- **Trigger**: 用户需要画架构图、UML 图、活动图、时序图、类图、部署图、E-R 图，或明确提到 `drawio` / `draw.io` / `.drawio`。
- **Inputs**: 项目路径、业务场景、目标图型、部署环境、是否需要导出格式。
- **Outputs**: 图型适用性判断、证据清单、建模简报、原生 `.drawio` 图文件，以及可选的 `.drawio.svg` / `.drawio.png` / `.drawio.pdf` / `.drawio.jpg` 导出文件。
- **Quality Gate**: 每张图都必须能回溯到真实证据；E-R 图必须严格使用陈氏画法；最终作图必须通过 `drawio` skill 完成。
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

执行时先明确说明：

> 我将先做图型适用性判断和证据抽取，再调用 `drawio` skill 生成符合规范的图。

## Teaming Requirement

- 每次执行都必须先明确 `Owner`、`Executor`、`Reviewer` 三个视角。
- 如果运行环境只有单个 Agent，也要在交付中显式给出这三个角色的结论，形成“决策-执行-复核”的闭环。

## The Iron Law

```text
NO DIAGRAM WITHOUT EVIDENCE
NO RELATION WITHOUT TRACEABILITY
NO FINAL UML DELIVERY WITHOUT DRAWIO
NO ER DIAGRAM WITHOUT CHEN NOTATION
```

## Workflow

1. 扫描项目上下文：代码结构、配置、接口、数据库表设计、部署信息、业务流程。
2. 产出图型适用性矩阵：对 14 类 UML 图逐项判断 `required` / `recommended` / `not-applicable`，并给出理由与证据。
3. 为每张 `required` / `recommended` 图整理建模简报，明确图目标、证据来源、记法约束和禁画项。
4. 调用 `drawio` skill 生成原生 `.drawio` 文件；如果用户要求导出格式，再继续导出。
5. 做跨图一致性检查，输出维护建议和剩余风险。

## Quality Gates

- 每张图都必须给出对应证据：`file:line`、配置路径、接口定义、表结构来源或需求文本片段。
- 所有 `not-applicable` 图型都必须说明原因，不能留空。
- 图中核心对象命名必须与代码、配置或业务术语一致。
- 部署图、组件图、配置图之间必须可以互相印证。
- 活动图必须体现控制流，不能把流程文字直接堆成框图。
- 用例图必须区分系统边界、参与者和用例关系，不能把流程步骤画成用例。
- 时序图必须体现时间顺序，不能把静态依赖关系误画成消息交互。
- 类图必须区分关联、依赖、聚合、组合、泛化，不能混用箭头。
- E-R 图必须严格使用陈氏画法：实体矩形、联系菱形、属性椭圆、主键下划线、必要时使用弱实体/多值属性/全参与等符号。
- 最终可编辑源文件必须是 `.drawio`；如果导出了 `svg/png/pdf`，应优先保留带嵌入 XML 的导出物，或明确说明由 `drawio` skill 自动删除了源 `.drawio` 文件。

## Expert Standards

- UML 图的语义基线默认对齐 `UML 2.5.1 / ISO 19505`。
- 所有正式图必须通过 `drawio` 生成原生源文件；`drawio` 是正式交付路径，不是可选附加项。
- 中国高校课程或毕设场景下，优先保证“元素齐全、关系准确、命名清楚、版式整齐、可一眼检查”的表达方式。
- E-R 图是概念数据模型，不是物理库表说明书。严禁把外键、字段类型、长度、索引等物理实现细节直接画进 Chen 图。
- 活动图在跨角色或跨部门流程中，优先补泳道；并发必须用 fork / join 粗横条表达，不能用菱形冒充并发。
- 用例图中的 `<<include>>` 与 `<<extend>>` 必须按语义使用：前者表示被复用的必经行为，后者表示条件性扩展行为。
- 时序图必须补出关键分支、异常、超时或重试情形，不能只画 happy path。
- 类图只画当前沟通目标所需的关键属性和操作，不要为了“看起来完整”把所有字段一股脑塞进去。
- 每张图都要显式记录建模假设、证据位置和适用边界，避免“看起来对、实际不可用”的图。

## Scripts & Commands

- 初始化 UML 交付目录与 `.drawio` 骨架：
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types class,sequence,deployment
  ```
- 初始化全部 UML 图骨架：
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types all
  ```
- 同时生成陈氏 E-R 图骨架：
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types all --include-er-chen
  ```
- 运行时主命令：
  ```bash
  arc uml
  ```

`drawio` 的 XML 细节、CLI 导出和打开方式，不在本 skill 重复定义，统一遵循 `drawio` skill 本身的约束。

## Red Flags

- 不看项目证据，直接套模板出图。
- 明明只需要 2 到 3 张关键图，却强行输出 14 图全套。
- E-R 图偷换成 Crow's Foot、IDEF1X、类图或数据库表结构图。
- 活动图不画起点、终点、分支或并发控制，只堆步骤框。
- 用例图里把数据库表、接口地址、实现类名当作用例主体。
- 时序图没有返回、异常、条件分支，无法支撑设计评审。
- **时序图画成"面条图"**：消息过多(>20)、生命线无序排列、缺少激活条、线条交叉混乱。
- **跳过自动化验证**：不执行 `validate_diagram.py` 直接交付，劣质图流入产物。
- 仍然把 Mermaid 作为默认交付格式。

## Context Budget

- 只抽取关键目录、关键配置、关键流程与关键表结构，不贴全仓代码。
- 每张图的证据片段控制在 5 到 20 行关键内容。
- 大型系统按域拆图，避免单图过载。

## When to Use

- **Primary Trigger**: 需要用 UML 或 E-R 图辅助架构沟通、交接、评审、论文/课程建模、毕设答辩或系统演进。
- **Typical Scenario**: 新成员入项、架构评审、技术尽调、需求梳理、数据库建模、上线前知识沉淀。
- **Boundary Note**:
  - 如果仓库结构还没摸清，先用 `arc:cartography`。
  - 如果主要诉求是质量审计而非建模，先用 `arc:audit`。
  - 如果用户只要“把图文件导出成 png/pdf/svg”，直接交给 `drawio`。

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | 项目根目录绝对路径 |
| `project_name` | string | no | 项目标识；默认从路径推断 |
| `diagram_types` | array | no | 指定图型列表；默认自动判定 |
| `business_scenarios` | array | no | 关键业务场景，用于活动图/用例图/时序图 |
| `deployment_targets` | array | no | 部署目标，如 `k8s` / `vm` / `on-prem` |
| `include_er` | boolean | no | 是否输出 E-R 图；默认按证据自动判断 |
| `depth_level` | string | no | `quick` / `standard` / `deep`，默认 `standard` |
| `export_format` | string | no | `none` / `svg` / `png` / `pdf` / `jpg`，默认 `none` |
| `render_format` | string | no | 向后兼容参数，等价于 `export_format` |
| `output_dir` | string | no | 默认 `<project_path>/.arc/uml/<project-name>/` |

## Notation Standards

- 所有正式图源文件必须是原生 `draw.io` / `drawio` 文件，即 `.drawio`。
- 任何实际作图动作必须委托给 `drawio` skill 完成。
- 如需交付图片或打印版，优先导出 `.drawio.svg` / `.drawio.png` / `.drawio.pdf`，以便保留嵌入 XML 的可编辑能力。
- E-R 图必须使用陈氏画法，详细规则见 [references/notation-standards.md](./references/notation-standards.md)。
- 中国高校课程和毕业设计常见评图要求见 [references/china-university-diagram-guidelines.md](./references/china-university-diagram-guidelines.md)。

### drawio Skill Contract

在开始绘制每一张图前，至少明确以下信息再调用 `drawio`：

- 图文件路径：如 `diagrams/activity.drawio`
- 图目标：这张图用于解释什么，不用于解释什么
- 证据来源：代码、配置、表结构、需求或日志中的哪些位置
- 必画元素：例如活动图的初始节点 / 判定 / 泳道，或 E-R 图的实体 / 联系 / 主键
- 禁止事项：例如“不要把外键画成属性”“不要把分支条件写到节点名里”
- 导出需求：是否需要 `svg/png/pdf/jpg`

## Dependencies

- **Organization Contract**: 必需。执行时保持 `Owner / Executor / Reviewer` 三角色闭环。
- **ace-tool / 代码检索能力**: 必需。用于收集代码与配置证据。
- **drawio skill**:
  - **类图/活动图/用例图/部署图等**: 必需。所有正式图必须通过它生成。
  - **时序图**: **禁止依赖**。时序图必须使用 Mermaid (`.mmd`) 格式，详见"时序图致命排版预警"。
- **联网搜索（web / Exa）**: 按需。用于补充 UML 规范、教材标准或框架约定。

### 时序图特殊依赖

时序图**不依赖** `drawio` skill，而是依赖：
- **Mermaid 语法生成能力**: 必需。时序图必须使用 `.mmd` 纯文本格式。
- **drawio 内嵌 Mermaid 节点** (可选): 如需要在 `.drawio` 文件中嵌入时序图，使用 `shape=mermaid` 节点包含 Mermaid 源码。

## Instructions (execution process)

### Phase 1: Project Modeling Reconnaissance

1. 扫描项目模块、实体、接口、配置、数据库表设计、部署和关键业务流程。
2. 生成 `context/project-snapshot.md`，沉淀初步证据。

### Phase 2: Determination of Diagram Applicability

1. 用户未指定 `diagram_types` 时，评估全部 14 类 UML 图。
2. 同时判断是否需要补充 E-R 图。
3. 输出 `diagram-plan.md`，记录结论、理由、证据和优先级。

### Phase 3: Modeling Briefs and drawio Output

1. 先初始化交付目录与骨架文件：
   ```bash
   python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <output_dir> --types all --include-er-chen
   ```
2. 为每张 `required` / `recommended` 图补全 `diagram-briefs/<diagram>.md`。
3. 逐张调用 `drawio` skill 生成 `.drawio` 文件。
4. 如果用户要求导出，再继续导出为对应格式。
5. 更新 `diagram-index.md` 记录交付物状态。

### Phase 4: Consistency Verification

1. **自动化质量验证**：对每张生成的 `.drawio` 图调用验证脚本
   ```bash
   python3 Arc/arc:uml/scripts/validate_diagram.py <output_dir>/diagrams/<diagram>.drawio --type <type>
   ```
   - 若验证失败（error），必须修复后重新生成，不得流入交付物
   - 若验证警告（warning），评估是否可在当前迭代修复，或记录到技术债
2. 检查命名一致性：类、组件、服务、节点、角色、实体是否统一。
3. 检查跨图一致性：组件图 ↔ 部署图 ↔ 配置图，活动图 ↔ 时序图 ↔ 用例图。
4. 若输出 E-R 图，复核是否严格符合陈氏画法和概念模型边界。
5. 输出 `validation-summary.md` 与后续维护建议。

## Outputs

```text
<project_path>/.arc/uml/<project-name>/
├── context/
│   └── project-snapshot.md
├── diagram-plan.md
├── diagram-index.md
├── validation-summary.md
├── diagram-briefs/
│   ├── <diagram-type>.md
│   └── er-chen.md
└── diagrams/
    ├── <diagram-type>.drawio       # 类图/活动图/用例图/部署图等
    ├── <diagram-type>.drawio.svg
    ├── <diagram-type>.drawio.png
    ├── sequence.mmd                # 时序图（Mermaid格式）
    ├── sequence.mmd.svg            # 时序图渲染输出
    ├── er-chen.drawio
    ├── er-chen.drawio.svg
    └── ...
```

说明：

- **类图/活动图/用例图/部署图等**：使用 `.drawio` 格式，通过 `drawio` skill 生成。
- **时序图**：**必须使用** `.mmd` (Mermaid) 格式，严禁使用原生 `drawio` XML 生成时序图（避免坐标计算错误导致面条图）。
- **E-R 图**：使用 `.drawio` 格式，必须使用陈氏画法。
- 如果 `drawio` skill 在导出后删除了 `.drawio` 源文件，则以带嵌入 XML 的导出文件作为可编辑主产物，并在 `diagram-index.md` 中说明。
- 并不是所有扩展名都要同时生成，只交付用户明确需要的格式。

## Gotchas

- 画“好看但不可追溯”的图。
- 在一张图里同时混入需求、设计、部署、数据库和测试语义，导致主线模糊。
- 只画 happy path，不画异常、超时、回滚、重试或人工介入。
- 用数据库物理字段去污染概念 E-R 图。
- 明明要的是中国高校课程 / 毕设可检查图，却输出随意草图。
- 没有维护策略，导致图在一次交付后迅速过期。
