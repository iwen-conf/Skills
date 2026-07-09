---
name: arc:audit
description: "Read-only project/AppSec audit: assets, data map, vuln review; Lark risks via arc:docs."
---

# arc:audit

## Overview

`arc:audit` performs read-only engineering and application-security review. It reports evidence-backed risks and recommendations only; it does not patch code or create Lark resources directly.

Two review modes share one skill:

| Mode | Purpose |
|---|---|
| `general` | Architecture, maintainability, tests, dependency health, code-rot taxonomy |
| `appsec` | Cross-project vulnerability / project security audit: assets → data map → soft targets → finding cards |

For CLI scanners (SAST/SCA/secrets/DAST), hand off to `arc:security` after Phase 1 assets exist—or when the user explicitly wants automation-first.

AppSec methodology lives in [`references/appsec-playbook.md`](references/appsec-playbook.md).

## Quick Contract

- **Trigger**: The user asks for review, health check, risk assessment, architecture review, release readiness, code audit, vulnerability audit, AppSec review, or project security体检.
- **Inputs**: Project path, scope, mode (`general` / `appsec`), risk focus, optional changed files, authorized environment notes.
- **Outputs**: Severity-ordered findings with evidence; for `appsec`, also asset table, data map, top hypotheses, manual gaps; optional Lark handoff.
- **Quality Gate**: Every finding has concrete evidence or is explicitly marked as an assumption. AppSec findings include permission class and data yield.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` if audit scope, environment authorization, or active-test boundaries are vague.
- Use `arc:security` when the user wants local scanner automation, dependency/secrets CLI reports, API fuzz, or authorized DAST—after recon when possible.
- Use `arc:fix` if a concrete failure or confirmed vuln must be repaired.
- Use `arc:task-doc-progress-conventions` before remediation when findings become large, multi-step, cross-module, or tracked implementation work; as role `R-task` after emitting a Handoff Package (项目定位 / 项目口径 / 功能角色 / findings)—never dump a vague “修安全” task list.
- Use `arc:build` / `arc:fix` only after task docs exist for tracked remediation, or for a single small confirmed fix the user wants implemented immediately.
- Use `arc:docs` only when Lark is active for audit reports, risk Base rows, remediation tasks, approval gates, or `.lark.json.lifecycle[]`.

## Context Search

- MUST use `.ai-code-index/search.sh` first for relevant code paths, tests, dependencies, and architecture boundaries.
- MUST use `.ai-code-index/struct-search.sh` for risky code shapes when relevant (auth, SQL, uploads, SSRF, JWT, payments).
- MUST prefer inventory collectors (routers, OpenAPI, migrations, configs) over free-form AI “scan everything”.
- If `.lark.json` exists, MUST read it before reporting prior risks, tasks, approvals, and audit records.

## Announce

Begin by stating clearly:
"I am using `arc:audit` to perform a read-only, evidence-backed project review."

When `mode=appsec` or the user asked for vulnerability/security audit, also state:
"AppSec mode follows the asset → data-map → soft-target playbook; scanners stay in `arc:security` unless already requested."

## The Iron Law

```text
NO FINDING WITHOUT EVIDENCE.
NO CODE EDIT DURING AUDIT.
NO LARK AUDIT UPDATE OUTSIDE arc:docs.
NO APPSEC CLAIM WITHOUT PERMISSION CLASS AND DATA YIELD (OR EXPLICIT CAPABILITY-ONLY).
NO MASS AI DEEP-DIVE BEFORE ASSET INVENTORY.
```

## Hard Constraints

- MUST remain read-only unless the user explicitly changes the task to implementation.
- MUST inspect the project before giving findings.
- MUST order findings by severity (and for appsec, by data yield when severities tie).
- MUST include file path, command output, config, behavior, or other evidence for each confirmed issue.
- MUST check frontend platform stack drift against `arc:frontend` when relevant: Web = React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod; mobile = React Native + Expo + TypeScript + NativeWind + Zustand + TanStack Query + Expo Router; desktop = Tauri 2 + Web stack; mini-program = Taro 4 + React + TypeScript + Zustand, unless an explicit project exception exists.
- MUST mark inferred risks as assumptions.
- MUST route all Lark audit/risk/task/approval updates through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- MUST route multi-finding or multi-step remediation planning through `arc:task-doc-progress-conventions` before execution starts.
- MUST follow [`references/appsec-playbook.md`](references/appsec-playbook.md) when mode is `appsec` or risk_focus is security/vuln/appsec.
- MUST treat config/secret/default-admin/debug exposure as first-class risks, not afterthoughts.
- MUST NOT run active DAST, exploit payloads, or live attacks under `arc:audit`; authorize and route those to `arc:security`.
- MUST redact secrets in chat; keep raw sensitive evidence local when needed.
- NEVER present preferences as defects.
- NEVER hide uncertainty behind numeric scores.
- NEVER equate “server/admin foothold narrative” with audit value when bulk PII/payment/credential impact is unstated.

## Workflow

### Shared

1. Confirm scope, constraints, mode (`general` vs `appsec`), and risk focus.
2. Inspect structure, dependencies, critical paths, tests, and recent changes via local index tools.
3. Produce severity-ordered findings with evidence and recommended action.
4. If findings become large or tracked remediation work: as `R-recon`, write the Handoff Package (定位/口径/角色/assets/data-map/findings/task-seed), then hand to `arc:task-doc-progress-conventions` (`R-task`) before any `arc:build` / `arc:fix`. Pipeline: [`../arc:task-doc-progress-conventions/references/security-audit-task-pipeline.md`](../arc:task-doc-progress-conventions/references/security-audit-task-pipeline.md).
5. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with findings, risk rows, tasks, and approval needs.

### Mode `general`

1. Check the 36-item code-rot taxonomy in [`docs/code-rot-taxonomy.md`](../../docs/code-rot-taxonomy.md).
2. Cover architecture boundaries, Dependency Health, tests, state/data risks, and release readiness as relevant.
3. Use Expert Review Card / compact 9 Tab summary only when evidence supports scoring.

### Mode `appsec` (default when user asks 漏洞/安全/AppSec/代码审计 in a security sense)

Follow the playbook phases without skipping inventory:

1. **Phase 0 — Scope gate**: environments, active-test ban (default), protected paths.
2. **Phase 1 — Asset table**: APIs, SSR, admin, identity, storage, payments, integrations, config/ops, clients. AI may format/dedupe tables; tools/index collect surfaces.
3. **Phase 2 — Sensitive data map**: tag PII / AUTH / PAY / BIZ / SECRET; assign yield class L0–L4 and access paths.
4. **Phase 3 — Soft-target queue**: secrets/config first, then AuthN, AuthZ/IDOR, privileged APIs, upload, payment, injection, SSRF, frontend tokens, then dependency CVEs with reachability.
5. **Phase 4 — Verify read-only**: code/config reachability proofs; mark live checks as manual gaps or hand off to `arc:security`.
6. **Phase 5 — Finding cards**: every confirmed item uses permission class + data yield + evidence + fix direction (playbook template).
7. **Phase 6 — Bundle**: `assets` + `data-map` + top hypotheses + findings + manual gaps; optional `arc:security` handoff for automation.
8. **Phase 6b — Task handoff (when fix campaign follows)**: write `00-项目定位与口径.md` + `06-task-seed.md` with risk-domain groups only; do not author full implementation subtasks here—that is `R-task`.

Do not spend the whole engagement on low-yield host/RCE fantasies while bulk data paths remain unmapped.

## Quality Gates

- Findings lead the report; summary is secondary.
- Each confirmed finding has concrete evidence.
- Recommendations are scoped and actionable.
- Large remediation follow-up names `arc:task-doc-progress-conventions` as the required local planning gate.
- Security, data-layer, state, dependency, and test risks are considered when relevant.
- Frontend audits check same-duty library duplication, state-layer mixing, and undocumented default-stack exceptions when relevant.
- Lark audit state is linked through `.lark.json` only when Lark is active.
- AppSec mode includes asset inventory and sensitive data map before deep module claims.
- AppSec findings state permission class and data yield (or explicit capability-only).
- Manual AuthZ / payment / business-logic gaps are listed even when no bug is proven.
- Scanner-only statements are deferred to `arc:security` or labeled non-coverage.

## Expert Standards

- Business impact is described with `Business Maturity` when relevant.
- Dependency risk uses `Dependency Health`.
- Major findings can be summarized as an `Expert Review Card`.
- If scoring is useful, use a compact `9 Tab` summary only when evidence supports it.
- AppSec prioritization uses **data-value thinking**: bulk multi-field PII/credentials/payment integrity outranks isolated low-priv footholds with no data path.
- Use OWASP ASVS-style coverage questions for AuthN, AuthZ, session, files, and business logic without pretending scanners answered them.
- Treat half-implemented auth features (e.g. mounted reset-password without business logic) as soft targets.

## Scripts & Commands

No dedicated runtime scripts. Use:

- `.ai-code-index/search.sh`, `struct-search.sh`, `symbols.sh`
- project-native tests, linters, and package managers
- static inspection of routers, migrations, configs, deploy scripts
- `arc:security` scripts only after handoff / user request for automation

Suggested appsec collectors (adapt per stack):

```bash
# examples — prefer project-native equivalents
.ai-code-index/struct-search.sh 'raw SQL|ExecContext|Find\(|jwt|password|upload|webhook'
# list HTTP registrations, OpenAPI, permission matrices, *.yaml configs, deploy scripts
```

## Red Flags

- Generic advice without inspection.
- Style nits hiding security, data, state, or test risk.
- Fixing code during read-only review.
- Turning audit findings into broad remediation work without current local task docs.
- Audit report created in Lark but missing from `.lark.json`.
- AI whole-repo monologue with no asset table.
- CVE count theater without reachability or data yield.
- Reporting admin/server “win” without obtainable data or business impact.
- Running live exploits under audit mode.
- Skipping config, secrets, debug endpoints, and deploy artifacts.

## When to Use

- **Preferred Trigger**: The user asks whether a codebase, PR, architecture, or module is healthy or risky; or asks for code/project/vulnerability audit, AppSec review, or security体检 without requiring scanner installation.
- **Typical Scenario**: Pre-refactor assessment, takeover review, release readiness, quality gap analysis, or cross-project AppSec pass using the shared playbook.
- **Boundary Tip**: Use `arc:build` for implementation, `arc:fix` for known failures, and `arc:security` for local scanner/DAST automation. Use this skill for methodology-first read-only review.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `scope` | string | no | Whole project, module, PR, or concern |
| `mode` | enum | no | `general` (default for health/architecture) or `appsec` (default for vuln/security audit language) |
| `risk_focus` | string | no | Architecture, security, tests, dependencies, performance, maintainability, data, auth, config-deploy |
| `changed_files` | string | no | Optional PR or local diff focus |
| `environment` | string | no | local / test / staging / prod notes; prod active tests require explicit authorization |

## Outputs

```text
Audit Report
- Mode and scope
- Findings by severity (finding cards in appsec mode)
- Evidence
- Impact (permission class + data yield in appsec mode)
- Recommendation
- Residual risks
- Appsec bundle when applicable:
  - assets
  - data-map
  - top hypotheses
  - manual-gaps
  - optional arc:security handoff
  - Handoff Package for R-task when remediation is tracked:
    - 项目定位 / 项目口径 / 功能角色
    - findings + task-seed (not full subtasks)
- Lark / .lark.json handoff, if applicable
```
