#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import shutil
from pathlib import Path

from sqlverify_common import (
    evidence_files,
    evidence_hashes,
    git_metadata,
    json_dump,
    json_load,
    load_run_manifest,
    now_iso,
    sha256_file,
    verify_candidate,
)
from release_gate import revalidate_gate


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Promote a PASS SQL verification evidence set to a versioned performance baseline"
    )
    ap.add_argument("--evidence-dir", required=True)
    ap.add_argument("--gate", required=True)
    ap.add_argument("--destination", required=True)
    ap.add_argument("--root", default=".")
    ap.add_argument("--allow-nonpass", action="store_true", help="explicit emergency override; manifest records it")
    ns = ap.parse_args()
    src = Path(ns.evidence_dir).resolve()
    gate_path = Path(ns.gate).resolve()
    dst = Path(ns.destination).resolve()
    gate = json_load(gate_path)
    if gate.get("status") != "PASS" and not ns.allow_nonpass:
        raise SystemExit(f"refusing baseline promotion: release gate is {gate.get('status')!r}, not PASS")
    if not src.is_dir():
        raise SystemExit(f"evidence directory not found: {src}")
    # The gate verdict covers one run; promoting any other evidence would bless unjudged numbers.
    run, issues = load_run_manifest(src)
    if not run or run["run_id"] != gate.get("run_id"):
        raise SystemExit(
            f"refusing baseline promotion: gate run_id {gate.get('run_id')!r} does not match evidence run "
            f"{run['run_id'] if run else None!r} {issues or ''}"
        )
    evidence, validation_issues = revalidate_gate(gate, src, Path(ns.root))
    if issues or validation_issues:
        raise SystemExit(
            f"refusing baseline promotion: gated run could not be revalidated: {issues + validation_issues}"
        )
    binding = gate["input_binding"]
    current_hashes = evidence_hashes(src)
    files = evidence_files(src)
    if not evidence:
        raise SystemExit("no evidence JSON files to promote")
    if dst.exists() and any(dst.iterdir()):
        raise SystemExit(f"destination must be absent or empty: {dst}")
    dst.mkdir(parents=True, exist_ok=True)
    manifest_files = []
    for p in files:
        name = p.relative_to(src).as_posix()
        target = dst / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(p, target)
        digest = sha256_file(target)
        if digest != current_hashes[name]:
            raise SystemExit(f"refusing baseline promotion: {name} changed while copying")
        manifest_files.append({"name": name, "sha256": digest})
    if evidence_hashes(src) != current_hashes or verify_candidate(run, Path(ns.root)):
        raise SystemExit("refusing baseline promotion: inputs changed while copying")
    manifest = {
        "schema_version": 1,
        "created_at": now_iso(),
        "source_evidence_dir": str(src),
        "gate_status": gate.get("status"),
        "run_id": run["run_id"],
        "input_binding": binding,
        "override_nonpass": bool(ns.allow_nonpass and gate.get("status") != "PASS"),
        "git": git_metadata(Path(ns.root).resolve()),
        "files": manifest_files,
    }
    json_dump(dst / "baseline-manifest.json", manifest)
    print(
        json.dumps(
            {"status": "PASS", "destination": str(dst), "files": len(files), "gate_status": gate.get("status")},
            ensure_ascii=False,
            indent=2,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
