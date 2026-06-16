from pathlib import Path

from arc_core.skill_validation import (
    build_skill_document,
    parse_frontmatter,
    run_validation,
    validate_skill_schema,
    validate_text,
)

ROOT = Path("/Users/iluwen/Documents/Code/Skills")


def _arc_skill_text(name: str, expert: str) -> str:
    return f'''---
name: {name}
description: "Short description."
---
# Skill
## Overview
overview body
## Quick Contract
- **Trigger**: trigger body
- **Inputs**: input summary
- **Outputs**: output summary
- **Quality Gate**: gate summary
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
## Routing Matrix
- For routing, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
## Announce
announce body
## The Iron Law
rule body
## Workflow
workflow body
## Quality Gates
quality body
## Expert Standards
{expert}
## Scripts & Commands
scripts body
## Red Flags
flags body
## When to Use
- **Preferred Trigger**: preferred trigger body
- **Typical Scenario**: typical scenario body
- **Boundary Tip**: boundary tip body
## Input Arguments
| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | root path |
## Outputs
```text
output.md
```
'''


def test_parse_frontmatter_extracts_core_fields() -> None:
    text = """---
name: "arc:build"
description: "Code delivery."
---
# Title
"""
    frontmatter, error = parse_frontmatter(text)
    assert error is None
    assert frontmatter["name"] == "arc:build"
    assert frontmatter["description"] == "Code delivery."


def test_validate_text_reports_missing_required_heading_for_routed_skill() -> None:
    text = """---
name: "arc:build"
description: "Code delivery."
---
# Skill
## Overview
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert "virtual/SKILL.md: missing heading ## When to Use" in errors


def test_build_skill_document_extracts_sections() -> None:
    text = """---
name: "arc:build"
description: "Code delivery."
---
# Skill
## Overview
overview body
## Workflow
workflow body
"""
    document = build_skill_document(text)
    assert document["frontmatter"]["name"] == "arc:build"
    assert document["sections"][0]["heading"] == "## Overview"
    assert document["section_index"]["## Workflow"]["body"] == "workflow body"


def test_validate_skill_schema_accepts_minimal_structured_document() -> None:
    text = """---
name: "arc:build"
description: "Code delivery."
---
# Skill
## Overview
overview body
## When to Use
when-to-use body
"""
    document = build_skill_document(text)
    assert validate_skill_schema(document, "virtual/SKILL.md", ROOT) == []


def test_build_skill_document_extracts_quick_contract_inputs_and_outputs() -> None:
    text = """---
name: "arc:build"
description: "Code delivery."
---
# Skill
## Quick Contract
- **Trigger**: build trigger
- **Inputs**: input summary
- **Outputs**: output summary
- **Quality Gate**: gate summary
- **Decision Tree**: tree summary
## Input Arguments
| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | root path |
## Outputs
```text
project/.arc/example/
└── report.md
```
"""
    document = build_skill_document(text)
    assert document["quick_contract"]["trigger"] == "build trigger"
    assert document["input_arguments"][0]["parameter"] == "project_path"
    assert document["outputs_section"]["format"] == "text"
    assert "report.md" in document["outputs_section"]["content"]


def test_build_skill_document_extracts_numbered_input_arguments() -> None:
    text = """---
name: "arc:fix"
description: "Failure repair."
---
# Skill
## **Input Arguments**
1. **failure** (string, required)
   * Description: failure signal
2. **verification** (string, optional)
   * Description: verification command
"""
    document = build_skill_document(text)
    assert document["input_arguments"][0]["parameter"] == "failure"
    assert document["input_arguments"][0]["type"] == "string"
    assert document["input_arguments"][0]["required"] == "required"
    assert document["input_arguments"][1]["description"] == "verification command"


def test_validate_text_accepts_lean_arc_skills() -> None:
    cases = {
        "arc:clarify": "IEEE 29148 INVEST Given-When-Then",
        "arc:build": "DoD SemVer Contract Test RTO/RPO SBOM",
        "arc:define": "IEEE 29148 Domain-Driven Design Positioning Statement",
        "arc:docs": "Lark .lark.json Traceability Matrix Information Architecture",
        "arc:frontend": "Design Token Accessibility Responsive RBAC",
        "arc:fix": "SEV 5 Whys Fault Tree Blameless Postmortem Mandatory Hypothesis Rationalization Watch",
        "arc:audit": "Business Maturity Dependency Health Expert Review Card 9 Tab",
        "arc:security": "SAST SCA DAST OpenAPI Fuzz SBOM SARIF CWE CVSS OWASP Top 10 OWASP ASVS AuthZ",
    }
    for name, expert in cases.items():
        errors, warnings = validate_text(_arc_skill_text(name, expert), "virtual/SKILL.md", root=ROOT)
        assert errors == []
        assert warnings == []


def test_validate_text_rejects_unapproved_plain_skill() -> None:
    text = """---
name: "plain-skill"
description: "Non-arc namespace."
---
# Skill
## Overview
overview body
## When to Use
when body
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert "virtual/SKILL.md: skill name must use arc:xxx namespace or be an approved plain skill" in errors


def test_validate_text_accepts_approved_plain_skill() -> None:
    cases = [
        (
            "code-comment-conventions",
            "Apply Chinese comment templates for functions and controllers.",
        ),
        (
            "project-architecture-conventions",
            "Apply mandatory DIP and ONC-style architecture rules before coding.",
        ),
    ]
    for name, description in cases:
        text = f"""---
name: "{name}"
description: "{description}"
---
# Skill
## Overview
overview body
## When to Use
when body
"""
        errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
        assert errors == []
        assert warnings == []


def test_project_architecture_skill_locks_dip_onc_and_ponytail_contract() -> None:
    text = (ROOT / "project-architecture-conventions" / "SKILL.md").read_text(encoding="utf-8")

    required_phrases = [
        "Dependency Inversion Principle (DIP)",
        "Do not read an external ONC project",
        "Ponytail Conflict Resolution",
        "Required DIP boundary interfaces are not \"unrequested abstraction\"",
        "Do not create service interfaces, factories, config objects, or adapter interfaces solely because a folder exists.",
        "`contract`: Business-layer interface definitions.",
        "`services`: Business core logic.",
        "`helpers`: Helper utilities that belong only to this business module.",
        "`main` / `cmd/<app>` / `main.go`: Composition root.",
        "`pkg/utils/<name>`: Project-wide common utilities.",
        "pkg/utils -> no business-module dependency",
        "If a ponytail simplification would remove a required DIP boundary, keep the boundary",
    ]
    for phrase in required_phrases:
        assert phrase in text


def test_arc_code_editing_skills_require_project_architecture_conventions() -> None:
    for relative_path in ["Arc/arc:build/SKILL.md", "Arc/arc:fix/SKILL.md"]:
        text = (ROOT / relative_path).read_text(encoding="utf-8")
        assert "MUST apply `project-architecture-conventions` before" in text
        assert "stop and report if ponytail is required but unavailable or conflicting" in text


def test_validate_text_accepts_arc_frontend_skill() -> None:
    text = """---
name: "arc:frontend"
description: "Frontend engineering."
---
# Skill
## Overview
overview body
## Quick Contract
- **Trigger**: frontend baseline trigger
- **Inputs**: product type and palette
- **Outputs**: stack and token artifacts
- **Quality Gate**: baseline dependencies and theme tokens match
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
## Announce
announce body
## Input Arguments
| parameter | type | required | description |
|---|---|---|---|
| `product_type` | string | yes | target product type |
## The Iron Law
rule body
## Workflow
workflow body
## Quality Gates
quality body
## Expert Standards
Design Token Accessibility Responsive RBAC
## Scripts & Commands
scripts body
## Red Flags
flags body
## When to Use
- **Preferred Trigger**: frontend lifecycle work
- **Typical Scenario**: React, Vite, Tailwind, shadcn/ui project
- **Boundary Tip**: use another skill for backend-only work
## Outputs
```text
stack and palette summary
```
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert errors == []
    assert warnings == []


def test_run_validation_rejects_github_workflows_directory(tmp_path: Path) -> None:
    workflows_dir = tmp_path / ".github" / "workflows"
    workflows_dir.mkdir(parents=True)
    (workflows_dir / "ci.yml").write_text("name: CI\n", encoding="utf-8")

    errors, warnings, count = run_validation(tmp_path)

    assert count == 0
    assert warnings == []
    assert len(errors) == 1
    assert "GitHub Actions workflows are not allowed" in errors[0]
