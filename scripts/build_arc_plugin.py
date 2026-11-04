#!/usr/bin/env python3
"""Generate the checked-in Arc plugin distribution from its canonical sources.

Arc/ owns the six workflow skills; plugins/arc-policy/plugin.json owns the plugin
identity and version. Run this after changing either source, then use --check in
repository verification. The generated skills are real files so a marketplace
can copy only plugins/arc-policy without losing their referenced resources.

This bundles skills and the existing policy engine, not external executables or
a Python environment. Runtime provisioning remains a separate installation step.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from scripts import sync_skills

PLUGIN = Path("plugins/arc-policy")
BRIDGE_SKILL = "arc-policy"
PINNED_SKILL = "arch-diagram"
METADATA_PATHS = (
    PLUGIN / ".claude-plugin/plugin.json",
    PLUGIN / ".codex-plugin/plugin.json",
    PLUGIN / "lib/arc_policy/__init__.py",
    Path(".claude-plugin/marketplace.json"),
    Path(".grok-plugin/marketplace.json"),
    Path(".agents/plugins/marketplace.json"),
)
LEGACY_MARKETPLACE = Path("plugins/marketplace.json")
# Match sync_skills.build_installation's copytree exclusions. Executing a bundled
# Python tool must not turn its bytecode or local tool caches into source drift.
IGNORED_PACKAGE_FILES = shutil.ignore_patterns(
    "__pycache__",
    "*.pyc",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    ".DS_Store",
    ".git",
    "node_modules",
)
SEMVER = re.compile(
    r"(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
)


def canonical_manifest(root: Path) -> dict:
    path = root / PLUGIN / "plugin.json"
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or data.get("name") != "arc-policy":
        raise ValueError(f"{path}: expected the arc-policy portable manifest")
    version = data.get("version")
    if not isinstance(version, str) or not SEMVER.fullmatch(version):
        raise ValueError(f"{path}: version must be semantic version text")
    if not isinstance(data.get("description"), str) or not data["description"]:
        raise ValueError(f"{path}: description is required")
    return data


def _json(data: dict) -> str:
    return json.dumps(data, indent=2, ensure_ascii=False) + "\n"


def generated_metadata(manifest: dict) -> dict[Path, str]:
    """Keep each host's existing native format; portable components stay portable."""
    name, version, description = (
        manifest[key] for key in ("name", "version", "description")
    )
    identity = {"name": name, "version": version, "description": description}
    claude = dict(identity)
    for key in ("author", "keywords"):
        if key in manifest:
            claude[key] = manifest[key]
    claude.update(skills="./skills/", hooks="./hooks/hooks.json")
    codex = dict(identity, skills="./skills/", hooks="./hooks/codex.json")
    common = {
        "name": "skills",
        "description": "Arc skills and the Arc Policy Runtime",
        "owner": {"name": "Arc"},
    }
    entry = {"name": name, "description": description}
    claude_marketplace = dict(
        common,
        plugins=[dict(entry, source="./plugins/arc-policy", version=version)],
    )
    grok_marketplace = dict(
        common,
        plugins=[
            dict(
                entry,
                category="development",
                source={"type": "local", "path": "./plugins/arc-policy"},
                version=version,
                keywords=manifest.get("keywords", []),
            )
        ],
    )
    # Codex resolves a local source against the repository root of this supported
    # .agents/plugins/marketplace.json layout, not the JSON file's directory.
    codex_marketplace = {
        "name": "skills",
        "plugins": [
            {
                "name": name,
                "source": {"source": "local", "path": "./plugins/arc-policy"},
                "policy": {
                    "installation": "AVAILABLE",
                    "authentication": "ON_INSTALL",
                },
                "category": "Productivity",
                "interface": {"displayName": "Arc Policy Runtime"},
            }
        ],
    }
    return dict(
        zip(
            METADATA_PATHS,
            (
                _json(claude),
                _json(codex),
                (
                    '"""Arc Policy Runtime: skills guide, hooks intercept, CLI verifies."""\n'
                    f"\n__version__ = {json.dumps(version)}\n"
                ),
                _json(claude_marketplace),
                _json(grok_marketplace),
                _json(codex_marketplace),
            ),
            strict=True,
        )
    )


def build_generated(root: Path, stage: Path) -> list[str]:
    """Build and validate all generated content before changing the checkout."""
    manifest = canonical_manifest(root)
    selected = sync_skills.selectable_names(root, ["Arc"])
    if not selected:
        raise ValueError("no Arc workflow skills found")
    if BRIDGE_SKILL in selected or "arc:policy" in selected:
        raise ValueError(
            "the policy bridge is maintained separately from Arc workflows"
        )
    skills = stage / PLUGIN / "skills"
    included = sync_skills.build_installation(root, skills, selected, "dash")
    # A plugin copy cannot rely on separately installed sibling skills.
    errors = sync_skills.validate_installation(skills)
    if errors:
        raise ValueError("plugin skill closure is incomplete:\n" + "\n".join(errors))
    # The reviewed upstream bundle is the canonical source, not generated Arc prose.
    # Verify it locally; a build or hook never downloads or silently updates it.
    sys.path.insert(0, str(ROOT / PLUGIN / "lib"))
    from arc_policy.capabilities import check_pinned_skill

    pinned = root / PLUGIN / "skills" / PINNED_SKILL
    check_pinned_skill(pinned)
    shutil.copytree(pinned, skills / PINNED_SKILL)
    for relative, content in generated_metadata(manifest).items():
        destination = stage / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(content, encoding="utf-8")
    return included


def _managed_files(root: Path) -> dict[Path, tuple[str, int]]:
    """Fingerprint published files and modes, excluding transient tool caches."""
    candidates = [root / path for path in METADATA_PATHS]
    skills = root / PLUGIN / "skills"
    if skills.is_symlink():
        raise ValueError(f"generated skill root must not be a symlink: {skills}")
    for directory in sorted([*skills.glob("arc-*"), skills / PINNED_SKILL]):
        if directory.name == BRIDGE_SKILL:
            continue
        candidates.append(directory)
        if not directory.is_symlink():
            for current, dirs, files in os.walk(directory, followlinks=False):
                ignored = IGNORED_PACKAGE_FILES(current, dirs + files)
                dirs[:] = sorted(name for name in dirs if name not in ignored)
                candidates.extend(
                    Path(current) / name
                    for name in [*dirs, *sorted(files)]
                    if name not in ignored
                )
    result = {}
    for path in candidates:
        if path.is_symlink():
            raise ValueError(f"generated plugin content must not be a symlink: {path}")
        if path.is_file():
            result[path.relative_to(root)] = (
                hashlib.sha256(path.read_bytes()).hexdigest(),
                path.stat().st_mode & 0o111,
            )
    return result


def drift(root: Path, stage: Path) -> list[str]:
    actual, expected = _managed_files(root), _managed_files(stage)
    problems = []
    for path in sorted(actual.keys() | expected.keys()):
        if path not in actual:
            problems.append(f"missing: {path}")
        elif path not in expected:
            problems.append(f"unexpected generated file: {path}")
        elif actual[path] != expected[path]:
            problems.append(f"changed: {path}")
    if (root / LEGACY_MARKETPLACE).exists():
        problems.append(f"unsupported Codex marketplace location: {LEGACY_MARKETPLACE}")
    return problems


def publish_generated(root: Path, stage: Path) -> None:
    """Replace only generated workflow skills and metadata; keep the bridge intact."""
    actual, expected = _managed_files(root), _managed_files(stage)
    for relative in sorted(expected):
        if actual.get(relative) == expected[relative]:
            continue
        destination = root / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(stage / relative, destination)
    for relative in sorted(actual.keys() - expected.keys()):
        (root / relative).unlink()
    skills = root / PLUGIN / "skills"
    for directory in sorted([*skills.glob("arc-*"), skills / PINNED_SKILL]):
        if directory.name == BRIDGE_SKILL:
            continue
        for path in sorted(directory.rglob("*"), reverse=True):
            if path.is_dir() and not any(path.iterdir()):
                path.rmdir()
        if directory.is_dir() and not any(directory.iterdir()):
            directory.rmdir()
    legacy = root / LEGACY_MARKETPLACE
    if legacy.exists():
        legacy.unlink()


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check", action="store_true", help="report generated drift without writing"
    )
    args = parser.parse_args(argv)
    try:
        manifest = canonical_manifest(ROOT)
        with tempfile.TemporaryDirectory(prefix="arc-plugin-build-") as temporary:
            stage = Path(temporary)
            included = build_generated(ROOT, stage)
            problems = drift(ROOT, stage)
            if args.check and problems:
                raise ValueError(
                    "Arc plugin distribution is stale; run scripts/build_arc_plugin.py:\n"
                    + "\n".join(problems)
                )
            if not args.check:
                publish_generated(ROOT, stage)
        status = "is up to date" if args.check else "generated"
        print(
            f"Arc plugin {manifest['version']} {status}: {len(included)} bundled workflow skills, "
            "the policy bridge and pinned arch-diagram."
        )
        return 0
    except (OSError, ValueError, sync_skills.yaml.YAMLError) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
