#!/usr/bin/env python3
"""
scaffold_uml_pack.py - 初始化 arc:uml 的图交付骨架

用法：
    python3 scaffold_uml_pack.py --output-dir .arc/uml/demo --types all
    python3 scaffold_uml_pack.py --output-dir .arc/uml/demo --types class,sequence,deployment
    python3 scaffold_uml_pack.py --output-dir .arc/uml/demo --types all --include-er-chen
"""

from __future__ import annotations

import argparse
import html
from pathlib import Path

SEQUENCE_MMD_TEMPLATE = """sequenceDiagram
    participant A as User
    participant B as Controller
    participant C as Service
    participant D as Repository

    A->>B: request()
    activate B
    B->>C: process()
    activate C
    C->>D: query()
    activate D
    alt query success
        D-->>C: return data
    else query failed
        D-->>C: return error
    end
    deactivate D
    C-->>B: return result
    deactivate C
    B-->>A: return response
    deactivate B
"""

def write_sequence_mmd_file(diagrams_dir: Path) -> Path:
    output_file = diagrams_dir / "sequence.mmd"
    content = SEQUENCE_MMD_TEMPLATE
    output_file.write_text(content, encoding="utf-8")
    return output_file



DIAGRAMS = {
    "class": "类图",
    "object": "对象图",
    "component": "组件图",
    "deployment": "部署图",
    "package": "包图",
    "composite-structure": "复合结构图",
    "configuration": "配置文件图",
    "use-case": "用例图",
    "activity": "活动图",
    "state-machine": "状态机图",
    "sequence": "时序图",
    "communication": "通信图",
    "interaction-overview": "交互概述图",
    "timing": "时间图",
}


DRAWING_HINTS = {
    "class": [
        "类名/属性/操作三段式",
        "区分关联、依赖、聚合、组合、泛化",
        "只放关键属性和关键操作",
    ],
    "object": [
        "突出实例名与实例值",
        "用于说明某一时刻对象关系",
    ],
    "component": [
        "强调逻辑模块边界与接口",
        "不要与物理部署节点混画",
    ],
    "deployment": [
        "强调运行节点、容器、中间件与连接",
        "不要与逻辑组件职责混画",
        "先按网络区/运行层分区，再摆节点，不要散点排布",
        "同层节点贴 10px 网格对齐，保持主链路方向一致",
        "连接线必须用 source/target 挂到节点，不要用漂浮端点硬拉线",
        "连接样式优先正交路由，减少斜线和交叉线",
    ],
    "package": [
        "突出命名空间、模块分组与依赖方向",
    ],
    "composite-structure": [
        "突出组件内部端口、部件与连接器",
    ],
    "configuration": [
        "标明配置来源、覆盖顺序和依赖",
    ],
    "use-case": [
        "参与者在系统边界外，用例在系统边界内",
        "include/extend 不能混用",
    ],
    "activity": [
        "必须有起点，至少有一条结束路径",
        "判定/合并用菱形，并发用 fork/join 粗横条",
        "跨角色流程优先补泳道",
    ],
    "state-machine": [
        "一张图只描述一个对象的生命周期",
        "区分状态与动作",
    ],
    "sequence": [
        "**必须使用 Mermaid (.mmd) 格式，严禁原生 drawio XML**",
        "时间自上而下，参与者顺序稳定",
        "生命线按调用层级从左到右排列：前端→Controller→Service→外部服务",
        "单图消息数控制在15个以内，复杂流程拆分或使用ref引用",
        "同步调用使用 activate/deactivate 显示激活范围",
        "使用 loop/alt/opt 结构化片段组织分支",
        "两种不同情况必须放进同一个 alt 矩形框，不要直接拿两条箭头冒充分支",
        "补关键分支、异常、超时或重试",
        "如必须承载在 drawio 中，只允许内嵌 Mermaid 文本节点，不允许原生手工拉消息线",
    ],
    "communication": [
        "强调对象连接关系与消息编号",
    ],
    "interaction-overview": [
        "聚焦高层交互编排，不要塞太多实现细节",
    ],
    "timing": [
        "突出时间轴、状态变化和时间约束",
    ],
}


ER_HINTS = [
    "必须使用陈氏画法",
    "实体用矩形，联系用菱形，属性用椭圆",
    "主键以下划线表示，基数标在联系边上",
    "严禁把外键、数据类型、长度等物理信息画入概念模型",
]


def parse_types(value: str) -> list[str]:
    if value.strip().lower() == "all":
        return list(DIAGRAMS.keys())
    selected = [item.strip() for item in value.split(",") if item.strip()]
    invalid = [item for item in selected if item not in DIAGRAMS]
    if invalid:
        raise ValueError(f"未知图型: {', '.join(invalid)}")
    return selected


def drawio_escape(value: str) -> str:
    return html.escape(value, quote=True).replace("\n", "&#xa;")


def build_note_value(diagram_id: str, label: str) -> str:
    hints = DRAWING_HINTS[diagram_id]
    lines = [
        f"{label}（{diagram_id}）",
        "证据来源：file:line / config:path / api:route / db:table / req:section",
        "FILL_IN：补充本图的对象、关系、约束和范围边界",
        "必画项：",
    ]
    lines.extend(f"- {hint}" for hint in hints)
    lines.append("禁止项：不要脱离证据凭空作图")
    return "\n".join(lines)


def build_er_note_value() -> str:
    lines = [
        "E-R 图（Chen Notation）",
        "证据来源：db:table / schema.sql / migration / data-dictionary",
        "FILL_IN：补充实体、联系、关键属性与基数",
        "必画项：",
    ]
    lines.extend(f"- {hint}" for hint in ER_HINTS)
    lines.append("禁止项：不要把外键字段当作属性椭圆直接画在实体上")
    return "\n".join(lines)


def render_drawio_xml(page_name: str, note_value: str, diagram_id: str) -> str:
    escaped_page_name = html.escape(page_name, quote=True)
    escaped_note = drawio_escape(note_value)
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-10T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="{diagram_id}" name="{escaped_page_name}">
    <mxGraphModel dx="1420" dy="900" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="note" value="{escaped_note}" style="rounded=1;whiteSpace=wrap;html=1;align=left;verticalAlign=top;spacing=12;fillColor=#fff2cc;strokeColor=#d6b656;fontSize=14;" vertex="1" parent="1">
          <mxGeometry x="80" y="80" width="720" height="240" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
"""


def render_brief_template(diagram_id: str, label: str) -> str:
    hints = "\n".join(f"- {hint}" for hint in DRAWING_HINTS[diagram_id])
    output_lines = [f"- 输出文件：`diagrams/{diagram_id}.drawio`"]
    
    sequence_warning = ""
    if diagram_id == "sequence":
        output_lines = [
            "- 输出文件：`diagrams/sequence.mmd`",
            "- 如明确要求 drawio 承载：`diagrams/sequence.drawio`（仅允许内嵌 Mermaid）",
        ]
        sequence_warning = "\n> **[致命排版预警]** 时序图必须使用内嵌 Mermaid 节点（例如 `style=\"shape=mermaid;\"`）或纯文本 Mermaid（.mmd），严禁由大模型硬算原生连线坐标（极易造成左侧连线堆叠）。\n"

    deployment_warning = ""
    if diagram_id == "deployment":
        deployment_warning = "\n> **[连线错位预警]** 部署图必须先确定分区和节点对齐规则，再画连接线。已知端点的连线必须绑定 `source/target`，优先使用正交路由，禁止用漂浮端点补坐标。\n"

    base = f"""# {label} 建模简报{sequence_warning}{deployment_warning}

## 图目标

- FILL_IN：这张图要解释什么
- FILL_IN：这张图不解释什么

## 证据来源

- file:line
- config:path
- api:route
- req:section

## 必画元素

{hints}

## 命名约束

- 与代码实体、配置名、业务术语保持一致

## 禁画项

- 不要脱离证据凭空连线
- 不要把其他图型的语义混入本图

## 调用 drawio skill 时必须说明

{chr(10).join(output_lines)}
- 布局目标：FILL_IN
- 导出需求：FILL_IN（none / svg / png / pdf / jpg）

## 推荐生成路径

"""

    if diagram_id == "deployment":
        return base + """- 先补 `diagram-specs/deployment.seed.txt` 或 `diagram-specs/deployment.json`
- 如使用 seed 文本，先运行：
  ```bash
  python3 Arc/arc:uml/scripts/draft_deployment_spec.py \
    --input diagram-specs/deployment.seed.txt \
    --output diagram-specs/deployment.json
  ```
- 再运行：
  ```bash
  python3 Arc/arc:uml/scripts/generate_deployment_drawio.py \
    --spec diagram-specs/deployment.json \
    --output diagrams/deployment.drawio
  ```
"""

    if diagram_id == "sequence":
        return base + """- 默认维护 `diagrams/sequence.mmd`
- 如需 SVG 成品图，再运行：
  ```bash
  node Arc/arc:uml/scripts/render_beautiful_mermaid_svg.mjs \
    --input diagrams/sequence.mmd \
    --output diagrams/sequence.mmd.svg
  ```
- 如需稳定的 drawio 承载文件，再运行：
  ```bash
  python3 Arc/arc:uml/scripts/generate_sequence_drawio.py \
    --input diagrams/sequence.mmd \
    --output diagrams/sequence.drawio
  ```
"""

    return base + """- 按简报补足证据后，再调用 `drawio` skill 生成正式图
"""


def render_er_brief_template() -> str:
    hints = "\n".join(f"- {hint}" for hint in ER_HINTS)
    return f"""# E-R 图（陈氏画法）建模简报

## 图目标

- FILL_IN：本图要表达的核心领域数据关系

## 证据来源

- schema.sql
- migration files
- table definitions
- data dictionary

## 必画元素

{hints}

## 禁画项

- 不要把外键字段直接画成属性椭圆
- 不要把数据类型、长度、索引、约束等物理信息画进图里

## 调用 drawio skill 时必须说明

- 输出文件：`diagrams/er-chen.drawio`
- 目标版式：实体矩形、联系菱形、属性椭圆，布局整齐可检查
- 导出需求：FILL_IN（none / svg / png / pdf / jpg）
"""


def write_diagram_file(diagrams_dir: Path, diagram_id: str) -> Path:
    label = DIAGRAMS[diagram_id]
    output_file = diagrams_dir / f"{diagram_id}.drawio"
    output_file.write_text(
        render_drawio_xml(label, build_note_value(diagram_id, label), diagram_id),
        encoding="utf-8",
    )
    return output_file


def write_brief_file(briefs_dir: Path, diagram_id: str) -> Path:
    label = DIAGRAMS[diagram_id]
    output_file = briefs_dir / f"{diagram_id}.md"
    output_file.write_text(render_brief_template(diagram_id, label), encoding="utf-8")
    return output_file


def build_deployment_spec_stub() -> str:
    return """{
  "title": "FILL_IN：部署图标题",
  "zones": [
    {
      "id": "client",
      "label": "客户端区",
      "nodes": [
        {
          "id": "frontend",
          "label": "FILL_IN：客户端入口",
          "kind": "client",
          "subtitle": "FILL_IN：如 Web / App"
        }
      ]
    },
    {
      "id": "app",
      "label": "应用层",
      "nodes": [
        {
          "id": "service-a",
          "label": "FILL_IN：核心服务",
          "kind": "service"
        }
      ]
    },
    {
      "id": "data",
      "label": "数据层",
      "nodes": [
        {
          "id": "db",
          "label": "FILL_IN：数据库",
          "kind": "database"
        }
      ]
    }
  ],
  "edges": [
    {
      "source": "frontend",
      "target": "service-a",
      "label": "FILL_IN：主入口连接"
    },
    {
      "source": "service-a",
      "target": "db",
      "label": "FILL_IN：数据访问"
    }
  ]
}
"""


def build_deployment_seed_stub() -> str:
    return """title: FILL_IN：部署图标题

zone client | 客户端区
node frontend | FILL_IN：客户端入口 | client | FILL_IN：如 Web / App

zone app | 应用层
node service-a | FILL_IN：核心服务 | service

zone data | 数据层
node db | FILL_IN：数据库 | database

edge frontend -> service-a | FILL_IN：主入口连接
edge service-a -> db | FILL_IN：数据访问
"""


def write_project_snapshot_stub(context_dir: Path) -> Path:
    output_file = context_dir / "project-snapshot.md"
    output_file.write_text(
        """# 项目快照

## 当前任务

- FILL_IN：本次建模目标

## 关键证据

- [ ] 代码结构
- [ ] 配置与环境
- [ ] 接口与消息流
- [ ] 数据库表设计
- [ ] 业务流程

## 建模边界

- FILL_IN：本轮纳入的范围
- FILL_IN：本轮明确不纳入的范围
""",
        encoding="utf-8",
    )
    return output_file


def write_plan_stub(base_dir: Path, selected: list[str], include_er: bool) -> Path:
    lines = [
        "# 图型适用性计划",
        "",
        "| 图型 | 结论 | 证据 | 备注 |",
        "|---|---|---|---|",
    ]
    for diagram_id in selected:
        lines.append(f"| {DIAGRAMS[diagram_id]} | pending | FILL_IN | FILL_IN |")
    if include_er:
        lines.append("| E-R 图（陈氏） | pending | FILL_IN | FILL_IN |")
    lines.append("")
    output_file = base_dir / "diagram-plan.md"
    output_file.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return output_file


def write_index_file(base_dir: Path, selected: list[str], include_er: bool) -> Path:
    lines = [
        "# UML 图谱索引",
        "",
        "| 图型 | 简报 | 图文件 | 状态 |",
        "|---|---|---|---|",
    ]
    for diagram_id in selected:
        label = DIAGRAMS[diagram_id]
        if diagram_id == "sequence":
            lines.append(
                f"| {label} | `diagram-briefs/{diagram_id}.md` | `diagrams/{diagram_id}.mmd` | scaffolded |"
            )
        else:
            lines.append(
                f"| {label} | `diagram-briefs/{diagram_id}.md` | `diagrams/{diagram_id}.drawio` | scaffolded |"
            )
    if include_er:
        lines.append(
            "| E-R 图（陈氏） | `diagram-briefs/er-chen.md` | `diagrams/er-chen.drawio` | scaffolded |"
        )
    lines.append("")
    output_file = base_dir / "diagram-index.md"
    output_file.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return output_file


def write_validation_stub(base_dir: Path) -> Path:
    output_file = base_dir / "validation-summary.md"
    output_file.write_text(
        """# 一致性校验摘要

## 自动化验证结果

### 验证脚本执行

每张图生成后必须执行：
```bash
python3 Arc/arc:uml/scripts/validate_diagram.py <diagram-file> --type <type>
```

整包生成完成后必须再执行一次多轮复核：
```bash
python3 Arc/arc:uml/scripts/review_uml_pack.py --uml-dir <output-dir>
```

### 验证结果记录

| 图文件 | 类型 | 结果 | 错误数 | 警告数 |
|--------|------|------|--------|--------|
| FILL_IN | sequence/activity/class/deployment | pass/warning/error | 0 | 0 |

### 阻断性问题（必须修复）

- [ ] FILL_IN：时序图消息数超过20个，需拆分
- [ ] FILL_IN：缺少激活条，需补充
- [ ] FILL_IN：部署图存在漂浮连接线，需改为节点挂接
- [ ] FILL_IN：时序图把两种不同情况直接画成箭头，需改为 alt 矩形片段
- [ ] FILL_IN：复核脚本发现源文件与派生产物不一致

### 警告事项（建议修复）

- [ ] FILL_IN：线条交叉较多，建议调整生命线排序
- [ ] FILL_IN：消息标签为空，建议补充描述
- [ ] FILL_IN：部署图节点未对齐到统一网格，建议整理版式

## 命名一致性

- FILL_IN

## 跨图一致性

- FILL_IN

## 特殊检查

- [ ] 活动图的控制流是否完整
- [ ] 用例图的系统边界是否清楚
- [ ] 时序图是否补足关键异常/分支
- [ ] 时序图两种不同情况是否已放进 alt/opt 结构化矩形片段
- [ ] 类图关系是否未混用
- [ ] E-R 图是否严格符合陈氏画法

## 回归验证命令

```bash
# 重新验证所有图
for f in diagrams/*.drawio diagrams/*.mmd; do
    python3 Arc/arc:uml/scripts/validate_diagram.py "$f"
done

# 对整个交付目录做多轮复核
python3 Arc/arc:uml/scripts/review_uml_pack.py --uml-dir .
```
""",
        encoding="utf-8",
    )
    return output_file


def write_er_diagram_file(diagrams_dir: Path) -> Path:
    output_file = diagrams_dir / "er-chen.drawio"
    output_file.write_text(
        render_drawio_xml("E-R 图（陈氏）", build_er_note_value(), "er-chen"),
        encoding="utf-8",
    )
    return output_file


def write_er_brief_file(briefs_dir: Path) -> Path:
    output_file = briefs_dir / "er-chen.md"
    output_file.write_text(render_er_brief_template(), encoding="utf-8")
    return output_file


def write_deployment_spec_stub(specs_dir: Path) -> Path:
    output_file = specs_dir / "deployment.json"
    output_file.write_text(build_deployment_spec_stub(), encoding="utf-8")
    return output_file


def write_deployment_seed_stub(specs_dir: Path) -> Path:
    output_file = specs_dir / "deployment.seed.txt"
    output_file.write_text(build_deployment_seed_stub(), encoding="utf-8")
    return output_file


def main() -> int:
    parser = argparse.ArgumentParser(description="初始化 arc:uml 的图交付骨架")
    parser.add_argument("--output-dir", required=True, help="输出目录")
    parser.add_argument(
        "--types",
        default="all",
        help="图型列表（逗号分隔）或 all。可选值: " + ", ".join(DIAGRAMS.keys()),
    )
    parser.add_argument(
        "--include-er-chen",
        action="store_true",
        help="同时生成陈氏画法 E-R 图骨架（er-chen.drawio）",
    )
    args = parser.parse_args()

    output_dir = Path(args.output_dir).resolve()
    diagrams_dir = output_dir / "diagrams"
    briefs_dir = output_dir / "diagram-briefs"
    specs_dir = output_dir / "diagram-specs"
    context_dir = output_dir / "context"

    diagrams_dir.mkdir(parents=True, exist_ok=True)
    briefs_dir.mkdir(parents=True, exist_ok=True)
    specs_dir.mkdir(parents=True, exist_ok=True)
    context_dir.mkdir(parents=True, exist_ok=True)

    try:
        selected = parse_types(args.types)
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc

    created_files = []
    for diagram_id in selected:
        # 时序图特殊处理：生成 .mmd 文件而不是 .drawio
        if diagram_id == "sequence":
            created_files.append(write_sequence_mmd_file(diagrams_dir))
        else:
            created_files.append(write_diagram_file(diagrams_dir, diagram_id))
        if diagram_id == "deployment":
            created_files.append(write_deployment_seed_stub(specs_dir))
            created_files.append(write_deployment_spec_stub(specs_dir))
        created_files.append(write_brief_file(briefs_dir, diagram_id))

    created_files.append(write_project_snapshot_stub(context_dir))
    created_files.append(write_plan_stub(output_dir, selected, args.include_er_chen))
    created_files.append(write_index_file(output_dir, selected, args.include_er_chen))
    created_files.append(write_validation_stub(output_dir))

    if args.include_er_chen:
        created_files.append(write_er_diagram_file(diagrams_dir))
        created_files.append(write_er_brief_file(briefs_dir))

    print(f"✓ 已初始化 arc:uml 图交付骨架: {output_dir}")
    print(f"  - 创建文件数量: {len(created_files)}")
    print(f"  - 图文件目录: {diagrams_dir}")
    print(f"  - 建模简报目录: {briefs_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
