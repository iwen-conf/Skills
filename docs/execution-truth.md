# Execution Truth Gates

Cross-skill anti-drift rules. Skills link here; do not paste long copies into every `SKILL.md`.

## 1. Environment / branch / deploy surface

1. **Runtime truth beats cwd.** Prefer production/staging deploy branch, compose labels, and user-stated environment over the current checkout branch or newest task doc.
2. **Name the surface.** When diagnosing or deploying, state which of: local workstation, `.26` test, `.31` sync, production. Do not mix evidence across surfaces.
3. **Dual-track repos.** If a project keeps parallel tracks (for example `pgx` vs `mongo`), follow the track that matches the named surface. Do not assume the branch under the working tree is production.
4. **Build artifacts must match the track.** Client packages, admin panels, and server images built from the wrong track are defects even if they compile.

## 2. Completion definition

A task, capability matrix row, or audit item is **done** only when all of the following hold:

1. **Code path exists** for the claimed behavior (not only a stub that returns a typed "not implemented" unless that is the intentional product state).
2. **Gates pass** that the project owns (tests, inventory, golden contracts, typecheck, or named manual check).
3. **Reachable behavior** matches the claim under the intended surface (API returns success path, UI shows the control, agent can complete the flow).

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

If scope is missing on risky work, stop and use `arc:clarify` rather than guessing.

## 4. Domain identity keys

1. **Do not invent routing keys.** Domain identity fields (user type, source type, pool id, shop id, principal roles) come from project contracts, enums, DB columns, or user-authored docs—not from model inference.
2. **Prefer the current contract.** When the project retired a key, do not reintroduce it as the primary route because older tests or docs still mention it.
3. **Project AGENTS / domain docs own product-specific tables.** Arc only forbids guessing; it does not redefine business enums for a specific product.

## 5. Architecture paper-overs

When `arc:arch` / `arc:fix` apply:

1. No temporary package-level `var` flags to bypass product rules without an explicit task and review.
2. No type assertions to grab optional infrastructure capabilities that should be injected contracts.
3. No silent hardcoding of epochs, auth schemes, or magic IDs "for now".
4. No domain/application workflow dumped into `domain/services` when it belongs in `usecase`.
5. No paper-over: changing tests only, swallowing errors, or documenting aspirational behavior as shipped.

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

## 7. Downstream task authoring (cheap executors)

When writing tasks for a lower-capability model:

1. Bound: in-scope files/symbols, out-of-scope list, execution order, acceptance checks, and forbidden shortcuts.
2. One subtask = one verifiable outcome; no "fix all security issues".
3. Point at current paths from latest repo state; re-check before mark complete.
4. Include "do not" for compatibility shims, wrong deploy surface, and invented domain keys.

## 8. Think before coding

Applies to every implementation or repair request before the first edit.

1. **State assumptions.** Name the assumptions the plan depends on. If multiple readings of the
   request exist, present them — do not silently pick one.
2. **Offer the simpler path.** If a materially simpler approach exists, say so before building the
   requested one. Push back when the request itself looks wrong.
3. **Ask/autonomy boundary.** Proceed end-to-end without asking for bug fixes and for work with one
   clearly correct answer. Stop and ask only when the tradeoff touches interfaces, persisted data,
   or architecture — or when a blocking ambiguity survives reading the available context.
4. **Verifiable goal first.** Convert the task into a checkable success criterion before editing
   ("fix bug" → "write the failing reproducer, then make it pass"; "refactor X" → "tests pass
   before and after"). A strong criterion lets the agent loop autonomously; "make it work" does not.

## 9. Simplicity ladder

Solve the problem with the least code. Run the ladder only **after** understanding the problem —
read the code the change touches and trace the real flow first; the ladder does not replace
understanding. Stop at the first rung that holds:

1. Does this need to exist at all? If not, skip it (YAGNI).
2. Does the codebase already have it? Reuse the existing helper/util/pattern — do not rewrite.
3. Can the standard library do it? Use it.
4. Can a platform-native feature cover it? Prefer it (`<input type="date">` over a date-picker
   library, CSS over JS, DB constraints over app-layer checks).
5. Can an already-installed dependency solve it? Use it. Never add a new dependency for something
   a few lines of code can do.
6. Can one line do it? Write one line.
7. Only then write the minimal new code.

Boundaries:

1. Never simplify away: input validation at trust boundaries, error handling that prevents data
   loss, security measures, accessibility basics, or anything the user explicitly asked for.
2. A deliberate simplification with a known ceiling (global lock, O(n²) scan, naive heuristic)
   gets a `debt:` comment naming the ceiling and the upgrade path
   (e.g. `# debt: 全局锁，吞吐量成瓶颈时改每账户锁`).
3. Required DIP boundary interfaces from `arc:arch` are not "unrequested abstraction"; keep the
   boundary and shrink everything around it.
4. Self-check: would a senior engineer call this over-engineered? If yes, simplify. If 200 lines
   could be 50, rewrite before delivering.

## 10. Surgical edits

1. Do not "improve" adjacent code, comments, or formatting outside the request. Match the existing
   style even when you prefer another.
2. Do not refactor what is not broken unless the refactor is the task.
3. Delete the imports/variables/functions that **your change** orphaned, in the same change.
4. Do not delete pre-existing dead code; mention it instead (cleanup becomes its own task,
   see `code-rot-taxonomy` item 14).
5. Traceability test: every changed line maps to the user's request or to an orphan the change
   created.
6. Bounded elegance pass: for a non-trivial change, before delivery ask "knowing everything I know
   now, is there a more elegant way?" If the fix feels hacky, rewrite — but only the code this
   change introduced, never expanding into pre-existing code. Skip the pass for simple, obvious
   fixes; do not over-design.

## 11. Self-improvement loop

1. After every user correction, record a rule that prevents the same mistake: repo-specific
   lessons go to that repository's `tasks/lessons.md`; cross-project preferences and working
   styles go to persistent memory.
2. At session start in a repository that has `tasks/lessons.md`, review the lessons relevant to
   the task.
3. Skills in this repository are the source of truth: when a lesson invalidates a skill rule,
   update the skill under `Arc/` (or the vertical directory) here first, not only the installed
   copy.
