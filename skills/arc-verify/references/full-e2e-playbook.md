# Veteran-QA Full-Business E2E Playbook (`老测试全量业务审计手册`)

How to test an entire project with the completeness, skepticism, and traceability of a senior QA lead—never stopping at happy-path smoke tests or implementation-mirroring unit stubs.

## 1. Phase 1: Project-Wide Business Inventory (`全盘摸底`)

Before writing any test case, build a complete surface map so zero modules or roles are silently skipped:

1. **Automated Surface Scan**: Run `arc verify scan-surface --repo . --format ai-md` and `arc idx stats` to enumerate:
   - **External Entry Points**: HTTP/REST handlers, gRPC/Protobuf methods, WebSocket/SSE streams, CLI subcommands, frontend pages/routes, mobile deep links.
   - **Background Entry Points**: Cron/scheduled jobs, MQ consumers, webhook callbacks, outbox relays, reconciliation workers.
   - **Core Stateful Entities**: SQL tables, ORM models, Redis locks/quotas, object-storage buckets.
2. **Role x Permission x Tenant Matrix**:
   - Extract real roles/principals from auth middleware and schema enums (e.g., anonymous, normal user, frozen/disabled user, tenant A vs tenant B, operator/admin, external webhook caller).
   - Every mutating business flow must test **cross-tenant / horizontal IDOR rejection** and **disabled/unprivileged role rejection** alongside the authorized actor.
3. **Coupling & Closure Audit**:
   - Run `arc verify scan-world <entity.pk>` on every stateful core table so coupled foreign-key tables, balances, quotas, and topics are 100% bound into `world_flags`.
   - When a state-machine spec exists, run `python3 ../arc-plan/scripts/check_closure.py check <spec>` to expose dead-end states, missing timeout/exception exits, unhandled duplicate/late events, and race points before runtime E2E begins.

## 2. Phase 2: Designing Cases Across the 5 QA Behavior Tracks

For every business capability, design routes covering all applicable tracks rather than testing only success:

| Track (`expected_behavior`) | What a Veteran QA Probes | Mandatory Assertions |
|---|---|---|
| `ALLOW_TRANSITION` | End-to-end multi-role main chain (create -> pay -> callback -> notify -> query/refund) | Exact DB state transition, exact minor-unit amounts, emitted events, UI/API reload consistency |
| `REJECT_INVALID_INPUT` | Equivalence classes & boundary values: `0`, `-1`, `max`, `max+1`, empty/`null`/omitted JSON fields, precision overflow, wrong timezone, oversized/spoofed MIME upload | Deterministic 4xx/error code, **zero DB row mutation**, **zero outbox/MQ side effect** |
| `HARD_BLOCK` | Unauthenticated/wrong-tenant actor, insufficient balance/stock, terminal-state re-entry, expired token, disabled account | Explicit rejection reason; `before_state == after_state` across focal + coupled tables |
| `ASYNC_HOLD_ESCAPE` | In-transit payment, cooling-off window, third-party timeout, delayed webhook arrival | Intermediate state enters bounded hold (`COOLING`/`PENDING`) AND deterministically settles or reverts via `anti_softlock` without trapping user funds/account |
| `IDEMPOTENT_REPLAY` / `CONCURRENT_CONTENTION` / `FORCE_CASCADE` | Duplicate callback/submit, parallel double-spend, admin compliance purge | Single logical effect, conserved ledger sum, anonymized counterparty snapshots without broken non-null FKs |

## 3. Phase 3: Dual Independent Oracles & UI/API Discipline

1. **Never Trust Single-Layer Echoes**:
   - A UI toast ("Saved!") or HTTP `200 OK` response body is an *observation*, not an independent oracle.
   - Always pair the action response with an **independent readback** (direct read-only SQL query against `$ARC_CLONE_PATH` / `$SHARD_NS` DB, independent lookup API under a verification observer, or persisted outbox/audit table check), then **reload/reopen** UI screens to prove durability rather than transient client memory.
2. **Positive Readability Before Negative Assertions**:
   - Before asserting an error banner is absent or an admin button is hidden for a guest, first assert the page/container and authenticated identity rendered cleanly—otherwise a blank crash page or 404 falsely passes negative assertions.
3. **Dynamic Context Continuity Across Long Journeys**:
   - Pass generated IDs (`order_id`, `refund_id`, `session_token`) across steps via `[ARC_STEP_EVIDENCE]` `extracted_vars` (persisted by `arc verify play` into `$ARC_VARS_FILE` and `ARC_VARS_JSON`), never hardcoded magic IDs that collide under `--parallel`.
   - Redact raw passwords, tokens, and keys (`<REDACTED>`) in `actual_input` logs while keeping exact structural keys and digests.
4. **Bounded Async Waiting (Zero `sleep`)**:
   - Poll the independent DB/API oracle with an explicit deadline and poll interval derived from the business SLA; fail immediately if an illegal intermediate state appears.

## 4. Phase 4: Audit Ledger & Scoped Repair Handoff

When `arc verify play` or `arc verify explain` surfaces deviations, classify strictly before reporting:

- **`BLOCKED` (Environment / Precondition Gap)**: Missing target server, unavailable device (`redroid`/`avdslim`), broken seed fixture, or failed `precheck_cmd`. Record exact missing prerequisite; never fake a mock `PASS`.
- **`SPECIFICATION GAP` (Ambiguous Contract)**: Docs/OpenAPI and schema disagree or omit edge-case rules (e.g., whether a disabled merchant still processes historical refunds). Surface both citations to the user/`arc-plan` instead of guessing.
- **`FAIL` (Confirmed Product Defect)**: Retain the initial `FAIL` receipt in `.arc/arc.db` (`historical_fail_count` must remain `>= 1` even after later fix) and deliver a structured repair packet to `arc-code`:
  - **Route & Step**: `<story_id>/<route_id>#step-<N>` (`choice`, `expected_behavior`)
  - **Reproducer**: Exact one-liner (`SHARD_NS=... arc verify play <story.yaml> --route <route_id>`)
  - **6-Dimension Diff**: Preconditions verified -> Actual Input sent -> `BeforeState -> AfterState` -> `Expected` vs `Actual` status/result/probes
  - **Suspected Fault Surface**: Exact `file:line` traced after blind execution completes.
