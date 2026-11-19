"""Tests for write-set isolated plan-dag scripts and atomic gate.sh subcommands."""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DAG_CHECK = ROOT / "Arc" / "arc-plan" / "scripts" / "dag" / "dag_check.py"
DAG_DISPATCH = ROOT / "Arc" / "arc-plan" / "scripts" / "dag" / "dag_dispatch.py"
GATE = ROOT / "Arc" / "arc-code" / "scripts" / "gate.sh"


def _git(repo: Path, *args: str) -> None:
    subprocess.run(
        ["git", *args],
        cwd=repo,
        check=True,
        capture_output=True,
        text=True,
    )


def _init_repo(tmp_path: Path) -> Path:
    repo = tmp_path / "repo"
    repo.mkdir()
    _git(repo, "init", "-q")
    _git(repo, "config", "user.name", "Test")
    _git(repo, "config", "user.email", "test@example.invalid")
    (repo / "api").mkdir()
    (repo / "api" / "openapi.yaml").write_text("openapi: 3.1.0\n", encoding="utf-8")
    (repo / "internal" / "wallet").mkdir(parents=True)
    (repo / "internal" / "wallet" / "guard.go").write_text("package wallet\n", encoding="utf-8")
    (repo / "internal" / "order").mkdir(parents=True)
    (repo / "internal" / "order" / "guard.go").write_text("package order\n", encoding="utf-8")
    _git(repo, "add", "-A")
    _git(repo, "commit", "-qm", "init")
    return repo


def test_dag_check_valid_parallel_waves_and_collisions(tmp_path: Path) -> None:
    valid_dag = tmp_path / "valid-dag.yaml"
    valid_dag.write_text(
        """
frozen_contracts:
  - api/openapi.yaml
nodes:
  - id: wallet-guard
    depends_on: []
    write_scope: [internal/wallet/**]
    verify_cmd: "echo ok"
  - id: order-guard
    depends_on: []
    write_scope: [internal/order/**]
    verify_cmd: "echo ok"
  - id: orchestrator
    depends_on: [wallet-guard, order-guard]
    write_scope: [internal/account/**]
    verify_cmd: "echo ok"
""",
        encoding="utf-8",
    )
    res = subprocess.run([sys.executable, str(DAG_CHECK), str(valid_dag)], capture_output=True, text=True)
    assert res.returncode == 0, res.stdout + res.stderr
    data = json.loads(res.stdout)
    assert data["status"] == "PASS"
    assert data["waves"] == [["order-guard", "wallet-guard"], ["orchestrator"]]

    bad_dag = tmp_path / "bad-dag.yaml"
    bad_dag.write_text(
        """
frozen_contracts:
  - api/openapi.yaml
nodes:
  - id: a
    depends_on: []
    write_scope: [internal/wallet/**, api/openapi.yaml]
    verify_cmd: "echo ok"
  - id: b
    depends_on: []
    write_scope: [internal/wallet/guard.go]
    verify_cmd: ""
""",
        encoding="utf-8",
    )
    res_bad = subprocess.run([sys.executable, str(DAG_CHECK), str(bad_dag), "--max-errors", "5"], capture_output=True, text=True)
    assert res_bad.returncode == 1
    bad_data = json.loads(res_bad.stdout)
    codes = {e["code"] for e in bad_data["errors"]}
    assert "FROZEN_CONTRACT_OVERLAP" in codes
    assert "PARALLEL_WRITE_COLLISION" in codes
    assert "MISSING_VERIFY_CMD" in codes


def test_dag_dispatch_worktree_and_atomic_gate_scope(tmp_path: Path) -> None:
    repo = _init_repo(tmp_path)
    dag = repo / "plan-dag.yaml"
    dag.write_text(
        """
frozen_contracts:
  - api/openapi.yaml
nodes:
  - id: wallet-guard
    depends_on: []
    write_scope: [internal/wallet/**]
    verify_cmd: "echo wallet-pass"
""",
        encoding="utf-8",
    )
    prep = subprocess.run(
        [sys.executable, str(DAG_DISPATCH), "prepare", str(dag), "--repo", str(repo), "--wave", "0"],
        capture_output=True,
        text=True,
    )
    assert prep.returncode == 0, prep.stdout + prep.stderr
    wt = Path(json.loads(prep.stdout)["prepared"][0]["worktree"])
    assert (wt / ".arc-task.json").is_file()

    # In-scope edit passes atomic scope + test gates.
    (wt / "internal" / "wallet" / "guard.go").write_text("package wallet\n// ok\n", encoding="utf-8")
    scope_ok = subprocess.run(["bash", str(GATE), "scope", "--json"], cwd=wt, capture_output=True, text=True)
    assert scope_ok.returncode == 0, scope_ok.stdout + scope_ok.stderr
    assert json.loads(scope_ok.stdout)["status"] == "PASS"

    test_ok = subprocess.run(["bash", str(GATE), "test", "--json"], cwd=wt, capture_output=True, text=True)
    assert test_ok.returncode == 0, test_ok.stdout + test_ok.stderr
    assert json.loads(test_ok.stdout)["status"] == "PASS"

    # Touching frozen contract or another module's scope fails atomic scope gate.
    (wt / "api" / "openapi.yaml").write_text("openapi: 3.1.1\n", encoding="utf-8")
    (wt / "internal" / "order" / "guard.go").write_text("package order\n// bad\n", encoding="utf-8")
    scope_bad = subprocess.run(["bash", str(GATE), "scope", "--json"], cwd=wt, capture_output=True, text=True)
    assert scope_bad.returncode == 1
    bad_codes = {e["code"] for e in json.loads(scope_bad.stdout)["errors"]}
    assert "FROZEN_CONTRACT_MODIFIED" in bad_codes
    assert "OUT_OF_WRITE_SCOPE" in bad_codes
