# Hostile-Input Fuzz Campaigns

Feed code that parses untrusted input with generated, mutated and malformed data until a
business or safety property breaks. A fuzz result is evidence only for the harness, corpus,
budget and coverage actually reached. Scanning or attacking deployed systems is `arc:audit`
(authorized DAST); SQL parameter and contention fuzzing is db mode.

## 1. Targets

Fuzz where bytes or strings cross a trust boundary into logic: parsers and decoders (JSON/XML/
YAML/CSV, binary frames, protocol messages), deserializers, file import and upload processing
(archives, images, documents), query, filter or template languages, URL and path handling, token
and signature parsing, compression, money and date parsing, and state machines driven by
external messages.

Campaign card, one per target (merge into the change verification card):

```text
Target / entry function / trust boundary / input grammar or format
Oracles (§3) / harness file / seed corpus + dictionary / sanitizers or race detector
Budget per cadence (project-given, else proposed and recorded) / stop condition / owner
```

## 2. Harness rules

- One entry per target that calls the real parsing code the production path uses; no reimplementation.
- Deterministic: no wall clock, network, real filesystem or shared global state; reset state per
  input. Bounded input size and per-input timeout; record both.
- Structure-aware where the format needs it: typed fuzz arguments (Go), `arbitrary` (Rust),
  `FuzzedDataProvider` (libFuzzer, Atheris, Jazzer), or a grammar/model-based generator, so
  inputs reach logic past the first length or checksum check.
- Expected rejection is not a finding: catch only the documented error types for invalid input.
  Any other exception, panic, crash, hang or memory blow-up is a finding.
- Measure executions per second on the seed corpus; a slow harness is usually doing I/O or setup
  per input. Fix the harness before spending budget.
- Harness code gets its own small unit test (it reaches the target and rejects one known-bad input).

## 3. Oracles beyond "it did not crash"

Select properties only where the confirmed contract promises them. Normalization, lossy formats,
signed amounts and decompression can legitimately violate generic round-trip or size examples.

| Oracle | Assertion |
|---|---|
| Safety | no crash, panic, sanitizer report (ASan, UBSan, MSan), data race, hang or unbounded allocation |
| Documented errors only | invalid input yields the contracted error type or code, never an unexpected exception or partial state |
| Round trip | lossless contract: `decode(encode(x)) == x`; otherwise compare the contracted canonical/semantic form |
| Idempotence | `normalize(normalize(x)) == normalize(x)` |
| Differential | same verdict and value as a reference implementation, an older version or a stricter spec parser, after aligning allowed differences |
| Invariant | confirmed domain bounds, path/tenant isolation and explicit output allocation limit; never infer nonnegative amounts or output ≤ input without that contract |

## 4. Tooling

| Stack | Run | Corpus, minimize, coverage |
|---|---|---|
| Go | `go test -run '^$' -fuzz=FuzzParse -fuzztime=10m ./internal/parser` | failing inputs land in `testdata/fuzz/FuzzParse/` and replay in plain `go test`; the generated corpus stays in `$(go env GOCACHE)/fuzz/`, so copy it into `testdata` before `go test -run FuzzParse -coverprofile=cover.out` |
| Rust | `cargo fuzz run parse -- -max_total_time=600` (nightly; ASan by default) | `cargo fuzz cmin parse`, `cargo fuzz tmin parse <crash>`, `cargo fuzz coverage parse` |
| C/C++ | libFuzzer: build with `-fsanitize=fuzzer,address,undefined`; run with `-max_total_time=`, `-max_len=`, `-dict=`, `-jobs=`/`-workers=` | `-merge=1` to minimize the corpus; `-minimize_crash=1`; AFL++: `afl-fuzz -i seeds -o out -- ./target @@` |
| Python | Atheris: import the target inside `with atheris.instrument_imports():`, then `atheris.Setup(sys.argv, TestOneInput)` and `atheris.Fuzz()` | libFuzzer flags apply; Hypothesis for property tests |
| JVM | Jazzer `@FuzzTest` (JUnit 5) with `FuzzedDataProvider` | crashing inputs become regression inputs |
| JavaScript/TypeScript | Jazzer.js; `fast-check` for properties | keep failing seeds in the repository |
| HTTP API with OpenAPI | Schemathesis against the test environment; check the CLI of the installed version before running | each failure becomes an API contract test |

Record the tool, version, command, seed and corpus hash in the evidence.

## 5. Seeds and dictionaries

Start from valid, sanitized real samples, existing test inputs, spec or RFC examples and every
past crasher. Add a dictionary of format tokens (keywords, delimiters, magic bytes, boundary
numbers, encodings). Minimize the corpus before long runs and commit the minimized seeds the
project keeps.

## 6. Campaign phases

| Cadence | Run | Exit |
|---|---|---|
| PR | corpus replay as a normal test (every committed seed and past crasher), plus a short smoke run on targets the change touches | replay green; smoke without findings |
| Nightly | each target for its budget, in parallel, with sanitizers or the race detector | findings triaged; corpus grows |
| Pre-release | campaign on the release build until coverage plateaus or the budget ends | coverage report reviewed, blocked branches handled |

After each long run, generate coverage over the corpus and list unreached branches inside the
trust-boundary code. For each blocked area add a dictionary entry (magic values), a test-build
seam or structure-aware generation (checksums, lengths), or a sequence harness (deep state).
Plateau means no new coverage over the last stretch of the run; record when it started.

## 7. Triage

1. Reproduce with the single saved input on the same build.
2. Minimize it and confirm the failure cause stays the same.
3. Deduplicate by sanitizer report or top stack frames.
4. Classify: crash, hang, memory, assertion, oracle violation; root cause per [execution-truth](../../../docs/execution-truth.md) §6.
5. Each unique defect: `FAIL` with the reproducer and a repair packet to `arc:code`; the input
   joins the corpus and a plain regression test.

## 8. Verdicts

| Observation | Verdict |
|---|---|
| Oracle violated, crash, sanitizer report | `FAIL` + minimized reproducer |
| Budget spent, oracles held, coverage reached the target code | `PASS` for "harness H, corpus C, budget B, coverage X: no violation"; never "secure" or "bug-free" |
| Harness never reaches the target logic (coverage shows it) | `BLOCKED` until the harness is fixed |
| Required campaign not run this cycle | `NOT_RUN` with reason |
