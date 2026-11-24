# Code-Level Testing Catalog (26 Techniques & 8 Coverage Metrics)

`arc verify code` provides risk-driven, multi-language **code-level testing** (unit, coverage, fuzzing, mutation, property-based, concurrency, memory safety, static/security analysis, performance, formal/model verification, docs, compatibility). It complements—rather than replaces—business E2E storyline verification (`arc verify play`).

## 1. Master 26-Technique Catalog (7 Structured Dimensions)

Every technique in `arc verify code catalog` defines: **(1) Applicable Languages**, **(2) Trigger & Risk-Promotion Signals**, **(3) Execution Tools**, **(4) Result Metrics**, **(5) Failure Criteria**, **(6) Report Formats**, and **(7) Evidence Storage** (`code_verify_results` table + `.arc/verify/code/<run_id>/<tech_id>/` hashed artifacts).

| # | ID | 技术名称 | English Name | Tier | Rust Flagship Tool | Multi-Lang Tools | Trigger / Risk-Promote Signals | Key Failure Criteria |
|---|---|---|---|---|---|---|---|---|
| 1 | `unit` | 单元测试 | Unit Testing | `P0` | `cargo test` | Go `go test`, TS `vitest`, Py `pytest`, C `ctest` | `always_baseline`, `has_tests` | `exit != 0` or `tests_failed > 0` |
| 2 | `integration` | 集成测试 | Integration Testing | `P0` | `cargo test --tests` | Go `go test`, TS `vitest`, Py `pytest`, C `ctest` | `has_multi_module`, `has_tests` | `exit != 0` or `tests_failed > 0` |
| 3 | `coverage` | 代码覆盖率分析 | Code Coverage | `P0` | `cargo-llvm-cov` | Go `go tool cover`, TS `vitest --coverage`, Py `pytest-cov` | `always_baseline`, `has_tests` | `exit != 0` or below coverage threshold |
| 4 | `fuzz` | 模糊测试 ★ | Fuzz Testing | `P1` | `cargo-fuzz` (Nightly) | Go `go test -fuzz`, Py `atheris`, TS `jazzer.js`, C `libFuzzer` | **Promote**: `has_parser_or_untrusted_io`, `has_unsafe_or_ffi` | `crashes > 0` or `exit != 0`; missing target = `BLOCKED` |
| 5 | `property` | 属性测试 ★ | Property-based Testing | `P1` | `proptest` | Go `rapid`, Py `hypothesis`, TS `fast-check` | **Promote**: `has_codec_or_algebraic_logic`, `has_state_machine_or_protocol` | Counterexample breaks invariant (`tests_failed > 0`) |
| 6 | `mutation` | 变异测试 ★ | Mutation Testing | `P1` | `cargo-mutants` | Go `gremlins`, Py `mutmut`, TS `stryker` | **Promote**: `has_critical_core_logic` | `mutation_score_pct < 80%` or `exit != 0` |
| 7 | `regression` | 回归测试 | Regression Testing | `P0` | `cargo-nextest` (fallback `cargo test`) | Go `go test -count=1`, TS `vitest`, Py `pytest` | `always_baseline`, `has_tests` | `tests_failed > 0` or `exit != 0` |
| 8 | `snapshot` | 快照测试 | Snapshot Testing | `P1` | `insta` | Go golden, TS `vitest`, Py `syrupy` | `has_serialized_outputs` | Unapproved snapshot diff (`exit != 0`) |
| 9 | `differential` | 差分测试 | Differential Testing | `P2` | Custom differential harness | Go/Py/TS differential harness | `has_reference_or_dual_impl`, `has_codec_or_algebraic_logic` | Dual implementations diverge |
| 10 | `metamorphic` | 变形测试 | Metamorphic Testing | `P2` | `proptest` (metamorphic) | Go/Py/TS metamorphic harness | `has_numeric_or_search_transform`, `has_codec_or_algebraic_logic` | Input-output metamorphic relation violated |
| 11 | `boundary` | 边界值测试 | Boundary Value Testing | `P1` | `rstest` | Go table-driven, Py `parametrize`, TS `test.each` | `has_boundary_constraints`, `always_baseline` | Off-by-one / overflow / null boundary failure |
| 12 | `parameterized` | 参数化测试 | Parameterized Testing | `P1` | `rstest` | Go table-driven, Py `parametrize`, TS `it.each` | `has_table_or_matrix_inputs`, `has_tests` | Any input matrix case fails |
| 13 | `combinatorial` | 组合测试 | Combinatorial Testing | `P2` | Pairwise/t-wise generator | Go pairwise, Py `allpairspy`, TS pairwise | `has_feature_or_option_matrix`, `has_boundary_constraints` | Any factor combination conflicts or fails |
| 14 | `concurrency` | 并发测试 | Concurrency Testing | `P2` | `loom` | Go `-race -count=5`, Py/TS concurrent | **Promote**: `has_concurrency` | Deadlock, ordering violation, or race |
| 15 | `data_race` | 数据竞争检测 | Data Race Detection | `P2` | `ThreadSanitizer` (Nightly) | Go `-race`, C/C++ TSan, Swift `--sanitize=thread` | **Promote**: `has_concurrency` | `data_races > 0` (zero tolerance) |
| 16 | `memory_safety` | 内存安全测试 | Memory Safety Testing | `P2` | `Miri` (Nightly) + ASan | C/C++ ASan+UBSan, Go `checkptr=2`, Swift ASan | **Promote**: `has_unsafe_or_ffi` | `ub_errors > 0` (zero tolerance) |
| 17 | `benchmark` | 性能基准测试 | Benchmark Testing | `P3` | `criterion` / `divan` | Go `go test -bench`, Py `pytest-benchmark`, TS `vitest bench` | `has_hot_path_or_perf_budget`, `has_bench_harness` | `bench_regression_pct > 10%` or `exit != 0` |
| 18 | `stress` | 压力测试 | Stress Testing | `P3` | Custom stress harness | Go/Py/TS stress harness | `has_network_or_service_entrypoint`, `has_hot_path_or_perf_budget` | `crashes > 0` or unrecoverable degradation |
| 19 | `soak` | 稳定性测试 | Soak Testing | `P3` | Custom soak harness | Go/Py/TS soak harness | `has_network_or_service_entrypoint` | Memory/FD leak or `crashes > 0` |
| 20 | `static_analysis` | 静态分析 | Static Analysis | `P0` | `clippy` (`-D warnings`) | Go `golangci-lint`/`go vet`, TS `tsc --noEmit`, Py `ruff`, C `clang-tidy` | `always_baseline` | `warnings > 0` or `exit != 0` |
| 21 | `sast` | 安全代码扫描 | SAST | `P1` | `semgrep` / CodeQL | `semgrep`, Go `gosec`, Py `bandit`, CodeQL | **Promote**: `has_security_surface` | `high_vulns > 0` or `exit != 0` |
| 22 | `dependency_audit` | 依赖安全审计 | Dependency Auditing | `P0` | `cargo-audit` / `cargo-deny` | Go `govulncheck`, TS `npm audit`, Py `pip-audit` | `has_dependencies`, `always_baseline` | Known CVE or supply-chain/license policy fail |
| 23 | `formal_verification` | 形式化验证 | Formal Verification | `P3` | `Kani` / `Prusti` | C/C++ `cbmc` | **Promote**: `has_formal_harness` | `proofs_failed > 0` or counterexample found |
| 24 | `model_based` | 模型测试 | Model-based Testing | `P3` | `proptest-state-machine` | Go `rapid` StateMachine, Py `hypothesis`, TS `fast-check` | `has_state_machine_or_protocol` | Real implementation diverges from state model |
| 25 | `doctest` | 文档测试 | Documentation Testing | `P1` | `cargo test --doc` | Go `Example*` tests, Py `--doctest-modules` | `has_public_api_or_docs` | Doc examples fail to compile or assert |
| 26 | `compatibility` | 兼容性测试 | Compatibility Testing | `P2` | `cargo check` / `cargo-hack` | Go cross-build, TS `tsc`, Py `tox` | `has_feature_or_option_matrix`, `has_cross_target_or_msrv` | Feature-matrix, MSRV, or target build fails |

★ = Top-3 most commonly overlooked techniques (`mutation`, `property`, `fuzz`).

---

## 2. Eight Code Coverage Metrics (`CoverageBreakdown`)

Never collapse coverage into a single unlabeled percentage. `arc verify code` tracks all 8 metrics separately and marks unmeasured dimensions explicitly as `NOT_MEASURED` inside `unsupported_metrics`:

| Metric ID | English Name | 中文名称 | What It Measures | Rust Support (`cargo-llvm-cov`) |
|---|---|---|---|---|
| `line` | Line Coverage | 行覆盖率 | Executable source lines hit at least once | Stable (`lines.percent`) |
| `statement` | Statement Coverage | 语句覆盖率 | Syntactic statements / LLVM regions executed | Stable approximation (`regions.percent`) |
| `function` | Function Coverage | 函数覆盖率 | Defined functions/methods invoked | Stable (`functions.percent`) |
| `branch` | Branch Coverage | 分支覆盖率 | True/false & match control-flow outcomes | Nightly (`cargo +nightly llvm-cov --branch`) |
| `condition` | Condition Coverage | 条件覆盖率 | Atomic boolean sub-expressions hit T/F | Nightly `--branch` / `--mcdc` + property tests |
| `decision` | Decision Coverage | 决策覆盖率 | Overall `if`/`while`/`match` predicate outcomes | Nightly `--branch` |
| `mcdc` | MC/DC Coverage | 修正条件/判定覆盖率 | Each atomic condition independently toggles outcome | Nightly (`-C instrument-coverage=mcdc`) |
| `path` | Path Coverage | 路径覆盖率 | Distinct end-to-end execution paths traversed | Approximated via `proptest` / `kani` symbolic paths |

---

## 3. Four-Stage Rollout & Risk-Driven Promotion

1. **P0 (Essential Baseline)**: `unit`, `integration`, `regression`, `coverage`, `static_analysis`, `dependency_audit`.
2. **P1 (Deep Defect Discovery)**: `property`, `fuzz`, `mutation`, `boundary`, `parameterized`, `snapshot`, `sast`, `doctest`.
3. **P2 (Low-Level Correctness & Reliability)**: `concurrency`, `data_race`, `memory_safety`, `differential`, `metamorphic`, `combinatorial`, `compatibility`.
4. **P3 (Advanced Assurance & Performance)**: `benchmark`, `stress`, `soak`, `model_based`, `formal_verification`.

**Automatic Risk Promotion**: Even when running at `--max-tier P0`, `arc verify code plan` scans real code lines (`File:Line`) and forcibly promotes matching high-risk checks to `REQUIRED` (`EffectivePriority: P0`):
- `unsafe` / FFI detected (`has_unsafe_or_ffi`) -> promotes `memory_safety` (Miri/ASan) & `fuzz`
- Locks/atomics/spawns (`has_concurrency`) -> promotes `data_race` (TSan/Go `-race`) & `concurrency` (`loom`)
- Untrusted input parsers (`has_parser_or_untrusted_io`) -> promotes `fuzz`
- Codecs / state machines (`has_codec_or_algebraic_logic`, `has_state_machine_or_protocol`) -> promotes `property`
- Auth / crypto / SQL / exec surfaces (`has_security_surface`) -> promotes `sast`
- Existing `#[kani::proof]` (`has_formal_harness`) -> promotes `formal_verification`

---

## 4. Top-3 Most Overlooked Techniques (Deep Focus)

1. **Mutation Testing (`cargo-mutants` / `gremlins` / `stryker`)**: Exposes tests with 100% line coverage that fail to assert behavior by inverting conditions, mutating operators, or dropping return values.
   ```bash
   cargo mutants --in-place=false --output "$ARC_CODE_ARTIFACT_DIR"
   ```
2. **Property-based Testing (`proptest` / `rapid` / `hypothesis` / `fast-check`)**: Replaces brittle hand-picked constants with universally quantified invariants (`decode(encode(x)) == x`, ledger conservation, idempotency, ordering) plus automatic counterexample shrinking.
   ```bash
   cargo test --workspace -- proptest
   ```
3. **Fuzz Testing (`cargo-fuzz` / `go test -fuzz` / `libFuzzer`)**: Continuously mutates byte streams against parsers, decoders, serialization, and untrusted boundaries to uncover panics, OOMs, and UB.
   ```bash
   cargo +nightly fuzz run fuzz_target_1 -- -max_total_time=30
   ```

---

## 5. Custom Harness Protocol (`[ARC_CODE_METRIC]`)

Custom stress, soak, differential, metamorphic, or benchmark harnesses can emit a single-line JSON prefixed with `[ARC_CODE_METRIC]` so `arc verify code run` stores deterministic metrics alongside native tool parsing:

```text
[ARC_CODE_METRIC]{"tests_passed":120,"tests_failed":0,"mutants_caught":45,"mutants_missed":3,"mutation_score_pct":93.8,"bench_regression_pct":1.2,"rss_growth_pct":0.4}
```

Uninstalled tools, missing test harnesses, `--dry-run`, or `0 tests ran` always record `BLOCKED` with the missing prerequisite and install hint—never `PASS`. All historical `FAIL` receipts in `code_verify_results` remain immutable.
