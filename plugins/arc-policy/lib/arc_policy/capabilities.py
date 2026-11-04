"""Resolve shipped skills from this installation, never from a guessed host cache."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path
from typing import Any

import yaml

from arc_policy.detect import resolve_skill_name
from arc_policy.paths import plugin_root

# Distribution integrity, not a security boundary against an owner who can edit
# this runtime. Change this anchor only after reviewing the new upstream commit,
# verifying every upstream file, and regenerating source.json with that evidence.
REVIEWED_ARCH_DIAGRAM_SOURCE_SHA256 = (
    "9fd5580d844ca3642050ec2c916666723108097ee335559d9ca33e41f76a246e"
)


def check_pinned_skill(directory: Path) -> dict[str, Any]:
    """Check the complete reviewed upstream bundle before executing its scripts."""
    directory = directory.resolve()
    source = (directory / "source.json").read_bytes()
    if hashlib.sha256(source).hexdigest() != REVIEWED_ARCH_DIAGRAM_SOURCE_SHA256:
        raise ValueError(
            "pinned skill source manifest checksum mismatch; upstream review required"
        )
    manifest = json.loads(source)
    if (
        not isinstance(manifest, dict)
        or manifest.get("repository")
        != "https://github.com/bassnova42/arch-diagram-skill"
        or not re.fullmatch(r"[0-9a-f]{40}", str(manifest.get("commit", "")))
        or not isinstance(manifest.get("files"), list)
        or not manifest["files"]
    ):
        raise ValueError("invalid arch-diagram source manifest")
    recorded: set[str] = set()
    for item in manifest["files"]:
        relative = item.get("path") if isinstance(item, dict) else None
        if (
            not isinstance(relative, str)
            or not relative
            or Path(relative).is_absolute()
            or ".." in Path(relative).parts
            or relative == "source.json"
            or relative in recorded
        ):
            raise ValueError(f"unsafe or repeated pinned skill path: {relative!r}")
        path = directory / relative
        if path.is_symlink() or not path.resolve().is_relative_to(directory):
            raise ValueError(f"unsafe pinned skill path: {relative}")
        if not path.is_file():
            raise ValueError(f"pinned skill file is missing: {relative}")
        body = path.read_bytes()
        if hashlib.sha256(body).hexdigest() != item.get("sha256"):
            raise ValueError(f"pinned skill checksum mismatch: {relative}")
        blob = b"blob " + str(len(body)).encode("ascii") + b"\0" + body
        if hashlib.sha1(blob).hexdigest() != item.get("git_blob_sha"):
            raise ValueError(f"pinned skill Git blob checksum mismatch: {relative}")
        recorded.add(relative)
    if any(path.is_symlink() for path in directory.rglob("*")):
        raise ValueError("unsafe symlink in pinned skill")
    actual = {
        path.relative_to(directory).as_posix()
        for path in directory.rglob("*")
        if path.is_file() and path != directory / "source.json"
    }
    if actual != recorded:
        raise ValueError(f"unrecorded pinned skill files: {sorted(actual - recorded)}")
    if (
        not {"LICENSE", "SKILL.md", "scripts/build.mjs", "scripts/check.mjs"}
        <= recorded
    ):
        raise ValueError("pinned skill is missing its license or required entry points")
    return manifest


def skill_catalog(root: Path | None = None) -> list[dict[str, Any]]:
    """Read entry metadata only; discovery does not activate every skill's body."""
    owner = (root or plugin_root()).resolve()
    result = []
    for entry in sorted((owner / "skills").glob("*/SKILL.md")):
        if not entry.resolve().is_relative_to(owner):
            raise ValueError(f"skill entry escapes plugin: {entry}")
        parts = entry.read_text(encoding="utf-8").split("---", 2)
        metadata = (
            yaml.safe_load(parts[1])
            if len(parts) == 3 and not parts[0].strip()
            else None
        )
        if not isinstance(metadata, dict) or metadata.get("name") != entry.parent.name:
            raise ValueError(f"invalid bundled skill metadata: {entry}")
        extra = metadata.get("metadata") or {}
        source_file = entry.parent / "source.json"
        source = (
            json.loads(source_file.read_text(encoding="utf-8"))
            if source_file.is_file()
            else None
        )
        item = {
            "name": metadata["name"],
            "version": str(
                metadata.get("version")
                or extra.get("version")
                or (source or {}).get("version")
                or ""
            ),
            "description": str(metadata.get("description") or "").strip(),
            "directory": str(entry.parent),
            "entry": str(entry),
            "bundled": True,
        }
        if source:
            item["source"] = {
                key: source[key] for key in ("repository", "commit", "version")
            }
        result.append(item)
    return result


def resolve_skill(name: str, root: Path | None = None) -> dict[str, Any]:
    requested = name.removeprefix("arc-policy:")
    canonical = resolve_skill_name(requested, {})
    for item in skill_catalog(root):
        if item["name"] == canonical:
            return item
    raise ValueError(
        f"skill {name!r} is not bundled; resolve it from the host's available skill inventory"
    )
