from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from jsonschema import ValidationError, validate

ManifestDocument = dict[str, Any]


def _resolve_schema_root(root: Path) -> Path:
    schema_path = root / "schemas" / "artifact-manifest.schema.json"
    if schema_path.exists():
        return root
    return Path(__file__).resolve().parents[2]


def load_manifest_schema(root: Path) -> dict[str, Any]:
    schema_root = _resolve_schema_root(root)
    schema_path = schema_root / "schemas" / "artifact-manifest.schema.json"
    return json.loads(schema_path.read_text(encoding="utf-8"))


def validate_manifest(document: ManifestDocument, root: Path) -> list[str]:
    try:
        validate(instance=document, schema=load_manifest_schema(root))
    except ValidationError as exc:
        return [f"manifest schema validation failed: {exc.message}"]
    return []


def write_manifest(root: Path, manifest: ManifestDocument, output_path: Path) -> Path:
    errors = validate_manifest(manifest, root)
    if errors:
        raise ValueError("; ".join(errors))
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return output_path
