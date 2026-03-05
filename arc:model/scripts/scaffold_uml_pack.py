#!/usr/bin/env python3
"""
scaffold_uml_pack.py - 初始化 UML 图谱骨架文件

用法：
    python3 scaffold_uml_pack.py --output-dir .arc/arc:model/demo --types all
    python3 scaffold_uml_pack.py --output-dir .arc/arc:model/demo --types class,sequence,deployment
"""

from __future__ import annotations

import argparse
from pathlib import Path


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
    "sequence": "序列图",
    "communication": "通信图",
    "interaction-overview": "交互概述图",
    "timing": "时间图",
}


MERMAID_KINDS = {
    "class": "classDiagram",
    "object": "flowchart LR",
    "component": "flowchart LR",
    "deployment": "flowchart LR",
    "package": "flowchart LR",
    "composite-structure": "flowchart LR",
    "configuration": "flowchart LR",
    "use-case": "flowchart LR",
    "activity": "flowchart TD",
    "state-machine": "stateDiagram-v2",
    "sequence": "sequenceDiagram",
    "communication": "flowchart LR",
    "interaction-overview": "flowchart TD",
    "timing": "timeline",
}


def parse_types(value: str) -> list[str]:
    if value.strip().lower() == "all":
        return list(DIAGRAMS.keys())
    selected = [item.strip() for item in value.split(",") if item.strip()]
    invalid = [item for item in selected if item not in DIAGRAMS]
    if invalid:
        raise ValueError(f"未知图型: {', '.join(invalid)}")
    return selected


def render_template(diagram_id: str, label: str) -> str:
    kind = MERMAID_KINDS[diagram_id]
    if diagram_id == "timing":
        return f"""%% Mermaid syntax
timeline
    title {label} ({diagram_id})
    %% 证据来源: file:line / config:path / api:route
    %% TODO: 补充时间轴事件与状态变化
"""

    return f"""%% Mermaid syntax
{kind}
    %% {label} ({diagram_id})
    %% 证据来源: file:line / config:path / api:route
    %% TODO: 补充参与者、实体、关系与约束
"""


def render_er_chen_template() -> str:
    return """%% Mermaid syntax
flowchart LR
    %% E-R Diagram (Chen Notation)
    %% 陈氏画法约束：
    %% - 实体: 矩形    例: Customer[客户]
    %% - 联系: 菱形    例: PlacesOrder{下单}
    %% - 属性: 椭圆    例: Name((姓名))
    %% - 主键属性: 属性名加下划线标记（文本约定）
    %% - 多值属性: 双椭圆可用文本标记（如 Phone((电话*)))

    Customer[客户]
    PlacesOrder{下单}
    Name((姓名))

    Customer --- PlacesOrder
    Customer --- Name

    %% TODO: 按 Chen 语义补充实体、联系、属性和基数
"""


def write_diagram_file(diagrams_dir: Path, diagram_id: str) -> Path:
    label = DIAGRAMS[diagram_id]
    output_file = diagrams_dir / f"{diagram_id}.mmd"
    output_file.write_text(render_template(diagram_id, label), encoding="utf-8")
    return output_file


def write_index_file(base_dir: Path, selected: list[str]) -> Path:
    lines = [
        "# UML 图谱索引",
        "",
        "| 图型 | 文件 | 状态 |",
        "|---|---|---|",
    ]
    for diagram_id in selected:
        label = DIAGRAMS[diagram_id]
        lines.append(f"| {label} | `diagrams/{diagram_id}.mmd` | scaffolded |")
    lines.append("")
    content = "\n".join(lines)
    index_path = base_dir / "diagram-index.md"
    index_path.write_text(content, encoding="utf-8")
    return index_path


def append_er_index(index_path: Path) -> None:
    lines = index_path.read_text(encoding="utf-8").splitlines()
    lines.append("| E-R 图（陈氏） | `diagrams/er-chen.mmd` | scaffolded |")
    index_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="初始化 UML 图谱骨架文件")
    parser.add_argument("--output-dir", required=True, help="输出目录")
    parser.add_argument(
        "--types",
        default="all",
        help="图型列表（逗号分隔）或 all。可选值: "
        + ", ".join(DIAGRAMS.keys()),
    )
    parser.add_argument(
        "--include-er-chen",
        action="store_true",
        help="同时生成陈氏画法 E-R 图骨架（er-chen.mmd）",
    )
    args = parser.parse_args()

    output_dir = Path(args.output_dir).resolve()
    diagrams_dir = output_dir / "diagrams"
    diagrams_dir.mkdir(parents=True, exist_ok=True)

    try:
        selected = parse_types(args.types)
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc

    created_files = [write_diagram_file(diagrams_dir, diagram_id) for diagram_id in selected]
    index_path = write_index_file(output_dir, selected)

    if args.include_er_chen:
        er_file = diagrams_dir / "er-chen.mmd"
        er_file.write_text(render_er_chen_template(), encoding="utf-8")
        created_files.append(er_file)
        append_er_index(index_path)

    print(f"✓ 已初始化 UML 图谱骨架: {output_dir}")
    print(f"  - 图文件数量: {len(created_files)}")
    print(f"  - 索引文件: {index_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
