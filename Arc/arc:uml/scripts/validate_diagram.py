#!/usr/bin/env python3
"""
validate_diagram.py - 自动化 UML 图质量验证

支持格式:
- .drawio (drawio XML)
- .mmd (Mermaid 文本)

检查清单:
- 时序图：消息数量、生命线排序建议、激活条使用、交叉线估算
- 部署图：节点挂接、正交路由、网格对齐、重叠与漂浮连线
- 活动图：初始节点、结束路径、控制流完整性
- 类图：关系类型正确性、属性可见性
- 用例图：系统边界、参与者位置、用例位置、include/extend 关系
- 状态机图：初始状态、终止状态、稳定状态、迁移边
- 组件图：组件元素、接口/依赖关系、标准记法

用法:
    python3 validate_diagram.py <diagram.drawio> [--type sequence|deployment|activity|class|use-case|state-machine|component]
    python3 validate_diagram.py <diagram.mmd> [--type sequence]
"""

from __future__ import annotations

import argparse
import html
import json
import re
import sys
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Tuple


@dataclass
class ValidationIssue:
    severity: str  # error, warning, info
    message: str
    element_id: Optional[str] = None


@dataclass
class ValidationResult:
    diagram_type: str
    issues: List[ValidationIssue] = field(default_factory=list)
    stats: Dict[str, int] = field(default_factory=dict)

    def add_issue(self, severity: str, message: str, element_id: Optional[str] = None) -> None:
        self.issues.append(ValidationIssue(severity, message, element_id))

    def has_errors(self) -> bool:
        return any(i.severity == "error" for i in self.issues)

    def has_warnings(self) -> bool:
        return any(i.severity == "warning" for i in self.issues)


class MermaidParser:
    """解析 Mermaid 时序图"""

    def __init__(self, content: str) -> None:
        self.content = content
        self.lines = [line.strip() for line in content.split('\n') if line.strip()]

    def parse_sequence(self) -> Dict:
        """解析时序图元素"""
        result = {
            'participants': [],
            'messages': [],
            'loops': [],
            'alts': [],
            'opts': [],
            'activations': [],
            'notes': [],
        }

        in_loop = False
        in_alt = False
        loop_count = 0
        alt_count = 0

        for line in self.lines:
            # 跳过 diagram 声明
            if line.startswith('sequenceDiagram'):
                continue

            # 解析参与者声明
            if line.startswith('participant '):
                match = re.match(r'participant\s+(\w+)(?:\s+as\s+(.+))?', line)
                if match:
                    result['participants'].append({
                        'id': match.group(1),
                        'label': match.group(2) or match.group(1)
                    })

            # 解析消息 (A->>B: message / A-->>B: message)
            msg_match = re.match(r'(\w+)\s*(--?>>?)\s*(\w+)\s*:\s*(.+)', line)
            if msg_match:
                source = msg_match.group(1)
                arrow_token = msg_match.group(2)
                target = msg_match.group(3)
                msg_text = msg_match.group(4).strip()

                msg_type = 'return' if arrow_token.startswith('--') else ('async' if arrow_token.endswith('>>') else 'sync')
                result['messages'].append({
                    'source': source,
                    'target': target,
                    'text': msg_text,
                    'type': msg_type,
                })

            # 解析激活 (activate/deactivate)
            if line.startswith('activate '):
                result['activations'].append({'action': 'activate', 'target': line[9:].strip()})
            if line.startswith('deactivate '):
                result['activations'].append({'action': 'deactivate', 'target': line[11:].strip()})

            # 解析 loop
            if line.startswith('loop '):
                loop_count += 1
                in_loop = True
                result['loops'].append({'condition': line[5:].strip()})
            if line == 'end' and in_loop:
                in_loop = False

            # 解析 alt
            if line.startswith('alt '):
                alt_count += 1
                in_alt = True
                result['alts'].append({'condition': line[4:].strip()})
            if line == 'end' and in_alt and not in_loop:
                in_alt = False

            # 解析 opt
            if line.startswith('opt '):
                result['opts'].append({'condition': line[4:].strip()})

            # 解析 note
            if line.startswith('Note '):
                result['notes'].append({'text': line})

        return result


def extract_mermaid_source(content: str) -> str:
    stripped = content.strip()
    if stripped.startswith("sequenceDiagram"):
        return stripped

    lines = content.splitlines()
    capturing = False
    captured: list[str] = []

    for line in lines:
        trimmed = line.strip()
        if trimmed.startswith("sequenceDiagram"):
            capturing = True
        if capturing:
            if trimmed.startswith("```") and captured:
                break
            if not trimmed.startswith("```"):
                captured.append(line.rstrip())

    source = "\n".join(captured).strip()
    return source if source.startswith("sequenceDiagram") else stripped


class MermaidSequenceValidator:
    """Mermaid 时序图验证器"""

    MAX_MESSAGES = 20
    BRANCH_SIGNAL_KEYWORDS = (
        "成功", "失败", "通过", "不通过", "异常", "超时", "取消", "拒绝", "命中", "未命中",
        "已支付", "未支付", "存在", "不存在", "有库存", "无库存", "yes", "no", "true", "false",
        "else", "otherwise", "error",
    )

    def __init__(self, parser: MermaidParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("sequence (Mermaid)")
        data = self.parser.parse_sequence()

        participants = data['participants']
        messages = data['messages']
        loops = data['loops']
        alts = data['alts']
        opts = data['opts']
        activations = data['activations']

        # 统计
        result.stats['participants'] = len(participants)
        result.stats['messages'] = len(messages)
        result.stats['loops'] = len(loops)
        result.stats['alts'] = len(alts)
        result.stats['opts'] = len(opts)
        result.stats['activations'] = len(activations)

        # 1. 检查参与者数量
        if len(participants) == 0:
            result.add_issue("error", "未定义任何参与者 (participant)，时序图至少需要两个参与者")
        elif len(participants) < 2:
            result.add_issue("warning", f"只有{len(participants)}个参与者，时序图通常需要多个参与者交互")
        elif len(participants) > 8:
            result.add_issue("warning", f"参与者数量({len(participants)})较多，建议拆分为多个子图或使用 ref")

        # 2. 检查消息数量
        if len(messages) > self.MAX_MESSAGES:
            result.add_issue(
                "error",
                f"消息数量({len(messages)})超过建议上限({self.MAX_MESSAGES})，请拆分子图"
            )
        elif len(messages) > 15:
            result.add_issue(
                "warning",
                f"消息数量({len(messages)})接近上限，建议优化"
            )
        elif len(messages) == 0:
            result.add_issue("error", "没有定义任何消息，时序图至少需要一条消息")

        # 3. 检查激活块使用
        sync_messages = [m for m in messages if m['type'] == 'sync']
        activate_count = len([a for a in activations if a['action'] == 'activate'])

        if len(sync_messages) > 0 and activate_count == 0:
            result.add_issue(
                "warning",
                f"检测到{len(sync_messages)}个同步调用但未使用 activate/deactivate，"
                f"建议使用激活块明确调用范围"
            )

        # 4. 检查结构化片段使用
        if len(messages) > 10 and len(alts) == 0 and len(opts) == 0 and len(loops) == 0:
            result.add_issue(
                "info",
                "消息较多但未使用 loop/alt/opt 片段组织，建议使用结构化片段提高可读性"
            )

        # 4.1 检查分支是否使用矩形组合片段
        branch_signal_count = self._count_branch_signal_messages(messages)
        if branch_signal_count >= 2 and len(alts) == 0:
            result.add_issue(
                "error",
                "检测到多个不同情况/结果消息，但未使用 alt 组合片段矩形框。"
                "两种情况应放进同一个 alt 矩形框，而不是直接用箭头并列表达。"
            )
        elif branch_signal_count == 1 and len(alts) == 0 and len(opts) == 0:
            result.add_issue(
                "warning",
                "检测到条件性消息，但未使用 opt/alt 矩形片段。"
                "单一可选场景建议使用 opt，两种不同情况建议使用 alt。"
            )

        # 5. 检查空消息标签
        empty_messages = [m for m in messages if not m['text']]
        if empty_messages:
            result.add_issue(
                "warning",
                f"发现{len(empty_messages)}个空消息标签，建议添加描述"
            )

        # 6. 检查生命线排序建议
        participant_names = [p['id'] for p in participants]
        crossing_estimate = self._estimate_crossings(messages, participant_names)
        if crossing_estimate > len(participants) * 3:
            result.add_issue(
                "warning",
                f"估计消息交叉较多({crossing_estimate})，建议按调用顺序排列参与者："
                f"前端→Controller→Service→DAO→外部服务"
            )

        return result

    def _count_branch_signal_messages(self, messages: List[Dict]) -> int:
        count = 0
        for message in messages:
            text = (message.get('text') or "").lower()
            if any(keyword.lower() in text for keyword in self.BRANCH_SIGNAL_KEYWORDS):
                count += 1
        return count

    def _estimate_crossings(self, messages: List[Dict], participants: List[str]) -> int:
        """估算消息交叉数"""
        crossings = 0
        participant_idx = {p: i for i, p in enumerate(participants)}

        for i, msg1 in enumerate(messages):
            idx1_src = participant_idx.get(msg1['source'], -1)
            idx1_tgt = participant_idx.get(msg1['target'], -1)

            for msg2 in messages[i+1:]:
                idx2_src = participant_idx.get(msg2['source'], -1)
                idx2_tgt = participant_idx.get(msg2['target'], -1)

                if all(idx >= 0 for idx in [idx1_src, idx1_tgt, idx2_src, idx2_tgt]):
                    if (idx1_src < idx2_src and idx1_tgt > idx2_tgt) or \
                       (idx1_src > idx2_src and idx1_tgt < idx2_tgt):
                        crossings += 1

        return crossings


class DrawioParser:
    """解析 drawio XML 文件，提取图元素信息"""

    def __init__(self, xml_content: str) -> None:
        self.root = ET.fromstring(xml_content)
        self.ns = {"mx": "http://www.w3.org/1999/xhtml"}

    def get_all_cells(self) -> List[ET.Element]:
        """获取所有 mxCell 元素"""
        cells = []
        for cell in self.root.iter("mxCell"):
            cells.append(cell)
        return cells

    def get_lifelines(self) -> List[Dict]:
        """提取时序图生命线（垂直线/泳道）"""
        lifelines = []
        for cell in self.get_all_cells():
            style = cell.get("style", "")
            value = cell.get("value", "")
            # 生命线通常是带 text 的矩形或垂直线
            if "text" in style or "swimlane" in style or "ellipse" not in style:
                if value and len(value) < 50:  # 排除长文本注释
                    lifelines.append({
                        "id": cell.get("id"),
                        "value": value,
                        "style": style,
                        "x": self._get_geometry_attr(cell, "x"),
                        "y": self._get_geometry_attr(cell, "y"),
                    })
        return lifelines

    def get_vertices(self) -> List[Dict]:
        """提取所有节点"""
        vertices = []
        for cell in self.get_all_cells():
            if cell.get("vertex") != "1":
                continue

            geom = self._get_geometry(cell)
            vertices.append({
                "id": cell.get("id"),
                "value": cell.get("value", ""),
                "style": cell.get("style", ""),
                "parent": cell.get("parent", ""),
                "x": self._get_int_attr(geom, "x"),
                "y": self._get_int_attr(geom, "y"),
                "width": self._get_int_attr(geom, "width"),
                "height": self._get_int_attr(geom, "height"),
            })
        return vertices

    def get_top_level_vertices(self) -> List[Dict]:
        """提取部署图常见的顶层节点（过滤注释和超长文本框）"""
        vertices = []
        for vertex in self.get_vertices():
            if vertex["parent"] != "1":
                continue
            if vertex["x"] is None or vertex["y"] is None:
                continue
            if not vertex["width"] or not vertex["height"]:
                continue
            if len(vertex["value"]) > 120:
                continue
            if self._looks_like_annotation(vertex["style"], vertex["value"]):
                continue
            vertices.append(vertex)
        return vertices

    def get_edges(self) -> List[Dict]:
        """提取所有边及其几何信息"""
        edges = []
        for cell in self.get_all_cells():
            if cell.get("edge") != "1":
                continue

            geom = self._get_geometry(cell)
            edges.append({
                "id": cell.get("id"),
                "value": cell.get("value", ""),
                "style": cell.get("style", ""),
                "source": cell.get("source", ""),
                "target": cell.get("target", ""),
                "has_source_point": self._has_named_point(geom, "sourcePoint"),
                "has_target_point": self._has_named_point(geom, "targetPoint"),
                "waypoint_count": self._count_waypoints(geom),
            })
        return edges

    def get_messages(self) -> List[Dict]:
        """提取时序图消息（带箭头的边）"""
        messages = []
        for edge in self.get_edges():
            style = edge["style"]
            value = edge["value"]

            # 识别消息边（edge=1 且带箭头样式）
            if "arrow" in style or "endArrow" in style:
                # 判断消息类型
                msg_type = self._classify_message(style)

                messages.append({
                    "id": edge["id"],
                    "value": value,
                    "source": edge["source"],
                    "target": edge["target"],
                    "type": msg_type,
                    "style": style,
                })
        return messages

    def get_embedded_mermaid_sources(self) -> List[str]:
        """提取 drawio 中挂载的 Mermaid 源码"""
        sources = []
        for cell in self.get_all_cells():
            payload = cell.get("mermaidData")
            if not payload:
                continue
            try:
                parsed = json.loads(payload)
            except json.JSONDecodeError:
                continue
            source = parsed.get("data")
            if isinstance(source, str) and source.strip():
                sources.append(source)
        return sources

    def get_activation_bars(self) -> List[Dict]:
        """提取激活条（实心矩形）"""
        bars = []
        for cell in self.get_all_cells():
            style = cell.get("style", "")
            # 激活条通常是 fillColor=none 或特定填充色的矩形
            if "rounded=0" in style or "rectangle" in style:
                fill = self._extract_style_attr(style, "fillColor")
                if fill and fill not in ("none", "#ffffff", "#FFFFFF", "white"):
                    bars.append({
                        "id": cell.get("id"),
                        "parent": cell.get("parent"),
                        "fill": fill,
                    })
        return bars

    def get_fragments(self) -> List[Dict]:
        """提取结构化片段（alt/opt/loop）"""
        fragments = []
        for cell in self.get_all_cells():
            style = cell.get("style", "")
            value = cell.get("value", "")
            # 片段通常是大括号样式的容器
            if any(kw in value.lower() for kw in ["alt", "opt", "loop", "par", "break"]):
                fragments.append({
                    "id": cell.get("id"),
                    "value": value,
                    "type": self._classify_fragment(value),
                })
        return fragments

    def _get_geometry(self, cell: ET.Element) -> Optional[ET.Element]:
        return cell.find("mxGeometry")

    def _get_geometry_attr(self, cell: ET.Element, attr: str) -> Optional[int]:
        """从 mxGeometry 子元素获取属性"""
        return self._get_int_attr(self._get_geometry(cell), attr)

    def _get_int_attr(self, element: Optional[ET.Element], attr: str) -> Optional[int]:
        if element is None:
            return None
        value = element.get(attr)
        if value is None:
            return None
        try:
            return int(float(value))
        except (TypeError, ValueError):
            return None

    def _has_named_point(self, geom: Optional[ET.Element], point_name: str) -> bool:
        if geom is None:
            return False
        return geom.find(f"mxPoint[@as='{point_name}']") is not None

    def _count_waypoints(self, geom: Optional[ET.Element]) -> int:
        if geom is None:
            return 0
        points = geom.find("Array[@as='points']")
        if points is None:
            return 0
        return len(points.findall("mxPoint"))

    def _looks_like_annotation(self, style: str, value: str) -> bool:
        lowered = style.lower()
        return (
            "note" in lowered
            or "text" in lowered
            or "fillColor=#fff2cc" in style
            or value.strip().startswith("#")
        )

    def clean_value_lines(self, value: str) -> List[str]:
        normalized = (
            value.replace("&#xa;", "\n")
            .replace("<br>", "\n")
            .replace("<br/>", "\n")
            .replace("<div>", "\n")
            .replace("</div>", "")
            .replace("<hr>", "\n")
            .replace("<hr/>", "\n")
        )
        normalized = re.sub(r"<[^>]+>", "", normalized)
        normalized = html.unescape(normalized)
        return [line.strip() for line in normalized.splitlines() if line.strip()]

    def _extract_style_attr(self, style: str, attr: str) -> Optional[str]:
        """从 style 字符串提取属性值"""
        pattern = rf"{attr}=([^;]+)"
        match = re.search(pattern, style)
        return match.group(1) if match else None

    def _classify_message(self, style: str) -> str:
        """根据样式判断消息类型"""
        if "dashed=1" in style:
            return "return"
        elif "async" in style.lower() or "open" in style.lower():
            return "async"
        elif "arrow" in style and "classic" in style:
            return "sync"
        return "unknown"

    def _classify_fragment(self, value: str) -> str:
        """分类结构化片段类型"""
        val_lower = value.lower()
        if "alt" in val_lower:
            return "alt"
        elif "opt" in val_lower:
            return "opt"
        elif "loop" in val_lower:
            return "loop"
        elif "par" in val_lower:
            return "par"
        elif "break" in val_lower:
            return "break"
        return "unknown"


class DrawioSequenceValidator:
    """时序图专用验证器"""

    MAX_MESSAGES = 20  # 消息数量上限
    MAX_CROSSINGS = 3  # 每生命线最大交叉数
    BRANCH_SIGNAL_KEYWORDS = MermaidSequenceValidator.BRANCH_SIGNAL_KEYWORDS

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("sequence (drawio)")

        lifelines = self.parser.get_lifelines()
        messages = self.parser.get_messages()
        bars = self.parser.get_activation_bars()
        fragments = self.parser.get_fragments()

        # 统计
        result.stats["lifelines"] = len(lifelines)
        result.stats["messages"] = len(messages)
        result.stats["activation_bars"] = len(bars)
        result.stats["fragments"] = len(fragments)

        # 1. 检查消息数量
        if len(messages) > self.MAX_MESSAGES:
            result.add_issue(
                "error",
                f"消息数量({len(messages)})超过建议上限({self.MAX_MESSAGES})，"
                f"请拆分子图或使用 ref 引用"
            )
        elif len(messages) > 15:
            result.add_issue(
                "warning",
                f"消息数量({len(messages)})接近上限，建议优化布局或拆分"
            )

        # 2. 检查生命线排序（按 X 坐标）
        sorted_lifelines = sorted(
            [ll for ll in lifelines if ll["x"] is not None],
            key=lambda x: x["x"] or 0
        )

        # 3. 检查激活条使用
        sync_messages = [m for m in messages if m["type"] == "sync"]
        if len(sync_messages) > 0 and len(bars) == 0:
            result.add_issue(
                "error",
                f"检测到{len(sync_messages)}个同步调用但未使用激活条，"
                f"请为同步调用添加实心矩形激活条"
            )
        elif len(bars) < len(sync_messages) * 0.5:
            result.add_issue(
                "warning",
                f"激活条数量({len(bars)})明显少于同步调用({len(sync_messages)})，"
                f"建议补充"
            )

        # 4. 检查交叉线估算
        crossings = self._estimate_crossings(messages, lifelines)
        if crossings > self.MAX_CROSSINGS * len(lifelines):
            result.add_issue(
                "warning",
                f"估计交叉线数量({crossings})较多，建议调整生命线排序："
                f"按调用依赖从左到右排列（前端→Controller→Service→外部）"
            )

        # 5. 检查结构化片段使用
        alt_fragments = [f for f in fragments if f["type"] == "alt"]
        opt_fragments = [f for f in fragments if f["type"] == "opt"]
        loop_fragments = [f for f in fragments if f["type"] == "loop"]
        if len(messages) > 10 and len(alt_fragments) == 0 and len(opt_fragments) == 0 and len(loop_fragments) == 0:
            result.add_issue(
                "info",
                "消息较多但未使用 alt/opt/loop 片段组织，"
                "建议使用结构化片段提高可读性"
            )

        # 5.1 检查分支是否使用矩形组合片段
        branch_signal_count = self._count_branch_signal_messages(messages)
        if branch_signal_count >= 2 and len(alt_fragments) == 0:
            result.add_issue(
                "error",
                "检测到多个不同情况/结果消息，但未使用 alt 组合片段矩形框。"
                "两种情况应放进同一个 alt 矩形框，而不是直接用箭头并列表达。"
            )
        elif branch_signal_count == 1 and len(alt_fragments) == 0 and len(opt_fragments) == 0:
            result.add_issue(
                "warning",
                "检测到条件性消息，但未使用 opt/alt 矩形片段。"
                "单一可选场景建议使用 opt，两种不同情况建议使用 alt。"
            )

        # 6. 检查消息标签
        empty_labels = [m for m in messages if not m["value"].strip()]
        if empty_labels:
            result.add_issue(
                "warning",
                f"发现{len(empty_labels)}个无标签消息，建议添加描述"
            )

        # 7. 时序图致命排版预警检查
        if len(messages) > 0 and len(lifelines) > 0:
            # 检查是否有明显的坐标问题（所有线从左侧发出）
            leftmost_x = min((ll["x"] for ll in lifelines if ll["x"] is not None), default=0)
            problematic_lines = 0
            for cell in self.parser.get_all_cells():
                geom = cell.find("mxGeometry")
                if geom is not None:
                    x = geom.get("x")
                    if x and int(x) < leftmost_x + 50:  # 太靠左
                        problematic_lines += 1

            if problematic_lines > len(messages) * 0.5:
                result.add_issue(
                    "error",
                    f"【致命排版预警】检测到{problematic_lines}条线从左侧堆积发出，"
                    f"这是原生 drawio XML 时序图的典型问题！"
                    f"请改用 Mermaid (.mmd) 格式生成时序图。"
                )

        return result

    def _estimate_crossings(self, messages: List[Dict], lifelines: List[Dict]) -> int:
        """简单估算线条交叉数"""
        crossings = 0
        lifeline_positions = {ll["id"]: ll["x"] for ll in lifelines if ll["x"] is not None}

        for i, msg1 in enumerate(messages):
            for msg2 in messages[i+1:]:
                src1 = lifeline_positions.get(msg1["source"])
                tgt1 = lifeline_positions.get(msg1["target"])
                src2 = lifeline_positions.get(msg2["source"])
                tgt2 = lifeline_positions.get(msg2["target"])

                if all(p is not None for p in [src1, tgt1, src2, tgt2]):
                    # 检查是否交叉
                    if (src1 < src2 and tgt1 > tgt2) or (src1 > src2 and tgt1 < tgt2):
                        crossings += 1

        return crossings

    def _count_branch_signal_messages(self, messages: List[Dict]) -> int:
        count = 0
        for message in messages:
            text = (message.get("value") or "").lower()
            if any(keyword.lower() in text for keyword in self.BRANCH_SIGNAL_KEYWORDS):
                count += 1
        return count


class DrawioDeploymentValidator:
    """部署图专用验证器"""

    GRID_SIZE = 10
    MAX_WAYPOINTS = 2

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("deployment (drawio)")

        vertices = self.parser.get_top_level_vertices()
        edges = self.parser.get_edges()

        result.stats["top_level_nodes"] = len(vertices)
        result.stats["edges"] = len(edges)

        if len(vertices) < 2:
            result.add_issue(
                "warning",
                f"顶层节点数量只有{len(vertices)}个，部署图通常至少需要两个可连接节点"
            )

        if len(edges) == 0:
            result.add_issue(
                "warning",
                "未检测到任何连接线，部署图通常需要明确节点之间的连接关系"
            )

        floating_edges = [edge for edge in edges if not edge["source"] or not edge["target"]]
        if floating_edges:
            result.add_issue(
                "error",
                f"发现{len(floating_edges)}条未绑定 source/target 的漂浮连接线，"
                "部署图连接必须挂接到具体节点"
            )

        manual_endpoint_edges = [
            edge for edge in edges
            if edge["source"] and edge["target"] and (edge["has_source_point"] or edge["has_target_point"])
        ]
        if manual_endpoint_edges:
            severity = "error" if len(manual_endpoint_edges) >= max(1, len(edges) // 2) else "warning"
            result.add_issue(
                severity,
                f"发现{len(manual_endpoint_edges)}条同时使用节点绑定和手工端点坐标的连接，"
                "这类连线容易与节点错位，建议删除 sourcePoint/targetPoint 并改用正交路由"
            )

        non_orthogonal_edges = [edge for edge in edges if not self._uses_orthogonal_routing(edge["style"])]
        if non_orthogonal_edges:
            severity = "warning"
            if len(non_orthogonal_edges) >= max(2, len(edges) // 2):
                severity = "error"
            result.add_issue(
                severity,
                f"发现{len(non_orthogonal_edges)}条未使用正交/肘形路由的连接，"
                "部署图连接线应优先使用正交路由以避免线框错位"
            )

        heavy_waypoint_edges = [edge for edge in edges if edge["waypoint_count"] > self.MAX_WAYPOINTS]
        if heavy_waypoint_edges:
            result.add_issue(
                "warning",
                f"发现{len(heavy_waypoint_edges)}条连接包含超过{self.MAX_WAYPOINTS}个手工拐点，"
                "建议拆分布局或交给正交路由自动处理"
            )

        off_grid_vertices = [vertex for vertex in vertices if not self._is_on_grid(vertex)]
        if off_grid_vertices:
            result.add_issue(
                "warning",
                f"发现{len(off_grid_vertices)}个顶层节点未对齐到 {self.GRID_SIZE}px 网格，"
                "建议统一节点位置，减少视觉错位"
            )

        overlaps = self._count_overlaps(vertices)
        if overlaps > 0:
            result.add_issue(
                "warning",
                f"检测到{overlaps}处顶层节点重叠，建议按分区重新排版"
            )

        return result

    def _uses_orthogonal_routing(self, style: str) -> bool:
        lowered = style.lower()
        return "orthogonaledgestyle" in lowered or "elbowedgestyle" in lowered

    def _is_on_grid(self, vertex: Dict) -> bool:
        coords = [vertex["x"], vertex["y"]]
        for coord in coords:
            if coord is None or coord % self.GRID_SIZE != 0:
                return False
        return True

    def _count_overlaps(self, vertices: List[Dict]) -> int:
        overlaps = 0
        for index, first in enumerate(vertices):
            for second in vertices[index + 1:]:
                if self._boxes_overlap(first, second):
                    overlaps += 1
        return overlaps

    def _boxes_overlap(self, first: Dict, second: Dict) -> bool:
        first_right = (first["x"] or 0) + (first["width"] or 0)
        first_bottom = (first["y"] or 0) + (first["height"] or 0)
        second_right = (second["x"] or 0) + (second["width"] or 0)
        second_bottom = (second["y"] or 0) + (second["height"] or 0)

        separated = (
            first_right <= (second["x"] or 0)
            or second_right <= (first["x"] or 0)
            or first_bottom <= (second["y"] or 0)
            or second_bottom <= (first["y"] or 0)
        )
        return not separated


class DrawioActivityValidator:
    """活动图专用验证器"""

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("activity (drawio)")
        vertices = [vertex for vertex in self.parser.get_vertices() if not self.parser._looks_like_annotation(vertex["style"], vertex["value"])]
        edges = self.parser.get_edges()

        initial_nodes = [vertex for vertex in vertices if self._is_initial_node(vertex)]
        final_nodes = [vertex for vertex in vertices if self._is_final_node(vertex)]
        action_nodes = [vertex for vertex in vertices if self._is_action_node(vertex)]
        decision_nodes = [vertex for vertex in vertices if self._is_decision_node(vertex)]
        swimlanes = [vertex for vertex in vertices if "swimlane" in vertex["style"].lower()]
        guard_edges = [edge for edge in edges if self._has_guard(edge["value"])]

        result.stats["vertices"] = len(vertices)
        result.stats["edges"] = len(edges)
        result.stats["initial_nodes"] = len(initial_nodes)
        result.stats["final_nodes"] = len(final_nodes)
        result.stats["action_nodes"] = len(action_nodes)
        result.stats["decision_nodes"] = len(decision_nodes)
        result.stats["swimlanes"] = len(swimlanes)

        if len(initial_nodes) == 0:
            result.add_issue("error", "活动图缺少初始节点，标准 UML 活动图必须从初始节点开始")
        if len(final_nodes) == 0:
            result.add_issue("error", "活动图缺少结束节点，标准 UML 活动图至少需要一条结束路径")
        if len(action_nodes) == 0:
            result.add_issue("error", "活动图未检测到动作节点，标准 UML 活动图不能只剩箭头和说明文字")
        if len(edges) == 0:
            result.add_issue("error", "活动图未检测到控制流边，标准 UML 活动图必须体现控制流")

        if len(guard_edges) > 0 and len(decision_nodes) == 0:
            result.add_issue("error", "活动图存在 guard 条件边，但未检测到判定节点（菱形）")
        elif len(decision_nodes) > 0 and len(guard_edges) == 0:
            result.add_issue("warning", "活动图检测到判定节点，但未看到 guard 条件标注，建议补 `[条件]`")

        return result

    def _is_initial_node(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "activity.initial" in style
            or "umlactivityinitial" in style
            or ("ellipse" in style and "fillcolor=#000000" in style and (vertex["width"] or 0) <= 30 and (vertex["height"] or 0) <= 30)
        )

    def _is_final_node(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "activity.final" in style
            or "umlactivityfinal" in style
            or "doubleellipse" in style
            or ("ellipse" in style and "double=1" in style)
        )

    def _is_action_node(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "rounded=1" in style
            and "ellipse" not in style
            and "rhombus" not in style
            and "diamond" not in style
            and "swimlane" not in style
        )

    def _is_decision_node(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return "rhombus" in style or "diamond" in style

    def _has_guard(self, value: str) -> bool:
        return bool(re.search(r"\[[^\]]+\]", value or ""))


class DrawioClassValidator:
    """类图专用验证器"""

    UML_RELATION_MARKERS = (
        "endarrow=open",
        "endarrow=diamond",
        "endarrow=diamondthin",
        "endarrow=block",
        "endarrow=none",
        "startarrow=open",
        "startarrow=diamond",
        "startarrow=diamondthin",
    )

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("class (drawio)")
        vertices = [vertex for vertex in self.parser.get_vertices() if not self.parser._looks_like_annotation(vertex["style"], vertex["value"])]
        edges = self.parser.get_edges()
        class_vertices = [vertex for vertex in vertices if self._is_class_box(vertex)]

        result.stats["vertices"] = len(vertices)
        result.stats["class_boxes"] = len(class_vertices)
        result.stats["edges"] = len(edges)

        if len(class_vertices) == 0:
            result.add_issue("error", "未检测到类框，标准 UML 类图至少需要一个类")

        invalid_member_lines = 0
        for vertex in class_vertices:
            lines = self.parser.clean_value_lines(vertex["value"])
            if len(lines) == 0:
                result.add_issue("error", f"类框 {vertex['id']} 缺少类名")
                continue
            for line in lines[1:]:
                if not self._is_visibility_line(line):
                    invalid_member_lines += 1

        if invalid_member_lines > 0:
            result.add_issue(
                "warning",
                f"检测到{invalid_member_lines}条类成员未使用标准可见性记号 (+/-/#/~)，建议修正为标准 UML 写法"
            )

        if len(edges) == 0:
            result.add_issue("warning", "类图未检测到任何关系边，建议补充类之间的关联/依赖/继承关系")
        else:
            non_uml_edges = [edge for edge in edges if not self._looks_like_uml_relation(edge["style"])]
            if non_uml_edges:
                result.add_issue(
                    "warning",
                    f"检测到{len(non_uml_edges)}条关系边未体现明显的 UML 关系记号，建议检查关联/依赖/聚合/组合/泛化是否画对"
                )

        return result

    def _is_class_box(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        lines = self.parser.clean_value_lines(vertex["value"])
        if "swimlane" in style or "childlayout=stacklayout" in style:
            return True
        if len(lines) >= 2 and any(self._is_visibility_line(line) for line in lines[1:]):
            return True
        return False

    def _is_visibility_line(self, line: str) -> bool:
        stripped = line.strip()
        if not stripped:
            return True
        return stripped[0] in {"+", "-", "#", "~"}

    def _looks_like_uml_relation(self, style: str) -> bool:
        lowered = style.lower()
        return any(marker in lowered for marker in self.UML_RELATION_MARKERS)


class DrawioUseCaseValidator:
    """用例图专用验证器"""

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("use-case (drawio)")
        vertices = [vertex for vertex in self.parser.get_vertices() if not self.parser._looks_like_annotation(vertex["style"], vertex["value"])]
        edges = self.parser.get_edges()

        actors = [vertex for vertex in vertices if self._is_actor(vertex)]
        use_cases = [vertex for vertex in vertices if self._is_use_case(vertex)]
        boundaries = [vertex for vertex in vertices if self._is_system_boundary(vertex)]

        result.stats["actors"] = len(actors)
        result.stats["use_cases"] = len(use_cases)
        result.stats["boundaries"] = len(boundaries)
        result.stats["edges"] = len(edges)

        if len(actors) == 0:
            result.add_issue("error", "用例图缺少参与者（Actor），标准 UML 用例图至少需要一个参与者")
        if len(use_cases) == 0:
            result.add_issue("error", "用例图缺少用例椭圆，标准 UML 用例图至少需要一个用例")
        if len(boundaries) == 0:
            result.add_issue("error", "用例图缺少系统边界，标准 UML 用例图必须区分系统内外")

        primary_boundary = self._select_primary_boundary(boundaries)
        if primary_boundary is not None:
            inside_actors = [actor for actor in actors if self._inside(actor, primary_boundary)]
            if inside_actors:
                result.add_issue("error", f"发现{len(inside_actors)}个参与者被画在系统边界内，参与者必须位于边界外")

            outside_use_cases = [use_case for use_case in use_cases if not self._inside(use_case, primary_boundary)]
            if outside_use_cases:
                result.add_issue("error", f"发现{len(outside_use_cases)}个用例未放在系统边界内，用例必须位于边界内")

        include_edges = [edge for edge in edges if "<<include>>" in (edge["value"] or "").lower()]
        extend_edges = [edge for edge in edges if "<<extend>>" in (edge["value"] or "").lower()]
        for relation_name, relation_edges in (("include", include_edges), ("extend", extend_edges)):
            invalid_relation_edges = [edge for edge in relation_edges if "dashed=1" not in edge["style"].lower()]
            if invalid_relation_edges:
                result.add_issue(
                    "warning",
                    f"检测到{len(invalid_relation_edges)}条 `<<{relation_name}>>` 关系未使用虚线依赖样式，建议修正为标准 UML 关系"
                )

        return result

    def _is_actor(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return "umlactor" in style or "shape=actor" in style

    def _is_use_case(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return "ellipse" in style and not self._is_actor(vertex) and not self._is_system_boundary(vertex)

    def _is_system_boundary(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "swimlane" in style
            or ("rounded=0" in style and "ellipse" not in style and (vertex["width"] or 0) >= 220 and (vertex["height"] or 0) >= 160)
        )

    def _select_primary_boundary(self, boundaries: List[Dict]) -> Optional[Dict]:
        if not boundaries:
            return None
        return max(boundaries, key=lambda boundary: (boundary["width"] or 0) * (boundary["height"] or 0))

    def _inside(self, inner: Dict, outer: Dict) -> bool:
        inner_x = inner["x"] or 0
        inner_y = inner["y"] or 0
        inner_right = inner_x + (inner["width"] or 0)
        inner_bottom = inner_y + (inner["height"] or 0)
        outer_x = outer["x"] or 0
        outer_y = outer["y"] or 0
        outer_right = outer_x + (outer["width"] or 0)
        outer_bottom = outer_y + (outer["height"] or 0)
        return inner_x >= outer_x and inner_y >= outer_y and inner_right <= outer_right and inner_bottom <= outer_bottom


class DrawioStateMachineValidator:
    """状态机图专用验证器"""

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("state-machine (drawio)")
        vertices = [vertex for vertex in self.parser.get_vertices() if not self.parser._looks_like_annotation(vertex["style"], vertex["value"])]
        edges = self.parser.get_edges()

        initial_nodes = [vertex for vertex in vertices if self._is_initial_state(vertex)]
        final_nodes = [vertex for vertex in vertices if self._is_final_state(vertex)]
        stable_states = [vertex for vertex in vertices if self._is_stable_state(vertex)]

        result.stats["initial_states"] = len(initial_nodes)
        result.stats["final_states"] = len(final_nodes)
        result.stats["stable_states"] = len(stable_states)
        result.stats["transitions"] = len(edges)

        if len(initial_nodes) == 0:
            result.add_issue("error", "状态机图缺少初始状态，标准 UML 状态机图必须从初始状态开始")
        if len(final_nodes) == 0:
            result.add_issue("warning", "状态机图未检测到终止状态，建议明确终止边界")
        if len(stable_states) == 0:
            result.add_issue("error", "状态机图未检测到稳定状态，不能只剩起点和箭头")
        if len(edges) == 0:
            result.add_issue("error", "状态机图未检测到迁移边，标准 UML 状态机图必须体现状态转换")

        action_like_states = [state for state in stable_states if self._looks_like_action_name(state["value"])]
        if action_like_states:
            result.add_issue(
                "warning",
                f"检测到{len(action_like_states)}个状态名更像动作而不是稳定状态，建议改成“待支付/已完成”这类状态名"
            )

        return result

    def _is_initial_state(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "initialstate" in style
            or "statediagram.initial" in style
            or ("ellipse" in style and "fillcolor=#000000" in style and (vertex["width"] or 0) <= 30 and (vertex["height"] or 0) <= 30)
        )

    def _is_final_state(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "finalstate" in style
            or "statediagram.final" in style
            or "doubleellipse" in style
            or ("ellipse" in style and "double=1" in style)
        )

    def _is_stable_state(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "rounded=1" in style
            and "ellipse" not in style
            and "swimlane" not in style
            and bool(self.parser.clean_value_lines(vertex["value"]))
        )

    def _looks_like_action_name(self, value: str) -> bool:
        first_line = self.parser.clean_value_lines(value)
        if not first_line:
            return False
        name = first_line[0]
        action_keywords = ("提交", "创建", "执行", "处理", "发送", "调用", "支付", "更新", "删除", "同步", "校验")
        english_action_prefixes = ("create", "submit", "send", "update", "delete", "process", "pay", "sync", "validate")
        lowered = name.lower()
        return any(keyword in name for keyword in action_keywords) or any(lowered.startswith(prefix) for prefix in english_action_prefixes)


class DrawioComponentValidator:
    """组件图专用验证器"""

    def __init__(self, parser: DrawioParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("component (drawio)")
        vertices = [vertex for vertex in self.parser.get_vertices() if not self.parser._looks_like_annotation(vertex["style"], vertex["value"])]
        edges = self.parser.get_edges()

        components = [vertex for vertex in vertices if self._is_component(vertex)]
        interfaces = [vertex for vertex in vertices if self._is_interface(vertex)]
        dependency_edges = [edge for edge in edges if self._is_dependency(edge)]

        result.stats["components"] = len(components)
        result.stats["interfaces"] = len(interfaces)
        result.stats["edges"] = len(edges)
        result.stats["dependency_edges"] = len(dependency_edges)

        if len(components) == 0:
            result.add_issue("error", "组件图未检测到标准组件元素，请使用 component 记法或 `<<component>>` stereotype")

        if len(components) >= 2 and len(edges) == 0:
            result.add_issue("error", "组件图包含多个组件但没有任何关系边，无法表达依赖或接口关系")
        elif len(components) >= 2 and len(dependency_edges) == 0 and len(interfaces) == 0:
            result.add_issue(
                "warning",
                "组件图未检测到明显的依赖边或接口记法，建议补充 provided/required interface 或依赖关系"
            )

        plain_boxes = [vertex for vertex in vertices if self._looks_like_plain_box(vertex) and not self._is_component(vertex)]
        if plain_boxes and len(components) == 0:
            result.add_issue(
                "warning",
                f"检测到{len(plain_boxes)}个普通矩形节点，但未体现标准组件记法，容易退化成一般框图"
            )

        return result

    def _is_component(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        lines = self.parser.clean_value_lines(vertex["value"])
        joined = " ".join(lines).lower()
        return (
            "shape=component" in style
            or "umlcomponent" in style
            or "<<component>>" in joined
            or "&lt;&lt;component&gt;&gt;" in (vertex["value"] or "").lower()
        )

    def _is_interface(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        lines = self.parser.clean_value_lines(vertex["value"])
        joined = " ".join(lines).lower()
        small_circle = "ellipse" in style and (vertex["width"] or 0) <= 60 and (vertex["height"] or 0) <= 60
        return (
            "providedinterface" in style
            or "requiredinterface" in style
            or "lollipop" in style
            or "<<interface>>" in joined
            or small_circle
        )

    def _is_dependency(self, edge: Dict) -> bool:
        style = edge["style"].lower()
        return (
            "dashed=1" in style
            or "endarrow=open" in style
            or "startarrow=open" in style
        )

    def _looks_like_plain_box(self, vertex: Dict) -> bool:
        style = vertex["style"].lower()
        return (
            "rounded=1" in style or "rounded=0" in style
        ) and "ellipse" not in style and "swimlane" not in style


def format_result(result: ValidationResult, verbose: bool = False) -> str:
    """格式化验证结果输出"""
    lines = []
    lines.append(f"\n📊 验证结果: {result.diagram_type} 图")
    lines.append("=" * 50)

    # 统计
    lines.append("\n📈 统计:")
    for key, value in result.stats.items():
        lines.append(f"  - {key}: {value}")

    # 问题
    if result.issues:
        lines.append("\n📝 检查结果:")
        for issue in result.issues:
            icon = {"error": "❌", "warning": "⚠️", "info": "ℹ️"}.get(issue.severity, "•")
            lines.append(f"  {icon} [{issue.severity.upper()}] {issue.message}")
    else:
        lines.append("\n✅ 所有检查通过")

    # 总结
    lines.append("\n" + "=" * 50)
    if result.has_errors():
        lines.append("❌ 验证失败：存在需要修复的错误")
    elif result.has_warnings():
        lines.append("⚠️ 验证通过：存在警告，建议优化")
    else:
        lines.append("✅ 验证通过：图质量良好")

    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="验证 UML 图质量 (支持 .drawio 和 .mmd)")
    parser.add_argument("diagram_file", help="图文件路径 (.drawio 或 .mmd)")
    parser.add_argument(
        "--type",
        choices=["sequence", "deployment", "activity", "class", "use-case", "state-machine", "component", "auto"],
        default="auto",
        help="图类型（默认自动检测）"
    )
    parser.add_argument("-v", "--verbose", action="store_true", help="详细输出")
    args = parser.parse_args()

    diagram_path = Path(args.diagram_file)
    if not diagram_path.exists():
        print(f"错误: 文件不存在: {diagram_path}", file=sys.stderr)
        return 1

    try:
        content = diagram_path.read_text(encoding="utf-8")
    except Exception as e:
        print(f"错误: 无法读取文件: {e}", file=sys.stderr)
        return 1

    # 根据文件扩展名判断格式
    is_mermaid = diagram_path.suffix.lower() in ['.mmd', '.mermaid']
    is_drawio = diagram_path.suffix.lower() in ['.drawio', '.xml']

    # 自动检测图类型
    diagram_type = args.type
    if diagram_type == "auto":
        filename = diagram_path.stem.lower()
        if "sequence" in filename or "时序" in filename:
            diagram_type = "sequence"
        elif "deployment" in filename or "部署" in filename:
            diagram_type = "deployment"
        elif "activity" in filename or "活动" in filename:
            diagram_type = "activity"
        elif "use-case" in filename or "用例" in filename:
            diagram_type = "use-case"
        elif "state-machine" in filename or "状态机" in filename:
            diagram_type = "state-machine"
        elif "component" in filename or "组件" in filename:
            diagram_type = "component"
        elif "class" in filename or "类图" in filename:
            diagram_type = "class"
        else:
            diagram_type = "sequence"  # 默认

    # 根据格式选择验证器
    try:
        if is_mermaid and diagram_type == "sequence":
            mermaid_parser = MermaidParser(extract_mermaid_source(content))
            validator = MermaidSequenceValidator(mermaid_parser)
            result = validator.validate()
        elif is_drawio and diagram_type == "sequence":
            drawio_parser = DrawioParser(content)
            mermaid_sources = drawio_parser.get_embedded_mermaid_sources()
            if mermaid_sources:
                mermaid_parser = MermaidParser(mermaid_sources[0])
                validator = MermaidSequenceValidator(mermaid_parser)
                result = validator.validate()
                result.diagram_type = "sequence (drawio embedded Mermaid)"
            else:
                validator = DrawioSequenceValidator(drawio_parser)
                result = validator.validate()
        elif is_drawio and diagram_type == "deployment":
            drawio_parser = DrawioParser(content)
            validator = DrawioDeploymentValidator(drawio_parser)
            result = validator.validate()
        elif is_drawio and diagram_type == "activity":
            drawio_parser = DrawioParser(content)
            validator = DrawioActivityValidator(drawio_parser)
            result = validator.validate()
        elif is_drawio and diagram_type == "class":
            drawio_parser = DrawioParser(content)
            validator = DrawioClassValidator(drawio_parser)
            result = validator.validate()
        elif is_drawio and diagram_type == "use-case":
            drawio_parser = DrawioParser(content)
            validator = DrawioUseCaseValidator(drawio_parser)
            result = validator.validate()
        elif is_drawio and diagram_type == "state-machine":
            drawio_parser = DrawioParser(content)
            validator = DrawioStateMachineValidator(drawio_parser)
            result = validator.validate()
        elif is_drawio and diagram_type == "component":
            drawio_parser = DrawioParser(content)
            validator = DrawioComponentValidator(drawio_parser)
            result = validator.validate()
        else:
            # 其他组合暂未实现详细验证
            result = ValidationResult(f"{diagram_type} ({'Mermaid' if is_mermaid else 'drawio'})")
            if is_mermaid:
                result.add_issue("info", f"{diagram_type} 图的 Mermaid 验证尚未完全实现")
            else:
                result.add_issue("info", f"{diagram_type} 图的 drawio 验证尚未完全实现")
    except ET.ParseError as exc:
        print(f"错误: 无法解析 drawio XML: {exc}", file=sys.stderr)
        return 1

    print(format_result(result, args.verbose))

    return 1 if result.has_errors() else 0


if __name__ == "__main__":
    raise SystemExit(main())
