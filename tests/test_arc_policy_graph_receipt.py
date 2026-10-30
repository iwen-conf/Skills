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
