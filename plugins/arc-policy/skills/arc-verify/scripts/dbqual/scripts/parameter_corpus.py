#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import random
from pathlib import Path
from typing import Any


def generate(spec: dict[str, Any]) -> dict[str, Any]:
    seed = int(spec.get("seed", 1))
    rng = random.Random(seed)
    cases: list[dict[str, Any]] = []
    seen = set()

    def add(name: str, params: list[Any], source: str):
        key = json.dumps(params, sort_keys=True, ensure_ascii=False, default=str)
        if key in seen:
            return
        seen.add(key)
        cases.append({"name": name, "params": params, "source": source})

    for c in spec.get("fixed_cases", []):
        add(c["name"], c["params"], c.get("source", "fixed"))

    params = spec.get("parameters", [])
    runs = int(spec.get("random_runs", 0))
    for i in range(runs):
        row = []
        for p in params:
            values = p["values"]
            weights = p.get("weights")
            if weights:
                row.append(rng.choices(values, weights=weights, k=1)[0])
            else:
                row.append(rng.choice(values))
        add(f"generated-{i+1:04d}", row, "seeded_weighted" if any(x.get("weights") for x in params) else "seeded_choice")

    return {"schema_version": 1, "seed": seed, "count": len(cases), "cases": cases}


def main() -> int:
    ap = argparse.ArgumentParser(description="Build a deterministic business-parameter corpus; explicit boundary/incident cases come first")
    ap.add_argument("spec")
    ap.add_argument("--out", required=True)
    ns = ap.parse_args()
    spec = json.loads(Path(ns.spec).read_text(encoding="utf-8"))
    out = generate(spec)
    Path(ns.out).write_text(json.dumps(out, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"out": ns.out, "seed": out["seed"], "count": out["count"]}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
