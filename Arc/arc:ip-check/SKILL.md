---
name: arc:ip-check
description: "知识产权可行性审查：评估软著/专利价值、风险与申报优先级；当用户说“专利评估/IP review/FTO 风险”时触发。"
---

# arc:ip-check — project patent/software review report

## Overview

`arc:ip-check` adopts the **Multi-Agent collaboration model** and evaluates the feasibility, risk and priority of applying for **software copyright** and **invention patent** for the project based on the real code and architectural context of the project.

**Core Competencies**:
- Three-Agent concurrent independent assessment (architecture architecture perspective/deep engineering perspective/writing document and compliance perspective)
- Cross-rebuttal mechanism eliminates blind spots and overly optimistic/pessimistic assessments
- Evidence-driven feasibility scoring and risk matrix
- Structured handover document output to `arc:ip-draft`

This skill does not directly produce formal application documents. The review conclusion will be output in a structured handover document for `arc:ip-draft` to continue completing document writing.

## Quick Contract

- **Trigger**: Prepare for pre-application assessment, due diligence or compliance review, and need to determine the feasibility of software copyright and patent filing.
- **Inputs**: Project path, application subject information, software name and business goals (optional).
- **Outputs**: Feasibility report, risk matrix, priority recommendations and `ip-drafting-input.json`.
- **Quality Gate**: Must pass the evidence binding and dual-track scoring check of `## Quality Gates` before output.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:ip-check` to do feasibility review and risk classification first, and then give application suggestions."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO FILING FEASIBILITY CLAIM WITHOUT CODE EVIDENCE AND RISK MATRIX
```

Without code evidence and risk matrix, no "applicable" conclusion should be given.

## Workflow

1. Summarize project context and historical review products to establish a review baseline.
2. Multiple agents can concurrently complete software-copyright and patent dual-track independent evaluation.
3. Perform cross-rebuttal, convergence scoring and key points of dispute.
4. Output feasibility report and `ip-drafting-input.json` handover document.

## Quality Gates

- Software copyright and patents must be scored separately, and conclusions cannot be mixed.
- Each risk must be labeled with probability, impact and mitigation recommendations.
- Conclusions must cite specific module/documentary evidence.
- When a scoring dimension lacks direct evidence, use `N/A` and explain the missing evidence instead of forcing a number.
- The handover file fields must be complete and can be consumed by `arc:ip-draft`.
- Feasibility scorecards, risk matrices, and priority summaries delivered in chat should prefer `terminal-table-output` when the content is compact and alignable.

## Expert Standards

- Patent review needs to separately evaluate `novelty/creativeness/practicability' and give a grade of evidence strength.
- Added `FTO` (freedom to implement) risk perspective, marking potential infringements and avoidance strategies.
- The evaluation of software copyright readiness requires attention to `original expression` and a provable author/version chain.
- Evidence first, then conclusion: Every innovation claim must be bound to code, process, documents or experimental records.
- The output should distinguish between technical assessment and legal conclusions, and it is prohibited to give deterministic legal commitments.

## Scripts & Commands

- Audit workspace scaffolding: `python3 Arc/arc:ip-check/scripts/scaffold_audit_case.py --project-path <project_path>`
- Report rendering: `python3 Arc/arc:ip-check/scripts/render_audit_report.py --case-dir <case_dir> --project-name <project_name>`
- Format compliance check: `python3 Arc/arc:ip-check/scripts/format_compliance_checker.py --project-path <project_path> --output <checklist.md>`
- Fee reduction eligibility check: `python3 Arc/arc:ip-check/scripts/fee_reduction_check.py --applicant-type enterprise --annual-income <amount>`
- Runtime main command: `arc ip-check`

## Red Flags

- Write the review opinions into legal conclusions or guarantee commitments.
- The evaluation dimensions of software copyright and patents are not distinguished.
- High-risk items are missing mitigation paths.
- There is no handover document but it is claimed that it can enter the paperwork stage.

## Mandatory Linkage (cannot be fought alone)

Must follow the link below to collaborate:

1. The `CLAUDE.md` index of the item produced by `arc:init` is read first.
2. If there is a `arc:audit` product, reuse its architectural depth and risk conclusions.
3. When the requirements are vague, call `arc:clarify` first to clarify product boundaries and business goals.
4. Highly controversial technical routes can be connected in series with `arc:decide` to make long Agent arguments.
5. The review output is handed over to `arc:ip-draft`, and writing is not allowed in this skill.

## When to Use

- **Primary Trigger**: It is necessary to evaluate the feasibility, risks and priorities of software copyright and patent filing before applying.
- **Typical Scenario**: Financing, bidding, and intellectual property due diligence before listing.
- **Boundary Note**: Please refer to `arc:ip-draft` for drafting formal application documents.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the target project root directory |
| `project_name` | string | no | Project name; deduced from path by default |
| `software_name` | string | no | Apply for a software name; if not provided, use a temporary name and mark it in the report |
| `applicant_type` | enum | no | `individual` / `enterprise` / `institution`, default `enterprise` |
| `business_goal` | string | no | Product business goals (used to evaluate application priority) |
| `output_dir` | string | no | Default `<project_path>/.arc/ip-check/<project-name>/` |

## Dependencies

* **Organization Contract**: Required. Following `docs/orchestration-contract.md`, scheduling is implemented through the runtime adaptation layer.
- **ace-tool MCP** (required): Search project code and implementation evidence.
- **Exa MCP** (recommended): Supplement existing technology/policy basis.
- **Scheduling API** (required): Dispatch `architecture` / `deep` / `writing` three Agent collaboration.
- **arc:init** (strongly recommended): Read the `CLAUDE.md` level index.
- **arc:audit** (optional): Reuse existing review report.

## Context Priority (mandatory)

Strictly follow `.arc/context-priority-protocol.md`:

1. `.arc/ip-check/<project>/context/project-ip-snapshot.md`(24h)
2. `.arc/audit/<project>/` already has a review report
3. Project `CLAUDE.md` Hierarchy Index (7 days)
4. `ace-tool` Source code semantic scanning
5. `Exa` external reference

If the first three levels of information are insufficient, the scan must be downgraded and "context source and freshness" must be noted in the report.

## Critical Rules

1. **Read-only review**: User source code may not be modified.
2. **Evidence-driven**: Each conclusion must be bound to a file path or module evidence.
3. **Dual-track independent scoring**: Software-copyright feasibility and patent feasibility are scored separately; use bounded scores or `N/A` rather than forcing unsupported precision.
4. **Risk Explicit**: Rejectable risks, corrective risks, time risks and material gaps must be listed.
5. **Handover standardization**: `handoff/ip-drafting-input.json` must be generated to `arc:ip-draft`.
6. **Legal Boundary**: The output is engineering and process suggestions and does not replace the legal advice of a practicing lawyer/patent agent.
7. **Multi-Agent collaboration**: Architecture/deep/writing three-Agent concurrent analysis + cross-refutation must be used.

## Multi-Agent Architecture

### Agent role division

| Agent | role positioning | Assessment Dimensions | output file |
|-------|---------|---------|---------|
| **architecture** (role) | Architecture and Innovation Expert | Originality of technical solutions, novelty of architectural design, differentiation of existing technologies, and feasibility of patent application | `agents/architecture/innovation-analysis.md` |
| **deep** (lane) | Engineering implementation expert | Code integrity, implementation adequacy, quantifiable technical effects, feasibility of software application | `agents/deep/implementation-analysis.md` |
| **writing** (lane) | Documentation and Compliance Analysis Expert | Document completeness, material readiness, application process risks, intellectual property compliance | `agents/writing/compliance-analysis.md` |

### Collaboration process

**Phase 1: Contextual Collection** → **Phase 2: Concurrent Independent Assessment** → **Phase 3: Cross-Rebuttal** → **Phase 4: Comprehensive Feasibility Report**

### file system communication

```
<project_path>/.arc/ip-check/<project-name>/
├── context/
│ ├── project-ip-snapshot.md (shared input)
│ └── external-references.md (Exa search results)
├── agents/
│   ├── architecture/
│ │ ├── innovation-analysis.md (independent evaluation)
│ │ └── critique.md (refute other Agents)
│   ├── deep/
│   │   ├── implementation-analysis.md
│   │   └── critique.md
│   └── writing/
│       ├── compliance-analysis.md
│       └── critique.md
├── convergence/
│ ├── round-1-summary.md (first round summary)
│ └── final-consensus.md (final consensus)
├── analysis/
│ └── ip-assets.md (asset list)
├── reports/
│ ├── ip-feasibility-report.md (feasibility summary report)
│ └── filing-readiness-checklist.md (readiness checklist)
└── handoff/
└── ip-drafting-input.json (handover to arc:ip-draft)
```

## Instructions

This section has been moved to a reference file to reduce context bloat.

👉 **Please see [Detailed Execution Instructions](references/execution-instructions.md) for the full details.**

## Scripts

Prefer using scripts to generate standardized skeletons:

```bash
# Generate working directory skeleton
python Arc/arc:ip-check/scripts/scaffold_audit_case.py \
  --project-path <project_path> \
  --project-name <project_name>

# Render final report
python Arc/arc:ip-check/scripts/render_audit_report.py \
  --case-dir <output_dir> \
  --project-name <name>

# Fee reduction eligibility check (output to analysis/fee-reduction-assessment.md)
python Arc/arc:ip-check/scripts/fee_reduction_check.py \
  --applicant-type <individual|enterprise|institution> \
  --annual-income <number> \
  --output <path-to-analysis/fee-reduction-assessment.md>

# Format compliance check (output to convergence/format-compliance.md)
python Arc/arc:ip-check/scripts/format_compliance_checker.py \
  --project-path <project_path> \
  --output <path-to-convergence/format-compliance.md>
```

## Artifacts

Default output directory:`<project_path>/.arc/ip-check/<project-name>/`

- `context/project-ip-snapshot.md` (project snapshot)
- `context/external-references.md` (external reference)
- `agents/architecture/innovation-analysis.md` (Architecture independent evaluation)
- `agents/architecture/critique.md` (Architecture rebuttal)
- `agents/deep/implementation-analysis.md` (Deep independent evaluation)
- `agents/deep/critique.md` (Deep rebuttal)
- `agents/writing/compliance-analysis.md` (Writing independent evaluation)
- `agents/writing/critique.md` (Writing rebuttal)
- `convergence/round-1-summary.md` (first round comprehensive)
- `convergence/final-consensus.md` (final consensus)
- `convergence/format-compliance.md` (format/naming compliance evaluation)
- `convergence/tech-elements-map.md` (technical three elements/program product mapping)
- `analysis/ip-assets.md` (asset list)
- `analysis/fee-reduction-assessment.md` (fee reduction qualification assessment)
- `reports/ip-feasibility-report.md` (feasibility summary report)
- `reports/filing-readiness-checklist.md` (Readiness Checklist)
- `handoff/ip-drafting-input.json` (handover to arc:ip-draft)

## Quick Reference

| output | use |
|------|------|
| `ip-feasibility-report.md` | Application feasibility report |
| `filing-readiness-checklist.md` | Material gap inspection before submission |
| `ip-drafting-input.json` | Structured input handed over to `arc:ip-draft` |
| `ip-assets.md` | Intellectual Property Asset List |

## Failure Recovery

- **Agent timeout (>10min)**: Ask the user whether to continue waiting or downgrade to dual-Agent analysis.
- **Agent analysis is missing**: Use other two agents to fill it in and mark "dual source analysis".
- **MCP not available**: downgrade to Grep + Read direct scan.
- **Conflict cannot be resolved**: List the dispute points in the report and mark "requires manual decision".
