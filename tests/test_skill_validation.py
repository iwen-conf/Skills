from pathlib import Path

from arc_core.skill_validation import (
    build_skill_document,
    parse_frontmatter,
    run_validation,
    validate_skill_schema,
    validate_text,
)

ROOT = Path(__file__).resolve().parents[1]


def _router_skill_text(name: str, description: str) -> str:
    return f'''---
name: {name}
version: 1.0.0
description: "{description}"
---
# Skill
## Intent Router
| When | Load |
|---|---|
| matching work | this SKILL.md |
| unclear scope | `arc:clarify` |
## Red Lines
NO GUESSING.
NO SILENT SCOPE EXPANSION.
## When to Use
Use for the described work. Not for unrelated tasks.
'''


def test_parse_frontmatter_extracts_core_fields() -> None:
    text = """---
name: "arc:build"
version: "1.0.0"
description: "Code delivery."
---
# Title
"""
    frontmatter, error = parse_frontmatter(text)
    assert error is None
    assert frontmatter["name"] == "arc:build"
    assert frontmatter["description"] == "Code delivery."
    assert frontmatter["version"] == "1.0.0"


def test_validate_text_reports_missing_intent_router() -> None:
    text = """---
name: "arc:build"
version: "1.0.0"
description: "Implements scoped code changes and verifies them when the scheme is already clear enough to start."
---
# Skill
## Overview
overview body
"""
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert "virtual/SKILL.md: missing heading ## Intent Router" in errors


def test_validate_text_rejects_short_or_first_person_description() -> None:
    text = _router_skill_text("arc:build", "I can help you write code.")
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert any("third person" in item or "at least 80" in item for item in errors)


def test_validate_text_rejects_retired_frontmatter_keys() -> None:
    text = """---
name: arc:build
version: 1.0.0
description: "Implements scoped code changes and verifies them when the scheme is already clear enough to start."
enforce_arc_profile: true
---
# Skill
## Intent Router
| When | Load |
|---|---|
| matching work | this SKILL.md |
| unclear scope | `arc:clarify` |
## Red Lines
NO GUESSING.
## When to Use
Use for delivery.
"""
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert any("retired keys" in item for item in errors)


def test_build_skill_document_extracts_intent_router() -> None:
    text = _router_skill_text(
        "arc:build",
        "Implements scoped code changes and verifies them when the scheme is already clear enough to start.",
    )
    document = build_skill_document(text)
    assert document["frontmatter"]["name"] == "arc:build"
    assert document["intent_router"][0]["When"] == "matching work"


def test_validate_skill_schema_accepts_router_document() -> None:
    text = _router_skill_text(
        "arc:build",
        "Implements scoped code changes and verifies them when the scheme is already clear enough to start.",
    )
    document = build_skill_document(text)
    assert validate_skill_schema(document, "virtual/SKILL.md", ROOT) == []


def test_validate_text_accepts_router_arc_skills() -> None:
    description = (
        "Implements scoped code changes and verifies them when the scheme is already "
        "clear enough to start writing production files."
    )
    errors, warnings = validate_text(
        _router_skill_text("arc:build", description),
        "virtual/SKILL.md",
    )
    assert errors == []
    assert warnings == []


def test_validate_text_rejects_non_arc_skill_name() -> None:
    text = _router_skill_text(
        "plain-skill",
        "Implements scoped code changes and verifies them when the scheme is already clear enough to start.",
    )
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert "virtual/SKILL.md: skill name must use arc:xxx namespace" in errors


def test_validate_text_accepts_constraint_skill_names() -> None:
    description = (
        "Applies default backend architecture, DIP, layering, and helper limits when "
        "writing or reviewing Go services."
    )
    for name in ("arc:idx", "arc:comment", "arc:trace", "arc:arch", "arc:sdlc", "arc:prewalk"):
        errors, warnings = validate_text(
            _router_skill_text(name, description),
            "virtual/SKILL.md",
        )
        assert errors == []
        assert warnings == []


def test_all_skill_names_use_arc_namespace() -> None:
    for path in (ROOT / "Arc").rglob("SKILL.md"):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        assert frontmatter.get("name", "").startswith("arc:"), path


def test_skill_packages_keep_lifecycle_invariants() -> None:
    cases = {
        "arc:build": ["arc:arch", "arc:sdlc", "arc:prewalk", "execution-truth.md"],
        "arc:fix": ["arc:arch", "arc:sdlc", "arc:prewalk", "execution-truth.md"],
        "arc:frontend": ["arc:sdlc", "arc:prewalk"],
        "arc:security": ["arc:sdlc"],
        "arc:clarify": ["arc:sdlc"],
        "arc:define": ["arc:sdlc"],
        "arc:audit": ["arc:sdlc"],
        "arc:docs": ["task docs"],
        "arc:sdlc": ["进度跟踪表.md", "00-前置约束.md", "arc:prewalk"],
        "arc:arch": ["DIP", "ponytail", "_helpers.go"],
        "arc:prewalk": ["First-Edit", "plan-postcard", "production code"],
    }
    for name, phrases in cases.items():
        folder = ROOT / "Arc" / name
        package = "\n".join(path.read_text(encoding="utf-8") for path in sorted(folder.rglob("*.md")))
        for phrase in phrases:
            assert phrase in package, f"{name} missing {phrase}"


def test_run_validation_rejects_github_workflows_directory(tmp_path: Path) -> None:
    workflows_dir = tmp_path / ".github" / "workflows"
    workflows_dir.mkdir(parents=True)
    (workflows_dir / "ci.yml").write_text("name: CI\n", encoding="utf-8")

    errors, warnings, count = run_validation(tmp_path)

    assert count == 0
    assert warnings == []
    assert len(errors) == 1
    assert "GitHub Actions workflows are not allowed" in errors[0]
