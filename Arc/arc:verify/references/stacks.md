# Stacks

Native tools and commands per platform; read only the detected platform's section. Versions
checked 2026-10: Go 1.26, Vitest 5, Jest 30, Playwright 1.63, pytest 9, Hypothesis 6, k6 2.x,
vegeta 12, Maestro CLI 2.x. Record the version actually used in the evidence.

## Selection by marker

| Marker | Default |
|---|---|
| `go.mod` | `testing` + `go test` |
| `package.json` | the existing runner; new web projects: Vitest + Testing Library + Playwright |
| `pyproject.toml` / `pytest.ini` / `setup.cfg` | pytest |
| `Cargo.toml` | `cargo test` (`cargo nextest run` when configured) |
| `AndroidManifest.xml` / `build.gradle(.kts)` / `.maestro/` | JUnit for logic, Maestro for flows |
| `oh-package.json5` / `*.test.ets` | arkxtest: `@ohos/hypium` + `UiTest` |
| `.xcodeproj` / `.xcworkspace` | Xcode test plans; Swift Testing/XCTest for supported logic targets, XCUITest for app UI |
| `Package.swift` | `swift test` for package logic; app/platform integration still needs the actual app target |

Run the project's own entry (`make test`, `vp test`, the CI job) first when it exists. Never
introduce a bespoke cross-language runner.

## Go

```bash
go test ./...                                          # table-driven, t.Run subtests
go test -race ./...                                    # every concurrency claim
go test -run '^TestName$' -count=1 ./pkg               # -count=1 bypasses the test cache for evidence
go test -covermode=atomic -coverprofile=cover.out ./... && go tool cover -func=cover.out
go test -run '^$' -bench=. -benchmem -count=10 ./pkg > new.txt && benchstat old.txt new.txt
go test -run '^$' -fuzz=FuzzParse -fuzztime=30s ./internal/parser
go test -tags=integration ./...                        # when the repo gates integration by tag
go test -artifacts -outputdir=.arc/artifacts/<task-slug>/logs ./pkg   # keeps t.ArtifactDir() files
```

- Benchmarks use `for b.Loop() { … }` (Go 1.24+) instead of `b.N` loops. `benchstat` is
  `golang.org/x/perf/cmd/benchstat`.
- Fake time and goroutine coordination: `testing/synctest` with `synctest.Test(t, func(t *testing.T){…})`
  and `synctest.Wait()` (Go 1.25+; `synctest.Run` is gone in 1.26). It does not model database locks.
- Contracts: `net/http/httptest`. Properties and state machines: `pgregory.net/rapid`
  ([model-conformance](model-conformance.md)); `testing/quick` is frozen and does not shrink.
- The fuzz corpus lives in `testdata/fuzz/`; a crasher stays as a permanent regression seed.
  Campaign rules, oracles and other stacks: [fuzz-campaign.md](fuzz-campaign.md).
- Check goroutine lifetime, context cancellation, channel ownership and connection cleanup.

## TypeScript / JavaScript

```bash
vp test                           # configured Vite+ unit/component tests
vp test --coverage                # coverage provider must match the installed runner
vpx jest --coverage               # existing Jest project
vpx playwright test               # E2E
```

- Vitest 5: Node ≥ 22.12 and Vite ≥ 6.4; `clearMocks` defaults to true; an un-awaited async
  assertion fails; `-t` filters join suite and test names with ` > `; reporter output defaults to
  `.vitest/`; coverage `include`/`exclude` match paths relative to the project root.
- Vite+ owns frontend dev/check/test/build; bun owns dependencies and TS/JS execution. Use
  `bun install` for project dependencies, `vpx`/`vp dlx` for one-off tools and `vp install -g`
  for an explicitly authorized global CLI install. Do not use npm/npx/yarn to install packages.
- Component queries by role or label (`getByRole`, `getByLabelText`), not CSS selectors.
- Properties and model runs: `fast-check` ([model-conformance](model-conformance.md)). HTTP stubs (MSW or similar) only for dependencies outside the claim,
  never for the API whose contract is under test.

## Python

```bash
python -m pytest -q
python -m pytest --cov=<pkg> --cov-branch --cov-report=term-missing     # pytest-cov
python -m pytest -p no:cacheprovider -x "tests/test_x.py::test_name"
```

Properties: Hypothesis `@given`, stateful `RuleBasedStateMachine` ([model-conformance](model-conformance.md));
keep the `.hypothesis/` database or pin failures with `@example`.
Load for Python services: Locust.

## Rust

```bash
cargo test                                 # or cargo nextest run
cargo llvm-cov --branch --summary-only     # branch coverage requires a nightly toolchain
cargo bench                                # Criterion, separate run
cargo fuzz run <target>                    # nightly toolchain
```

`proptest` (`proptest-state-machine` for stateful) for properties; Kani for bounded proofs
([model-conformance](model-conformance.md) §6); `loom` for small concurrency primitives (it does not model the full
memory model); Miri (`cargo +nightly miri test`) or sanitizers for undefined behavior in `unsafe`
code; `cargo mutants` scoped to important claims. Respect `#![forbid(unsafe_code)]`. Do not change
the default toolchain for one test.

## Android and HarmonyOS

```bash
./gradlew test                         # JUnit/Kotlin pure logic
./gradlew connectedAndroidTest         # instrumentation, when a device target exists
maestro test .maestro/ --format junit --test-output-dir .arc/artifacts/<task-slug>/logs
maestro hierarchy                      # view tree when writing selectors
hdc shell aa test -b <bundle> -m <module> -s unittest OpenHarmonyTestRunner
```

- Maestro steps: `assertVisible` / `assertNotVisible`, `tapOn`, `extendedWaitUntil`; never fixed
  sleeps. Maestro CLI 2.x has no `studio` subcommand.
- HarmonyOS: Hypium `describe/it/expect` for units; `UiTest` `Driver.create()`, `ON.text(...)`,
  `driver.assertComponentExist(...)`, `findComponent(...).click()` for UI.
- Devices ([android-execution.md](../../../docs/android-execution.md)): remote redroid when the
  project configures it; local AVD lifecycle only through `avdslim` and only when the user asks. Redroid without GMS cannot prove Google-service behavior.

## Apple and other native clients

- iOS/iPadOS/macOS: use the existing Xcode scheme/test plan and an available target destination.
  `xcodebuild test -scheme <actual-scheme> -destination <actual-destination>` is a command shape;
  discover the real project/workspace, scheme and device before running. Use Swift Testing or
  XCTest as supported by the test target, XCUITest for UI. `swift test` alone does not prove an
  iOS app's lifecycle or permissions.
- Check the changed native behavior: system navigation/windows, lifecycle/state restoration,
  permissions, input/focus, safe areas, accessibility/text scaling and system appearance as applicable.
- Windows/Linux: use the chosen platform's existing build, test runner and accessibility/UI
  automation on that target; no browser-only substitute for a native window claim.
- Mini Programs: use platform developer tools/automation and a device for claims involving actual
  platform APIs, permissions or rendering. A desktop browser mock is not platform evidence.
- Follow [native direction](../../_shared/conventions/native-platforms.md). Missing SDK, signing,
  runner or device blocks the affected native claim; it does not justify switching frameworks.

## UI checks

Every UI claim proves four checks on the target surface. Web matcher names below are examples;
native UI uses its platform semantics and automation:

1. **Interaction loop**: the journey completes and the UI returns to a consistent state; assert
   the end state, not the click.
2. **Visibility**: key elements are visible (`toBeVisible`), not under system bars, overlapped or
   overflowed.
3. **Clickability**: controls are reachable and enabled (`getByRole('button')`, `toBeEnabled`),
   with correct disabled/loading states and an adequate hit area.
4. **Readability**: no overlapping or truncated text, sufficient contrast, stable responsive layout.

Also assert that loading, empty, error and permission-denied render through different branches
and that empty never uses error styling ([frontend conventions](../../_shared/conventions/frontend.md)),
plus labels, visible focus and keyboard reachability.

```ts
await page.getByRole('button', { name: 'Add to cart' }).click();
await expect(page.getByText('Added')).toBeVisible();
await expect(page.getByRole('button', { name: 'Checkout' })).toBeEnabled();
```

- Save console errors, failed network requests and screenshots to `.arc/artifacts/<task-slug>/logs/`.
- Starting the app, recon before act and business-end-state oracles: [web-e2e.md](web-e2e.md).
  Without a scripted runner: `agent-browser open <url>`, `snapshot -i`, `screenshot <path>`,
  `console`, `errors` for an exploratory pass; scripted E2E stays `NOT_RUN`.
- Native without a runner: record authorized device/platform observations and label automated
  checks `NOT_RUN`; do not substitute browser evidence.
- Web Playwright CI runs with `--fail-on-flaky-tests` so a pass-on-retry is visible.

## Load and benchmark

```bash
k6 run script.js                       # thresholds in options; threshold breach exits 99
echo "GET https://<test-host>/path" | vegeta attack -rate=50/s -duration=60s | tee r.bin | vegeta report
```

- k6 2.x: open-model load uses `constant-arrival-rate` / `ramping-arrival-rate`; write JSON with
  `handleSummary()` (preferred) or `--summary-export`. Removed in 2.x: `externally-controlled`,
  `--no-summary` (use `--summary-mode=disabled`), `k6 cloud script.js` (use `k6 cloud run`).
- vegeta attacks at a constant rate (open model) by default.
- Network faults (latency, reset, bandwidth, timeout) between services: Toxiproxy from its official
  GHCR image on the remote Docker host, tag and digest pinned.
- SQL-level workloads (`pgbench`) belong to db mode.
- Targets: the project's test environment, or a shared target under an authorization naming
  target, rate, duration and stop condition.

## Profile capture

Capture evidence for a performance verdict; optimizing belongs to `arc:code`.

| Stack | Capture |
|---|---|
| Go | `go test -cpuprofile cpu.out -memprofile mem.out -bench=. ./pkg`; `go tool pprof -top cpu.out`; profiles `cpu heap allocs goroutine block mutex` |
| Rust | `cargo flamegraph --bench <name>`; `samply record <bin>` |
| Linux | `perf record -g -- <cmd>` |
| Android | Perfetto trace → `perfetto-trace-analysis`; `simpleperf record -g`; Macrobenchmark |
| HarmonyOS | DevEco Profiler, `hiperf`, `hitrace` |
| Web | DevTools Performance, React Profiler, Lighthouse / `web-vitals` → [web-perf.md](web-perf.md) |

## Repeat runs for flaky triage

Use these only for hermetic tests or authorized targets whose initial state is independently
reconstructed before every iteration. Preserve the first FAIL and every attempt, use a deadline,
and stop when the actual state is unknown; repeat flags never grant permission to replay effects.

| Stack | Command |
|---|---|
| Go | `go test -run '^TestX$' -count=50 -race ./pkg`; vary `-cpu=1,4` and `-shuffle=on` |
| Vitest / Jest | the project's bounded repeat runner via `vp test` or `vpx jest`; retain every attempt and aggregate a nonzero exit |
| pytest | the project's bounded repeat entry invoking `python -m pytest -q "tests/x.py::test_y"`; retain each exit and return nonzero if any iteration fails |
| Playwright | `vpx playwright test <file> --repeat-each=50 --retries=0` |

## Environment

| Step | Rule |
|---|---|
| Prepare | reuse the authorized environment and clients; name the resources this run owns |
| Reset / fixtures | reset only data this run owns; never mix the standalone test environment with a sync test environment |
| Healthcheck | app, dependencies and every oracle observer ready before the run |
| Isolation | never consume production topics, join production leader election, or send notifications or payments to real targets |
| Teardown | release owned resources on failure and timeout; record teardown failures; never delete others' data |

- Docker runs only through a user-specified remote context/SSH after confirming the target
  daemon; project Testcontainers config is not permission for a local runtime.
- PostgreSQL: an existing remote test instance or CI only. Installing `psql`, `libpq` or `usql`
  must not pull a server package; use `usql` for interactive SQL where the project does. SSH
  tunnels bind `127.0.0.1`.
- Test connection config for database, cache, queue, index or object storage lives in hidden
  `.<name>.json` files that separate standalone-test from sync-test environments; fields and seeds
  come from the project; credentials come from existing secret/env mechanisms; never guess a DSN.
- Tool acquisition order: built-in → project dependency with lockfile → user-scoped install from
  the official registry → pinned container image on the remote host. No `sudo`, no `curl | sh`,
  no privileged daemon. Record installed name and version; pin versions and image digests in CI.
- Harness code you add (observers, reconcilers, generators) gets its own hermetic unit test;
  real-database checks of the harness run as explicit integration tests. An observer that loses
  data fails closed.
