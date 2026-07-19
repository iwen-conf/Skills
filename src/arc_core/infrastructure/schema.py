from __future__ import annotations
import json
from pathlib import Path
from typing import Any
from jsonschema import ValidationError, validate
from ..domain.skill import SkillDocument

def load_skill_schema(root: Path) -> dict[str, Any]:
    schema_path = root / "schemas" / "skill.schema.json"
    return json.loads(schema_path.read_text(encoding="utf-8"))

def validate_skill_schema(document: SkillDocument, path_label: str, root: Path) -> list[str]:
    try:
        validate(instance=document, schema=load_skill_schema(root))
    except ValidationError as exc:
        return [f"{path_label}: schema validation failed: {exc.message}"]
    return []
