"""Read-only diagrams must preserve business evidence and renderer provenance."""

import hashlib
import json
import re
import shutil
from copy import deepcopy
from pathlib import Path

import pytest
from arc_policy import diagram

REPO = Path(__file__).resolve().parents[1]
SAMPLE = REPO / "tools/arc-verify/workbench/dev/sample-snapshot.json"
RENDERER = REPO / "plugins/arc-policy/skills/arch-diagram"


@pytest.fixture
def sample():
    return json.loads(SAMPLE.read_text(encoding="utf-8"))


def candidate_snapshot():
    nodes = [
        {"id": "candidate.system", "kind": "system", "name": "候选系统"},
        {"id": "candidate.domain", "kind": "domain", "name": "报名业务"},
        {"id": "candidate.enroll", "kind": "capability", "name": "提交报名"},
    ]
    edges = [
        {
            "id": "contains.domain",
            "from": "candidate.system",
            "to": "candidate.domain",
            "kind": "contains",
        },
        {
            "id": "contains.capability",
            "from": "candidate.domain",
            "to": "candidate.enroll",
            "kind": "contains",
        },
    ]
    graph = {
        "id": "candidate-v2",
        "schema_version": 2,
        "system": "candidate.system",
        "scope": "Synthetic candidate fixture; not a product contract",
        "nodes": nodes,
        "edges": edges,
        "rules": None,
        "risks": None,
    }
    return {
        "meta": {
            "graph": graph["id"],
            "graph_sha256": "a" * 64,
            "generated": "2026-11-04T00:00:00Z",
            "version": "test",
            "argv": [],
        },
        "model": {"data": graph, "approved": False, "rules": {}, "sha256": "a" * 64},
        "views": {
            "capabilities": {
                "graph": graph["id"],
                "view": "capabilities",
                "nodes": [
                    dict(
                        node, status="NOT_RUN", history_failures=0, passed=0, required=0
                    )
                    for node in nodes
                ],
                "edges": edges,
                "verdict": "BLOCKED",
                "reason": "catalog needs user approval",
                "context": {},
                "history_failures": 0,
                "total": 3,
                "truncated": False,
            },
        },
        "runs": [],
    }


def presentation_for(snapshot):
    return {
        "graph": snapshot["meta"]["graph"],
        "graph_sha256": snapshot["meta"]["graph_sha256"],
        "nodes": {},
    }


def visible_text(spec):
    displayed = {
        key: spec[key]
        for key in ("title", "subtitle", "nodes", "edges", "notes", "footer")
    }
    return json.dumps(displayed, ensure_ascii=False)


def test_real_sample_keeps_failed_attempt_and_passing_retry_separate(sample):
    original = deepcopy(sample)
    spec = diagram.snapshot_to_spec(sample)
    assert sample == original  # conversion has no mutation or execution side effect
    assert spec["arc"]["business_verdict"] == "BLOCKED"
    assert spec["arc"]["history_failures"] == 1
    assert [run["status"] for run in spec["arc"]["runs"]] == ["FAIL", "PASS"]
    failed = spec["arc"]["runs"][0]
    assert failed["first_divergence"] == "1"
    assert failed["evidence_status"] == "PASS"  # RAW validity is not business success
    assert failed["steps"][0]["verdict"] == "FAIL"
    assert failed["evidence"][1]["sha256"] == sample["runs"][0]["evidence"][1]["sha256"]
    assert spec["arc"]["node_status"]["enrollment.submit"]["status"] == "FLAKY"
    assert all(node["status"] != "done" for node in spec["nodes"])
    assert spec["arc"]["fixture"] is True
    assert "工具示例" in visible_text(spec)
    assert sample["model"]["data"]["scope"] in visible_text(spec)
    assert "证据完整性 PASS 不等于业务 PASS" in visible_text(spec)
    assert "log" not in spec
    assert all(edge["packet"] is False for edge in spec["edges"])


def test_candidate_nil_arrays_need_no_rule_environment_or_runner():
    spec = diagram.snapshot_to_spec(candidate_snapshot(), view="capabilities")
    assert spec["arc"]["model_approved"] is False
    assert spec["arc"]["rules"] == []
    assert spec["arc"]["risks"] == []
    assert spec["arc"]["runs"] == []
    assert len(spec["arc"]["candidate_nodes"]) == 3
    assert all(node["status"] == "none" for node in spec["nodes"])
    assert all(
        "【业务包待核实】" in node["title"].replace("\n", "") for node in spec["nodes"]
    )
    assert "环境：未设置" in visible_text(spec)


def test_source_ids_with_dots_and_parallel_canonical_edges_are_not_lost(sample):
    source_edge = deepcopy(sample["model"]["data"]["edges"][2])
    source_edge["id"] = "same.relationship.other.source"
    sample["model"]["data"]["edges"].append(source_edge)
    spec = diagram.snapshot_to_spec(sample)
    assert source_edge["id"] in spec["arc"]["edge_ids"].values()
    assert "implements-enroll" in spec["arc"]["edge_ids"].values()
    assert "enrollment.submit" in spec["arc"]["node_ids"].values()
    assert all(re.fullmatch(r"[\w-]+", node["id"]) for node in spec["nodes"])
    for edge in spec["edges"]:
        assert edge["from"].split(".", 1)[0] in spec["arc"]["node_ids"]
        assert edge["to"].split(".", 1)[0] in spec["arc"]["node_ids"]
        assert re.search(r"[\u3400-\u9fff]", edge["label"])


def test_large_or_truncated_scope_requires_explicit_selection():
    snapshot = candidate_snapshot()
    for number in range(15):
        node = {
            "id": f"extra.{number}",
            "kind": "capability",
            "name": f"候选能力 {number}",
        }
        snapshot["model"]["data"]["nodes"].append(node)
    with pytest.raises(diagram.DiagramError, match="node_ids"):
        diagram.snapshot_to_spec(snapshot, view="capabilities")
    labels = presentation_for(snapshot)
    labels["node_ids"] = ["candidate.system", "candidate.domain", "candidate.enroll"]
    spec = diagram.snapshot_to_spec(snapshot, labels, "capabilities")
    assert len(spec["arc"]["omitted"]["nodes"]) == 15
    assert "省略 15 个节点" in visible_text(spec)
    snapshot["views"]["capabilities"]["truncated"] = True
    snapshot["model"]["data"]["nodes"] = snapshot["model"]["data"]["nodes"][:3]
    with pytest.raises(diagram.DiagramError, match="truncated"):
        diagram.snapshot_to_spec(snapshot, view="capabilities")
    spec = diagram.snapshot_to_spec(snapshot, labels, "capabilities")
    assert spec["arc"]["omitted"]["projection_truncated"] is True
    assert "来源投影已截断" in visible_text(spec)


@pytest.mark.parametrize("field", ["environment", "round"])
def test_snapshot_projection_context_mismatch_is_refused(sample, field):
    sample["views"]["flows"][field] = "different-source"
    with pytest.raises(diagram.DiagramError, match=field):
        diagram.snapshot_to_spec(sample)


def test_frozen_old_graph_context_is_visible_but_cannot_carry_pass(sample):
    sample["views"]["flows"]["context"]["graph_sha256"] = "f" * 64
    spec = diagram.snapshot_to_spec(sample)
    assert "上下文已过期" in visible_text(spec)
    assert "f" * 64 in visible_text(spec)
    sample["views"]["flows"]["nodes"][0]["status"] = "PASS"
    with pytest.raises(diagram.DiagramError, match="different graph fingerprint"):
        diagram.snapshot_to_spec(sample)


@pytest.mark.parametrize(
    "damage", ["graph", "hash", "approval", "node-status", "duplicate-node"]
)
def test_inconsistent_source_is_refused(sample, damage):
    if damage == "graph":
        sample["meta"]["graph"] = "another-graph"
    elif damage == "hash":
        sample["model"]["sha256"] = "b" * 64
    elif damage == "approval":
        sample["model"]["approved"] = False
        sample["views"]["flows"]["verdict"] = "PASS"
    elif damage == "node-status":
        sample["views"]["flows"]["nodes"][0]["status"] = "GUESSED_PASS"
    else:
        sample["model"]["data"]["nodes"].append(sample["model"]["data"]["nodes"][0])
    with pytest.raises(diagram.DiagramError):
        diagram.snapshot_to_spec(sample)


def test_presentation_can_neither_change_verdict_nor_supply_unbound_labels(sample):
    labels = presentation_for(sample)
    labels["nodes"]["enroll"] = {"name_zh": "创建报名记录", "status": "PASS"}
    with pytest.raises(diagram.DiagramError, match="source facts or verdicts"):
        diagram.snapshot_to_spec(sample, labels)
    labels["nodes"]["enroll"].pop("status")
    labels["graph_sha256"] = "c" * 64
    with pytest.raises(diagram.DiagramError, match="does not match"):
        diagram.snapshot_to_spec(sample, labels)


def test_or_guard_negation_and_constraints_are_visible_without_becoming_and(sample):
    graph = sample["model"]["data"]
    graph["guards"] = [
        {
            "rule": "R1",
            "any_of": [
                [{"condition": "login", "value": False}],
                [{"condition": "login", "value": True}],
            ],
        }
    ]
    graph["constraints"] = [
        {
            "id": "C1",
            "literals": [{"condition": "login", "value": False}],
            "source": "document",
            "source_ref": "fixture",
        }
    ]
    spec = diagram.snapshot_to_spec(sample)
    text = visible_text(spec)
    assert "（已认证=不满足） 或 （已认证=满足）" in text
    assert "约束 C1" in text
    assert spec["arc"]["business_logic"]["guards"] == graph["guards"]
    assert spec["arc"]["business_logic"]["invariants"] == graph["invariants"]
    assert spec["arc"]["business_logic"]["tests"][0]["id"] == "test.idempotency"


def test_exploration_risk_and_raw_errors_stay_visible(sample):
    sample["exploration"] = {
        "flow.enrollment": {
            "business_verdict": "RISK",
            "evidence_trust": "UNTRUSTED",
            "trusted": False,
            "ai_inferred": ["H1"],
            "human_confirmed": [],
            "ai_candidates": ["C1"],
            "unspecified": ["U1"],
            "confirmed_fails": [],
            "first_divergence": "step-2",
        }
    }
    sample["runs"][0]["evidence"][1] = {
        "run": "run-id",
        "phase": "action",
        "step": 1,
        "error": "RAW hash changed",
    }
    spec = diagram.snapshot_to_spec(sample)
    text = visible_text(spec)
    assert "RISK" in text and "AI 推断 1 项" in text
    assert "UNSPECIFIED 未定义 1 项" in text
    assert "人工确认 0 项" in text
    assert "探索首次偏离：step-2" in text
    assert "RAW 校验错误 1 项" in text
    assert spec["arc"]["runs"][0]["evidence"][1]["error"] == "RAW hash changed"


def test_package_evidence_verifies_packages_not_arbitrary_api_tokens(tmp_path, sample):
    directory = tmp_path / "enrollment"
    directory.mkdir()
    source = directory / "handler.js"
    source.write_text(
        "export function cancelEnrollment() { return true; }\n", encoding="utf-8"
    )
    labels = presentation_for(sample)
    node = labels["nodes"]["enrollment.submit"] = {
        "name_zh": "提交报名",
        "packages": [
            {"name": "cancelEnrollment", "file": "enrollment/handler.js", "line": 1}
        ],
    }
    diagram.snapshot_to_spec(sample, labels)
    facts, warnings = diagram._package_facts(labels, tmp_path)
    assert not facts and warnings
    node["packages"][0]["name"] = "enrollment"
    facts, warnings = diagram._package_facts(labels, tmp_path)
    assert not warnings
    assert facts["enrollment.submit"][0]["kind"] == "directory"
    spec = diagram.snapshot_to_spec(sample, labels, package_facts=facts)
    rendered = next(
        node for node in spec["nodes"] if node["sourceId"] == "enrollment.submit"
    )
    assert "提交报名【enrollment】" in rendered["title"]
    assert "line_text" not in json.dumps(spec)
    assert "cancelEnrollment" not in json.dumps(spec)


def test_single_line_manifest_does_not_copy_unrelated_settings(tmp_path, sample):
    (tmp_path / "package.json").write_text(
        json.dumps(
            {"name": "@example/enrollment", "config": {"apiToken": "PRIVATE_SENTINEL"}}
        ),
        encoding="utf-8",
    )
    labels = presentation_for(sample)
    labels["nodes"]["enrollment.submit"] = {
        "packages": [{"name": "@example/enrollment", "file": "package.json", "line": 1}]
    }
    facts, warnings = diagram._package_facts(labels, tmp_path)
    assert facts and not warnings
    spec = diagram.snapshot_to_spec(sample, labels, package_facts=facts)
    assert "PRIVATE_SENTINEL" not in json.dumps(spec)
    assert (
        facts["enrollment.submit"][0]["file_sha256"]
        == hashlib.sha256((tmp_path / "package.json").read_bytes()).hexdigest()
    )


def test_package_declaration_and_outside_root_are_checked(tmp_path, sample):
    (tmp_path / "handler.go").write_text("package enrollment\n", encoding="utf-8")
    labels = presentation_for(sample)
    item = {"name": "enrollment", "file": "handler.go", "line": 1}
    labels["nodes"]["enroll"] = {"packages": [item]}
    facts, warnings = diagram._package_facts(labels, tmp_path)
    assert facts["enroll"][0]["kind"] == "declaration"
    assert not warnings
    item["file"] = "../handler.go"
    facts, warnings = diagram._package_facts(labels, tmp_path)
    assert not facts and warnings


def test_actual_upstream_build_embeds_the_source_map_without_claiming_check(tmp_path):
    if not shutil.which("node") and not shutil.which("bun"):
        pytest.skip("upstream renderer requires Node.js or Bun")
    before = hashlib.sha256(SAMPLE.read_bytes()).hexdigest()
    result = diagram.build_diagram(
        SAMPLE, tmp_path / "view", check=False, skill_dir=RENDERER
    )
    assert result["status"] == "NOT_RUN", result
    assert result["readiness"]["build"] == "READY"
    assert result["readiness"]["layout_check"] == "NOT_RUN"
    assert result["business_verdict"]["recorded"] == "BLOCKED"
    assert result["business_verdict"]["reverified"] is False
    html = Path(result["artifacts"]["html"]).read_text(encoding="utf-8")
    assert 'id="spec" type="application/json"' in html
    assert '"node_ids"' in html and "enrollment.submit" in html
    assert '"packet": false' in html
    embedded = json.loads(
        html.split('<script id="spec" type="application/json">', 1)[1].split(
            "</script>", 1
        )[0]
    )
    renderer = embedded["arc"]["renderer"]
    assert renderer["repository"] == "https://github.com/bassnova42/arch-diagram-skill"
    assert renderer["version"] == "1.2.3"
    assert renderer["commit"] == "879ca7fa503a64ab46b5bc77d2062ded32fadc84"
    assert renderer["license"] == "MIT"
    assert renderer["license_text"] == (RENDERER / "LICENSE").read_text(encoding="utf-8")
    assert "Copyright (c) 2026 bassnova42" in html
    assert "Permission is hereby granted, free of charge" in html
    assert renderer["commit"] in html
    source_spec = json.loads(Path(result["artifacts"]["spec"]).read_text(encoding="utf-8"))
    assert source_spec["arc"]["renderer"] == renderer
    assert not list(tmp_path.rglob("*.db"))
    assert hashlib.sha256(SAMPLE.read_bytes()).hexdigest() == before


def test_failed_browser_check_retains_real_spec_and_html(tmp_path, monkeypatch):
    if not shutil.which("node") and not shutil.which("bun"):
        pytest.skip("upstream renderer requires Node.js or Bun")
    upstream_run = diagram._run

    def no_browser(argv):
        if Path(argv[1]).name == "check.mjs":
            return {
                "argv": argv,
                "exit_code": 2,
                "stdout": "",
                "stderr": "no Chrome/Chromium found",
            }
        return upstream_run(argv)

    monkeypatch.setattr(diagram, "_run", no_browser)
    result = diagram.build_diagram(SAMPLE, tmp_path / "view.html", skill_dir=RENDERER)
    assert result["status"] == "BLOCKED"
    assert result["readiness"]["build"] == "READY"
    assert result["readiness"]["layout_check"] == "BLOCKED"
    assert result["readiness"]["screenshot"] == "NOT_RUN"
    assert Path(result["artifacts"]["html"]).is_file()
    assert Path(result["artifacts"]["spec"]).is_file()
    assert result["check"]["stderr"] == "no Chrome/Chromium found"
    assert "screenshot" not in result["artifacts"]


def test_snapshot_digest_binds_the_bytes_actually_rendered(tmp_path, monkeypatch):
    if not shutil.which("node") and not shutil.which("bun"):
        pytest.skip("upstream renderer requires Node.js or Bun")
    snapshot = tmp_path / "snapshot.json"
    snapshot.write_bytes(SAMPLE.read_bytes())
    original_digest = hashlib.sha256(snapshot.read_bytes()).hexdigest()
    convert = diagram.snapshot_to_spec

    def replace_after_read(*args, **kwargs):
        spec = convert(*args, **kwargs)
        snapshot.write_text('{"concurrent_writer":"replaced the input"}\n')
        return spec

    monkeypatch.setattr(diagram, "snapshot_to_spec", replace_after_read)
    result = diagram.build_diagram(snapshot, tmp_path / "view", check=False)
    assert result["readiness"]["build"] == "READY"
    spec = json.loads(Path(result["artifacts"]["spec"]).read_text())
    assert spec["arc"]["snapshot"]["sha256"] == original_digest
    assert spec["arc"]["meta"]["graph"] == "course-v2"
    assert hashlib.sha256(snapshot.read_bytes()).hexdigest() != original_digest


def test_tampered_renderer_is_never_executed(tmp_path, monkeypatch):
    copied = tmp_path / "renderer"
    shutil.copytree(RENDERER, copied)
    script = copied / "scripts/build.mjs"
    script.write_text(
        script.read_text(encoding="utf-8") + "\n// changed\n", encoding="utf-8"
    )
    monkeypatch.setattr(
        diagram,
        "_run",
        lambda argv: pytest.fail("unreviewed renderer must not execute"),
    )
    result = diagram.build_diagram(SAMPLE, tmp_path / "output", skill_dir=copied)
    assert result["status"] == "BLOCKED"
    assert "checksum mismatch" in result["reason"]
    assert "spec" in result["artifacts"]
    assert "html" not in result["artifacts"]


def test_duplicate_json_and_existing_outputs_are_not_repaired(tmp_path):
    damaged = tmp_path / "snapshot.json"
    damaged.write_text('{"model": {}, "model": {"approved": true}}', encoding="utf-8")
    result = diagram.build_diagram(damaged, tmp_path / "new")
    assert result["status"] == "BLOCKED" and "duplicate" in result["reason"]
    assert not (tmp_path / "new").exists()
    existing = tmp_path / "existing.html"
    existing.write_text("keep original", encoding="utf-8")
    result = diagram.build_diagram(SAMPLE, existing)
    assert result["status"] == "BLOCKED" and "already exist" in result["reason"]
    assert existing.read_text(encoding="utf-8") == "keep original"
