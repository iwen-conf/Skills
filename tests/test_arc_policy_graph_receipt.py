from __future__ import annotations

from arc_policy.gitutil import tree_fingerprint


def test_graph_receipt_fingerprint_tracks_ignored_local_database(tmp_path):
    import os
    import subprocess

    root = tmp_path / "repo"
    root.mkdir()
    env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
    subprocess.run(["git", "init", "-q"], cwd=root, env=env, check=True)
    (root / ".gitignore").write_text(".arc/\n")
    subprocess.run(["git", "add", ".gitignore"], cwd=root, env=env, check=True)
    subprocess.run(
        ["git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
         "commit", "-qm", "init"],
        cwd=root, env=env, check=True,
    )
    folder = root / ".arc" / "verify"
    folder.mkdir(parents=True)
    (folder / "config.jsonc").write_text('{"environments":{}}', encoding="utf-8")
    db = folder / "data.db"
    db.write_bytes(b"graph-v1")
    first = tree_fingerprint(root)["digest"]
    db.write_bytes(b"graph-v2")
    second = tree_fingerprint(root)["digest"]
    assert first != second, "mutating the gitignored graph must revoke a receipt"
    (folder / "data.db-wal").write_bytes(b"new-write")
    third = tree_fingerprint(root)["digest"]
    assert second != third, "pending WAL writes must be fingerprinted too"
    (folder / "config.jsonc").write_text('{"environments":{"test":{}}}', encoding="utf-8")
    assert tree_fingerprint(root)["digest"] != third


def test_no_graph_opt_in_does_not_fingerprint_runtime_files(tmp_path):
    import os
    import subprocess

    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    (tmp_path / ".gitignore").write_text(".arc/\n")
    subprocess.run(["git", "-C", str(tmp_path), "add", ".gitignore"], check=True)
    subprocess.run(["git", "-C", str(tmp_path), "-c", "user.name=Test",
                    "-c", "user.email=test@example.invalid",
                    "commit", "-qm", "init"],
                   env={**os.environ,"GIT_CONFIG_GLOBAL":os.devnull,"GIT_CONFIG_NOSYSTEM":"1"},check=True)
    db = tmp_path / ".arc/verify/data.db"
    db.parent.mkdir(parents=True)
    db.write_bytes(b"one")
    first = tree_fingerprint(tmp_path)["digest"]
    db.write_bytes(b"two")
    assert tree_fingerprint(tmp_path)["digest"] == first


def test_active_shared_graph_database_ignores_unrelated_resource_writes(tmp_path):
    """The graph gate revalidates independent proof; resource-table writes are not tests."""
    import json
    import os
    import subprocess

    repo = tmp_path / "shared"
    repo.mkdir()
    env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
    subprocess.run(["git", "init", "-q"], cwd=repo, env=env, check=True)
    (repo / ".gitignore").write_text(".arc/\n")
    subprocess.run(["git", "add", ".gitignore"], cwd=repo, env=env, check=True)
    subprocess.run(
        ["git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
         "commit", "-qm", "init"],
        cwd=repo, env=env, check=True,
    )
    verify = repo / ".arc/verify"
    verify.mkdir(parents=True)
    config = verify / "config.jsonc"
    config.write_text('{"environments":{}}', encoding="utf-8")
    manifest = repo / ".arc/arc.json"
    manifest.write_text(json.dumps({
        "schema_version": 1, "active": True, "db": ".arc/arc.db",
    }))
    db = repo / ".arc/arc.db"
    db.write_bytes(b"graph-and-task-state-v1")
    first = tree_fingerprint(repo)["digest"]
    # Task/Outbox updates must not revoke a valid business graph receipt.
    db.write_bytes(b"graph-and-task-state-v1-and-more-task-events")
    assert tree_fingerprint(repo)["digest"] == first
    (repo / ".arc/arc.db-wal").write_bytes(b"new task event")
    assert tree_fingerprint(repo)["digest"] == first
    # But activation and graph configuration remain part of proof identity.
    config.write_text('{"environments":{"test":{}}}', encoding="utf-8")
    assert tree_fingerprint(repo)["digest"] != first
    manifest.write_text(json.dumps({
        "schema_version": 2, "active": True, "db": ".arc/arc.db",
    }))
    import pytest
    with pytest.raises(RuntimeError, match="activation manifest"):
        tree_fingerprint(repo)


def test_active_graph_receipt_still_rechecks_independent_gate(tmp_path, monkeypatch):
    import json
    import os
    import subprocess

    from arc_policy.store import begin_task, current_receipt, receipt_matches_tree, save_receipt

    repo = tmp_path / "graph-proof"
    repo.mkdir()
    env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
    subprocess.run(["git", "init", "-q"], cwd=repo, env=env, check=True)
    (repo / ".gitignore").write_text(".arc/\n")
    subprocess.run(["git", "add", ".gitignore"], cwd=repo, env=env, check=True)
    subprocess.run(
        ["git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
         "commit", "-qm", "init"],
        cwd=repo, env=env, check=True,
    )
    verify = repo / ".arc/verify"
    verify.mkdir(parents=True)
    (verify / "config.jsonc").write_text("{}")
    (repo / ".arc/arc.json").write_text(
        json.dumps({"schema_version": 1, "active": True, "db": ".arc/arc.db"})
    )
    db = repo / ".arc/arc.db"
    db.write_bytes(b"graph-before")

    task = begin_task(repo, skill="arc-verify", slug="proof")
    snapshot = tree_fingerprint(repo)
    graph_check = {
        "id": "business_graph",
        "verdict": "PASS",
        "proof_scope": "planned_rule_tasks",
        "context": {"graph": "verified-business"},
    }
    receipt = {
        "task_id": task["id"], "slug": task["slug"], "skill": task["skill"],
        "verdict": "PASS", "checks": [graph_check],
        **snapshot,
    }
    save_receipt(repo, receipt)
    monkeypatch.setattr(
        "arc_policy.verify.check_business_graph",
        lambda root, policy: dict(graph_check),
    )
    assert receipt_matches_tree(repo, current_receipt(repo))
    db.write_bytes(b"graph-and-task-changes")
    assert receipt_matches_tree(repo, current_receipt(repo))

    # A fresh independent business FAIL still revokes the receipt even though
    # unrelated changes to the shared file do not change the Git fingerprint.
    monkeypatch.setattr(
        "arc_policy.verify.check_business_graph",
        lambda root, policy: {**graph_check, "verdict": "FAIL"},
    )
    assert not receipt_matches_tree(repo, current_receipt(repo))
