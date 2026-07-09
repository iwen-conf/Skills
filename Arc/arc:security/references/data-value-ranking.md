# Data-Value Ranking for Security Findings

Use after scanners run (`arc:security`) or during AppSec review (`arc:audit` mode `appsec`).  
Re-rank tool severity using obtainable data and control, not CVSS alone.

## Ranking Inputs

For each finding, record:

| Input | Question |
|---|---|
| Permission class | anonymous / user / privileged / admin / service / config-ops |
| Auth context | public, low-priv, stolen token, insider |
| Data tags | PII, AUTH, PAY, BIZ, SECRET |
| Yield class | L0–L4 from `arc:audit` appsec playbook |
| Reachability | internet-facing, internal-only, dead code, needs chain |
| Environment | local sample, test, staging, production artifact |
| Exploit maturity | code-proven, scanner-only, needs manual chain |

## Re-Rank Rules

1. **Bulk multi-field PII or decryptable credentials** outrank isolated host/admin narratives with no data path.
2. **Payment integrity and credit double-spend** outrank low-impact XSS without session theft.
3. **Secrets in deployable config / images / scripts** outrank theoretical RCE in unreachable admin tooling.
4. **IDOR on orders, messages, phone numbers, or drafts** often beats “dependency has CVE” without call-graph reachability.
5. **Scanner Critical** becomes Medium/Low when unreachable, auth-blocked, or tool-false-positive after review.
6. **Scanner Low/Info** becomes High when it proves a real secret, public debug endpoint, or default admin path.

## Priority Buckets

| Bucket | Examples |
|---|---|
| P0 | Unauth bulk PII/PAY/AUTH; prod secrets in repo/artifacts; payment callback forgery |
| P1 | AuthZ bypass to other users’ sensitive objects; admin action as normal user; RCE with clear data path |
| P2 | Limited injection, misconfig with partial exposure, reachable high CVE with exploit notes |
| P3 | Hardening, missing headers, low-yield leaks, unreachable CVEs |
| Gap | AuthZ matrix, business workflow, race conditions—manual only |

## Report Language

Prefer:

```text
P0 — anonymous export of phone+name+order (L3) via GET /api/...
```

Avoid:

```text
Critical because CVSS 9.8 (no data path stated)
```

## Handoff

- Confirmed P0/P1 code issues → `arc:fix` or `arc:build` after task docs when multi-finding.
- Read-only methodology expansion → `arc:audit` appsec mode.
- Durable project risk rows → `arc:docs` only when Lark is active.
