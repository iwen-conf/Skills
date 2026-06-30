---
name: arc:audit
description: "Read-only project audit; hand active Lark findings, risks, remediation tasks, approvals, and lifecycle to arc:docs."
---

# arc:audit

## Overview

`arc:audit` performs read-only engineering review. It reports evidence-backed risks and recommendations only; it does not patch code or create Lark resources directly.

## Quick Contract

- **Trigger**: The user asks for review, health check, risk assessment, architecture review, or release readiness.
- **Inputs**: Project path, scope, risk focus, optional changed files.
- **Outputs**: Severity-ordered findings, evidence, impact, recommendations, and optional Lark handoff.
- **Quality Gate**: Every finding has concrete evidence or is explicitly marked as an assumption.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` if audit scope is vague.
- Use `arc:fix` if a concrete failure is already known.
- Use `arc:task-doc-progress-conventions` before remediation when audit findings become large, multi-step, cross-module, or tracked implementation work.
- Use `arc:build` only if the user asks to implement fixes.
- Use `arc:docs` only when Lark is active for audit reports, risk Base rows, remediation tasks, approval gates, or `.lark.json.lifecycle[]`.

## Context Search

- MUST use `.ai-code-index/search.sh` first for relevant code paths, tests, dependencies, and architecture boundaries.
- MUST use `.ai-code-index/struct-search.sh` for risky code shapes when relevant.
- If `.lark.json` exists, MUST read it before reporting prior risks, tasks, approvals, and audit records.

## Announce

Begin by stating clearly:
"I am using `arc:audit` to perform a read-only, evidence-backed project review."

## The Iron Law

```text
NO FINDING WITHOUT EVIDENCE.
NO CODE EDIT DURING AUDIT.
NO LARK AUDIT UPDATE OUTSIDE arc:docs.
```

## Hard Constraints

- MUST remain read-only unless the user explicitly changes the task to implementation.
- MUST inspect the project before giving findings.
- MUST order findings by severity.
- MUST include file path, command output, config, behavior, or other evidence for each confirmed issue.
- MUST check frontend platform stack drift against `arc:frontend` when relevant: Web = React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod; mobile = React Native + Expo + TypeScript + NativeWind + Zustand + TanStack Query + Expo Router; desktop = Tauri 2 + Web stack; mini-program = Taro 4 + React + TypeScript + Zustand, unless an explicit project exception exists.
- MUST mark inferred risks as assumptions.
- MUST route all Lark audit/risk/task/approval updates through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST route multi-finding or multi-step remediation planning through `arc:task-doc-progress-conventions` before execution starts.
- NEVER present preferences as defects.
- NEVER hide uncertainty behind numeric scores.

## Workflow

1. Confirm scope, constraints, and risk focus.
2. Inspect structure, dependencies, critical paths, tests, and recent changes.
3. Check the 36-item code-rot taxonomy in [`docs/code-rot-taxonomy.md`](../../docs/code-rot-taxonomy.md).
4. Produce severity-ordered findings with evidence and recommended action.
5. If findings become large or tracked remediation work, hand off to `arc:task-doc-progress-conventions` before `arc:build` or `arc:fix`.
6. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with findings, risk rows, tasks, and approval needs.

## Quality Gates

- Findings lead the report; summary is secondary.
- Each confirmed finding has concrete evidence.
- Recommendations are scoped and actionable.
- Large remediation follow-up names `arc:task-doc-progress-conventions` as the required local planning gate.
- Security, data-layer, state, dependency, and test risks are considered when relevant.
- Frontend audits check same-duty library duplication, state-layer mixing, and undocumented default-stack exceptions when relevant.
- Lark audit state is linked through `.lark.json` only when Lark is active.

## Expert Standards

- Business impact is described with `Business Maturity` when relevant.
- Dependency risk uses `Dependency Health`.
- Major findings can be summarized as an `Expert Review Card`.
- If scoring is useful, use a compact `9 Tab` summary only when evidence supports it.

## Scripts & Commands

No dedicated runtime scripts. Use `.ai-code-index/`, project-native inspection commands, tests, package managers, and static analysis tools.

## Red Flags

- Generic advice without inspection.
- Style nits hiding security, data, state, or test risk.
- Fixing code during read-only review.
- Turning audit findings into broad remediation work without current local task docs.
- Audit report created in Lark but missing from `.lark.json`.

## When to Use

- **Preferred Trigger**: The user asks whether a codebase, PR, architecture, or module is healthy or risky.
- **Typical Scenario**: Pre-refactor assessment, takeover review, release readiness, or quality gap analysis.
- **Boundary Tip**: Use `arc:build` for implementation and `arc:fix` for known failures.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `scope` | string | no | Whole project, module, PR, or concern |
| `risk_focus` | string | no | Architecture, security, tests, dependencies, performance, maintainability |

## Outputs

```text
Audit Report
- Findings by severity
- Evidence
- Impact
- Recommendation
- Residual risks
- Lark / .lark.json handoff, if applicable
```
