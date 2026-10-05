# AppSec Audit and Threat Model

Read-only, inventory-first security audit for any stack; only the collectors change per stack.
Vocabulary and artifact paths: [contract.md](../../_shared/contract.md). Queries:
[code-intel.md](../../_shared/code-intel.md). Write-time security rules: family C of
[code-rot.md](../../_shared/code-rot.md). Tool scans and DAST: [scanners.md](scanners.md).

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

## Phase 3: Soft-target queue

Order by data yield × exposure × effort, not by tool severity:

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

Output `hypotheses.md`: top 10 ordered by data yield, each `needs-triage` until Phase 4.

## Phase 4: Read-only verification

- Allowed: entry → sink code trace, config proof, static reachability, read-only queries.
- Forbidden here: exploit payloads, live requests, brute force. Live checks go to scan mode
  with written authorization, or stay `manual-gap`.
- A config leak or credential pattern is High priority even with no RCE path.
- On any theoretical high-privilege path, also check what it exposes locally: secrets files,
  connection strings, export caches, admin tokens.

## Phase 5: Finding card and ranking

```markdown
### F-001 [Severity] Title
- Status: confirmed | likely | needs-triage | false-positive | manual-gap
- Permission class: anonymous | user | author | admin | service | config/ops
- Data yield: tags + estimated rows/fields (or "capability only")
- Why it scores: one sentence linking permission → obtainable data or control
- Evidence: entry → sink hops as `file:line` + symbol; command/query results (no secret values)
- Exploit conditions: auth needed, race, feature flag, environment
- Blast radius: single tenant / whole DB / payment integrity / account takeover
- Detection gap: would logs or alerts catch abuse?
- Fix direction: concrete layer and change; owner arc:code
- Verification: the check that fails before and passes after; owner arc:verify
- Residual risk: what remains after the fix
```

Ranking inputs per finding: permission class, auth context (public, low-priv, stolen token,
insider), data tags, yield, reachability (internet-facing, internal-only, dead code, needs
chain), environment (sample, test, staging, production artifact), exploit maturity
(code-proven, scanner-only, needs manual chain).

Severity (evidence overrides; priority follows only from the contract mapping):

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

IF the user asks for remediation tasks THEN also write the handoff package at the path and in
the format of [finding-campaign.md](../../_shared/finding-campaign.md) §2 (project positioning
and caliber taken from this repository, finding cards, task seeds by risk domain only) and name
`arc:plan` as next. Never write implementation subtasks.

Numeric total scores are allowed only when the user supplies the template and every cell cites
evidence. Manual AuthZ, payment and business-logic gaps are listed even when no bug is proven.

## Parallel work

When the user runs several agents: one owns Phases 1–2, others take soft-target deep dives from
the Phase 3 queue; findings merge into one `findings.md` with deduplicated IDs.
