# Instructions


### Phase 1: Read review handoff and project context

**Step 1.1: Read the handover file**
1. Read `<audit_case_dir>/handoff/ip-drafting-input.json`.
2. extract:
   - Target asset list (IPA-001, IPA-002...)
   - Software-copyright and patent feasibility score
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
python Arc/arc:ip-draft/scripts/scaffold_drafting_case.py \
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
- Don't make up technical details
- Do not use vague expressions ("maybe", "probably", "generally")
- The output of other agents must not be read at this stage (cross-review only in Phase 3)
- Do not confuse the description style of software and patents

[CONTEXT]: Project path <project_path>, working directory .arc/ip-draft/<project-name>/
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
  5. Description of software-copyright code materials (recommended code sections to submit)
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
- Don't fabricate performance data
- Do not disclose trade secrets or sensitive information
- No other Agent output may be read at this stage
- The complete source code may not be used (only key snippets)

[CONTEXT]: Project path <project_path>, working directory .arc/ip-draft/<project-name>/
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
  3. Software-copyright application abstract (300-500 words, for Copyright Office examiners)
  4. Draft patent claims (independent claims + dependent claims)
  5. User interface description (UI screenshot + operation process)
  6. Header/footer/naming consistency check hints (referencing the format baseline)
  7. Signature page, non-professional development guarantee, open source statement placeholder reminder
  8. Four-piece set of claims (method + system/device + computer program product + storage medium) sentence placeholder
  9. 撰写一份“保姆级”软著与专利提交操作指南（Step-by-step submission guide），详细说明：
     - 应该先登录哪个官方平台。
     - 软著应该先填报哪些信息，前后端如果分开交应该怎么填软件名称。
     - 文件上传的顺序和注意事项（代码文档前30页后30页的整理要求，UI截图的要求）。
     - 申请过程中的避坑要点，例如保证所有的名称（文档、截图、申请表）一字不差，代码不能有大段空行等。

[REQUIRED TOOLS]: ace-tool (search documents), Read (read context/), Write (write agents/writing/)

[MUST DO]:
- Read context/doc-context.md and context/handoff-input.json
- Use ace-tool to search project documentation (README/docs/)
- Function description must be oriented to non-technical users (clear and understandable)
- The operating instructions must be complete (covering the entire process from installation to use)
- The software-copyright application abstract must comply with the format requirements of the Copyright Office
- Claims must comply with the patent law format (one sentence, clear hierarchy)
- Verify the consistency of software name and version and mark it in the document
- Insert screenshot placeholder description (must be consistent with the software name)
- Mark signature page/guarantee/open source statement to be added

[MUST NOT DO]:
- No technical jargon (for users)
- No fabricated functions are allowed (must be supported by code)
- No other Agent output may be read at this stage
- Do not confuse software-copyright abstracts with patent abstracts

[CONTEXT]: Project path <project_path>, working directory .arc/ip-draft/<project-name>/
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
- Don't simply agree with the other person's point of view
- No review without evidence

[CONTEXT]: working directory .arc/ip-draft/<project-name>/
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

**Step 4.2: Generate software-copyright documents** (such as `target_docs` contains `copyright`)

Use a script to generate standardized documentation:
```bash
python Arc/arc:ip-draft/scripts/render_ip_documents.py \
  --case-dir .arc/ip-draft/<project-name>/ \
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
python Arc/arc:ip-draft/scripts/render_ip_documents.py \
  --case-dir .arc/ip-draft/<project-name>/ \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs patent
```

Generate files:
- `patent/disclosure-draft.md` (Technical communication document, integration of Architecture technical solutions + Deep implementation details + terminology unification)
- `patent/claims-draft.md` (draft claims, integrated writing drafting + Architecture/Deep review)
  - Default includes: 1 independent method + 1 system/device + 1 computer program product + 1 storage medium; dependent claims cite differences in performance/data flow/module parameters
- `patent/drawings-description.md` (Chart description, integrated Architecture architecture diagram + Deep data flow diagram + Writing UI screenshot)

**Step 4.4: Generate writing log and Submission Guide**

Generate `reports/doc-writing-log.md`:
- Input source (handoff file path, CLAUDE.md path)
- Assumptions and editorial placeholders (record only non-technical gaps to be confirmed later; do not infer unverified technical facts)
- Items to be added manually (information provided by the applicant is required)
- Agent collaboration records (drafting content, review comments, and terminology unified decision-making by each agent)
- Fee reduction materials list, electronic copyright options, format compliance check results, missing performance data/screenshot list

Generate `reports/submission-guide.md` (保姆级实操提交指南):
- 提炼一份完全面向小白用户的操作文档。
- **软件定名与版本**：明确告诉你该用什么名字去填表。
- **前后端申请策略**：明确指出需要提交几次软著，前后端各叫什么名字，如何区分代码交集。
- **提交顺序**：明确专利与软著的先后顺序，避免破坏专利新颖性。
- **材料准备清单**：精确到哪个文件用于填哪一项，截图需要多少张。
- **官方平台填报避坑**：名称一致性死命令、文档页眉页脚要求、代码格式要求（不可有大段空行、不可有乱码、必须包含关键版权头部）。
