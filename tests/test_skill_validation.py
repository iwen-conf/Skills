from pathlib import Path

from arc_core.skill_validation import (
    build_skill_document,
    extract_relative_links,
    parse_frontmatter,
    run_validation,
    validate_vertical_skill_files,
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


def test_intent_router_preserves_multiple_inline_code_spans() -> None:
    text = _router_skill_text(
        "arc:build",
        "Implements scoped code changes and verifies them when the scheme is already clear enough to start.",
    ).replace("| matching work | this SKILL.md |", "| matching work | `arc:fix` / `arc:build` |")
    document = build_skill_document(text)
    assert document["intent_router"][0]["Load"] == "`arc:fix` / `arc:build`"


def test_relative_link_extraction_ignores_inline_code_examples() -> None:
    text = "Use `[name](attachment-url)` as an example and [load](references/load.md) for details."
    assert extract_relative_links(text) == ["references/load.md"]


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


def test_repo_validation_requires_root_cause_gate_for_repair_capable_skill(tmp_path: Path) -> None:
    skill_dir = tmp_path / "Arc" / "arc:fix"
    skill_dir.mkdir(parents=True)
    description = (
        "Diagnoses a reproduced failure and repairs the confirmed cause with focused verification "
        "when logs, failing tests, or incident evidence are available."
    )
    (skill_dir / "SKILL.md").write_text(_router_skill_text("arc:fix", description), encoding="utf-8")
    (tmp_path / "schemas").mkdir()
    (tmp_path / "schemas" / "skill.schema.json").write_text(
        (ROOT / "schemas" / "skill.schema.json").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    (tmp_path / "schemas" / "trigger_corpus.yaml").write_text(
        "skills:\n  arc:fix:\n    must_contain: [Diagnoses, failure, repairs]\n    positive: [a, b, c, d, e, f, g, h]\n    negative: [i, j, k]\n",
        encoding="utf-8",
    )

    errors, _warnings = validate_text(
        (skill_dir / "SKILL.md").read_text(encoding="utf-8"),
        str(skill_dir / "SKILL.md"),
        root=tmp_path,
        skill_path=skill_dir / "SKILL.md",
    )

    assert any("evidence-first root-cause repair gate" in item for item in errors)


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


def test_vertical_skill_validation_rejects_oversized_entrypoint(tmp_path: Path) -> None:
    skill_dir = tmp_path / "Android" / "oversized"
    skill_dir.mkdir(parents=True)
    (skill_dir / "SKILL.md").write_text("\n".join(["---", "name: oversized", "description: test", "---", *(["line"] * 501)]), encoding="utf-8")

    errors = validate_vertical_skill_files(tmp_path)

    assert any("SKILL.md exceeds 500 lines" in item for item in errors)


def test_run_validation_counts_arc_and_vertical_entrypoints(tmp_path: Path) -> None:
    vertical_dir = tmp_path / "Android" / "sample"
    vertical_dir.mkdir(parents=True)
    (vertical_dir / "SKILL.md").write_text("---\nname: sample\ndescription: sample\n---\n# Sample\n", encoding="utf-8")

    errors, warnings, count = run_validation(tmp_path)

    assert errors == []
    assert warnings == []
    assert count == 1


def test_vertical_skill_validation_requires_basic_frontmatter(tmp_path: Path) -> None:
    skill_dir = tmp_path / "CNB" / "invalid"
    skill_dir.mkdir(parents=True)
    (skill_dir / "SKILL.md").write_text("---\ndescription: missing name\n---\n# Invalid\n", encoding="utf-8")

    errors = validate_vertical_skill_files(tmp_path)

    assert any("missing frontmatter name" in item for item in errors)


def test_vertical_skill_validation_requires_root_cause_gate_for_repair_skill(tmp_path: Path) -> None:
    skill_dir = tmp_path / "CNB" / "cnb-pipeline"
    skill_dir.mkdir(parents=True)
    (skill_dir / "SKILL.md").write_text(
        "---\nname: cnb-pipeline\ndescription: diagnose and repair pipelines\n---\n# Pipeline\n",
        encoding="utf-8",
    )

    errors = validate_vertical_skill_files(tmp_path)

    assert any("evidence-first root-cause repair gate" in item for item in errors)


def test_run_validation_rejects_duplicate_vertical_skill_names(tmp_path: Path) -> None:
    for family in ("Android", "CNB"):
        skill_dir = tmp_path / family / family.lower()
        skill_dir.mkdir(parents=True)
        (skill_dir / "SKILL.md").write_text(
            "---\nname: duplicate-skill\ndescription: duplicate\n---\n# Duplicate\n",
            encoding="utf-8",
        )

    errors, _warnings, _count = run_validation(tmp_path)

    assert any("duplicate skill name duplicate-skill" in item for item in errors)
