#!/usr/bin/env python3
"""
generate_sequence_drawio.py - 将 Mermaid 时序图包装为稳定可显示的 draw.io 文件

用法:
    python3 generate_sequence_drawio.py --input diagrams/sequence.mmd --output diagrams/sequence.drawio
    python3 generate_sequence_drawio.py --input diagrams/sequence.mmd --svg cache/sequence.svg --output diagrams/sequence.drawio
"""

from __future__ import annotations

import argparse
import base64
import json
import subprocess
import tempfile
import xml.etree.ElementTree as ET
from pathlib import Path


TITLE_X = 80
TITLE_Y = 40
IMAGE_X = 80
IMAGE_Y = 110
MAX_WIDTH = 1100
PAGE_MARGIN_X = 80
PAGE_MARGIN_BOTTOM = 80


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="将 Mermaid 图表包装为 draw.io 文件")
    parser.add_argument("--input", required=True, help="Mermaid 图表 .mmd 文件")
    parser.add_argument("--output", required=True, help="输出 .drawio 文件路径")
    parser.add_argument("--svg", help="已存在的 SVG 文件；如果不传则自动渲染")
    parser.add_argument("--title", help="图标题；默认从文件名推断")
    return parser.parse_args()


def infer_title(path: Path, title: str | None) -> str:
    if title and title.strip():
        return title.strip()
    stem = path.stem.replace("-", " ").replace("_", " ").strip()
    return stem or "时序图"


def extract_mermaid_source(content: str) -> str:
    stripped = content.strip()
    if not stripped:
        raise RuntimeError("输入文件为空")

    lines = content.splitlines()
    capturing = False
    captured: list[str] = []

    for line in lines:
        trimmed = line.strip()
        # Generic mermaid starting block or just raw code
        if trimmed.startswith("```mermaid"):
            capturing = True
            continue
        elif trimmed.startswith("```") and capturing:
            break
        
        if capturing:
            captured.append(line.rstrip())
            
    if not captured:
        # Fallback: assume the whole file is Mermaid
        return content.strip() + "\n"

    source = "\n".join(captured).strip()
    return source + "\n"


def render_svg(mmd_path: Path) -> Path:
    with tempfile.TemporaryDirectory(prefix="sequence-drawio-") as temp_dir:
        temp_dir_path = Path(temp_dir)
        temp_svg = temp_dir_path / f"{mmd_path.stem}.svg"
        temp_mmd = temp_dir_path / f"{mmd_path.stem}.mmd"
        temp_mmd.write_text(extract_mermaid_source(mmd_path.read_text(encoding="utf-8")), encoding="utf-8")
        script_path = Path(__file__).with_name("render_beautiful_mermaid_svg.mjs")
        result = subprocess.run(
            [
                "node",
                str(script_path),
                "--input",
                str(temp_mmd),
                "--output",
                str(temp_svg),
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(result.stderr.strip() or result.stdout.strip() or "Mermaid 渲染失败")

        persisted = mmd_path.parent / f"{mmd_path.name}.svg"
        persisted.write_text(temp_svg.read_text(encoding="utf-8"), encoding="utf-8")
        return persisted


def svg_data_uri(svg_content: str) -> str:
    payload = base64.b64encode(svg_content.encode("utf-8")).decode("ascii")
    return f"data:image/svg+xml;base64,{payload}"


def parse_svg_size(svg_path: Path) -> tuple[int, int]:
    root = ET.fromstring(svg_path.read_text(encoding="utf-8"))
    width = parse_numeric(root.get("width"))
    height = parse_numeric(root.get("height"))

    if width and height:
        return width, height

    view_box = root.get("viewBox")
    if view_box:
        parts = view_box.split()
        if len(parts) == 4:
            view_width = parse_numeric(parts[2])
            view_height = parse_numeric(parts[3])
            if view_width and view_height:
                return view_width, view_height

    return 960, 640


def parse_numeric(value: str | None) -> int:
    if not value:
        return 0
    cleaned = value.strip().replace("px", "")
    try:
        return int(float(cleaned))
    except ValueError:
        return 0


def build_xml(title: str, mmd_source: str, svg_path: Path) -> ET.ElementTree:
    svg_content = svg_path.read_text(encoding="utf-8")
    original_width, original_height = parse_svg_size(svg_path)
    if original_width <= 0 or original_height <= 0:
        original_width, original_height = 960, 640

    scale = min(1.0, MAX_WIDTH / original_width)
    width = int(original_width * scale)
    height = int(original_height * scale)
    page_width = max(width + PAGE_MARGIN_X * 2, 1200)
    page_height = max(IMAGE_Y + height + PAGE_MARGIN_BOTTOM, 720)

    mermaid_payload = json.dumps({"data": mmd_source, "config": {}}, ensure_ascii=False, indent=2)

    mxfile = ET.Element(
        "mxfile",
        {
            "host": "app.diagrams.net",
            "modified": "2026-04-15T00:00:00.000Z",
            "agent": "Codex",
            "version": "24.7.17",
        },
    )
    diagram = ET.SubElement(mxfile, "diagram", {"id": "sequence", "name": title})
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
            "pageWidth": str(page_width),
            "pageHeight": str(page_height),
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
            "value": title,
            "style": "text;html=1;strokeColor=none;fillColor=none;align=left;verticalAlign=middle;fontSize=24;fontStyle=1;",
            "vertex": "1",
            "parent": "1",
        },
    )
    ET.SubElement(
        title_cell,
        "mxGeometry",
        {"x": str(TITLE_X), "y": str(TITLE_Y), "width": "900", "height": "40", "as": "geometry"},
    )

    image_cell = ET.SubElement(
        root,
        "mxCell",
        {
            "id": "sequence-image",
            "value": "",
            "style": f"shape=image;noLabel=1;verticalAlign=top;imageAspect=1;image={svg_data_uri(svg_content)};",
            "vertex": "1",
            "parent": "1",
            "mermaidData": mermaid_payload,
        },
    )
    ET.SubElement(
        image_cell,
        "mxGeometry",
        {"x": str(IMAGE_X), "y": str(IMAGE_Y), "width": str(width), "height": str(height), "as": "geometry"},
    )

    ET.indent(mxfile, space="  ")
    return ET.ElementTree(mxfile)


def main() -> int:
    args = parse_args()
    input_path = Path(args.input).resolve()
    output_path = Path(args.output).resolve()

    if not input_path.exists():
        raise SystemExit(f"输入文件不存在: {input_path}")

    title = infer_title(input_path, args.title)
    mmd_source = extract_mermaid_source(input_path.read_text(encoding="utf-8"))

    if args.svg:
        svg_path = Path(args.svg).resolve()
        if not svg_path.exists():
            raise SystemExit(f"SVG 文件不存在: {svg_path}")
    else:
        try:
            svg_path = render_svg(input_path)
        except RuntimeError as exc:
            raise SystemExit(str(exc)) from exc

    output_path.parent.mkdir(parents=True, exist_ok=True)
    tree = build_xml(title, mmd_source, svg_path)
    tree.write(output_path, encoding="utf-8", xml_declaration=True)

    print(f"✓ 已生成 Mermaid drawio 文件: {output_path}")
    print(f"  - source: {input_path}")
    print(f"  - svg: {svg_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
