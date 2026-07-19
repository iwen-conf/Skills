# AppSec Playbook (Cross-Project)

Use this playbook whenever `arc:audit` runs with `risk_focus`/`mode` of `security`, `appsec`, `vuln`, or when the user asks for code / project / vulnerability audit across any repository.

This is methodology, not scanner automation. For CLI scanners, hand off to `arc:security` after Phase 1 assets exist.

## Iron Priorities

```text
1. Sensitive data and credentials beat "how many shells/CVEs".
2. Asset inventory before AI deep dives or mass POC scanning.
3. Config / secret / default-admin surfaces before flashy RCE narratives.
4. Soft targets and edge modules before core product rabbit holes.
5. Every finding states permission class + data yield + exploit conditions.
6. AI washes tables and reviews scoped modules; tools do bulk recon.
```

## Phase 0 — Scope Gate (≤ 30 min)

Confirm before inspecting code:

| Gate | Required answer |
|---|---|
| Target path | Repo root or monorepo slice |
| Environments allowed | local only / test / staging / prod (prod needs explicit auth) |
| Active testing | none (default) / authorized DAST via `arc:security` |
| Focus | full appsec / auth only / data only / config-deploy only / PR diff |
| Out of scope | protected paths, third-party trees, personal devices |

If authorization for live probing is unclear, stay read-only and mark active checks as skipped.

## Phase 1 — Asset Table (Recon First)

Do **not** start with free-form AI “audit the whole repo”.

Build one inventory first:

| Asset class | How to collect | Minimum fields |
|---|---|---|
| HTTP/API surface | routers, OpenAPI, gateway configs | method, path, auth required, role, handler |
| Web/SSR/forms | templates, cookie middleware | route, session/cookie, CSRF risk |
| Admin / privileged | admin routes, permission constants | action, permission, object scope |
| Identity | login, register, reset, OAuth, JWT, sessions | endpoint, token type, storage |
| Data stores | migrations, entities, schemas | collection/table, sensitive columns |
| Files / storage | upload, signed URL, object storage | path, ACL, content type checks |
| Payments / credits | order, callback, wallet, points | amount source of truth, idempotency |
| Integrations | SMS, email, search, recommendation, SSO | secrets, SSRF, trust boundary |
| Config / ops | env samples, deploy scripts, debug/pprof | secret presence, exposure risk |
| Clients | web/mobile/desktop token handling | storage, route guards |

Rules of thumb (from real red-team friction):

- First pass is **alive + common surface only**: enumerate routes and auth gates; do not mass-run every POC.
- Expand inventory when subdomains/modules appear outside the initial list (root domain / shared IP style expansion in code terms: parent packages, shared gateways, optional features).
- Prefer project-native lists + local index search over AI guessing.
- AI is good at **deduping and formatting** asset exports, not discovering unknown surfaces alone.

Deliverable:

```text
assets.md
- Public endpoints
- Authenticated endpoints
- Privileged endpoints
- Config/ops exposure candidates
```

## Phase 2 — Sensitive Data Map

Map **what can be stolen**, not only **what can be executed**.

Tag fields with at least:

| Tag | Examples |
|---|---|
| `PII` | phone, email, real name, ID number, address, student/employee id |
| `AUTH` | password hash, reset token, session, refresh token, API key |
| `PAY` | order, payment intent, card last4, refund, invoice |
| `BIZ` | unpaid content, draft chapter, internal price, audit log |
| `SECRET` | DB DSN, JWT secret, cloud keys, webhook secret |

For each store/export path, estimate yield class:

| Yield class | Guidance |
|---|---|
| L0 | No sensitive fields observed |
| L1 | Single low-value field or non-personal business metadata |
| L2 | Credentials or multi-field PII possible at hundreds–thousands of rows |
| L3 | Multi-field PII or payment data at large scale, or mass export path |
| L4 | Bulk export / dump of citizen-grade or payment-grade data without strong auth |

Deliverable:

```text
data-map.md
- Entity/table → fields → tags → yield class → access paths
```

## Phase 3 — Soft-Target Queue

Prioritize by **data yield × exposure × effort**, not by tool severity alone.

Default soft-target order:

1. Secrets in config, samples committed as real values, deploy scripts, debug endpoints
2. AuthN gaps: brute force, OTP/SMS flood, reset/forgot password half-implemented flows
3. AuthZ/IDOR: object-level access, tenant/user ownership filters
4. Privileged API missing permission checks
5. Upload / path traversal / signed URL abuse
6. Payment amount tampering / callback forgery / double spend of credits
7. Injection (SQL/NoSQL/SSTI/command) on high-yield queries
8. SSRF / webhook / internal metadata
9. Frontend token leakage and missing route guards
10. Supply chain / dependency CVEs only after reachability notes

“Soft target” means edge modules, obscure admin tools, half-finished features, third-party commercial subsystems, and ops surfaces—not only famous CVE names.

## Phase 4 — Verification Style

| Mode | Allowed | Forbidden |
|---|---|---|
| Read-only audit (`arc:audit`) | code path proof, config proof, static reachability | exploit payloads against unauthorized hosts |
| Local automation (`arc:security` quick) | SAST/SCA/secrets/govulncheck on local tree | claiming business-logic coverage |
| Active testing (`arc:security` full) | authorized local/test DAST/fuzz only | third-party production without written auth |

When a config leak or credential pattern is found, treat it like a high-priority “web.conf” class finding even if no RCE is proven yet.

After gaining a theoretical high-privilege path, still check **local intelligence** on that path: secrets files, connection strings, export caches, admin tokens—not only “can we scan further.”

## Phase 5 — Finding Card (Mandatory Shape)

Every confirmed finding uses this card. Prefer this over CVSS-only narratives.

```markdown
### [SEV] Title

- **Permission class**: anonymous | user | author | admin | service | config/ops
- **Data yield**: tags + estimated rows/fields (or “capability only”)
- **Why it scores**: one sentence linking permission → obtainable data/control
- **Evidence**: path + symbol + snippet/command (no full secrets)
- **Exploit conditions**: auth needed, race, feature flag, env-only
- **Blast radius**: single tenant / whole DB / payment integrity / account takeover
- **Detection gap**: would logs/alerts catch abuse?
- **Fix direction**: concrete, scoped
- **Residual risk**: what remains after fix
- **Status**: confirmed | likely | assumption | manual-gap
```

Severity heuristic (override with evidence):

| SEV | When |
|---|---|
| Critical | unauth or low-priv path to bulk PII/PAY/AUTH secrets, RCE with data path, payment integrity break |
| High | authz bypass to other users’ sensitive data, admin action as normal user, secret in deployable artifact |
| Medium | limited injection, misconfig with partial exposure, dependency with clear reachability |
| Low | defense-in-depth, hard-to-reach, low-yield information leak |
| Info | hardening, missing telemetry, documentation mismatch |

## Phase 6 — Report Bundle + Task Handoff

Minimum outputs for any project:

1. `assets.md` — attack surface inventory  
2. `data-map.md` — sensitive data map  
3. `hypotheses.md` — top 10 ordered by data yield (before deep proof)  
4. `findings.md` — finding cards  
5. `manual-gaps.md` — what scanners cannot cover  
6. Optional: `arc:security` for automation  

When remediation will be multi-finding or tracked, also emit a **Handoff Package** for `arc:sdlc` (role `R-task`). Do not skip straight to code.

Handoff Package (narrow; see task pipeline):

```text
security-handoff/
  00-项目定位与口径.md   # product positioning + engineering caliber + role matrix
  01-assets.md
  02-data-map.md
  03-findings.md
  04-manual-gaps.md
  06-task-seed.md        # suggested T-groups by risk domain only
```

`00-项目定位与口径.md` must answer, from **this** repo (not generic fluff):

| Block | Content |
|---|---|
| 项目定位 | product form, user roles, critical data, trust boundaries, engagement purpose |
| 项目口径 | deploy shape, naming/storage, protected paths, env order, architecture limits, test auth boundary |
| 功能角色 | R-recon / R-scan / R-task / R-fix / R-verify ownership for this run |

Then:

```text
arc:sdlc  # expands seeds into very detailed subtasks
  → arc:fix / arc:build per subtask
```

Full field contracts: [`../../arc:sdlc/references/security-audit-task-pipeline.md`](../../arc:sdlc/references/security-audit-task-pipeline.md).

Do **not** hide uncertainty behind fake numeric total scores. Optional compact score tables are allowed only when each cell cites evidence.

## Multi-Project Operating Notes

- Keep the same phases for Go, Node, Python, Java, mobile monorepos; only collectors change.
- Protected trees (reference clones, iOS-only zones, vendor dumps) stay out of scope unless the user names them.
- Prefer one person/agent on inventory + data map, another on soft-target deep dives when parallelizing.
- Fatigue rule: if inventory is incomplete, do not jump to exploit-style conclusions.
- Public/demo environments: redact tokens and production-like secrets in chat; keep raw evidence local.

## Anti-Patterns

- Trusting AI to replace route enumeration on multi-service repos
- Reporting “admin RCE-equivalent” without stating obtainable data
- Mass scanning every package before soft-target triage
- Fixing code during read-only audit
- Treating dependency CVE counts as the audit result
- Skipping config/deploy/debug surfaces because “business code looks fine”
- Claiming full coverage when AuthZ and payment flows were never manually reviewed
