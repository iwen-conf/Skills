#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import shutil
from pathlib import Path

from sqlverify_common import git_metadata, json_dump, json_load, now_iso, sha256_file


def main() -> int:
    ap = argparse.ArgumentParser(description="Promote a PASS SQL verification evidence set to a versioned performance baseline")
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument("--gate", required=True)
    ap.add_argument("--destination", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--allow-nonpass", action="store_true", help="explicit emergency override; manifest records it")
    ns = ap.parse_args()
    src = Path(ns.evidence_dir).resolve(); gate_path = Path(ns.gate).resolve(); dst = Path(ns.destination).resolve()
    gate = json_load(gate_path)
    if gate.get("status") != "PASS" and not ns.allow_nonpass:
        raise SystemExit(f"refusing baseline promotion: release gate is {gate.get('status')!r}, not PASS")
    if not src.is_dir():
        raise SystemExit(f"evidence directory not found: {src}")
    files = sorted(p for p in src.glob("*.json") if p.is_file() and p.name != "regression.baseline.json")
    if not files:
        raise SystemExit("no evidence JSON files to promote")
    if dst.exists() and any(dst.iterdir()):
        raise SystemExit(f"destination must be absent or empty: {dst}")
    dst.mkdir(parents=True, exist_ok=True)
    manifest_files = []
    for p in files:
        target = dst / p.name
        shutil.copy2(p, target)
        manifest_files.append({"name": p.name, "sha256": sha256_file(target)})
    manifest = {
        "schema_version": 1,
        "created_at": now_iso(),
        "source_evidence_dir": str(src),
        "gate_status": gate.get("status"),
        "override_nonpass": bool(ns.allow_nonpass and gate.get("status") != "PASS"),
        "git": git_metadata(Path(ns.root).resolve()),
        "files": manifest_files,
    }
    json_dump(dst / "baseline-manifest.json", manifest)
    print(json.dumps({"status":"PASS","destination":str(dst),"files":len(files),"gate_status":gate.get("status")}, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
