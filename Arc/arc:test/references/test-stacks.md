# Test Stacks Reference

Concrete, native tooling and commands per platform. `arc:test` picks the stack from detected markers and never introduces a bespoke cross-language runner.

## Selection by marker

| Marker | Platform | Default stack |
|---|---|---|
| `go.mod` | Go | built-in `testing` (`go test`, `testing.B`, `testing.F`) |
| `Cargo.toml` | Rust | `cargo test`, Criterion, `cargo-fuzz`, `proptest` |
| `AndroidManifest.xml` / `build.gradle` / `.maestro/` | Android (native or RN) | Maestro for UI/flows; JUnit for pure logic |
| `oh-package.json5` / `*.test.ets` | HarmonyOS | arkxtest (`@ohos/hypium` + `UiTest`) |
| `package.json` (React 19 default) | Frontend web | Vitest + Testing Library + Playwright; `agent-browser` |

## Go — built-in `testing`

```bash
go test ./...                                   # unit (table-driven, t.Run subtests)
go test -race ./...                             # data-race detection
go test -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out               # coverage summary
go tool cover -html=coverage.out               # per-line HTML

go test -bench=. -benchmem ./...               # benchmarks (run separately)
go test -bench=. -count=10 ./... > new.txt && benchstat old.txt new.txt

go test -run=^$ -fuzz=FuzzParse -fuzztime=30s ./internal/parser   # native fuzz
go test -tags=integration ./...                # integration behind build tags
```

- Contract/API: `net/http/httptest` for handler and client contracts.
- Property: standard `testing/quick`, or `pgregory.net/rapid` when richer generators are needed.
- Keep fuzz corpora under `testdata/fuzz/`; a crash becomes a permanent regression seed.

## Rust — `cargo` + Criterion + cargo-fuzz + proptest

```bash
cargo test                                      # unit (in-module) + integration (tests/)
cargo test --doc                                # doctests
cargo llvm-cov --branch --summary-only          # coverage (line + branch); or cargo tarpaulin

cargo bench                                      # Criterion benchmarks (run separately)
cargo fuzz run parse_target                      # cargo-fuzz / libFuzzer (nightly toolchain)
cargo mutants                                    # optional mutation testing (mature suites)
```

- Property: `proptest` or `quickcheck` for invariants and round-trip.
- Integration tests live in `tests/`; each file is its own crate.

## Android — Maestro (UI / flows)

Maestro is black-box and works for native Android, React Native (the `arc:frontend` mobile default), and Flutter.

```yaml
# .maestro/login.yaml
appId: com.example.app
---
- launchApp
- assertVisible: "Sign in"          # page visibility
- tapOn: "Email"                    # control clickable
- inputText: "user@example.com"
- tapOn: "Continue"
- assertVisible: "Welcome"          # interaction closed-loop reached a consistent state
```

```bash
maestro test .maestro/                          # run all flows
maestro test .maestro/login.yaml --format junit # CI report
maestro studio                                  # author/inspect flows interactively
```

- Pure business logic still uses JUnit/Kotlin (`./gradlew test`); on-device instrumentation via `./gradlew connectedAndroidTest` when needed.
- Key assertions: `assertVisible` / `assertNotVisible` (visibility), `tapOn` (clickable), `extendedWaitUntil` (stable waits, not sleeps).

## HarmonyOS — arkxtest (`@ohos/hypium` + `UiTest`)

Unit tests use the Hypium `describe/it/expect` API; UI automation uses `UiTest` (`Driver`, `ON`, `Component`). Test files are `*.test.ets`.

```typescript
// unit: Hypium
import { describe, it, expect } from '@ohos/hypium';
export default function calcTest() {
  describe('calcTest', () => {
    it('adds', 0, () => { expect(add(2, 3)).assertEqual(5); });
  });
}

// UI: UiTest
import { Driver, ON } from '@ohos.UiTest';
const driver = Driver.create();
await driver.assertComponentExist(ON.text('Home'));   // visibility
const btn = await driver.findComponent(ON.text('Start'));
await btn.click();                                     // clickable + interaction
```

```bash
# run via DevEco Studio test runner, or on device:
hdc shell aa test -b <bundle> -m <module> -s unittest OpenHarmonyTestRunner
```

## Frontend (web) — Vitest + Testing Library + Playwright

Aligns with the `arc:frontend` default stack (React 19 + TypeScript + Vite). Route interaction/visual checks through `arc:frontend`.

```bash
vitest run --coverage                           # unit/component; branch coverage (v8/istanbul)
playwright test                                 # interaction / E2E
```

Frontend acceptance the suite must prove (the four checks):

- **Interaction closed-loop (交互闭环)**: a journey completes and the UI returns to a consistent, non-broken state — assert the end state, not just the click.
- **Visibility (页面可见性)**: key elements are present and visible (`toBeVisible`), not hidden behind system bars, overlapped, or overflowed.
- **Clickability (按钮可点击)**: interactive controls are reachable and enabled (`getByRole('button')`, correct `disabled`/loading states, adequate hit area).
- **Readability (样式布局可读性)**: no overlapping/truncated text, sufficient contrast, and a stable responsive layout at target viewports.

```typescript
// Playwright: closed-loop + visibility + clickability
await page.getByRole('button', { name: 'Add to cart' }).click();
await expect(page.getByText('Added')).toBeVisible();          // visibility
await expect(page.getByRole('button', { name: 'Checkout' })).toBeEnabled();  // clickable
```

- Use `agent-browser` for exploratory interaction verification, screenshots, and dogfood-style passes when scripted E2E is not yet in place.

## Profiling & flame graphs (性能检测)

Profiling locates *where* time and memory go and renders it as a **flame graph (火焰图)** — distinct from benchmarks, which measure *how much*. Loop: reproduce the hotspot → capture a profile → read the flame graph (widest frame = biggest cost) → fix → re-benchmark to confirm. Run separately from the functional pass.

### Go — pprof + trace

```bash
go test -cpuprofile cpu.out -memprofile mem.out -bench=. ./pkg
go tool pprof -http=:8080 cpu.out       # interactive Flame Graph / Top / Graph / Source
go tool pprof -http=:8080 mem.out       # allocation flame graph
go tool trace trace.out                 # execution timeline: scheduler, GC, blocking
# live service: import _ "net/http/pprof", then:
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile?seconds=30
```

Profiles: `cpu`, `heap`, `allocs`, `goroutine`, `block`, `mutex`, `threadcreate`. Continuous profiling (optional): Pyroscope / Parca.

### Rust — cargo-flamegraph / samply / heap

```bash
cargo flamegraph --bench my_bench       # perf + inferno → flamegraph.svg
cargo flamegraph --bin app              # profile a binary run
samply record ./target/release/app      # sampling profiler → Firefox Profiler UI
# heap: dhat (feature) or valgrind --tool=massif ./target/release/app
```

`pprof-rs` emits pprof + flame graphs from inside the app; Criterion can attach a profiler via `criterion-perf-events` / `pprof`.

### Linux general — perf + FlameGraph/inferno

```bash
perf record -g -- ./app
perf script | inferno-collapse-perf | inferno-flamegraph > flame.svg
# or Brendan Gregg's scripts: stackcollapse-perf.pl | flamegraph.pl
```

### Android — Perfetto / simpleperf

- System trace and flame charts via **Perfetto**; query the trace through the `perfetto-trace-analysis` and `perfetto-sql` skills.
- `simpleperf record -g` + `simpleperf report-sample --show-callchain` / `report --flamegraph`; Android Studio CPU Profiler (Flame Chart / Call Chart); Jetpack `macrobenchmark` + baseline profiles for startup/scroll/jank.

### HarmonyOS — DevEco Profiler / SmartPerf

- DevEco Studio **Profiler**: Time (CPU call-stack flame graph), Allocation, Frame, Snapshot.
- SmartPerf-Host / SmartPerf Editor; command-line `hitrace` (bytrace) and `hiperf` for sampling → flame graph.

### Frontend — DevTools / web-vitals

- Chrome DevTools **Performance** panel → flame chart; React DevTools **Profiler** → render flame graph; Lighthouse + `web-vitals` (LCP/INP/CLS) for lab and field metrics.
- Deep frontend performance work routes to the `web-perf` skill.
