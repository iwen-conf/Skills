from pathlib import Path

from arc_core.artifact_manifest import validate_manifest
from arc_core.skill_registry import (
    build_registry,
    update_context_hub,
    validate_registry,
    write_registry,
    write_registry_and_context,
)
from arc_core.skill_validation import FUSION_GENERIC_SKILLS, collect_skill_files

ROOT = Path("/Users/iluwen/Documents/Code/Skills")


def test_build_registry_includes_structured_skill_fields() -> None:
    registry = build_registry(ROOT)
    assert registry["schema_version"] == "1.0.0"
    assert registry["skill_count"] >= 1
    build_entry = next(item for item in registry["skills"] if item["name"] == "arc:build")
    context_entry = next(item for item in registry["skills"] if item["name"] == "arc:context")
    assert build_entry["source_path"] == "Arc/arc:build/SKILL.md"
    assert build_entry["quick_contract"]["trigger"]
    assert build_entry["input_arguments"][0]["parameter"] == "project_path"
    assert context_entry["source_path"] == "Arc/arc:context/SKILL.md"
    assert context_entry["quick_contract"]["trigger"]
    assert context_entry["outputs_section"]["format"] == "text"


def test_validate_registry_accepts_generated_registry() -> None:
    registry = build_registry(ROOT)
    assert validate_registry(registry, ROOT) == []
    names = {item["name"] for item in registry["skills"]}
    assert "arc:build" in names
    assert "terminal-table-output" in names


def test_collect_skill_files_indexes_supported_namespaces_and_fusion_skills() -> None:
    files = collect_skill_files(ROOT)
    assert ROOT / "Arc" / "arc:context" / "SKILL.md" in files
    assert ROOT / "terminal-table-output" / "SKILL.md" in files
    allowed_roots = [ROOT / "Arc", *(ROOT / name for name in FUSION_GENERIC_SKILLS)]
    assert all(
        any(path.is_relative_to(root) for root in allowed_roots)
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
