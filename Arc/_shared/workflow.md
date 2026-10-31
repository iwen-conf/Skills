# Arc Workflow and Handoff

Read with [contract.md](contract.md) when entering an Arc phase. One phase has one owner;
the same agent may activate the next skill and continue within the user's existing authorization.
Load only that phase's skill and relevant references. Do not recursively load all six skills,
treat a skill switch as new permission, or stop at a handoff when the next action is already authorized.

## 1. Select the owner by the requested result

| Result or missing input | Owner | Receives → returns |
|---|---|---|
| Code location, call/data flow, impact, current architecture | `arc:explore` | question/seeds → queries, read source locations, unresolved paths |
| Reuse, adopt, borrow or build for a new dependency or generic component | `arc:vet` | need/constraints → dated candidate evidence, vetoes, decision record |
| PRD, scope, acceptance, business closure model, executable tasks or progress | `arc:plan` | intent/facts/findings → requirements, model + strict gate, bounded tasks, one tracker |
| Authorized implementation or confirmed root-cause repair | `arc:code` | contract/model + scope + evidence → implementation, impact map, entry-point evidence |
| Tests, runtime claims or release evidence | `arc:verify` | requirements/model + implementation → independent oracles, per-claim verdicts, repair packets |
| Read-only code, risk or security evaluation | `arc:audit` | scope + contracts/model + code/evidence → findings, coverage limits, next owner |

Ordinary comments, presentation edits and internal refactors do not acquire a model gate. Apply it
to explicitly requested closure work or changes to modeled business behavior; discover the existing
model before deciding it is absent.

Typical paths (skip phases whose current outputs already exist):

- New lifecycle: `plan (prd → model → tasks) → code → verify`.
- New dependency: `vet → code → verify`; the record's pin and proof test feed `arc:code`.
- Existing lifecycle change: `explore → plan (model change) → code → verify`.
- Business review: `plan (model review) → plan (tasks for confirmed findings)`.
- Regression: `verify → code (repair) → verify`; if the intended contract is unresolved,
  return only that decision to plan first.
- Test build-out for existing behavior: `verify (flow: inventory → charts → branch ledger) → verify (write)`.
- Test-first feature: `verify (red acceptance tests) → code (red → green) → verify`
  ([test-first](test-first.md) §5).

## 2. One handoff record, updated in place

Use the active task card's Handoff section; for untracked work use the phase report in the
conversation. Persist evidence at the contract's artifact path. Do not create a second tracker
or task tree just for skill transitions.

```text
Goal / active owner / next owner:
Scope and non-goals (copy user restrictions):
Authorization already given / decisions still missing:
Requirement and API contract paths + existing IDs:
Model path + spec/checker hashes + check/gate commands and results, or not applicable:
REQ → entity/transition/event/invariant/scenario → task → code → runtime claim:
Facts and source locations / assumptions / unresolved questions:
Context-Queries (exact arc-idx commands):
Evidence (command, result, candidate revision, environment, artifact):
Open findings (ID, severity, status, owner) / invalidated evidence:
Next action and readiness condition:
```

Reuse IDs from the owning documents and model; do not rename or invent domain values during
handoff. When the model format names entities/transitions rather than assigning IDs, use those
existing names. Missing mappings remain explicit gaps. A downstream phase re-runs the relevant
queries and freshness checks; it does not repeat completed discovery without evidence of change.

## 3. Artifact ownership

| Artifact | Authority |
|---|---|
| User requirements / PRD | Existing project requirement documents, normally `docs/00-产品需求`; plan owns revisions |
| API contract | Existing Markdown API documentation, normally `docs/02-API记录` |
| Business model | `docs/01-系统设计/业务闭环/03-closure-spec.yaml` (legacy `docs/arc/`) or the discovered path; plan model mode owns changes |
| Closure prose and manual gaps | Beside the model; cite requirements and spec, do not duplicate product or execution truth |
| Tasks and status | Existing task cards and one `进度跟踪表.md`, normally `docs/03-执行任务` |
| Runtime behavior | Source and executed entry-point checks; never inferred from a model report |
| Business flows and branch ledger | `docs/07-测试质量/业务流/`; verify owns; flows cite model IDs and never redefine states |
| Read-only model review | `.arc/artifacts/<task-slug>/closure/`; reconstructed models are observations, not replacements for the canonical spec |

The model is the executable source for modeled business rules, not a replacement for user intent,
API contracts, implementation facts or task progress. Conflicts are recorded with both sources.
Only the affected decision blocks; keep independent authorized work moving.

## 4. Closure gate and downstream evidence

1. For a modeled behavior change, plan model mode updates the spec and runs the installed checker:
   `python <plan-dir>/scripts/closure/check_closure.py check <spec>`, then `gate <spec>`.
   Resolve `<plan-dir>` from the active skill installation, never from an assumed host path.
2. Before implementing that behavior, code runs `gate` using the same checker and current spec.
   A missing, stale, failed or truncated check blocks dependent implementation. Return the exact
   command and errors to plan model mode. Do not bypass with `--allow-warn` or an old copied checker.
3. A successful gate means the checked finite model passed its implemented rules. It does not
   prove the requirements are correct, manual gaps are resolved, authorization exists, runtime
   behavior works or a release is qualified. Record these separately.
4. Carry unresolved assumptions, manual findings and waivers into the handoff. A waiver needs
   a concrete reason and the user's applicable approval; it cannot be manufactured to turn a
   failure green. Structural completeness and successful enumeration are not waivable.
5. Verify turns model requirements, invariants and scenarios into EXPECTED/FORBIDDEN runtime
   claims with independent oracles and real entry points, mapping each scenario to flow-chart
   branches in its flow catalog. A single-instance finite model does
   not prove concurrent, multi-instance, database, broker or recovery semantics.
6. Spec or checker changes invalidate the model gate. Changed requirements, source, tests,
   config or environment invalidate affected downstream mappings/evidence. Re-check affected
   claims and update task status before asserting completion.

## 5. Return findings to the owning phase

- Missing code fact → explore with exact question and queries already tried.
- Unclear requirement/acceptance → plan with the source conflict and decision needed.
- Missing lifecycle/recovery rule or model contradiction → plan model mode with the model element,
  counterexample and current check result.
- Confirmed implementation defect → code with the execution-truth repair packet and model IDs.
- Missing or invalid oracle/test/environment evidence → verify with the unproven claim.
- Re-evaluation of fixed risk → audit with original finding ID and new evidence.

Read-only requests remain read-only across every transition. Model review uses artifact copies
and the installed checker; it never initializes project docs, changes the canonical model or
injects project rules. Docs-only requests stop before code. For authorized end-to-end work,
activate the next available skill and continue; ask only for an actual unresolved decision or
action outside the existing authorization. If a required skill is unavailable, name it and the
blocked output; do not claim the handoff ran or silently use a retired alias.

## 6. Graph-based business-chain verification across Arc skills (opt-in)

This extends sections 1–5; it does not replace the canonical closure spec,
business-flow catalog, phase authorities or existing handoff task card.
Trigger only for an explicitly requested business-chain verification effort or
a project with a configured `verification.business_graph` / confirmed graph
version. Ordinary code changes do **not** require starting a new graph.

The local `arc-verify` Go CLI is an ephemeral process. Its persistent ledger is
the current *target repository*'s `.arc/verify/data.db` (local Turso file).
It must not contact Turso Cloud. Remote addresses in `config.jsonc` are
**authorization-scoped targets under test**, never the ledger database.
No remote host/service other than those explicitly permitted by the user's
environment constraints may be accessed as a workaround.

| Existing skill | Receives | Output / boundary |
|---|---|---|
| `arc:explore` | feature/action IDs and missing implementation facts | Read-only indexed entry / route / API / caller facts with `file:line`. Never write graph DB or assert a missing feature from one failed search. |
| `arc:plan` | user intent, PRD, closure-spec, unresolved RULE / RISK | Own canonical requirement and business rule wording in project docs. Get explicit user approval for each normative rule; risk is not a mandatory business contract. Docs-only authority means no DB updates in plan phase. |
| `arc:verify` | confirmed rule IDs, current implementation/flow facts and environment scope | Import approved graph, lazy task generation, real E2E/independent Oracle, per-step trace + RAW and gate. Owns the local graph ledger and test-flow catalog, never production repairs. |
| `arc:code` | confirmed failing run/step, rule, independent RAW, impacted route and tested commit | Root-cause repair on existing code; keep frozen rule, old FAIL and RAW unchanged. Hand to verify for new runtime proof. Fix-phase code/test receipt must not require a pre-fix red graph to go green. |
| `arc:audit` | rule/risk and implementation gap candidates | Read-only critical review; distinguish confirmed contract violation from `NOT_FOUND` candidate, absent evidence and industry hypotheses. |
| `arc:vet` | a concrete missing runner or tool dependency with cost/environment constraints | Date-stamped choice and version/rollback record; never install persistent infra or substitute mock proof. |

**Bridge command** (read-only; no extra tracker):

```bash
arc-verify handoff --to arc-explore --graph academic-v1
arc-verify handoff --to arc-plan --graph academic-v1
arc-verify handoff --to arc-verify --graph academic-v1
arc-verify handoff --to arc-code --run R-FAILED   # requires failed run
arc-verify handoff --to arc-audit --graph academic-v1
arc-verify handoff --to arc-vet --graph academic-v1
```

This bounded JSON packet is **context**, not an authority to change scope, access
an unauthorized server, edit canonical docs, alter the graph or approve rules.
Use the existing handoff record (§2) as the unique tracker. Preserve IDs
`REQ → RULE → graph version → edge/condition → task → run → step → RAW` and
source commit/environment/actor. Missing links remain explicit gaps, not
synthetic "PASS". Record a code or model change as making affected prior runs
stale; do not silently rewrite immutable history. A later PASS only proves a
new run of the newer version.

Business expectations derive from explicit user-approved rules. Other domain
guesses remain `RISK` and do not become `MISSING`/business FAIL. Actual UI/API
behavior is evidence of implementation, **not** of correctness. A failed or
not-yet-run graph blocks the graph-verification receipt; it must not prevent
`arc:code` from repairing the failure under its own test gates.

Before planning, present the expected feature/action catalog from user intent and
accepted requirements; confirm only unresolved decisions. Explore implementation
entrances independently, keeping possibly absent expected functionality visible.
Each confirmed operation requires satisfied, per-condition unsatisfied and forced
cases plus its known condition-recovery paths. Generate bounded cards from IDs;
load only the selected neighborhood.

Verdicts require complete observations: missing RAW, wrong setup or runner failure
is BLOCKED; a successful observer proving a RULE violation is FAIL. Preserve the
first deviation before diagnosis; resume from a verified safe actual state in a
new run. Fixture reconstruction and a later PASS never rewrite earlier FAIL.
Graph gate PASS currently covers planned positive/negative/forced obligations;
additional recovery, cross-role, concurrency and E2E claims need their own RAW.

A handoff/model operation does not prove runner authenticity or fixture restoration.
