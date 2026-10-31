from pathlib import Path
import json
import subprocess

import pytest

from arc_policy.verify import check_business_graph


def graph_proof(root):
    return {
        "verdict": "PASS",
        "proof_scope": "planned_rule_tasks",
        "inputs_unchanged": True,
        "context": {
            "graph": "academic-v1",
            "environment": "isolated-test",
            "target": "https://target.invalid",
            "source_commit": "a" * 40,
            "source_sha256": "b" * 64,
            "config_sha256": "c" * 64,
            "graph_sha256": "d" * 64,
            "source_root": str(root.resolve()),
            "config_path": str((root / ".arc/verify/config.jsonc").resolve()),
        },
    }


@pytest.mark.parametrize(
    "damage",
    [
        "empty",
        "missing_scope",
        "missing_context",
        "changed_input",
        "wrong_graph",
        "wrong_root",
        "duplicate_verdict",
        "invalid_verdict",
    ],
)
def test_graph_exit_zero_without_current_proof_is_blocked(
    tmp_path, monkeypatch, damage
):
    proof = graph_proof(tmp_path)
    if damage == "empty":
        output = ""
    else:
        if damage == "missing_scope":
            proof.pop("proof_scope")
        elif damage == "missing_context":
            proof.pop("context")
        elif damage == "changed_input":
            proof["inputs_unchanged"] = False
        elif damage == "wrong_graph":
            proof["context"]["graph"] = "other-graph"
        elif damage == "wrong_root":
            proof["context"]["source_root"] = str(tmp_path / "another-candidate")
        elif damage == "invalid_verdict":
            proof["verdict"] = []
        output = json.dumps(proof)
        if damage == "duplicate_verdict":
            output = output.replace(
                '{"verdict": "PASS",', '{"verdict": "FAIL", "verdict": "PASS",', 1
            )
    monkeypatch.setattr(
        "arc_policy.verify._run",
        lambda root, argv: subprocess.CompletedProcess(argv, 0, output, ""),
    )
    result = check_business_graph(
        tmp_path, {"verification": {"business_graph": {"graph": "academic-v1"}}}
    )
    assert result["verdict"] == "BLOCKED"


def test_graph_receipt_cannot_change_its_proof_context(tmp_path, monkeypatch):
    from arc_policy.store import receipt_matches_tree

    task = {"id": "task1", "slug": "verify", "skill": "arc-verify"}
    tree = {"head": "a", "dirty": [], "digest": "same-tree"}
    original = {"id": "business_graph", **graph_proof(tmp_path)}
    current = json.loads(json.dumps(original))
    current["context"]["environment"] = "other-environment"
    receipt = {
        "task_id": task["id"],
        "slug": task["slug"],
        "skill": task["skill"],
        "verdict": "PASS",
        **tree,
        "checks": [original],
    }
    monkeypatch.setattr("arc_policy.store.active_task", lambda root: task)
    monkeypatch.setattr("arc_policy.store.tree_fingerprint", lambda root: tree)
    monkeypatch.setattr("arc_policy.policy.load_for_repo", lambda root: {})
    monkeypatch.setattr(
        "arc_policy.verify.check_business_graph", lambda root, policy: current
    )
    assert not receipt_matches_tree(tmp_path, receipt)


def test_unconfigured_graph_is_blocked(tmp_path):
    (tmp_path / ".arc/verify").mkdir(parents=True)
    (tmp_path / ".arc/verify/config.jsonc").write_text("{}")
    result = check_business_graph(tmp_path, {"verification": {}})
    assert result["verdict"] == "BLOCKED"


def test_no_opt_in_preserves_existing_policy(tmp_path):
    assert check_business_graph(tmp_path, {"verification": {}}) is None


def test_graph_gate_reads_project_config(tmp_path, monkeypatch):
    calls = []

    def fake_run(root, argv):
        calls.append(argv)
        return subprocess.CompletedProcess(argv, 0, json.dumps(graph_proof(root)), "")

    monkeypatch.setattr("arc_policy.verify._run", fake_run)
    r = check_business_graph(
        tmp_path, {"verification": {"business_graph": {"graph": "academic-v1"}}}
    )
    assert r["id"] == "business_graph" and r["verdict"] == "PASS"
    assert calls[0] == [
        "arc-verify",
        "gate",
        "--db",
        ".arc/verify/data.db",
        "--graph",
        "academic-v1",
    ]


def test_code_phase_does_not_need_green_business_graph(tmp_path, monkeypatch):
    from arc_policy.store import begin_task
    from arc_policy.verify import verify_task
    import subprocess

    repo = tmp_path / "repo"
    repo.mkdir()
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    (repo / "README").write_text("test")
    subprocess.run(["git", "add", "README"], cwd=repo, check=True)
    subprocess.run(
        [
            "git",
            "-c",
            "user.name=Test",
            "-c",
            "user.email=example@example.invalid",
            "commit",
            "-qm",
            "init",
        ],
        cwd=repo,
        check=True,
    )
    (repo / ".arc/verify").mkdir(parents=True)
    (repo / ".arc/verify/config.jsonc").write_text("{}")
    begin_task(repo, skill="arc-code", slug="fix")

    def must_not_call(*args, **kwargs):
        raise AssertionError("arc-code must not require green runtime business graph")

    monkeypatch.setattr("arc_policy.verify.check_business_graph", must_not_call)
    receipt = verify_task(
        repo,
        policy={"verification": {"required": ["cli_receipt"], "fail_closed": True}},
        run_tests=False,
    )
    assert receipt["verdict"] == "PASS"


def test_verify_phase_fails_closed_when_graph_missing(tmp_path, monkeypatch):
    from arc_policy.store import begin_task
    from arc_policy.verify import verify_task
    import subprocess

    repo = tmp_path / "repo"
    repo.mkdir()
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    (repo / "README").write_text("test")
    subprocess.run(["git", "add", "README"], cwd=repo, check=True)
    subprocess.run(
        [
            "git",
            "-c",
            "user.name=Test",
            "-c",
            "user.email=example@example.invalid",
            "commit",
            "-qm",
            "init",
        ],
        cwd=repo,
        check=True,
    )
    (repo / ".arc/verify").mkdir(parents=True)
    (repo / ".arc/verify/config.jsonc").write_text("{}")
    begin_task(repo, skill="arc-verify", slug="verify")
    receipt = verify_task(
        repo,
        policy={"verification": {"required": ["cli_receipt"], "fail_closed": True}},
        run_tests=False,
    )
    assert receipt["verdict"] == "FAIL"
    assert any(
        x["id"] == "business_graph" and x["verdict"] == "BLOCKED"
        for x in receipt["checks"]
    )


def test_doctor_reports_local_graph_runtime_requirements(tmp_path, monkeypatch, capsys):
    from arc_policy.cli import main
    import subprocess

    repo = tmp_path / "repo"
    repo.mkdir()
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    (repo / ".arc/verify").mkdir(parents=True)
    (repo / ".arc/verify/config.jsonc").write_text("{}")
    import arc_policy.cli as m

    monkeypatch.setattr(
        m.shutil if hasattr(m, "shutil") else __import__("shutil"),
        "which",
        lambda cmd: None,
    )
    result = main(["--cwd", str(repo), "doctor"])
    import json

    report = json.loads(capsys.readouterr().out)
    assert result == 1
    assert report["graph_configured"] is True
    assert report["graph_runtime_ready"] is False
    assert report["arc_verify_on_path"] is False
    assert report["tursodb_on_path"] is False


def _gate_argv(tmp_path, monkeypatch, config, proof_scope="planned_rule_tasks"):
    calls = []

    def fake_run(root, argv):
        calls.append(argv)
        proof = graph_proof(root)
        proof["proof_scope"] = proof_scope
        return subprocess.CompletedProcess(argv, 0, json.dumps(proof), "")

    monkeypatch.setattr("arc_policy.verify._run", fake_run)
    result = check_business_graph(
        tmp_path, {"verification": {"business_graph": config}}
    )
    return result, calls[0]


def test_risk_review_switch_passes_flag(tmp_path, monkeypatch):
    result, argv = _gate_argv(
        tmp_path, monkeypatch, {"graph": "academic-v1", "risk_review": True}
    )
    assert result["verdict"] == "PASS"
    assert argv[-1] == "--risk-review"


@pytest.mark.parametrize("value", [None, False])
def test_risk_review_off_keeps_plain_gate(tmp_path, monkeypatch, value):
    config = {"graph": "academic-v1"}
    if value is not None:
        config["risk_review"] = value
    _, argv = _gate_argv(tmp_path, monkeypatch, config)
    assert "--risk-review" not in argv


def test_risk_review_must_be_boolean(tmp_path):
    r = check_business_graph(
        tmp_path,
        {
            "verification": {
                "business_graph": {"graph": "academic-v1", "risk_review": "yes"}
            }
        },
    )
    assert r["verdict"] == "BLOCKED" and "risk_review" in r["reason"]


def test_v2_model_proof_scope_is_accepted(tmp_path, monkeypatch):
    result, _ = _gate_argv(
        tmp_path,
        monkeypatch,
        {"graph": "academic-v1"},
        proof_scope="complete_business_model_obligations",
    )
    assert result["verdict"] == "PASS"


def test_unknown_proof_scope_is_blocked(tmp_path, monkeypatch):
    result, _ = _gate_argv(
        tmp_path, monkeypatch, {"graph": "academic-v1"}, proof_scope="partial_sample"
    )
    assert result["verdict"] == "BLOCKED"
