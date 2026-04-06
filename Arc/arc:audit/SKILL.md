---
name: arc:audit
description: "项目体检与七维技术审计：基于证据输出诊断报告、评分边界与改进建议；当用户说“technical audit/health check/技术尽调/代码审计/架构评审”时触发。"
version: 1.0.0
allowed_tools:
  - Bash
  - Read
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

# arc:audit -- evidence-first technical audit

## Overview

`arc:audit` performs evidence-first audits for repositories, subsystems, and high-risk pull requests. It reviews seven dimensions, records only what is directly observable, and returns a bounded assessment rather than a presentation-first package.

Default outputs stay narrow:
- `diagnostic-report.md`
- `scorecard.md`
- `recommendations.md`
- `evidence-registry.md`

Two specialist indices stay conditional:
- **Business Maturity Index**: score only when core business flows or operating artifacts are observable.
- **Dependency Health Score**: score only when dependency versions, vulnerability signals, and maintenance status are observable.

`arc:audit` keeps **observations** separate from **recommendations**, stays read-only by default, and hands merge/release decisions to `arc:gate` and implementation work to `arc:build`.

## Quick Contract

- **Trigger**: A repository or change set needs an evidence-based audit, baseline diagnosis, or improvement route.
- **Inputs**: `project_path`, optional scope, depth, focus areas, and known business flows.
- **Outputs**: `diagnostic-report.md`, `scorecard.md`, `recommendations.md`, `evidence-registry.md`.
- **Optional Outputs**: `business-maturity.md`, `dependency-health.md`, and `quantitative-dashboard.html` only when the user explicitly asks for presentation deliverables, or when `depth_level=deep` and a showcase artifact clearly adds value.
- **Quality Gate**: Every scored conclusion must satisfy the evidence and boundary rules in `## Quality Gates`.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting-started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** in `## When to Use` prevails.

## Announce

Begin by stating clearly:
> "I am using `arc:audit` to collect evidence first, then produce a bounded seven-dimensional audit with clear findings, score limits, and recommendations."

## Teaming Requirement

- Every execution should clarify `Owner`, `Executor`, and `Reviewer` responsibilities.
- In a single-agent environment, keep an explicit decision-execution-review perspective before finalizing conclusions.

## The Iron Law

```text
NO SCORE WITHOUT DIRECT EVIDENCE.
USE N/A WHEN EVIDENCE IS INSUFFICIENT.
KEEP OBSERVATIONS SEPARATE FROM RECOMMENDATIONS.
```

Additional specialist constraints:
- No `Business Maturity Index` without observable business-flow or operating evidence.
- No `Dependency Health Score` without dependency/version/vulnerability/maintenance evidence.

## Workflow

1. Read project context, define scope, and choose the audit depth.
2. Collect repository evidence and build `evidence-registry.md`.
3. Evaluate the seven dimensions and score only where evidence supports it; otherwise use `N/A`.
4. Produce `diagnostic-report.md`, `scorecard.md`, and `recommendations.md`.
5. Add rebuttal artifacts or `quantitative-dashboard.html` only when the chosen depth and delivery goal justify them.

### Depth Guidance

| Depth | Default Path | Recommended Use |
|---|---|---|
| `quick` | Focused evidence pass on the highest-risk areas; no mandatory multi-agent review or dashboard | Fast baseline, PR spot check, narrow concern audit |
| `standard` | Default repository audit with evidence registry, bounded scoring, and prioritized recommendations | Most repository health checks and due-diligence requests |
| `deep` | May add tripartite panels, cross-rebuttal, and presentation deliverables when they add decision value | Board-level review, high-risk programs, or audits that explicitly require presentation artifacts |

### Deep Mode Recommendation

The tripartite pattern (architecture / engineering / business perspectives) and cross-rebuttal mechanism are **recommended for `depth_level=deep`**, not a mandatory baseline for `quick` or `standard` runs.

## Quality Gates

- Every finding must anchor to a verifiable source such as `file:line`, config path, dependency manifest, CI artifact, or documented process artifact.
- A score is allowed only when the supporting evidence is direct enough to justify a bounded judgment.
- When evidence is missing, conflicting, or outside the audit surface, use `N/A` instead of forced precision.
- Every `N/A` entry must include `missing_reason`, `missing_evidence_type`, and `how_to_collect_more_evidence`.
- `Business Maturity Index` must state which business flows or operating artifacts were inspected.
- `Dependency Health Score` must state which version, vulnerability, and maintenance signals were inspected.
- `diagnostic-report.md` must separate **Observed Facts** from **Recommendations**.
- The audit must remain read-only outside the local audit artifact directory.
- Scorecards, short risk matrices, and compact status summaries delivered in chat should prefer `terminal-table-output`; the audit artifact set on disk keeps its native Markdown/HTML formats.

## Expert Standards

- Seven-dimensional outputs should follow `finding + evidence + score_or_NA + confidence`.
- Scores are bounded judgments, not exact truth claims; say explicitly when the repository surface is incomplete.
- `Business Maturity Index` should describe observed flow coverage, breakpoints, manual fallback signs, and operating-loop evidence.
- `Dependency Health Score` should describe outdated versions, known vulnerabilities, maintenance activity, and upgrade automation signals.
- **Dependency License Risk Analysis (依赖开源合规风险分析)**: You MUST scan all dependency manifests (e.g., package.json, requirements.txt, go.mod, etc.) and generate a dedicated analysis. Explicitly identify any libraries with restrictive licenses, especially those that **prohibit commercial use** (e.g., CC-BY-NC, certain Commons Clause variants) or **viral copyleft licenses** that force the commercial project to be open-sourced (e.g., GPL, AGPL). Detail the risk level and mitigation plan for each.
- If an audit needs an executive surface, keep a compact `专家评审卡` and ensure the `9 Tab` view maps back to findings, evidence, and score boundaries.
- Recommendations should be prioritized (`P0`-`P3`) and explain expected value, delivery cost, and migration risk when material.
- **Actionable Remediation (闭环修复)**: For every recommendation, you MUST provide actionable remediation. For simple issues (e.g. linting, deprecated APIs, unused variables), provide a **Quick Fix** script (bash/python snippet). For complex issues (e.g. refactoring, architecture), provide a **Dispatch Command** (e.g., `arc build ...` or `arc fix ...` with precise parameters) so the user can copy-paste it to trigger the fix immediately.
- `quantitative-dashboard.html` is optional. If produced, it should follow `references/spa-dashboard-spec.md` and `templates/dashboard-template.html`, but it must not redefine the default audit contract.
- If a dashboard is produced, it may read final audit artifacts such as `scorecard.md`, `diagnostic-report.md`, and `recommendations.md` in addition to quantitative inputs.
- Dashboard KPI cards and charts must explicitly degrade to `Derived`, `Heuristic`, or `N/A` when direct evidence is insufficient; do not present fabricated precision.
- If the user needs a merge/release decision, hand off to `arc:gate`. If the user needs implementation work, hand off to `arc:build`.

## Scripts & Commands

- Runtime main command: `arc audit`
- Optional dashboard integration: `python3 Arc/arc:audit/scripts/integrate_score.py --project-path <project_path> --review-dir <review_dir>`
- Dashboard template reference: `Arc/arc:audit/templates/dashboard-template.html`

## Red Flags

- Scoring a dimension without direct evidence.
- Turning evidence gaps into fake precision instead of `N/A`.
- Treating the tripartite rebuttal flow as mandatory for every audit.
- Treating `quantitative-dashboard.html` as a required baseline artifact.
- Mixing observed facts and remediation advice in the same section.
- Using `arc:audit` as a substitute for `arc:gate` or `arc:build`.
- Modifying repository source code during the audit.

## When to Use

- **Preferred Trigger**: A repository, subsystem, or pull request needs a structured health check, technical due diligence, or evidence-based improvement route.
- **Typical Scenarios**: Architecture baseline before refactoring, pre-release technical due diligence, multi-dimensional risk review, evidence-first repository health check.
- **Boundary Note**: Use `arc:gate` for explicit Go/No-Go decisions. Use `arc:build` for code changes and implementation delivery.

## Expected Input Arguments

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_path` | string | **YES** | Absolute path to the target repository root |
| `project_name` | string | NO | Logical name of the project; infer if omitted |
| `scope_dimensions` | array | NO | Target dimensions; default is all 7 |
| `depth_level` | string | NO | `quick` / `standard` / `deep`, default `standard` |
| `focus_areas` | array | NO | Areas to inspect more deeply, such as `security` or `tech-debt` |
| `business_flow_catalog` | array | NO | Known core business flows used to evaluate business observability |
| `output_dir` | string | NO | Default: `<project_path>/.arc/audit/` |
| `require_dashboard` | boolean | NO | Force dashboard generation when presentation output is explicitly needed |

## Dependencies

- **Orchestration Contract** (recommended): `docs/orchestration-contract.md`
- **Semantic Retrieval MCP** (recommended): Use a retrieval-capable tool to locate code structure, dependency files, and evidence anchors efficiently.
- **External Research MCP** (optional): Use for dependency lifecycle, CVE, or ecosystem status checks when needed.

## Core Rules

1. **Read-only by default**: The audit does not change source code.
2. **Evidence first**: Quote only the smallest useful evidence fragment and keep references traceable.
3. **Bounded scoring**: Prefer `N/A` over unsupported numbers.
4. **Specialist indices are conditional**: Score them only when their evidence preconditions are met.
5. **Recommendation discipline**: Recommendations are downstream of observations, not replacements for them.
6. **Cost-aware delivery**: Keep `quick` and `standard` lightweight; reserve high-cost adversarial review and dashboards for `deep` or explicit presentation requests.

## Seven-Dimensional Assessment Framework

Refer to `references/dimensions.md` for dimension criteria and specialist index rules. Use `references/spa-dashboard-spec.md` only when producing the optional HTML dashboard.

| # | Dimension | Slug | Primary Question |
|---|-----------|------|------------------|
| 1 | Architectural Design & Longevity | architecture | Can the structure evolve without excessive coupling or brittle boundaries? |
| 2 | Security Posture & Access Control | security | Are authentication, authorization, and common exploit boundaries implemented defensibly? |
| 3 | Code Quality & Engineering Discipline | code-quality | Is the codebase maintainable, testable, and consistent enough for safe change? |
| 4 | Business Value & Flow Observability | business | Do observable flows and artifacts show that core business paths are actually supported? |
| 5 | Observability & Delivery Operations | devops | Can the system be observed, deployed, and recovered with reasonable confidence? |
| 6 | Team Collaboration & Knowledge Flow | team | Does the engineering workflow reduce concentration risk and improve maintainability? |
| 7 | Technical Debt & Dependency Risk | tech-debt | What current design and dependency choices increase future change cost or operational risk? |

## Output Artifact Topology

```text
<workdir>/.arc/audit/<project-name>/
├── context/
│   └── project-snapshot.md          # Optional context snapshot
├── diagnostic-report.md             # Required: findings and boundaries
├── scorecard.md                     # Required: dimension scores or N/A
├── recommendations.md               # Required: prioritized actions
├── evidence-registry.md             # Required: evidence index and trace map
├── license-risk-analysis.md         # Required: dependency license compliance, viral/non-commercial risk analysis
├── business-maturity.md             # Optional: only when evidence supports specialist scoring
├── dependency-health.md             # Optional: only when evidence supports specialist scoring
├── quantitative-dashboard.html      # Optional: presentation deliverable only
├── architecture/                    # Optional: deep-mode panel artifacts
├── deep/                            # Optional: deep-mode panel artifacts
└── deep-business/                   # Optional: deep-mode panel artifacts
```

## Fault Tolerance & Degradation Policies

| Condition | Handling |
|-----------|----------|
| Missing business evidence | Mark business specialist score `N/A`, explain the gap, and provide evidence collection steps |
| Missing dependency evidence | Mark dependency specialist score `N/A`, explain the gap, and provide evidence collection steps |
| Tight time budget | Prefer `quick` or `standard`; skip rebuttal and dashboard generation |
| Retrieval tooling unavailable | Fall back to manual repository inspection, but keep the same evidence rules |
| Presentation not requested | Do not generate `quantitative-dashboard.html` |

## Gotchas

Real failures from prior sessions, in order of frequency:

- **Missing evidence turned into N/A silently.** Do not just write N/A; explain the gap and how to collect evidence.
- **Score assigned without evidence.** The Iron Law is "NO SCORE WITHOUT DIRECT EVIDENCE". Never guess scores based on vibes.
- **Scored specialist index without business flows.** "Business Maturity Index" requires observable flows. If missing, mark N/A.

## Sign-off

```text
files changed:    N (+X -Y)
scope:            on target / drift: [what]
hard stops:       N found, N fixed, N deferred
signals:          N noted
verification:     [audit complete] → pass / fail
```
