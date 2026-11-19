---
name: arc-verify
version: 5.0.0
description: >-
  Full-stack verification via Branching Storyline Engine (world_flags, checkpoints, blind concurrent
  routes, anti_softlock, 100% multi-ending completion): 完整测试, 完整业务测试, 测试整个系统, 所有交互,
  业务图, 业务树图, 写测试, 测试方案, 用例设计, 测试范围, 覆盖率, 可靠性, 业务不变量, 事务并发, 幂等,
  PostgreSQL, 压测, 发布验收, 发布证据, 测试先行, 测试价值, 网页实测, fuzz, 模糊测试, SQL 语法检查,
  事务设计, 业务流, 端到端业务流, 分支覆盖, 属性测试, 模型一致性, flaky. Not for code repair (use arc-code)
  or read-only review (use arc-audit).
---

# arc-verify

Verify business systems via reverse-scanned `world_flags`, branching `checkpoints`, blind concurrent routes, `butterfly_probes`, `anti_softlock` guards and 100% multi-ending completion; preserve first FAIL forever.

## Non-trigger

- Production code fixes → `arc-code`; read-only audit/AppSec → `arc-audit`; code search → `arc-explore`; PRD/DAG/closure → `arc-plan`.

## Goal

Prove every coupled entity state, intermediate escape path and required business ending through real execution and independent probes — never stub tests from implementation `if` branches.

## Inputs

- `target` (required: entity/feature/service); `mode` (matched via Intent Router); `scope` (`plan-only|read-only|full`).

## Intent Router

| Condition | Mode | Read / Run |
|---|---|---|
| Full system/business/interaction testing, business tree/graph or storyline (完整测试, 业务图, 业务流) | `storyline` | [storyline-verification.md](references/storyline-verification.md); `arc verify scan-world → lint-story → plan-shards → play → completion` |
| Test-first red acceptance tests (测试先行, TDD) | `red` | [test-first.md](../_shared/test-first.md), [case-design.md](references/case-design.md) |
| Test plan, scope or test value audit only (测试方案, 测试价值) | `design` | [case-design.md](references/case-design.md), [test-value.md](references/test-value.md), [scope-cadence.md](references/scope-cadence.md) |
| Concurrency, idempotency, transaction faults, money/inventory (事务并发, 幂等, 业务不变量) | `reliability` | [txn-faults.md](references/txn-faults.md), [txn-design.md](references/txn-design.md) |
| PostgreSQL gates G0–G31 or SQL static lint (PostgreSQL, SQL 语法检查) | `db` | [db.md](references/db.md) / [sql-static.md](references/sql-static.md) |
| Web E2E / tester-army E2E / web perf (网页实测) | `e2e` | [web-e2e.md](references/web-e2e.md) / [tester-army-e2e.md](references/tester-army-e2e.md) / [web-perf.md](references/web-perf.md) |
| Fuzzing, property/model conformance, flaky or release go/no-go (模糊测试, 属性测试, flaky, 发布验收) | `specialized` | [fuzz-campaign.md](references/fuzz-campaign.md) / [model-conformance.md](references/model-conformance.md) / [release-ops.md](references/release-ops.md) |

## Workflow

Shared contract: [contract.md](../_shared/contract.md), [business-verification.md](../_shared/business-verification.md). Diagnose failures via evidence-first root-cause repair in [execution-truth.md](../../docs/execution-truth.md) §6.

1. **Scan World Flags**: run `arc verify scan-world <focal_entity> --format yaml` (e.g. `users.id`) to extract every coupled SQL table/FK/status/amount column, ORM struct and async MQ topic.
2. **Author Storyline (`<feature>.story.yaml`)**: declare `world_flags` (100% of scanned couplings), ordered `checkpoints`, parallel `routes` (unique `runtime_ns`, disjoint `write_scope`, `butterfly_probes`, `anti_softlock` for every cooling/pending/frozen state), `blind_mask` (private impl paths), and `required_endings` (clean exit, hard block, cooling+escape, forced cascade/anonymization).
3. **Static Storyline Lint**: run `arc verify lint-story <file> --world-scan --max-errors 5`. Fix every unbound flag, softlock risk, write overlap or unreachable ending before writing test code.
4. **Compile Blind Shards & Clone Checkpoints**: run `arc verify plan-shards <file> --out verify-shards.yaml`. Author tests against frozen contracts, schema and storyline ONLY (blind to `blind_mask` impl files); E2E reuses checkpoint seeders via `SHARD_NS`.
5. **Concurrent Execution & 100% Completion Gate**:
   - Run `arc verify play <file> --parallel --blind` (appends immutable receipts to `.arc/arc.db`; never overwrites first FAIL).
   - Run `arc verify completion <file> --world-scan` — exits `0` ONLY when `world_flag_binding_pct`, `route_coverage_pct`, `butterfly_probe_hit_pct`, `ending_unlock_pct` are all **100%** with `softlock_count == 0`.
   - Export static read-only workbench: `arc verify snapshot --file <file> --out .arc/artifacts/<slug>/workbench`.

## Tool Strategy

- **Preferred**: `arc verify scan-world|lint-story|plan-shards|checkpoint|play|completion|snapshot`, project native test runners, `scripts/with_server.py`.
- **Forbidden**: editing production source, writing stubbed tests from impl `if` branches, re-blessing snapshots, `sleep` sync, mocking the DB/broker to claim real durability PASS, claiming PASS on routes without executable commands, or erasing historical FAILs.

## Decision Rules

- IF local unit/SQL-lint only (no full-system/business-flow request) → run native runner directly; do not force a storyline DB.
- IF `read-only` or `plan-only` → do not mutate ledger or execute target actions; mark runtime claims `NOT_RUN`.
- IF a route enters a suspended/cooling/frozen status without an escape step or `anti_softlock` guard → `FAIL` (`SOFTLOCK_RISK`).
- IF a product defect is proven → preserve original FAIL in ledger and hand a repair packet to `arc-code`.

## Verification

| Claim | Evidence |
|---|---|
| Storyline complete & softlock-free | `arc verify lint-story --world-scan` exits 0 |
| 100% endings & butterfly probes | `arc verify completion --world-scan` exits 0 (all 4 metrics = 100%, softlocks = 0) |
| Visual review artifact | `arc verify snapshot` UTF-8 HTML+JSON directory |

## Failure Recovery

| Failure | Action | Stop |
|---|---|---|
| `UNBOUND_WORLD_FLAG` / `SOFTLOCK_RISK` | Bind missing entity or add escape + `anti_softlock` invariant | Re-run `lint-story` until PASS |
| Missing real DB/env/auth | Record exact missing prerequisite | `BLOCKED` (never mock PASS) |
| Route FAIL | Retain FAIL receipt in `.arc/arc.db`, diagnose via root-cause gate | Product bug → repair packet to `arc-code` |

## Completion Criteria

- [ ] `arc verify scan-world` and `lint-story --world-scan` passed with 0 unbound flags and 0 softlock risks.
- [ ] `arc verify play --parallel --blind` executed real commands with unique `SHARD_NS`; historical FAILs preserved.
- [ ] `arc verify completion --world-scan` exited 0 (or explicit `BLOCKED`/`FAIL` + repair packet reported); production source unchanged.
