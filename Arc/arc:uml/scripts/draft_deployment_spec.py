#!/usr/bin/env python3
"""
draft_deployment_spec.py - 从简洁文本种子草拟 deployment.json

用法:
    python3 draft_deployment_spec.py --input deployment.seed.txt --output deployment.json
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="从简洁文本种子草拟 deployment.json")
    parser.add_argument("--input", required=True, help="种子文本文件")
    parser.add_argument("--output", required=True, help="输出 deployment.json")
    return parser.parse_args()


def parse_seed(content: str) -> dict:
    title = "部署图"
    zones: list[dict] = []
    edges: list[dict] = []
    current_zone: dict | None = None
    zone_ids: set[str] = set()
    node_ids: set[str] = set()

    for line_no, raw_line in enumerate(content.splitlines(), start=1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue

        if line.lower().startswith("title:"):
            title = line.split(":", 1)[1].strip() or title
            continue

        if line.lower().startswith("zone "):
            body = line[5:].strip()
            parts = [part.strip() for part in body.split("|")]
            if len(parts) != 2:
                raise ValueError(f"第 {line_no} 行 zone 格式错误，应为: zone <id> | <label>")
            zone_id, label = parts
            if not zone_id or not label:
                raise ValueError(f"第 {line_no} 行 zone 缺少 id 或 label")
            if zone_id in zone_ids:
                raise ValueError(f"第 {line_no} 行 zone id 重复: {zone_id}")
            current_zone = {"id": zone_id, "label": label, "nodes": []}
            zones.append(current_zone)
            zone_ids.add(zone_id)
            continue

        if line.lower().startswith("node "):
            if current_zone is None:
                raise ValueError(f"第 {line_no} 行 node 之前必须先声明 zone")
            body = line[5:].strip()
            parts = [part.strip() for part in body.split("|")]
            if len(parts) < 3 or len(parts) > 4:
                raise ValueError(f"第 {line_no} 行 node 格式错误，应为: node <id> | <label> | <kind> | <subtitle?>")
            node_id, label, kind = parts[:3]
            subtitle = parts[3] if len(parts) == 4 else ""
            if not node_id or not label or not kind:
                raise ValueError(f"第 {line_no} 行 node 缺少必填字段")
            if node_id in node_ids:
                raise ValueError(f"第 {line_no} 行 node id 重复: {node_id}")
            current_zone["nodes"].append(
                {
                    "id": node_id,
                    "label": label,
                    "kind": kind,
                    **({"subtitle": subtitle} if subtitle else {}),
                }
            )
            node_ids.add(node_id)
            continue

        if line.lower().startswith("edge "):
            body = line[5:].strip()
            left, separator, right = body.partition("->")
            if not separator:
                raise ValueError(f"第 {line_no} 行 edge 格式错误，应为: edge <source> -> <target> | <label?>")
            source = left.strip()
            target_and_label = [part.strip() for part in right.split("|")]
            target = target_and_label[0] if target_and_label else ""
            label = target_and_label[1] if len(target_and_label) > 1 else ""
            if not source or not target:
                raise ValueError(f"第 {line_no} 行 edge 缺少 source 或 target")
            edges.append({"source": source, "target": target, **({"label": label} if label else {})})
            continue

        raise ValueError(f"第 {line_no} 行无法识别: {line}")

    if not zones:
        raise ValueError("至少需要一个 zone")

    for zone in zones:
        if not zone["nodes"]:
            raise ValueError(f"zone {zone['id']} 没有 node")

    for edge in edges:
        if edge["source"] not in node_ids:
            raise ValueError(f"edge source 不存在: {edge['source']}")
        if edge["target"] not in node_ids:
            raise ValueError(f"edge target 不存在: {edge['target']}")

    return {"title": title, "zones": zones, "edges": edges}


def main() -> int:
    args = parse_args()
    input_path = Path(args.input).resolve()
    output_path = Path(args.output).resolve()

    if not input_path.exists():
        raise SystemExit(f"输入文件不存在: {input_path}")

    try:
        spec = parse_seed(input_path.read_text(encoding="utf-8"))
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(spec, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    print(f"✓ 已生成 deployment spec: {output_path}")
    print(f"  - zones: {len(spec['zones'])}")
    print(f"  - edges: {len(spec['edges'])}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
