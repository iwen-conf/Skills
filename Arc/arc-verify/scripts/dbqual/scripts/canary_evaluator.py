#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import time
from pathlib import Path
from typing import Any

from regression_compare import bootstrap_delta_ci, ratio_delta
from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load


def _number(value: Any, label: str) -> float:
    if type(value) not in (int, float) or not math.isfinite(value):
        raise ValueError(f"{label}: a finite numeric observation/policy is required")
    return float(value)


def _series(obj: dict[str, Any], key: str) -> list[float]:
    values = obj.get(key)
    if not isinstance(values, list):
        raise ValueError(f"{key}: metric samples are missing")
    return [_number(value, f"{key} sample") for value in values]


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Evaluate control vs canary database guardrail metrics with fail-closed sample requirements"
    )
    ap.add_argument("--metrics", required=True, help="JSON with control/canary metric arrays and bake metadata")
    ap.add_argument("--policy", required=True, help="JSON canary policy")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    start = time.monotonic()
    ev = evidence_base("canary_evaluation", "canary.database")
    blockers: list[str] = []
    obs: dict[str, Any] = {"comparisons": [], "blockers": blockers}
    ev["observed"] = obs
    ev["reproduction"] = {"metrics": str(Path(ns.metrics).resolve()), "policy": str(Path(ns.policy).resolve())}
    try:
        data = json_load(Path(ns.metrics))
        policy = json_load(Path(ns.policy))
        control = data.get("control", {})
        canary = data.get("canary", {})
        obs.update({"bake_seconds": data.get("bake_seconds"), "canary_fraction": data.get("canary_fraction")})
        min_samples = int(policy.get("min_samples_per_metric", 3))
        if min_samples < 1:
            raise ValueError("minimum sample count must be positive")
        for field, threshold in (("bake_seconds", "min_bake_seconds"), ("canary_fraction", "min_canary_fraction")):
            if policy.get(threshold) is None:
                continue
            try:
                actual = _number(data.get(field), field)
                minimum = _number(policy[threshold], threshold)
                if actual < minimum:
                    ev["violations"].append(f"{field} {actual} < {minimum}")
            except Exception as e:
                blockers.append(str(e))
        for i, rule in enumerate(policy.get("metric", [])):
            key = rule["name"]
            row = {"name": key}
            obs["comparisons"].append(row)
            try:
                if not any(
                    rule.get(field) is not None for field in ("absolute_max", "absolute_min", "max_relative_regression")
                ):
                    raise ValueError(f"{key}: no explicit metric comparator")
                c = _series(control, key)
                k = _series(canary, key)
                row.update({"control_n": len(c), "canary_n": len(k)})
                if len(c) < min_samples or len(k) < min_samples:
                    raise ValueError(
                        f"{key}: insufficient samples control={len(c)} canary={len(k)} minimum={min_samples}"
                    )
                import statistics

                cm = statistics.median(c)
                km = statistics.median(k)
                delta = ratio_delta(km, cm)
                row.update(
                    {
                        "control_median": cm,
                        "canary_median": km,
                        "relative_delta": delta,
                        "direction": rule.get("direction", "lower_is_better"),
                    }
                )
                abs_max = rule.get("absolute_max")
                abs_min = rule.get("absolute_min")
                if abs_max is not None and km > _number(abs_max, f"{key} absolute_max"):
                    ev["violations"].append(f"{key}: canary median {km} > absolute_max {abs_max}")
                if abs_min is not None and km < _number(abs_min, f"{key} absolute_min"):
                    ev["violations"].append(f"{key}: canary median {km} < absolute_min {abs_min}")
                max_reg = rule.get("max_relative_regression")
                if max_reg is not None:
                    t = _number(max_reg, f"{key} max_relative_regression")
                    direction = rule.get("direction", "lower_is_better")
                    ci = bootstrap_delta_ci(k, c, seed=20260920 + i, samples=int(policy.get("bootstrap_samples", 4000)))
                    row["delta_ci95"] = ci
                    if delta is None or ci is None:
                        raise ValueError(f"{key}: relative comparison is undefined")
                    if direction == "lower_is_better":
                        if ci["low"] > t:
                            ev["violations"].append(f"{key}: regression CI low {ci['low']:.3f} > {t:.3f}")
                        elif delta > t:
                            blockers.append(
                                f"{key}: point regression {delta:.3f} exceeds threshold but CI is inconclusive"
                            )
                    elif direction == "higher_is_better":
                        if ci["high"] < -t:
                            ev["violations"].append(f"{key}: degradation CI high {ci['high']:.3f} < {-t:.3f}")
                        elif delta < -t:
                            blockers.append(
                                f"{key}: point degradation {-delta:.3f} exceeds threshold but CI is inconclusive"
                            )
                    else:
                        raise ValueError(f"unsupported direction {direction}")
            except Exception as e:
                blockers.append(str(e))
        if not policy.get("metric"):
            blockers.append("canary policy declares no metric; nothing was compared")
    except Exception as e:
        blockers.append(repr(e))
    ev["warnings"].extend(blockers)
    status = "FAIL" if ev["violations"] else "BLOCKED" if blockers else "PASS"
    json_dump(Path(ns.out), finalize_evidence(ev, start, status))
    print(json.dumps(ev, ensure_ascii=False, indent=2, default=str))
    return 0 if status == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
