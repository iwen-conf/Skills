from __future__ import annotations

import hashlib
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

from jsonschema import ValidationError, validate

from .artifact_manifest import write_manifest
from .skill_validation import build_skill_document, collect_skill_files

RegistryDocument = dict[str, Any]
ContextHubDocument = dict[str, Any]


def _iso_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def _iso_future(days: int) -> str:
    return (datetime.now(timezone.utc) + timedelta(days=days)).isoformat().replace("+00:00", "Z")


def _sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def build_skill_entry(path: Path, root: Path) -> dict[str, Any]:
    document = build_skill_document(path.read_text(encoding="utf-8"))
    frontmatter = document["frontmatter"]
    return {
        "name": frontmatter.get("name", ""),
        "description": frontmatter.get("description", ""),
        "source_path": str(path.relative_to(root)),
        "sections": document["sections"],
        "quick_contract": document.get("quick_contract"),
        "input_arguments": document.get("input_arguments"),
        "outputs_section": document.get("outputs_section"),
    }


def build_registry(root: Path) -> RegistryDocument:
    skills = [build_skill_entry(path, root) for path in collect_skill_files(root)]
    return {
        "schema_version": "1.0.0",
        "generated_at": _iso_now(),
        "skill_count": len(skills),
        "skills": skills,
    }


def _resolve_schema_root(root: Path) -> Path:
    schema_path = root / "schemas" / "skills.index.schema.json"
    if schema_path.exists():
        return root
    return Path(__file__).resolve().parents[2]


def load_registry_schema(root: Path) -> dict[str, Any]:
    schema_root = _resolve_schema_root(root)
    schema_path = schema_root / "schemas" / "skills.index.schema.json"
    return json.loads(schema_path.read_text(encoding="utf-8"))


def validate_registry(document: RegistryDocument, root: Path) -> list[str]:
    try:
        validate(instance=document, schema=load_registry_schema(root))
    except ValidationError as exc:
        return [f"registry schema validation failed: {exc.message}"]
    return []


def write_registry(root: Path, output_path: Path | None = None) -> Path:
    registry = build_registry(root)
    errors = validate_registry(registry, root)
    if errors:
        raise ValueError("; ".join(errors))
    target = output_path or (root / "skills.index.json")
    target.write_text(json.dumps(registry, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return target


def load_context_hub_index(root: Path) -> ContextHubDocument:
    index_path = root / ".arc" / "context-hub" / "index.json"
    if not index_path.exists():
        return {"generated_at": _iso_now(), "artifacts": []}
    try:
        data = json.loads(index_path.read_text(encoding="utf-8"))
    except Exception:
        return {"generated_at": _iso_now(), "artifacts": []}
    artifacts = data.get("artifacts")
    if not isinstance(artifacts, list):
        data["artifacts"] = []
    return data


def build_registry_artifact(root: Path, registry_path: Path) -> dict[str, Any]:
    return {
        "name": "skills.index",
        "producer_skill": "arc:registry",
        "path": str(registry_path.relative_to(root)),
        "content_hash": _sha256_file(registry_path),
        "generated_at": _iso_now(),
        "expires_at": _iso_future(7),
        "ttl_seconds": 604800,
        "refresh_skill": "scripts/build_skills_index.py",
        "refresh_command_hint": "uv run python scripts/build_skills_index.py",
        "artifact_type": "skills-registry",
        "consumers": ["arc:exec", "arc:build", "arc:audit", "arc:cartography"],
    }


def update_context_hub(root: Path, registry_path: Path) -> Path:
    index_path = root / ".arc" / "context-hub" / "index.json"
    index_path.parent.mkdir(parents=True, exist_ok=True)
    index = load_context_hub_index(root)
    artifacts = [artifact for artifact in index.get("artifacts", []) if not (isinstance(artifact, dict) and artifact.get("artifact_type") == "skills-registry")]
    artifacts.append(build_registry_artifact(root, registry_path))
    index["generated_at"] = _iso_now()
    index["artifacts"] = artifacts
    index_path.write_text(json.dumps(index, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return index_path


def write_registry_and_context(root: Path, output_path: Path | None = None) -> tuple[Path, Path, Path]:
    registry_path = write_registry(root, output_path=output_path)
    index_path = update_context_hub(root, registry_path)
    manifest = {
        "schema_version": "1.0.0",
        "producer_skill": "arc:registry",
        "generated_at": _iso_now(),
        "artifacts": [build_registry_artifact(root, registry_path)],
    }
    manifest_path = write_manifest(root, manifest, root / ".arc" / "registry" / "manifest.json")
    return registry_path, index_path, manifest_path
