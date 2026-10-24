"""Offline checks for scripts/route_probe.py; a fake `jev` replays fixed answers."""

import importlib.util
import json
from pathlib import Path
import stat

ROOT = Path(__file__).resolve().parents[1]


def _module():
    spec = importlib.util.spec_from_file_location("route_probe", ROOT / "scripts" / "route_probe.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _fake_jev(tmp_path: Path, answers: dict) -> str:
    script = tmp_path / "jev"
    script.write_text("#!/bin/sh\ncat > /dev/null\nprintf '%s' '" + json.dumps({"answers": answers}) + "'\n")
    script.chmod(script.stat().st_mode | stat.S_IEXEC)
    return str(script)


def test_probe_scores_expected_lists_and_confidence(tmp_path: Path) -> None:
    module = _module()
    roster = {"arc-code": "changes code", "ui-design": "visual direction", "check": "delivery"}
    answers = {
        "q0": {"choice": "ui-design", "confidence": 0.8, "probabilities": {"ui-design": 0.86, "arc-code": 0.14}},
        "q1": {"choice": "check", "confidence": 0.3, "probabilities": {"check": 0.4, "arc-code": 0.35, "ui-design": 0.25}},
        "q2": {"choice": "arc-code", "confidence": 0.9, "probabilities": {"arc-code": 0.95, "check": 0.05}},
    }
    cases = [
        {"utterance": "landing page", "expect": ["arc-code", "ui-design"]},
        {"utterance": "commit it", "expect": "check"},
        {"utterance": "fix it", "expect": "check"},
    ]
    rows = module.probe(cases, roster, _fake_jev(tmp_path, answers))
    assert [row["status"] for row in rows] == ["PASS", "FAIL", "FAIL"]
    assert rows[0]["top3"][0] == ("ui-design", 0.86)


def test_roster_excludes_library_entries() -> None:
    module = _module()
    roster = module.load_roster(Path("/nonexistent"), [])
    assert {"arc-code", "lark", "android", "cnb", "research-write"} <= set(roster)
    assert not {"lark-doc", "perfetto-sql", "arc-closure", "cnb-api", "read"} & set(roster)
