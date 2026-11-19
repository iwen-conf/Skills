from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]
SKILLS = ["explore", "plan", "code", "verify", "audit", "vet"]


def test_each_arc_skill_is_slim_and_uses_dash_naming():
    for skill in SKILLS:
        path = ROOT / "Arc" / f"arc-{skill}" / "SKILL.md"
        assert path.is_file(), f"missing {path}"
        content = path.read_text(encoding="utf-8")
        assert f"name: arc-{skill}" in content
        lines = content.splitlines()
        assert len(lines) <= 105, f"{path} has {len(lines)} lines (expected <= 105)"


def test_workflow_enforces_write_set_dag_and_storyline_verification():
    workflow = (ROOT / "Arc/_shared/workflow.md").read_text(encoding="utf-8")
    assert "plan-dag.yaml" in workflow
    assert "write_scope" in workflow
    assert "frozen_contracts" in workflow
    assert "arc verify scan-world" in workflow
    assert "arc verify completion" in workflow
    assert "arc-code" in workflow and "arc-verify" in workflow


def test_registry_matches_skill_versions_and_line_counts():
    index = json.loads((ROOT / "skills.index.json").read_text(encoding="utf-8"))
    assert index["skill_count"] == 6
    for item in index["skills"]:
        assert item["name"].startswith("arc-")
        path = ROOT / item["source_path"]
        content = path.read_text(encoding="utf-8")
        assert f"version: {item['version']}" in content.split("---", 2)[1]
        assert item["line_count"] == len(content.splitlines())
