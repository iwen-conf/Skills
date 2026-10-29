from pathlib import Path

from arc_policy.verify import check_business_graph


def test_unconfigured_graph_is_blocked(tmp_path):
    (tmp_path / ".arc/verify").mkdir(parents=True)
    (tmp_path / ".arc/verify/config.jsonc").write_text("{}")
    result = check_business_graph(tmp_path, {"verification": {}})
    assert result["verdict"] == "BLOCKED"


def test_no_opt_in_preserves_existing_policy(tmp_path):
    assert check_business_graph(tmp_path, {"verification": {}}) is None


def test_graph_gate_reads_project_config(tmp_path, monkeypatch):
    calls = []
    def fake_run(root, argv, cwd, *, configured):
        calls.append(argv)
        return {"verdict": "PASS", "reason": "fixture"}
    monkeypatch.setattr("arc_policy.verify._check_command", fake_run)
    r = check_business_graph(tmp_path, {"verification": {"business_graph": {"graph": "academic-v1"}}})
    assert r["id"] == "business_graph" and r["verdict"] == "PASS"
    assert calls[0] == ["arc-verify", "gate", "--db", ".arc/verify/data.db", "--graph", "academic-v1"]

def test_code_phase_does_not_need_green_business_graph(tmp_path, monkeypatch):
    from arc_policy.store import begin_task
    from arc_policy.verify import verify_task
    import subprocess

    repo = tmp_path / "repo"
    repo.mkdir()
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    (repo / "README").write_text("test")
    subprocess.run(["git", "add", "README"], cwd=repo, check=True)
    subprocess.run(["git", "-c", "user.name=Test",
                    "-c", "user.email=example@example.invalid",
                    "commit", "-qm", "init"], cwd=repo, check=True)
    (repo / ".arc/verify").mkdir(parents=True)
    (repo / ".arc/verify/config.jsonc").write_text("{}")
    begin_task(repo, skill="arc-code", slug="fix")
    def must_not_call(*args, **kwargs):
        raise AssertionError("arc-code must not require green runtime business graph")
    monkeypatch.setattr("arc_policy.verify.check_business_graph", must_not_call)
    receipt = verify_task(
        repo, policy={"verification": {"required": ["cli_receipt"], "fail_closed": True}},
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
    subprocess.run(["git", "-c", "user.name=Test",
                    "-c", "user.email=example@example.invalid",
                    "commit", "-qm", "init"], cwd=repo, check=True)
    (repo / ".arc/verify").mkdir(parents=True)
    (repo / ".arc/verify/config.jsonc").write_text("{}")
    begin_task(repo, skill="arc-verify", slug="verify")
    receipt = verify_task(
        repo, policy={"verification": {"required": ["cli_receipt"], "fail_closed": True}},
        run_tests=False,
    )
    assert receipt["verdict"] == "FAIL"
    assert any(x["id"] == "business_graph" and x["verdict"] == "BLOCKED" for x in receipt["checks"])

def test_doctor_reports_local_graph_runtime_requirements(tmp_path, monkeypatch, capsys):
    from arc_policy.cli import main
    import subprocess
    repo = tmp_path / "repo"
    repo.mkdir()
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    (repo / ".arc/verify").mkdir(parents=True)
    (repo / ".arc/verify/config.jsonc").write_text("{}")
    import arc_policy.cli as m
    monkeypatch.setattr(m.shutil if hasattr(m, "shutil") else __import__("shutil"),
                       "which", lambda cmd: None)
    result = main(["--cwd", str(repo), "doctor"])
    import json
    report = json.loads(capsys.readouterr().out)
    assert result == 1
    assert report["graph_configured"] is True
    assert report["graph_runtime_ready"] is False
    assert report["arc_verify_on_path"] is False
    assert report["tursodb_on_path"] is False
