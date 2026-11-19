#!/usr/bin/env python3
"""Deterministic validator for write-set isolated plan-dag.yaml manifests.

Enforces:
1. Frozen contracts cannot be overlapped by any node's write_scope.
2. Dependency graph is a valid acyclic DAG (computes parallel topological waves).
3. Every node within the same parallel wave has pairwise disjoint write_scope.
4. Every node declares a non-empty verify_cmd.
5. Emits compact structured JSON capped to --max-errors (default 5).
"""
from __future__ import annotations

import argparse
import fnmatch
import json
import sys
from collections import defaultdict, deque
from pathlib import Path
from typing import Any

import yaml


def _norm(path: str) -> str:
    p = path.strip().replace("\\", "/")
    while p.startswith("./"):
        p = p[2:]
    return p.rstrip("/")


def _has_glob(p: str) -> bool:
    return any(ch in p for ch in "*?[")


def _glob_prefix(p: str) -> str:
    for idx, ch in enumerate(p):
        if ch in "*?[":
            return p[:idx].rstrip("/")
    return p.rstrip("/")


def scopes_overlap(a: str, b: str) -> bool:
    na, nb = _norm(a), _norm(b)
    if not na or not nb:
        return True
    if na == nb:
        return True
    if not _has_glob(na) and not _has_glob(nb):
        return na == nb
    if not _has_glob(na) and fnmatch.fnmatch(na, nb):
        return True
    if not _has_glob(nb) and fnmatch.fnmatch(nb, na):
        return True
    pa, pb = _glob_prefix(na), _glob_prefix(nb)
    if not pa or not pb:
        return True
    return pa == pb or pa.startswith(pb + "/") or pb.startswith(pa + "/")


def validate_dag(doc: dict[str, Any], *, max_errors: int = 5) -> dict[str, Any]:
    errors: list[dict[str, str]] = []

    def add(code: str, scope: str, message: str) -> None:
        errors.append({"code": code, "scope": scope, "message": message})

    frozen = doc.get("frozen_contracts") or doc.get("contract_freeze", {}).get("locked_paths") or []
    if not isinstance(frozen, list):
        add("INVALID_FROZEN", "frozen_contracts", "frozen_contracts must be a list of paths")
        frozen = []

    nodes = doc.get("nodes") or doc.get("tasks") or []
    if not isinstance(nodes, list) or not nodes:
        add("EMPTY_NODES", "nodes", "plan-dag.yaml must define a non-empty nodes/tasks list")
        nodes = []

    by_id: dict[str, dict[str, Any]] = {}
    for idx, node in enumerate(nodes):
        if not isinstance(node, dict):
            add("INVALID_NODE", f"nodes[{idx}]", "node entry must be a mapping")
            continue
        nid = str(node.get("id", "")).strip()
        if not nid:
            add("MISSING_ID", f"nodes[{idx}]", "node id is required")
            continue
        if nid in by_id:
            add("DUPLICATE_ID", nid, f"duplicate node id {nid!r}")
            continue
        by_id[nid] = node

        write_scope = node.get("write_scope") or []
        if not isinstance(write_scope, list) or not write_scope:
            add("MISSING_WRITE_SCOPE", nid, "node must declare non-empty write_scope")
        else:
            for ws in write_scope:
                for locked in frozen:
                    if scopes_overlap(str(ws), str(locked)):
                        add(
                            "FROZEN_CONTRACT_OVERLAP",
                            nid,
                            f"write_scope {ws!r} overlaps frozen contract {locked!r}",
                        )

        if not str(node.get("verify_cmd", "")).strip():
            add("MISSING_VERIFY_CMD", nid, "node must declare a non-empty verify_cmd")

    # Compute parallel topological waves (Kahn's algorithm).
    indeg: dict[str, int] = {nid: 0 for nid in by_id}
    children: dict[str, list[str]] = defaultdict(list)
    for nid, node in by_id.items():
        deps = node.get("depends_on") or []
        if not isinstance(deps, list):
            add("INVALID_DEPENDS_ON", nid, "depends_on must be a list")
            continue
        for dep in deps:
            dep_id = str(dep).strip()
            if dep_id not in by_id:
                add("UNKNOWN_DEPENDENCY", nid, f"depends_on unknown node {dep_id!r}")
            else:
                children[dep_id].append(nid)
                indeg[nid] += 1

    waves: list[list[str]] = []
    ready = deque(sorted(nid for nid, d in indeg.items() if d == 0))
    visited = 0
    while ready:
        wave = sorted(ready)
        ready.clear()
        waves.append(wave)
        next_ready: list[str] = []
        for nid in wave:
            visited += 1
            for ch in children[nid]:
                indeg[ch] -= 1
                if indeg[ch] == 0:
                    next_ready.append(ch)
        ready.extend(sorted(next_ready))

    if visited != len(by_id):
        cyclic = sorted(nid for nid, d in indeg.items() if d > 0)
        add("DAG_CYCLE", ",".join(cyclic), f"dependency cycle detected among nodes: {', '.join(cyclic)}")

    # Enforce pairwise disjoint write_scope within every parallel wave.
    for wave_idx, wave in enumerate(waves):
        seen_scopes: list[tuple[str, str]] = []
        for nid in wave:
            for ws in by_id[nid].get("write_scope") or []:
                ws_s = str(ws)
                for prev_nid, prev_ws in seen_scopes:
                    if prev_nid != nid and scopes_overlap(ws_s, prev_ws):
                        add(
                            "PARALLEL_WRITE_COLLISION",
                            f"wave[{wave_idx}]:{nid}",
                            f"write_scope {ws_s!r} collides with parallel node {prev_nid!r} scope {prev_ws!r}",
                        )
                seen_scopes.append((nid, ws_s))

    status = "PASS" if not errors else "FAIL"
    truncated = len(errors) > max_errors
    return {
        "status": status,
        "frozen_contracts": frozen,
        "node_count": len(by_id),
        "waves": waves,
        "total_errors": len(errors),
        "truncated": truncated,
        "errors": errors[:max_errors],
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dag_file", type=Path, help="Path to plan-dag.yaml")
    parser.add_argument("--max-errors", type=int, default=5, help="Cap returned fatal errors")
    args = parser.parse_args(argv)
    if not args.dag_file.is_file():
        print(json.dumps({"status": "FAIL", "errors": [{"code": "FILE_NOT_FOUND", "scope": str(args.dag_file), "message": "DAG file not found"}]}))
        return 1
    doc = yaml.safe_load(args.dag_file.read_text(encoding="utf-8")) or {}
    report = validate_dag(doc, max_errors=args.max_errors)
    print(json.dumps(report, indent=2, ensure_ascii=False))
    return 0 if report["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
