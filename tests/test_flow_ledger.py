from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys
from typing import Callable

import pytest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "Arc" / "arc:verify" / "scripts" / "flow_ledger.py"
CATALOG = ROOT / "docs" / "07-测试质量" / "业务流"

README = """# 业务流清单

| ID | 名称 | 类型 | 文件 |
|---|---|---|---|
| FLOW-001 | 下单占库存 | 主业务流 | [FLOW-001](FLOW-001-下单.md) |
| FLOW-002 | 对账 | 辅助业务流 | 未建模 |
"""

FLOW = """# FLOW-001 下单占库存

```mermaid
flowchart TD
    S([开始：用户提交订单]) --> N01{库存是否足够}
    N01 -->|B01 可用库存 ≥ 下单数量| P1[写订单 HELD 并占库存]
    N01 -->|B02 可用库存 < 下单数量| T02([拒绝：库存不足])
    P1 --> T01([正常：订单 HELD])
```

| 终点 | 结果 | 闭环 | 观测点 |
|---|---|---|---|
| FLOW-001-T01 | 订单 HELD，库存占用 | 正常 | 订单一行 HELD；可用库存减少下单数量 |
| FLOW-001-T02 | 拒绝，无写入 | 拒绝 | 订单与库存前后快照一致 |

| 分支 | 条件 | 去向 | 闭环 | 风险 | 用例 | 绑定 | 判定 | 证据 |
|---|---|---|---|---|---|---|---|---|
| FLOW-001-N01-B01 | 可用库存 ≥ 下单数量 | P1 | 正常 | High | C01 | tests/test_order.py::test_hold | PASS | logs/hold.txt |
| FLOW-001-N01-B02 | 可用库存 < 下单数量 | T02 | 拒绝 | Medium | — | — | NOT_RUN (未设计用例) | — |
"""


def _catalog(tmp_path: Path) -> Path:
    catalog = tmp_path / "flows"
    catalog.mkdir()
    (catalog / "README.md").write_text(README, encoding="utf-8")
    (catalog / "FLOW-001-下单.md").write_text(FLOW, encoding="utf-8")
    return catalog


def _run(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        capture_output=True,
        text=True,
        check=False,
    )


def _edit(old: str, new: str) -> Callable[[Path], None]:
    def apply(catalog: Path) -> None:
        path = catalog / "FLOW-001-下单.md"
        text = path.read_text(encoding="utf-8")
        assert old in text, old
        path.write_text(text.replace(old, new), encoding="utf-8")

    return apply


def test_valid_catalog_passes_and_reports_branch_coverage(tmp_path: Path) -> None:
    result = _run("check", str(_catalog(tmp_path)), "--json")

    assert result.returncode == 0, result.stdout + result.stderr
    summary = json.loads(result.stdout.strip().splitlines()[-1])
    flow = summary["flows"]["FLOW-001"]
    assert flow["branches"] == 2
    assert flow["verdicts"] == {"PASS": 1, "NOT_RUN": 1}
    assert flow["designed"] == 1 and flow["bound"] == 1
    assert flow["open_high_risk"] == []
    assert summary["unmodeled"] == ["FLOW-002"]
    assert summary["errors"] == []


@pytest.mark.parametrize(
    ("mutate", "expected"),
    [
        (
            _edit(
                "| FLOW-001-N01-B02 | 可用库存 < 下单数量 | T02 | 拒绝 | Medium | — | — | NOT_RUN (未设计用例) | — |\n",
                "",
            ),
            "FLOW-001-N01-B02 is in the chart but not in the branch table",
        ),
        (
            _edit(
                "    N01 -->|B02 可用库存 < 下单数量| T02([拒绝：库存不足])\n",
                "    S --> T02([拒绝：库存不足])\n",
            ),
            "N01 has fewer than 2 branches",
        ),
        (
            _edit(
                "    P1 --> T01([正常：订单 HELD])\n",
                "    P1 --> P2[发通知]\n    S --> T01([正常：订单 HELD])\n",
            ),
            "P2 cannot reach a terminal",
        ),
        (
            _edit("| PASS | logs/hold.txt |", "| PASS | — |"),
            "FLOW-001-N01-B01: PASS needs case, binding and evidence",
        ),
        (
            _edit("NOT_RUN (未设计用例)", "NOT_RUN"),
            "FLOW-001-N01-B02: NOT_RUN needs a reason",
        ),
        (
            _edit("|B01 可用库存 ≥ 下单数量|", "|B01 是|"),
            "N01-B01 condition must name the business condition",
        ),
        (
            _edit(
                "    P1 --> T01([正常：订单 HELD])\n",
                "    P1 --> T01([正常：订单 HELD])\n    T01 --> P1\n",
            ),
            "terminal T01 has an outgoing edge",
        ),
        (
            _edit(
                "| FLOW-001-N01-B02 | 可用库存 < 下单数量 | T02 |",
                "| FLOW-001-N01-B02 | 可用库存 < 下单数量 | P1 |",
            ),
            "FLOW-001-N01-B02: target P1 differs from the chart edge to T02",
        ),
        (
            _edit(
                "    P1 --> T01([正常：订单 HELD])\n",
                "    P1 -- 成功 --> T01([正常：订单 HELD])\n",
            ),
            "unsupported Mermaid syntax",
        ),
        (
            _edit(
                "    P1 --> T01([正常：订单 HELD])\n",
                "    P1 --> T01([正常：订单 HELD])\n    P1 --> T03\n",
            ),
            "node T03 is used but never defined",
        ),
        (
            _edit(
                "| FLOW-001-T02 | 拒绝，无写入 | 拒绝 |",
                "| FLOW-001-T02 | 拒绝，无写入 | 搞定 |",
            ),
            "FLOW-001-T02: closure 搞定 is not one of",
        ),
        (
            _edit("| PASS | logs/hold.txt |", "| GREEN | logs/hold.txt |"),
            "FLOW-001-N01-B01: verdict GREEN is not one of",
        ),
    ],
)
def test_each_structural_defect_fails_the_check(
    tmp_path: Path, mutate: Callable[[Path], None], expected: str
) -> None:
    catalog = _catalog(tmp_path)
    mutate(catalog)

    result = _run("check", str(catalog))

    assert result.returncode == 1, result.stdout + result.stderr
    assert expected in result.stdout


def test_readme_and_flow_files_must_match(tmp_path: Path) -> None:
    catalog = _catalog(tmp_path)
    (catalog / "FLOW-003-退款.md").write_text(
        FLOW.replace("FLOW-001", "FLOW-003"), encoding="utf-8"
    )
    readme = catalog / "README.md"
    readme.write_text(
        readme.read_text(encoding="utf-8").replace(
            "FLOW-001-下单.md", "FLOW-001-missing.md"
        ),
        encoding="utf-8",
    )

    result = _run("check", str(catalog))

    assert result.returncode == 1
    assert "README links FLOW-001-missing.md, which does not exist" in result.stdout
    assert "FLOW-003-退款.md is not linked from README.md" in result.stdout


def test_gap_terminals_and_open_high_risk_branches_are_reported_without_failing(
    tmp_path: Path,
) -> None:
    catalog = _catalog(tmp_path)
    _edit(
        "| FLOW-001-T02 | 拒绝，无写入 | 拒绝 |",
        "| FLOW-001-T02 | 拒绝，无写入 | 缺口 |",
    )(catalog)
    _edit("| T02 | 拒绝 | Medium |", "| T02 | 缺口 | High |")(catalog)

    result = _run("check", str(catalog), "--json")

    assert result.returncode == 0, result.stdout
    flow = json.loads(result.stdout.strip().splitlines()[-1])["flows"]["FLOW-001"]
    assert flow["gap_terminals"] == ["FLOW-001-T02"]
    assert flow["open_high_risk"] == ["FLOW-001-N01-B02"]


def test_missing_catalog_is_a_usage_error(tmp_path: Path) -> None:
    result = _run("check", str(tmp_path / "absent"))

    assert result.returncode == 2
    assert "catalog directory" in result.stderr


def test_repository_flow_catalog_passes() -> None:
    result = _run("check", str(CATALOG))

    assert result.returncode == 0, result.stdout + result.stderr
