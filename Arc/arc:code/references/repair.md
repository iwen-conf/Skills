# Repair Procedure

Repair mode runs this procedure. The gate it implements and the required repair packet are
defined in [execution-truth.md](../../../docs/execution-truth.md) §6; this file adds the executable
steps. Every step ends with the named artifact. No production edit happens before steps 1–5
have produced theirs.

## 1. Capture

- Write three lines: **Observed**, **Expected**, **Surface** (environment, branch, deploy track
  per execution-truth §1).
- Re-check every claimed earlier fix on the current code before relying on it.
- Before substantial edits, save sanitized evidence under `.arc/artifacts/<task-slug>/logs/`:
  command stdout/stderr, server logs, traces, screenshots; for UI failures also the browser
  console, failed network requests and runtime errors. Redact per contract §5.
- IF runtime evidence can be captured THEN capture it; reading source alone is not a diagnosis.
- IF the target environment, data or access is unavailable THEN record `BLOCKED(<missing item>)`
  and do not change production behavior.

Artifact: the three lines plus the saved log paths.

When the failure is a regression from a known-good version or visual reference, or depends on
installed command behavior, load [regression-evidence.md](regression-evidence.md). It adds
history, oracle and runtime-boundary probes without replacing the repair gate below.

## 2. Reproduce as a failing check

Pick the first option that works:

1. An automated test at the owning boundary that enters through the real entry (handler,
   command, consumer) and asserts the expected behavior.
2. A script or command (`curl`, CLI call) with the exact input and the observed output.
3. Manual steps with the exact observed output; state why options 1–2 are impossible.

Run it on the unchanged code. It must fail for the reported reason: the assertion or error text
matches the observed failure, not a setup, fixture or environment error. Save the command and
output as the **before** record; for options 1–2 run it as `gate.sh red -- <command>`, which
refuses if production files already changed or the command passes.

- IF it passes on unchanged code THEN the failure is not reproduced → §6.
- IF repeated identical runs disagree THEN → §7.

Artifact: before record (command, output, exit code).

## 3. Locate the owner

- Query exact strings with code-intel: error text, request IDs, log keys, route paths; query
  symbols for stack frames.
- Trace entry → owning boundary → control and data flow → state transitions → shared invariants.
- Compare that model with the business contract (requirement, Markdown API contract, domain
  docs). Architecture sets the search order; the business contract decides correct behavior.
- IF the expected behavior is undefined in every source THEN stop and hand the decision to
  `arc:plan` (brief); do not choose a contract.

Artifact: call chain with `file:line` from entry to owner.

## 4. Hypotheses

Write ≥2 hypotheses before testing any. Status uses the contract finding vocabulary.

| # | Hypothesis | Check that could disprove it | Status |
|---|---|---|---|
| H1 | … | command or query | `needs-triage` → `false-positive` / `likely` / `confirmed` |

- Run the cheapest disproving check first.
- A hypothesis is `confirmed` only when its disproving check ran and failed to disprove it,
  and every competitor is `false-positive` or explains less of the evidence.
- A scanner warning, a plausible story or the user's suspicion stays `needs-triage` until a check
  runs.

Artifact: the filled table.

## 5. Root cause and affected siblings

- State the root cause: `<cause> at <file:line> in <layer>`; symptom `<symptom> at <file:line>`.
- Derive the defect pattern (faulty call shape, missing guard, wrong key, wrong default) and
  search it: code-intel §4 rows 1–3 for the owning symbol plus one AST or exact-string query for
  the pattern.
- Mark each hit `fix in this change`, `not affected: <reason>` or `outside scope: reported`.
- State why the fix will not leave the same defect on another path.

Artifact: root-cause line and the sibling list with queries (code-intel §7 format).

## 6. Bounded investigation probes

Use when step 2 cannot reproduce or step 4 cannot confirm.

- Allowed: behavior-preserving logs, traces, timing probes ([perf-probes.md](perf-probes.md)),
  extra assertions in tests, reproducer tests. Label this work `investigation` in the report.
- Place probes at the edges of the suspect segment: inputs, outputs, state before and after,
  error-wrap points, request-ID correlation.
- Forbidden: production behavior changes, speculative fixes, secrets, tokens or full payloads in
  output.
- Budget: 2 probe rounds. Then stop and report `hypothesis only` with the exact data,
  environment or access needed.
- Before completion remove the probes, or keep them behind a log level when the task asks.

## 7. Flaky checks

Signal: the same check on the same code and input gives different results.

- Run ≤5 identical runs; save pass/fail counts and the differing output; verdict `FLAKY`.
- Never add retries, sleeps or longer timeouts to make it pass.
- IF the evidence points to product code (race, ordering, shared mutable state, missing
  transaction or state guard) THEN treat it as a defect: build a deterministic reproducer
  (`go test -race -count=<n>`, forced interleaving, fixed clock) and continue at step 3.
- IF the cause is the test, fixture or environment (shared database state, wall clock, test
  order, network) THEN hand off to `arc:verify` with the run log; do not quarantine silently.

## 8. Fix

- Change the owning layer, and the shared contract when the evidence requires it; fix every
  sibling marked `fix in this change` in the same diff.
- Data-layer failures: check affected rows, transaction boundaries, state guards in `WHERE`,
  soft-delete filters, `LIMIT` or pagination, N+1 loops and indexes (code-rot families D and E,
  [code-rot.md](../../_shared/code-rot.md)).
- UI failures: fix at the owning UI, data or state boundary and check analogous routes and
  components; a CSS or state patch that hides a contract cause is not a fix.
- The SKILL.md Forbidden list applies.

## 9. Fail → pass evidence

- Re-run the step 2 check unchanged: **before** `FAIL` and **after** `PASS`, same command.
- Run the sibling regressions and the tests of the affected invariant.
- Run `gate.sh check --require-red`.
- Risky change (data write, auth, money, concurrency, migration): write a rollback note (how to
  revert code and data) and a monitoring note (which log line or metric shows recurrence).
- Transaction, duplicate-effect, retry or recovery incidents: hand off to `arc:verify` for
  real-boundary business evidence and prevention, detection and recovery controls.

## 10. Report

- Use the execution-truth §6 repair packet. **Evidence** links the saved logs;
  **Verification** holds the before/after records and sibling results.
- Separate confirmed results, residual uncertainty and `NOT_RUN` items; a patch does not by itself
  show the failing path is repaired.
- When the contract Lark hook applies, hand `lark` (project-docs) the incident severity
  (contract vocabulary), cause, changed feature, verification, follow-ups and the affected task
  row.
