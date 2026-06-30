from pathlib import Path

from arc_core.artifact_manifest import validate_manifest
from arc_core.skill_registry import (
    build_registry,
    update_context_hub,
    validate_registry,
    write_registry,
    write_registry_and_context,
)
from arc_core.skill_validation import collect_skill_files

ROOT = Path(__file__).resolve().parents[1]


def test_build_registry_includes_structured_skill_fields() -> None:
    registry = build_registry(ROOT)
    assert registry["schema_version"] == "1.0.0"
    assert registry["skill_count"] >= 1
    build_entry = next(item for item in registry["skills"] if item["name"] == "arc:build")
    assert build_entry["source_path"] == "Arc/arc:build/SKILL.md"
    assert build_entry["quick_contract"]["trigger"]
    assert build_entry["input_arguments"][0]["parameter"] == "project_path"


def test_validate_registry_accepts_generated_registry() -> None:
    registry = build_registry(ROOT)
    assert validate_registry(registry, ROOT) == []
    names = {item["name"] for item in registry["skills"]}
    assert names == {
        "arc:define",
        "arc:clarify",
        "arc:docs",
        "arc:build",
        "arc:frontend",
        "arc:fix",
        "arc:audit",
        "arc:security",
        "arc:code-comment-conventions",
        "arc:go-gin-ssr-fmt-tracing",
        "arc:project-architecture-conventions",
        "arc:task-doc-progress-conventions",
    }


def test_collect_skill_files_indexes_only_arc_namespaced_skills() -> None:
    files = collect_skill_files(ROOT)
    assert ROOT / "Arc" / "arc:build" / "SKILL.md" in files
    assert ROOT / "Arc" / "arc:docs" / "SKILL.md" in files
    assert ROOT / "Arc" / "arc:frontend" / "SKILL.md" in files
    assert ROOT / "Arc" / "arc:code-comment-conventions" / "SKILL.md" in files
    assert ROOT / "Arc" / "arc:go-gin-ssr-fmt-tracing" / "SKILL.md" in files
    assert ROOT / "Arc" / "arc:project-architecture-conventions" / "SKILL.md" in files
    assert ROOT / "Arc" / "arc:task-doc-progress-conventions" / "SKILL.md" in files
    assert all(
        path.is_relative_to(ROOT / "Arc")
        for path in files
    )


def test_update_context_hub_registers_skills_registry_artifact() -> None:
    temp_root = ROOT / ".arc" / "tmp-skill-registry-context-test"
    if temp_root.exists():
        import shutil

        shutil.rmtree(temp_root)
    temp_root.mkdir(parents=True, exist_ok=True)
    try:
        registry_path = write_registry(ROOT, output_path=temp_root / "skills.index.json")
        index_path = update_context_hub(temp_root, registry_path)
        import json

        index = json.loads(index_path.read_text(encoding="utf-8"))
        artifact = index["artifacts"][0]
        assert artifact["artifact_type"] == "skills-registry"
        assert artifact["producer_skill"] == "arc-registry"
        assert artifact["path"] == "skills.index.json"
        assert "arc:code-comment-conventions" in artifact["consumers"]
        assert "arc:go-gin-ssr-fmt-tracing" in artifact["consumers"]
        assert "arc:project-architecture-conventions" in artifact["consumers"]
        assert "arc:task-doc-progress-conventions" in artifact["consumers"]
    finally:
        import shutil

        if temp_root.exists():
            shutil.rmtree(temp_root)


def test_write_registry_and_context_generates_valid_manifest() -> None:
    temp_root = ROOT / ".arc" / "tmp-skill-registry-manifest-test"
    if temp_root.exists():
        import shutil

        shutil.rmtree(temp_root)
    temp_root.mkdir(parents=True, exist_ok=True)
    try:
        registry_path, index_path, manifest_path = write_registry_and_context(temp_root, output_path=temp_root / "skills.index.json")
        import json

        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        assert registry_path.exists()
        assert index_path.exists()
        assert manifest_path.exists()
        assert validate_manifest(manifest, ROOT) == []
        assert manifest["artifacts"][0]["artifact_type"] == "skills-registry"
        assert manifest["artifacts"][0]["refresh_command_hint"] == "uv run python scripts/build_skills_index.py"
    finally:
        import shutil

        if temp_root.exists():
            shutil.rmtree(temp_root)
