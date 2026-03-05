---
name: "arc:audit"
description: "项目体检：七维评审并输出 HTML 可视化报告、风险与改进路线。"
---

# Enterprise-level project review (multi-agent confrontational)

## Overview

Give Agent the ability to be an “enterprise-level software review expert”. Through the three professional agents of architecture, deep, deep (engineering), and deep (business), each independently evaluates the project or key PR according to seven dimensions, and then refutes each other's scores and findings, and finally converges on a deliverable diagnostic report and improvement roadmap.

In addition to the seven-dimensional total score, the review must add two new special scores: **Business Maturity Score** (focusing on business link opening rate/broken link rate) and **Dependency Health Score** (focusing on outdated dependencies/vulnerable dependencies/discontinued maintenance dependencies).

The seven-dimensional assessment framework refers to the ISO/IEC 25010 software quality model, TOGAF enterprise architecture framework and modern software engineering best practices (see `references/dimensions.md` for details).

The process is divided into four stages:

1. **Project reconnaissance**: Scan code structure, dependencies, technology stack → generate project snapshot
2. **Independent evaluation**: Multi-Agent concurrent independent evaluation according to seven dimensions → each produces 7 dimensional analyses.
3. **Cross Refutation**: Each Agent refutes each other's scores and findings → each issues a refutation report
4. **Convergence Report**: Master Process Aggregation → Final Diagnostic Report + Scorecard + Improvement Suggestions

## Quick Contract

- **Trigger**: Enterprise-level multi-dimensional diagnosis, risk identification and improvement roadmap required.
- **Inputs**: Project path, assessment dimension range, depth level and focus areas.
- **Outputs**: Diagnostic report, score card, improvement suggestions and evidence list (including business maturity score and dependency health score).
- **Quality Gate**: Reports must pass `## Quality Gates`'s evidence and trade-off check before publishing.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:audit` to do an evidence-based review first, and then output the seven-dimensional + special scoring diagnosis and roadmap."

## The Iron Law

```
NO SCORE WITHOUT EVIDENCE, NO RECOMMENDATION WITHOUT TRADEOFF
NO BUSINESS MATURITY SCORE WITHOUT FLOW EVIDENCE
NO DEPENDENCY HEALTH SCORE WITHOUT VERSION EVIDENCE
```

No ratings can be given without evidence, and no suggestions can be given without weighing them up.

## Workflow

1. Scout project context and build review snapshots.
2. Multiple agents can complete seven-dimensional independent assessment concurrently.
3. Cross-rebuttal and correction of scoring disagreements.
4. Produce final diagnostic reports, scorecards, special scores and improvement roadmaps.

## Quality Gates

- Each dimensional conclusion must be bound to code or configuration evidence.
- Ratings must include addition/deduction items and explanations.
- The business dimension must give "business link opening rate + link disconnection rate + business maturity score".
- The technical debt dimension must give "dependency obsolescence rate + vulnerability risk + dependency health score".
- Recommendations must include benefit, cost and risk trade-offs.
- The report needs to distinguish between “observed facts” and “improvement suggestions”.

## Expert Standards

- 七维评审必须给出 `评分依据 + 证据 + 置信度`，并保留跨评审组校准记录。
- 业务成熟度必须量化 `链路通畅率 / 断链率 / 关键场景通过率 / 业务异常信号`。
- 依赖健康度必须覆盖 `EOL 状态 / CVE 暴露 / 升级阻塞 / 维护活跃度` 四类指标。
- 报告必须输出 `治理路线图`（P0/P1/P2）并标注 owner、时限、验收口径。
- HTML 报告必须提供 `9 Tab`，每个 Tab 内置专家评审卡（结论/Gate/风险/SLA/证据）。

## Scripts & Commands

- 运行时主命令：`arc audit`
- 量化集成（自动选主题 + 9 Tab）：`python3 arc:audit/scripts/integrate_score.py --project-path <project_path> --review-dir <project_path>/.arc/arc:audit/<project_name>`
- 指定 score 目录：`python3 arc:audit/scripts/integrate_score.py --score-dir <score_dir> --review-dir <project_path>/.arc/arc:audit/<project_name>`
- 若缺少 score 输入，先执行：`arc gate`

## Red Flags

- Only subjective judgments are given, and no evidence path is given.
- The seven dimensions are simplified into a single total score conclusion.
- The business dimension does not quantify "how many businesses are connected and how many are disconnected."
- The dependency review only lists the package name and does not evaluate the risks of obsolescence/vulnerabilities/discontinuation of maintenance.
- Ignore the rebuttal stage and merge the reports directly.
- Reports cannot lead to actionable improvement actions.

## Context Budget (avoid Request too large)

- Don't paste the entire code base into the conversation during the project reconnaissance phase; only extract key file paths, directory structures, dependency lists, and configuration summaries.
- Each dimension analysis is controlled within 200-500 lines; if it is too long, split it into independent files and quote the path.
- Code evidence only quotes key snippets (lines 5-20) and does not paste the complete file.

## When to Use

- **首选触发**: Evidence-based and traceable review of projects or key PRs is required.
- **典型场景**: Technical due diligence, milestone review, health assessment before architecture upgrade, high-risk PR review before merger, pre-release testing strategy/performance regression/security baseline assessment.
- **边界提示**: Use `arc:gate` only for access control blocking; use `arc:build` for floor-to-ceiling transformation plan.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | The absolute path to the root directory of the project to be reviewed |
| `project_name` | string | no | Project name (used for directory naming); if not provided, deduced from path |
| `scope_dimensions` | array | no | Evaluation dimension range, default is all 7 dimensions; optional `["architecture", "security", "code-quality", "business", "devops", "team", "tech-debt"]` |
| `depth_level` | string | no | Evaluation depth: `"quick"` / `"standard"` / `"deep"`; default `"standard"` |
| `focus_areas` | array | no | Areas of special concern (such as `["security", "tech-debt"]`) will be analyzed more deeply in these dimensions. |
| `business_flow_catalog` | array | no | Business flow list (such as `["signup->order->pay", "refund->reconcile"]`), used to compute flow-through rate and broken-flow rate |
| `output_dir` | string | no | Output root directory; default `<project_path>/.arc/arc:audit/` |

## Dependencies

* **Organization Contract**: Required. Following `docs/orchestration-contract.md`, scheduling is implemented through the runtime adaptation layer.
* **ace-tool (MCP)**: Required. Used to semantically search project code structure, implementation patterns, CLAUDE.md index.
* **Exa MCP**: Recommended. Used to search for industry standards, best practices, and security vulnerability information that projects depend on.
* **Runtime Independent Scheduling Layer**: Required. Supports concurrent task dispatch, result collection and failure retry (not limited to specific APIs).

## Review division of labor model (runtime independent)

| judging panel | focus | Output directory |
|------|---------|------|
| **Architecture Group** | architecture/security/tech-debt | `architecture/` |
| **Engineering Team** | code-quality/devops | `deep/` |
| **Business Group** | business/team | `deep-business/` |

## Critical Rules

0. **Markdown format verification (highest priority)**
   - All tables that produce Markdown must have column numbers aligned (table headers, separated rows, and data rows must be consistent).
   - It must be verified after generation, and verification failure must be repaired.

1. **Read-Only without changing the code (Read-Only)**
   - **It is strictly prohibited** to modify, delete, or add any source code or configuration files of the reviewed project.
   - **Allow** to create review output files in the `.arc/arc:audit/` directory.

2. **Must cite code evidence**
   - Each evaluation conclusion must be accompanied by a specific file path + line number or code snippet.
   - Format: `file_path:line_number` or quote 5-20 lines of key code blocks.
   - **BANNED** Subjective assertions without evidence.

3. **Best Practices of Benchmarking Enterprises**
   - The evaluation of each dimension must refer to the checks in `references/dimensions.md`.
   - The security dimension must be benchmarked against the OWASP Top 10.
   - The architectural dimension must consider the ISO/IEC 25010 quality model.

4. **Distinguish between "observation" and "suggestion"**
   - Observation = fact, must have code evidence.
   - Recommendation = opinion, reasons and trade-offs must be stated.
   - The two must not be confused.

5. **Ratings must be based on evidence**
   - A 0-10 rating for each dimension must list deductions and bonus points.
   - In the multi-Agent rebuttal stage, you can challenge the opponent's score and make corrections.

6. **Resource Control & Cleanup**
   - All MCP searches and file reads must be capped, and infinite recursive scans are prohibited.
   - Limit analysis to a maximum of 5 key documents per dimension when using `depth_level="quick"`.

7. **Special rating required**
   - Dimension 4 (business) must produce "Business Maturity Score (0-10)", with details of the opening rate/broken link rate.
   - Dimension 7 (tech-debt) must produce "Dependency Health Score (0-10)", with details of outdated/vulnerable/out of maintenance dependencies.
   - If it cannot be quantified, the "missing data + reasons for obstruction + supplementary plan" must be clearly defined and should not be left blank.

## Seven Dimensions Assessment Framework

See `references/dimensions.md` for details. Each dimension contains inspection items, evaluation points, and sources of code evidence.

| # | Dimensions | English logo | core issues |
|---|------|---------|---------|
| 1 | Architecture design and technology evolution | architecture | Does the architecture support long-term evolution? |
| 2 | Security compliance and authority control | security | Are security and permissions reliable? |
| 3 | Code quality and engineering specifications | code-quality | How is the code quality and maintainability? |
| 4 | Business value and demand fit | business | Does it solve real business problems? |
| 5 | Operations Observability and Deployment Delivery | devops | Is operation and maintenance robust and observable? |
| 6 | Team effectiveness and collaboration | team | Is the team maintained sustainably? |
| 7 | Technical Debt and Evolution Resistance | tech-debt | How "heavy" is the system? |

## Instructions (execution process)

#### Step 1.0: Check snapshot cache

**Before starting the project scan, check whether existing snapshots can be reused:**

1. **Check if `.arc/arc:audit/<project-name>/context/project-snapshot.md` exists**
2. **Verify Snapshot Freshness**: Check generation time < 24 hours in snapshot metadata
3. **Verify snapshot integrity**: Check whether the snapshot contains all required fields (basic information, directory structure, technology stack, code size, CI/CD, test structure)
4. **If the snapshot exists and is fresh**:
   - Compare file hashes (if the snapshot contains hashes):
     - The hashes are the same: update the timestamp, use the snapshot directly, skip Step 1.1-1.3
     - Hashes are different: Regenerate snapshot
   - If the snapshot does not contain a hash: use `git diff --name-only` or file timestamp to check if the item has changed
     - No changes: use the snapshot directly
     - Changes: Regenerate snapshot
5. **If the snapshot does not exist or is expired (≥ 24 hours)**: Perform a full project scan (Step 1.1-1.3)

#### Step 1.0A: Shared index and upstream handover pre-check (mandatory)

Before entering Step 1.1, you must first try to reuse the upstream product:

1. Read `.arc/context-hub/index.json` and retrieve the following products:
   - `.arc/score/<project>/handoff/review-input.json`
   - `.arc/arc:build/<task>/handoff/change-summary.md` (if present)
   - Latest `codemap.md` / `CLAUDE.md` metadata
2. Verify product validity: `expires_at`, `content_hash`, path existence.
3. If valid, it will be loaded directly into the review context to reduce the scope of repeated scanning.
4. If it is invalid, a reflow update will be triggered:
   - score invalid → `score` module refresh (triggered by `arc:gate` orchestration)
   - codemap/CLAUDE metadata invalid → `arc:init --mode update` / `arc:cartography`
5. Write the reused product path into `context/project-snapshot.md` metadata.

---

### Phase 1: Project Reconnaissance

**Goal**: Collect basic information about the project being reviewed and build a panoramic snapshot of the project.

#### Step 1.1: Scan project structure

1. **Use ace-tool MCP** to search project code structure, technology stack, and core modules
2. **Read the project CLAUDE.md hierarchical index** (if it exists) to quickly understand the project vision and module relationships
3. **Collect basic metadata**:
   - Directory structure (top level + key subdirectories)
   - Technology stack derivation (from go.mod / package.json / Cargo.toml / requirements.txt, etc.)
   - Dependency list and version
   - Dependency health snapshot (outdated dependencies, vulnerability dependencies, maintenance dependencies, upgrade automation)
   - List of core business links (opened, partially connected, disconnected/manually covered)
   - Code size (number of files, distribution of major languages)
   - CI/CD configuration (.github/workflows, Makefile, Dockerfile, etc.)
   - Test structure (test directory, coverage configuration)

#### Step 1.2: Search for industry standards

Search using **Exa MCP**:
- Latest security bulletins for core frameworks/libraries used by the project
- Architecture best practices for similar projects in the industry
- Known vulnerabilities, deprecation and maintenance announcements related to the technology stack
- LTS/latest stable versions of key dependencies and upgrade recommendations

#### Step 1.3: Generate project snapshot

Output `context/project-snapshot.md`, containing:

```markdown
# Project snapshot: <project_name>

## Basic information
* **Path**: <project_path>
* **Main language**: <languages>
* **Framework/Library**: <frameworks>
* **Review Time**: <timestamp>
* **Review depth**: <depth_level>

## Directory structure
<tree output, top 3 levels>

## technology stack
<dependency list with versions>

## Dependency health snapshot
* **Outdated dependency rate**: <percent>
* **Vulnerability dependency**: <critical/high/medium/low counts>
* **Stop maintaining dependencies**: <count + package list>
* **Upgrade Automation**: <dependabot/renovate/manual>

## Business link maturity snapshot
* **Total number of core links**: <N>
* **Link has been opened**: <N>
* **Broken link/Manual hidden link**: <N>
* **Business opening rate**: <percent>
* **Business disconnection rate**: <percent>
* **Current business maturity score (rough calculation)**: <X/10>

## code size
<file counts by language>

## CI/CD
<pipeline summary>

## test structure
<test directory layout, coverage config>
```
## Snapshot metadata (for cache validation)
- **Generation time**: <ISO 8601 timestamp>
- **Project path**: <absolute path>
- **File Hash**: <SHA-256 of key files: go.mod, package.json, etc.>
- **Scan range**: <list of scanned directories>

---

### Phase 2: Multi-Agent Independent Assessment

**Goal**: Multi-Agent concurrently, each independently evaluates projects according to seven dimensions.

#### Step 2.1: Multi-Agent concurrent evaluation

**CRITICAL**: The three review groups must be started concurrently (implemented by the current runtime concurrency mechanism, the calling syntax is not limited).

After each Agent reads `context/project-snapshot.md`, it evaluates all 7 dimensions independently.

**Mandatory requirements for special scoring (must be implemented by all Agents)**:
- Dimension 4 (business) additional output: `business_maturity: X/10`, `flow_through_rate: Y%`, `broken_flow_rate: Z%`
- Dimension 7 (tech-debt) additional output: `dependency_health: X/10`, `outdated_dependency_rate: Y%`, `high_risk_vuln_dependencies: N`

Concurrent execution requirements (runtime independent):

1. **Architecture Group Assessment** (focusing on architecture/security/tech-debt)
   - Input: `context/project-snapshot.md` + project code evidence
   - Output: `architecture/dim-N-<name>.md`
2. **Engineering team evaluation** (focusing on code-quality/devops)
   - Input: `context/project-snapshot.md` + project code evidence
   - Output: `deep/dim-N-<name>.md`
3. **Business group assessment** (focusing on business/team)
   - Input: `context/project-snapshot.md` + project code evidence
   - Output: `deep-business/dim-N-<name>.md`

All three review groups must cover all 7 dimensions; dimension 4/7 must output special scores, and all conclusions must be accompanied by `file:line` evidence.

#### Step 2.2: Wait for completion

Wait for the tasks of the three review groups to be completed and collect the dimensional analysis files in each directory.

---

### Phase 3: Cross-Review & Rebuttal

**Goal**: Each Agent refutes each other's ratings and findings, eliminates blind spots, and calibrates ratings.

#### Step 3.1: Multi-Agent concurrent rebuttal

**CRITICAL**: Each Agent must **refute all 7 dimensions of the other two Agents**'s evaluations.

Each Agent must:
1. Read all 7 dimensional analyzes of the other two Agents
2. **Rebutting overly optimistic or pessimistic ratings** (reasons and code evidence must be given)
3. **Point out missed issues or overlooked strengths**
4. **Give revised scoring suggestions**

**Architecture team refutes engineering team + business team**:
- Read `deep/dim-*.md` and `deep-business/dim-*.md`
- Refutation from an architectural perspective
- Output `architecture/critique.md`

**Engineering team refutes architecture team + business team**:
- Read `architecture/dim-*.md` and `deep-business/dim-*.md`
- Rebuttal from an engineering/code quality/security perspective
- Output `deep/critique.md`

**Business Group refutes Architecture Group + Engineering Group**:
- Read `architecture/dim-*.md` and `deep/dim-*.md`
- Refutation from the quality/UX/operation and maintenance perspective
- Output `deep-business/critique.md`

---

### Phase 4: Convergence & Report

**Goal**: The main process aggregates the analysis and rebuttal of all parties and generates the final deliverable.

#### Step 4.1: Aggregate Ratings

Read each party’s dimensional analysis + rebuttal report, for each dimension:
1. Take the **professional weighted average** of the ratings from all parties:
   - architecture/security/tech-debt: architecture 50%, deep 25%, deep(business) 25%
   - code-quality/devops: deep 50%, architecture 25%, deep (business) 25%
   - business/team: deep(business) 50%, architecture 25%, deep 25%
2. Adjust according to the rebuttal report (if a certain party's score is strongly refuted by the other two parties, its weight will be reduced)
3. Generate final score and basis
4. Aggregate two special points separately:
   - **Business Maturity Score**: Combined with the three-party dimension 4 conclusions, summarized by `references/scoring-weights.yaml` and `special_scores.business_maturity`
   - **Dependence on Health Score**: Combined with the 7 conclusions of the three dimensions, summarized by `special_scores.dependency_health`

#### Step 4.2: Generate diagnostic report

Output `diagnostic-report.md`:

```markdown
# Project diagnostic report: <project_name>

## executive summary
* **Overall Rating**: X.X/10
* **Business maturity score**: X.X/10 (opening rate: Y%, broken link rate: Z%)
* **Dependency health score**: X.X/10 (outdated dependency rate: Y%, high-risk vulnerability dependency: N)
* **Review Time**: <timestamp>
* **Review depth**: <depth_level>
* **Key Risks**: <top 3 risks>

## Rating overview

| Dimensions | score | Rating | Key findings |
|------|------|------|---------|
| Architecture design | X/10 | good | <sentence> |
| Security Compliance | X/10 | warn | <sentence> |
| ... | ... | ... | ... |

## Dimension 1: Architecture design and technology evolution (X/10)
### observe
<Facts + Code Evidence>
### analyze
<Synthesis of tripartite perspectives>
### suggestion
<Specific improvement measures>

## Dimension 2-7: ...
(The structure is the same as dimension 1)

## Disagreements
<The opinions and reasons why each agent did not reach a consensus>

## Special scoring instructions
### business maturity
<Core links, open rate, broken link rate, key evidence>
### dependent on health
<Outdated dependencies, vulnerability dependencies, out-of-maintenance dependencies, key evidence>

## Methodological statement
<Seven-dimensional framework source, scoring criteria, multi-agent confrontation mechanism>
```

#### Step 4.3: Generate scorecard

Output `scorecard.md`:

```markdown
# Scorecard: <project_name>

## Seven Dimensions Score

| Dimensions | architecture | deep | deep(business) | final | Rating |
|------|--------|------|-------|------|------|
| Architecture design | 7 | 8 | 7 | 7.3 | good |
| Security Compliance | 6 | 5 | 6 | 5.6 | qualified |
| ... | ... | ... | ... | ... | ... |

## Special scoring

| index | Fraction | key evidence |
|------|------|---------|
| Business maturity points | X.X/10 | Core link opening rate, link disconnection rate, manual backup list |
| Rely on health points | X.X/10 | Outdated dependency rate, high-risk vulnerability dependencies, maintenance dependencies |

## Rating criteria
| Fraction | Rating | meaning |
|------|------|------|
| 9-10 | excellence | Industry benchmark |
| 7-8 | good | Meet enterprise-level standards |
| 5-6 | qualified | Basically usable, with obvious shortcomings |
| 3-4 | warn | more serious problem |
| 0-2 | Danger | systemic flaws |
```

#### Step 4.4: Generate improvement suggestions

Output `recommendations.md`:

```markdown
# Improvement suggestions: <project_name>

## P0 — Fix now (blocking problem)
- [ ] <suggestion> — dimension: <dimension name>, evidence: `file:line`

## P1 — Short-term improvement (within 1-2 iterations)
- [ ] <suggestion> — dimension: <dimension name>, evidence: `file:line`

## P2 - Medium-Term Planning (Quarterly Level)
- [ ] <suggestion> — Dimension: <dimension name>

## P3 — Long-term evolution (half a year+)
- [ ] <suggestion> — Dimension: <dimension name>
```

#### Step 4.5: Generate quantitative visualization HTML (mandatory)

Must be performed at the end of Phase 4:

```bash
python3 <skills_root>/arc:audit/scripts/integrate_score.py \
  --project-path <project_path> \
  --review-dir <project_path>/.arc/arc:audit/<project-name>
```

At least the following products should exist after execution:
- `quantitative-dashboard.html` (single output; 9 Tabs = 总览 + 七维详细分析 + 业务完成度；theme selected by `time now()` at generation time)
- Each tab must include expert review fields: `结论 / Gate 建议 / 风险等级 / 整改时限 / 关键证据` and avoid duplicated findings across tabs.

If the `.arc/score/<project-name>/` quantification input is missing, you must first trigger `arc:gate` to refresh the score product, and then retry this step.

---

## Artifacts & Paths

```
<workdir>/.arc/arc:audit/<project-name>/
├── context/
│ └── project-snapshot.md # Snapshot of basic project information
├── architecture/
│ ├── dim-1-architecture.md # Dimension 1 Analysis
│   ├── dim-2-security.md
│   ├── dim-3-code-quality.md
│   ├── dim-4-business-value.md
│   ├── dim-5-devops.md
│   ├── dim-6-team.md
│   ├── dim-7-tech-debt.md
│ └── critique.md # Cross rebuttal
├── deep/
│   ├── dim-1-architecture.md
│ ├── ...(same as architecture)
│   └── critique.md
├── deep-business/
│   ├── dim-1-architecture.md
│ ├── ...(same as architecture)
│   └── critique.md
├── diagnostic-report.md # Final diagnostic report
├── scorecard.md # scorecard
├── quantitative-dashboard.html # Quantitative evaluation HTML (single output; 9 tabs, no-dup sections, theme from time now())
├── business-maturity.md # Business maturity special score
├── dependency-health.md # Dependency health special score
└── recommendations.md # Improvement recommendations (by priority)
```

## Timeouts and downgrades

| Condition | deal with |
|------|------|
| Agent timeout > 10min | Use AskUserQuestion to ask the user whether to continue using the remaining Agents |
| A certain Agent dimension analysis is missing | Fill in the analysis with the other two agents and mark "single source evaluation" |
| ace-tool MCP is not available | Downgrade to Grep + Read to manually scan critical files |
| depth_level="quick" | Limit 5 key documents per dimension, skip Phase 3 rebuttal |

## status feedback

```
[Arc Review] Project: <project_name>

=== Phase 1: Project Scouting ===
├── ace-tool scan... [Complete]
├── Exa Search Industry Standards... [Complete]
└── Generate project snapshot... [Complete]

=== Phase 2: Independent Assessment ===
├── Architecture Group 7 Dimensions... [Completed]
├── Engineering Team 7 Dimensions... [Complete]
└── Business Group 7 Dimensions... [Complete]

=== Stage 3: Cross-rebuttal ===
├── architecture rebuttal deep+deep(business)... [Complete]
├── deep rebuttal architecture+deep(business)... [Complete]
└── deep(business) rebuttal architecture+deep... [Complete]

=== Phase 4: Convergence Report ===
├── Aggregated Rating... [Complete]
├── Diagnostic report... [Complete]
├── Scorecard... [Complete]
└── Suggestions for improvement... [Completed]
```
## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:audit execution:**

### Review Process Anti-Patterns

- **Source Modification**: Editing project source code during review — only write to `.arc/arc:audit/`
- **Single-Perspective Review**: Only evaluating one dimension — must cover all 7 ISO/IEC 25010 dimensions
- **Business Blind Spot**: Not reporting business flow coverage and breakpoints in dimension 4
- **Dependency Blind Spot**: Not quantifying outdated/vulnerable/unmaintained dependencies in dimension 7
- **Rubber Stamp**: Marking PASS without thorough analysis — each finding requires evidence
- **Score Skipping**: Not preparing score artifacts first (typically via arc:gate) — quantitative data required before qualitative review
- **Dashboard Skipping**: Not running `arc:audit/scripts/integrate_score.py` in Phase 4 — HTML dashboard artifacts are mandatory

### Finding Anti-Patterns

- **Vague Recommendations**: "Improve performance" without specific metrics — actionable items only
- **Priority Inflation**: Marking everything High priority — dilutes critical issues
- **Evidence Omission**: Findings without code references or metrics — not actionable

### Handoff Anti-Patterns

- **Missing Roadmap**: Not generating improvement roadmap — breaks downstream planning
- **Orphaned Artifacts**: Not registering findings in context-hub — consumers can't discover

## Quick Reference
| stage | step | Output path |
|------|------|---------|
| Project Scouting | MCP Scan → Snapshot | `context/project-snapshot.md` |
| independent assessment | Multi-Agent×7 dimensions | `(architecture|deep|deep-business)/dim-N-<name>.md` |
| cross rebuttal | Agents refute each other | `(architecture|deep|deep-business)/critique.md` |

## Quick check on collaboration and division of labor

| Role | Responsibilities | Concurrency advice |
|------|------|---------|
| architecture group | Assess architecture, security, technical debt and output rebuttals | Concurrent with the engineering group and business group |
| engineering team | Assess code quality, DevOps and output rebuttals | Concurrent with the architecture group and business group |
| business group | Assess business value, collaborate as a team, and generate rebuttals | Concurrent with the architecture group and engineering group |
| Main process (aggregation/reporting) | Aggregated scoring, output reporting and dashboards | Execute after completion of three groups |
