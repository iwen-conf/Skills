#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

from sqlverify_common import percentile, mean


def parse_files(paths: list[Path], *, has_rate: bool, has_retries: bool) -> dict[str, Any]:
    latencies_ms: list[float] = []
    lag_ms: list[float] = []
    scheduled_latency_ms: list[float] = []
    retries: list[int] = []
    by_script: dict[int, list[float]] = defaultdict(list)
    failures: Counter[str] = Counter()
    total = 0

    for path in paths:
        with path.open("r", encoding="utf-8", errors="replace") as f:
            for raw in f:
                raw = raw.strip()
                if not raw:
                    continue
                parts = raw.split()
                if len(parts) < 6:
                    continue
                total += 1
                time_field = parts[2]
                try:
                    script_no = int(parts[3])
                except ValueError:
                    script_no = -1
                tx_ms = None
                if time_field.isdigit():
                    tx_ms = int(time_field) / 1000.0
                    latencies_ms.append(tx_ms)
                    by_script[script_no].append(tx_ms)
                else:
                    failures[time_field] += 1
                idx = 6
                if has_rate and len(parts) > idx:
                    try:
                        lag = int(parts[idx]) / 1000.0
                        lag_ms.append(lag)
                        if tx_ms is not None:
                            scheduled_latency_ms.append(tx_ms + lag)
                    except ValueError:
                        pass
                    idx += 1
                if has_retries and len(parts) > idx:
                    try:
                        retries.append(int(parts[idx]))
                    except ValueError:
                        pass

    ok = len(latencies_ms)
    return {
        "transactions_logged": total,
        "successful_logged": ok,
        "failures": dict(failures),
        "failure_rate": (sum(failures.values()) / total) if total else None,
        "latency_ms": {
            "mean": mean(latencies_ms),
            "p50": percentile(latencies_ms, 0.50),
            "p90": percentile(latencies_ms, 0.90),
            "p95": percentile(latencies_ms, 0.95),
            "p99": percentile(latencies_ms, 0.99),
            "p999": percentile(latencies_ms, 0.999),
            "max": max(latencies_ms) if latencies_ms else None,
        },
        "schedule_lag_ms": {
            "p50": percentile(lag_ms, 0.50),
            "p95": percentile(lag_ms, 0.95),
            "p99": percentile(lag_ms, 0.99),
            "max": max(lag_ms) if lag_ms else None,
        } if lag_ms else None,
        "scheduled_latency_ms": {
            "p50": percentile(scheduled_latency_ms, 0.50),
            "p95": percentile(scheduled_latency_ms, 0.95),
            "p99": percentile(scheduled_latency_ms, 0.99),
            "max": max(scheduled_latency_ms) if scheduled_latency_ms else None,
        } if scheduled_latency_ms else None,
        "retries": {
            "transactions_with_retry": sum(1 for x in retries if x > 0),
            "total_retries": sum(retries),
            "max_retries": max(retries) if retries else 0,
        } if retries else None,
        "per_script": {
            str(k): {
                "count": len(v),
                "p50_ms": percentile(v, 0.50),
                "p95_ms": percentile(v, 0.95),
                "p99_ms": percentile(v, 0.99),
                "max_ms": max(v) if v else None,
            } for k, v in sorted(by_script.items())
        },
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="Parse PostgreSQL pgbench per-transaction logs")
    ap.add_argument("paths", nargs="+")
    ap.add_argument("--rate", action="store_true", help="logs contain schedule_lag column")
    ap.add_argument("--retries", action="store_true", help="logs contain retries column")
    ns = ap.parse_args()
    print(json.dumps(parse_files([Path(p) for p in ns.paths], has_rate=ns.rate, has_retries=ns.retries), ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
