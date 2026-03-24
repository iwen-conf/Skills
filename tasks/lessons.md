# Lessons

## 2026-03-15

- When the user says `arc` or names an `arc:*` capability in this environment, first map it to the local skill set under `/Users/iluwen/Documents/Code/Skills/Arc/` before concluding that a standalone CLI/runtime is required.
- When the user pushes back on Python-heavy orchestration for local long-running services, prefer a one-shot Go or shell controller that exits immediately after updating `tmux` and the project-local registry.
- When a short-lived Go controller will be invoked repeatedly, add a tiny cached launcher so later runs can reuse the compiled binary instead of paying `go run` startup cost every time.
- When migrating Arc engineering helpers off Python, prefer a thin `sh` wrapper that preserves the existing script path while moving parsing, state, and reporting logic into a cached Go binary.

## 2026-03-25

- For this Skills repository, treat "no GitHub Actions" as a hard repository policy, not a soft preference. If the user has already rejected Actions, add a validator/test guard so `.github/workflows/` cannot come back silently.
