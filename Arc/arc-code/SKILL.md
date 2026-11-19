---
name: arc-code
version: 3.0.0
description: >-
  Every task that changes source code: implement features or endpoints (写代码, 实现, 开发功能, 落地改动),
  build frontend/UI pages (前端, shadcn, 小程序), fix bugs/errors/incidents by evidence-first root-cause
  repair (修复, 报错, 根因, incident), refactor (重构), apply Go DIP/usecase layering (DIP, usecase, 分层),
  write comments (注释), or add Gin SSR timing probes (Gin SSR, 计时, 探针, 埋点). Enforces write_scope
  isolation and atomic gate.sh checks. Not for review (use arc-audit), verification suites (use arc-verify),
  or planning (use arc-plan).
---

# arc-code

Implement or repair behavior on the real entry point inside the task's exclusive `write_scope`, passing atomic `gate.sh` checks with zero contract drift.

## Non-trigger

- Lookup/impact only → `arc-explore`; read-only audit → `arc-audit`; full test/storyline suites → `arc-verify`; PRD/DAG/closure → `arc-plan`; git/PR delivery → `check`.

## Goal

Deliver the smallest complete diff inside `write_scope` (never touching `frozen_contracts`) verified by red-first evidence and `gate.sh`.

## Inputs

- `request` (required); `.arc-task.json` (when dispatched from `plan-dag.yaml`: `frozen_contracts`, `read_scope`, `write_scope`, `verify_cmd`); `task_slug`.

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| New feature, endpoint, job or behavior | `implement` | Stack row below + [api-contract.md](../_shared/api-contract.md) when API changes |
| Bug, crash, failing test or incident (修复, 报错, 根因) | `repair` | [repair.md](references/repair.md) + evidence-first root-cause repair in [execution-truth.md](../../docs/execution-truth.md) §6 |
| Refactor, rename, move or schema migration (重构) | `refactor` | [refactor.md](references/refactor.md) |
| Web/mobile/mini-program UI (前端, shadcn, 小程序) | `frontend` | [frontend.md](../_shared/conventions/frontend.md), [frontend-tokens.md](../_shared/conventions/frontend-tokens.md) |
| Comments only (注释) / Gin SSR timing probes (计时, 探针) | `comment` / `probe` | [comments.md](../_shared/conventions/comments.md) / [perf-probes.md](references/perf-probes.md) |
| Go backend code (DIP, usecase, 分层) | adds to mode | [go-backend.md](../_shared/conventions/go-backend.md), [go-type-safety.md](../_shared/conventions/go-type-safety.md) |

## Workflow

1. Read [contract.md](../_shared/contract.md), [code-intel.md](../_shared/code-intel.md) and [coding-discipline.md](../_shared/coding-discipline.md). If `.arc-task.json` exists, lock onto its `read_scope`, `write_scope`, `frozen_contracts` and `verify_cmd`.
2. Run `arc idx skeleton` / `arc idx impact` on target symbols; for T1/T2 run `bash <skill-dir>/scripts/gate.sh start --task <slug>` before the first edit.
3. Prepare by mode:
   - `implement` / `repair`: write a requirement-derived failing test or reproducer and record it with `bash <skill-dir>/scripts/gate.sh red -- <cmd>` BEFORE editing production code ([test-first.md](../_shared/test-first.md)).
   - `refactor`: confirm green baseline & characterization tests first.
4. Edit strictly inside `write_scope` (never touch `frozen_contracts`): types → implementation → callers → tests.
5. Run atomic gates in order; fix any non-zero exit in place:
   - `bash <skill-dir>/scripts/gate.sh scope --json` (blocks out-of-scope or frozen-contract edits)
   - `bash <skill-dir>/scripts/gate.sh hygiene --json` (blocks placeholders, suppressions, sleep sync)
   - `bash <skill-dir>/scripts/gate.sh test --json` (runs `.arc-task.json` `verify_cmd`)
   - For full T1/T2 delivery: `bash <skill-dir>/scripts/gate.sh check --task <slug> --require-red`
6. Exercise the real entry point (HTTP/CLI/UI/handler) and report per [contract.md](../_shared/contract.md) §9.

## Tool Strategy

- **Preferred**: `arc idx skeleton/impact/search`, project compiler/test runner, atomic `gate.sh scope|hygiene|test|check`.
- **Forbidden**: touching `frozen_contracts`, editing outside `write_scope`, `--no-verify`, lint/type suppressions (`//nolint`, `@ts-ignore`, `# type: ignore`), weakened assertions, swallowed errors, or skipping `gate.sh red` in `implement`/`repair`.

## Decision Rules

- IF failure evidence exists (error/stack/failing test) → force `repair` mode and root-cause repair gate even if asked to "implement".
- IF `gate.sh scope` fails → revert out-of-scope edits or escalate to `arc-plan` to amend `plan-dag.yaml`; never bypass scope.
- IF writing stateful/money/inventory DB code → enforce transactions, optimistic locks or state-encoded `WHERE` + row-count verification.
- IF required gates and entry tests pass → stop; hand full multi-ending/E2E verification to `arc-verify`.

## Verification

| Claim | Evidence |
|---|---|
| Scope & frozen contracts respected | `gate.sh scope --json` exits 0 (`status: PASS`) |
| Test-first & clean hygiene | `gate.sh red` + `gate.sh hygiene --json` + `gate.sh check` PASS |
| Real entry works | Handler/CLI/HTTP test command & output cited |

## Failure Recovery

| Failure | Action | Stop |
|---|---|---|
| Atomic gate fails | Fix root cause (≤3 attempts per gate); never suppress | Report top-5 JSON errors |
| Repair hypothesis fails twice | Return to evidence-first root-cause step 3 with new hypothesis | 3rd failure → report falsified hypotheses |
| Missing toolchain | Mark `NOT_RUN(<tool>)`; ask before installing | Never report `PASS` |

## Completion Criteria

- [ ] Edits stayed strictly inside `write_scope`; zero `frozen_contracts` modified.
- [ ] `gate.sh red` recorded before production edits (`implement`/`repair`) and `gate.sh scope/hygiene/test/check` PASS.
- [ ] Real entry exercised; report follows [contract.md](../_shared/contract.md) §9.
