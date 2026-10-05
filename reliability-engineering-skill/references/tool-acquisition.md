# Verification Tool Acquisition Policy

This reference defines how the skill acquires verification tooling. The guiding rule is: **a testing tool must not create more supply-chain or environment risk than the failure class it is intended to test.**

## Acquisition order

Prefer, in order:

1. language/runtime built-ins already present;
2. project-scoped dependency recorded by the project's normal package manager and lockfile;
3. user-scoped executable installed from the language's official registry/toolchain;
4. pinned container image for isolated services/tools;
5. system package or privileged daemon only when no safe project/user/container option exists.

The bootstrap script never uses `sudo`, never starts a privileged daemon, and never executes `curl | sh`.

## Rust catalog

- `cargo test`: built in.
- `cargo-nextest`: source install supported by official docs with `cargo install cargo-nextest --locked`; official docs also describe prebuilt and `cargo-binstall --secure` paths.
- `cargo-llvm-cov`: official project documents `cargo +stable install cargo-llvm-cov --locked`, prebuilt binaries, Homebrew, and artifact attestations for recent releases.
- `cargo-mutants`: mutation testing; nextest's official integration docs use `cargo install cargo-mutants` and support `--test-tool=nextest`.
- `cargo-audit`: RustSec dependency vulnerability audit; official RustSec docs install with `cargo install cargo-audit`.
- `cargo-fuzz`: official rust-fuzz project installs with `cargo install cargo-fuzz`; it requires nightly Rust, LLVM sanitizer support, x86-64/AArch64 Unix-like hosts, and a C++11 compiler.
- Miri: official rust-lang/miri usage installs through rustup on nightly (`rustup +nightly component add miri` or equivalent nightly toolchain install with the component).
- `proptest`: project dependency for generated property tests and shrinking.
- `loom`: project dependency for systematic Rust concurrency exploration. Loom explicitly does not model the entire C11 memory model; passing Loom is evidence, not proof of all possible hardware executions.
- Testcontainers for Rust: project dependency for real database/broker integration tests; requires a working container runtime.

## Go catalog

- `go test`: built in.
- `go test -race`: built-in race detector on supported platforms.
- native fuzzing: built in since Go 1.18; use `go test -fuzz=...`.
- coverage: built in; `go test -coverprofile=...` and `go tool cover`.
- `gotestsum`: optional structured runner/reporting, installed from `gotest.tools/gotestsum`.
- `govulncheck`: official Go vulnerability scanner, installed from `golang.org/x/vuln/cmd/govulncheck`.
- `staticcheck`: high-signal Go static analysis, officially installed with `go install honnef.co/go/tools/cmd/staticcheck@latest` or a pinned release.
- Testcontainers for Go: project dependency; requires a working container runtime.

## Cross-language fault tooling

- Shopify Toxiproxy: use the official GHCR image by default instead of modifying the host. This skill pins `ghcr.io/shopify/toxiproxy:2.12.0`; hardened CI may additionally pin the image digest.
- Docker/Podman: treated as a system prerequisite, never silently installed or started by the bootstrap script.

## Versioning rules

Local exploratory installs may use the latest registry release when the ecosystem's normal tool does so, but the observed version must be recorded in `.reliability/toolchain.lock.json`.

For CI or long-lived critical verification:

- pin tool versions where the installer supports it;
- pin container tags and preferably digests;
- preserve `Cargo.lock` / `go.sum` changes for project dependencies;
- review tool upgrades separately from behavioral changes when a verification result changes;
- record tool versions in failure artifacts when material to reproduction.

## Source references checked for this skill revision

- cargo-nextest installation: https://nexte.st/docs/installation/from-source/ and /pre-built-binaries/
- cargo-llvm-cov: https://github.com/taiki-e/cargo-llvm-cov
- cargo-mutants integration: https://nexte.st/docs/integrations/cargo-mutants/
- cargo-fuzz: https://github.com/rust-fuzz/cargo-fuzz
- Miri: https://github.com/rust-lang/miri
- Loom: https://docs.rs/loom/latest/loom/
- Proptest: https://docs.rs/proptest/latest/
- Go fuzzing: https://go.dev/doc/security/fuzz/
- Go race detector: https://go.dev/doc/articles/race_detector
- Go coverage: https://go.dev/doc/build-cover
- govulncheck: https://go.dev/doc/security/vuln/
- Staticcheck: https://staticcheck.dev/docs/getting-started/
- cargo-audit / RustSec: https://github.com/rustsec/rustsec/tree/main/cargo-audit
- gotestsum: https://github.com/gotestyourself/gotestsum
- Testcontainers Rust: https://rust.testcontainers.org/
- Testcontainers Go: https://golang.testcontainers.org/
- Toxiproxy: https://github.com/Shopify/toxiproxy and GHCR package page


## Business transaction tooling

- Psycopg 3 is the current-generation PostgreSQL adapter for Python. The skill pins `psycopg[binary]==3.3.5` inside `.reliability/py-tools`, a project-local verification venv. This keeps the harness separate from application dependencies and system Python.
- `pgbench` is PostgreSQL's official concurrent workload tool. It supports custom transaction scripts, multiple clients, weighted workloads, retry counts, and detailed serialization/deadlock failure reporting. Because it is distributed with PostgreSQL client/server packages, the bootstrapper detects it as a system prerequisite instead of silently installing host packages.
- If `pgbench` is unavailable, the Skill should prefer the application's own load harness or a containerized PostgreSQL client matching the target major version rather than weakening the test to mocks.

Official references:

- Psycopg 3: https://www.psycopg.org/
- PostgreSQL pgbench: https://www.postgresql.org/docs/current/pgbench.html
- PostgreSQL locking/deadlocks: https://www.postgresql.org/docs/current/explicit-locking.html
- PostgreSQL isolation/retry: https://www.postgresql.org/docs/current/transaction-iso.html
- PostgreSQL blocking inspection: https://www.postgresql.org/docs/current/functions-info.html
