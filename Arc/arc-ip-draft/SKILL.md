---
name: arc-ip-draft
description: "知识产权申请文档起草：基于审查结论生成软著/专利草稿；当用户说“撰写专利/起草软著材料/draft IP filing”时触发。"
---

# arc-ip-draft — patent/software document writing

## Overview

`arc-ip-draft` adopts the **multi-Agent collaboration model** to focus on document writing and does not make feasibility decisions. It consumes the review results of `arc-ip-check` and outputs an editable application document draft combined with the project code context.

**Core Competencies**:
- Three Agents draft concurrently and independently (architecture technical solution/deep implementation details/writing user documentation)
- Cross-review mechanism ensures terminology consistency and technical accuracy
- Evidence-driven documentation writing (each technical description can be traced back to the code)
- Structured draft output (software/patent separate output)

## Quick Contract

- **Trigger**: The review handover is ready and a draft of software-copyright and/or patent application materials needs to be generated.
- **Inputs**: Project path, `arc-ip-check` handover directory, target document scope and application subject information.
- **Outputs**: Requested track drafts (`copyright/`, `patent/`, or both according to `target_docs`), term alignment results, and writing logs.
- **Quality Gate**: The evidence traceability and terminology consistency check of `## Quality Gates` must be passed before delivery.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc-ip-draft` to read the review handover first, and then draft the software and patent documents separately."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO IP DRAFTING WITHOUT AUDIT HANDOFF OR TRACEABLE CODE EVIDENCE
```

High-confidence technical document drafts should not be output without review handoff or traceability evidence.

## Workflow

1. Read the `arc-ip-check` handover file and establish the writing context.
2. Multi-Agent and initiate draft technical solutions, implementation details and documentation chapters.
3. Cross-review and harmonize terminology, boundaries and evidence references.
4. Output the requested software-copyright and/or patent drafts plus writing logs in separate sub-directories.

## Quality Gates

- Each key technical description should be traceable back to code or review handoff.
- Requested copyright and patent materials must stay separated by track when they are both produced.
- The glossary must be consistent throughout the text.
- The draft must be marked with an "editable draft" position to avoid being misled into the final legal text.

## Expert Standards

- The patent draft must follow the structure of `Abstract-Background-Content of the Invention-Description of Drawings-Detailed Embodiments-Claims'.
- Claims must adopt a `wide-medium-narrow` gradient layout, and check the basic consistency of previous and later references.
- The software-copyright materials must cover functional description, architecture description, key source code evidence and version information consistency.
- Full-text implementation of `Term Control`: single naming of the same object to avoid claim scope drift.
- The delivery must be accompanied by a "list to be reviewed by legal affairs", which clearly identifies high-risk statements that need to be confirmed by a lawyer.

## Scripts & Commands

- Drafting workspace scaffolding: `python3 Arc/arc-ip-draft/scripts/scaffold_drafting_case.py --project-path <project_path>`
- Document rendering (dual track): `python3 Arc/arc-ip-draft/scripts/render_ip_documents.py --case-dir <case_dir> --target-docs both`
- Document rendering (patent only): `python3 Arc/arc-ip-draft/scripts/render_ip_documents.py --case-dir <case_dir> --target-docs patent`
- Document rendering (software only): `python3 Arc/arc-ip-draft/scripts/render_ip_documents.py --case-dir <case_dir> --target-docs copyright`
- Runtime main command: `arc ip-draft`

## Red Flags

- Skip the handover documents and go straight to speculating on the technical details.
- Mixing software-copyright and patent content leads to structural confusion.
- Term drift, multiple names for the same object.
- Outputs a conclusive conclusion without citing evidence.

## Mandatory Linkage (cannot be fought alone)

1. By default, `arc-ip-check` is read first and the product: `handoff/ip-drafting-input.json` is read.
2. If the handover document is missing, priority will be given to executing `arc-ip-check`; only a minimal draft will be made when explicitly requested by the user.
3. Document content must be linked back to the project context: `CLAUDE.md` + `ace-tool` evidence.
4. For terminological conflicts or unclear technical routes, connect `arc-decide` in series to make corrections before finalizing.

## When to Use

- **Primary Trigger**: `arc-ip-check` has been handed over, and an editable draft of application materials needs to be generated.
- **Typical Scenario**: Writing software-copyright descriptions, patent disclosure documents, claims, and accompanying drawing descriptions.
- **Boundary Note**: If the feasibility review has not been completed, execute `arc-ip-check` first.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the target project root directory |
| `project_name` | string | no | Deduced from path by default |
| `audit_case_dir` | string | no | `arc-ip-check` Output directory; default `<project_path>/.arc/ip-check/<project-name>/` |
| `software_name` | string | no | Software name, if not provided, it will be read from the handover file. |
| `applicant_name` | string | no | Application subject name |
| `target_docs` | enum | no | `copyright` / `patent` / `both`, default `both` |
| `output_dir` | string | no | Default `<project_path>/.arc/ip-draft/<project-name>/` |

## Dependencies

* **Organization Contract**: Required. Following `docs/orchestration-contract.md`, scheduling is implemented through the runtime adaptation layer.
- **arc-ip-check** (strongly recommended): as the main input.
- **ace-tool MCP** (required): Correct technical details and code evidence in the documentation.
- **arc-init** (recommended): Reuse module index to reduce repeated scanning.
- **Scheduling API** (required): Dispatch `architecture` / `deep` / `writing` three Agent collaboration.

## Context Priority (mandatory)

1. `audit_case_dir/handoff/ip-drafting-input.json`
2. `.arc/ip-draft/<project>/context/doc-context.md`
3. Project `CLAUDE.md`
4. `ace-tool` Source code scan

## Critical Rules

1. **Don't make it up**: Any technical details must be traceable back to code or review handoff information.
2. **Consistent terminology**: The same object is called uniformly throughout the text, and synonyms are not allowed to be freely substituted.
3. **Complete structure**: Each requested track must be output completely according to its template chapters.
4. **Draft Positioning**: The output is an "editable application draft" and may not be claimed to be the final legal text.
5. **Double-track splitting**: Software-copyright materials and patent materials are produced in separate catalogs, without mixing.
6. **Multi-Agent collaboration**: The default path uses architecture/deep/writing three agents plus cross-review; only the explicitly requested minimal-draft fallback may downgrade this path, and it must be labeled accordingly.

## Multi-Agent Architecture

### Agent role division

| Agent | role positioning | Draft content | output file |
|-------|---------|---------|---------|
| **architecture** (role) | Technical solution description expert | Architectural design, technical solutions, system processes, core parts of patent technology briefing documents | `agents/architecture/technical-description.md` |
| **deep** (lane) | Implementation details expert | Code implementation, algorithm details, performance optimization, quantification of technical effects, software technical explanation | `agents/deep/implementation-details.md` |
| **writing** (lane) | User documentation writing expert | User manual, operating instructions, functional description, software-copyright application abstract, claims | `agents/writing/user-documentation.md` |

### Collaboration process

**Phase 1: Reading handoffs and context** → **Phase 2: Concurrent independent drafting** → **Phase 3: Cross-review** → **Phase 4: Finalizing document draft**

### file system communication

```
<project_path>/.arc/ip-draft/<project-name>/
├── context/
│ ├── doc-context.md (shared input)
│ └── handoff-input.json (arc-ip-check handover)
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
├── copyright/                  # Optional: when `target_docs` includes `copyright`
│ ├── software-summary.md (software summary)
│ ├── manual-outline.md (operating manual)
│ └── source-code-package-notes.md (code material description)
├── patent/                     # Optional: when `target_docs` includes `patent`
│ ├── disclosure-draft.md (technical disclosure document)
│ ├── claims-draft.md (draft claims)
│ └── drawings-description.md (with drawing description)
└── reports/
└── doc-writing-log.md (writing log)
```

## Instructions

This section has been moved to a reference file to reduce context bloat.

👉 **Please see [Detailed Execution Instructions](references/execution-instructions.md) for the full details.**

## Scripts

Prefer using scripts to generate standardized skeletons:

```bash
# Generate working directory skeleton
python Arc/arc-ip-draft/scripts/scaffold_drafting_case.py \
  --project-path <project_path> \
  --project-name <project_name>

# Render the final document
python Arc/arc-ip-draft/scripts/render_ip_documents.py \
  --case-dir <output_dir> \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs both
```

## Artifacts

Default output directory:`<project_path>/.arc/ip-draft/<project-name>/`

- `context/doc-context.md` (document context)
- `context/handoff-input.json` (arc-ip-check handover)
- `agents/architecture/technical-description.md` (drafted independently by Architecture)
- `agents/architecture/review.md` (Architecture review)
- `agents/deep/implementation-details.md` (Drafted independently by Deep)
- `agents/deep/review.md` (Deep review)
- `agents/writing/user-documentation.md` (Writing independently drafted)
- `agents/writing/review.md` (Writing review)
- `convergence/terminology-alignment.md` (terminology unification)
- `convergence/final-review.md` (final review)
- `copyright/software-summary.md` (software-copyright abstract; only when `target_docs` includes `copyright`)
- `copyright/manual-outline.md` (operating instructions; only when `target_docs` includes `copyright`)
- `copyright/source-code-package-notes.md` (software-copyright code material description; only when `target_docs` includes `copyright`)
- `patent/disclosure-draft.md` (technical communication document; only when `target_docs` includes `patent`)
- `patent/claims-draft.md` (draft claims; only when `target_docs` includes `patent`)
- `patent/drawings-description.md` (picture description; only when `target_docs` includes `patent`)
- `reports/doc-writing-log.md` (writing log)
- `reports/submission-guide.md` (保姆级实操提交指南)

## Quick Reference

| output | use |
|------|------|
| `disclosure-draft.md` | Patent technology disclosure draft |
| `claims-draft.md` | Draft claims |
| `software-summary.md` | Software-copyright application summary |
| `manual-outline.md` | Software operating instructions outline |
| `source-code-package-notes.md` | Software-copyright code material description |
| `doc-writing-log.md` | Writing journal (record assumptions and items to be added) |
| `submission-guide.md` | 保姆级实操提交指南（含命名、前后端拆分、提交顺序及避坑）|

## Failure Recovery

- **Agent timeout (>10min)**: Ask the user whether to continue waiting or downgrade to dual-Agent drafting.
- **Agent Drafting Missing**: Fill in with two other Agents, marked "Dual Source Drafting".
- **Term conflict cannot be resolved**: List the conflicting terms in the document and mark "requires manual decision".
- **The handover document is missing**: Prompt the user to execute `arc-ip-check` first, or make a minimal draft (marked "Unreviewed Evaluation") when the user explicitly requests it.
