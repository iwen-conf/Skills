# Arc

Arc is the cross-project software-engineering family: six skills sharing one
[workflow and handoff](_shared/workflow.md), read only when a phase hands off or a closure spec exists. Each phase has one owner; authorized work continues
across phases with the same scope, requirement IDs, model bindings and evidence.

Client direction is [native platforms](_shared/conventions/native-platforms.md): target SDKs,
UI and system behavior take priority; cross-platform UI reuse is no longer the goal.

## Routing

| Work | Skill | Modes | Authority |
|---|---|---|---|
| Understand code: where, who calls, what a change touches, architecture map, index upkeep | [`arc:explore`](arc:explore/SKILL.md) | locate · trace · impact · map · index-ops | read-only |
| Turn intent into specs and checkable models: PRD, brief, business closure model and gate, task documents, progress, finding campaigns | [`arc:plan`](arc:plan/SKILL.md) | prd · brief · model · tasks · layout · campaign | writes `docs/` only; model review writes artifacts only |
| Change source code: implement, repair, refactor/migrate, frontend, comments, timing probes | [`arc:code`](arc:code/SKILL.md) | implement · repair · refactor · frontend · comment · probe | edits source |
| Produce behavior evidence: business-flow catalog and branch ledger, test-first acceptance tests, test-value review, browser E2E, fuzz campaigns, reliability, PostgreSQL qualification, flaky triage, release gates | [`arc:verify`](arc:verify/SKILL.md) | flow · design · red · write · reliability · db · flaky · release | edits tests and the flow catalog, runs checks |
| Evaluate read-only: HTTP API design, diff/PR review, health/code rot/over-engineering, AppSec, scans | [`arc:audit`](arc:audit/SKILL.md) | api-design · review · health · appsec · scan | read-only, reports only |
| Choose before depending: open-source due diligence, build vs buy, stale or relicensed dependency replacement | [`arc:vet`](arc:vet/SKILL.md) | select · check · replace | writes the selection record and artifacts only |

Lark project workspaces (`.lark.json`, sync to Lark docs/base/wiki) live in the [`lark`](../Lark/lark/SKILL.md)
router's project-docs entry. Delivery (commit, push, PR, release) is `check`; visual direction without code is
`ui-design`.

Typical sequences: `explore → code → verify` for a change (verify starts from the affected business flows);
`plan (prd → model → tasks) → code → verify` for a new business lifecycle;
`plan (model review) → plan (campaign) → code → verify` for business findings;
`vet → code → verify` for a new dependency.
Skip phases with current usable outputs. A closure model gate proves model checks only;
implementation and release claims still require their own runtime evidence.

## Shared layer

[`_shared/`](_shared/) holds rules several skills apply. It has no `SKILL.md` and is never
discovered; the packager copies each linked file into the skills that reference it.

| File | Content |
|---|---|
| [contract.md](_shared/contract.md) | authority, task tiers, scope lock, vocabulary, artifacts, environment, report format |
| [workflow.md](_shared/workflow.md) | bidirectional handoffs, artifact ownership, model-to-runtime traceability, freshness and authorization |
| [code-intel.md](_shared/code-intel.md) | arc-idx preflight, query ladder, 8-surface impact checklist, fallback rules |
| [coding-discipline.md](_shared/coding-discipline.md) | think before coding, simplicity ladder, surgical edits, paper-overs |
| [code-rot.md](_shared/code-rot.md) | 41 code-rot smells in families A–G (G: over-engineering) |
| [api-contract.md](_shared/api-contract.md) | Markdown API contract rules |
| [http-api-design.md](_shared/http-api-design.md) | sourced HTTP design checks; JSON fields, uploads, limits and evidence shared by plan/code/audit/verify |
| [finding-campaign.md](_shared/finding-campaign.md) | findings → task batches |
| [conventions/](_shared/conventions/) | Go backend (core + structure) and Go type safety, frontend, color tokens, comment templates |
| [native-platforms.md](_shared/conventions/native-platforms.md) | native target selection, legacy maintenance boundary and platform-specific acceptance |

Repository-wide gates (completion, scope tokens, domain keys, root-cause repair) stay in
[`docs/execution-truth.md`](../docs/execution-truth.md). Summed load per typical task is capped by
`ARC_LOAD_SCENARIOS` in `src/arc_core/domain/skill.py`.

## Retired names (2.0.0)

No aliases are installed; use the new name.

| Old | New |
|---|---|
| `arc:idx` | `arc:explore` |
| `arc:define`, `arc:clarify`, `arc:sdlc` | `arc:plan` (prd, brief, tasks/layout/campaign) |
| `arc:build`, `arc:fix`, `arc:frontend`, `arc:arch`, `arc:comment`, `arc:trace` | `arc:code` (implement, repair, frontend, conventions, comment, probe) |
| `arc:test`, `arc:reliability`, `arc:dbqual`, `reliability-engineering-skill` | `arc:verify` (design/write, reliability, db, release) |
| `arc:audit`, `arc:security` | `arc:audit` (health/appsec, scan; new review mode) |
| `arc:docs` | `lark` (project-docs entry) |
| `arc:closure` (2.5.0) | `arc:plan` model mode; specs under `docs/01-系统设计/业务闭环/` (legacy `docs/arc/` kept) |
| `arc:prewalk` | removed (design record: [`docs/01-系统设计/prewalk.md`](../docs/01-系统设计/prewalk.md)) |

Design rationale: [ARC 技能架构重构评审](../docs/01-系统设计/01-ARC技能架构重构评审.md) and
[全量已安装技能架构评审](../docs/01-系统设计/02-全量技能架构评审.md).

## Installation

```bash
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --dry-run
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --check
.venv/bin/python scripts/link_hosts.py --check
```

This installs `arc-explore`, `arc-plan`, `arc-code`, `arc-verify`, `arc-audit`, `arc-vet` under `~/.agents/skills`,
including shared references, the closure checker (`arc-plan/scripts/closure/`), the gate script and the
GitHub health precheck (`arc-vet/scripts/github_health.py`). Host
roots link to them per `schemas/host_profiles.yaml`.
