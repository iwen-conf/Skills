#!/usr/bin/env python3
"""Build and validate complete skill packages before replacing an installation."""
from __future__ import annotations

import argparse
from collections import deque
import os
from pathlib import Path
import re
import shutil
import sys
import tempfile

import yaml

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from arc_core.domain.skill import VERTICAL_SKILL_ROOTS  # noqa: E402
from arc_core.infrastructure.markdown import extract_relative_links, parse_frontmatter  # noqa: E402

DEFAULT_DEST = Path.home() / ".agents" / "skills"
ARC_NAME_RE = re.compile(r"\barc:([a-z0-9-]+)\b")


def published_name(name: str, naming: str) -> str:
    return name.replace(":", "-") if naming == "dash" else name


def read_metadata(path: Path) -> dict:
    metadata, error = parse_frontmatter(path.read_text(encoding="utf-8"))
    if error:
        raise ValueError(f"{path}: {error}")
    name = metadata.get("name")
    if not isinstance(name, str) or not re.fullmatch(r"[a-z0-9:-]+", name):
        raise ValueError(f"{path}: invalid skill name {name!r}")
    return metadata


def list_skill_dirs(root: Path = ROOT) -> list[Path]:
    return sorted(
        path.parent
        for family in ("Arc", *VERTICAL_SKILL_ROOTS)
        for path in (root / family).glob("*/SKILL.md")
        if path.is_file()
    )


def required_skills(metadata: dict) -> list[str]:
    requires = metadata.get("metadata", {}).get("requires", {})
    names: list[str] = []
    for field in ("skills", "siblings"):
        values = requires.get(field, [])
        if not isinstance(values, list) or any(not isinstance(name, str) for name in values):
            raise ValueError(f"{metadata['name']}: required skills must be a list of names")
        names.extend(values)
    return names


def validate_installation(
    root: Path,
    allowed_siblings: set[str] | None = None,
    only_folders: set[str] | None = None,
) -> list[str]:
    """检查实际安装内容，包含按需资料、跨技能引用及宿主元数据。"""
    errors: list[str] = []
    names: set[str] = set()
    metadata_by_path: dict[Path, dict] = {}
    root = root.resolve()
    for path in sorted(root.glob("*/SKILL.md")):
        if only_folders is not None and path.parent.name not in only_folders:
            continue
        try:
            metadata = read_metadata(path)
        except (OSError, ValueError) as exc:
            errors.append(str(exc))
            continue
        name = metadata["name"]
        if name in names:
            errors.append(f"{path}: duplicate skill name {name}")
        names.add(name)
        metadata_by_path[path] = metadata
        if path.parent.name != name:
            errors.append(f"{path}: folder name must match {name}")
        ui_path = path.parent / "agents" / "openai.yaml"
        if ui_path.is_file():
            ui = yaml.safe_load(ui_path.read_text(encoding="utf-8")) or {}
            prompt = ui.get("interface", {}).get("default_prompt", "")
            if prompt and re.search(r"\$" + re.escape(name) + r"(?![a-z0-9:-])", prompt) is None:
                errors.append(f"{ui_path}: default_prompt must invoke $" + name)

    for path, metadata in metadata_by_path.items():
        for dependency in required_skills(metadata):
            if dependency not in names:
                errors.append(f"{path}: missing required skill {dependency}")
        for document in sorted(path.parent.rglob("*.md")):
            for target in extract_relative_links(document.read_text(encoding="utf-8")):
                logical = Path(os.path.normpath(document.parent / target))
                if logical.exists() and logical.is_relative_to(root):
                    continue
                try:
                    relative = logical.resolve().relative_to(root)
                except ValueError:
                    errors.append(f"{document}: unavailable packaged link {target}")
                    continue
                if allowed_siblings and relative.parts and relative.parts[0] in allowed_siblings:
                    continue
                errors.append(f"{document}: unavailable packaged link {target}")
    return errors


def build_installation(root: Path, stage: Path, selected: list[str], naming: str = "dash") -> list[str]:
    """复制包及其传递依赖；共享文档在发布时随引用它的包生成。"""
    root = root.resolve()
    catalog: dict[str, Path] = {}
    for directory in list_skill_dirs(root):
        name = read_metadata(directory / "SKILL.md")["name"]
        if name in catalog:
            raise ValueError(f"duplicate source skill {name}")
        catalog[name] = directory.resolve()
    owners = {directory: name for name, directory in catalog.items()}
    pending: deque[tuple[Path, Path, str]] = deque()
    copied: set[Path] = set()
    included: set[str] = set()

    def copy(source: Path, destination: Path, owner: str) -> None:
        if destination in copied:
            return
        copied.add(destination)
        if source.is_dir():
            shutil.copytree(source, destination, ignore=shutil.ignore_patterns("__pycache__", "*.pyc", ".DS_Store", ".git"))
            for child in sorted(destination.rglob("*")):
                if child.is_file():
                    copied.add(child)
                    pending.append((source / child.relative_to(destination), child, owner))
        else:
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, destination)
            pending.append((source, destination, owner))

    def include(name: str) -> None:
        if name in included:
            return
        if name not in catalog:
            raise ValueError(f"missing source skill {name}")
        included.add(name)
        directory = catalog[name]
        destination = stage / published_name(name, naming)
        copy(directory / "SKILL.md", destination / "SKILL.md", name)
        for extra in ("agents", "scripts"):
            extra_dir = directory / extra
            if extra_dir.is_dir():
                copy(extra_dir, destination / extra, name)
        for dependency in required_skills(read_metadata(directory / "SKILL.md")):
            include(dependency)

    def shared_destination(owner: str, target: Path) -> Path:
        relative_source = target.relative_to(root)
        if naming == "dash":
            relative_source = Path(*[ARC_NAME_RE.sub(r"arc-\1", part) for part in relative_source.parts])
        return stage / published_name(owner, naming) / "references" / "_shared" / relative_source

    def target_path(target: Path, owner: str) -> Path:
        target = target.resolve()
        if not target.exists() or not target.is_relative_to(root) or target == root:
            raise ValueError(f"{owner}: missing or external source resource {target}")
        for directory, name in owners.items():
            if not target.is_relative_to(directory):
                continue
            if name in included:
                destination = stage / published_name(name, naming) / target.relative_to(directory)
                copy(target, destination, name)
                return destination
            if target == directory / "SKILL.md":
                return stage / published_name(name, naming) / "SKILL.md"
            destination = shared_destination(owner, target)
            copy(target, destination, owner)
            return destination
        destination = shared_destination(owner, target)
        copy(target, destination, owner)
        return destination

    for name in selected:
        include(name)
    while pending:
        source, destination, owner = pending.popleft()
        if source.suffix not in {".md", ".yaml", ".yml"}:
            continue
        text = source.read_text(encoding="utf-8")
        if source.suffix == ".md":
            for link in extract_relative_links(text):
                target = target_path(source.parent / link, owner)
                relative = os.path.relpath(target, destination.parent)
                pattern = r"(\]\(<?)" + re.escape(link) + r"(?=[#)>\s])"
                text = re.sub(pattern, lambda match: match.group(1) + relative, text)
        if naming == "dash":
            text = ARC_NAME_RE.sub(r"arc-\1", text)
        destination.write_text(text, encoding="utf-8")

    siblings = {published_name(name, naming) for name in catalog} - {
        published_name(name, naming) for name in included
    }
    errors = validate_installation(stage, siblings)
    if errors:
        raise ValueError("Installation validation failed:\n" + "\n".join(errors))
    return sorted(included)


def publish_installation(stage: Path, destination: Path, canonical_names: list[str], naming: str) -> Path:
    """先保存旧目录/符号链接；替换中途失败时恢复本次变更。"""
    destination.mkdir(parents=True, exist_ok=True)
    backup = Path(tempfile.mkdtemp(prefix="skills-backup-", dir=destination.parent))
    installed: list[str] = []
    saved: list[str] = []
    targets = {published_name(name, naming) for name in canonical_names}
    obsolete = {
        published_name(name, "colon" if naming == "dash" else "dash")
        for name in canonical_names if name.startswith("arc:")
    } - targets
    try:
        for name in sorted(targets | obsolete):
            target = destination / name
            if target.exists() or target.is_symlink():
                target.replace(backup / name)
                saved.append(name)
            if name in targets:
                (stage / name).replace(target)
                installed.append(name)
    except BaseException:
        for name in reversed(installed):
            shutil.rmtree(destination / name)
        for name in reversed(saved):
            (backup / name).replace(destination / name)
        raise
    return backup


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dest", type=Path, default=DEFAULT_DEST)
    parser.add_argument("--naming", choices=("dash", "colon"), default="dash")
    parser.add_argument("--family", action="append", choices=("Arc", *VERTICAL_SKILL_ROOTS))
    parser.add_argument("--only", action="append", default=[], help="Canonical skill name; includes required file/skill dependencies")
    parser.add_argument("--dry-run", action="store_true", help="Build and validate without changing the destination")
    parser.add_argument("--check", action="store_true", help="Validate the existing destination without installing")
    args = parser.parse_args(argv)
    destination = args.dest.expanduser().absolute()
    try:
        if args.check:
            if not destination.is_dir():
                raise ValueError(f"installation directory does not exist: {destination}")
            managed = {
                published_name(read_metadata(path / "SKILL.md")["name"], naming)
                for path in list_skill_dirs(ROOT)
                for naming in ("dash", "colon")
            }
            errors = validate_installation(destination, only_folders=managed)
            if errors:
                raise ValueError("\n".join(errors))
            print(f"Installation validation passed: {destination}")
            return 0
        if destination.resolve().is_relative_to(ROOT) or ROOT.is_relative_to(destination.resolve()):
            raise ValueError("installation destination must not overlap the source repository")
        selected = args.only or [
            read_metadata(path / "SKILL.md")["name"]
            for path in list_skill_dirs(ROOT)
            if path.parent.name in (args.family or ["Arc"])
        ]
        if not selected:
            raise ValueError("no skills selected")
        with tempfile.TemporaryDirectory(prefix="skills-stage-") as temporary:
            stage = Path(temporary)
            included = build_installation(ROOT, stage, selected, args.naming)
            print(f"Verified {len(included)} packages ({args.naming} names): {', '.join(included)}")
            if args.dry_run:
                print(f"Dry run: destination unchanged: {destination}")
                return 0
            backup = publish_installation(stage, destination, included, args.naming)
        print(f"Installed into {destination}\nPrevious entries saved in {backup}")
        return 0
    except (OSError, ValueError, yaml.YAMLError) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
