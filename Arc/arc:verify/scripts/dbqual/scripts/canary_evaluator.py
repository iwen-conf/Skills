#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any

from regression_compare import bootstrap_delta_ci, ratio_delta
from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load


def _series(obj: dict[str, Any], key: str) -> list[float]:
    v=obj.get(key,[])
    if not isinstance(v,list): return []
    return [float(x) for x in v if isinstance(x,(int,float))]


def main() -> int:
    ap=argparse.ArgumentParser(description="Evaluate control vs canary database guardrail metrics with fail-closed sample requirements")
    ap.add_argument("--metrics",required=True,help="JSON with control/canary metric arrays and bake metadata")
    ap.add_argument("--policy",required=True,help="JSON canary policy")
    ap.add_argument("--out",required=True)
    ns=ap.parse_args(); start=time.monotonic(); ev=evidence_base("canary_evaluation","canary.database")
    try:
        data=json_load(Path(ns.metrics)); policy=json_load(Path(ns.policy)); control=data.get("control",{}); canary=data.get("canary",{})
        obs: dict[str,Any]={"comparisons":[],"bake_seconds":data.get("bake_seconds"),"canary_fraction":data.get("canary_fraction")}
        min_bake=float(policy.get("min_bake_seconds",0)); min_samples=int(policy.get("min_samples_per_metric",3))
        if float(data.get("bake_seconds",0))<min_bake: ev["violations"].append(f"bake_seconds {data.get('bake_seconds')} < {min_bake}")
        frac=data.get("canary_fraction")
        if policy.get("min_canary_fraction") is not None and (not isinstance(frac,(int,float)) or frac<float(policy["min_canary_fraction"])): ev["violations"].append("canary fraction below policy")
        for i,rule in enumerate(policy.get("metric",[])):
            key=rule["name"]; c=_series(control,key); k=_series(canary,key)
            row={"name":key,"control_n":len(c),"canary_n":len(k)}
            if len(c)<min_samples or len(k)<min_samples:
                ev["violations"].append(f"{key}: insufficient samples control={len(c)} canary={len(k)} minimum={min_samples}"); obs["comparisons"].append(row); continue
            import statistics
            cm=statistics.median(c); km=statistics.median(k); delta=ratio_delta(km,cm); ci=bootstrap_delta_ci(k,c,seed=20260920+i,samples=int(policy.get("bootstrap_samples",4000)))
            row.update({"control_median":cm,"canary_median":km,"relative_delta":delta,"delta_ci95":ci,"direction":rule.get("direction","lower_is_better")})
            abs_max=rule.get("absolute_max"); abs_min=rule.get("absolute_min")
            if abs_max is not None and km>float(abs_max): ev["violations"].append(f"{key}: canary median {km} > absolute_max {abs_max}")
            if abs_min is not None and km<float(abs_min): ev["violations"].append(f"{key}: canary median {km} < absolute_min {abs_min}")
            max_reg=rule.get("max_relative_regression")
            if max_reg is not None:
                t=float(max_reg); direction=rule.get("direction","lower_is_better")
                if direction=="lower_is_better":
                    if ci and ci["low"]>t: ev["violations"].append(f"{key}: regression CI low {ci['low']:.3f} > {t:.3f}")
                    elif delta is not None and delta>t: ev["warnings"].append(f"{key}: point regression {delta:.3f} exceeds threshold but CI is inconclusive")
                elif direction=="higher_is_better":
                    if ci and ci["high"] < -t: ev["violations"].append(f"{key}: degradation CI high {ci['high']:.3f} < {-t:.3f}")
                    elif delta is not None and delta < -t: ev["warnings"].append(f"{key}: point degradation {-delta:.3f} exceeds threshold but CI is inconclusive")
                else: raise ValueError(f"unsupported direction {direction}")
            obs["comparisons"].append(row)
        ev["observed"]=obs; ev["reproduction"]={"metrics":str(Path(ns.metrics).resolve()),"policy":str(Path(ns.policy).resolve())}
        status="PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status="BLOCKED"
    json_dump(Path(ns.out),finalize_evidence(ev,start,status)); print(json.dumps(ev,ensure_ascii=False,indent=2,default=str))
    return 0 if status=="PASS" else 2

if __name__=="__main__": raise SystemExit(main())
