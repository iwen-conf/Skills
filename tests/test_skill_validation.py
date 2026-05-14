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
description: "包含中文描述"
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
description: "包含中文描述"
---
# Title
"""
    frontmatter, error = parse_frontmatter(text)
    assert error is None
    assert frontmatter["name"] == "arc:build"
    assert frontmatter["description"] == "包含中文描述"


def test_validate_text_reports_missing_required_heading_for_routed_skill() -> None:
    text = """---
name: "arc:build"
description: "包含中文描述"
---
# Skill
## Overview
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert "virtual/SKILL.md: missing heading ## When to Use" in errors


def test_build_skill_document_extracts_sections() -> None:
    text = """---
name: "arc:build"
description: "包含中文描述"
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
description: "包含中文描述"
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
description: "包含中文描述"
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
description: "包含中文描述"
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
        "arc:fix": "SEV 5 Whys Fault Tree Blameless Postmortem Mandatory Hypothesis Rationalization Watch",
        "arc:audit": "Business Maturity Dependency Health Expert Review Card 9 Tab",
    }
    for name, expert in cases.items():
        errors, warnings = validate_text(_arc_skill_text(name, expert), "virtual/SKILL.md", root=ROOT)
        assert errors == []
        assert warnings == []


def test_validate_text_does_not_enforce_deleted_arc_profiles_on_generic_skills() -> None:
    text = _arc_skill_text("arc:build", "DoD SemVer Contract Test RTO/RPO SBOM")
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert errors == []
    assert warnings == []


def test_validate_text_accepts_fusion_terminal_table_skill() -> None:
    text = """---
name: "terminal-table-output"
description: "包含中文描述，用于终端盒线表输出"
---
# Skill
## Overview
overview body
## Quick Contract
- **Trigger**: table trigger
- **Inputs**: compact table data
- **Outputs**: box-drawing table
- **Quality Gate**: alignment gate
- **Decision Tree**: use a table only when compact
## Announce
announce body
## Input Arguments
| parameter | type | required | description |
|---|---|---|---|
| `headers` | array | yes | column titles |
| `rows` | array | yes | row data |
## The Iron Law
rule body
## Workflow
workflow body
## Quality Gates
quality body
## Red Flags
flags body
## When to Use
- **首选触发**：需要紧凑表格
- **典型场景**：状态汇总、比较矩阵
- **边界提示**：长文本不用表格
## Outputs
```text
┌──┬──┐
│A │B │
└──┴──┘
```
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert errors == []
    assert warnings == []


def test_validate_text_accepts_frontend_stack_baseline_skill() -> None:
    text = """---
name: "frontend-stack-baseline"
description: "包含中文描述，用于通用前端基线"
---
# Skill
## Overview
overview body
## Quick Contract
- **Trigger**: frontend baseline trigger
- **Inputs**: product type and palette
- **Outputs**: stack and token artifacts
- **Quality Gate**: baseline dependencies and theme tokens match
- **Decision Tree**: use the baseline for React frontend work
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
## Red Flags
flags body
## When to Use
- **首选触发**：需要 Web 前端基线
- **典型场景**：React、Vite、Tailwind、shadcn/ui 项目
- **边界提示**：非前端任务不用本 skill
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
