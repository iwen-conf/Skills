#!/usr/bin/env python3
"""Make every host skill root match schemas/host_profiles.yaml.

--check reports missing, wrong, dangling and unmanaged entries and exits 1 when any exist.
--apply moves unmanaged entries into a timestamped backup (never deletes) and creates the symlinks.
"""
from __future__ import annotations

import argparse
from datetime import datetime
from pathlib import Path
import sys

import yaml

ROOT = Path(__file__).resolve().parents[1]
PROFILES = ROOT / "schemas" / "host_profiles.yaml"


def expand(path: str) -> Path:
    return Path(path).expanduser()


def desired_links(config: dict) -> dict[str, tuple[Path, dict[str, Path], set[str]]]:
    source_root = expand(config["source_root"])
    external = {name: expand(path) for name, path in (config.get("external_targets") or {}).items()}
    result: dict[str, tuple[Path, dict[str, Path], set[str]]] = {}
    for host, spec in config["hosts"].items():
        names: list[str] = []
        for profile in spec.get("profiles", []):
            names.extend(config["profiles"][profile])
        names.extend(spec.get("extra", []))
        links = {name: external.get(name, source_root / name) for name in dict.fromkeys(names)}
        result[host] = (expand(spec["root"]), links, set(spec.get("ignore", [])))
    return result


def inspect(root: Path, links: dict[str, Path], ignore: set[str]) -> list[tuple[str, str]]:
    """Return (kind, name) problems for one host root."""
    problems: list[tuple[str, str]] = []
    for name, target in links.items():
        entry = root / name
        if not (target / "SKILL.md").is_file():
            problems.append(("missing-source", name))
        elif not entry.is_symlink():
            problems.append(("missing" if not entry.exists() else "not-a-link", name))
        elif entry.resolve() != target.resolve():
            problems.append(("wrong-target", name))
    if root.is_dir():
        for entry in sorted(root.iterdir()):
            if entry.name.startswith(".") or entry.name in ignore or entry.name in links:
                continue
            problems.append(("dangling" if entry.is_symlink() and not entry.exists() else "unmanaged", entry.name))
    return problems


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true")
    mode.add_argument("--apply", action="store_true")
    parser.add_argument("--profiles", type=Path, default=PROFILES)
    parser.add_argument("--host", action="append", help="limit to these hosts")
    args = parser.parse_args(argv)
    config = yaml.safe_load(args.profiles.read_text(encoding="utf-8"))
    plan = desired_links(config)
    backup = Path.home() / ".skills-backup" / f"{datetime.now():%Y-%m-%d-%H%M%S}-link-hosts"
    remaining = 0
    for host, (root, links, ignore) in plan.items():
        if args.host and host not in args.host:
            continue
        problems = inspect(root, links, ignore)
        if args.apply:
            root.mkdir(parents=True, exist_ok=True)
            for kind, name in problems:
                entry = root / name
                try:
                    if kind in {"unmanaged", "dangling", "not-a-link", "wrong-target"}:
                        target = backup / host / name
                        target.parent.mkdir(parents=True, exist_ok=True)
                        entry.rename(target)
                    if kind in {"missing", "not-a-link", "wrong-target"}:
                        entry.symlink_to(links[name])
                except OSError as error:
                    print(f"{host:<7} apply stopped at {entry} ({kind}): {error}; entries moved so far are in {backup}")
                    return 1
            problems = inspect(root, links, ignore)
        remaining += len(problems)
        status = "OK" if not problems else ", ".join(f"{kind}:{name}" for kind, name in problems)
        print(f"{host:<7} {root} — {len(links)} entries — {status}")
    if args.apply and backup.exists():
        print(f"Moved entries saved in {backup}")
    return 1 if remaining else 0


if __name__ == "__main__":
    sys.exit(main())
