---
name: "arc:ip-check"
description: "准备申请软著/专利前使用：评估可行性、风险、优先级和证据充分性。"
---

# arc:ip-check — project patent/software review report

## Overview

`arc:ip-check` adopts the **Multi-Agent collaboration model** and evaluates the feasibility, risk and priority of applying for **software copyright** and **invention patent** for the project based on the real code and architectural context of the project.

**Core Competencies**:
- Three-Agent concurrent independent assessment (oracle architecture perspective/deep engineering perspective/writing document and compliance perspective)
- Cross-rebuttal mechanism eliminates blind spots and overly optimistic/pessimistic assessments
- Evidence-driven feasibility scoring and risk matrix
- Structured handover document output to `arc:ip-draft`

This skill does not directly produce formal application documents. The review conclusion will be output in a structured handover document for `arc:ip-draft` to continue completing document writing.

## Quick Contract

- **Trigger**: Prepare for pre-application assessment, due diligence or compliance review, and need to determine the feasibility of the soft copy/patent.
- **Inputs**: Project path, application subject information, software name and business goals (optional).
- **Outputs**: Feasibility report, risk matrix, priority recommendations and `ip-drafting-input.json`.
- **Quality Gate**: Must pass the evidence binding and dual-track scoring check of `## Quality Gates` before output.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:ip-check` to do feasibility review and risk classification first, and then give application suggestions."

## The Iron Law

```
NO FILING FEASIBILITY CLAIM WITHOUT CODE EVIDENCE AND RISK MATRIX
```

Without code evidence and risk matrix, no "applicable" conclusion should be given.

## Workflow

1. Summarize project context and historical review products to establish a review baseline.
2. Multiple agents can concurrently complete soft/patent dual-track independent evaluation.
3. Perform cross-rebuttal, convergence scoring and key points of dispute.
4. Output feasibility report and `ip-drafting-input.json` handover document.

## Quality Gates

- Soft works and patents must be scored separately, and conclusions cannot be mixed.
- Each risk must be labeled with probability, impact and mitigation recommendations.
- Conclusions must cite specific module/documentary evidence.
- The handover file fields must be complete and can be consumed by `arc:ip-draft`.

## Red Flags

- Write the review opinions into legal conclusions or guarantee commitments.
- The evaluation dimensions of soft works and patents are not distinguished.
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

- **首选触发**: It is necessary to evaluate the feasibility, risks and priorities of soft works/patents before applying.
- **典型场景**: Financing, bidding, and intellectual property due diligence before listing.
- **边界提示**: Please refer to `arc:ip-draft` for drafting formal application documents.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the target project root directory |
| `project_name` | string | no | Project name; deduced from path by default |
| `software_name` | string | no | Apply for a software name; if not provided, use a temporary name and mark it in the report |
| `applicant_type` | enum | no | `individual` / `enterprise` / `institution`, default `enterprise` |
| `business_goal` | string | no | Product business goals (used to evaluate application priority) |
| `output_dir` | string | no | Default `<project_path>/.arc/ip-audit/<project-name>/` |

## Dependencies

* **Organization Contract**: Required. Following `docs/orchestration-contract.md`, scheduling is implemented through the runtime adaptation layer.
- **ace-tool MCP** (required): Search project code and implementation evidence.
- **Exa MCP** (recommended): Supplement existing technology/policy basis.
- **Dispatch API** (required): Dispatch `oracle` / `deep` / `writing` three Agent collaboration.
- **arc:init** (strongly recommended): Read the `CLAUDE.md` level index.
- **arc:audit** (optional): Reuse existing review report.

## Context Priority (mandatory)

Strictly follow `.arc/context-priority-protocol.md`:

1. `.arc/ip-audit/<project>/context/project-ip-snapshot.md`(24h)
2. `.arc/review/<project>/` already has a review report
3. Project `CLAUDE.md` Hierarchy Index (7 days)
4. `ace-tool` Source code semantic scanning
5. `Exa` external reference

If the first three levels of information are insufficient, the scan must be downgraded and "context source and freshness" must be noted in the report.

## Critical Rules

1. **Read-only review**: User source code may not be modified.
2. **Evidence-driven**: Each conclusion must be bound to a file path or module evidence.
3. **Dual-track independent scoring**: Soft copy feasibility and patent feasibility are scored separately and are not allowed to be mixed into a single conclusion.
4. **Risk Explicit**: Rejectable risks, corrective risks, time risks and material gaps must be listed.
5. **Handover standardization**: `handoff/ip-drafting-input.json` must be generated to `arc:ip-draft`.
6. **Legal Boundary**: The output is engineering and process suggestions and does not replace the legal advice of a practicing lawyer/patent agent.
7. **Multi-Agent collaboration**: Oracle/deep/writing three-Agent concurrent analysis + cross-refutation must be used.

## Multi-Agent Architecture

### Agent role division

| Agent | role positioning | Assessment Dimensions | output file |
|-------|---------|---------|---------|
| **oracle** (role) | Architecture and Innovation Expert | Originality of technical solutions, novelty of architectural design, differentiation of existing technologies, and feasibility of patent application | `agents/oracle/innovation-analysis.md` |
| **deep** (lane) | Engineering implementation expert | Code integrity, implementation adequacy, quantifiable technical effects, feasibility of software application | `agents/deep/implementation-analysis.md` |
| **writing** (lane) | Documentation and Compliance Analysis Expert | Document completeness, material readiness, application process risks, intellectual property compliance | `agents/writing/compliance-analysis.md` |

### Collaboration process

**Phase 1: Contextual Collection** → **Phase 2: Concurrent Independent Assessment** → **Phase 3: Cross-Rebuttal** → **Phase 4: Comprehensive Feasibility Report**

### file system communication

```
<project_path>/.arc/ip-audit/<project-name>/
├── context/
│ ├── project-ip-snapshot.md (shared input)
│ └── external-references.md (Exa search results)
├── agents/
│   ├── oracle/
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

### Phase 1: Context collection

**Step 1.1: Check cache and index**
1. Check if `.arc/ip-audit/<project>/context/project-ip-snapshot.md` exists and is fresh (<24h).
2. Read the project `CLAUDE.md` level index (root level + core module level).
3. If there is a `.arc/review/<project>/` review report, extract the architecture/technical debt/security conclusions.

**Step 1.2: Generate project snapshot**
Use `ace-tool` to search for the following and generate `context/project-ip-snapshot.md`:
- Core algorithm and performance optimization implementation
- Key module boundaries and system interaction
- Code snippets that can be submitted as soft copy samples
- Technical solution description (architecture diagram, flow chart, data flow)
- Comparison of existing technologies (if included in project documents)
- Format/naming consistency baseline: software full name/abbreviation/version, header and footer examples, screenshot naming consistency check entry (for writing use)

**Step 1.3: External reference search**
Use `Exa` to search and generate `context/external-references.md`:
- Patent search for similar products (keywords: project core technology + field)
- Soft publication application policy and review standards
- Patent application thresholds and common reasons for rejection
- Policy anchors (marked with date/source): Paperless real-name rules, code/description format requirements, App electronic copyright process, 2024 "computer program product" terms, 2025 fee reduction threshold and ratio

**Step 1.4: Scaffolding generation**
```bash
python arc:ip-check/scripts/scaffold_audit_case.py \
  --project-path <project_path> \
  --project-name <project_name>
```

### Phase 2: Multi-Agent concurrent independent evaluation

**Start three Agents concurrently** (in the same message):

```typescript
// Oracle: Architecture and Innovation Assessment
dispatch_job(
  role="oracle",
  capabilities=["arc:ip-check"],
  execution_mode="background",
description="Oracle evaluates technological innovation and patent feasibility",
  prompt=`
[TASK]: Evaluate the technological innovation and patent application feasibility of the project

[EXPECTED OUTCOME]:
- Generate agents/oracle/innovation-analysis.md, including:
  1. Technical solution originality score (1-10 points)
  2. Architectural design novelty score (1-10 points)
  3. Difference analysis of existing technologies (reference context/external-references.md)
  4. Patent application feasibility score (high/medium/low)
  5. Each rating must be accompanied by file path evidence
  6. Mapping table of three elements of technology (technical issues/technical means/technical effects)
  7. Determination of the patentability of program products (yes/no + basis)
  8. Suggested claim combination (method + system/device + computer program product + storage medium)

[REQUIRED TOOLS]: ace-tool (code search), Read (read context/), Write (write agents/oracle/)

[MUST DO]:
- Read context/project-ip-snapshot.md and context/external-references.md
- Use ace-tool to deeply analyze the core algorithm implementation
- Compare existing technologies and identify technical differences
- Each innovation point must reference a specific file path (file:line)
- Ratings must be quantitatively based
- Mark potential OA risks (object/creativity/out-of-scope) and cite policy anchors

[MUST NOT DO]:
- Project source code must not be modified
- No legal advice is allowed (project evaluation only)
- Do not confuse software and patent evaluation criteria
- The output of other Agents must not be read at this stage (cross-rebuttal only in Phase 3)

[CONTEXT]: Project path <project_path>, working directory .arc/ip-audit/<project-name>/
`
)

// Deep: Project implementation evaluation
dispatch_job(
  lane="deep",
  capabilities=["arc:ip-check"],
  execution_mode="background",
description="Deep evaluates code integrity and software feasibility",
  prompt=`
[TASK]: Evaluate the code integrity of the project and the feasibility of software application

[EXPECTED OUTCOME]:
- Generate agents/deep/implementation-analysis.md, including:
  1. Code integrity score (1-10 points)
  2. Implementation adequacy score (1-10 points)
  3. Technical effect quantifiable score (1-10 points)
  4. Soft copy application feasibility score (high/medium/low)
  5. Each rating must be accompanied by code evidence
  6. Software authors can submit a code sample list (file + starting and ending lines, estimated number of pages that meet ≥50 lines/page, desensitization/deletion of comments suggestions)
  7. Technical effect quantification data table (including benchmarks/comparisons, if missing, mark the indicators that need to be supplemented)

[REQUIRED TOOLS]: ace-tool (code search), Read (read context/), Write (write agents/deep/)

[MUST DO]:
- Read context/project-ip-snapshot.md
- Use ace-tool to analyze code structural integrity
- Evaluate the adequacy of core function implementation (whether there are TODO/FIXME/not implemented)
- Check whether the technical effect is quantifiable (performance indicators, test coverage)
- Identify code sections that can be submitted as soft copies (3000-5000 lines recommended)
- Each conclusion must reference a specific file path
- Record the file/line number of the first and last sample pages, and prompt whether desensitization/deletion of comments is required
- If there is a lack of performance/comparison data, output the "supplementary test indicators" list

[MUST NOT DO]:
- Project source code must not be modified
- No legal advice may be given
- Do not confuse software and patent evaluation criteria
- No other Agent output may be read at this stage

[CONTEXT]: Project path <project_path>, working directory .arc/ip-audit/<project-name>/
`
)

// Writing: Documentation and Compliance Analysis
dispatch_job(
  lane="writing",
  capabilities=["arc:ip-check"],
  execution_mode="background",
description="Writing evaluates document completeness and application readiness",
  prompt=`
[TASK]: Evaluate the project’s document completeness and readiness for intellectual property application

[EXPECTED OUTCOME]:
- Generate agents/writing/compliance-analysis.md, including:
  1. Document completeness score (1-10 points)
  2. Material readiness score (1-10 points)
  3. Application process risk assessment (high/medium/low)
  4. Intellectual property compliance check (open source protocol conflicts, third-party dependencies)
  5. Each rating must be accompanied by evidence
  6. Naming/real name consistency, header and footer format check results
  7. Signature page, non-professional development guarantee, and open source statement readiness status
  8. Feasibility of App electronic copyright channel (if the target is to put it on the application market)
  9. Pre-evaluation of fee reduction eligibility and list of required supporting documents

[REQUIRED TOOLS]: ace-tool (search documents), Read (read context/), Write (write agents/writing/)

[MUST DO]:
- Read context/project-ip-snapshot.md
- Use ace-tool to search project documentation (README/docs/comments)
- Check license compatibility of open source dependencies (package.json/requirements.txt/go.mod)
- Assess application material gaps (user manuals, technical documents, test reports)
- Identify application process risks (naming conflicts, similar software, review cycles)
- Mitigation recommendations must be given for each risk
- Check paperless real-name, format compliance, fee reduction conditions, and electronic copyright fungibility based on policy anchor points

[MUST NOT DO]:
- Project source code must not be modified
- No legal advice may be given
- No other Agent output may be read at this stage

[CONTEXT]: Project path <project_path>, working directory .arc/ip-audit/<project-name>/
`
)
```

**Wait for the three Agents to complete** and use `background_output(task_id="...")` to collect the results.

### Phase 3: Cross-rebuttal

**Mandatory rebuttal mechanism** (each Agent must challenge the other two Agents):

```typescript
// Oracle refutes Deep and Writing
dispatch_job(
continuation_id="<oracle_continuation_id>", // Reuse Phase 2 session
  capabilities=["arc:ip-check"],
  execution_mode="foreground",
description="Oracle Cross Refutation Deep and Writing",
  prompt=`
[TASK]: Refute Deep and Writing’s assessment and point out overly optimistic/pessimistic aspects

[EXPECTED OUTCOME]:
- Generate agents/oracle/critique.md, including:
  1. Refutation of Deep implementation evaluation (cite specific scoring points)
  2. Rebuttal to Writing Compliance Assessment
  3. Each rebuttal must be accompanied by arguments (file path/code snippet/external reference)

[MUST DO]:
- Read agents/deep/implementation-analysis.md and agents/writing/compliance-analysis.md
- Challenging overly high ratings (pointing out the risk of being ignored)
- Challenge low ratings (point out undervalued innovations)
- Use ace-tool to verify dispute points
- Suggest corrections
- Counterarguments must include the three elements of technology/program product perspective, and can cite policy anchors.

[MUST NOT DO]:
- Don’t simply agree with the other person’s point of view
- No rebuttal without evidence

[CONTEXT]: working directory .arc/ip-audit/<project-name>/
`
)

// Deep refutes Oracle and Writing (same reason)
// Writing refutes Oracle and Deep (same reason)
```

**Collect refutation results** and generate `convergence/round-1-summary.md`.

### Phase 4: Comprehensive Feasibility Report

**Step 4.1: Weighted comprehensive score**

Dynamically adjust weights based on rebuttal reports:
- **Patent feasibility**: oracle 60% + deep 30% + writing 10%
- **Soft writing feasibility**: deep 60% + writing 30% + oracle 10%

If an Agent is strongly refuted (2+ strong arguments), its weight will be reduced by 10%.

**Step 4.2: Generate asset list**

Generate `analysis/ip-assets.md`:

| Asset number | Asset type | evidence path | soft cover feasibility | patent feasibility | preliminary risk |
|---------|---------|---------|-----------|-----------|---------|
| IPA-001 | Core algorithm | src/core/algorithm.ts:45-120 | high | middle | Similarity of existing technology to be confirmed |
| IPA-002 | System architecture | docs/architecture.md + src/server/ | middle | high | Novel architecture but conventional implementation |

**Step 4.3: Output final report**

Generate standardized reports using scripts:
```bash
python arc:ip-check/scripts/render_audit_report.py \
  --case-dir .arc/ip-audit/<project-name>/ \
  --project-name <project_name>
```

Generate files:
- `reports/ip-feasibility-report.md` (feasibility summary report)
- `reports/filing-readiness-checklist.md` (Readiness Checklist)
- `handoff/ip-drafting-input.json` (handover to arc:ip-draft)

`ip-feasibility-report.md` needs to add a new section:
- Soft copy material compliance (page format ≥50 lines/page, naming consistency, code sample coverage, description document screenshot consistency, signature page/guarantee status)
- Patent object compliance (integrity of three technical elements, feasibility of program product, adequacy of drawings/pseudocode)
- Fee reduction qualifications and economics (qualification judgment + original/fee reduction comparison)
- App electronic copyright fungibility (whether recommended, required materials)

`filing-readiness-checklist.md` Added: format check, naming consistency, fee reduction filing materials, signature page/non-job guarantee, and electronic copyright options.

**Step 4.4: handoff JSON extension fields**

`handoff/ip-drafting-input.json` New/extended fields (maintain backward compatibility, mark to be added if missing):
- `format_compliance`: {`code_pages_ok`, `doc_lines_ok`, `name_consistency`, `signature_page_ready`}
- `program_product_recommended`: boolean
- `fee_reduction`: {`eligible`: boolean, `basis`: string, `required_proofs`: []}
- `app_e_copyright`: {`recommended`: boolean, `materials`: []}

**Step 4.5: Give application suggestions**

Clear recommendations at the end of the report:
- **Soft work first**: complete code, complete documentation, insufficient patent threshold
- **Patent First**: Significant technological innovation, large differences in existing technologies, and limited software value
- **Parallel advancement**: High feasibility of both tracks, tight time limit, and sufficient budget

## Scripts

Prefer using scripts to generate standardized skeletons:

```bash
# Generate working directory skeleton
python arc:ip-check/scripts/scaffold_audit_case.py \
  --project-path <project_path> \
  --project-name <project_name>

# Render final report
python arc:ip-check/scripts/render_audit_report.py \
  --case-dir <output_dir> \
  --project-name <name>

# Fee reduction eligibility check (output to analysis/fee-reduction-assessment.md)
python arc:ip-check/scripts/fee_reduction_check.py \
  --applicant-type <individual|enterprise|institution> \
  --annual-income <number> \
  --output <path-to-analysis/fee-reduction-assessment.md>

# Format compliance check (output to convergence/format-compliance.md)
python arc:ip-check/scripts/format_compliance_checker.py \
  --project-path <project_path> \
  --output <path-to-convergence/format-compliance.md>
```

## Artifacts

Default output directory:`<project_path>/.arc/ip-audit/<project-name>/`

- `context/project-ip-snapshot.md` (project snapshot)
- `context/external-references.md` (external reference)
- `agents/oracle/innovation-analysis.md` (Oracle independent evaluation)
- `agents/oracle/critique.md` (Oracle rebuttal)
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
