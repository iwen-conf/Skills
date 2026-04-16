#!/usr/bin/env python3
"""
generate_er_drawio.py - 根据 JSON 规格生成陈氏 E-R 图的 draw.io XML

用法:
    python3 generate_er_drawio.py --spec er.json --output er.drawio

er.json 格式示例:
{
  "entities": [{"id": "e1", "name": "User", "type": "strong"}],
  "attributes": [{"id": "a1", "name": "user_id", "type": "primary", "entity_id": "e1"}],
  "relationships": [{"id": "r1", "name": "Buys", "type": "strong"}],
  "connections": [
    {"source": "e1", "target": "r1", "label": "1"}
  ]
}
"""

import argparse
import json
import xml.etree.ElementTree as ET
from pathlib import Path
import math

def parse_args():
    parser = argparse.ArgumentParser(description="生成陈氏 E-R 图的 draw.io 文件")
    parser.add_argument("--spec", required=True, help="ER JSON 规格文件")
    parser.add_argument("--output", required=True, help="输出 .drawio 文件路径")
    return parser.parse_args()

def get_style(node_type, category):
    styles = {
        "entity": {
            "strong": "shape=rectangle;whiteSpace=wrap;html=1;align=center;",
            "weak": "shape=rectangle;whiteSpace=wrap;html=1;align=center;strokeWidth=2;" # Ideally double line, but thick line is ok
        },
        "attribute": {
            "normal": "shape=ellipse;whiteSpace=wrap;html=1;align=center;",
            "primary": "shape=ellipse;whiteSpace=wrap;html=1;align=center;fontStyle=4;", # Underlined
            "multivalued": "shape=ellipse;whiteSpace=wrap;html=1;align=center;strokeWidth=2;",
            "derived": "shape=ellipse;whiteSpace=wrap;html=1;align=center;dashed=1;"
        },
        "relationship": {
            "strong": "shape=rhombus;whiteSpace=wrap;html=1;align=center;",
            "weak": "shape=rhombus;whiteSpace=wrap;html=1;align=center;strokeWidth=2;"
        }
    }
    return styles.get(category, {}).get(node_type, styles.get(category, {}).get("strong") or styles.get(category, {}).get("normal"))

def main():
    args = parse_args()
    spec_path = Path(args.spec)
    output_path = Path(args.output)
    
    if not spec_path.exists():
        raise SystemExit(f"规格文件不存在: {spec_path}")
        
    with open(spec_path, "r", encoding="utf-8") as f:
        spec = json.load(f)
        
    mxfile = ET.Element("mxfile", {"version": "24.7.17"})
    diagram = ET.SubElement(mxfile, "diagram", {"id": "er", "name": "ER Diagram"})
    graph = ET.SubElement(diagram, "mxGraphModel", {"dx": "1000", "dy": "800", "grid": "1", "gridSize": "10", "guides": "1", "tooltips": "1", "connect": "1", "arrows": "1", "fold": "1", "page": "1", "pageScale": "1", "pageWidth": "1169", "pageHeight": "827", "math": "0", "shadow": "0"})
    root = ET.SubElement(graph, "root")
    ET.SubElement(root, "mxCell", {"id": "0"})
    ET.SubElement(root, "mxCell", {"id": "1", "parent": "0"})

    nodes = []
    
    # Place entities in a circle
    entities = spec.get("entities", [])
    relationships = spec.get("relationships", [])
    attributes = spec.get("attributes", [])
    
    center_x, center_y = 500, 400
    radius = 300
    
    total_nodes = len(entities) + len(relationships)
    angle_step = (2 * math.pi) / max(1, total_nodes)
    
    idx = 0
    # Add Entities
    for ent in entities:
        x = center_x + radius * math.cos(idx * angle_step) - 60
        y = center_y + radius * math.sin(idx * angle_step) - 20
        cell = ET.SubElement(root, "mxCell", {
            "id": ent["id"],
            "value": ent["name"],
            "style": get_style(ent.get("type", "strong"), "entity"),
            "vertex": "1",
            "parent": "1"
        })
        ET.SubElement(cell, "mxGeometry", {"x": str(int(x)), "y": str(int(y)), "width": "120", "height": "40", "as": "geometry"})
        idx += 1
        
    # Add Relationships
    for rel in relationships:
        x = center_x + (radius/2) * math.cos(idx * angle_step) - 60
        y = center_y + (radius/2) * math.sin(idx * angle_step) - 40
        cell = ET.SubElement(root, "mxCell", {
            "id": rel["id"],
            "value": rel["name"],
            "style": get_style(rel.get("type", "strong"), "relationship"),
            "vertex": "1",
            "parent": "1"
        })
        ET.SubElement(cell, "mxGeometry", {"x": str(int(x)), "y": str(int(y)), "width": "120", "height": "80", "as": "geometry"})
        idx += 1

    # Add Attributes near their entities/relationships
    # For simplicity, arrange attributes in a small circle around their owner
    attr_map = {}
    for attr in attributes:
        owner_id = attr.get("entity_id") or attr.get("relationship_id")
        if owner_id not in attr_map:
            attr_map[owner_id] = []
        attr_map[owner_id].append(attr)

    for owner_id, attrs in attr_map.items():
        # find owner cell geometry
        owner_cell = root.find(f".//mxCell[@id='{owner_id}']")
        if owner_cell is not None:
            geom = owner_cell.find("mxGeometry")
            ox = float(geom.get("x")) + float(geom.get("width")) / 2
            oy = float(geom.get("y")) + float(geom.get("height")) / 2
            
            a_radius = 100
            a_step = (2 * math.pi) / max(1, len(attrs))
            for i, attr in enumerate(attrs):
                ax = ox + a_radius * math.cos(i * a_step) - 40
                ay = oy + a_radius * math.sin(i * a_step) - 20
                
                cell = ET.SubElement(root, "mxCell", {
                    "id": attr["id"],
                    "value": attr["name"],
                    "style": get_style(attr.get("type", "normal"), "attribute"),
                    "vertex": "1",
                    "parent": "1"
                })
                ET.SubElement(cell, "mxGeometry", {"x": str(int(ax)), "y": str(int(ay)), "width": "80", "height": "40", "as": "geometry"})
                
                # Connect attribute to owner
                edge = ET.SubElement(root, "mxCell", {
                    "id": f"edge_{attr['id']}_{owner_id}",
                    "style": "endArrow=none;html=1;rounded=0;",
                    "edge": "1",
                    "parent": "1",
                    "source": attr["id"],
                    "target": owner_id
                })
                ET.SubElement(edge, "mxGeometry", {"relative": "1", "as": "geometry"})

    # Add Connections
    connections = spec.get("connections", [])
    for i, conn in enumerate(connections):
        style = "endArrow=none;html=1;rounded=0;"
        if conn.get("label"):
            style += f"labelBackgroundColor=#ffffff;"
            
        edge = ET.SubElement(root, "mxCell", {
            "id": f"conn_{i}",
            "value": conn.get("label", ""),
            "style": style,
            "edge": "1",
            "parent": "1",
            "source": conn["source"],
            "target": conn["target"]
        })
        ET.SubElement(edge, "mxGeometry", {"relative": "1", "as": "geometry"})

    ET.indent(mxfile, space="  ")
    tree = ET.ElementTree(mxfile)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    tree.write(output_path, encoding="utf-8", xml_declaration=True)
    print(f"✓ 已生成 E-R drawio 文件: {output_path}")

if __name__ == "__main__":
    main()