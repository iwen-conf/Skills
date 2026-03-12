from pathlib import Path

from arc_core.skill_validation import (
    build_skill_document,
    parse_frontmatter,
    validate_skill_schema,
    validate_text,
)

ROOT = Path("/Users/iluwen/Documents/Code/Skills")


def test_parse_frontmatter_extracts_core_fields() -> None:
    text = """---
name: \"arc:test\"
description: \"包含中文描述\"
---
# Title
"""
    frontmatter, error = parse_frontmatter(text)
    assert error is None
    assert frontmatter["name"] == "arc:test"
    assert frontmatter["description"] == "包含中文描述"


def test_validate_text_reports_missing_required_heading_for_routed_skill() -> None:
    text = """---
name: \"arc:build\"
description: \"包含中文描述\"
---
# Skill
## Overview
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert "virtual/SKILL.md: missing heading ## When to Use" in errors


def test_build_skill_document_extracts_sections() -> None:
    text = """---
name: \"arc:build\"
description: \"包含中文描述\"
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
name: \"arc:build\"
description: \"包含中文描述\"
---
# Skill
## Overview
overview body
## When to Use
when-to-use body
"""
    document = build_skill_document(text)
    assert validate_skill_schema(document, "virtual/SKILL.md", ROOT) == []


def test_build_skill_document_extracts_quick_contract_and_inputs() -> None:
    text = """---
name: \"arc:build\"
description: \"包含中文描述\"
---
# Skill
## Quick Contract
- **Trigger**: build trigger
- **Inputs**: input summary
- **Outputs**: output summary
- **Quality Gate**: gate summary
- **Decision Tree**: tree summary
## Input Arguments
| parameter | type | Required | illustrate |
|------|------|------|------|
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
name: \"arc:e2e\"
description: \"包含中文描述\"
---
# Skill
## **Input Arguments**
1. **test_objective** (string, required)
   * Description: business goal
2. **personas** (array, required)
   * Description: persona list
"""
    document = build_skill_document(text)
    assert document["input_arguments"][0]["parameter"] == "test_objective"
    assert document["input_arguments"][0]["type"] == "string"
    assert document["input_arguments"][0]["required"] == "required"
    assert document["input_arguments"][1]["description"] == "persona list"


def test_validate_text_accepts_structured_sections_for_real_patterns() -> None:
    text = """---
name: \"arc:build\"
description: \"包含中文描述\"
---
# Skill
## Overview
overview body
## Quick Contract
- **Trigger**: build trigger
- **Inputs**: input summary
- **Outputs**: output summary
- **Quality Gate**: gate summary
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).
## Routing Matrix
- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
## Announce
announce body
## The Iron Law
rule body
## Workflow
workflow body
## Quality Gates
quality body
## Expert Standards
DoD SemVer Contract Test RTO/RPO SBOM
## Scripts & Commands
scripts body
## Red Flags
flags body
## When to Use
- **Preferred Trigger**: preferred trigger body
- **Typical Scenario**: typical scenario body
- **Boundary Tip**: boundary tip body
## Input Arguments
| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | root path |
| `task_name` | string | yes | task name |
## Outputs
```text
project/.arc/example/
└── report.md
```
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert errors == []
    assert warnings == []


def test_validate_text_accepts_arc_aigc_keywords() -> None:
    text = """---
name: "arc:aigc"
description: "包含中文描述"
---
# Skill
## Overview
overview body
## Quick Contract
- **Trigger**: aigc trigger
- **Inputs**: input summary
- **Outputs**: output summary
- **Quality Gate**: gate summary
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).
## Routing Matrix
- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
## Announce
announce body
## The Iron Law
rule body
## Workflow
workflow body
## Quality Gates
quality body
## Expert Standards
chunked rewrite citation fidelity two-stage polish semantic drift authorial voice human review
## Scripts & Commands
scripts body
## Red Flags
flags body
## When to Use
- **Preferred Trigger**: preferred trigger body
- **Typical Scenario**: typical scenario body
- **Boundary Tip**: boundary tip body
## Input Arguments
| parameter | type | Required | illustrate |
|------|------|------|------|
| `target_text` | string | yes | draft path |
## Outputs
```text
project/.arc/aigc/session/
└── polished-text.md
```
"""
    errors, warnings = validate_text(text, "virtual/SKILL.md", root=ROOT)
    assert errors == []
    assert warnings == []
