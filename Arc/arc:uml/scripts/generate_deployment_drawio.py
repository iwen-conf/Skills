#!/usr/bin/env python3
"""
generate_deployment_drawio.py - 根据结构化规格生成对齐良好的部署图 draw.io XML

用法:
    python3 generate_deployment_drawio.py --spec deployment.json --output deployment.drawio
"""

from __future__ import annotations

import argparse
import html
import json
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List


PAGE_MARGIN_X = 80
TITLE_X = 80
TITLE_Y = 40
ZONE_Y = 120
ZONE_WIDTH = 260
ZONE_GAP = 60
ZONE_HEADER_HEIGHT = 38
ZONE_PADDING_X = 22
ZONE_PADDING_TOP = 24
ZONE_PADDING_BOTTOM = 28
NODE_GAP = 22
MIN_ZONE_HEIGHT = 300
DEFAULT_NODE_HEIGHT = 74
SUBTITLE_EXTRA_HEIGHT = 18

ZONE_COLORS = [
    ("#f6efe3", "#c8923d", "#5f3d12"),
    ("#ebe5f8", "#7f5dbb", "#3f2b67"),
    ("#e3eef9", "#4d7cc1", "#1f3f6d"),
    ("#e7f5eb", "#4f9c63", "#1d5b34"),
]

NODE_KIND_STYLES = {
    "client": "rounded=1;arcSize=16;whiteSpace=wrap;html=1;fillColor=#fff4d6;strokeColor=#d4a843;fontSize=14;fontStyle=1;spacing=10;",
    "gateway": "rounded=1;arcSize=16;whiteSpace=wrap;html=1;fillColor=#e9f0ff;strokeColor=#5f84c9;fontSize=14;fontStyle=1;spacing=10;",
    "service": "rounded=1;arcSize=16;whiteSpace=wrap;html=1;fillColor=#efe8fb;strokeColor=#8662c7;fontSize=14;fontStyle=1;spacing=10;",
    "runtime": "rounded=1;arcSize=16;whiteSpace=wrap;html=1;fillColor=#edf3f8;strokeColor=#6f879d;fontSize=14;fontStyle=1;spacing=10;",
    "queue": "shape=hexagon;perimeter=hexagonPerimeter2;whiteSpace=wrap;html=1;fillColor=#eef7ff;strokeColor=#5f84c9;fontSize=13;fontStyle=1;spacing=10;",
    "database": "shape=cylinder3;whiteSpace=wrap;html=1;boundedLbl=1;backgroundOutline=1;size=15;fillColor=#e7f7eb;strokeColor=#4f9c63;fontSize=14;fontStyle=1;spacing=10;",
    "external": "shape=cloud;whiteSpace=wrap;html=1;fillColor=#fff0ef;strokeColor=#d07063;fontSize=13;fontStyle=1;spacing=10;",
}


@dataclass
class NodeSpec:
    id: str
    label: str
    kind: str = "service"
    subtitle: str = ""
    width: int = 0
    height: int = 0
    x: int = 0
    y: int = 0


@dataclass
class ZoneSpec:
    id: str
    label: str
    nodes: List[NodeSpec] = field(default_factory=list)
    x: int = 0
    y: int = 0
    width: int = ZONE_WIDTH
    height: int = MIN_ZONE_HEIGHT
    fill_color: str = ""
    stroke_color: str = ""
    font_color: str = ""


@dataclass
class EdgeSpec:
    source: str
    target: str
    label: str = ""


@dataclass
class DeploymentSpec:
    title: str
    zones: List[ZoneSpec]
    edges: List[EdgeSpec]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="根据结构化规格生成部署图 draw.io XML")
    parser.add_argument("--spec", required=True, help="结构化规格 JSON 文件")
    parser.add_argument("--output", required=True, help="输出 .drawio 文件路径")
    return parser.parse_args()


def read_spec(path: Path) -> DeploymentSpec:
    payload = json.loads(path.read_text(encoding="utf-8"))
    title = str(payload.get("title", "")).strip() or "部署图"
    raw_zones = payload.get("zones")
    raw_edges = payload.get("edges", [])

    if not isinstance(raw_zones, list) or not raw_zones:
        raise ValueError("spec.zones 必须是非空数组")

    zone_ids: set[str] = set()
    node_ids: set[str] = set()
    zones: List[ZoneSpec] = []

    for index, raw_zone in enumerate(raw_zones):
        zone_id = str(raw_zone.get("id", "")).strip()
        label = str(raw_zone.get("label", "")).strip()
        raw_nodes = raw_zone.get("nodes", [])

        if not zone_id:
            raise ValueError(f"第 {index + 1} 个 zone 缺少 id")
        if zone_id in zone_ids:
            raise ValueError(f"zone id 重复: {zone_id}")
        if not label:
            raise ValueError(f"zone {zone_id} 缺少 label")
        if not isinstance(raw_nodes, list) or not raw_nodes:
            raise ValueError(f"zone {zone_id} 必须至少包含一个 node")

        zone_ids.add(zone_id)
        fill, stroke, font = ZONE_COLORS[index % len(ZONE_COLORS)]
        zone = ZoneSpec(id=zone_id, label=label, fill_color=fill, stroke_color=stroke, font_color=font)

        for node_index, raw_node in enumerate(raw_nodes):
            node_id = str(raw_node.get("id", "")).strip()
            node_label = str(raw_node.get("label", "")).strip()
            kind = str(raw_node.get("kind", "service")).strip() or "service"
            subtitle = str(raw_node.get("subtitle", "")).strip()

            if not node_id:
                raise ValueError(f"zone {zone_id} 的第 {node_index + 1} 个 node 缺少 id")
            if node_id in node_ids:
                raise ValueError(f"node id 重复: {node_id}")
            if not node_label:
                raise ValueError(f"node {node_id} 缺少 label")

            node_ids.add(node_id)
            zone.nodes.append(NodeSpec(id=node_id, label=node_label, kind=kind, subtitle=subtitle))

        zones.append(zone)

    edges: List[EdgeSpec] = []
    if not isinstance(raw_edges, list):
        raise ValueError("spec.edges 必须是数组")

    for edge_index, raw_edge in enumerate(raw_edges):
        source = str(raw_edge.get("source", "")).strip()
        target = str(raw_edge.get("target", "")).strip()
        label = str(raw_edge.get("label", "")).strip()

        if not source or not target:
            raise ValueError(f"第 {edge_index + 1} 条 edge 缺少 source 或 target")
        if source not in node_ids:
            raise ValueError(f"edge source 不存在: {source}")
        if target not in node_ids:
            raise ValueError(f"edge target 不存在: {target}")
        edges.append(EdgeSpec(source=source, target=target, label=label))

    return DeploymentSpec(title=title, zones=zones, edges=edges)


def node_height(node: NodeSpec) -> int:
    return DEFAULT_NODE_HEIGHT + (SUBTITLE_EXTRA_HEIGHT if node.subtitle else 0)


def layout(spec: DeploymentSpec) -> Dict[str, str]:
    node_to_zone: Dict[str, str] = {}
    max_zone_height = MIN_ZONE_HEIGHT

    for zone_index, zone in enumerate(spec.zones):
        zone.x = PAGE_MARGIN_X + zone_index * (ZONE_WIDTH + ZONE_GAP)
        zone.y = ZONE_Y
        zone.width = ZONE_WIDTH

        current_y = ZONE_HEADER_HEIGHT + ZONE_PADDING_TOP
        for node in zone.nodes:
            node.width = ZONE_WIDTH - ZONE_PADDING_X * 2
            node.height = node_height(node)
            node.x = ZONE_PADDING_X
            node.y = current_y
            current_y += node.height + NODE_GAP
            node_to_zone[node.id] = zone.id

        used_height = current_y - NODE_GAP + ZONE_PADDING_BOTTOM
        zone.height = max(MIN_ZONE_HEIGHT, used_height)
        max_zone_height = max(max_zone_height, zone.height)

    for zone in spec.zones:
        zone.height = max_zone_height

    page_width = PAGE_MARGIN_X * 2 + len(spec.zones) * ZONE_WIDTH + max(0, len(spec.zones) - 1) * ZONE_GAP
    page_height = ZONE_Y + max_zone_height + 120

    return {
        "page_width": str(max(page_width, 1200)),
        "page_height": str(max(page_height, 720)),
        "node_to_zone": json.dumps(node_to_zone, ensure_ascii=False),
    }


def node_label_html(node: NodeSpec) -> str:
    label = html.escape(node.label, quote=True)
    if not node.subtitle:
        return f"<b>{label}</b>"
    subtitle = html.escape(node.subtitle, quote=True)
    return f"<b>{label}</b><div style=\"font-size:11px;color:#52606d;\">{subtitle}</div>"


def zone_style(zone: ZoneSpec) -> str:
    return (
        "swimlane;horizontal=0;startSize=38;rounded=1;arcSize=12;whiteSpace=wrap;html=1;"
        f"fillColor={zone.fill_color};strokeColor={zone.stroke_color};fontColor={zone.font_color};"
        "fontStyle=1;fontSize=16;swimlaneFillColor=#ffffff;spacingLeft=14;"
    )


def node_style(node: NodeSpec) -> str:
    return NODE_KIND_STYLES.get(node.kind, NODE_KIND_STYLES["service"])


def edge_style(source_zone_index: int, target_zone_index: int, source_node: NodeSpec, target_node: NodeSpec) -> str:
    base = (
        "edgeStyle=orthogonalEdgeStyle;orthogonalLoop=1;jettySize=auto;rounded=0;html=1;"
        "endArrow=block;endFill=1;strokeColor=#566373;strokeWidth=1.6;fontSize=12;"
        "labelBackgroundColor=#ffffff;"
    )

    if source_zone_index < target_zone_index:
        return base + "exitX=1;exitY=0.5;entryX=0;entryY=0.5;exitPerimeter=1;entryPerimeter=1;"
    if source_zone_index > target_zone_index:
        return base + "exitX=0;exitY=0.5;entryX=1;entryY=0.5;exitPerimeter=1;entryPerimeter=1;"
    if source_node.y <= target_node.y:
        return base + "exitX=0.5;exitY=1;entryX=0.5;entryY=0;exitPerimeter=1;entryPerimeter=1;"
    return base + "exitX=0.5;exitY=0;entryX=0.5;entryY=1;exitPerimeter=1;entryPerimeter=1;"


def stable_edge_id(edge: EdgeSpec, index: int) -> str:
    label = edge.label or "edge"
    cleaned = "".join(ch.lower() if ch.isalnum() else "-" for ch in f"{edge.source}-{edge.target}-{label}")
    compact = "-".join(part for part in cleaned.split("-") if part)
    return f"edge-{index + 1}-{compact[:48]}"


def indent_tree(element: ET.Element) -> None:
    ET.indent(element, space="  ")


def build_xml(spec: DeploymentSpec) -> ET.ElementTree:
    layout_info = layout(spec)
    node_to_zone = json.loads(layout_info["node_to_zone"])
    zone_index_by_id = {zone.id: index for index, zone in enumerate(spec.zones)}
    node_lookup = {node.id: node for zone in spec.zones for node in zone.nodes}

    mxfile = ET.Element(
        "mxfile",
        {
            "host": "app.diagrams.net",
            "modified": "2026-04-15T00:00:00.000Z",
            "agent": "Codex",
            "version": "24.7.17",
        },
    )
    diagram = ET.SubElement(mxfile, "diagram", {"id": "deployment", "name": html.escape(spec.title, quote=True)})
    graph = ET.SubElement(
        diagram,
        "mxGraphModel",
        {
            "dx": "1420",
            "dy": "900",
            "grid": "1",
            "gridSize": "10",
            "guides": "1",
            "tooltips": "1",
            "connect": "1",
            "arrows": "1",
            "fold": "1",
            "page": "1",
            "pageScale": "1",
            "pageWidth": layout_info["page_width"],
            "pageHeight": layout_info["page_height"],
            "math": "0",
            "shadow": "0",
        },
    )
    root = ET.SubElement(graph, "root")
    ET.SubElement(root, "mxCell", {"id": "0"})
    ET.SubElement(root, "mxCell", {"id": "1", "parent": "0"})

    title_cell = ET.SubElement(
        root,
        "mxCell",
        {
            "id": "title",
            "value": html.escape(spec.title, quote=True),
            "style": "text;html=1;strokeColor=none;fillColor=none;align=left;verticalAlign=middle;fontSize=24;fontStyle=1;",
            "vertex": "1",
            "parent": "1",
        },
    )
    ET.SubElement(
        title_cell,
        "mxGeometry",
        {"x": str(TITLE_X), "y": str(TITLE_Y), "width": "700", "height": "40", "as": "geometry"},
    )

    for zone in spec.zones:
        zone_cell = ET.SubElement(
            root,
            "mxCell",
            {
                "id": zone.id,
                "value": html.escape(zone.label, quote=True),
                "style": zone_style(zone),
                "vertex": "1",
                "parent": "1",
            },
        )
        ET.SubElement(
            zone_cell,
            "mxGeometry",
            {
                "x": str(zone.x),
                "y": str(zone.y),
                "width": str(zone.width),
                "height": str(zone.height),
                "as": "geometry",
            },
        )

        for node in zone.nodes:
            node_cell = ET.SubElement(
                root,
                "mxCell",
                {
                    "id": node.id,
                    "value": node_label_html(node),
                    "style": node_style(node),
                    "vertex": "1",
                    "parent": zone.id,
                },
            )
            ET.SubElement(
                node_cell,
                "mxGeometry",
                {
                    "x": str(node.x),
                    "y": str(node.y),
                    "width": str(node.width),
                    "height": str(node.height),
                    "as": "geometry",
                },
            )

    for edge_index, edge in enumerate(spec.edges):
        source_zone = node_to_zone[edge.source]
        target_zone = node_to_zone[edge.target]
        source_node = node_lookup[edge.source]
        target_node = node_lookup[edge.target]

        edge_cell = ET.SubElement(
            root,
            "mxCell",
            {
                "id": stable_edge_id(edge, edge_index),
                "value": html.escape(edge.label, quote=True),
                "style": edge_style(
                    zone_index_by_id[source_zone],
                    zone_index_by_id[target_zone],
                    source_node,
                    target_node,
                ),
                "edge": "1",
                "parent": "1",
                "source": edge.source,
                "target": edge.target,
            },
        )
        ET.SubElement(edge_cell, "mxGeometry", {"relative": "1", "as": "geometry"})

    tree = ET.ElementTree(mxfile)
    indent_tree(mxfile)
    return tree


def main() -> int:
    args = parse_args()
    spec_path = Path(args.spec).resolve()
    output_path = Path(args.output).resolve()

    if not spec_path.exists():
        raise SystemExit(f"spec 文件不存在: {spec_path}")

    try:
        spec = read_spec(spec_path)
    except (json.JSONDecodeError, ValueError) as exc:
        raise SystemExit(str(exc)) from exc

    output_path.parent.mkdir(parents=True, exist_ok=True)
    tree = build_xml(spec)
    tree.write(output_path, encoding="utf-8", xml_declaration=True)

    print(f"✓ 已生成部署图: {output_path}")
    print(f"  - zones: {len(spec.zones)}")
    print(f"  - edges: {len(spec.edges)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
