# ARC Verify · Business-chain graph runtime implementation work

This is a staged implementation record. Do not mark a real-browser/real-Turso test PASS
when only Go unit tests or model validation ran.

## Completed in this branch (review before merge)

- C01: Reuse existing `arc:verify` and `arc-policy` rather than introducing a seventh Arc skill.
- C02: Add graph nodes/edges, rule/risk separation and model validation.
- C03: Add short-lived Go CLI and project-local Turso database schema via official `tursodb`.
- C04: Require manual business catalog and per-rule confirmation before planning mandatory tasks.
- C05: Generate positive, per-condition negative and forced-operation task references lazily.
- C06: Record run/step with independent expected-vs-observed comparison and SHA-256 RAW reference.
- C07: Fail closed in `arc-policy task verify` when project graph verification is configured.
- C08: Add model and policy regression tests, sample academic contract and CLI documentation.
- C09: Add fixture reference import/query and implementation observation graph gap inventory (absence remains unconfirmed).
- C10: Enrich task cards with actor, entry, preconditions and independent oracle; confirm the structural rule-edge mapping.
- C11: Add per-step RAW, first divergence, verified before-action conditions, halted/blocked/replan outcomes.
- C12: Add bounded recovery graph traversal with cycle detection; unconfirmed recovery actions remain unresolved.
- C13: Add abort/retry with reason and immutable historic runs.
- C14: Revalidate gitignored local DB and evidence at arc-policy receipt check; add Turso integration test and CI job (execution pending).

## Next verified work packages (not done)

- T01 **Native Turso integration gate**: install official `tursodb` in CI, exercise real
  SQL transactions, serialization, snapshots and concurrent claims; test failure exit codes.
  Do not claim DB integration on Go-only unit tests.
- T02 **Implementation observation/absence (partial)**: evidence-backed action observations and gap listing exist; expand to real UI/API crawler, versioned implementation graph, false-negative review and explicit MISSING confirmation. `NOT_FOUND` stays a candidate gap.
- T03 **Business condition graph traversal**: recovery tasks for OR/AND prerequisites,
  cross-role sequences, cycles and bounded interleavings; deduplicate equivalent states.
- T04 **Runner adapter & oracle**: real browser/mini-app/device adapter, stable step IDs,
  response+page+readback oracles, external-effect observation, trusted RAW provenance.
  Real E2E coverage remains NOT_RUN until an authorized target and credentials exist.
- T05 **Fixture registry & replay (partial)**: alias/secret-reference import exists; add
  per-task fixture locks, state verification before reuse, deterministic reconstruction
  and safe isolation. Never treat a logged checkpoint as automatic rollback of external actions.
- T06 **Policy hardening**: enforce graph freshness at commit/push time independently of
  local receipts; protect source/graph/evidence identities through CI; simulate agent
  bypass attempts. Same-user hooks never become a security boundary by themselves.
- T07 **Reports & graph diff**: inspectable coverage ledger per confirmed rule and edge,
  branches not reached, first divergence and downstream impact with historic runs.

Each task must deliver changed files, real test command and exit, expected+RAW evidence,
an exact verdict (PASS/FAIL/BLOCKED/NOT_RUN), and an explicit list of unverified claims.

## Review/acceptance matrix (do not claim all complete)

| Requirement | Source of truth | Status | Required real evidence |
|---|---|---|---|
| User-reviewed feature catalog | graph approval + scoped RULE | Implemented, trust limited to local terminal | human-reviewed contract and user decision |
| Risk never becomes RULE automatically | validation + approval | Implemented | negative tests for derived-rule rejection |
| Expected vs actual feature inventory | scan/gaps | Partial | actual browser/API traversal and source evidence |
| Directed graph + AND prerequisite recovery | nodes/edges/recover | Partial | recovery chains including OR/cycles and other actors |
| Direct executable task cards | next (actor/entry/conditions/oracle) | Partial | real runner action binding |
| Per-step observed precondition | step RAW | Implemented core | actual independently observed conditions |
| First deviation and safe continuation | step/trace | Implemented core | fault E2E and readback |
| Stable retry and historical attempts | abort/retry | Implemented core | process-crash test and data restoration |
| Accounts/fixtures reuse | fixtures + JSONC refs | Partial | true identity bindings and live state reconstruction |
| Local-only Turso | tursodb path | Implemented adapter, integration NOT_RUN | official shell, persistence and concurrency |
| Independent, real E2E | browser/device adapter | NOT_RUN | user-like interaction on authorized test server |
| Strong trust against same-user tampering | CI/principal separation | NOT_DONE | protected CI; separate runner authority |

The terminal approval prompt can be spoofed by an agent able to drive a PTY.
It is a workflow guard, not cryptographic user authorization.
The RAW runner identity is currently a declarative string; do not call it a signature.
The `tursodb` integration job must actually report PASS before this is considered
production-qualified. No automatic fixture restoration, real browser driving or
complete graph-path enumeration is claimed yet.

## Cross-skill integration (2026-10-29)

- Connected the six existing Arc SKILL.md files to the existing shared
  handoff §6; did not install a seventh Arc entry.
- Added `arc-verify handoff --to arc-* --graph/--run` with bounded, read-only
  context packets. `arc:code` requires an actual FAIL run to consume a repair
  packet; `arc:plan` and `arc:audit` cannot change graph state.
- Made `arc-policy` business-graph gate phase-aware: `arc:verify` must
  prove the graph, while `arc:code` can repair a red graph under its
  ordinary source/test checks. Explicit `require_for_skills` adds phases.
- Added static cross-skill handoff/registry tests and phase-aware policy tests.
- Synchronized the registry and enforced SKILL heading/token budgets.
- Still NOT_RUN: full skill distribution validation on a cloned repository,
  official Turso integration CI, real E2E and detached evidence principal.
  `REQ → RULE → FLOW` remains a documented mapping rather than a
  fully enforced foreign-key relation; do not claim otherwise.
