#!/usr/bin/env python3
"""Sync Arc skills from this repo (SSOT) into agent skill directories.

Default destination: ~/.agents/skills
Writes both colon ids (arc:name) and dash aliases (arc-name) used by hosts
that forbid ':' in skill names.
"""
from __future__ import annotations

import argparse
import re
import shutil
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ARC = ROOT / "Arc"
DEFAULT_DEST = Path.home() / ".agents" / "skills"

REASONIX_BANNER = (
    "# Reasonix-compatible alias of `{dash_name}` "
    '(skill names cannot contain ":")\n\n'
)


def list_skill_dirs() -> list[Path]:
    dirs: list[Path] = []
    for path in sorted(ARC.iterdir()):
        if path.is_dir() and path.name.startswith("arc:") and (path / "SKILL.md").is_file():
            dirs.append(path)
    return dirs


def dash_name(colon_name: str) -> str:
    return colon_name.replace(":", "-")


def transform_skill_md_for_dash(text: str, colon_name: str) -> str:
    """Rewrite frontmatter name and arc: refs for dash-only hosts."""
    dash = dash_name(colon_name)
    if not text.startswith("---"):
        body = text.replace(f"`{colon_name}`", f"`{dash}`").replace(colon_name, dash)
        return REASONIX_BANNER.format(dash_name=dash) + body

    parts = text.split("---", 2)
    if len(parts) < 3:
        body = text.replace(colon_name, dash)
        return REASONIX_BANNER.format(dash_name=dash) + body

    fm, body = parts[1], parts[2]
    # Replace name: arc:xxx only in frontmatter
    fm = re.sub(
        r"(?m)^name:\s*[\"']?" + re.escape(colon_name) + r"[\"']?\s*$",
        f"name: {dash}",
        fm,
    )
    # Also normalize quoted names and arc: cross-refs in body
    body_out = body
    # Prefer backtick-safe replacement for skill ids
    body_out = body_out.replace(f"`{colon_name}`", f"`{dash}`")
    body_out = re.sub(r"\barc:([a-z0-9-]+)\b", r"arc-\1", body_out)
    # docs paths that mention arc: stay as-is if any (none expected)
    return f"---{fm}---\n" + REASONIX_BANNER.format(dash_name=dash) + body_out.lstrip("\n")


def copy_tree(src: Path, dest: Path, *, dry_run: bool) -> None:
    if dry_run:
        print(f"DRY  {src} -> {dest}")
        return
    if dest.exists():
        shutil.rmtree(dest)
    shutil.copytree(
        src,
        dest,
        ignore=shutil.ignore_patterns("__pycache__", "*.pyc", ".DS_Store"),
    )


def sync_one(src: Path, dest_root: Path, *, dry_run: bool) -> None:
    colon = src.name  # arc:foo
    dash = dash_name(colon)

    colon_dest = dest_root / colon
    dash_dest = dest_root / dash

    # Colon install: faithful copy
    copy_tree(src, colon_dest, dry_run=dry_run)

    # Dash install: copy then rewrite SKILL.md
    if dry_run:
        print(f"DRY  {src} -> {dash_dest} (dash transform)")
        return

    copy_tree(src, dash_dest, dry_run=False)
    skill_md = dash_dest / "SKILL.md"
    original = skill_md.read_text(encoding="utf-8")
    skill_md.write_text(transform_skill_md_for_dash(original, colon), encoding="utf-8")

    # Rewrite relative mentions of other skills inside references if present
    for path in dash_dest.rglob("*.md"):
        if path.name == "SKILL.md":
            continue
        text = path.read_text(encoding="utf-8")
        new = re.sub(r"\barc:([a-z0-9-]+)\b", r"arc-\1", text)
        if new != text:
            path.write_text(new, encoding="utf-8")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--dest",
        type=Path,
        default=DEFAULT_DEST,
        help=f"Destination skills root (default: {DEFAULT_DEST})",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print planned copies without writing",
    )
    parser.add_argument(
        "--only",
        action="append",
        default=[],
        help="Sync only these skill ids (e.g. arc:build). Repeatable.",
    )
    args = parser.parse_args(argv)

    skills = list_skill_dirs()
    if args.only:
        wanted = set(args.only)
        skills = [s for s in skills if s.name in wanted]
        missing = wanted - {s.name for s in skills}
        if missing:
            print(f"Unknown skills: {', '.join(sorted(missing))}", file=sys.stderr)
            return 1

    if not skills:
        print("No Arc skills found under Arc/", file=sys.stderr)
        return 1

    dest_root: Path = args.dest.expanduser()
    if not args.dry_run:
        dest_root.mkdir(parents=True, exist_ok=True)

    for src in skills:
        sync_one(src, dest_root, dry_run=args.dry_run)
        print(f"OK   {src.name} -> {dest_root / src.name} and {dest_root / dash_name(src.name)}")

    print(f"Synced {len(skills)} skill(s) to {dest_root}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
