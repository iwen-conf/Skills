---
name: arc:audit
description: "项目体检：只读评估代码质量、架构风险与测试缺口，输出证据化发现与改进建议。"
---

# arc:audit

## Overview

`arc:audit` is a read-only review skill. It identifies risks, evidence, and pragmatic improvements. It does not generate dashboards, scores, release gates, or implementation patches by itself.

## Quick Contract

- **Trigger**: The user asks for code review, health check, technical audit, architecture review, or risk assessment.
- **Inputs**: Project path, review scope, risk focus, and optional changed files.
- **Outputs**: Findings ordered by severity, evidence references, and actionable recommendations.
- **Quality Gate**: Each finding is backed by concrete evidence or explicitly marked as an assumption.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:build` if the user wants changes implemented after the audit.
- Use `arc:fix` if a concrete failure is already known.
- Use `arc:clarify` if audit scope is too vague.
- Use external domain skills for specialized audits outside engineering quality.

## Context Search

- Use `.ai-code-index/search.sh "query"` as the first choice for relevant code paths, architecture boundaries, tests, and dependency usage.
- If the index is missing or stale, run `.ai-code-index/reindex.sh`.
- Use `.ai-code-index/struct-search.sh` for risky code shapes and `.ai-code-index/symbols.sh` for definitions or symbol inventory.
- Use `rg` only for narrow exact follow-up, new files, non-indexed files, or fallback when the index is insufficient.

## Announce

Begin by stating clearly:
"I am using `arc:audit` to perform a read-only, evidence-backed project review."

## The Iron Law

```text
NO FINDING WITHOUT EVIDENCE.
```

## Workflow

1. Confirm review scope and constraints.
2. Inspect structure, dependencies, critical paths, tests, and recent changes.
3. Identify findings with severity and file references.
4. Separate confirmed issues from residual risks and assumptions.
5. Recommend the smallest useful remediation path.

## Code Rot Gates

Full catalog: [`docs/code-rot-taxonomy.md`](../../docs/code-rot-taxonomy.md). Use all 36 items as the review rubric — this skill owns the complete catalog. Walk the six families and produce evidence-backed findings per family:

- A — Convention drift: naming (`phone`/`mobile`), error-code type, JSON nesting, Redis keys, pagination, timezone, logging spec (#1,2,8,16,18,19,22,24,33).
- B — Redundancy & dead code: unused symbols/deps, competing logging libs, duplicate endpoints, re-implemented helpers, over-design (#9,14,15,17,20,21,23,32).
- C — Security: horizontal/vertical authz, brute-force protection, predictable RNG, hardcoded backdoors, zero-amount purchase (#6,12,28,29,30,31).
- D — Data layer: soft-delete consistency, N+1/slow queries, JSONB misuse, raw-SQL/ORM mixing, unbounded lists, unindexed `LIKE` (#3,5,10,11,24,25,26).
- E — Error & state: scattered status codes, unstable state machines, swallowed exceptions, races (#4,7,13,27).
- F — AI execution integrity: half-finished refactors reported as done, leftover placeholders, build left broken (#34,35,36).

Report each finding with a file/line reference or command output; mark anything inferred as an assumption.

## Quality Gates

- Findings are ordered by severity.
- Each finding includes a file, command output, config, or behavioral evidence when possible.
- Recommendations are actionable and scoped.
- The audit remains read-only unless the user explicitly asks for implementation.

## Expert Standards

- Business impact is described with `Business Maturity` where relevant.
- Dependency and maintenance risks include `Dependency Health` considerations.
- Significant findings can be summarized as an `Expert Review Card`.
- If scoring is useful, keep it lightweight and avoid pretending precision; a compact `9 Tab` style summary is acceptable only when evidence supports it.

## Scripts & Commands

No dedicated Arc runtime scripts. Use `.ai-code-index/` for repository context search, then repository-native inspection commands, tests, package managers, and static analysis tools when available.

## Red Flags

- Producing generic advice without inspecting the project.
- Treating preferences as defects.
- Hiding uncertainty behind numeric scores.
- Implementing fixes during a read-only audit without permission.
- Reporting only style nits while missing a family-C security finding (authz, 0元购, backdoor) (#12,29,31).

## When to Use

- **Preferred Trigger**: The user asks whether a codebase, PR, architecture, or module is healthy or risky.
- **Typical Scenario**: Pre-refactor assessment, takeover review, release readiness discussion, or quality gap analysis.
- **Boundary Tip**: Use `arc:build` for implementation and `arc:fix` for known failures.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `scope` | string | no | Whole project, module, PR, or specific concern |
| `risk_focus` | string | no | Architecture, security, tests, dependencies, performance, maintainability |

## Outputs

```text
Audit Report
- Findings by severity
- Evidence
- Impact
- Recommendation
- Residual risks
```
