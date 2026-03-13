---
name: "arc:ip-check"
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

### Phase 1: Context collection

**Step 1.1: Check cache and index**
1. Check if `.arc/ip-check/<project>/context/project-ip-snapshot.md` exists and is fresh (<24h).
2. Read the project `CLAUDE.md` level index (root level + core module level).
3. If there is a `.arc/audit/<project>/` review report, extract the architecture/technical debt/security conclusions.

**Step 1.2: Generate project snapshot**
Use `ace-tool` to search for the following and generate `context/project-ip-snapshot.md`:
- Core algorithm and performance optimization implementation
- Key module boundaries and system interaction
- Code snippets that can be submitted as software-copyright samples
- Technical solution description (architecture diagram, flow chart, data flow)
- Comparison of existing technologies (if included in project documents)
- Format/naming consistency baseline: software full name/abbreviation/version, header and footer examples, screenshot naming consistency check entry (for writing use)

**Step 1.3: External reference search**
Use `Exa` to search and generate `context/external-references.md`:
- Patent search for similar products (keywords: project core technology + field)
- Software copyright filing policy and review standards
- Patent application thresholds and common reasons for rejection
- Policy anchors (marked with date/source): Paperless real-name rules, code/description format requirements, App electronic copyright process, 2024 "computer program product" terms, 2025 fee reduction threshold and ratio

**Step 1.4: Scaffolding generation**
```bash
python Arc/arc:ip-check/scripts/scaffold_audit_case.py \
  --project-path <project_path> \
  --project-name <project_name>
```

### Phase 2: Multi-Agent concurrent independent evaluation

**Start three Agents concurrently** (in the same message):

```typescript
// Architecture: Architecture and Innovation Assessment
schedule_task(
  capability_profile="architecture",
  capabilities=["arc:ip-check"],
  execution_mode="background",
description="Architecture evaluates technological innovation and patent feasibility",
  prompt=`
[TASK]: Evaluate the technological innovation and patent application feasibility of the project

[EXPECTED OUTCOME]:
- Generate agents/architecture/innovation-analysis.md, including:
  1. Technical solution originality score (`1-10` or `N/A` when evidence is insufficient)
  2. Architectural design novelty score (`1-10` or `N/A` when evidence is insufficient)
  3. Difference analysis of existing technologies (reference context/external-references.md)
  4. Patent application feasibility score (high/medium/low or `N/A` when evidence is insufficient)
  5. Each rating must be accompanied by file path evidence
  6. Mapping table of three elements of technology (technical issues/technical means/technical effects)
  7. Determination of the patentability of program products (yes/no + basis)
  8. Suggested claim combination (method + system/device + computer program product + storage medium)

[REQUIRED TOOLS]: ace-tool (code search), Read (read context/), Write (write agents/architecture/)

[MUST DO]:
- Read context/project-ip-snapshot.md and context/external-references.md
- Use ace-tool to deeply analyze the core algorithm implementation
- Compare existing technologies and identify technical differences
- Each innovation point must reference a specific file path (file:line)
- Ratings must be bounded by evidence; use `N/A` when evidence is insufficient.
- Mark potential OA risks (object/creativity/out-of-scope) and cite policy anchors

[MUST NOT DO]:
- Project source code must not be modified
- No legal advice is allowed (project evaluation only)
- Do not confuse software and patent evaluation criteria
- The output of other Agents must not be read at this stage (cross-rebuttal only in Phase 3)

[CONTEXT]: Project path <project_path>, working directory .arc/ip-check/<project-name>/
`
)

// Deep: Project implementation evaluation
schedule_task(
  capability_profile="deep",
  capabilities=["arc:ip-check"],
  execution_mode="background",
description="Deep evaluates code integrity and software feasibility",
  prompt=`
[TASK]: Evaluate the code integrity of the project and the feasibility of software application

[EXPECTED OUTCOME]:
- Generate agents/deep/implementation-analysis.md, including:
  1. Code integrity score (`1-10` or `N/A` when evidence is insufficient)
  2. Implementation adequacy score (`1-10` or `N/A` when evidence is insufficient)
  3. Technical effect quantifiability score (`1-10` or `N/A` when evidence is insufficient)
  4. Software-copyright feasibility score (high/medium/low or `N/A` when evidence is insufficient)
  5. Each rating must be accompanied by code evidence
  6. Software authors can submit a code sample list (file + starting and ending lines, estimated number of pages that meet ≥50 lines/page, desensitization/deletion of comments suggestions)
  7. Technical effect quantification data table (including benchmarks/comparisons, if missing, mark the indicators that need to be supplemented)

[REQUIRED TOOLS]: ace-tool (code search), Read (read context/), Write (write agents/deep/)

[MUST DO]:
- Read context/project-ip-snapshot.md
- Use ace-tool to analyze code structural integrity
- Evaluate the adequacy of core function implementation (whether there are TODO/FIXME/not implemented)
- Check whether the technical effect is quantifiable (performance indicators, test coverage)
- Identify code sections that can be submitted as software-copyright samples (3000-5000 lines recommended)
- Each conclusion must reference a specific file path
- Record the file/line number of the first and last sample pages, and prompt whether desensitization/deletion of comments is required
- If there is a lack of performance/comparison data, output the "supplementary test indicators" list

[MUST NOT DO]:
- Project source code must not be modified
- No legal advice may be given
- Do not confuse software and patent evaluation criteria
- No other Agent output may be read at this stage

[CONTEXT]: Project path <project_path>, working directory .arc/ip-check/<project-name>/
`
)

// Writing: Documentation and Compliance Analysis
schedule_task(
  capability_profile="writing",
  capabilities=["arc:ip-check"],
  execution_mode="background",
description="Writing evaluates document completeness and application readiness",
  prompt=`
[TASK]: Evaluate the project's document completeness and readiness for intellectual property application

[EXPECTED OUTCOME]:
- Generate agents/writing/compliance-analysis.md, including:
  1. Document completeness score (`1-10` or `N/A` when evidence is insufficient)
  2. Material readiness score (`1-10` or `N/A` when evidence is insufficient)
  3. Application process risk assessment (high/medium/low or `N/A` when evidence is insufficient)
  4. Intellectual property compliance check (open source protocol conflicts, third-party dependencies)
  5. Each rating must be accompanied by evidence
  6. Naming/real name consistency, header and footer format check results
  7. Signature page, non-professional development guarantee, and open source statement readiness status
  8. Feasibility of App electronic copyright channel (if the target is to put it on the application market)
  9. Pre-evaluation of fee reduction eligibility and list of required supporting documents
  10. 极具实操性的提交策略建议（Practical submission strategies）：
      - 软著申请拆分建议：根据代码结构，明确建议是前后端合并申请，还是前端和后端分别独立申请软著（通常独立申请能更好保护，且避免代码量过大审查被拒）。
      - 申请顺序规划：明确建议先交专利还是先交软著（例如：若专利涉及核心首发创新，务必先交专利以保新颖性；软著则可在发布前提交）。
      - 软件名称定名建议：结合代码实际情况给出3个以上符合规范的软件名称候选项（格式建议：品牌/企业简称 + 核心功能描述 + 软件系统/平台/App V1.0），并说明定名理由。
      - 避坑与注意事项：软著UI截图与文档中名称必须严格一致、代码中绝不能出现竞品名称或开源未授权声明等。

[REQUIRED TOOLS]: ace-tool (search documents), Read (read context/), Write (write agents/writing/)

[MUST DO]:
- Read context/project-ip-snapshot.md
- Use ace-tool to search project documentation (README/docs/comments)
- Check license compatibility of open source dependencies (package.json/requirements.txt/go.mod)
- Assess application material gaps (user manuals, technical documents, test reports)
- Identify application process risks (naming conflicts, similar software, review cycles)
- Mitigation recommendations must be given for each risk
- Check paperless real-name, format compliance, fee reduction conditions, and App electronic copyright channel readiness based on policy anchor points

[MUST NOT DO]:
- Project source code must not be modified
- No legal advice may be given
- No other Agent output may be read at this stage

[CONTEXT]: Project path <project_path>, working directory .arc/ip-check/<project-name>/
`
)
```

**Wait for the three Agents to complete** and use `collect_task_output(task_id="...")` to collect the results.

### Phase 3: Cross-rebuttal

**Mandatory rebuttal mechanism** (each Agent must challenge the other two Agents):

```typescript
// Architecture refutes Deep and Writing
schedule_task(
task_ref="<architecture_task_ref>", // Reuse Phase 2 session
  capabilities=["arc:ip-check"],
  execution_mode="foreground",
description="Architecture Cross Refutation Deep and Writing",
  prompt=`
[TASK]: Refute Deep and Writing's assessment and point out overly optimistic/pessimistic aspects

[EXPECTED OUTCOME]:
- Generate agents/architecture/critique.md, including:
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
- Don't simply agree with the other person's point of view
- No rebuttal without evidence

[CONTEXT]: working directory .arc/ip-check/<project-name>/
`
)

// Deep refutes Architecture and Writing (same reason)
// Writing refutes Architecture and Deep (same reason)
```

**Collect refutation results** and generate `convergence/round-1-summary.md`.

### Phase 4: Comprehensive Feasibility Report

**Step 4.1: Weighted comprehensive score**

Use rebuttal results to synthesize a final judgment, but treat weighting as a calibration guide rather than a fixed formula:
- **Patent feasibility**: architecture evidence should usually carry the most weight, followed by deep, with writing acting mainly as a compliance and risk modifier.
- **Software copyright feasibility**: deep evidence should usually carry the most weight, followed by writing, with architecture acting mainly as a boundary and context modifier.

If an Agent is strongly refuted, reduce its influence only with an explicit explanation in `convergence/final-consensus.md`; do not apply a mechanical percentage shift.

**Step 4.2: Generate asset list**

Generate `analysis/ip-assets.md`:

| Asset number | Asset type | evidence path | software-copyright feasibility | patent feasibility | preliminary risk |
|---------|---------|---------|-----------|-----------|---------|
| IPA-001 | Core algorithm | src/core/algorithm.ts:45-120 | high | middle | Similarity of existing technology to be confirmed |
| IPA-002 | System architecture | docs/architecture.md + src/server/ | middle | high | Novel architecture but conventional implementation |

**Step 4.3: Output final report**

Generate standardized reports using scripts:
```bash
python Arc/arc:ip-check/scripts/render_audit_report.py \
  --case-dir .arc/ip-check/<project-name>/ \
  --project-name <project_name>
```

Generate files:
- `reports/ip-feasibility-report.md` (feasibility summary report)
- `reports/filing-readiness-checklist.md` (Readiness Checklist)
- `handoff/ip-drafting-input.json` (handover to arc:ip-draft)

`ip-feasibility-report.md` needs to add a new section:
- Software-copyright material compliance (page format ≥50 lines/page, naming consistency, code sample coverage, description document screenshot consistency, signature page/guarantee status)
- Patent object compliance (integrity of three technical elements, feasibility of program product, adequacy of drawings/pseudocode)
- Fee reduction qualifications and economics (qualification judgment + original/fee reduction comparison)
- App electronic copyright channel readiness (whether recommended, required materials)
- **Practical Application Guide (实操申请指南)**: 
  - **软件名称建议 (Software Naming Suggestions)**: 结合项目实际功能和代码库名称，给出符合软著局规范的具体名称建议（如 [品牌] [功能] [系统/平台/App] V1.0）。
  - **拆分与组合策略 (Submission Strategy)**: 明确指出针对当前项目，前后端是否应该分开独立申请软著（基于前后端解耦程度、代码量大小），以及专利是否需要拆分多个等。
  - **申报先后顺序 (Submission Order)**: 强烈建议先申请发明专利（保护新颖性），再申请软件著作权，并给出时间线规划。
  - **提交注意事项与避坑指南 (Common Pitfalls & Tips)**: 详细列出：代码中不能有TODO或无关注释、说明书截图名称必须与申请表完全一致、哪些核心文件在系统中先传、软著申请平台的实操建议等。

`filing-readiness-checklist.md` Added: format check, naming consistency, fee reduction filing materials, signature page/non-job guarantee, and App electronic copyright options.

**Step 4.4: handoff JSON extension fields**

`handoff/ip-drafting-input.json` New/extended fields (always include the keys; use explicit `null` values when evidence is insufficient):
- `format_compliance`: {`code_pages_ok`: boolean|null, `doc_lines_ok`: boolean|null, `name_consistency`: boolean|null, `signature_page_ready`: boolean|null}
- `program_product_recommended`: boolean|null
- `fee_reduction`: {`eligible`: boolean|null, `basis`: string, `required_proofs`: []}
- `app_e_copyright`: {`recommended`: boolean|null, `materials`: []}

**Step 4.5: Give application suggestions**

Clear recommendations at the end of the report:
- **Software copyright first**: complete code, complete documentation, insufficient patent threshold
- **Patent First**: Significant technological innovation, large differences in existing technologies, and limited software value
- **Parallel advancement**: High feasibility of both tracks, tight time limit, and sufficient budget
- **Defer filing / collect evidence first**: material readiness or evidence strength is insufficient for a bounded recommendation

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
