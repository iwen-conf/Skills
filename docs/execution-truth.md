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

## 6. Downstream task authoring (cheap executors)

When writing tasks for a lower-capability model:

1. Bound: in-scope files/symbols, out-of-scope list, execution order, acceptance checks, and forbidden shortcuts.
2. One subtask = one verifiable outcome; no "fix all security issues".
3. Point at current paths from latest repo state; re-check before mark complete.
4. Include "do not" for compatibility shims, wrong deploy surface, and invented domain keys.
