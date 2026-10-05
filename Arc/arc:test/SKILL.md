---
name: arc:test
version: 1.2.1
description: >
  Designs risk-driven test scope and cases, then runs unit, integration, contract,
  E2E, fuzz, property and performance tests with platform-native tools. Use for
  写测试, 测试范围, 用例设计, 覆盖率, 回归, E2E, fuzz, benchmark, Maestro, or quality gate.
  Not for repairing production code or security scanning.
---
# arc:test

## When to Use

Design, add, run or review tests and quality gates for a concrete change or contract.
Select evidence by risk and consequence. Ordinary low-risk work may reuse adequate
existing checks without adding tests. Production repair belongs to `arc:fix`;
specialist security assessment belongs to `arc:security` / `arc:audit`.

## Intent Router

| Condition | Read / use |
|---|---|
| Scope, case design, assertion strength or generator blind spots | [Test case design](references/test-case-design.md) |
| Credible boundary, enabled layers, priority or execution cadence | [Test scope matrix](references/test-scope-matrix.md) |
| Platform commands, coverage or profiling | [Native stacks](references/test-stacks.md); read only the target platform |
| Business invariants, transaction/retry effects, fault/recovery or release evidence | `arc:reliability` defines claims/oracles; execute its bindings here |
| Recommending a production fix from a failed/flaky test | **Evidence-first root-cause repair** in [docs/execution-truth.md](../../docs/execution-truth.md), then `arc:fix` |
| Required implementation/test seam | `arc:build` or `arc:fix` |
| Security scanners / penetration testing | `arc:security` / `arc:audit` |
| Frontend interaction, visibility, clickability and layout | `arc:frontend` |
| Large / multi-module / cross-platform / tracked test build-out | `arc:sdlc` |
| Existing `.lark.json` or explicit Lark activation | `arc:docs` for all remote records |

Accept an existing reliability handoff without repeating its planning. Keep normal
suite work here; do not introduce a second reliability framework.

## Red Lines

- No PASS without current executed evidence, no expected result copied from the
  implementation, and no coverage percentage/test count as business proof.
- No universal coverage threshold, E2E ratio, priority-by-test-type, or test-type
  sprawl. Retain justified project-owned gates and historical regressions.
- Never hide a failure by weakening assertions, deleting failing tests, blanket
  retries, quarantine or longer timeouts. Preserve every attempted rerun.
- Separate functional, robustness and performance runs/verdicts. Performance safety
  needs a meaningful benchmark/baseline; a fast wrong result is still wrong.
- No bespoke cross-language test framework, local PostgreSQL server or local
  container runtime by default. Missing required environments remain BLOCKED.
- No shared/third-party load, stress or E2E outside existing explicit authorization.
  Reuse authorization when target, operations and data scope are already covered.
- No production repair from an unconfirmed issue/cause; no implicit Lark activation.

## Case and Execution Contract

1. Discover affected contracts, state, tests, runners, fixtures, CI and environment
   with `arc:idx`. Identify required claims, user consequences and excluded scope;
   select the smallest boundary that can expose each failure. Run highest-risk
   evidence early; capacity, recovery and upgrade checks may be release-critical.
2. Design boundary/decision, state-transition and operation-history cases with
   representative data. Cover relevant error, malformed-input, empty-state and
   forbidden-effect paths. Define EXPECTED/FORBIDDEN outcomes and an independent
   oracle; verify identity/content and side effects, with explicit async deadlines.
3. Name the realistic defect important assertions should detect. For large input or
   state spaces, assess fuzz/property tests and generator reachability; preserve
   counterexamples. Use known-bad versions, negative controls or isolated mutation
   only when they add meaningful confidence. Do not test merely to raise counts.
4. Bind or reuse native tests and credible environments. Implement only missing
   evidence; send production edits through build/fix. For large tracked work, keep
   task cards and progress current under `arc:sdlc`. Keep regression as the CI default
   and smoke as a labeled subset, not a replacement.
5. Execute the selected checks and retain candidate/version, commands, environment,
   observations and artifacts. Report the coverage metric actually supported,
   measured scope and high-risk gaps; line coverage cannot establish branch coverage.
   Stop when required claims and project gates pass; broaden only for a remaining risk.

## Native Tools and Environments

| Platform | Default |
|---|---|
| Go | `testing`, `go test`, `-race`, `-cover`, `testing.B`, `testing.F` |
| Rust | `cargo test`, Criterion, `cargo-fuzz`, `proptest`; coverage via `cargo llvm-cov` |
| Android | JUnit/Kotlin for pure rules, Maestro for UI/flows |
| HarmonyOS | arkxtest: `@ohos/hypium` and UiTest |
| Web frontend | Existing runner; defaults Vitest + Testing Library, Playwright; agent-browser for exploration |

Read concrete commands from native stacks only for the detected platform. Follow
current user/project environment rules: remote Docker/PostgreSQL; Android tests on
remote redroid when configured, local AVD lifecycle through avdslim when explicitly
needed. Redroid without GMS cannot prove Google-dependent behavior. For PostgreSQL
semantics or release claims, load `arc-dbqual` and retain its fail-closed qualification
requirements; framework self-tests do not establish database release readiness.

Frontend acceptance includes a completed interaction loop and consistent return
state, visible controls, clickability/enabled state and readable responsive layout.

For performance, compare representative workloads with the project baseline/SLA
when one exists. Capture CPU/allocation/lock profiles and inspect flame graphs before
optimization; then rerun benchmarks. Android traces route to perfetto-trace-analysis /
perfetto-sql, frontend profiling to web-perf. A single timing is not a root cause.

## Results and Handoff

Keep `PASS / FAIL / BLOCKED / NOT_RUN / FLAKY` distinct. Diagnose harness/environment
versus product failures; confirmed product failures go to `arc:fix` with observed and
expected behavior, evidence and affected boundary. Investigation may continue when
root cause is unknown, but it is not a completed repair.

Quarantine must record owner, reason, repair condition and lost claim coverage.
A required claim remains blocked until fixed/replacement evidence or a separately
recorded authorized risk decision exists; accepting risk is not a test PASS.

Report required/excluded risks, case → oracle → native binding → current evidence,
commands and verdicts, coverage gaps, separate performance results, retained flaky
attempts and residual risk. Include profiling/generator evidence when performed.
Synchronize tracked docs; if Lark is active, read `.lark.json` and use `arc:docs` for
status, `task_base` and durable records. State any unavailable verification explicitly.
