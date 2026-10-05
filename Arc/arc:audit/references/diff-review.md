# Diff, PR and Branch Review

Read-only review of a change set. Queries and impact rows: [code-intel.md](../../_shared/code-intel.md).
Rubrics: [api-contract.md](../../_shared/api-contract.md),
[go-backend.md](../../_shared/conventions/go-backend.md),
[frontend.md](../../_shared/conventions/frontend.md),
[comments.md](../../_shared/conventions/comments.md),
[code-rot.md](../../_shared/code-rot.md). Security order: [appsec.md](appsec.md) Phase 3.

## 1. Obtain the diff

| Input | Commands |
|---|---|
| Branch or PR refs present locally | `git merge-base <base> <head>`; `git diff --stat <base>...<head>`; `git diff --numstat <base>...<head>`; `git diff --name-status <base>...<head>`; `git diff -U10 <base>...<head> -- <path>` |
| Uncommitted work | `git diff HEAD --stat`; `git diff HEAD -U10 -- <path>`; `git status --porcelain --untracked-files=all` (read untracked files whole) |
| One commit | `git show --stat <sha>`; `git show -U10 <sha> -- <path>` |
| Patch file from the user | read the file; `git apply --numstat <file>` for sizes (prints only, applies nothing) |

- Base default: the ref the user named; else `origin/HEAD`, `origin/main`, `origin/master`,
  `main`, `master`, first that exists locally.
- Read code at the head revision: the working tree when head is `HEAD`, else
  `git show <head>:<path>`.
- Never `fetch`, `pull`, `checkout`, `switch`, `stash` or `push`. IF the base or head ref is
  missing locally THEN report `BLOCKED` and ask for a locally available ref or a patch file.
- A remote PR (CNB, GitHub) is reviewed from refs or a patch the user already has locally;
  posting comments is `cnb-code-review`'s job.

## 2. Classify and size

Exclude from the line count (list them in coverage): generated files (review their generator
input instead), vendored code, formatting-only changes (confirm with `git diff -w --stat`).
Lockfile and manifest changes are not excluded; they trigger the dependency check in §5.

| Tier | Contents |
|---|---|
| 1 | auth, session, permission; money, payment, credits; schema migrations and SQL; public routes, DTOs, events, error codes; concurrency, transactions, locks; security config, secrets, env, deploy; deletion and data lifecycle; dependency manifests |
| 2 | other business logic, handlers, services; frontend state and data fetching |
| 3 | tests, docs, comments, styles, log-message-only changes |

Changed lines = added + deleted from `--numstat` after exclusions.

- IF ≤ 1,500 THEN review every tier in full.
- IF > 1,500 THEN Tier 1 in full; Tier 2 at least one file per directory, highest churn first,
  up to 600 lines; Tier 3 by file name only. State the sampled scope: `reviewed X of Y changed
  lines; Tier 1 full; Tier 2 sampled: <files>`.
- IF > 5,000 lines or > 100 files THEN do the Tier 1 review and recommend splitting the
  change by commit or directory in the report.

## 3. Map changed symbols

For every hunk in scope, name the enclosing function, method, type, route, DTO field, column,
config key, error code or event (hunk header, then read the file at head). Record:

| Symbol | Kind | Public | Change | `file:line` |
|---|---|---|---|---|
| `OrderService.Cancel` | method | yes | semantics: now refunds credits | `internal/order/service.go:142` |

Public = exported identifier, route, DTO field or json tag, column, config key, error code,
event or message type. Change = added / removed / renamed / signature / semantics.

## 4. Impact on changed public symbols

For each public symbol whose signature, name or meaning changed, run all eight rows of the
code-intel impact checklist against the head revision and record them in code-intel §7 format.

- Removed or renamed symbol: any remaining reference outside the diff is a finding (`High`;
  `confirmed` when the reference is a direct call or field access).
- A caller, serializer, client type, test or doc outside the diff that still relies on the old
  meaning is a finding at that caller's `file:line`.
- Unresolved dynamic dispatch stays `likely` with `dynamic — confirm at runtime`.

## 5. Per-hunk checks (Tier 1 and Tier 2)

Mark each row checked or N/A per file.

| Area | Check |
|---|---|
| Contract | route, DTO, status, error-code and pagination changes follow api-contract; server, client, docs, tests and migrations change together |
| Conventions | layering, dependency direction, naming, DI and frontend state rules of the stack's conventions file |
| Error paths | errors returned and wrapped, never swallowed (E13); named status codes (E4); empty result is success (E8) |
| Nil, empty, bounds | nil dereference, empty collection, zero value, index bounds; every list bounded (D25, D24) |
| Concurrency, transactions | shared mutable state, goroutine/promise leaks, read-modify-write guarded (E27), all writes of one business action inside one transaction, affected rows checked |
| Data layer | soft-delete filter (D3), N+1 (D5), leading-wildcard LIKE (D26), JSONB filters (D11); migrations reversible and never editing an applied migration |
| Security | authentication and ownership/role on every access (C12); server-side amount (C31); brute-force limits (C6, C30); CSPRNG (C28); no literal secrets (C29); injection, SSRF, upload paths; secrets or tokens in logs |
| Dependencies | new or upgraded package: maintained (B20), justified, no known advisory for that version (scan mode can confirm) |
| Rot introduced | duplicated constants or helpers (A16, B23); code orphaned by this diff left behind (B14); speculative surface (B32); `TODO`/`FIXME`/not-implemented in changed lines (F35) |

A rule that is a preference without a source in the files above is not a finding.

## 6. Tests versus behavior

Behavior change = changed condition, return value, error, query, contract or state transition.

- IF behavior changed AND no test in the diff exercises the changed path (impact row 7 on the
  symbol) THEN finding `Medium`, owner `arc:verify`.
- IF a test changed while behavior did not, check for weakened assertions: removed asserts,
  broadened expected errors, added skips, loosened golden files. Hiding a failure is a `High`
  finding (paper-over).
- IF a test was deleted without a stated reason in the commit message or description THEN
  finding `Medium`.
- Do not run tests or builds. Analyzers in check mode (`go vet ./...`, `tsc --noEmit`, `eslint`
  without `--fix`) are allowed when the read-only status check stays unchanged afterwards.

## 7. Output

```text
| ID | Sev | Status | Blocking | file:line | Evidence | Fix direction | Owner |
| F-001 | High | confirmed | yes | api/order.go:88 | GetOrder loads by id only; Q2 → no owner predicate | add owner_id = caller to OrderRepo.Get (repo/order.go:41) | arc:code |
```

- `file:line` refers to the head revision. Evidence is a hunk excerpt of at most 5 lines or a
  query line in code-intel §7 format.
- Blocking = correctness, security, data loss or contract break. Readability and style are
  advisory and never blocking.
- Owner: `arc:code` for source fixes; `arc:verify` for missing or weakened tests and proof;
  `arc:plan` when three or more confirmed or likely findings need a tracked campaign.
- Order: blocking first, then severity, then tier.
- Verdict: `FAIL` if any blocking finding is `confirmed` or `likely`; `PASS` only when §8
  holds; `BLOCKED` if the diff cannot be obtained. Unreviewed parts are listed as `NOT_RUN`
  with the reason.

## 8. "No findings" criteria

Write "no findings in reviewed scope" only when all hold:

1. Every Tier 1 file was read in full at head.
2. Every changed public symbol has its eight impact rows recorded.
3. §5 is marked checked or N/A for every Tier 1 and Tier 2 file in scope.
4. §6 was applied to every behavior change.
5. The sampled scope is stated when the diff exceeded 1,500 lines.
6. Zero `confirmed` or `likely` findings; `needs-triage` items are listed separately.

Never write "safe", "approved" or "LGTM".
