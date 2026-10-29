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
