#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import time
from collections import defaultdict
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load, sha256_file


def _cycles(graph: dict[str, set[str]], limit: int = 100) -> list[list[str]]:
    """Return canonical simple cycles. This is a design linter, not a DB deadlock proof."""
    found: set[tuple[str, ...]] = set()
    nodes = sorted(set(graph) | {v for vs in graph.values() for v in vs})

    def canon(cycle: list[str]) -> tuple[str, ...]:
        # cycle without repeated terminal node
        n = len(cycle)
        rots = [tuple(cycle[i:] + cycle[:i]) for i in range(n)]
        return min(rots)

    for start in nodes:
        stack: list[tuple[str, list[str], set[str]]] = [(start, [start], {start})]
        while stack and len(found) < limit:
            cur, path, seen = stack.pop()
            for nxt in sorted(graph.get(cur, ())):
                if nxt == start and len(path) > 1:
                    found.add(canon(path))
                elif nxt not in seen and len(path) < len(nodes):
                    stack.append((nxt, path + [nxt], seen | {nxt}))
    return [list(x) + [x[0]] for x in sorted(found)]


def analyze(catalog: dict[str, Any]) -> dict[str, Any]:
    graph: dict[str, set[str]] = defaultdict(set)
    edge_sources: dict[tuple[str, str], list[str]] = defaultdict(list)
    operations = catalog.get("operations", [])
    problems: list[str] = []

    for op in operations:
        oid = op.get("id")
        locks = op.get("lock_order") or []
        if not oid:
            problems.append("operation missing id")
            continue
        if len(set(locks)) != len(locks):
            problems.append(f"operation {oid}: duplicate resource in lock_order")
        for a, b in zip(locks, locks[1:]):
            if a == b:
                continue
            graph[str(a)].add(str(b))
            edge_sources[(str(a), str(b))].append(str(oid))

    cycles = _cycles(graph)
    cycle_details = []
    for cyc in cycles:
        edges = []
        for a, b in zip(cyc, cyc[1:]):
            edges.append({"from": a, "to": b, "operations": sorted(edge_sources.get((a, b), []))})
        cycle_details.append({"cycle": cyc, "edges": edges})

    return {
        "operation_count": len(operations),
        "resource_count": len(set(graph) | {v for vs in graph.values() for v in vs}),
        "edge_count": sum(len(v) for v in graph.values()),
        "cycles": cycle_details,
        "catalog_problems": problems,
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Static business transaction lock-order cycle linter")
    ap.add_argument("--catalog", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--allow-cycles", action="store_true", help="Record cycles but do not fail (normally unsafe)")
    ns = ap.parse_args()
    started = time.monotonic()
    p = Path(ns.catalog)
    ev = evidence_base("lock_order_analysis", "lock-order.analysis")
    try:
        catalog = json_load(p)
        result = analyze(catalog)
        ev["reproduction"] = {"catalog": str(p), "catalog_sha256": sha256_file(p)}
        ev["observed"] = result
        for x in result["catalog_problems"]:
            ev["violations"].append(x)
        if result["cycles"]:
            msg = f"potential lock-order cycles found: {len(result['cycles'])}; real PostgreSQL contention tests are required"
            (ev["warnings"] if ns.allow_cycles else ev["violations"]).append(msg)
        status = "PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status = "FAIL"
    json_dump(Path(ns.out), finalize_evidence(ev, started, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
