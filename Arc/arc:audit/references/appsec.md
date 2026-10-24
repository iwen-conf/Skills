# AppSec Audit and Threat Model

Read-only, inventory-first security audit for any stack; only the collectors change per stack.
Vocabulary and artifact paths: [contract.md](../../_shared/contract.md). Queries:
[code-intel.md](../../_shared/code-intel.md). Write-time security rules: family C of
[code-rot.md](../../_shared/code-rot.md). Tool scans and DAST: [scanners.md](scanners.md). Attack
classes: [attack-classes.md](attack-classes.md). The boundary, invariant, severity-anchor and coverage
rules below adapt [cloudflare/security-audit-skill](https://github.com/cloudflare/security-audit-skill)
at `c1c8a8c` ([MIT](upstream/security-audit-LICENSE)).

## Priorities

1. Obtainable sensitive data and credentials outrank shell, RCE or CVE counts.
2. Asset inventory before any deep dive; no exploit-style conclusion while the inventory is
   incomplete.
3. Config, secrets, default admin and debug surfaces before core business logic.
4. Edge modules (obscure admin tools, half-finished features, third-party subsystems, ops
   surfaces) before core product paths.
5. Every finding states permission class, data yield and exploit conditions.
6. Tools and project-native lists collect surfaces; the model dedupes, formats and reviews
   scoped modules. It never replaces route enumeration on multi-service repositories.

## Phase 0: Scope gate

Record before reading code:

| Gate | Answer |
|---|---|
| Target | repository root or monorepo slice |
| Environments | local / test / staging / production (production needs explicit authorization) |
| Active testing | none (default) / authorized DAST via scan mode |
| Focus | full / auth only / data only / config-deploy only / diff |
| Prior runs | earlier `findings.md` and `coverage.md` for this target, or none |
| Out of scope | protected paths, reference clones, vendor dumps, third-party trees, personal devices |

IF authorization for live probing is unclear THEN stay read-only and list active checks as
`manual-gap`.

## Phase 1: Asset table

| Asset class | Collect from | Minimum fields |
|---|---|---|
| HTTP/API | router registration, OpenAPI, gateway config | method, path, auth required, role, handler `file:line` |
| Web/SSR/forms | templates, cookie middleware | route, session/cookie, CSRF exposure |
| Admin/privileged | admin routes, permission constants | action, permission, object scope |
| Identity | login, register, reset, OAuth, JWT, sessions | endpoint, token type, storage |
| Data stores | migrations, entities, schemas | table/collection, sensitive columns |
| Files/storage | upload, signed URL, object storage | path rule, ACL, content-type check |
| Payments/credits | order, callback, wallet, points | amount source of truth, idempotency |
| Integrations | SMS, email, search, SSO, webhooks | secret location, SSRF exposure, trust boundary |
| Config/ops | env samples, deploy scripts, debug/pprof | secret presence, exposure |
| Clients | web/mobile/desktop token handling | token storage, route guards |

Collector starting points (verify syntax per code-intel §4):

| Need | Query |
|---|---|
| Go routes | `ast --lang go '$R.GET($$$)'` (repeat for POST/PUT/PATCH/DELETE/Handle/HandleFunc/Group) |
| TS/Node routes | `search 'lang:typescript (router\|app)\.(get\|post\|put\|patch\|delete)\('` |
| Auth middleware | `search 'case:yes (RequireAuth\|AuthMiddleware\|jwt\|session)'` |
| Risky shapes | `search 'ExecContext\|Raw(\|exec.Command\|child_process\|http.Get(\|fetch(\|multipart\|ParseWithClaims'` |
| Secrets/config | `files '\.env\|config.*\.(ya?ml\|json\|toml)$'`; `search 'case:yes (SECRET\|PASSWORD\|DSN\|API_KEY)'` |

Rules:
- Every asset row also names its trust boundary and the strongest source-visible control on it.
- First pass is reachable, common surface: enumerate routes and auth gates; run no mass POCs.
- IF modules appear outside the initial list (parent packages, shared gateways, optional
  features) THEN add them to the table before continuing.

Output `assets.md`: public, authenticated, privileged endpoints; config/ops exposure candidates.

## Phase 2: Sensitive data map

Map what can be stolen, not only what can be executed.

| Tag | Examples |
|---|---|
| `PII` | phone, email, real name, ID number, address, student/employee id |
| `AUTH` | password hash, reset token, session, refresh token, API key |
| `PAY` | order, payment intent, card last4, refund, invoice |
| `BIZ` | unpaid content, drafts, internal price, audit log |
| `SECRET` | DB DSN, JWT secret, cloud keys, webhook secret |

| Yield | Meaning |
|---|---|
| L0 | no sensitive field |
| L1 | one low-value field or non-personal metadata |
| L2 | credentials, or multi-field PII at hundreds–thousands of rows |
| L3 | multi-field PII or payment data at scale, or a mass export path |
| L4 | bulk dump of citizen-grade or payment-grade data without strong auth |

Output `data-map.md`: entity/table → fields → tags → yield → access paths (route or job `file:line`).

## Phase 3: Soft-target queue and attack classes

For each surface select the ordinary classes and every domain in
[attack-classes.md](attack-classes.md) §2 whose boundary is present; record selected and excluded
domains with the source fact behind each. Order the resulting hypotheses by data yield × exposure ×
effort, not by tool severity:

1. Secrets in config, real values in samples, deploy scripts, debug endpoints.
2. AuthN gaps: brute force, OTP/SMS flood, half-implemented reset/forgot/register flows.
3. AuthZ/IDOR: object ownership and tenant filters.
4. Privileged APIs missing permission checks.
5. Upload, path traversal, signed-URL abuse.
6. Payment amount tampering, callback forgery, credit double spend.
7. Injection (SQL/NoSQL/SSTI/command) on high-yield queries.
8. SSRF, webhooks, internal metadata endpoints.
9. Frontend token leakage, missing route guards.
10. Dependency CVEs, only with reachability notes.

Output `hypotheses.md`: top 10 ordered by data yield, each `surface × boundary × class` and
`needs-triage` until Phase 4.

## Phase 4: Read-only verification

Work every hypothesis from a concrete invariant:

1. The lower-trust principal and what it starts with.
2. The accepted value, action, state transition or resource selector.
3. The control that should reject, bind, isolate, limit or revoke it.
4. The exact source path after that decision to the sink, including derived copies.
5. The smallest affected record, wrong return value or locally observable effect.
6. The source change and regression case that would enforce the invariant.

Rules:
- Read sibling, legacy, batch, retry, cancellation, migration and error paths that reach the same
  effect; compare their controls for equivalence, not presence.
- Try sad inputs the interface accepts: empty, zero, negative, maximum, duplicate, mixed encoding,
  stale, revoked, reordered, concurrent, failed dependency.
- Stop a line of inquiry once the invariant is settled either way and record it in coverage.
- After a High or Critical candidate, search the scope for lexical, structural and logical variants
  of the root cause; establish each variant's conditions separately.
- Allowed: entry → sink code trace, config proof, static reachability, read-only queries. A local
  reproduction runs only in a sandbox with no network, an empty allowlisted environment, a
  read-only target, writes confined to a scratch directory and CPU, memory, disk and time limits;
  without all of these, do not execute and mark the item `manual-gap` with the missing control.
- Forbidden here: exploit payloads, live requests, brute force. Live checks go to scan mode
  with written authorization, or stay `manual-gap`.
- A fact held outside the repository (proxy, identity provider, cloud policy, deployed config) is
  not proof either way: `manual-gap` naming the fact and the owner's check.
- On any theoretical high-privilege path, also check what it exposes locally: secrets files,
  connection strings, export caches, admin tokens.

Candidate gate (all must hold before `likely` or `confirmed`):
- Lower-trust principal, accepted input, intended control, crossed boundary, affected principal or
  resource, and a concrete result are all named.
- No stronger source-visible control blocks the path.
- The claim is no stronger than the evidence: a crash is not code execution, ordinary load is not a
  shared outage, a same-principal action is not privilege gain.
- A missing best practice with no affected principal or resource is hardening (`Info`), not a finding.

## Phase 5: Finding card and ranking

```markdown
### F-001 [Severity] Title
- Status: confirmed | likely | needs-triage | false-positive | manual-gap
- Fingerprint: stable root-cause id (path + symbol + class; no line, date or severity)
- Boundary: lower-trust principal → control → affected principal or resource
- Permission class: anonymous | user | author | admin | service | config/ops
- Data yield: tags + estimated rows/fields (or "capability only")
- Why it scores: one sentence linking permission → obtainable data or control
- Evidence: entry → sink hops as `file:line` + symbol; command/query results (no secret values)
- Exploit conditions: auth needed, race, feature flag, environment
- Blast radius: single tenant / whole DB / payment integrity / account takeover
- Detection gap: would logs or alerts catch abuse?
- Fix direction: the invariant and the narrowest change at the last trusted decision point; owner arc:code
- Verification: the check that fails before and passes after; owner arc:verify
- Residual risk: what remains after the fix
```

Ranking inputs per finding: permission class, auth context (public, low-priv, stolen token,
insider), data tags, yield, reachability (internet-facing, internal-only, dead code, needs
chain), environment (sample, test, staging, production artifact), exploit maturity
(code-proven, scanner-only, needs manual chain).

Severity (evidence overrides; priority follows only from the contract mapping). Only `confirmed` and
`likely` cards carry a severity; `needs-triage` and `manual-gap` carry none. Severity never exceeds
the demonstrated impact. High versus Medium: does the result fully defeat an explicit control for an
action with real consequences (High), or only weaken it (Medium)?

| Severity | When |
|---|---|
| Critical | unauthenticated or low-privilege path to bulk PII/PAY/AUTH or production secrets; RCE with a data path; payment integrity break or callback forgery |
| High | AuthZ bypass to other users' sensitive objects; admin action as a normal user; secret in a deployable artifact |
| Medium | limited injection; misconfiguration with partial exposure; reachable dependency CVE with exploit notes |
| Low | defense in depth, hard to reach, low-yield information leak, unreachable CVE |
| Info | hardening, missing headers or telemetry, documentation mismatch |

Re-rank rules (record tool severity and final severity):
1. Bulk multi-field PII or decryptable credentials outrank host/admin narratives with no data path.
2. Payment integrity and credit double spend outrank low-impact XSS without session theft.
3. Secrets in deployable config, images or scripts outrank theoretical RCE in unreachable admin tooling.
4. IDOR on orders, messages, phone numbers or drafts outranks a dependency CVE without call-graph reachability.
5. Tool Critical drops to Medium/Low when unreachable, auth-blocked or a false positive after review.
6. Tool Low/Info rises to High when it proves a real secret, a public debug endpoint or a default admin path.
7. AuthZ matrix, business workflow and race conditions without static proof are `manual-gap`, not a severity.

Write titles with the data path: `Critical — anonymous export of phone+name+order (L3) via GET /api/…`,
never `Critical because CVSS 9.8` with no data path.

## Phase 6: Bundle and handoff

Write under `.arc/artifacts/<task-slug>/reports/`:

1. `assets.md`
2. `data-map.md`
3. `hypotheses.md`
4. `findings.md` (finding cards)
5. `manual-gaps.md` (AuthZ matrix, payment flow, business logic, races, live config; use the
   manual checklist in [scanners.md](scanners.md))
6. `coverage.md`: one row per `surface × boundary × class` with status `covered`, `candidate`,
   `blocked`, `not-applicable`, `out-of-scope` or `deferred`, the paths read and the check run.
   The rows are the coverage claim; "auth reviewed" is not. A scoped or sampled run states that it
   is partial.

Prior runs: carry a prior `confirmed` card only when its source is unchanged, and re-verify it;
changed source makes it a new hypothesis. Prior `manual-gap`, `deferred` and `blocked` rows are
current work. A prior `false-positive` suppresses only the unchanged claim, never coverage of its
surface. Keep fingerprints stable across runs.

IF the user asks for remediation tasks THEN also write the handoff package at the path and in
the format of [finding-campaign.md](../../_shared/finding-campaign.md) §2 (project positioning
and caliber taken from this repository, finding cards, task seeds by risk domain only) and name
`arc:plan` as next. Never write implementation subtasks.

Numeric total scores are allowed only when the user supplies the template and every cell cites
evidence. Manual AuthZ, payment and business-logic gaps are listed even when no bug is proven.

## Parallel work

When the user runs several agents: one owns Phases 1–2, others take soft-target deep dives from
the Phase 3 queue; findings merge into one `findings.md` deduplicated by fingerprint. On hosts with
subagents, a fresh agent that did not find it re-verifies every High or Critical card before the
report; the parent alone writes `coverage.md`.

## Anti-patterns

- Checklist deviations presented as vulnerabilities.
- Guessing proxy, browser, identity-provider or deployment behavior the repository does not show.
- Same-principal or self-impact presented as a cross-boundary result.
- An effect reported stronger than the one observed; a report written before verification.
