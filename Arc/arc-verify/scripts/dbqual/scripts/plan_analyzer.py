#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

from sqlverify_common import ratio_error


def _walk(plan: dict[str, Any], path: str = "0"):
    yield path, plan
    for i, child in enumerate(plan.get("Plans", []) or []):
        yield from _walk(child, f"{path}.{i}")


def analyze_explain(explain: Any) -> dict[str, Any]:
    # psycopg may return list[dict], psql JSON file is typically list with one root object.
    if isinstance(explain, list):
        root = explain[0]
    else:
        root = explain
    plan = root.get("Plan", root)
    nodes = []
    max_estimate_error = 1.0
    worst = None
    total_temp_read = 0
    total_temp_written = 0
    total_shared_read = 0
    total_shared_hit = 0
    total_wal_bytes = 0
    node_types: dict[str, int] = {}

    for path, n in _walk(plan):
        typ = n.get("Node Type", "Unknown")
        node_types[typ] = node_types.get(typ, 0) + 1
        est = n.get("Plan Rows")
        actual = n.get("Actual Rows")
        loops_raw = n.get("Actual Loops", 1)
        loops = loops_raw if isinstance(loops_raw, (int, float)) else 1
        actual_total = actual * loops if isinstance(actual, (int, float)) and isinstance(loops, (int, float)) else actual
        # PostgreSQL reports Actual Rows as a per-execution average when loops > 1, specifically
        # to make it comparable with Plan Rows. Do not multiply by loops for cardinality q-error.
        comparable = loops != 0 and typ not in {"BitmapAnd", "BitmapOr"}
        err = ratio_error(est, actual) if comparable else None
        if err is not None and err > max_estimate_error:
            max_estimate_error = err
            worst = {"path": path, "node_type": typ, "plan_rows": est, "actual_rows_per_loop": actual, "actual_total_rows": actual_total, "loops": loops, "ratio": err}
        total_temp_read += int(n.get("Temp Read Blocks", 0) or 0)
        total_temp_written += int(n.get("Temp Written Blocks", 0) or 0)
        total_shared_read += int(n.get("Shared Read Blocks", 0) or 0)
        total_shared_hit += int(n.get("Shared Hit Blocks", 0) or 0)
        total_wal_bytes += int(n.get("WAL Bytes", 0) or 0)
        nodes.append({
            "path": path,
            "node_type": typ,
            "relation": n.get("Relation Name"),
            "index": n.get("Index Name"),
            "plan_rows": est,
            "actual_rows": actual,
            "actual_loops": loops,
            "actual_total_rows": actual_total,
            "estimate_error_ratio": err,
            "actual_total_time_ms_per_loop": n.get("Actual Total Time"),
            "actual_total_time_ms_all_loops": (n.get("Actual Total Time") * loops) if isinstance(n.get("Actual Total Time"), (int, float)) and isinstance(loops, (int, float)) else None,
            "rows_removed_by_filter_per_loop": n.get("Rows Removed by Filter", 0),
            "rows_removed_by_filter_all_loops": (n.get("Rows Removed by Filter", 0) * loops) if isinstance(n.get("Rows Removed by Filter", 0), (int, float)) and isinstance(loops, (int, float)) else None,
            "shared_hit_blocks": n.get("Shared Hit Blocks", 0),
            "shared_read_blocks": n.get("Shared Read Blocks", 0),
            "temp_read_blocks": n.get("Temp Read Blocks", 0),
            "temp_written_blocks": n.get("Temp Written Blocks", 0),
            "wal_bytes": n.get("WAL Bytes", 0),
            "sort_method": n.get("Sort Method"),
            "sort_space_used_kb": n.get("Sort Space Used"),
            "sort_space_type": n.get("Sort Space Type"),
        })

    return {
        "planning_time_ms": root.get("Planning Time"),
        "execution_time_ms": root.get("Execution Time"),
        "startup_cost": plan.get("Startup Cost") if isinstance(plan, dict) else None,
        "total_cost": plan.get("Total Cost") if isinstance(plan, dict) else None,
        "node_types": node_types,
        "node_count": len(nodes),
        "max_estimate_error_ratio": max_estimate_error,
        "worst_estimate_node": worst,
        "shared_read_blocks_sum_nodes": total_shared_read,
        "shared_hit_blocks_sum_nodes": total_shared_hit,
        "temp_read_blocks_sum_nodes": total_temp_read,
        "temp_written_blocks_sum_nodes": total_temp_written,
        "wal_bytes_sum_nodes": total_wal_bytes,
        "nodes": nodes,
    }


def compare(current: dict[str, Any], baseline: dict[str, Any]) -> dict[str, Any]:
    c = analyze_explain(current)
    b = analyze_explain(baseline)
    def delta(key: str):
        cv, bv = c.get(key), b.get(key)
        if not isinstance(cv, (int, float)) or not isinstance(bv, (int, float)) or bv == 0:
            return None
        return (cv - bv) / bv
    return {
        "execution_time_delta_ratio": delta("execution_time_ms"),
        "planning_time_delta_ratio": delta("planning_time_ms"),
        "current_node_types": c["node_types"],
        "baseline_node_types": b["node_types"],
        "node_types_added": sorted(set(c["node_types"]) - set(b["node_types"])),
        "node_types_removed": sorted(set(b["node_types"]) - set(c["node_types"])),
        "current": c,
        "baseline": b,
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("explain_json")
    ap.add_argument("--baseline")
    ns = ap.parse_args()
    cur = json.loads(Path(ns.explain_json).read_text())
    out = compare(cur, json.loads(Path(ns.baseline).read_text())) if ns.baseline else analyze_explain(cur)
    print(json.dumps(out, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
