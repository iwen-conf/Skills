#!/usr/bin/env python3
"""Align managed host skill links with schemas/host_profiles.yaml.

--check fails on drift in declared links and known Arc entrypoints, and on standalone
copies of skills that a host's declared plugins already ship. Unrelated entries
are reported but preserved. --apply backs up managed drift and creates declared links.
--prune-unmanaged also checks or backs up unrelated entries; nothing is deleted.
"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
PROFILES = ROOT / "schemas" / "host_profiles.yaml"
sys.path.insert(0, str(ROOT / "src"))

from arc_core.domain.skill import LIFECYCLE_PHASES, RETIRED_ARC_SKILLS

# Only names owned by this repository, never every user-created `arc-*` skill.
ARC_SKILL_NAMES = {
    published
    for name in (*LIFECYCLE_PHASES, *RETIRED_ARC_SKILLS)
    for published in (name, name.replace(":", "-"))
}


def expand(path: str) -> Path:
    return Path(path).expanduser()


def plugin_skills(plugin_root: Path, plugins: list[str]) -> set[str]:
    """Skill names an installed plugin already ships to its host."""
    return {
        skill.parent.name
        for plugin in plugins
        for skill in (plugin_root / plugin / "skills").glob("*/SKILL.md")
    }


def desired_links(
    config: dict,
) -> dict[str, tuple[Path, dict[str, Path], set[str], set[str]]]:
    source_root = expand(config["source_root"])
    plugin_root = expand(config.get("plugin_root") or str(ROOT / "plugins"))
    external = {
        name: expand(path)
        for name, path in (config.get("external_targets") or {}).items()
    }
    result: dict[str, tuple[Path, dict[str, Path], set[str], set[str]]] = {}
    for host, spec in config["hosts"].items():
        names: list[str] = []
        for profile in spec.get("profiles", []):
            names.extend(config["profiles"][profile])
        names.extend(spec.get("extra", []))
        shipped = plugin_skills(plugin_root, spec.get("plugins", []))
        links = {
            name: external.get(name, source_root / name)
            for name in dict.fromkeys(names)
            if name not in shipped
        }
        result[host] = (
            expand(spec["root"]),
            links,
            set(spec.get("ignore", [])),
            shipped,
        )
    return result


def inspect(
    root: Path,
    links: dict[str, Path],
    ignore: set[str],
    shipped: set[str] | None = None,
) -> list[tuple[str, str]]:
    """Return all (kind, name) findings, including preserved unrelated entries."""
    shipped = shipped or set()
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
            if (
                entry.name.startswith(".")
                or entry.name in ignore
                or entry.name in links
            ):
                continue
            problems.append(
                (
                    "plugin-duplicate"
                    if entry.name in shipped
                    else "dangling"
                    if entry.is_symlink() and not entry.exists()
                    else "unmanaged",
                    entry.name,
                )
            )
    return problems


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true")
    mode.add_argument("--apply", action="store_true")
    parser.add_argument("--profiles", type=Path, default=PROFILES)
    parser.add_argument("--host", action="append", help="limit to these hosts")
    parser.add_argument(
        "--prune-unmanaged",
        action="store_true",
        help="also fail on unrelated entries in --check, or back them up in --apply",
    )
    args = parser.parse_args(argv)
    config = yaml.safe_load(args.profiles.read_text(encoding="utf-8"))
    plan = desired_links(config)
    unknown = sorted(set(args.host or ()) - plan.keys())
    if unknown:
        parser.error(
            f"unknown host(s): {', '.join(unknown)}; available hosts: {', '.join(sorted(plan))}"
        )
    backup = (
        Path.home() / ".skills-backup" / f"{datetime.now():%Y-%m-%d-%H%M%S}-link-hosts"
    )
    remaining = 0
    for host, (root, links, ignore, shipped) in plan.items():
        if args.host and host not in args.host:
            continue
        managed = links.keys() | ARC_SKILL_NAMES | shipped
        findings = inspect(root, links, ignore, shipped)
        problems = [
            finding
            for finding in findings
            if args.prune_unmanaged or finding[1] in managed
        ]
        if args.apply:
            root.mkdir(parents=True, exist_ok=True)
            for kind, name in problems:
                entry = root / name
                try:
                    if kind in {
                        "unmanaged",
                        "dangling",
                        "not-a-link",
                        "wrong-target",
                        "plugin-duplicate",
                    }:
                        target = backup / host / name
                        target.parent.mkdir(parents=True, exist_ok=True)
                        entry.rename(target)
                    if kind in {"missing", "not-a-link", "wrong-target"}:
                        entry.symlink_to(links[name])
                except OSError as error:
                    print(
                        f"{host:<7} apply stopped at {entry} ({kind}): {error}; entries moved so far are in {backup}"
                    )
                    return 1
            findings = inspect(root, links, ignore, shipped)
            problems = [
                finding
                for finding in findings
                if args.prune_unmanaged or finding[1] in managed
            ]
        remaining += len(problems)
        status = (
            "OK"
            if not problems
            else ", ".join(f"{kind}:{name}" for kind, name in problems)
        )
        preserved = [finding for finding in findings if finding not in problems]
        if preserved:
            status += "; preserved unrelated entries: " + ", ".join(
                f"{kind}:{name}" for kind, name in preserved
            )
        print(f"{host:<7} {root} — {len(links)} entries — {status}")
    if args.apply and backup.exists():
        print(f"Moved entries saved in {backup}")
    return 1 if remaining else 0


if __name__ == "__main__":
    sys.exit(main())
