from pathlib import Path

import pytest

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


@pytest.mark.parametrize(
    "text, expected",
    [
        ("# Missing header\n", "missing frontmatter"),
        ("---\nname: [invalid\n---\n", "yaml parsing error"),
        ("---\n- invalid\n---\n", "YAML mapping"),
        ("---\nfalse\n---\n", "YAML mapping"),
        ("---\n\n---\n", "YAML mapping"),
    ],
)
def test_malformed_arc_entrypoint_is_counted_and_rejected(tmp_path: Path, text: str, expected: str) -> None:
    skill = tmp_path / "Arc" / "arc:broken" / "SKILL.md"
    skill.parent.mkdir(parents=True)
    skill.write_text(text, encoding="utf-8")

    errors, warnings, count = run_validation(tmp_path)

    assert count == 1
    assert warnings == []
    assert any(str(skill) in error and expected in error for error in errors)


DESCRIPTION = (
    "Implements scoped code changes and verifies them when the scheme is already clear enough "
    "to start. Not for read-only review (use arc:audit)."
)

ARC_SECTIONS = [
    ("Trigger", "- implement a scoped change"),
    ("Non-trigger", "- read-only review → arc:audit"),
    ("Goal", "The requested behavior exists and its checks pass."),
    ("Inputs", "- request (required)"),
    (
        "Intent Router",
        "| When | Load |\n|---|---|\n| matching work | this SKILL.md |\n| unclear scope | `arc:plan` |",
    ),
    ("Workflow", "1. Read the contract."),
    ("Tool Strategy", "- Preferred: arc-idx."),
    ("Decision Rules", "- IF scope is missing THEN stop."),
    ("Verification", "- Command output for every claim."),
    ("Failure Recovery", "- Same check fails 3 times → stop and report."),
    ("Completion Criteria", "- [ ] checks pass"),
]


def _arc_body(sections: list[tuple[str, str]] = ARC_SECTIONS) -> str:
    return "".join(f"## {title}\n{body}\n" for title, body in sections)


def _router_skill_text(name: str, description: str, body: str | None = None) -> str:
    return f'''---
name: {name}
version: 1.0.0
description: "{description}"
---
# Skill
{_arc_body() if body is None else body}'''


def test_parse_frontmatter_extracts_core_fields() -> None:
    text = """---
name: "arc:code"
version: "1.0.0"
description: "Code delivery."
---
# Title
"""
    frontmatter, error = parse_frontmatter(text)
    assert error is None
    assert frontmatter["name"] == "arc:code"
    assert frontmatter["description"] == "Code delivery."
    assert frontmatter["version"] == "1.0.0"


def test_validate_text_reports_missing_intent_router() -> None:
    text = _router_skill_text(
        "arc:code",
        DESCRIPTION,
        _arc_body([item for item in ARC_SECTIONS if item[0] != "Intent Router"]),
    )
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert any("missing ## Intent Router" in item for item in errors)
    assert "virtual/SKILL.md: ## Intent Router must contain a markdown table" in errors


def test_validate_text_rejects_out_of_order_and_empty_sections() -> None:
    swapped = [ARC_SECTIONS[1], ARC_SECTIONS[0], *ARC_SECTIONS[2:]]
    errors, _warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION, _arc_body(swapped)), "virtual/SKILL.md")
    assert any("headings out of order" in item for item in errors)

    emptied = [(title, "" if title == "Goal" else body) for title, body in ARC_SECTIONS]
    errors, _warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION, _arc_body(emptied)), "virtual/SKILL.md")
    assert "virtual/SKILL.md: ## Goal must not be empty" in errors


def test_validate_text_ignores_headings_inside_code_fences() -> None:
    fenced = [
        (title, body + "\n```markdown\n## Not A Section\n```" if title == "Workflow" else body)
        for title, body in ARC_SECTIONS
    ]
    errors, _warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION, _arc_body(fenced)), "virtual/SKILL.md")
    assert errors == []


def test_validate_text_requires_not_for_clause() -> None:
    description = "Implements scoped code changes and verifies them when the scheme is already clear enough to start."
    errors, _warnings = validate_text(_router_skill_text("arc:code", description), "virtual/SKILL.md")
    assert any("'Not for … (use X)' clause" in item for item in errors)


def test_validate_text_rejects_retired_skill_names_and_token_overflow() -> None:
    body = _arc_body([(title, body + " Hand off to arc:fix." if title == "Workflow" else body) for title, body in ARC_SECTIONS])
    errors, _warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION, body), "virtual/SKILL.md")
    assert any("retired Arc skills: arc:fix" in item for item in errors)

    padded = _arc_body([(title, body + "\n" + "word " * 3000 if title == "Workflow" else body) for title, body in ARC_SECTIONS])
    errors, _warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION, padded), "virtual/SKILL.md")
    assert any("exceeds 3500 estimated tokens" in item for item in errors)


def test_validate_text_warns_on_vague_wording() -> None:
    body = _arc_body([(title, body + " Add checks as needed." if title == "Workflow" else body) for title, body in ARC_SECTIONS])
    errors, warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION, body), "virtual/SKILL.md")
    assert errors == []
    assert any("vague wording (as needed)" in item for item in warnings)


def test_validate_text_rejects_short_or_first_person_description() -> None:
    text = _router_skill_text("arc:code", "I can help you write code.")
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert any("third person" in item or "at least 80" in item for item in errors)


def test_validate_text_rejects_retired_frontmatter_keys() -> None:
    text = _router_skill_text("arc:code", DESCRIPTION).replace(
        "version: 1.0.0\n", "version: 1.0.0\nenforce_arc_profile: true\n"
    )
    errors, _warnings = validate_text(text, "virtual/SKILL.md")
    assert any("retired keys" in item for item in errors)


def test_build_skill_document_extracts_intent_router() -> None:
    document = build_skill_document(_router_skill_text("arc:code", DESCRIPTION))
    assert document["frontmatter"]["name"] == "arc:code"
    assert document["intent_router"][0]["When"] == "matching work"


def test_intent_router_preserves_multiple_inline_code_spans() -> None:
    text = _router_skill_text("arc:code", DESCRIPTION).replace(
        "| matching work | this SKILL.md |", "| matching work | `arc:code` / `arc:verify` |"
    )
    document = build_skill_document(text)
    assert document["intent_router"][0]["Load"] == "`arc:code` / `arc:verify`"


def test_relative_link_extraction_ignores_inline_code_examples() -> None:
    text = "Use `[name](attachment-url)` as an example and [load](references/load.md) for details."
    assert extract_relative_links(text) == ["references/load.md"]


def test_validate_skill_schema_accepts_router_document() -> None:
    document = build_skill_document(_router_skill_text("arc:code", DESCRIPTION))
    assert validate_skill_schema(document, "virtual/SKILL.md", ROOT) == []


def test_validate_text_accepts_router_arc_skills() -> None:
    errors, warnings = validate_text(_router_skill_text("arc:code", DESCRIPTION), "virtual/SKILL.md")
    assert errors == []
    assert warnings == []


def test_validate_text_rejects_non_arc_skill_name() -> None:
    errors, _warnings = validate_text(_router_skill_text("plain-skill", DESCRIPTION), "virtual/SKILL.md")
    assert "virtual/SKILL.md: skill name must use arc:xxx namespace" in errors


def test_repo_validation_requires_root_cause_gate_for_repair_capable_skill(tmp_path: Path) -> None:
    skill_dir = tmp_path / "Arc" / "arc:code"
    skill_dir.mkdir(parents=True)
    description = (
        "Diagnoses a reproduced failure and repairs the confirmed cause with focused verification "
        "when logs, failing tests, or incident evidence are available. Not for review (use arc:audit)."
    )
    (skill_dir / "SKILL.md").write_text(_router_skill_text("arc:code", description), encoding="utf-8")
    (tmp_path / "schemas").mkdir()
    (tmp_path / "schemas" / "skill.schema.json").write_text(
        (ROOT / "schemas" / "skill.schema.json").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    (tmp_path / "schemas" / "trigger_corpus.yaml").write_text(
        "skills:\n  arc:code:\n    must_contain: [Diagnoses, failure, repairs]\n    positive: [a, b, c, d, e, f, g, h]\n    negative: [i, j, k]\n",
        encoding="utf-8",
    )

    errors, _warnings = validate_text(
        (skill_dir / "SKILL.md").read_text(encoding="utf-8"),
        str(skill_dir / "SKILL.md"),
        root=tmp_path,
        skill_path=skill_dir / "SKILL.md",
    )

    assert any("evidence-first root-cause repair gate" in item for item in errors)


def test_validate_text_accepts_every_arc_skill_name() -> None:
    for name in ("arc:explore", "arc:plan", "arc:code", "arc:verify", "arc:audit"):
        errors, warnings = validate_text(_router_skill_text(name, DESCRIPTION), "virtual/SKILL.md")
        assert errors == []
        assert warnings == []


def test_arc_layout_rejects_stray_directories_and_shared_entrypoints(tmp_path: Path) -> None:
    (tmp_path / "schemas").mkdir()
    (tmp_path / "schemas" / "skill.schema.json").write_text(
        (ROOT / "schemas" / "skill.schema.json").read_text(encoding="utf-8"), encoding="utf-8"
    )
    (tmp_path / "Arc" / "scripts").mkdir(parents=True)
    shared = tmp_path / "Arc" / "_shared"
    shared.mkdir()
    (shared / "SKILL.md").write_text("---\nname: arc:shared\n---\n", encoding="utf-8")
    (shared / "contract.md").write_text("[gone](missing.md) arc:sdlc\n", encoding="utf-8")

    errors, _warnings, _count = run_validation(tmp_path)

    assert any("Arc directories must be skill packages" in item for item in errors)
    assert any("shared layer must not contain SKILL.md" in item for item in errors)
    assert any("contract.md: broken relative link missing.md" in item for item in errors)
    assert any("contract.md: references retired Arc skills: arc:sdlc" in item for item in errors)


def test_all_skill_names_use_arc_namespace() -> None:
    for path in (ROOT / "Arc").rglob("SKILL.md"):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        assert frontmatter.get("name", "").startswith("arc:"), path


def test_skill_packages_keep_lifecycle_invariants() -> None:
    cases = {
        "arc:explore": ["_shared/code-intel.md", "impact"],
        "arc:plan": ["进度跟踪表.md", "00-前置约束.md", "_shared/finding-campaign.md", "lark-workflow-project-docs"],
        "arc:code": [
            "execution-truth.md",
            "_shared/code-intel.md",
            "scripts/gate.sh",
            "_shared/conventions/go-backend.md",
            "_shared/conventions/frontend.md",
            "_shared/conventions/comments.md",
            "_shared/api-contract.md",
        ],
        "arc:verify": ["execution-truth.md", "references/db.md", "FLAKY", "BLOCKED", "NOT_RUN"],
        "arc:audit": ["execution-truth.md", "git status --porcelain", "_shared/code-rot.md", "cnb-code-review"],
    }
    for name, phrases in cases.items():
        folder = ROOT / "Arc" / name
        package = "\n".join(
            path.read_text(encoding="utf-8")
            for path in sorted(folder.rglob("*.md"))
            if "scripts" not in path.relative_to(folder).parts
        )
        for phrase in phrases:
            assert phrase in package, f"{name} missing {phrase}"
        assert "_shared/contract.md" in (folder / "SKILL.md").read_text(encoding="utf-8"), name


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
