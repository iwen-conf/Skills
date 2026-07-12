# Test Scope Matrix

Use this reference when `arc:test` decides which test layers to enable, in what order, and where to draw the failure boundary. Enable by risk — never spread every type to tick boxes.

## The One Picture

```text
                    ┌─ Performance: benchmark / load / stress
                    │
  Correctness ──────┼─ unit → integration → contract → E2E
                    │
                    ├─ Exploratory: fuzz / property / mutation
                    │
                    └─ Metrics: coverage / flaky / regression gate
```

## Layer 1 — Functional Correctness (base, almost always)

| Type | Purpose | Required when |
|---|---|---|
| Unit (单元) | Function/module logic, boundaries, error paths | Business logic, parsing, algorithms, state machines |
| Integration (集成) | Module collaboration, DB, messaging, HTTP edges | There is I/O or a cross-layer contract |
| Contract / API (契约) | Request/response shape, error codes, compatibility | External APIs and service-to-service interfaces |
| E2E / Acceptance (验收) | Critical user journeys | Few and curated main paths (~5%) |

Only-unit suites go falsely green without integration; integration exists so a real I/O or cross-layer break is caught.

## Layer 2 — Quality Metrics (gate, almost always)

| Type | Purpose | Note |
|---|---|---|
| Coverage (覆盖率) | Find code with no test at all | A signal, not a target; chasing 100% blindly is pointless |
| Branch / condition coverage | Closer to real risk than line coverage | Key modules may require a higher bar |
| Regression suite (回归) | Old behavior stays intact after changes | CI default; smoke ⊂ regression |

Reasonable default: ~70–85% line coverage on **core business** as a reference, prioritizing key paths and error branches over a global number.

## Layer 3 — Robustness / Exploratory (high value, per module)

| Type | Purpose | Typical surface |
|---|---|---|
| Fuzz (模糊) | Malformed input, parsers, protocols, deserialization | Parsing, codecs, network packets, file formats, security edges |
| Property-based (属性) | Invariants, round-trip, commutativity, idempotence | Encode/decode round-trip, sort stability, idempotent ops |
| Mutation (变异) | Tests whether the tests are good | After the suite matures; high cost |

Go has built-in fuzzing (`testing.F`); Rust uses `cargo-fuzz`/libFuzzer, with `proptest` for property tests. Fuzz and property tests are complementary: one hunts random crashes, the other checks invariants.

## Layer 4 — Performance & Capacity (with SLA or hotspot)

| Type | Purpose |
|---|---|
| Benchmark / micro-benchmark | Hotspot functions, allocations, algorithmic regressions |
| Profiling / flame graph (性能检测) | Locate CPU / allocation / lock hotspots; read as a flame graph before optimizing |
| Load | Latency/throughput under expected concurrency |
| Stress / limit | Find the breaking point and recovery behavior |
| Capacity planning | Resource curves, basis for scaling decisions |

Go: `testing.B` + optional `benchstat`, plus `go tool pprof` / `go tool trace` for CPU/alloc profiles and flame graphs; Rust: Criterion, plus `cargo flamegraph` / `samply`. Load tier uses external tools (k6, vegeta, ghz). Deep platform profiling routes to `perfetto-trace-analysis` / `perfetto-sql` (Android/system trace) and `web-perf` (frontend). Keep this out of the functional framework and judge failure separately.

**Benchmark vs profiling:** a benchmark tells you *how much* (and guards against regression); a profile/flame graph tells you *where* the cost is. Profile to find the hotspot, fix it, then re-benchmark to prove the win — never optimize on a guess.

## Layer 5 — Security & Other Non-Functional (by product risk)

| Type | Purpose |
|---|---|
| Security / pen-test, SAST / dependency scan | Vulnerabilities and supply chain |
| Compatibility | Multiple platforms and runtime versions |
| Chaos | Distributed fault tolerance (when distributed) |
| Observability verification | Do critical errors emit logs/metrics/traces |

Split this with `arc:security` / `arc:audit`; do not rebuild it inside `arc:test`.

## Default Strategy (P0–P2)

| Priority | Scope | Guidance |
|---|---|---|
| P0 default | Unit + key integration + CI regression | Every change with logic |
| P0 metrics | Coverage (line/package) + fail-blocking on key packages | Report trend, not number worship |
| P1 recommended | Fuzz (parsing/boundary input), API contract | High security and stability payoff |
| P1 with SLA | Benchmark + baseline comparison (guard against perf regression) | Only watch hotspot paths |
| P2 on demand | Load/stress, property, mutation, chaos, full-chain E2E | Only with a clear risk |

## Scope Paragraph (canonical wording)

Test scope defaults to: (1) functional — unit, integration, contract, a little E2E; (2) metrics — coverage and regression gate; (3) robustness — fuzzing (and optional property tests); (4) performance — micro-benchmark, profiling/flame graph on hotspots, and, with an SLA, load/stress; (5) security/compatibility and other non-functional — in cooperation with `arc:audit`/`arc:security`, not duplicated. Enable by risk; do not roll out every type just to cover the taxonomy.
