# Code Rot Taxonomy (41 items)

The single catalog of recurring failure modes in agent-written business code. Read with
[contract.md](contract.md) (vocabulary, artifacts) and [code-intel.md](code-intel.md) (query
syntax). Diagnosing a suspected defect found here follows the root-cause gate in
[execution-truth.md](../../docs/execution-truth.md) §6.

- IDs 1–41 are stable. Cite as family + ID (`D25`, `E27`); IDs 8 and 24 appear in two families.
- **Gate** is the write-time rule. **Detect** is the starting query for an audit; every hit is a
  hypothesis until the file is read and the rule violation is shown at `file:line`.
- An item that does not apply to the stack in scope is `N/A`, not a finding.

## Owners

| Skill | Role | Items |
|---|---|---|
| `arc:code` | Prevents at write time: applies every Gate to the lines it writes. Families **D and E are its data-layer write gate** for any SQL, migration, repository method or state transition. Runs `arc:code/scripts/gate.sh check` for F. | all |
| `arc:audit` | Detects: full catalog in health mode; changed lines only in review mode; family C in appsec mode; family G in health and review. | all |
| `arc:verify` | Proves: tests or checks that fail on the rot and pass after the fix (C authz/brute-force/amount, D bounds and filters, E transitions and races, F34). | C, D, E, F34 |

## A. Convention drift: one meaning → one name, type, shape and key style

| # | Rot | Gate | Detect |
|---|---|---|---|
| 1 | Timezone ambiguity | Store and compare in UTC; convert only at the presentation edge; pin the zone explicitly. | `search 'case:yes time.Now()'`, `'datetime.now('`, `'new Date('`; read for missing `.UTC()` / zone. |
| 2 | Error-code type drift | One type (`int` or `string`) for an error code on every boundary. | `search 'json:"code'`; compare field types across DTOs. |
| 8 | Same concept, different names | One identifier per domain concept (`phone` vs `mobile`, `userId` vs `uid`). | `search 'json:"(phone\|mobile)'` and similar synonym pairs. |
| 16 | Scattered duplicated constants | One definition per business constant; import it. | Same literal in > 2 files via `search 'case:yes <literal>'`. |
| 18 | Unstable request/response names | Field names for one concept identical across all APIs. | Compare request vs response json tags for the same concept. |
| 19 | Inconsistent JSON nesting | One response envelope depth and shape. | List response helpers / `c.JSON(` calls; count envelope shapes. |
| 22 | Redis key chaos | One key convention (`namespace:entity:id`) and one TTL policy. | `search 'Set(ctx\|HSet\|Expire'`; read key format strings and TTL arguments. |
| 24 | Pagination contract drift | One contract project-wide (`page+page_size` or `offset+limit`). | `search 'json:"(page_size\|pageSize\|offset\|limit)'`. |
| 33 | Unplanned logging | A logging spec (levels, fields, sink) exists before logs are written. | `search 'fmt.Println\|console.log\|log.Printf'` outside tests. |

## B. Redundancy and dead code: reuse before writing; every line reachable

| # | Rot | Gate | Detect |
|---|---|---|---|
| 9 | Unused functions written | No function without a current caller. | code-intel §4 row 2 = 0 for a new symbol. |
| 14 | Dead variables/functions/classes/deps | Remove what the change orphaned in the same change; pre-existing dead code becomes a separate cleanup task. | Row 2 = 0 for existing symbols; manifest deps with no import (`search 'case:yes "<module>'`). |
| 15 | Competing logging systems | Exactly one logging facade per project. | `search 'zap"\|logrus\|zerolog\|"log/slog"\|winston\|pino'`; > 1 family = finding. |
| 17 | Stale or dangerous tests | No test that drops/truncates/restores shared state; no permanently skipped test. | `search 'f:_test\.go$ (DROP\|TRUNCATE)'`; `t.Skip(`, `it.skip`, `xit(`. |
| 20 | Obsolete, undocumented libraries | Maintained, documented dependencies only; each new one justified. | Manifest additions; last release date of each (manual). |
| 21 | Duplicate near-identical modules/endpoints | Search for an existing one first; extend instead of cloning. | Route inventory: two routes with the same handler behavior. |
| 23 | Re-implemented helpers | One utilities home for formatters; import. | `symbol format --kind func` or `search 'func format'` with > 1 definition of one behavior. |
| 32 | Over-design | Ship exactly the surface the requirement needs. | Exported symbols / routes with row 2 or row 5 = 0 consumers. |

## C. Security: authorization and unpredictability are mandatory

| # | Rot | Gate | Detect |
|---|---|---|---|
| 6 | No anti-brute-force on codes | Rate limit, lockout, expiry and retry cap per identity and IP for SMS/OTP/real-name checks. | `search 'sms\|otp\|verify_code\|captcha'`; read handler for limiter and expiry. |
| 12 | Horizontal/vertical privilege escalation | Every data access checks authentication and ownership/role, in the query (`WHERE owner_id = :caller`) and at the handler. | Fetch-by-id calls (`ast '$R.FindByID($$$)'`, `search 'WHERE id = '`) without an owner or role predicate. |
| 28 | Predictable randomness | CSPRNG for tokens, codes and secrets. | `search 'math/rand\|Math.random\|random.random'` near token/code generation. |
| 29 | Hardcoded backdoor | No literal credentials, master passwords or bypass branches. | Gitleaks; `search 'password\s*[:=]\s*"'`; literal-user comparisons in auth code. |
| 30 | Brute-forceable interfaces | Throttle, lockout and monitoring on sensitive endpoints; identifiers unguessable or access-controlled. | Login/lookup routes without rate-limit middleware; sequential ids in public URLs. |
| 31 | Zero-amount purchase (0元购) | Server recomputes price and amount; never trusts client price/quantity; coupon and stock checks are atomic. | Request DTOs with `price\|amount\|total`; trace to order creation and payment. |

## D. Data layer: bounded, indexed, consistent queries

| # | Rot | Gate | Detect |
|---|---|---|---|
| 3 | Soft-delete inconsistency | Hard vs soft delete decided per table; if soft, every read, update, count and join applies the same filter. | `search 'deleted_at\|is_deleted'` → tables; queries on them without the filter. |
| 5 | N+1 / slow queries | Batch or join related reads; slow-query threshold set; index hot predicates before shipping list/detail paths. | Query calls inside `for`/`range`/`map` bodies; missing index for a hot `WHERE`. |
| 10 | Raw SQL fighting the ORM's PK strategy | Use the ORM consistently, or handle PK generation and `RETURNING` explicitly. | `search 'INSERT INTO'` with explicit ids beside ORM creates on the same table. |
| 11 | JSONB overuse | JSONB only for schemaless data; queried fields become indexed columns. | `search "->>'\|@>"` inside `WHERE` on hot paths. |
| 24 | Pagination drift (shared with A) | Every list query is paginated and bounded. | List endpoints without page/limit parameters. |
| 25 | Mixed ORM/raw SQL; unbounded lists | One data-access strategy; every list query has `LIMIT`. | `SELECT` in list functions without `LIMIT`; `.Find(&` without `.Limit(`. |
| 26 | Leading-wildcard `LIKE` | No `LIKE '%x%'` on hot paths without trigram/full-text index. | `search "LIKE '%\|ILIKE"` and `'%' \|\|` / `"%"+` concatenation. |

## E. Error and state: errors surface, states are contracts, writes are guarded

| # | Rot | Gate | Detect |
|---|---|---|---|
| 4 | Status codes scattered | HTTP and business codes defined once and referenced. | Raw `500`/`400`/`404` literals in handlers instead of named constants. |
| 7 | Unstable state machine | Full state set and legal transitions declared once; transitions explicit. | State string literals across files; `UPDATE … SET status` without a from-state predicate. |
| 8 | Empty state reported as error | No-data is success for list, search, dashboard and first-use flows; errors only for failed requests, invalid responses, denied permissions or single-resource not-found. | `sql.ErrNoRows` / `ErrNotFound` returned from list or search; error UI on an empty array. |
| 13 | Swallowed exceptions | Never catch and discard: handle, wrap with context, or rethrow; log with cause. | `ast --lang go 'if err != nil { }'`; `_ = $F($$$)` on fallible calls; `catch {}`, `catch (e) {}`, `except: pass`. |
| 27 | Race conditions | Concurrent writes use transactions, optimistic locking or state-encoded `WHERE` (`… WHERE status='pending'`) and check affected rows; isolation and locks per engine: [txn-design](../arc:verify/references/txn-design.md) §3–§4. | Read-modify-write of balance/counter/status without lock; unchecked `RowsAffected`. |

## G. Over-engineering: the cheapest correct code is the code not written

Ladder for writers ([coding-discipline.md](coding-discipline.md) §2): exists already → standard
library → platform feature → installed dependency → minimum new code. Reviewers report one line per
finding: `file:line: <tag> <what>. <replacement>.` with tag `yagni`, `stdlib`, `native`, `delete` or
`shrink`; end health reports with `net: -<N> lines, -<M> deps possible`.

| # | Rot | Gate | Detect |
|---|---|---|---|
| 37 | Speculative abstraction (`yagni`) | No interface with one implementation, factory with one product, wrapper that only delegates, config nobody sets, or layer with one caller, unless a contract or test seam requires it. | `arc-idx ast` for interface types, then `symbol` each method for implementer count; read wrappers whose body is one call. |
| 38 | Hand-rolled standard library (`stdlib`) | Use the language standard library before writing utilities. | `files 'util'`, `'helper'`; read each function against the stdlib of its language. |
| 39 | Dependency for a platform feature (`native`) | Platform first: `<input type="date">`, CSS, DB constraints, native SDK APIs before a package. | Manifest dependencies vs their import sites (`search 'import .*<pkg>'`); one or two call sites are candidates. |
| 40 | Removable flexibility or verbosity (`delete`, `shrink`) | Delete unused options, flags and branches; prefer the shorter equivalent with identical behavior. | `search` each flag or option name for readers; unread ones are candidates. |
| 41 | Untracked deliberate shortcut | Every intentional shortcut carries `debt: <ceiling>, <upgrade trigger>` (coding-discipline §2) or a task link; TODO/FIXME without either is debt. | `search '(//\|#) ?(debt\|ponytail\|TODO\|FIXME):'`; health lists them as a ledger grouped by file. |

## F. Agent execution integrity

| # | Rot | Gate |
|---|---|---|
| 34 | Broke the build, then thrashed to recover | Keep the project runnable at every step; verify after each meaningful change; fix forward or revert deliberately, never re-clone. |
| 35 | Half-finished refactor reported as done | Done means no placeholders, no half-migrated call sites, verification green; residual work reported. |
| 36 | Bulk replace broke everything | No unreviewed tree-wide `sed -i` / `find -exec sed` / `perl -i`; checkpoint first, edit directory by directory, verify between steps. |

### F coverage by `arc:code/scripts/gate.sh check`

| Item | Covered by check lines | Still manual |
|---|---|---|
| 34 | baseline from `start` (change set excludes pre-existing dirt), `format(<stack>)` (check mode), `lint(<stack>)`, `tests(<stack>)` (affected packages; full suites with `--strict`) | service actually starts |
| 35 | `placeholders` and `suppressions` (changed lines only), baseline | old symbol has 0 remaining references (code-intel §4 row 2); reachable behavior per execution-truth §2 |
| 36 | scope (WARN above the file-count limit; FAIL only under a scope lock) | blocking the rewrite command itself (contract §6 approval); checkpoint before the rewrite |
