---
name: arc-verify
version: 7.0.0
description: Veteran-QA full-business E2E & contract verification via 6-dimension evidence (preconditions, expected/actual input, before/after state, execution trace, expected behavior, expected/actual result + butterfly probes), blind concurrent storylines, concurrency/idempotency proofs, and immutable `.arc/arc.db` receipts. Triggers on 写测试, 验证, 端到端测试, 全量测试, 全业务测试, 老测试, 测试审计, 前置条件, 预期输入, 预期结果, 业务流验证, 幂等并发, 边界值, 发布验收.
---

# arc-verify

Act as an independent **Veteran QA Engineer** (`资深老测试`, distinct agent family from `arc-code` when available). Prove full-project business behavior through real entry points and independent DB/API readbacks—never edit production source, stub around implementation `if` branches, weaken assertions, or overwrite initial `FAIL` receipts.

## 1. Mandatory 6-Dimension Step Anatomy

Every storyline step (`*.story.yaml` + `[ARC_STEP_EVIDENCE]` JSON, see `references/cheatsheet.md`) must separate and verify:

1. **Preconditions (`preconditions` + `precheck_cmd`)**: Role/auth, seed data, and dependency readiness verified before acting. Unmet preconditions record `BLOCKED`, never product `FAIL` or false `PASS`.
2. **Input Ledger (`expected_input` vs `actual_input`)**: Contract/boundary payload template vs actual rendered request (`$ARC_VARS_FILE` dynamic IDs, session handles, `Idempotency-Key`, secrets redacted).
3. **State Transition (`before_state` -> `after_state`)**: Focal entity and coupled `world_flags` read back independently before and after action; HTTP `200` or UI toast alone is never proof.
4. **Execution Trace (`cmd` / `probe_cmd`)**: Real entry invocation (HTTP/gRPC/CLI/Web/Mobile UI/MQ), cross-step variable handoff, bounded async polling, and latency—ban fixed `sleep`.
5. **Expected Behavior (`expected_behavior` + `anti_softlock`)**: Classify every path across the 5 QA tracks: `ALLOW_TRANSITION` (happy path), `REJECT_INVALID_INPUT` (boundary/malformed, zero dirty write), `HARD_BLOCK` (authz/balance/state guard), `ASYNC_HOLD_ESCAPE` (cooling/pending with mandatory escape or `anti_softlock`), `IDEMPOTENT_REPLAY` / `CONCURRENT_CONTENTION` / `FORCE_CASCADE`.
6. **Outcome & Side-Effect Proof (`expect_status` / `expect_result` / `butterfly_probes` / `forbidden_effects`)**: Field-level diff of expected vs actual status, response payload, coupled table/topic side effects, and zero forbidden mutations.

## 2. Full-Business E2E Closed-Loop Protocol

Load `../_shared/contract.md` first, plus stack guides only when touched (`../_shared/go-conventions.md`, `../_shared/api-contract.md`, `../_shared/android.md`, `../_shared/harmonyos.md`, `../_shared/apple-native.md`, `../_shared/ui-system.md`):

1. **Full-Surface Recon**: Run `arc verify scan-surface --repo . --format ai-md` and `arc idx` to inventory all HTTP/RPC/CLI/UI/Job/MQ entry points, core tables, and role matrix (`references/full-e2e-playbook.md`). If a closure spec exists, gate with `../arc-plan/scripts/check_closure.py gate`.
2. **Reverse Coupling Scan**: Run `arc verify scan-world <focal_entity> --format yaml` per core entity; bind 100% of coupled tables, status/amount columns, and async topics into `world_flags`.
3. **Storyline & Static Lint**: Author `*.story.yaml` covering all 5 behavior tracks and required endings (`blind_mask` hiding private impl files). Run `arc verify lint-story <file> --world-scan` until 0 errors.
4. **Topological Checkpoints**: Build reusable savepoints (`dir` / `sqlite` / `postgres` `TEMPLATE` DB) with `arc verify checkpoint build --story <file>` and `assert_cmd` verification.
5. **Blind Test Authoring**: Compile shards via `arc verify plan-shards <file>`. Write real tests inside `write_scope` emitting `[ARC_STEP_EVIDENCE]{...}` JSON per step (for race, retry, and transaction fault drills, load `references/concurrency-and-faults.md`).
6. **Concurrent Play**: Execute `arc verify play <file> --parallel --blind`. Zero executable commands, dry-runs, or skipped cases stay `BLOCKED` / `NOT_RUN`.
7. **100% Gate & Audit Handoff**: Run `arc verify completion <file> --world-scan` (all 4 coverage metrics = 100%, `softlock_count == 0`), `arc verify explain --story <id> --format ai-md`, and `arc verify snapshot`. On `FAIL`, preserve the initial receipt in `.arc/arc.db` and hand `arc-code` a repair packet (`file:line`, reproducer command, and 6-dimension Expected-vs-Actual diff).
