#!/usr/bin/env python3
"""
Baseline lifecycle management for visual regression testing.

Subcommands:
  init    — Initialize baselines from a run's screenshots
  compare — Batch-compare run screenshots against baselines
  update  — Accept specific screenshots as new baselines
  status  — Show baseline inventory and freshness
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

# visual_diff.py lives in the same directory
_SCRIPTS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(_SCRIPTS_DIR))

from visual_diff import compute_diff, generate_diff_image, sha256_file  # noqa: E402


_MANIFEST_NAME = "baseline-manifest.json"
_SUMMARY_NAME = "visual-diff-summary.json"


def _now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _load_manifest(baseline_dir: Path) -> dict[str, Any]:
    path = baseline_dir / _MANIFEST_NAME
    if path.exists():
        return json.loads(path.read_text(encoding="utf-8"))
    return {"source_run_id": "", "created_at": "", "updated_at": "", "files": []}


def _save_manifest(baseline_dir: Path, manifest: dict[str, Any]) -> None:
    baseline_dir.mkdir(parents=True, exist_ok=True)
    (baseline_dir / _MANIFEST_NAME).write_text(
        json.dumps(manifest, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )


def _find_screenshots(directory: Path) -> list[Path]:
    """Find PNG screenshots in a run directory (screenshots/ subdir or flat)."""
    screenshots_dir = directory / "screenshots"
    search_dir = screenshots_dir if screenshots_dir.is_dir() else directory
    return sorted(search_dir.glob("*.png"))


# ── Subcommands ──


def cmd_init(args: argparse.Namespace) -> None:
    run_dir = Path(args.run_dir)
    baseline_dir = Path(args.baseline_dir)

    screenshots = _find_screenshots(run_dir)
    if not screenshots:
        print(f"[WARN] No screenshots found in {run_dir}")
        raise SystemExit(1)

    baseline_dir.mkdir(parents=True, exist_ok=True)
    files: list[dict[str, str]] = []
    for src in screenshots:
        dest = baseline_dir / src.name
        dest.write_bytes(src.read_bytes())
        files.append({
            "name": src.name,
            "sha256": sha256_file(dest),
            "update_reason": "initial",
        })

    manifest: dict[str, Any] = {
        "source_run_id": run_dir.name,
        "created_at": _now_iso(),
        "updated_at": _now_iso(),
        "files": files,
    }
    _save_manifest(baseline_dir, manifest)
    print(f"OK — initialized {len(files)} baseline(s) in {baseline_dir}")


def cmd_compare(args: argparse.Namespace) -> None:
    run_dir = Path(args.run_dir)
    baseline_dir = Path(args.baseline_dir)
    output_dir = Path(args.output_dir) if args.output_dir else run_dir / "visual-diffs"
    threshold = args.threshold

    output_dir.mkdir(parents=True, exist_ok=True)
    current_shots = _find_screenshots(run_dir)
    baseline_shots = {p.name: p for p in _find_screenshots(baseline_dir)}

    if not baseline_shots:
        print(f"[FAIL] No baselines found in {baseline_dir}", file=sys.stderr)
        raise SystemExit(2)

    results: list[dict[str, Any]] = []
    for current in current_shots:
        baseline = baseline_shots.get(current.name)
        if not baseline:
            results.append({
                "file": current.name,
                "similarity": 0.0,
                "passed": False,
                "diff_image": None,
                "message": "No matching baseline",
            })
            continue

        metrics = compute_diff(baseline, current, mask_regions=None)
        diff_path = output_dir / f"{current.stem}.diff.png"
        generate_diff_image(metrics, diff_path)

        passed = metrics["similarity"] >= threshold
        results.append({
            "file": current.name,
            "similarity": metrics["similarity"],
            "pixel_diff_ratio": metrics["pixel_diff_ratio"],
            "ncc": metrics["ncc"],
            "passed": passed,
            "diff_image": str(diff_path),
        })

    overall_passed = all(r["passed"] for r in results)
    summary: dict[str, Any] = {
        "baseline_dir": str(baseline_dir),
        "run_dir": str(run_dir),
        "threshold": threshold,
        "compared_at": _now_iso(),
        "results": results,
        "overall_passed": overall_passed,
    }

    summary_path = output_dir / _SUMMARY_NAME
    summary_path.write_text(
        json.dumps(summary, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )

    passed_count = sum(1 for r in results if r["passed"])
    total = len(results)

    if args.json_output:
        print(json.dumps(summary, indent=2))
    else:
        status = "PASS" if overall_passed else "FAIL"
        print(f"[{status}] {passed_count}/{total} screenshots within threshold ({threshold})")
        for r in results:
            mark = "PASS" if r["passed"] else "FAIL"
            sim = r.get("similarity", 0)
            print(f"  [{mark}] {r['file']} similarity={sim:.4f}")

    if args.fail_on_diff and not overall_passed:
        raise SystemExit(1)


def cmd_update(args: argparse.Namespace) -> None:
    run_dir = Path(args.run_dir)
    baseline_dir = Path(args.baseline_dir)
    reason = args.reason or "manual update"

    file_names = [f.strip() for f in args.files.split(",") if f.strip()]
    if not file_names:
        print("[FAIL] No files specified for update", file=sys.stderr)
        raise SystemExit(1)

    manifest = _load_manifest(baseline_dir)
    existing = {f["name"]: f for f in manifest["files"]}

    updated = 0
    for name in file_names:
        src = _find_source(run_dir, name)
        if not src:
            print(f"[WARN] Screenshot not found: {name}")
            continue
        dest = baseline_dir / name
        dest.write_bytes(src.read_bytes())
        entry = {
            "name": name,
            "sha256": sha256_file(dest),
            "update_reason": reason,
        }
        existing[name] = entry
        updated += 1

    manifest["files"] = list(existing.values())
    manifest["updated_at"] = _now_iso()
    _save_manifest(baseline_dir, manifest)
    print(f"OK — updated {updated} baseline(s), reason: {reason}")


def _find_source(run_dir: Path, name: str) -> Path | None:
    candidates = [run_dir / "screenshots" / name, run_dir / name]
    for c in candidates:
        if c.exists():
            return c
    return None


def cmd_status(args: argparse.Namespace) -> None:
    baseline_dir = Path(args.baseline_dir)
    manifest = _load_manifest(baseline_dir)

    if not manifest["files"]:
        print(f"No baselines in {baseline_dir}")
        return

    print(f"Baseline directory: {baseline_dir}")
    print(f"Source run: {manifest.get('source_run_id', 'unknown')}")
    print(f"Created: {manifest.get('created_at', 'unknown')}")
    print(f"Updated: {manifest.get('updated_at', 'unknown')}")
    print(f"Files ({len(manifest['files'])}):")
    for f in manifest["files"]:
        exists = (baseline_dir / f["name"]).exists()
        mark = "OK" if exists else "MISSING"
        print(f"  [{mark}] {f['name']} (reason: {f.get('update_reason', '-')})")


def main() -> None:
    parser = argparse.ArgumentParser(description="Visual regression baseline management")
    sub = parser.add_subparsers(dest="command", required=True)

    # init
    p_init = sub.add_parser("init", help="Initialize baselines from a run")
    p_init.add_argument("--run-dir", required=True, help="Run directory with screenshots/")
    p_init.add_argument("--baseline-dir", required=True, help="Target baseline directory")

    # compare
    p_cmp = sub.add_parser("compare", help="Compare run screenshots against baselines")
    p_cmp.add_argument("--run-dir", required=True)
    p_cmp.add_argument("--baseline-dir", required=True)
    p_cmp.add_argument("--threshold", type=float, default=0.95)
    p_cmp.add_argument("--output-dir", help="Output dir for diffs (default: <run-dir>/visual-diffs)")
    p_cmp.add_argument("--json", action="store_true", dest="json_output")
    p_cmp.add_argument("--fail-on-diff", action="store_true")

    # update
    p_upd = sub.add_parser("update", help="Accept screenshots as new baselines")
    p_upd.add_argument("--run-dir", required=True)
    p_upd.add_argument("--baseline-dir", required=True)
    p_upd.add_argument("--files", required=True, help="Comma-separated filenames to update")
    p_upd.add_argument("--reason", default="", help="Reason for update")

    # status
    p_st = sub.add_parser("status", help="Show baseline inventory")
    p_st.add_argument("--baseline-dir", required=True)

    args = parser.parse_args()
    {"init": cmd_init, "compare": cmd_compare, "update": cmd_update, "status": cmd_status}[args.command](args)


if __name__ == "__main__":
    main()
