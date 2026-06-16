# Code Rot Taxonomy — 36 AI Code Smells

Single source of truth for the 36 failure modes that LLM-written business code repeatedly falls into
(catalogued by @seclink). Each Arc skill references the slice it owns instead of re-listing the table —
keeping this catalog the only place the full set lives (which is itself rule #16 / #21 / #23 applied to our own docs).

**How to read each entry**: `ID. Symptom → Gate (the enforceable rule) → Smell (the rot signal to grep for)`.
Rules are written model-facing in English. IDs are stable and match the original author numbering 1–36.

---

## A. 约定漂移 — Convention Drift
*One business meaning must map to exactly one name, type, shape, and key style across the whole project.*

1. **Timezone ambiguity** → Store and compare time in UTC; convert to a local zone only at the presentation edge. Pin the zone explicitly, never inherit the model's or host's locale. → Smell: naive `datetime.now()` / `time.Now()` without zone; mixed Beijing and host time.
2. **Error code type drift** → Decide once whether an error code is `int` or `string` and enforce it on every boundary; never let the same field be `400` here and `"400"` there. → Smell: a code field typed differently across endpoints; string/int comparison casts.
8. **Same concept, different names** → One domain concept = one identifier everywhere (`phone` vs `mobile`, `userId` vs `uid`). Establish the ubiquitous term at definition time and reuse it. → Smell: synonyms for one field across functions/DTOs.
16. **Scattered duplicated constants** → Define a constant with one business meaning in one place; import it. No copy-pasted literal magic values. → Smell: the same magic number/string literal in many files, inconsistently.
18. **Unstable input/output param names** → Request and response field names for one concept are fixed and identical across all APIs. → Smell: `mobile` in the request but `phone` in the response.
19. **Inconsistent JSON nesting** → One canonical envelope depth and shape for all responses; do not return two-level here and three-level there. → Smell: response wrappers varying in nesting per endpoint.
22. **Redis key style chaos** → Adopt one key convention (`namespace:entity:id`) and one TTL policy; document it; apply uniformly. → Smell: ad-hoc, colon/underscore-mixed keys, no TTL discipline.
24. **Pagination contract drift** → Pick one pagination contract (`page+page_size` OR `offset+limit`) project-wide; do not mix. (Also a Data-Layer concern, see D.) → Smell: both conventions in different list endpoints.
33. **Unplanned structured logging** → Define a logging spec (levels, fields, where each event goes, e.g. Aliyun SLS) before writing logs; do not log wherever convenient. → Smell: free-form log lines, no level discipline, no field schema.

---

## B. 冗余死码 — Redundancy & Dead Code
*Reuse before you write; cut before you ship. Every line must be reachable and necessary.*

9. **Unused functions written** → Do not write a function until a caller needs it; delete functions with no callers. → Smell: defined-but-never-called functions.
14. **Dead variables / functions / classes / deps** → Remove unreferenced code and uninstall unused third-party packages in the same change that orphaned them. → Smell: unreferenced symbols; imported-but-unused dependencies in the manifest.
15. **3–4 competing logging systems** → Exactly one logging library/facade per project. Migrate, do not stack. → Smell: multiple logger imports across the codebase.
17. **Stale / dangerous test code** → Tests must be live and isolated; never leave tests that wipe or restore a shared DB as a side effect. → Smell: tests with `DROP`/`TRUNCATE`/`restore db` against shared state; skipped-forever tests.
20. **Obsolete, undocumented third-party libs** → Prefer maintained, documented dependencies; do not adopt abandoned libraries. Justify each new dependency. → Smell: deps with no recent releases or docs.
21. **Duplicate near-identical modules/endpoints** → Before adding an endpoint or module, search for an existing one that already does it; extend instead of cloning. → Smell: two endpoints with the same effective behavior.
23. **Re-implemented utility/format helpers** → One shared utilities home for formatters (string/date/number); import, never re-implement per file. → Smell: copies of the same `formatX` helper in many files.
32. **Over-design / no pruning** → Build exactly the API surface the requirement needs. If a PM needs 1 API, ship 1 — do not invent 3–4 speculative scenario variants. Prune aggressively. → Smell: speculative endpoints/abstractions with no current consumer.

---

## C. 安全 — Security
*Authorization and unpredictability are not optional; the happy path is not the spec.*

6. **No anti-brute-force on codes/verification** → Rate-limit and lock out SMS codes, real-name checks, and OTP attempts; expire codes; cap retries per identity/IP. → Smell: verification endpoints with no attempt cap or expiry.
12. **Horizontal / vertical privilege escalation** → Every data access checks both authentication and ownership/role: the caller may only act on resources they own and operations their role permits. Enforce in the query (`WHERE owner_id = :caller`) and at the handler. → Smell: resource fetched by id alone with no owner/role guard.
28. **Predictable randomness** → Use a cryptographically secure RNG for tokens, codes, and secrets; never `rand()`/`Math.random()` for security values. → Smell: non-CSPRNG seeding security-sensitive values.
29. **Hardcoded backdoor** → No hardcoded credentials, master passwords, or bypass branches. Secrets come from config/secret store. → Smell: literal credentials or `if user == "admin_backdoor"` branches.
30. **Brute-forceable interfaces** → Sensitive endpoints have rate limiting, lockout, and monitoring; enumerable identifiers are unguessable or access-controlled. → Smell: login/lookup endpoints with no throttle.
31. **Zero-amount purchase (0元购)** → Server recomputes price/amount authoritatively before payment; never trust client-supplied price/quantity. Validate amount > expected, check coupon/stock atomically. → Smell: order total taken from request body; price trusted from client.

---

## D. 数据层 — Data Layer
*The database is the source of truth; queries must be bounded, indexed, and consistent.*

3. **Soft-delete inconsistency** → Decide hard vs soft delete per table and document it. If soft (`deleted_at`/`is_deleted`), every read, update, count, and join applies the same filter. → Smell: some queries ignore the soft-delete flag.
5. **N+1 / slow queries unconsidered** → Batch or join related reads; set slow-query logging; add indexes for hot predicates before shipping list/detail paths. → Smell: per-row queries inside a loop; no slow-query threshold set.
10. **Raw SQL causing PK/auto-increment issues** → If using an ORM, use it consistently; if raw SQL, handle primary-key generation and `RETURNING` deliberately. Do not half-bypass the ORM. → Smell: manual id handling that fights the ORM's PK strategy.
11. **JSONB overuse blocking query/search** → Use JSONB only for genuinely schemaless data; promote queried/filtered fields to real columns with indexes. → Smell: filtering/searching inside a JSONB blob on a hot path.
24. **Pagination contract drift** → (Shared with A.) One pagination contract project-wide; every list query is paginated and bounded. → Smell: mixed `page/offset` styles; list endpoints with no bound.
25. **Mixed ORM/raw SQL; unbounded lists** → Consistent data-access strategy; every list query has a `LIMIT` (or paginates). → Smell: `SELECT ... ` lists with no `LIMIT`.
26. **`LIKE %keyword%` without index** → Avoid leading-wildcard `LIKE` on hot paths; use a proper index, full-text search, or trigram index. → Smell: `LIKE '%x%'` on large unindexed columns.

---

## E. 错误与状态 — Error & State
*State sets are contracts; errors must surface, and concurrency must be guarded.*

4. **Status codes scattered everywhere** → Centralize HTTP/business status codes in one place (enum/constants) and reference them; do not sprinkle raw `500`/`400` literals across handlers. → Smell: raw numeric status literals throughout the codebase.
7. **Unstable state machine** → Define the full state set and legal transitions once, up front; do not let states drift from 8 → 10 → 7 across days. Encode transitions explicitly. → Smell: states added/removed ad hoc; transitions implied, not declared.
8. **Empty state mislabeled as error** → Treat no-data as a successful business/UI state for list, search, dashboard, and first-use flows; reserve errors for failed requests, invalid responses, denied permissions, or intentional single-resource not-found cases. → Smell: zero rows trigger exceptions, rejected promises, failed fetch state, destructive alerts, or full-page error UI.
13. **Swallowed exceptions** → Never `catch` and discard. Handle, wrap with context, or rethrow; log with cause. An empty catch is forbidden. → Smell: `catch {}` / `except: pass` with no handling.
27. **Race conditions under concurrency** → Guard concurrent writes with transactions, optimistic locking, or state-encoded `WHERE` clauses (`... WHERE status='pending'`) and check affected rows. → Smell: read-modify-write without locking; unguarded counters/balances.

---

## F. AI 执行完整性 — AI Execution Integrity
*The agent's own discipline. These are about honesty and reversibility, not code shape — and are backed by scripts.*

34. **Self-broke the build, then burned tokens recovering** → Keep the project runnable at every step. Verify after each meaningful change; if a change breaks the build, fix forward or revert deliberately — do not blindly re-pull and thrash. → Gate script: `Arc/scripts/verify-project.sh`. → Smell: long edit streaks with no verification; recovery by re-cloning.
35. **Half-finished refactor reported as "done"** → "Done" requires: no placeholders, no half-migrated call sites, and verification green. Report residual work honestly; never claim completion you have not verified. → Gate scripts: `Arc/scripts/check-placeholders.sh` + `Arc/scripts/check-completion.sh`. → Smell: `TODO`/`FIXME`/`not implemented` markers in the diff; mixed old+new patterns left behind.
36. **Bulk-replace script that broke everything** → Do not run unreviewed project-wide `sed -i` / `find -exec sed` / `perl -i` rewrites. Ensure a clean commit/stash exists for rollback, then edit directory-by-directory and verify between steps. → Gate script: `Arc/scripts/check-destructive.sh`. → Smell: in-place bulk regex rewrites across the tree with no checkpoint.

---

## Skill Ownership

| Skill | Stage | Owns families | Primary item IDs |
|---|---|---|---|
| `arc:define` | project genesis | A (naming) | 8, 18 |
| `arc:clarify` | requirement | B (prune), E (state contract) | 32, 7 |
| `arc:build` | implementation | A, B, C, E (prevent) + F | 8,16,18,19,22,24 · 9,14,21,23,32 · 6,12,28,29,30,31 · 4,7,13 · 34,35,36 |
| `arc:fix` | failure repair | D, E (root cause) + F | 3,5,10,11,24,25,26 · 4,7,13,27 · 34,35,36 |
| `arc:audit` | read-only review | all 36 | full catalog as review rubric |

`arc:build` also relies on its existing `## SQL Standards` section (covers most of D at write time);
`arc:fix` relies on its existing `## SQL Failure Checks` section.

## Family F → Script Map

| Item | Script | What it enforces |
|---|---|---|
| 34 | `verify-project.sh` | project still runs/passes after changes |
| 35 | `check-placeholders.sh`, `check-completion.sh` | no placeholders / half-done; "done" gate |
| 36 | `check-destructive.sh` | blocks unreviewed bulk-rewrite commands |
