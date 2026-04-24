---
name: arc:uml
description: "UML 与 Chen E-R 建模：基于可验证的代码和文档证据生成标准合规的架构与 UML 图谱。强制使用 PlantUML 和 Mermaid 语法（必须包含这两种）生成图表。当用户请求架构图、UML 建模、时序图、ER 图、活动图或图表文件时触发。"
version: 1.0.0
allowed_tools:
  - Bash
  - Read
  - Edit
  - Write
  - Grep
  - Glob
hooks:
  PreToolUse:
    - matcher: Bash
      hooks:
        - type: command
          command: "bash ${ARC_SKILL_DIR}/scripts/check-destructive.sh"
          statusMessage: "Checking for destructive commands..."
---

# arc:uml — Evidence-Based UML & Chen E-R Modeling

## Overview

The `arc:uml` skill is responsible for the following core objectives:
1. **Applicability Analysis:** Determine the necessity and appropriateness of specific diagram types for the current project context.
2. **Evidence Extraction:** Extract verifiable traceability evidence from source code, configuration files, API specifications, business processes, and database schemas.
3. **Standardized Generation:** Generate robust, editable diagrams using both PlantUML (`.puml`) and Mermaid (`.mmd`) syntax.

**Standard Diagrams (Class, Use Case, Activity, State, Sequence):** Must be generated using both Mermaid syntax (`.mmd`) and PlantUML (`.puml`).

**Deployment Diagrams:** Must be generated using both Mermaid syntax (`.mmd`) and PlantUML (`.puml`).

**E-R Diagrams:** Must strictly adhere to Chen Notation. The LLM must output both PlantUML (`.puml`) and Mermaid (`.mmd`) representations to ensure standard UML shapes and relationships.

All standard diagrams (Activity, Use Case, Sequence, Class, Deployment) must comply with:
- **UML 2.5.1 / ISO 19505** semantic specifications.
- Academic grading standards for software engineering and database design.

Detailed specifications are available in:
- [references/notation-standards.md](./references/notation-standards.md)
- [references/diagram-catalog.md](./references/diagram-catalog.md)
- [references/china-university-diagram-guidelines.md](./references/china-university-diagram-guidelines.md)
- [references/deployment-layout-spec.md](./references/deployment-layout-spec.md)

## Quick Contract

- **Trigger**: User requests for architecture diagrams, UML diagrams, activity diagrams, sequence diagrams, class diagrams, deployment diagrams, E-R diagrams, or explicit mentions of `.puml`/`.mmd`/`PlantUML`/`Mermaid`.
- **Inputs**: Project path, business scenarios, target diagram types, deployment environments, export format requirements.
- **Outputs**: Diagram applicability matrix, evidence manifest, modeling briefs, PlantUML and Mermaid source files, and optional export formats.
- **Quality Gate**: Absolute traceability to evidence; strict Chen Notation for E-R; mandatory use of both PlantUML and Mermaid.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- Unified routing comparison: [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- Phased getting started view: [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- Cheat sheet: [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- In case of routing conflicts, the **When to Use** boundary notes within this skill specification supersede external routing matrices.

## Announce

Prior to execution, output the following precise statement:
> "Initiating diagram applicability analysis and evidence extraction. Both PlantUML and Mermaid will be utilized to generate standards-compliant diagrams."

## Teaming Requirement

- Every execution cycle must explicitly define the `Owner`, `Executor`, and `Reviewer` roles.
- In single-agent environments, the final output must still reflect the conclusions of these three distinct perspectives to ensure a closed-loop "Decision-Execution-Review" process.

## The Iron Law

```text
NO DIAGRAM WITHOUT EVIDENCE
NO RELATION WITHOUT TRACEABILITY
NO FINAL UML DELIVERY WITHOUT BOTH PLANTUML AND MERMAID
NO ER DIAGRAM WITHOUT CHEN NOTATION
```

## Workflow

1. **Context Scanning:** Analyze project structure, configurations, interfaces, database schemas, deployment manifests, and business flows.
2. **Applicability Matrix:** Evaluate 14 standard UML diagram types, classifying each as `required`, `recommended`, or `not-applicable`. Provide justification and evidence mappings.
3. **Modeling Briefs:** Generate a modeling brief for each `required` or `recommended` diagram, specifying objectives, evidence sources, notational constraints, and prohibited elements.
4. **Generation:** Construct the intermediate representations (`.mmd` and `.puml`) for standard diagrams, deployments, and Chen E-R. Both formats must be provided for every generated diagram.
5. **Validation:** Perform cross-diagram consistency verification. Output maintenance recommendations and residual risk assessments.

## Quality Gates

- **Traceability:** Every diagram must cite corresponding evidence (e.g., `file:line`, configuration path, interface definition, schema source, or specific requirement text).
- **Applicability Justification:** All `not-applicable` classifications must include explicit reasoning.
- **Nomenclature:** Core object naming must strictly align with code, configuration, or established business domain terminology.
- **Consistency:** Deployment, Component, and Configuration diagrams must cross-validate without contradiction.
- **Activity Diagrams:** Must depict explicit control flows. Direct textual transposition into sequential blocks is prohibited. Must include standard elements: Initial Node, Action Nodes, Control Flows, and at least one Activity Final Node.
- **Use Case Diagrams:** Must differentiate system boundaries, actors, and use case relationships. Process steps must not be modeled as use cases. Actors must reside outside the system boundary; use cases must reside inside.
- **Sequence Diagrams:** Must depict chronological interaction. Static dependencies must not be modeled as messages. Divergent conditions must utilize `alt` combined fragments; manual message vector branching is prohibited. Self-calls (an object invoking its own method) must use loopback arrows on the same lifeline, not separate box-and-arrow constructs.
- **Component Diagrams:** Must utilize standard component notation or the `<<component>>` stereotype, explicitly defining dependencies and interfaces.
- **Class Diagrams:** Must semantically differentiate association, dependency, aggregation, composition, and generalization. Arrow semantics must not be interchanged. Use standard visibility modifiers (`+`, `-`, `#`, `~`).
- **State Machine Diagrams:** Must include an initial state, at least one stable state, state transitions, and an explicit terminal state.
- **E-R Diagrams:** Must strictly adhere to Chen Notation (Entity rectangles, Relationship diamonds, Attribute ellipses, Primary Key underlines). Utilize weak entity, multi-valued attribute, and total participation notations where applicable.
- **Source Files:** The definitive source files must be `.puml` and `.mmd`.

## Styling & Formatting Requirements

- **Readability & Font Size:** Generated diagrams MUST be optimized for readability. Always explicitly configure larger font sizes. For PlantUML, include directives like `skinparam defaultFontSize 16` and adjust component padding. For Mermaid, use `%%{init: {'theme': 'default', 'themeVariables': {'fontSize': '16px'}}}%%` or similar directives.
- **Line Routing & Layout:** Minimize line crossings. Use orthogonal or polyline routing where possible. Keep elements well-spaced to prevent the diagram from looking cluttered or messy.

## Expert Standards

- **Semantic Baseline:** UML semantics default to `UML 2.5.1 / ISO 19505`.
- **Primary Delivery:** Formal diagrams must be generated as both `.puml` and `.mmd` files.
- **Academic Standards:** Ensure comprehensive element inclusion, precise relationship semantics, clear nomenclature, and orderly layout optimized for immediate inspection.
- **E-R Diagrams:** Represent conceptual data models. Physical implementation details (foreign keys, data types, lengths, indices) are strictly prohibited in Chen Notation.
- **Activity Diagrams:** Utilize swimlanes for cross-role or cross-department flows. Concurrency must utilize explicit fork/join synchronization bars; decision diamonds must not represent concurrency.
- **Use Case Diagrams:** Strictly adhere to `<<include>>` (mandatory reused behavior) and `<<extend>>` (conditional extended behavior) semantics.
- **Sequence Diagrams:** Must model critical branches, exceptions, timeouts, and retries. Do not restrict modeling to the happy path. Mutually exclusive conditions must be encapsulated within a single `alt` fragment; `opt` is reserved for singular optional conditions. Self-calls must use loopback arrows (`A->>A: method()` in Mermaid), not box-and-arrow or two separate messages.
- **Deployment Diagrams:** Establish partitions and tiers prior to node placement. Nodes must align orthogonally; edges must bind to node anchors.
- **Class Diagrams:** Restrict modeling to attributes and operations relevant to the current communication objective. Do not exhaustively dump entity fields.
- **Assumptions:** Explicitly document modeling assumptions, evidence locations, and applicability boundaries per diagram.

## Scripts & Commands

- **Initialize UML directory and scaffolding:**
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types class,sequence,deployment
  ```
- **Draft Deployment specification from text seed:**
  ```bash
  python3 Arc/arc:uml/scripts/draft_deployment_spec.py \
    --input <uml_dir>/diagram-specs/deployment.seed.txt \
    --output <uml_dir>/diagram-specs/deployment.json
  ```
- **Render Mermaid Sequence Diagram to SVG:**
  ```bash
  node Arc/arc:uml/scripts/render_beautiful_mermaid_svg.mjs \
    --input <uml_dir>/diagrams/sequence.mmd \
    --output <uml_dir>/diagrams/sequence.mmd.svg
  ```

- **Execute multi-pass review on UML delivery directory:**
  ```bash
  python3 Arc/arc:uml/scripts/review_uml_pack.py --uml-dir <uml_dir>
  ```
- **Initialize scaffolding for all UML diagram types:**
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types all
  ```
- **Initialize scaffolding including Chen E-R Diagram:**
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types all --include-er-chen
  ```
- **Primary execution command:**
  ```bash
  arc uml
  ```

## Red Flags

- Generating diagrams from templates without verifiable project evidence.
- Forcing the generation of all 14 UML diagram types when only a specific subset is required.
- Substituting E-R Diagrams (Chen Notation) with Crow's Foot, IDEF1X, Class Diagrams, or physical database schemas.
- Constructing Activity Diagrams without start/end nodes, branches, or concurrency controls (merely stacking sequential steps).
- Utilizing database tables, API endpoints, or implementation class names as actors or use cases within Use Case Diagrams.
- Designing Sequence Diagrams lacking returns, exceptions, or conditional branches.
- **Incorrect Self-Calls in Sequence Diagrams:** Rendering self-calls as a box with an arrow to another element, or as two separate messages, instead of a loopback arrow on the same lifeline.
- **Spaghetti Sequence Diagrams:** Excessive message counts (>20), unordered lifelines, missing activation boxes, or chaotic vector intersections.
- **Pseudo-Branching in Sequence Diagrams:** Modeling divergent conditions as parallel message vectors without utilizing `alt` or `opt` combined fragments.
- **Bypassing Automated Validation:** Delivering diagrams without executing `validate_diagram.py`.
- Defaulting to only one syntax (e.g. only Mermaid) instead of generating both PlantUML and Mermaid versions.

## Context Budget

- Restrict evidence extraction to critical directories, configurations, flows, and schemas. Do not parse the entire repository.
- Limit evidence citations to 5-20 critical lines per diagram.
- For large-scale systems, partition diagrams by domain to prevent cognitive overload.

## When to Use

- **Preferred Trigger**: Requirements for UML or E-R diagrams to support architectural communication, handovers, technical reviews, academic modeling, thesis defense, or system evolution.
- **Typical Scenario**: Onboarding, architecture review, technical due diligence, requirements analysis, database modeling, or pre-release documentation.
- **Boundary Tip**:
  - If the repository structure is unknown, utilize `arc:cartography` first.
  - If the primary objective is quality auditing rather than modeling, utilize `arc:audit` first.


## Input Arguments

| Parameter | Type | Required | Description |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the project root directory. |
| `project_name` | string | no | Project identifier; inferred from path if omitted. |
| `diagram_types` | array | no | Explicit list of diagram types; automatically inferred if omitted. |
| `business_scenarios` | array | no | Critical business scenarios for Activity/Use Case/Sequence diagrams. |
| `deployment_targets` | array | no | Target environments (e.g., `k8s`, `vm`, `on-prem`). |
| `include_er` | boolean | no | Flag to include E-R diagrams; inferred from evidence if omitted. |
| `depth_level` | string | no | Extent of modeling: `quick`, `standard`, `deep`. Default: `standard`. |
| `export_format` | string | no | Target export format: `none`, `svg`, `png`, `pdf`, `jpg`. Default: `none`. |
| `render_format` | string | no | Legacy alias for `export_format`. |
| `output_dir` | string | no | Output destination. Default: `<project_path>/.arc/uml/<project-name>/`. |

## Notation Standards

- The definitive source for formal diagrams must be both PlantUML and Mermaid files.
- E-R Diagrams must utilize Chen Notation. Detailed rules: [references/notation-standards.md](./references/notation-standards.md).
- Academic evaluation criteria: [references/china-university-diagram-guidelines.md](./references/china-university-diagram-guidelines.md).

### 15.1. Execution Script Contract

Prior to finalizing any diagram, generate both `.puml` and `.mmd` formats:
- **Standard Diagrams (Sequence, Class, Activity, Use Case, State):**
  - **Delivery Format:** Require both `.mmd` (Mermaid) and `.puml` (PlantUML).
- **E-R Diagrams:**
  - **Constraint:** Must strictly enforce Chen Notation mappings (entities, attributes, relationships, connections) in both formats.
- **Deployment Diagrams:**
  - **Delivery Format:** Require both `.mmd` (Mermaid) and `.puml` (PlantUML).

## Dependencies

- **Organization Contract:** Mandatory. Enforce the `Owner / Executor / Reviewer` closed-loop execution model.
- **ace-tool / Code Search:** Mandatory. Utilized for evidence extraction.
- **Web Search (web / Exa):** Conditional. Utilized for specification retrieval or standard verification.

## Instructions (Execution Process)

### Phase 1: Project Modeling Reconnaissance
1. Scan project modules, entities, interfaces, configurations, schemas, deployments, and critical flows.
2. Generate `context/project-snapshot.md` to persist preliminary evidence.

### Phase 2: Applicability Determination
1. If `diagram_types` is unspecified, evaluate all 14 standard UML diagram types.
2. Evaluate the necessity of E-R diagrams based on data persistence evidence.
3. Generate `diagram-plan.md`, documenting conclusions, justifications, evidence citations, and priorities.

### Phase 3: Modeling Briefs and Output Generation
1. Initialize delivery directory and scaffolding:
   ```bash
   python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <output_dir> --types all --include-er-chen
   ```
2. Populate `diagram-briefs/<diagram>.md` for all `required` and `recommended` diagrams.
3. Iteratively generate diagram files:
   - For every required diagram (Sequence, Class, Activity, Deployment, E-R, etc.), generate both the `.mmd` and `.puml` files.
4. Execute required format exports.
5. Update `diagram-index.md` to reflect delivery status.

### Phase 4: Consistency Verification
1. **Automated Quality Validation:** Execute validation scripts against all generated diagrams:
   ```bash
   python3 Arc/arc:uml/scripts/validate_diagram.py <output_dir>/diagrams/<diagram-file> --type <type>
   ```
   - *Failure protocol:* Diagram generation errors must be resolved and regenerated. Defective artifacts must not be delivered.
   - *Warning protocol:* Evaluate feasibility of immediate remediation or document as technical debt.
2. **Directory-Level Review:** Execute a comprehensive multi-pass review on the delivery directory:
   ```bash
   python3 Arc/arc:uml/scripts/review_uml_pack.py --uml-dir <output_dir>
   ```
   - Resolve identified state drift, missing artifacts, or secondary validation failures before concluding the task.
3. **Nomenclature Consistency:** Verify unified naming conventions across classes, components, services, nodes, roles, and entities.
4. **Cross-Diagram Consistency:** Verify bidirectional consistency: Component ↔ Deployment ↔ Configuration; Activity ↔ Sequence ↔ Use Case.
5. **E-R Diagram Verification:** Confirm strict adherence to Chen Notation and conceptual modeling boundaries.
6. **Finalization:** Generate `validation-summary.md` detailing outcomes and maintenance strategies.

## Outputs

```text
<project_path>/.arc/uml/<project-name>/
├── context/
│   └── project-snapshot.md
├── diagram-plan.md
├── diagram-index.md
├── validation-summary.md
├── diagram-specs/
│   ├── deployment.seed.txt
│   ├── deployment.json
│   └── er.json
├── diagram-briefs/
│   ├── <diagram-type>.md
│   └── er-chen.md
└── diagrams/
    ├── <diagram-type>.mmd          # Mermaid source
    ├── <diagram-type>.puml         # PlantUML source
    ├── <diagram-type>.mmd.svg      # Exported SVG
    ├── deployment.mmd
    ├── deployment.puml
    ├── er-chen.mmd
    ├── er-chen.puml
    └── ...
```

**Delivery Protocol:**
- Only generate file extensions explicitly requested by the user. Do not arbitrarily populate all formats.

