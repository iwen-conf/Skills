# Execution Truth Gates

Cross-skill anti-drift rules. Skills link here; do not paste long copies into every `SKILL.md`.

## 1. Environment / branch / deploy surface

1. **Runtime truth beats cwd.** Prefer production/staging deploy branch, compose labels, and user-stated environment over the current checkout branch or newest task doc.
2. **Name the surface.** When diagnosing or deploying, state the actual project-defined workstation/test/sync/production surface and address; host IDs are project facts, not global defaults. Do not mix evidence across surfaces.
3. **Dual-track repos.** If a project keeps parallel tracks (for example `pgx` vs `mongo`), follow the track that matches the named surface. Do not assume the branch under the working tree is production.
4. **Build artifacts must match the track.** Client packages, admin panels, and server images built from the wrong track are defects even if they compile.

## 2. Completion definition

For implementation work, a task, capability matrix row, or audit item is **done** only when all of the following hold:

1. **Code path exists** for the claimed behavior (not only a stub that returns a typed "not implemented" unless that is the intentional product state).
2. **Gates pass** that the project owns (tests, inventory, golden contracts, typecheck, or named manual check).
3. **Reachable behavior** matches the claim under the intended surface (API returns success path, UI shows the control, agent can complete the flow).

For document-only or read-only work, the reachable artifact is the requested document/review: verify its content, provenance, references and applicable artifact gates. This never marks unimplemented application behavior or unrun external tests complete.

**Not sufficient alone:** documentation checkboxes, "ability matrix" cells, README claims, or "engineer said fixed" without re-check on current code and evidence.

When status is unknown, mark `[?]` or report residual risk—do not promote to `[x]`.

## 3. Scope lock

Honor explicit user scope tokens. If present, do not expand:

| Token (examples) | Allowed | Forbidden without re-confirm |
|---|---|---|
| 只读 / read-only | Inspect, report | Edit production code, deploy, mutate data |
| 只写文档 / docs-only | Task docs, reports | Application code, configs that change runtime |
| 只做前端 / frontend-only | Frontend stack in scope | Backend/API/schema unless user opens scope |
| 禁止重启 / no restart | Code + local verify | Restart remote services; hand restart to user |
| 明确不做 / out of scope / P4 / Hold | Record as non-goal | Implement "while we are here" |
| 先别改包名/地址 | Temporary test edits only if user allows, then restore | Permanent package/id/url changes |

If scope is missing on risky work, stop and write a brief (`arc:plan`) rather than guessing.

## 4. Domain identity keys

1. **Do not invent routing keys.** Domain identity fields (user type, source type, pool id, shop id, principal roles) come from project contracts, enums, DB columns, or user-authored docs—not from model inference.
2. **Prefer the current contract.** When the project retired a key, do not reintroduce it as the primary route because older tests or docs still mention it.
3. **Project AGENTS / domain docs own product-specific tables.** Arc only forbids guessing; it does not redefine business enums for a specific product.

## 5. Architecture paper-overs

Moved to [coding-discipline.md](../Arc/_shared/coding-discipline.md) §4 (Arc code edits and reviews).

## 6. Evidence-first root-cause repair

This gate applies whenever a request diagnoses a suspected bug, regression, incident,
security finding, performance problem, or asks for an optimization justified by a failure.
The signal is not the defect; it is only a reason to investigate.

1. **Confirm the claim first.** State the observed behavior, expected behavior, data, and
   environment/branch surface. Reproduce it or preserve concrete evidence (logs, test output,
   traces, screenshots, or a contract mismatch). If the issue cannot be confirmed, do not edit
   production behavior; report the evidence gap or continue with read-only investigation.
2. **Separate facts from hypotheses.** Record the exact failure and each hypothesis. Do not
   turn a plausible explanation, scanner warning, flaky test, or user suspicion into a confirmed
   defect without a check that could disprove it.
3. **Trace architecture before patching.** Map the owning boundary, control/data flow,
   state transitions, dependency direction, and shared invariants. Then compare that model with
   the current business contract, user-visible semantics, and domain rules. Architecture is the
   search order; the business contract determines what is correct.
4. **Name the root cause and affected siblings.** Distinguish the cause from its symptom,
   identify other call sites or flows governed by the same boundary, and state why the proposed
   change will not silently leave the same defect elsewhere.
5. **Plan the smallest complete fix.** Change the owning layer and shared contract when the
   evidence requires it. A small diff is not a valid fix if it leaves the cause, adds a retry or
   compatibility shim, weakens an assertion, or hides an error. Do not broaden the change for
   hypothetical problems that have no evidence.
6. **Implement only a confirmed repair.** If the issue or root cause remains unconfirmed,
   do not change production behavior or claim a fix. Bounded, behavior-preserving logs, traces,
   or reproducer coverage may be added only to collect the missing evidence; label that work as
   investigation, then report the remaining blocker instead of using a speculative workaround
   to make a check green.
7. **Verify the original path and sibling risk.** Re-run the reproducer or equivalent check,
   then test the affected invariant and the most relevant analogous paths. Preserve the
   verification evidence and state residual uncertainty explicitly.

### Required repair packet

`Observed → Expected → Surface → Evidence → Hypothesis → Root cause → Affected siblings →
Complete fix boundary → Verification → Residual risk`.

## 7. Downstream task authoring

Tasks written for another executor follow `Arc/arc:plan/references/task-docs.md`: bounded scope,
one verifiable outcome per subtask, current paths, and explicit "do not" items.

## 8. Think before coding

Moved to [coding-discipline.md](../Arc/_shared/coding-discipline.md) §1.

## 9. Simplicity ladder

Moved to [coding-discipline.md](../Arc/_shared/coding-discipline.md) §2.

## 10. Surgical edits

Moved to [coding-discipline.md](../Arc/_shared/coding-discipline.md) §3.

## 11. Self-improvement loop

1. After every user correction, record a rule that prevents the same mistake: repo-specific
   lessons go to that repository's `tasks/lessons.md`; cross-project preferences and working
   styles go to persistent memory.
2. At session start in a repository that has `tasks/lessons.md`, review the lessons relevant to
   the task.
3. Skills in this repository are the source of truth: when a lesson invalidates a skill rule,
   update the skill under `Arc/` (or the vertical directory) here first, not only the installed
   copy.
