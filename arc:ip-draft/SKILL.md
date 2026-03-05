---
name: "arc:ip-draft"
description: "知产起草：根据审查结果生成申请文档草稿。"
---

# arc:ip-draft — patent/software document writing

## Overview

`arc:ip-draft` adopts the **multi-Agent collaboration model** to focus on document writing and does not make feasibility decisions. It consumes the review results of `arc:ip-check` and outputs an editable application document draft combined with the project code context.

**Core Competencies**:
- Three Agents draft concurrently and independently (architecture technical solution/deep implementation details/writing user documentation)
- Cross-review mechanism ensures terminology consistency and technical accuracy
- Evidence-driven documentation writing (each technical description can be traced back to the code)
- Structured draft output (software/patent separate output)

## Quick Contract

- **Trigger**: The review handover is ready and a draft of soft copy/patent application materials needs to be generated.
- **Inputs**: Project path, `ip-audit` handover directory, target document scope and application subject information.
- **Outputs**: Track drafts (`copyright/`, `patent/`), term alignment results, and writing logs.
- **Quality Gate**: The evidence traceability and terminology consistency check of `## Quality Gates` must be passed before delivery.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:ip-draft` to read the review handover first, and then draft the software and patent documents separately."

## The Iron Law

```
NO IP DRAFTING WITHOUT AUDIT HANDOFF OR TRACEABLE CODE EVIDENCE
```

High-confidence technical document drafts should not be output without review handoff or traceability evidence.

## Workflow

1. Read the `arc:ip-check` handover file and establish the writing context.
2. Multi-Agent and initiate draft technical solutions, implementation details and documentation chapters.
3. Cross-review and harmonize terminology, boundaries and evidence references.
4. Output soft works and patent drafts and writing logs in sub-directories.

## Quality Gates

- Each key technical description should be traceable back to code or review handoff.
- Soft copy and patented materials must be physically separated and exported.
- The glossary must be consistent throughout the text.
- The draft must be marked with an "editable draft" position to avoid being misled into the final legal text.

## Expert Standards

- 专利草稿需遵循 `摘要-背景-发明内容-附图说明-具体实施方式-权利要求` 结构。
- 权利要求需采用 `宽-中-窄` 梯度布局，并检查前后引用基础一致性。
- 软著材料需覆盖功能说明、架构说明、关键源码证据与版本信息一致性。
- 全文执行 `术语控制`：同一对象单一命名，避免 claim scope 漂移。
- 交付必须附 `待法务复核清单`，明确需律师确认的高风险表述。

## Scripts & Commands

- 起草工作区脚手架：`python3 arc:ip-draft/scripts/scaffold_drafting_case.py --project-path <project_path>`
- 文档渲染（双轨）：`python3 arc:ip-draft/scripts/render_ip_documents.py --case-dir <case_dir> --target-docs both`
- 文档渲染（仅专利）：`python3 arc:ip-draft/scripts/render_ip_documents.py --case-dir <case_dir> --target-docs patent`
- 文档渲染（仅软著）：`python3 arc:ip-draft/scripts/render_ip_documents.py --case-dir <case_dir> --target-docs copyright`
- 运行时主命令：`arc ip-draft`

## Red Flags

- Skip the handover documents and go straight to speculating on the technical details.
- Mixing soft works and patent content leads to structural confusion.
- Term drift, multiple names for the same object.
- Outputs a conclusive conclusion without citing evidence.

## Mandatory Linkage (cannot be fought alone)

1. By default, `arc:ip-check` is read first and the product: `handoff/ip-drafting-input.json` is read.
2. If the handover document is missing, priority will be given to executing `arc:ip-check`; only a minimal draft will be made when explicitly requested by the user.
3. Document content must be linked back to the project context: `CLAUDE.md` + `ace-tool` evidence.
4. For terminological conflicts or unclear technical routes, connect `arc:decide` in series to make corrections before finalizing.

## When to Use

- **首选触发**: `arc:ip-check` has been handed over, and an editable draft of application materials needs to be generated.
- **典型场景**: Writing of soft copy description, patent disclosure document, claims and accompanying drawing description.
- **边界提示**: If the feasibility review has not been completed, execute `arc:ip-check` first.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the target project root directory |
| `project_name` | string | no | Deduced from path by default |
| `audit_case_dir` | string | no | `arc:ip-check` Output directory; default `<project_path>/.arc/arc:ip-check/<project-name>/` |
| `software_name` | string | no | Software name, if not provided, it will be read from the handover file. |
| `applicant_name` | string | no | Application subject name |
| `target_docs` | enum | no | `copyright` / `patent` / `both`, default `both` |
| `output_dir` | string | no | Default `<project_path>/.arc/arc:ip-draft/<project-name>/` |

## Dependencies

* **Organization Contract**: Required. Following `docs/orchestration-contract.md`, scheduling is implemented through the runtime adaptation layer.
- **arc:ip-check** (strongly recommended): as the main input.
- **ace-tool MCP** (required): Correct technical details and code evidence in the documentation.
- **arc:init** (recommended): Reuse module index to reduce repeated scanning.
- **Scheduling API** (required): Dispatch `architecture` / `deep` / `writing` three Agent collaboration.

## Context Priority (mandatory)

1. `audit_case_dir/handoff/ip-drafting-input.json`
2. `.arc/arc:ip-draft/<project>/context/doc-context.md`
3. Project `CLAUDE.md`
4. `ace-tool` Source code scan

## Critical Rules

1. **Don’t make it up**: Any technical details must be traceable back to code or review handoff information.
2. **Consistent terminology**: The same object is called uniformly throughout the text, and synonyms are not allowed to be freely substituted.
3. **Complete structure**: The disclosure document, claims, and documentation must be completely output according to the template chapters.
4. **Draft Positioning**: The output is an "editable application draft" and may not be claimed to be the final legal text.
5. **Double-track splitting**: Soft-coated materials and patented materials are produced in separate catalogs, without mixing.
6. **Multi-Agent collaboration**: Architecture/deep/writing three agents must be used and draft + cross-review initiated.

## Multi-Agent Architecture

### Agent role division

| Agent | role positioning | Draft content | output file |
|-------|---------|---------|---------|
| **architecture** (role) | Technical solution description expert | Architectural design, technical solutions, system processes, core parts of patent technology briefing documents | `agents/architecture/technical-description.md` |
| **deep** (lane) | Implementation details expert | Code implementation, algorithm details, performance optimization, quantification of technical effects, soft technical explanation | `agents/deep/implementation-details.md` |
| **writing** (lane) | User documentation writing expert | User manual, operating instructions, functional description, soft copy application abstract, claims | `agents/writing/user-documentation.md` |

### Collaboration process

**Phase 1: Reading handoffs and context** → **Phase 2: Concurrent independent drafting** → **Phase 3: Cross-review** → **Phase 4: Finalizing document draft**

### file system communication

```
<project_path>/.arc/arc:ip-draft/<project-name>/
├── context/
│ ├── doc-context.md (shared input)
│ └── handoff-input.json (arc:ip-check handover)
├── agents/
│   ├── architecture/
│ │ ├── technical-description.md (independently drafted)
│ │ └── review.md (review other Agents)
│   ├── deep/
│   │   ├── implementation-details.md
│   │   └── review.md
│   └── writing/
│       ├── user-documentation.md
│       └── review.md
├── convergence/
│ ├── terminology-alignment.md (terminology unification)
│ └── final-review.md (final review)
├── copyright/
│ ├── software-summary.md (software summary)
│ ├── manual-outline.md (operating manual)
│ └── source-code-package-notes.md (code material description)
├── patent/
│ ├── disclosure-draft.md (technical disclosure document)
│ ├── claims-draft.md (draft claims)
│ └── drawings-description.md (with drawing description)
└── reports/
└── doc-writing-log.md (writing log)
```

## Instructions

### Phase 1: Read review handoff and project context

**Step 1.1: Read the handover file**
1. Read `<audit_case_dir>/handoff/ip-drafting-input.json`.
2. extract:
   - Target asset list (IPA-001, IPA-002...)
   - Soft work/patent feasibility score
   - Priority recommendations
   - Risk warning
   - Key technical points and evidence paths
   - New fields: `format_compliance`, `program_product_recommended`, `fee_reduction`, `app_e_copyright`

**Step 1.2: Generate document context**
Use `ace-tool` to double check the key technology description and generate `context/doc-context.md`:
- Description of core technical solutions (architecture diagram, flow chart)
- Key algorithm implementation (code snippets + comments)
- Quantitative data on technical effects (performance indicators, test results)
- User function description (UI screenshot, operation process)
- Format/naming baseline: software full name/abbreviation/version, header and footer examples, screenshot naming requirements, code sample page number and line number requirements

**Step 1.3: Scaffolding generation**
```bash
python arc:ip-draft/scripts/scaffold_drafting_case.py \
  --project-path <project_path> \
  --project-name <project_name>
```

### Phase 2: Multi-Agent concurrent and independent drafting

**Start three Agents concurrently** (in the same message):

```typescript
// Architecture: Technical solution description
schedule_task(
  capability_profile="architecture",
  capabilities=["arc:ip-draft"],
  execution_mode="background",
description="Architecture drafting technical solution description",
  prompt=`
[TASK]: Draft technical solution description and the core part of the patent technology briefing document

[EXPECTED OUTCOME]:
- Generate agents/architecture/technical-description.md, including:
  1. Technical background and existing technical issues
  2. Overall architecture of technical solution (Mermaid diagram + text description)
  3. System process and data flow (flow chart + text description)
  4. Description of technological innovation points (compared to existing technology)
  5. Each description must cite code evidence (file:line)
  6. Table of three technical elements (technical problems/technical means/technical effects)
  7. Program product claim skeleton (method + system/device + computer program product + storage medium)
  8. OA possibility list (object/creative patchwork/out of scope)

[REQUIRED TOOLS]: ace-tool (code search), Read (read context/), Write (write agents/architecture/)

[MUST DO]:
- Read context/doc-context.md and context/handoff-input.json
- Use ace-tool to verify the accuracy of the technical solution description
- Technical terminology must be unified (establish a glossary)
- Architecture diagrams must be in Mermaid format
- Each technological innovation point must be compared with existing technology (citing external references in handoff)
- The technical description must be oriented to the patent examiner (clear, complete, and implementable)
- Annotation program product feasibility and claim combination suggestions

[MUST NOT DO]:
- Don’t make up technical details
- Do not use vague expressions ("maybe", "probably", "generally")
- The output of other agents must not be read at this stage (cross-review only in Phase 3)
- Do not confuse the description style of software and patents

[CONTEXT]: Project path <project_path>, working directory .arc/arc:ip-draft/<project-name>/
`
)

// Deep: Implementation details and technical effects
schedule_task(
  capability_profile="deep",
  capabilities=["arc:ip-draft"],
  execution_mode="background",
description="Deep drafting implementation details and technical effects",
  prompt=`
[TASK]: Draft code implementation details, algorithm description and technical effect quantification

[EXPECTED OUTCOME]:
- Generate agents/deep/implementation-details.md, including:
  1. Core algorithm implementation (pseudocode + key code snippets)
  2. Data structure design (class diagram + field description)
  3. Performance optimization measures (specific implementation + effect comparison)
  4. Quantification of technical effects (performance indicators, test data, comparison tables)
  5. Description of soft copy code materials (recommended code sections to submit)
  6. List of code samples that can be submitted (file + starting and ending lines, estimated number of pages, desensitization requirements)
  7. Performance/comparison data table template, if missing, a list of "indicators that need to be supplemented" will be output.

[REQUIRED TOOLS]: ace-tool (code search), Read (read context/), Write (write agents/deep/)

[MUST DO]:
- Read context/doc-context.md and context/handoff-input.json
- Use ace-tool to extract key code fragments (lines 3-20)
- Algorithm description must be implementable (pseudocode + comments)
- Technical effects must be quantified (specific values ​​+ comparison benchmarks)
- Code snippets must be desensitized (sensitive information removed)
- Each implementation detail must reference a specific file path
- Generate an estimate of the number of submitted code pages and annotate the first and last lines of the sample
- If performance/comparison data is missing, list the indicators and benchmarks that need to be supplemented.

[MUST NOT DO]:
- Don’t fabricate performance data
- Do not disclose trade secrets or sensitive information
- No other Agent output may be read at this stage
- The complete source code may not be used (only key snippets)

[CONTEXT]: Project path <project_path>, working directory .arc/arc:ip-draft/<project-name>/
`
)

// Writing: User documentation and function description
schedule_task(
  capability_profile="writing",
  capabilities=["arc:ip-draft"],
  execution_mode="background",
description="Writing drafting user documentation and function description",
  prompt=`
[TASK]: Drafting user manuals, operating instructions and software application abstracts

[EXPECTED OUTCOME]:
- Generate agents/writing/user-documentation.md, including:
  1. Overview of software functions (list of user-oriented functions)
  2. Outline of operating instructions (installation, configuration, use, troubleshooting)
  3. Abstract of soft copy application (300-500 words, for Copyright Office examiners)
  4. Draft patent claims (independent claims + dependent claims)
  5. User interface description (UI screenshot + operation process)
  6. Header/footer/naming consistency check hints (referencing the format baseline)
  7. Signature page, non-professional development guarantee, open source statement placeholder reminder
  8. Four-piece set of claims (method + system/device + computer program product + storage medium) sentence placeholder

[REQUIRED TOOLS]: ace-tool (search documents), Read (read context/), Write (write agents/writing/)

[MUST DO]:
- Read context/doc-context.md and context/handoff-input.json
- Use ace-tool to search project documentation (README/docs/)
- Function description must be oriented to non-technical users (clear and understandable)
- The operating instructions must be complete (covering the entire process from installation to use)
- The abstract of the soft work must comply with the format requirements of the Copyright Office
- Claims must comply with the patent law format (one sentence, clear hierarchy)
- Verify the consistency of software name and version and mark it in the document
- Insert screenshot placeholder description (must be consistent with the software name)
- Mark signature page/guarantee/open source statement to be added

[MUST NOT DO]:
- No technical jargon (for users)
- No fabricated functions are allowed (must be supported by code)
- No other Agent output may be read at this stage
- Do not confuse soft copy abstracts with patent abstracts

[CONTEXT]: Project path <project_path>, working directory .arc/arc:ip-draft/<project-name>/
`
)
```

**Wait for the three Agents to complete** and use `collect_task_output(task_id="...")` to collect the results.

### Phase 3: Cross-review

**Mandatory review mechanism** (each Agent must review the other two Agents):

```typescript
// Architecture reviews Deep and Writing
schedule_task(
task_ref="<architecture_task_ref>", // Reuse Phase 2 session
  capabilities=["arc:ip-draft"],
  execution_mode="foreground",
description="Architecture cross-review Deep and Writing",
  prompt=`
[TASK]: Review Deep and Writing documents to ensure technical accuracy and terminology consistency

[EXPECTED OUTCOME]:
- Generate agents/architecture/review.md, including:
  1. Review of Deep implementation details (technical accuracy, architectural consistency)
  2. Review of Writing user documentation (functional description accuracy, terminology consistency)
  3. Terminology conflict list (different names for the same object)
  4. Technical description error list (descriptions that do not match the code)
  5. Suggestions for correction (specific modification plan)
  6. Inspection of the correspondence between the completeness of program product claims and the three technical elements
  7. Format compliance (number of lines/headers and footers) and naming consistency tips

[MUST DO]:
- Read agents/deep/implementation-details.md and agents/writing/user-documentation.md
- Use ace-tool to verify dispute technical descriptions
- Establish a terminology comparison table (unified title)
- Point out inconsistencies between the technical description and the code
- Make specific suggestions for correction (not simply point out the problem)
- Review format/naming compliance and program product claim coverage

[MUST NOT DO]:
- Don’t simply agree with the other person’s point of view
- No review without evidence

[CONTEXT]: working directory .arc/arc:ip-draft/<project-name>/
`
)

// Deep reviews Architecture and Writing (same reason)
// Writing review Architecture and Deep (same reason)
```

**Collect review results**, generate `convergence/terminology-alignment.md` and `convergence/final-review.md`.

### Phase 4: Finalizing the draft document

**Step 4.1: Terminology unification**

Based on the review report, `convergence/terminology-alignment.md` is generated:

| object | Architecture title | Deep title | Writing title | unified terminology |
|------|-----------|---------|----------|---------|
| Core algorithm | Intelligent scheduling algorithm | task allocation algorithm | Automatic scheduling function | Intelligent task scheduling algorithm |

**Step 4.2: Generate soft copy document** (such as `target_docs` contains `copyright`)

Use a script to generate standardized documentation:
```bash
python arc:ip-draft/scripts/render_ip_documents.py \
  --case-dir .arc/arc:ip-draft/<project-name>/ \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs copyright
```

Generate files:
- `copyright/software-summary.md` (software application abstract, integrated Writing drafting + Architecture/Deep review)
- `copyright/manual-outline.md` (outline of operating instructions, integrated writing drafting + terminology unification)
- `copyright/source-code-package-notes.md` (code material description, integrated Deep drafting + Architecture review)
  - The full name/abbreviation/version of the software needs to be explicitly recorded, and the name consistency has been checked in the writing log.

**Step 4.3: Generate patent document** (such as `target_docs` contains `patent`)

```bash
python arc:ip-draft/scripts/render_ip_documents.py \
  --case-dir .arc/arc:ip-draft/<project-name>/ \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs patent
```

Generate files:
- `patent/disclosure-draft.md` (Technical communication document, integration of Architecture technical solutions + Deep implementation details + terminology unification)
- `patent/claims-draft.md` (draft claims, integrated writing drafting + Architecture/Deep review)
  - Default includes: 1 independent method + 1 system/device + 1 computer program product + 1 storage medium; dependent claims cite differences in performance/data flow/module parameters
- `patent/drawings-description.md` (Chart description, integrated Architecture architecture diagram + Deep data flow diagram + Writing UI screenshot)

**Step 4.4: Generate writing log**

Generate `reports/doc-writing-log.md`:
- Input source (handoff file path, CLAUDE.md path)
- Assumptions and inferences (things not found in the code but reasonably inferred)
- Items to be added manually (information provided by the applicant is required)
- Agent collaboration records (drafting content, review comments, and terminology unified decision-making by each agent)
- Fee reduction materials list, electronic copyright options, format compliance check results, missing performance data/screenshot list

## Scripts

Prefer using scripts to generate standardized skeletons:

```bash
# Generate working directory skeleton
python arc:ip-draft/scripts/scaffold_drafting_case.py \
  --project-path <project_path> \
  --project-name <project_name>

# Render the final document
python arc:ip-draft/scripts/render_ip_documents.py \
  --case-dir <output_dir> \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs both
```

## Artifacts

Default output directory:`<project_path>/.arc/arc:ip-draft/<project-name>/`

- `context/doc-context.md` (document context)
- `context/handoff-input.json` (arc:ip-check handover)
- `agents/architecture/technical-description.md` (drafted independently by Architecture)
- `agents/architecture/review.md` (Architecture review)
- `agents/deep/implementation-details.md` (Drafted independently by Deep)
- `agents/deep/review.md` (Deep review)
- `agents/writing/user-documentation.md` (Writing independently drafted)
- `agents/writing/review.md` (Writing review)
- `convergence/terminology-alignment.md` (terminology unification)
- `convergence/final-review.md` (final review)
- `copyright/software-summary.md` (soft abstract)
- `copyright/manual-outline.md` (operating instructions)
- `copyright/source-code-package-notes.md` (code material description)
- `patent/disclosure-draft.md` (technical communication document)
- `patent/claims-draft.md` (draft claims)
- `patent/drawings-description.md` (picture description)
- `reports/doc-writing-log.md` (writing log)

## Quick Reference

| output | use |
|------|------|
| `disclosure-draft.md` | Patent technology disclosure draft |
| `claims-draft.md` | Draft claims |
| `software-summary.md` | Soft copy application summary |
| `manual-outline.md` | Soft operating instructions outline |
| `source-code-package-notes.md` | Soft code material description |
| `doc-writing-log.md` | Writing journal (record assumptions and items to be added) |

## Failure Recovery

- **Agent timeout (>10min)**: Ask the user whether to continue waiting or downgrade to dual-Agent drafting.
- **Agent Drafting Missing**: Fill in with two other Agents, marked "Dual Source Drafting".
- **Term conflict cannot be resolved**: List the conflicting terms in the document and mark "requires manual decision".
- **The handover document is missing**: Prompt the user to execute `arc:ip-check` first, or make a minimal draft (marked "Unreviewed Evaluation") when the user explicitly requests it.
