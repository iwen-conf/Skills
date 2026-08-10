---
name: arc:test
version: 0.2.0
description: 'Layered testing: unit/integration/contract/E2E, coverage, fuzz/property,
  perf; Go/Rust/Android/HarmonyOS/frontend.'
enforce_arc_profile: true
expert_keywords:
- Test Pyramid
- Branch Coverage
- Fuzzing
- Property-Based
- Benchmark
- Flaky
- Regression
- SLA
---

# arc:test

## Overview

`arc:test` strictly focuses on **Testing**: whether business functions meet expectations (functional correctness) and whether system performance/capacity meets requirements. It does NOT cover Security Gates (system compromise, permission leaks), which are delegated to `arc:security`.

It designs, generates, and runs a project's test suite by layer and by risk, then reports evidence-backed pass/fail and coverage gaps. It picks platform-native tooling instead of a bespoke cross-language framework, keeps functional and performance verdicts separate, and hands failures, security checks, and large build-outs to the right Arc skill. It does not repair failing code directly or create Lark resources directly.

Read:

- [`references/test-scope-matrix.md`](references/test-scope-matrix.md) for the layered scope taxonomy and the P0–P2 default strategy
- [`references/test-stacks.md`](references/test-stacks.md) for per-platform/per-language tools and concrete commands (Go, Rust, Android/Maestro, HarmonyOS/arkxtest, frontend)

## Quick Contract

- **Trigger**: The user asks to add, generate, run, or review tests; set up coverage/regression gates; add fuzz, property, benchmark, or load tests; or verify a suite across Go, Rust, Android, HarmonyOS, or frontend.
- **Inputs**: Project path, scope, platform/language, which layers to enable, optional SLA/hotspot, optional authorized target URL, expected commands.
- **Outputs**: Per-layer commands and pass/fail, coverage summary with high-risk gaps, robustness and performance results judged separately, flaky list, and handoffs to `arc:fix` / `arc:security` / `arc:sdlc` / `arc:docs`.
- **Quality Gate**: Every "it works" claim is backed by an executed test with evidence; layers are enabled by risk, not to tick boxes; performance is judged apart from functional; coverage is treated as a gap signal, not a target.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` first if test scope, target environment, acceptance criteria, or authorization for load/E2E targets are unclear.
- Use `arc:fix` when a test reproduces a concrete failure that must be repaired; `arc:test` writes the failing/regression test, `arc:fix` roots-causes and fixes it.
- Use `arc:build` for the feature/refactor edit itself; `arc:build` still runs its own targeted verification, while `arc:test` owns suite design, coverage/CI gates, and non-trivial test build-out.
- Use `arc:frontend` for frontend interaction/visual test surfaces so page interaction, visibility, clickability, and layout-readability checks stay aligned with the default frontend stack and its runnable-UI evidence rules.
- Use `arc:security` for local SAST/SCA/secrets/DAST/API-fuzz automation, and `arc:audit` for read-only security/non-functional review; `arc:test` does not re-implement scanner automation.
- Use `perfetto-trace-analysis` / `perfetto-sql` for Android and system-trace flame charts, and `web-perf` for frontend performance profiling; `arc:test` orchestrates and gates performance, it does not re-implement deep profilers.
- Use `arc:sdlc` before large, multi-module, or tracked test build-out (new test architecture, coverage-gate rollout, cross-platform suite) so subtasks stay detailed and progress stays current.
- Use `arc:docs` only when Lark is active for test reports, coverage trends, regression status, `task_base`, or `.lark.json.lifecycle[]`.

## Context Search

- MUST inspect existing test layout, test runners, CI config, coverage config, and fixtures before adding or running tests.
- MUST detect stack markers (`go.mod`, `Cargo.toml`, `package.json`, `build.gradle`/`AndroidManifest.xml`, `oh-package.json5`/`.test.ets`, `.maestro/` flows) to choose native tooling.
- MUST use `arc-idx search` first for broad repository context and existing test patterns.
- MUST use `arc-idx ast` for the code shapes that most need tests: parsers, decoders, state machines, business rules, error branches, and boundary handling.
- If `.lark.json` exists, MUST read it before test-report handoff and route durable records through `arc:docs`.

## Announce

Begin by stating clearly:
"I am using `arc:test` to build and run tests by layer and by risk, with platform-native tooling and separate functional and performance verdicts."

## The Iron Law

```text
NO FUNCTIONAL "IT WORKS" CLAIM WITHOUT EXECUTED TESTS AND EVIDENCE.
NO PERFORMANCE VERDICT MIXED INTO THE FUNCTIONAL TEST RUN—BENCHMARK AND LOAD ARE RUN AND JUDGED SEPARATELY.
NO BESPOKE CROSS-LANGUAGE TEST FRAMEWORK—USE PLATFORM-NATIVE TOOLING.
NO TEST-TYPE SPRAWL TO TICK BOXES—ENABLE LAYERS BY RISK.
NO COVERAGE NUMBER TREATED AS THE GOAL—IT IS A GAP SIGNAL FOR KEY PATHS AND ERROR BRANCHES.
NO GREEN FROM RETRYING FLAKY TESTS—QUARANTINE OR FIX THE ROOT CAUSE.
NO LARGE TEST BUILD-OUT WITHOUT CURRENT LOCAL TASK DOCS.
NO LARK TEST REPORT UPDATE OUTSIDE arc:docs.
```

## Hard Constraints

- MUST use platform-native tooling as the default: Android UI/flows via **Maestro**; Go via the built-in `testing` package (`go test`, `-race`, `-cover`, `testing.B`, `testing.F`); Rust via `cargo test`, Criterion, `cargo-fuzz`, and `proptest`; HarmonyOS via **arkxtest** (`@ohos/hypium` unit + `UiTest` UI automation); frontend via the project runner (default: Vitest + Testing Library and Playwright), with `agent-browser` for exploratory interaction checks.
- MUST NOT invent a custom unified test framework across languages or force unrelated layers into one runner.
- MUST enable layers by risk using the P0–P2 default strategy in [`references/test-scope-matrix.md`](references/test-scope-matrix.md); do not spread every test type over code that does not warrant it.
- MUST run and judge performance (benchmark/load/stress) separately from functional tests, with its own pass/fail and, when an SLA or hotspot exists, a baseline comparison.
- MUST profile to locate a hotspot — capture a CPU or allocation profile and read it as a flame graph — before optimizing it, then confirm the gain with a re-run benchmark; do not optimize by guessing. Route deep platform profiling to specialized skills (`perfetto-trace-analysis` / `perfetto-sql` for Android and system traces, `web-perf` for frontend).
- MUST treat coverage as a gap signal, not a target: report line and branch coverage, call out uncovered key paths and error branches, and never chase 100% blindly. Use ~70–85% on core business as a reference, not a global mandate.
- MUST prioritize functional correctness first: unit for logic/parse/algorithm/state-machine, integration where I/O or cross-layer contracts exist, contract/API tests for external and service-to-service interfaces, and a small, curated E2E set (~5%) for critical journeys.
- MUST cover error and boundary paths, not only happy paths; malformed input, empty state, and failure branches are first-class cases.
- MUST enable fuzzing (Go `testing.F`, Rust `cargo-fuzz`) and property-based tests (invariants, round-trip, idempotence) for parsers, codecs, protocols, deserialization, and security boundaries when such surfaces exist.
- MUST keep the regression suite as the CI default and treat smoke as a subset of regression; coverage does not replace historical regression cases.
- MUST hand security/non-functional testing (pen-test, SAST/dependency scan, chaos, compatibility, observability verification) to `arc:security`/`arc:audit` rather than rebuilding it here.
- MUST, for frontend, verify the interaction closed-loop (a user journey completes and returns to a consistent state), element visibility, control clickability/enabled state, and layout readability (no overlapping/truncated text, adequate contrast, stable responsive layout) — routed through `arc:frontend`.
- MUST get explicit authorization before running load, stress, or E2E against any shared or third-party target.
- MUST apply `arc:sdlc` before large, multi-module, cross-platform, or tracked test build-out; task docs must be generated from the latest project state and updated immediately when suites, gates, scope, assumptions, or status change.
- MUST report skipped layers with a reason, and report failing or flaky tests instead of hiding them.
- MUST route all Lark writes through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- NEVER weaken assertions, delete failing tests, add blanket retries, or raise timeouts just to make a suite green.
- NEVER claim a layer passed without running it, and never claim performance regression safety without a benchmark or baseline.

## Test Scope Layers

`arc:test` reasons about tests in five layers; enable them by risk (details and per-layer triggers in [`references/test-scope-matrix.md`](references/test-scope-matrix.md)):

| Layer | Purpose | Enable when |
|---|---|---|
| 1. Functional correctness | Unit → integration → contract/API → E2E | Almost always; E2E stays small (~5%) on critical journeys |
| 2. Quality metrics | Coverage (line/branch) + regression gate | Almost always; report trend, block on key packages |
| 3. Robustness / exploratory | Fuzz, property-based, (later) mutation | Parsers, codecs, protocols, deserialization, security edges |
| 4. Performance & capacity | Micro-benchmark, profiling/flame graph, load, stress, capacity | There is an SLA or a known hotspot; judged separately |
| 5. Security & non-functional | Pen-test, SAST/dep-scan, chaos, compatibility, observability | By product risk; via `arc:security` / `arc:audit`, not duplicated |

Priority ladder: **P0** unit + key integration + CI regression, plus coverage and fail-blocking on key packages. **P1** fuzz on parsing/boundary inputs and API contract tests, plus benchmark + baseline and profiling/flame graph on the hotspot when an SLA exists. **P2 on demand** load/stress, property, mutation, chaos, and full-chain E2E — only with a clear risk.

## Platform Test Stacks

Choose the native stack for the detected platform (concrete commands in [`references/test-stacks.md`](references/test-stacks.md)):

| Platform / language | Unit & functional | Robustness | Performance | UI / E2E |
|---|---|---|---|---|
| Go | `go test` (table-driven, `-race`), httptest for contract | `testing.F` native fuzz, `testing/quick` property | `testing.B` + `benchstat` | — |
| Rust | `cargo test`, integration in `tests/`, doctests | `cargo-fuzz`/libFuzzer, `proptest`/`quickcheck` | Criterion (`cargo bench`) | — |
| Android | JUnit/Kotlin for pure logic | — | macrobenchmark (optional) | **Maestro** flows (native + React Native) |
| HarmonyOS | **arkxtest** `@ohos/hypium` (`describe/it/expect`) | — | — | **arkxtest** `UiTest` (`Driver`/`ON`) |
| Frontend (web) | Vitest + Testing Library | — | Lighthouse/web-vitals (optional) | Playwright; `agent-browser` for exploration |

Coverage tooling: Go `go tool cover`; Rust `cargo llvm-cov`; frontend Vitest `--coverage` (v8/istanbul, branch-aware).

### Profiling & flame graphs (性能检测)

Profiling *locates* CPU/allocation/lock hotspots and renders them as a **flame graph (火焰图)**; benchmarks only *measure*. Loop: reproduce the hotspot → capture a profile → read the flame graph → fix the widest frames → re-benchmark to confirm. Run it separately from the functional pass, and hand deep platform tracing to the specialized skills.

| Platform | CPU/alloc profile → flame graph | Trace timeline | Deep-dive skill |
|---|---|---|---|
| Go | `go tool pprof` (built-in Flame Graph view), `net/http/pprof`, `runtime/pprof` | `runtime/trace` + `go tool trace` | — |
| Rust | `cargo flamegraph`, `samply`, `pprof-rs`; `dhat` for heap | `perf` + `inferno` | — |
| Android | `simpleperf --flamegraph`, Android Studio CPU Profiler, macrobenchmark | Perfetto system trace | `perfetto-trace-analysis`, `perfetto-sql` |
| HarmonyOS | DevEco Profiler (Time/Allocation/Frame), SmartPerf | `hitrace` / `hiperf` | — |
| Frontend | Chrome DevTools Performance (flame chart), React Profiler | DevTools Performance | `web-perf` |

## Workflow

1. Confirm scope, platform/language, which layers matter, any SLA/hotspot, and authorization for load/E2E targets.
2. Inspect existing tests, runners, CI, coverage config, and fixtures with local index tools; detect stack markers to pick native tooling.
3. Select layers by risk using the P0–P2 ladder; state explicitly which layers are enabled and which are skipped with a reason.
4. For large, multi-module, cross-platform, or tracked build-out, apply `arc:sdlc` before writing tests and keep local task status current.
5. Write functional tests first (unit → integration → contract → curated E2E), covering error and boundary paths; follow the platform stack and existing patterns.
6. Add robustness (fuzz/property) for parsing/boundary/security surfaces when present.
7. Run functional layers and collect coverage; record line/branch coverage and uncovered key paths and error branches.
8. Run performance layers **separately** (benchmark, then load/stress only when authorized and warranted); compare against baseline/SLA and judge pass/fail on their own.
9. Route frontend interaction/visibility/clickability/readability checks through `arc:frontend`; route security/non-functional checks to `arc:security`/`arc:audit`.
10. Triage results: quarantine or root-cause flaky tests; hand failing product code to `arc:fix`; summarize per-layer verdicts.
11. If `.lark.json` exists or the user explicitly triggered/confirmed Lark, hand off to `arc:docs` with per-layer results, coverage trend, regression status, task status, lifecycle link, and resource keys.

## Quality Gates

- Every enabled layer was actually run, with commands and pass/fail recorded; skipped layers list a reason.
- Functional tests cover happy path plus error and boundary branches, not only success cases.
- Coverage is reported as line and branch with named high-risk gaps, and is not presented as a target number to chase.
- Performance results are separate from functional results, with a baseline or SLA comparison when one exists.
- Fuzz/property tests exist for parsers, codecs, protocols, deserialization, or security boundaries when those surfaces are present.
- Regression suite runs in CI as the default; smoke is a labeled subset, not a replacement.
- Frontend deliverables prove interaction closed-loop, visibility, clickability, and layout readability through `arc:frontend`.
- Flaky tests are quarantined or fixed, never masked with blanket retries or inflated timeouts.
- Large, multi-module, cross-platform, or tracked test work has current local task docs and synchronized progress from `arc:sdlc`.
- Security/non-functional coverage is delegated to `arc:security`/`arc:audit` and labeled, not duplicated here.
- Lark test status and `task_base` are recorded via `.lark.json` only when Lark is active.

## Expert Standards

- Shape the suite as a `Test Pyramid`: many fast unit tests, fewer integration/contract tests, and a thin, curated E2E tier for critical journeys.
- Report `Branch Coverage` alongside line coverage; branch/condition coverage is closer to real risk, and key modules may warrant a higher bar — but the number is a signal, never the objective.
- Use `Fuzzing` (Go `testing.F`, Rust `cargo-fuzz`/libFuzzer) for malformed input, parsers, protocols, and deserialization, and pair it with `Property-Based` tests for invariants such as encode/decode round-trip, ordering stability, and idempotence.
- Keep performance a separate discipline: run `Benchmark`/micro-benchmarks for hotspot functions and allocation regressions, gate against a baseline, and reserve load/stress/capacity work for real `SLA` or hotspot cases — never mixed into the functional pass.
- Detect before optimizing: capture a CPU or allocation profile and read it as a flame graph to find the true hotspot, fix it, then re-run the `Benchmark` to confirm — profiling localizes, benchmarking quantifies, and neither is a guess. Use `go tool pprof`/`go tool trace`, `cargo flamegraph`/`samply`, `perf` + `inferno`, and route Android/system traces to `perfetto-trace-analysis` and frontend to `web-perf`.
- Treat the `Regression` suite as the memory of past bugs: it is the CI default, smoke is its subset, and high coverage never replaces historical cases.
- Manage `Flaky` tests explicitly — quarantine, deterministic seeds, controlled clocks/IO, and root-cause fixes — rather than retrying into green.
- Reserve mutation testing for mature suites (it measures whether the tests themselves are good, at high cost) and hand security, chaos, compatibility, and observability verification to `arc:security`/`arc:audit`.

## Scripts & Commands

Use project-native test tooling; do not add a custom cross-language runner. Concrete per-stack commands live in [`references/test-stacks.md`](references/test-stacks.md). Common entry points:

```bash
# Go
go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
go test -bench=. -benchmem ./...            # perf, run separately; compare with benchstat
go test -run=xxx -fuzz=FuzzXxx -fuzztime=30s ./pkg

# Rust
cargo test                                  # unit + integration (tests/) + doctests
cargo llvm-cov --branch --summary-only      # coverage (line/branch)
cargo bench                                  # Criterion, run separately
cargo fuzz run <target>                      # cargo-fuzz / libFuzzer (nightly)

# Android UI / E2E (native or React Native)
maestro test .maestro/flow.yaml --format junit

# HarmonyOS (arkxtest: hypium unit + UiTest UI)
hdc shell aa test -b <bundle> -m <module> -s unittest OpenHarmonyTestRunner

# Frontend (default stack)
vitest run --coverage                        # unit/component, branch coverage
playwright test                              # interaction / E2E; agent-browser for exploration

# Profiling → flame graph (locate hotspots; run separately from functional)
go test -cpuprofile cpu.out -bench=. ./pkg && go tool pprof -http=:8080 cpu.out
cargo flamegraph --bench my_bench            # perf + inferno → flamegraph.svg
perf record -g -- ./app && perf script | inferno-collapse-perf | inferno-flamegraph > fg.svg
```

## Red Flags

- Building a custom cross-language test harness instead of using `go test`, `cargo test`, Maestro, arkxtest, Vitest, or Playwright.
- Spreading every test type across code that does not warrant it just to look thorough.
- Mixing benchmark/load results into the functional run so a slow-but-correct or fast-but-broken build is misjudged.
- Optimizing a "slow" path by guessing instead of reading a profile/flame graph, or treating a single benchmark number as a diagnosis of where the time goes.
- Reporting a coverage percentage as the goal while key paths and error branches stay untested.
- Only happy-path tests; missing malformed-input, empty-state, and failure branches.
- Retrying, sleeping, or raising timeouts to turn a flaky suite green instead of fixing the root cause.
- Weakening or deleting a failing test to claim completion.
- Running load/stress/E2E against a shared or third-party target without authorization.
- Re-implementing security scanning that belongs in `arc:security`, or shipping frontend UI tests that never check interaction closed-loop, visibility, clickability, or readability.
- Large test build-out from stale task docs, or leaving local test progress inconsistent with the actual suite/gates.
- Updating Lark test reports directly instead of through `arc:docs`.

## When to Use

- **Preferred Trigger**: The user asks to write, generate, run, or review tests; set up coverage or regression gates; add fuzz/property/benchmark/load; or verify a suite for Go, Rust, Android, HarmonyOS, or frontend.
- **Typical Scenario**: Go service with table-driven unit + `testing.F` fuzz + `testing.B` benchmarks and coverage gate; Rust crate with `cargo test` + `proptest` + Criterion; Android/React Native app flows in Maestro; HarmonyOS app with arkxtest hypium + UiTest; React 19 frontend with Vitest + Playwright interaction tests.
- **Boundary Tip**: Use `arc:build` for the feature edit's own quick verification, `arc:fix` to repair a failing case `arc:test` reproduced, `arc:security`/`arc:audit` for security and non-functional testing, and `arc:frontend` for the frontend UI test surface.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `scope` | string | no | Module, package, feature, or full suite under test |
| `platform` | enum | no | `go`, `rust`, `android`, `harmonyos`, `frontend`, or `auto` |
| `layers` | string | no | Layers to enable: unit, integration, contract, e2e, coverage, fuzz, property, benchmark, profile, load |
| `priority` | enum | no | `P0` default, `P1` recommended, `P2` on-demand |
| `sla` | string | no | Latency/throughput target that gates performance tests |
| `target_url` | string | no | Authorized running app or URL for E2E/load |
| `verification` | string | no | Expected command to run the suite |

## Outputs

```text
Test Handoff
- Scope and platform/language
- Layers enabled + layers skipped (with reason)
- Commands run and pass/fail per layer
- Coverage summary (line/branch) + uncovered high-risk gaps
- Functional result (unit / integration / contract / E2E)
- Robustness result (fuzz / property), when enabled
- Performance result (benchmark / load), judged separately, vs baseline/SLA
- Profiling artifacts (flame graph / pprof / trace) and located hotspots, when profiling ran
- Flaky / quarantined tests
- Failing-code handoff to arc:fix
- Non-functional/security handoff to arc:security / arc:audit
- Frontend UI test handoff via arc:frontend
- Task docs handoff (arc:sdlc), when large
- Lark / .lark.json / task_base handoff, if applicable
- Residual risks
```
