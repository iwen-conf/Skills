import subprocess
import sys
from pathlib import Path


ROOT = Path("/Users/iluwen/Documents/Code/Skills")
SCAFFOLD_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "scaffold_uml_pack.py"
VALIDATE_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "validate_diagram.py"
GENERATE_DEPLOYMENT_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "generate_deployment_drawio.py"
RENDER_SEQUENCE_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "render_beautiful_mermaid_svg.mjs"
GENERATE_SEQUENCE_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "generate_sequence_drawio.py"
DRAFT_DEPLOYMENT_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "draft_deployment_spec.py"
REVIEW_UML_PACK_SCRIPT = ROOT / "Arc" / "arc:uml" / "scripts" / "review_uml_pack.py"


def test_scaffold_uml_pack_places_sequence_mermaid_in_diagrams_dir(tmp_path: Path) -> None:
    output_dir = tmp_path / "uml-pack"

    result = subprocess.run(
        [
            sys.executable,
            str(SCAFFOLD_SCRIPT),
            "--output-dir",
            str(output_dir),
            "--types",
            "sequence,deployment",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    assert (output_dir / "diagrams" / "sequence.mmd").exists()
    assert (output_dir / "diagram-briefs" / "sequence.md").exists()
    assert not (output_dir / "diagram-briefs" / "sequence.mmd").exists()
    assert not (output_dir / "diagrams" / "sequence.drawio").exists()
    assert (output_dir / "diagram-specs" / "deployment.json").exists()
    assert (output_dir / "diagram-specs" / "deployment.seed.txt").exists()

    index_text = (output_dir / "diagram-index.md").read_text(encoding="utf-8")
    assert "`diagrams/sequence.mmd`" in index_text


def test_validate_deployment_rejects_floating_edges(tmp_path: Path) -> None:
    diagram_path = tmp_path / "deployment.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-15T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="deployment" name="Deployment">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="client" value="客户端" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="80" y="80" width="140" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="app" value="应用服务" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="320" y="80" width="160" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="edge-1" value="HTTPS" style="edgeStyle=orthogonalEdgeStyle;orthogonalLoop=1;jettySize=auto;html=1;endArrow=block;" edge="1" parent="1">
          <mxGeometry relative="1" as="geometry">
            <mxPoint x="220" y="110" as="sourcePoint"/>
            <mxPoint x="320" y="110" as="targetPoint"/>
          </mxGeometry>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "deployment",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1
    assert "漂浮连接线" in result.stdout


def test_validate_deployment_accepts_connected_orthogonal_edges(tmp_path: Path) -> None:
    diagram_path = tmp_path / "deployment.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-15T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="deployment" name="Deployment">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="client" value="客户端" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="80" y="80" width="140" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="app" value="应用服务" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="320" y="80" width="160" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="edge-1" value="HTTPS" style="edgeStyle=orthogonalEdgeStyle;orthogonalLoop=1;jettySize=auto;html=1;endArrow=block;" edge="1" source="client" target="app" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "deployment",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stdout + result.stderr
    assert "验证失败" not in result.stdout


def test_generate_deployment_drawio_produces_valid_bound_edges(tmp_path: Path) -> None:
    spec_path = tmp_path / "deployment.json"
    output_path = tmp_path / "deployment.drawio"
    spec_path.write_text(
        """{
  "title": "订单服务部署图",
  "zones": [
    {
      "id": "client",
      "label": "客户端区",
      "nodes": [
        {
          "id": "frontend",
          "label": "前端页面",
          "kind": "client",
          "subtitle": "React"
        }
      ]
    },
    {
      "id": "app",
      "label": "应用层",
      "nodes": [
        {
          "id": "orders",
          "label": "Orders Service",
          "kind": "service"
        },
        {
          "id": "gateway",
          "label": "支付网关",
          "kind": "gateway"
        }
      ]
    },
    {
      "id": "data",
      "label": "数据层",
      "nodes": [
        {
          "id": "mysql",
          "label": "MySQL",
          "kind": "database"
        }
      ]
    }
  ],
  "edges": [
    {
      "source": "frontend",
      "target": "orders",
      "label": "HTTPS"
    },
    {
      "source": "orders",
      "target": "gateway",
      "label": "支付请求"
    },
    {
      "source": "orders",
      "target": "mysql",
      "label": "订单读写"
    }
  ]
}
""",
        encoding="utf-8",
    )

    generate = subprocess.run(
        [
            sys.executable,
            str(GENERATE_DEPLOYMENT_SCRIPT),
            "--spec",
            str(spec_path),
            "--output",
            str(output_path),
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert generate.returncode == 0, generate.stderr
    assert output_path.exists()

    validate = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(output_path),
            "--type",
            "deployment",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert validate.returncode == 0, validate.stdout + validate.stderr
    assert "漂浮连接线" not in validate.stdout


def test_draft_deployment_spec_from_seed(tmp_path: Path) -> None:
    seed_path = tmp_path / "deployment.seed.txt"
    output_path = tmp_path / "deployment.json"
    seed_path.write_text(
        """title: 支付订单部署图

zone client | 客户端区
node frontend | 前端页面 | client | React

zone app | 应用层
node orders | Orders Service | service
node gateway | 支付网关 | gateway | 支付接入

zone data | 数据层
node mysql | MySQL | database

edge frontend -> orders | HTTPS
edge orders -> gateway | 支付请求
edge orders -> mysql | 订单读写
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(DRAFT_DEPLOYMENT_SCRIPT),
            "--input",
            str(seed_path),
            "--output",
            str(output_path),
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    payload = output_path.read_text(encoding="utf-8")
    assert "\"title\": \"支付订单部署图\"" in payload
    assert "\"id\": \"client\"" in payload
    assert "\"source\": \"frontend\"" in payload


def test_render_sequence_script_has_cli_entrypoint() -> None:
    result = subprocess.run(
        [
            "node",
            str(RENDER_SEQUENCE_SCRIPT),
            "--help",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    assert "Usage:" in result.stdout


def test_generate_sequence_drawio_wraps_mermaid_and_validates(tmp_path: Path) -> None:
    mmd_path = tmp_path / "sequence.mmd"
    svg_path = tmp_path / "sequence.svg"
    output_path = tmp_path / "sequence.drawio"

    mmd_path.write_text(
        """sequenceDiagram
    participant User as 用户
    participant Frontend as 前端页面
    participant Service as Orders Service

    User->>Frontend: 发起下单
    activate Frontend
    Frontend->>Service: 创建订单
    activate Service
    Service-->>Frontend: 返回订单结果
    deactivate Service
    Frontend-->>User: 展示支付结果
    deactivate Frontend
""",
        encoding="utf-8",
    )

    svg_path.write_text(
        """<svg xmlns="http://www.w3.org/2000/svg" width="960" height="640" viewBox="0 0 960 640">
  <rect width="960" height="640" fill="white"/>
  <text x="40" y="80" font-size="28">Sequence</text>
</svg>
""",
        encoding="utf-8",
    )

    generate = subprocess.run(
        [
            sys.executable,
            str(GENERATE_SEQUENCE_SCRIPT),
            "--input",
            str(mmd_path),
            "--svg",
            str(svg_path),
            "--output",
            str(output_path),
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert generate.returncode == 0, generate.stderr
    assert output_path.exists()

    validate = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(output_path),
            "--type",
            "sequence",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert validate.returncode == 0, validate.stdout + validate.stderr
    assert "drawio embedded Mermaid" in validate.stdout


def test_validate_sequence_requires_alt_for_two_different_cases(tmp_path: Path) -> None:
    mmd_path = tmp_path / "sequence.mmd"
    mmd_path.write_text(
        """sequenceDiagram
    participant User as 用户
    participant Service as 服务

    User->>Service: 发起请求
    Service-->>User: 支付成功
    Service-->>User: 支付失败
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(mmd_path),
            "--type",
            "sequence",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1
    assert "alt 组合片段矩形框" in result.stdout


def test_validate_sequence_accepts_alt_frame_for_two_cases(tmp_path: Path) -> None:
    mmd_path = tmp_path / "sequence.mmd"
    mmd_path.write_text(
        """sequenceDiagram
    participant User as 用户
    participant Service as 服务

    User->>Service: 发起请求
    alt 支付成功
        Service-->>User: 支付成功
    else 支付失败
        Service-->>User: 支付失败
    end
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(mmd_path),
            "--type",
            "sequence",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stdout + result.stderr
    assert "alt 组合片段矩形框" not in result.stdout


def test_validate_activity_requires_initial_and_final_nodes(tmp_path: Path) -> None:
    diagram_path = tmp_path / "activity.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="activity" name="Activity">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="action-1" value="提交订单" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="120" y="120" width="160" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="flow-1" value="" style="endArrow=block;html=1;" edge="1" source="action-1" target="action-1" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "activity",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1
    assert "缺少初始节点" in result.stdout
    assert "缺少结束节点" in result.stdout


def test_validate_activity_accepts_basic_uml_activity_elements(tmp_path: Path) -> None:
    diagram_path = tmp_path / "activity.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="activity" name="Activity">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="start" value="" style="ellipse;fillColor=#000000;strokeColor=#000000;" vertex="1" parent="1">
          <mxGeometry x="120" y="80" width="20" height="20" as="geometry"/>
        </mxCell>
        <mxCell id="action-1" value="提交订单" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="80" y="140" width="160" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="end" value="" style="ellipse;double=1;fillColor=#ffffff;strokeColor=#000000;" vertex="1" parent="1">
          <mxGeometry x="120" y="240" width="24" height="24" as="geometry"/>
        </mxCell>
        <mxCell id="flow-1" value="" style="endArrow=block;html=1;" edge="1" source="start" target="action-1" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
        <mxCell id="flow-2" value="" style="endArrow=block;html=1;" edge="1" source="action-1" target="end" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "activity",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stdout + result.stderr


def test_validate_class_warns_on_non_standard_member_visibility(tmp_path: Path) -> None:
    diagram_path = tmp_path / "class.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="class" name="Class">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="order-class" value="Order&lt;hr&gt;id: String&lt;br&gt;create(): void" style="swimlane;childLayout=stackLayout;horizontal=0;startSize=26;" vertex="1" parent="1">
          <mxGeometry x="120" y="120" width="220" height="140" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "class",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0
    assert "可见性记号" in result.stdout


def test_validate_use_case_requires_boundary_and_actor_positions(tmp_path: Path) -> None:
    diagram_path = tmp_path / "use-case.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="use-case" name="UseCase">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="actor-1" value="客户" style="shape=umlActor;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="120" y="120" width="60" height="120" as="geometry"/>
        </mxCell>
        <mxCell id="use-1" value="提交订单" style="ellipse;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="260" y="140" width="150" height="70" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "use-case",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1
    assert "缺少系统边界" in result.stdout


def test_validate_use_case_accepts_basic_uml_use_case_layout(tmp_path: Path) -> None:
    diagram_path = tmp_path / "use-case.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="use-case" name="UseCase">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="actor-1" value="客户" style="shape=umlActor;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="80" y="180" width="60" height="120" as="geometry"/>
        </mxCell>
        <mxCell id="system" value="订单系统" style="swimlane;horizontal=0;startSize=32;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="220" y="100" width="420" height="280" as="geometry"/>
        </mxCell>
        <mxCell id="use-1" value="提交订单" style="ellipse;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="320" y="140" width="150" height="70" as="geometry"/>
        </mxCell>
        <mxCell id="use-2" value="支付订单" style="ellipse;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="320" y="240" width="150" height="70" as="geometry"/>
        </mxCell>
        <mxCell id="assoc-1" value="" style="endArrow=none;html=1;" edge="1" source="actor-1" target="use-1" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "use-case",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stdout + result.stderr


def test_validate_state_machine_requires_initial_state(tmp_path: Path) -> None:
    diagram_path = tmp_path / "state-machine.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="state-machine" name="StateMachine">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="state-1" value="待支付" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="160" y="140" width="140" height="60" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "state-machine",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1
    assert "缺少初始状态" in result.stdout


def test_validate_state_machine_accepts_basic_uml_states(tmp_path: Path) -> None:
    diagram_path = tmp_path / "state-machine.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="state-machine" name="StateMachine">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="start" value="" style="ellipse;fillColor=#000000;strokeColor=#000000;" vertex="1" parent="1">
          <mxGeometry x="140" y="90" width="20" height="20" as="geometry"/>
        </mxCell>
        <mxCell id="state-1" value="待支付" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="100" y="140" width="140" height="60" as="geometry"/>
        </mxCell>
        <mxCell id="end" value="" style="ellipse;double=1;fillColor=#ffffff;strokeColor=#000000;" vertex="1" parent="1">
          <mxGeometry x="140" y="260" width="24" height="24" as="geometry"/>
        </mxCell>
        <mxCell id="transition-1" value="支付成功" style="endArrow=block;html=1;" edge="1" source="start" target="state-1" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
        <mxCell id="transition-2" value="订单完成" style="endArrow=block;html=1;" edge="1" source="state-1" target="end" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "state-machine",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stdout + result.stderr


def test_validate_component_requires_standard_component_notation(tmp_path: Path) -> None:
    diagram_path = tmp_path / "component.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="component" name="Component">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="box-1" value="Orders Service" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="120" y="120" width="200" height="80" as="geometry"/>
        </mxCell>
        <mxCell id="box-2" value="Payment Service" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="420" y="120" width="200" height="80" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "component",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1
    assert "标准组件元素" in result.stdout


def test_validate_component_accepts_basic_component_dependency(tmp_path: Path) -> None:
    diagram_path = tmp_path / "component.drawio"
    diagram_path.write_text(
        """<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="app.diagrams.net" modified="2026-04-16T00:00:00.000Z" agent="Codex" version="24.7.17">
  <diagram id="component" name="Component">
    <mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="900" math="0" shadow="0">
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="component-1" value="&lt;&lt;component&gt;&gt;&lt;br&gt;Orders Service" style="shape=component;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="120" y="120" width="220" height="90" as="geometry"/>
        </mxCell>
        <mxCell id="component-2" value="&lt;&lt;component&gt;&gt;&lt;br&gt;Payment Service" style="shape=component;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="420" y="120" width="220" height="90" as="geometry"/>
        </mxCell>
        <mxCell id="dep-1" value="调用支付接口" style="dashed=1;endArrow=open;html=1;" edge="1" source="component-1" target="component-2" parent="1">
          <mxGeometry relative="1" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
""",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            str(VALIDATE_SCRIPT),
            str(diagram_path),
            "--type",
            "component",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0, result.stdout + result.stderr


def test_review_uml_pack_passes_when_sources_and_derived_outputs_are_in_sync(tmp_path: Path) -> None:
    pack_dir = tmp_path / "uml-pack"

    scaffold = subprocess.run(
        [
            sys.executable,
            str(SCAFFOLD_SCRIPT),
            "--output-dir",
            str(pack_dir),
            "--types",
            "sequence,deployment",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert scaffold.returncode == 0, scaffold.stderr

    draft = subprocess.run(
        [
            sys.executable,
            str(DRAFT_DEPLOYMENT_SCRIPT),
            "--input",
            str(pack_dir / "diagram-specs" / "deployment.seed.txt"),
            "--output",
            str(pack_dir / "diagram-specs" / "deployment.json"),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert draft.returncode == 0, draft.stderr

    deployment = subprocess.run(
        [
            sys.executable,
            str(GENERATE_DEPLOYMENT_SCRIPT),
            "--spec",
            str(pack_dir / "diagram-specs" / "deployment.json"),
            "--output",
            str(pack_dir / "diagrams" / "deployment.drawio"),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert deployment.returncode == 0, deployment.stderr

    sequence = subprocess.run(
        [
            sys.executable,
            str(GENERATE_SEQUENCE_SCRIPT),
            "--input",
            str(pack_dir / "diagrams" / "sequence.mmd"),
            "--output",
            str(pack_dir / "diagrams" / "sequence.drawio"),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert sequence.returncode == 0, sequence.stderr

    review = subprocess.run(
        [
            sys.executable,
            str(REVIEW_UML_PACK_SCRIPT),
            "--uml-dir",
            str(pack_dir),
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert review.returncode == 0, review.stdout + review.stderr
    summary = (pack_dir / "validation-summary.md").read_text(encoding="utf-8")
    assert "所有复核通过" in summary


def test_review_uml_pack_strict_warnings_fails_on_missing_derived_outputs(tmp_path: Path) -> None:
    pack_dir = tmp_path / "uml-pack"

    scaffold = subprocess.run(
        [
            sys.executable,
            str(SCAFFOLD_SCRIPT),
            "--output-dir",
            str(pack_dir),
            "--types",
            "sequence,deployment",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert scaffold.returncode == 0, scaffold.stderr

    review = subprocess.run(
        [
            sys.executable,
            str(REVIEW_UML_PACK_SCRIPT),
            "--uml-dir",
            str(pack_dir),
            "--strict-warnings",
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert review.returncode == 1
    summary = (pack_dir / "validation-summary.md").read_text(encoding="utf-8")
    assert "缺失" in summary
