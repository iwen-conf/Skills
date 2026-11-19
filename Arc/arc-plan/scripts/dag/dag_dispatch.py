#!/usr/bin/env python3
"""Isolated git worktree dispatcher & write_scope verifier for plan-dag.yaml."""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any

import yaml

from dag_check import scopes_overlap, validate_dag


def _run(cmd: list[str], cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, cwd=cwd, text=True, capture_output=True)


def prepare_worktree(repo: Path, worktree_root: Path, node: dict[str, Any], frozen: list[str], dry_run: bool) -> dict[str, Any]:
    nid = str(node["id"])
    wt_dir = worktree_root / nid
    task_manifest = {
        "task_id": nid,
        "agent_profile": node.get("runner") or node.get("agent_profile") or "default",
        "frozen_contracts": frozen,
        "read_scope": node.get("read_scope") or [],
        "write_scope": node.get("write_scope") or [],
        "verify_cmd": node.get("verify_cmd") or "",
        "depends_on": node.get("depends_on") or [],
    }
    if dry_run:
        return {"node": nid, "worktree": str(wt_dir), "dry_run": True, "task_manifest": task_manifest}

    worktree_root.mkdir(parents=True, exist_ok=True)
    if not wt_dir.exists():
        res = _run(["git", "worktree", "add", "--detach", str(wt_dir), "HEAD"], repo)
        if res.returncode != 0:
            raise RuntimeError(f"git worktree add failed for {nid}: {res.stderr.strip()}")
    (wt_dir / ".arc-task.json").write_text(json.dumps(task_manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return {"node": nid, "worktree": str(wt_dir), "task_file": str(wt_dir / ".arc-task.json")}


def check_worktree_scope(wt_dir: Path, max_errors: int = 5) -> dict[str, Any]:
    task_path = wt_dir / ".arc-task.json"
    if not task_path.is_file():
        return {"status": "FAIL", "errors": [{"code": "MISSING_TASK_FILE", "scope": str(wt_dir), "message": "missing .arc-task.json"}]}
    task = json.loads(task_path.read_text(encoding="utf-8"))
    write_scope = [str(x) for x in task.get("write_scope") or []]
    frozen = [str(x) for x in task.get("frozen_contracts") or []]

    diff = _run(["git", "status", "--porcelain"], wt_dir)
    changed: list[str] = []
    for line in diff.stdout.splitlines():
        if len(line) < 4:
            continue
        p = line[3:].strip().strip('"')
        if " -> " in p:
            p = p.split(" -> ", 1)[1].strip().strip('"')
        if p == ".arc-task.json":
            continue
        changed.append(p)

    errors: list[dict[str, str]] = []
    for path in changed:
        for locked in frozen:
            if scopes_overlap(path, locked):
                errors.append({"code": "FROZEN_CONTRACT_MODIFIED", "scope": path, "message": f"touched frozen contract {locked}"})
        if not any(scopes_overlap(path, ws) for ws in write_scope):
            errors.append({"code": "OUT_OF_WRITE_SCOPE", "scope": path, "message": f"modified file outside write_scope {write_scope}"})

    return {
        "status": "PASS" if not errors else "FAIL",
        "changed_files": changed,
        "total_errors": len(errors),
        "errors": errors[:max_errors],
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)

    p_prep = sub.add_parser("prepare", help="Create isolated git worktrees and .arc-task.json for a wave or node")
    p_prep.add_argument("dag_file", type=Path)
    p_prep.add_argument("--repo", type=Path, default=Path("."))
    p_prep.add_argument("--worktree-root", type=Path, default=Path(".arc/worktrees"))
    p_prep.add_argument("--wave", type=int, default=0)
    p_prep.add_argument("--node", type=str, default="")
    p_prep.add_argument("--dry-run", action="store_true")

    p_scope = sub.add_parser("verify-scope", help="Check worktree modified files against .arc-task.json")
    p_scope.add_argument("--worktree", type=Path, default=Path("."))
    p_scope.add_argument("--max-errors", type=int, default=5)

    args = parser.parse_args(argv)
    if args.action == "verify-scope":
        rep = check_worktree_scope(args.worktree.resolve(), max_errors=args.max_errors)
        print(json.dumps(rep, indent=2, ensure_ascii=False))
        return 0 if rep["status"] == "PASS" else 1

    doc = yaml.safe_load(args.dag_file.read_text(encoding="utf-8")) or {}
    check = validate_dag(doc)
    if check["status"] != "PASS":
        print(json.dumps(check, indent=2, ensure_ascii=False))
        return 1

    nodes_by_id = {str(n["id"]): n for n in (doc.get("nodes") or doc.get("tasks") or [])}
    if args.node:
        selected = [args.node]
    else:
        waves = check["waves"]
        if args.wave < 0 or args.wave >= len(waves):
            print(json.dumps({"status": "FAIL", "errors": [{"code": "INVALID_WAVE", "scope": str(args.wave), "message": f"wave must be 0..{len(waves)-1}"}]}))
            return 1
        selected = waves[args.wave]

    repo = args.repo.resolve()
    wt_root = (repo / args.worktree_root).resolve() if not args.worktree_root.is_absolute() else args.worktree_root
    prepared = [prepare_worktree(repo, wt_root, nodes_by_id[nid], check["frozen_contracts"], args.dry_run) for nid in selected]
    print(json.dumps({"status": "PASS", "wave": args.wave, "prepared": prepared}, indent=2, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
