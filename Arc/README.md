# Arc

Arc is the cross-project software-engineering family: five skills, one per kind of work. Each
skill is self-sufficient for its phase; skills never load each other, they hand off by name.

## Routing

| Work | Skill | Modes | Authority |
|---|---|---|---|
| Understand code: where, who calls, what a change touches, architecture map, index upkeep | [`arc:explore`](arc:explore/SKILL.md) | locate · trace · impact · map · index-ops | read-only |
| Turn intent into specs: PRD, brief with acceptance criteria, task documents, progress, finding campaigns | [`arc:plan`](arc:plan/SKILL.md) | prd · brief · tasks · layout · campaign | writes `docs/` only |
| Change source code: implement, repair, refactor/migrate, frontend, comments, timing probes | [`arc:code`](arc:code/SKILL.md) | implement · repair · refactor · frontend · comment · probe | edits source |
| Produce behavior evidence: tests, reliability, PostgreSQL qualification, flaky triage, release gates | [`arc:verify`](arc:verify/SKILL.md) | design · write · reliability · db · flaky · release | edits tests, runs checks |
| Evaluate read-only: diff/PR review, health and code rot, AppSec, scans | [`arc:audit`](arc:audit/SKILL.md) | review · health · appsec · scan | read-only, reports only |

Lark project workspaces (`.lark.json`, sync to Lark docs/base/wiki) live in the Lark family:
[`lark-workflow-project-docs`](../Lark/lark-workflow-project-docs/SKILL.md).

Typical sequences: `explore → code → verify` for a change; `plan → code → verify` for a T2
feature; `audit → plan (campaign) → code → verify` for a findings campaign.

## Shared layer

[`_shared/`](_shared/) holds rules several skills apply. It has no `SKILL.md` and is never
discovered; the packager copies each linked file into the skills that reference it.

| File | Content |
|---|---|
| [contract.md](_shared/contract.md) | authority, task tiers, scope lock, vocabulary, artifacts, environment, report format |
| [code-intel.md](_shared/code-intel.md) | arc-idx preflight, query ladder, 8-surface impact checklist, fallback rules |
| [code-rot.md](_shared/code-rot.md) | 36 code-rot smells in families A–F |
| [api-contract.md](_shared/api-contract.md) | Markdown API contract rules |
| [finding-campaign.md](_shared/finding-campaign.md) | findings → task batches |
| [conventions/](_shared/conventions/) | Go backend and Go type safety, frontend, color tokens, comment templates |

Repository-wide gates (completion, scope tokens, root-cause repair, simplicity, surgical edits)
stay in [`docs/execution-truth.md`](../docs/execution-truth.md).

## Retired names (2.0.0)

No aliases are installed; use the new name.

| Old | New |
|---|---|
| `arc:idx` | `arc:explore` |
| `arc:define`, `arc:clarify`, `arc:sdlc` | `arc:plan` (prd, brief, tasks/layout/campaign) |
| `arc:build`, `arc:fix`, `arc:frontend`, `arc:arch`, `arc:comment`, `arc:trace` | `arc:code` (implement, repair, frontend, conventions, comment, probe) |
| `arc:test`, `arc:reliability`, `arc:dbqual`, `reliability-engineering-skill` | `arc:verify` (design/write, reliability, db, release) |
| `arc:audit`, `arc:security` | `arc:audit` (health/appsec, scan; new review mode) |
| `arc:docs` | `lark-workflow-project-docs` |
| `arc:prewalk` | removed (design record: [`docs/01-系统设计/prewalk.md`](../docs/01-系统设计/prewalk.md)) |

Design rationale: [ARC 技能架构重构评审](../docs/01-系统设计/01-ARC技能架构重构评审.md).
