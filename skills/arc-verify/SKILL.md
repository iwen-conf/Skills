---
name: arc-verify
version: 6.0.0
description: Independent runtime verification through real entry points: acceptance/E2E tests, API/SQL invariants, mobile device gates (`redroid`/`avdslim`), concurrency proofs, and `.arc/arc.db` receipts. Triggers on 写测试, 验证, 端到端测试, 业务流验证, 幂等并发, 发布验收.
---

# arc-verify

Produce revision-bound behavior evidence against frozen contracts. Never edit production source or weaken assertions to force a green run.

## Protocol

1. Prefer an agent family distinct from the `arc-code` author whenever multiple agents are active.
2. Exercise real HTTP/CLI/UI/DB entry points (for Android device verification, follow `../_shared/android.md` `redroid` / `avdslim` rules). Zero executed tests, skipped suites, or mocked-out target units count as `NOT_RUN`, never `PASS`.
3. Inspect obligations via `arc verify` / `arc task verify-status`. Preserve initial `FAIL` evidence (`file:line`, reproducer command, expected vs actual) immutably in `.arc/arc.db` and route scoped repair packets back to `arc-code`.
