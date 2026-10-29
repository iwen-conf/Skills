from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]
SKILLS = ["explore", "plan", "code", "verify", "audit", "vet"]


def test_each_existing_arc_skill_links_shared_graph_handoff():
    for skill in SKILLS:
        content = (ROOT / "Arc" / f"arc:{skill}" / "SKILL.md").read_text()
        assert "workflow.md" in content
        assert f"arc-verify handoff --to arc-{skill}" in content


def test_graph_bridge_preserves_single_owner_and_risk_boundary():
    workflow = (ROOT / "Arc/_shared/workflow.md").read_text()
    assert "## 6. Graph-based business-chain verification" in workflow
    assert "RISK" in workflow and "user-approved" in workflow
    assert "existing handoff record" in workflow or "existing handoff" in workflow
    assert "Turso Cloud" in workflow
    assert "arc:code" in workflow and "arc:verify" in workflow


def test_registry_matches_skill_versions_and_line_counts():
    index = json.loads((ROOT / "skills.index.json").read_text())
    assert index["skill_count"] == 6
    for item in index["skills"]:
        path = ROOT / item["source_path"]
        content = path.read_text()
        assert f"version: {item['version']}" in content.split("---", 2)[1]
        assert item["line_count"] == len(content.splitlines())
