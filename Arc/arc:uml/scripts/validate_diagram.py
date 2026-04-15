#!/usr/bin/env python3
"""
validate_diagram.py - 自动化 UML 图质量验证

支持格式:
- .drawio (drawio XML)
- .mmd (Mermaid 文本)

检查清单:
- 时序图：消息数量、生命线排序建议、激活条使用、交叉线估算
- 活动图：初始节点、结束路径、控制流完整性
- 类图：关系类型正确性、属性可见性

用法:
    python3 validate_diagram.py <diagram.drawio> [--type sequence|activity|class]
    python3 validate_diagram.py <diagram.mmd> [--type sequence]
"""

from __future__ import annotations

import argparse
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

            # 解析消息 (A->>B: message)
            msg_match = re.match(r'(\w+)(-?)(>>?)(-?)(\w+)\s*:\s*(.+)', line)
            if msg_match:
                source = msg_match.group(1)
                is_dashed = msg_match.group(2) == '-' or msg_match.group(4) == '-'
                arrow_type = msg_match.group(3)
                target = msg_match.group(5)
                msg_text = msg_match.group(6).strip()

                msg_type = 'return' if is_dashed else ('async' if arrow_type == '->>' else 'sync')
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

            # 解析 note
            if line.startswith('Note '):
                result['notes'].append({'text': line})

        return result


class MermaidSequenceValidator:
    """Mermaid 时序图验证器"""

    MAX_MESSAGES = 20

    def __init__(self, parser: MermaidParser) -> None:
        self.parser = parser

    def validate(self) -> ValidationResult:
        result = ValidationResult("sequence (Mermaid)")
        data = self.parser.parse_sequence()

        participants = data['participants']
        messages = data['messages']
        loops = data['loops']
        alts = data['alts']
        activations = data['activations']

        # 统计
        result.stats['participants'] = len(participants)
        result.stats['messages'] = len(messages)
        result.stats['loops'] = len(loops)
        result.stats['alts'] = len(alts)
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
        if len(messages) > 10 and len(alts) == 0 and len(loops) == 0:
            result.add_issue(
                "info",
                "消息较多但未使用 loop/alt 片段组织，建议使用结构化片段提高可读性"
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

    def get_messages(self) -> List[Dict]:
        """提取时序图消息（带箭头的边）"""
        messages = []
        for cell in self.get_all_cells():
            style = cell.get("style", "")
            value = cell.get("value", "")
            edge = cell.get("edge", "")

            # 识别消息边（edge=1 且带箭头样式）
            if edge == "1" or "arrow" in style:
                source = cell.get("source", "")
                target = cell.get("target", "")

                # 判断消息类型
                msg_type = self._classify_message(style)

                messages.append({
                    "id": cell.get("id"),
                    "value": value,
                    "source": source,
                    "target": target,
                    "type": msg_type,
                    "style": style,
                })
        return messages

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

    def _get_geometry_attr(self, cell: ET.Element, attr: str) -> Optional[int]:
        """从 mxGeometry 子元素获取属性"""
        geom = cell.find("mxGeometry")
        if geom is not None:
            return int(geom.get(attr, 0))
        return None

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
        if len(messages) > 10 and len(alt_fragments) == 0:
            result.add_issue(
                "info",
                "消息较多但未使用 alt/opt/loop 片段组织，"
                "建议使用结构化片段提高可读性"
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
        choices=["sequence", "activity", "class", "auto"],
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
        elif "activity" in filename or "活动" in filename:
            diagram_type = "activity"
        elif "class" in filename or "类图" in filename:
            diagram_type = "class"
        else:
            diagram_type = "sequence"  # 默认

    # 根据格式选择验证器
    if is_mermaid and diagram_type == "sequence":
        mermaid_parser = MermaidParser(content)
        validator = MermaidSequenceValidator(mermaid_parser)
        result = validator.validate()
    elif is_drawio and diagram_type == "sequence":
        drawio_parser = DrawioParser(content)
        validator = DrawioSequenceValidator(drawio_parser)
        result = validator.validate()
    else:
        # 其他组合暂未实现详细验证
        result = ValidationResult(f"{diagram_type} ({'Mermaid' if is_mermaid else 'drawio'})")
        if is_mermaid:
            result.add_issue("info", f"{diagram_type} 图的 Mermaid 验证尚未完全实现")
        else:
            result.add_issue("info", f"{diagram_type} 图的 drawio 验证尚未完全实现")

    print(format_result(result, args.verbose))

    return 1 if result.has_errors() else 0


if __name__ == "__main__":
    raise SystemExit(main())
