---
name: arc:uml
description: "UML 与 Chen E-R 建模：基于可验证的代码和文档证据生成标准合规的架构与 UML 图谱。强制使用 drawio 技能生成原生图表。当用户请求架构图、UML 建模、时序图、ER 图、活动图或 draw.io 文件时触发。"
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
3. **Standardized Generation:** Generate robust, editable `.drawio` files via intermediate languages or JSON specifications, relying on execution scripts for layout and styling.

**Strict Prohibition:** LLMs are strictly prohibited from generating raw native `.drawio` XML coordinate arrays or style payloads directly. Direct generation consistently results in coordinate hallucinations, floating paths, and non-compliant styling. All native `.drawio` artifacts MUST be produced by proxy through intermediate scripts.

**Standard Diagrams (Class, Use Case, Activity, State, Sequence):** Must be generated using Mermaid syntax (`.mmd`) or PlantUML. The intermediate code must then be passed to `generate_mermaid_drawio.py` to compile into an embedded, perfectly rendered `.drawio` artifact.

**Deployment Diagrams:** Must be generated as structured JSON specifications (`deployment.json`), outlining nodes and orthogonal relationships, and then compiled via `generate_deployment_drawio.py`.

**E-R Diagrams:** Must strictly adhere to Chen Notation. The LLM must output a structured JSON specification (`er.json`) which is then compiled via `generate_er_drawio.py` to ensure standard UML shapes and anchored connectors.

All standard diagrams (Activity, Use Case, Sequence, Class, Deployment) must comply with:
- **UML 2.5.1 / ISO 19505** semantic specifications.
- Academic grading standards for software engineering and database design.

Detailed specifications are available in:
- [references/notation-standards.md](./references/notation-standards.md)
- [references/diagram-catalog.md](./references/diagram-catalog.md)
- [references/china-university-diagram-guidelines.md](./references/china-university-diagram-guidelines.md)
- [references/deployment-layout-spec.md](./references/deployment-layout-spec.md)

## Quick Contract

- **Trigger**: User requests for architecture diagrams, UML diagrams, activity diagrams, sequence diagrams, class diagrams, deployment diagrams, E-R diagrams, or explicit mentions of `drawio`/`draw.io`/`.drawio`.
- **Inputs**: Project path, business scenarios, target diagram types, deployment environments, export format requirements.
- **Outputs**: Diagram applicability matrix, evidence manifest, modeling briefs, native `.drawio` source files, and optional export formats.
- **Quality Gate**: Absolute traceability to evidence; strict Chen Notation for E-R; prohibition of XML coordinate computation for sequence diagrams; orthogonal binding for deployment diagrams; mandatory use of the `drawio` skill.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- Unified routing comparison: [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- Phased getting started view: [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- Cheat sheet: [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- In case of routing conflicts, the **When to Use** boundary notes within this skill specification supersede external routing matrices.

## Announce

Prior to execution, output the following precise statement:
> "Initiating diagram applicability analysis and evidence extraction. The `drawio` skill will be utilized to generate standards-compliant diagrams."

## Teaming Requirement

- Every execution cycle must explicitly define the `Owner`, `Executor`, and `Reviewer` roles.
- In single-agent environments, the final output must still reflect the conclusions of these three distinct perspectives to ensure a closed-loop "Decision-Execution-Review" process.

## The Iron Law

```text
NO DIAGRAM WITHOUT EVIDENCE
NO RELATION WITHOUT TRACEABILITY
NO FINAL UML DELIVERY WITHOUT DRAWIO
NO ER DIAGRAM WITHOUT CHEN NOTATION
```

## Workflow

1. **Context Scanning:** Analyze project structure, configurations, interfaces, database schemas, deployment manifests, and business flows.
2. **Applicability Matrix:** Evaluate 14 standard UML diagram types, classifying each as `required`, `recommended`, or `not-applicable`. Provide justification and evidence mappings.
3. **Modeling Briefs:** Generate a modeling brief for each `required` or `recommended` diagram, specifying objectives, evidence sources, notational constraints, and prohibited elements.
4. **Generation:** Construct the intermediate representations (e.g. `.mmd` for standard diagrams, `deployment.json` for deployments, `er.json` for Chen E-R). Execute the corresponding python compiler scripts (`generate_mermaid_drawio.py`, `generate_deployment_drawio.py`, `generate_er_drawio.py`) to generate the native `.drawio` files.
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
- **Source Files:** The definitive source file must be `.drawio`. If `svg/png/pdf` are exported, the exports containing embedded XML should be retained, or explicit documentation must state that the source `.drawio` was removed by the `drawio` skill.

## Expert Standards

- **Semantic Baseline:** UML semantics default to `UML 2.5.1 / ISO 19505`.
- **Primary Delivery:** Formal diagrams must be generated as native `.drawio` files. The `drawio` skill is the mandatory delivery pathway, not an optional extension.
- **Academic Standards:** Ensure comprehensive element inclusion, precise relationship semantics, clear nomenclature, and orderly layout optimized for immediate inspection.
- **E-R Diagrams:** Represent conceptual data models. Physical implementation details (foreign keys, data types, lengths, indices) are strictly prohibited in Chen Notation.
- **Activity Diagrams:** Utilize swimlanes for cross-role or cross-department flows. Concurrency must utilize explicit fork/join synchronization bars; decision diamonds must not represent concurrency.
- **Use Case Diagrams:** Strictly adhere to `<<include>>` (mandatory reused behavior) and `<<extend>>` (conditional extended behavior) semantics.
- **Sequence Diagrams:** Must model critical branches, exceptions, timeouts, and retries. Do not restrict modeling to the happy path. Mutually exclusive conditions must be encapsulated within a single `alt` fragment; `opt` is reserved for singular optional conditions. Self-calls must use loopback arrows (`A->>A: method()` in Mermaid), not box-and-arrow or two separate messages.
- **Deployment Diagrams:** Establish partitions and tiers prior to node placement. Nodes must align orthogonally; edges must bind to node anchors.
- **Class Diagrams:** Restrict modeling to attributes and operations relevant to the current communication objective. Do not exhaustively dump entity fields.
- **Assumptions:** Explicitly document modeling assumptions, evidence locations, and applicability boundaries per diagram.

## Scripts & Commands

- **Initialize UML directory and `.drawio` scaffolding:**
  ```bash
  python3 Arc/arc:uml/scripts/scaffold_uml_pack.py --output-dir <uml_dir> --types class,sequence,deployment
  ```
- **Generate Deployment Diagram from structured specification:**
  ```bash
  python3 Arc/arc:uml/scripts/generate_deployment_drawio.py \
    --spec <uml_dir>/diagram-specs/deployment.json \
    --output <uml_dir>/diagrams/deployment.drawio
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
- **Encapsulate Mermaid Diagram into a stable drawio file:**
  ```bash
  python3 Arc/arc:uml/scripts/generate_mermaid_drawio.py \
    --input <uml_dir>/diagrams/class.mmd \
    --output <uml_dir>/diagrams/class.drawio
  ```
- **Generate E-R Diagram from structured JSON specification:**
  ```bash
  python3 Arc/arc:uml/scripts/generate_er_drawio.py \
    --spec <uml_dir>/diagram-specs/er.json \
    --output <uml_dir>/diagrams/er-chen.drawio
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

Note: XML specifications, CLI export procedures, and file access protocols for `drawio` are governed by the `drawio` skill constraints and are not redefined here.

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
- **Disconnected Deployment Nodes:** Routing edges that are not anchored to nodes, or extensively relying on floating endpoints and manual coordinate overrides.
- Defaulting to Mermaid format for diagrams other than Sequence Diagrams.

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
  - If the user solely requests format conversion ("export this file to png/pdf/svg"), delegate directly to the `drawio` skill.

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

- The definitive source for formal diagrams must be native `draw.io` / `drawio` files (`.drawio`).
- All diagram generation actions must be delegated to the `drawio` skill.
- When delivering image or print formats, prioritize `.drawio.svg`, `.drawio.png`, or `.drawio.pdf` to preserve embedded XML for future editing.
- E-R Diagrams must utilize Chen Notation. Detailed rules: [references/notation-standards.md](./references/notation-standards.md).
- Academic evaluation criteria: [references/china-university-diagram-guidelines.md](./references/china-university-diagram-guidelines.md).

### 15.1. Execution Script Contract

Prior to finalizing any diagram, explicitly utilize the proxy scripts instead of attempting direct XML authoring:
- **Standard Diagrams (Sequence, Class, Activity, Use Case, State):**
  - **Delivery Format:** Prioritize `.mmd` (Mermaid) or `.puml` (PlantUML).
  - **Constraint:** Direct computation of native XML is strictly prohibited.
  - **Recommended Pipeline:** Generate `*.drawio` via `generate_mermaid_drawio.py`.
- **E-R Diagrams:**
  - **Input Generation:** Draft via `diagram-specs/er.json`.
  - **Constraint:** Must strictly enforce Chen Notation mappings (entities, attributes, relationships, connections).
  - **Recommended Pipeline:** Generate `.drawio` via `generate_er_drawio.py`.
- **Deployment Diagrams:**
  - **Input Generation:** Draft via `diagram-specs/deployment.seed.txt`.
  - **Partitioning Strategy:** e.g., "Client Zone / Gateway Tier / Application Tier / Data Tier".
  - **Alignment Protocol:** e.g., "Align peer nodes orthogonally to a 10px grid".
  - **Routing Protocol:** Enforce `source/target` node anchoring. Prioritize orthogonal routing. Floating endpoints are prohibited.
  - **Recommended Pipeline:** Maintain `diagram-specs/deployment.json` and generate `.drawio` via script.

## Dependencies

- **Organization Contract:** Mandatory. Enforce the `Owner / Executor / Reviewer` closed-loop execution model.
- **ace-tool / Code Search:** Mandatory. Utilized for evidence extraction.
- **drawio Skill:**
  - **Standard Diagrams:** Mandatory. Required for formal diagram generation.
  - **Sequence Diagrams:** Conditional. Permitted only when encapsulating Mermaid nodes; prohibited for native XML generation.
- **Web Search (web / Exa):** Conditional. Utilized for specification retrieval or standard verification.
- **Local Generation Scripts:**
  - `draft_deployment_spec.py`: Recommended. Converts concise text seeds into `deployment.json`.
  - `generate_deployment_drawio.py`: Recommended. Renders structured specifications into aligned `.drawio` files.
  - `generate_mermaid_drawio.py`: Recommended. Encapsulates intermediate `.mmd` diagrams into stable `.drawio` files.
  - `generate_er_drawio.py`: Recommended. Renders Chen E-R JSON specifications into compliant `.drawio` files.

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
   - **Standard Diagrams (Sequence, Class, Activity, etc.):** Generate intermediate code (`.mmd` or `.puml`). Invoke `generate_mermaid_drawio.py` to produce a containerized `.drawio` file.
   - **Deployment Diagrams:** Draft `diagram-specs/deployment.seed.txt`, convert to `deployment.json`, and invoke `generate_deployment_drawio.py` to produce `.drawio`.
   - **Chen E-R Diagrams:** Draft `diagram-specs/er.json` (listing entities, relationships, attributes, and connections) and invoke `generate_er_drawio.py` to generate the `.drawio` representation.
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
    ├── <diagram-type>.mmd          # Standard diagrams primary source
    ├── <diagram-type>.drawio       # Containerized native drawio files
    ├── <diagram-type>.drawio.svg
    ├── <diagram-type>.drawio.png
    ├── deployment.drawio
    ├── er-chen.drawio
    ├── er-chen.drawio.svg
    └── ...
```

**Delivery Protocol:**
- Only generate file extensions explicitly requested by the user. Do not arbitrarily populate all formats.
- Retain exports with embedded XML as the primary editable artifact if the `drawio` skill automatically removes the source `.drawio` post-export. Ensure this behavior is documented in `diagram-index.md`.
