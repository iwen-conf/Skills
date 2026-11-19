---
name: arc-plan
version: 3.0.0
description: >-
  Turns intent into frozen contracts, write-set isolated concurrent DAGs (plan-dag.yaml) and
  checkable closure models without editing code: PRD or blueprint (写PRD, 立项, 需求文档), clarify
  vague requests with acceptance criteria (需求澄清, 验收标准), business closure models and gate
  (业务闭环, 状态机, 事件联动, 闭环检查, 闭环闸门), API contracts, and tasks (拆任务, 任务文档, 进度跟踪).
  Not for code edits (use arc-code), review (use arc-audit), or tests (use arc-verify).
---

# arc-plan

Compile intent into frozen contracts, verified closure models and a write-set isolated `plan-dag.yaml` for parallel multi-agent execution; touch zero source/test files.

## Non-trigger

- Source/test/config edits → `arc-code`; verification/storylines → `arc-verify`; code audit → `arc-audit`; code lookup → `arc-explore`; Lark sync → `lark` (project-docs).

## Goal

Freeze contracts first, prove model closure when lifecycles change, and emit a validated `plan-dag.yaml` whose parallel waves have pairwise disjoint `write_scope` and runnable `verify_cmd`.

## Inputs

- `intent` (required); `mode` (`dag|brief|prd|model|tasks|campaign|layout`); `scope_tokens` (`只写文档|只读|Hold|P4|明确不做`).

## Intent Router

| Condition | Mode | Read / Run |
|---|---|---|
| Multi-module or multi-agent implementation (拆任务, 并发开发) | `dag` | [plan-dag.md](references/plan-dag.md); `dag_check.py` + `dag_dispatch.py` |
| Lifecycle states, events, compensation or race checks (业务闭环, 状态机) | `model` | [closure-design.md](references/closure-design.md), [closure-spec-format.md](references/closure-spec-format.md) |
| Read-only lifecycle gap review (闭环检查) | `model` (review) | [closure-review.md](references/closure-review.md), [closure-loopholes.md](references/closure-loopholes.md) |
| Vague request needing goal, constraints & acceptance (需求澄清) | `brief` | [brief.md](references/brief.md) |
| Product PRD or blueprint (写PRD, 立项) | `prd` | [prd.md](references/prd.md) |
| Persistent Markdown task cards or docs layout (任务文档, 进度跟踪) | `tasks` / `layout` | [task-docs.md](references/task-docs.md), [docs-layout.md](references/docs-layout.md) |
| Audit findings to remediation batches | `campaign` | [finding-campaign.md](../_shared/finding-campaign.md) |

## Workflow

1. Read [contract.md](../_shared/contract.md), [code-intel.md](../_shared/code-intel.md) and [execution-truth.md](../../docs/execution-truth.md) §2–§3; record baseline `git status --porcelain`.
2. Run `arc idx skeleton` and `arc idx impact <entity>` to ground every path, symbol, table and async topic in real code.
3. Classify unknowns into Facts, labeled Assumptions, and ≤3 blocking Questions (each with a default).
4. Execute matched mode:
   - `dag`: freeze API/DB contracts under `frozen_contracts` ([api-contract.md](../_shared/api-contract.md)), author `plan-dag.yaml`, validate via `python3 <skill-dir>/scripts/dag/dag_check.py plan-dag.yaml --max-errors 5`, and prepare worktrees via `dag_dispatch.py prepare plan-dag.yaml --wave 0`.
   - `model`: author `03-closure-spec.yaml` and pass both `python3 <skill-dir>/scripts/closure/check_closure.py check <spec> --json --max-errors 5` and `gate <spec>`.
   - `brief` / `prd` / `tasks`: write verifiable acceptance commands (zero subjective wording) at the docs-layout path.
5. Hand off to `arc-code` (with validated `plan-dag.yaml` / `.arc-task.json`) or `arc-verify` (for runtime storylines).

## Tool Strategy

- **Preferred**: `arc idx impact/skeleton`, `scripts/dag/dag_check.py`, `scripts/dag/dag_dispatch.py`, `scripts/closure/check_closure.py`.
- **Forbidden**: editing production source/tests/migrations, inventing symbol/table/key names, overlapping `write_scope` in the same parallel wave, using `--allow-warn` to open a closure gate, or using serial Markdown logs as a multi-agent scheduler.

## Decision Rules

- IF T0/T1 single-session work → return an inline brief; write no files unless asked.
- IF two tasks modify the same file → add an explicit `depends_on` edge or extract shared interfaces into `frozen_contracts`; never allow parallel write collisions.
- IF `dag_check.py` or `check_closure.py gate` exits non-zero → fix the spec/DAG before handing off to `arc-code`.
- IF a requirement touches auth, money, schema or external interfaces with unresolved ambiguity → ask (≤3 questions) and block dependent nodes.

## Verification

| Claim | Evidence |
|---|---|
| Parallel DAG safe | `dag_check.py plan-dag.yaml` exits 0 (`status: PASS`, waves listed) |
| Closure model sound | `check_closure.py check` + `gate` exit 0 with current SHA fingerprints |
| Docs-only authority | `git status --porcelain` changed only under docs/plan paths |

## Failure Recovery

| Failure | Action | Stop |
|---|---|---|
| `PARALLEL_WRITE_COLLISION` / `FROZEN_CONTRACT_OVERLAP` | Split `write_scope` or add `depends_on` edge | Re-run `dag_check.py` until PASS |
| Closure checker errors/truncation | Fix spec from source rules or narrow scope (≤2 passes) | Gate stays closed; block `arc-code` |
| Symbol/table not found | Widen via `arc idx` ladder | Mark as blocking Question; never guess |

## Completion Criteria

- [ ] Contracts frozen and grounded in `arc idx` evidence; zero source/test edits made.
- [ ] `dag_check.py` PASS (for `dag` mode) or `check_closure.py gate` PASS (for `model` mode).
- [ ] Every acceptance criterion is an executable command or observable assertion; next owner named.
