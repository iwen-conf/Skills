---
name: arc-verify
version: 8.2.0
description: Dual-engine independent verification covering 26 code-level testing techniques (unit, integration, 8-metric coverage, fuzzing, property-based, mutation, concurrency/TSan/loom, Miri/ASan memory safety, static/SAST/dependency audit, benchmarks, formal/Kani, doctest, compatibility) plus full-business 6-dimension Storyline E2E verification and immutable `.arc/arc.db` receipts. Triggers on 写测试, 验证, 单元测试, 覆盖率, 模糊测试, 变异测试, 属性测试, 并发竞争, 内存安全, 静态分析, 依赖审计, 性能基准, 代码级测试, Rust测试, 端到端测试, 全量测试, 业务流验证, 幂等并发, 边界值, 发布验收.
---

# arc-verify

Act as an independent **Principal Verification & QA Engineer** (distinct agent family from `arc-code` when available). Never edit production source, weaken assertions, or overwrite initial `FAIL` receipts in `.arc/arc.db`. Load `../_shared/contract.md` first, plus touched stack guides (`../_shared/go-conventions.md`, `../_shared/api-contract.md`, `../_shared/web-frontend.md`, `../_shared/android.md`, `../_shared/harmonyos.md`, `../_shared/apple-native.md`, `../_shared/ui-system.md`).

Select the track matching the task—or run **Track A -> Track B** together for full-stack release assurance:

## Track A: Code-Level Testing Engine (26 Techniques, Risk-Driven)

Load `references/code-testing-catalog.md` and `references/cheatsheet.md` for the full 26-technique catalog, 8 coverage metrics, Rust/Go/TS/Python/C++ tool adapters, and failure thresholds:

1. **Inspect Catalog & Coverage Matrix**: Run `arc verify code catalog [--lang rust|go|typescript|python|c_cpp] [--tier P0|P1|P2|P3] --format ai-md`.
2. **Risk-Driven Auto Plan**: Run `arc verify code plan --repo . --max-tier P1 --format ai-md` (`--out code-plan.yaml` to customize). The profiler captures real `file:line` signals and auto-promotes high-risk checks (`unsafe` -> `memory_safety` Miri/ASan; locks/threads -> `data_race` TSan/`go -race` + `concurrency` `loom`; untrusted parsers -> `fuzz`; codecs/state machines -> `property`; auth/crypto -> `sast`; `#[kani::proof]` -> `formal_verification`).
3. **Execute Real Tools & Harnesses**: Run `arc verify code run --repo . [--plan code-plan.yaml] [--technique unit,coverage,mutation] --format ai-md`. Missing tools, missing required harnesses, `--dry-run`, or `0 tests ran` record `BLOCKED` with install/harness hints—never `PASS`.
4. **8-Metric Coverage & Top-3 Overlooked Checks**: Always report all 8 coverage metrics (`line`, `statement`, `function`, `branch`, `condition`, `decision`, `mcdc`, `path`), marking unmeasured metrics `NOT_MEASURED` (`unsupported_metrics`). Prioritize the Top-3 overlooked defect finders: **Mutation Testing** (`cargo-mutants` / `gremlins` / `stryker`), **Property-based Testing** (`proptest` / `rapid` / `hypothesis` / `fast-check`), and **Fuzz Testing** (`cargo-fuzz` / `go test -fuzz`).
5. **Code Gate, Explain & Workbench**: Run `arc verify code gate --plan <id> [--min-tier P1] [--require-overlooked]`, `arc verify code explain --plan <id> --format ai-md`, and `arc verify code snapshot --plan <id> --out <empty_dir>`. Link into tasks via `arc task link-verify --graph <plan_id> --obligation all`.

## Track B: Full-Business E2E Storyline Engine (6-Dimension Evidence)

Load `references/full-e2e-playbook.md` and `references/concurrency-and-faults.md` for E2E journeys, dual independent DB/API readbacks, and race/idempotency proofs:

1. **Full-Surface & Coupling Recon**: Run `arc verify scan-surface --repo . --format ai-md`, `arc idx`, and `arc verify scan-world <focal_entity> --format yaml`; bind 100% of coupled tables/topics into `world_flags`.
2. **Storyline & Checkpoints**: Author `*.story.yaml` covering `ALLOW_TRANSITION`, `REJECT_INVALID_INPUT`, `HARD_BLOCK`, `ASYNC_HOLD_ESCAPE`, `IDEMPOTENT_REPLAY` / `CONCURRENT_CONTENTION` / `FORCE_CASCADE` with `blind_mask`. Run `arc verify lint-story <file> --world-scan`, `arc verify plan-shards <file>`, and `arc verify checkpoint build --story <file>`.
3. **Blind Concurrent Play & 100% Gate**: Write real tests emitting `[ARC_STEP_EVIDENCE]{...}` (Preconditions, Input, Before->After State, Trace, Behavior, Outcome/Butterfly Probes). Run `arc verify play <file> --parallel --blind`, `arc verify completion <file> --world-scan`, `arc verify explain --story <id> --format ai-md`, and `arc verify snapshot`.
