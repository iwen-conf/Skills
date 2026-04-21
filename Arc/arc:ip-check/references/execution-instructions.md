# Instructions


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

```text
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

```text
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
| IPA-001 | Core algorithm | src/core/algorithm.js:45-120 | high | middle | Similarity of existing technology to be confirmed |
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
