#!/usr/bin/env python3
"""
One-click beautify/format Markdown files using mdformat.

Examples:
  python scripts/beautify_md.py --run-dir reports/<run_id>
  python scripts/beautify_md.py --path report.md
  python scripts/beautify_md.py --path reports/<run_id> --non-recursive

Notes:
  - Requires: mdformat (python package)
  - Always run with --help first.
"""

from __future__ import annotations

import argparse
import fnmatch
import subprocess
import sys
from pathlib import Path


def _iter_markdown_files(root: Path, *, recursive: bool) -> list[Path]:
    if root.is_file():
        return [root]
    if not root.exists():
        raise FileNotFoundError(str(root))
    if recursive:
        return sorted(p for p in root.rglob("*.md") if p.is_file())
    return sorted(p for p in root.glob("*.md") if p.is_file())


def _should_exclude(path: Path, exclude: list[str]) -> bool:
    s = str(path)
    for pattern in exclude:
        if not pattern:
            continue
        if fnmatch.fnmatch(s, pattern) or pattern in s:
            return True
    return False


def _ensure_mdformat_available() -> None:
    try:
        import mdformat as _  # noqa: F401
    except Exception:
        raise SystemExit(
            "mdformat is not installed. Install (prefer venv) with:\n"
            "  python3 -m pip install mdformat\n"
        )


def main() -> None:
    parser = argparse.ArgumentParser(description="Beautify Markdown files via mdformat")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--run-dir", help="Run directory (e.g. reports/<run_id>/)")
    group.add_argument("--path", help="Markdown file or directory")
    parser.add_argument("--non-recursive", action="store_true", help="Do not recurse when --path is a directory")
    parser.add_argument(
        "--exclude",
        action="append",
        default=[],
        help="Exclude pattern (repeatable). Supports glob match and substring match.",
    )
    parser.add_argument(
        "--mdformat-args",
        default="",
        help='Extra args passed to mdformat (e.g. "--wrap keep").',
    )
    parser.add_argument("--dry-run", action="store_true", help="Print target files without formatting")
    args = parser.parse_args()

    root = Path(args.run_dir or args.path or ".")
    recursive = not args.non_recursive
    files = [p for p in _iter_markdown_files(root, recursive=recursive) if not _should_exclude(p, args.exclude)]

    if not files:
        raise SystemExit(f"No markdown files found under: {root}")

    if args.dry_run:
        for p in files:
            print(str(p))
        return

    _ensure_mdformat_available()

    extra_args = args.mdformat_args.strip().split() if args.mdformat_args.strip() else []
    failures: list[str] = []

    # Format per-file to keep failures isolated.
    for p in files:
        cmd = [sys.executable, "-m", "mdformat", *extra_args, str(p)]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            failures.append(f"{p}: {result.stderr.strip() or result.stdout.strip()}")

    if failures:
        print("\n".join(f"[FAIL] {f}" for f in failures))
        raise SystemExit(1)

    print(f"Formatted {len(files)} markdown file(s).")


if __name__ == "__main__":
    main()
