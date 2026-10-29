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
