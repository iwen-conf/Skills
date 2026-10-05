---
name: arc:audit
version: 2.0.0
description: >-
  Read-only evaluation (只读) that produces ranked, evidence-backed findings and never edits code:
  review a diff, PR or branch (代码评审), project health and code-rot audit (体检, 技术债),
  AppSec audit and threat model, secret, dependency and SAST scans (安全扫描, Semgrep, 漏洞), and
  authorized DAST. Not for fixes (use arc:code), tests (use arc:verify), or posting comments on
  CNB pull requests (use cnb-code-review).
---
# arc:audit

Read-only review that turns a diff, a repository or an authorized target into ranked, evidence-backed findings.

## Trigger

- Review a diff, PR, branch, commit or patch: "review this PR", "代码评审", "审查这次改动", "这个分支能合吗".
- Audit health, code rot or tech debt: "体检", "技术债盘点", "代码腐化", "scorecard", "release readiness review".
- Audit security or build a threat model: "安全审计", "漏洞审计", "AppSec review", "威胁建模", "只读审查".
- Scan secrets, dependencies or code: "漏洞扫描", "密钥扫描", "SCA", "Semgrep", "Trivy", "Gitleaks", "govulncheck", "npm audit".
- Run DAST (ZAP, nuclei, schemathesis) on a target the user authorized in writing; re-scan fixed paths.

## Non-trigger

- Fix a defect or add a control → `arc:code`.
- Write or run tests, prove a fix, release evidence → `arc:verify`.
- Turn findings into task documents or a tracked campaign → `arc:plan`.
- Map unfamiliar code or run index operations without judging it → `arc:explore`.
- Post review comments on a CNB pull request → `cnb-code-review`.
- Probe third-party or production systems without written authorization → refuse.

## Goal

Deliver a findings table where every row has `file:line`, evidence, severity and status, plus a coverage table, with the repository's `git status` unchanged.

## Inputs

| Input | Required | Default |
|---|---|---|
| target | yes | review: `<base>...HEAD`; other modes: repository root |
| mode | no | chosen by the Intent Router |
| base / head refs or patch file | review | base per [diff-review.md](references/diff-review.md) §1; head `HEAD` |
| task slug | no | `audit-<mode>-<YYYYMMDD>` |
| focus (auth, data, config, module) | no | whole mode scope |
| DAST target + written authorization | scan with DAST | none: DAST not run |
| install permission | no | not granted: missing tools skipped |
| prior findings (earlier reports, `.lark.json` records) | no | none |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Diff, PR, branch, commit range or patch file | review | [references/diff-review.md](references/diff-review.md) |
| Health, code rot, tech debt, maintainability, architecture or release readiness; "审计/audit" with no security word | health | [../_shared/code-rot.md](../_shared/code-rot.md) |
| Security, vulnerability, AppSec, threat model, data exposure, AuthZ | appsec | [references/appsec.md](references/appsec.md) |
| Scanner name, secrets/dependency/SAST scan, DAST, re-scan | scan | [references/scanners.md](references/scanners.md) |
| AppSec audit plus tool scans | appsec, then scan | [references/appsec.md](references/appsec.md), [references/scanners.md](references/scanners.md) |

## Workflow

1. Read [../_shared/contract.md](../_shared/contract.md) and [../_shared/code-intel.md](../_shared/code-intel.md); run the code-intel preflight. Record the baseline:
   `mkdir -p .arc/artifacts/<slug>/logs && git status --porcelain --untracked-files=all -- . ':(exclude).arc/artifacts' > .arc/artifacts/<slug>/logs/status-before.txt`.
   IF `.lark.json` exists THEN read its prior risk and audit records. Output: baseline file, mode, scope.
2. Read the rubric for each stack in scope: [go-backend.md](../_shared/conventions/go-backend.md) + [go-type-safety.md](../_shared/conventions/go-type-safety.md), [frontend.md](../_shared/conventions/frontend.md), [api-contract.md](../_shared/api-contract.md) when routes or DTOs are in scope, [comments.md](../_shared/conventions/comments.md) when comments are reviewed. Output: rubric list in the report.
3. Run the mode:
   - **review**: [diff-review.md](references/diff-review.md) §1–§8. Output: symbol table, impact rows, verdict.
   - **health**: pick hotspots from `arc-idx stats --format json` and `git log --since=12.months --name-only --format= | sort | uniq -c | sort -rn | head -30`. Screen all 36 [code-rot](../_shared/code-rot.md) items with their Detect queries, marking each `finding`, `clean` or `N/A`; also check architecture boundaries against the conventions, dependency health, tests, state/data risks and release readiness. Output: 36-row screen table.
   - **appsec**: [appsec.md](references/appsec.md) Phases 0–6. Output: assets, data map, hypotheses, finding cards, manual gaps.
   - **scan**: [scanners.md](references/scanners.md): asset snapshot, `security-scan.py` quick mode, DAST only under the authorization rule, triage of every signal group. Output: `security/` reports and a triage table.
4. Diagnose every suspected bug or security finding through the evidence-first root-cause repair gate in [execution-truth.md](../../docs/execution-truth.md) §6: observed vs expected, hypothesis, a check that could disprove it, root cause, affected siblings. Mark each analogous path sharing the boundary `checked`, `out of scope` or `unverified`. Output: status per finding.
5. Write finding cards to `.arc/artifacts/<slug>/reports/findings.md`: ID, severity, status, `file:line`, evidence (query or command with result), impact, fix direction, fix owner, verification suggestion; appsec cards add permission class and data yield.
6. Merge findings with one root cause into one card listing every location. Rank review findings by blocking, severity, tier; other modes by severity, then data yield. Write `reports/coverage.md`: checked, `N/A`, not checked (with reason), manual gaps, sampled scope. Output: Top-N list (N ≤ 10).
7. Hand off per Decision Rules; Lark status follows contract §7.
8. Re-run the step 1 status command into `logs/status-after.txt`; `diff` it against `status-before.txt`. Report per contract §9.

## Tool Strategy

- **Preferred**: code-intel commands; project-native inventories (route registration, OpenAPI, migrations, env samples, deploy manifests); read-only git (`diff`, `log`, `show`, `blame`, `merge-base`); `scripts/` for tool scans.
- **Query order**: (1) code-intel ladder for symbols and literals; (2) AST patterns for risky shapes: raw SQL, shell execution, upload, SSRF fetch, JWT parsing, payment amount; (3) inventories; (4) git history; (5) scanners, scan mode only; (6) host search under the code-intel fallback conditions.
- **When a query fails**: widen per code-intel §5; after the last rung record `not found` with every query and add the area to coverage as not checked. A failed query never supports a finding or a "no findings" claim.
- **Forbidden**:
  - creating or changing any file outside `.arc/artifacts/<slug>/`, including formatters, `--fix` flags, generators and tests that write;
  - git commands that change state or contact a remote: `commit`, `checkout`, `switch`, `stash`, `reset`, `merge`, `rebase`, `fetch`, `pull`, `push`;
  - DAST, fuzzing, exploit payloads or brute force without written authorization naming the target;
  - installing tools without permission; uploading source, reports, SARIF or secrets to cloud services;
  - printing secret values (cite `file:line` and a redacted prefix);
  - `confirmed` status without a disproving check; preferences presented as defects; numeric total scores without a user-supplied template whose every cell cites evidence;
  - posting PR comments.
- **Fallback**: `arc-idx` unavailable → host search labeled `fallback:<tool>`; scanner missing or failed → that layer's manual checklist in [scanners.md](references/scanners.md), status `manual-gap`; no git repository → review a user-provided patch file.

## Decision Rules

- **Status**
  - IF a reproduction ran, or an entry → sink trace has `file:line` for every hop, AND a disproving check failed to disprove it THEN `confirmed`.
  - IF code evidence holds but one hop is unverified (dynamic dispatch, runtime config, environment) THEN `likely`.
  - IF the only evidence is a scanner signal, a pattern match or inference THEN `needs-triage`.
  - IF the disproving check succeeds (unreachable, fixture, sanitized upstream, mitigated elsewhere) THEN `false-positive` with that evidence.
  - IF it cannot be decided statically (AuthZ matrix, payment flow, race, live config) THEN `manual-gap`.
- **Severity**: security per [appsec.md](references/appsec.md) Phase 5. Otherwise Critical = data loss, corruption or outage on a main path; High = wrong result on a main path or a contract break for existing clients; Medium = edge-case defect, missing test for changed behavior, partial exposure; Low = maintainability defect with a named code-rot or conventions rule; Info = hardening or doc mismatch. Priority comes only from the contract §4 mapping.
- IF scanner severity differs from reachability, auth context or data yield THEN re-rank and record both.
- IF a finding has no source rule (contract, conventions, api-contract, code-rot) and no defect THEN it is not a finding. An established frontend stack that differs from the default is not a defect; same-duty library duplication, mixed state layers and undocumented exceptions on new surfaces are.
- IF the fix direction does not name a layer and a change ("add `owner_id` predicate to `OrderRepo.Get`") THEN rewrite it; "strengthen validation" is rejected.
- **Review**: correctness, security, data loss and contract break are blocking; readability and style are advisory. IF any blocking finding is `confirmed` or `likely` THEN verdict `FAIL`.
- IF nothing is found THEN write "no findings in checked scope" beside the coverage table; never "safe".
- **Size**: IF a diff exceeds 1,500 changed lines THEN use the risk tiers in diff-review.md §2. IF health or appsec scope exceeds 50 entrypoints or 2,000 source files THEN deep-review the top 20 entrypoints or hotspots by data yield × exposure (appsec) or churn (health) and list the rest as not checked.
- **Scan volume**: IF more than 500 raw signals THEN group by rule and root cause, triage the top 50 groups by severity × yield, and leave the rest `needs-triage`. Run `quick` before `full` on any monorepo.
- **DAST**: IF the user has not written an authorization naming the target and environment THEN do not pass `--target-url` or `--mode full`; mark DAST `BLOCKED`.
- **Installs**: IF a tool is missing and install permission is absent THEN skip it and run its manual checklist.
- **Release gate**: the scan report is evidence, not a gate. IF the project has no explicit gate policy THEN report "no gate decision"; absent or unknown evidence never passes a gate.
- **Handoff**: IF the user asks for remediation tasks THEN write the handoff package at the path and in the format of [finding-campaign.md](../_shared/finding-campaign.md) §2 and name `arc:plan`. IF three or more `confirmed`/`likely` findings need fixes across sessions THEN recommend `arc:plan` in Next. IF one confirmed finding sits in one module THEN hand the card to `arc:code`; a small scoped fix needs no task tree. Proof of a fix goes to `arc:verify`.

## Verification

| Claim | Evidence |
|---|---|
| Nothing changed | `diff logs/status-before.txt logs/status-after.txt` prints nothing |
| Finding `confirmed` | trace hops as `file:line` plus the disproving check and its result |
| Impact covered (review) | eight code-intel rows per changed public symbol |
| Health screened | 36 rows, each `finding`, `clean` or `N/A` with the query run |
| Scan ran | per-tool `status` and `exit_code` from `security-summary.json`; raw artifact paths |
| Coverage complete | coverage table lists every in-scope area as checked, `N/A`, not checked or `manual-gap` |
| Priority | severity rule cited; priority from the contract mapping |

## Failure Recovery

| Failure | Action | Budget | Stop |
|---|---|---|---|
| Base or head ref missing | try the base fallbacks in diff-review.md §1 | 1 pass | `BLOCKED`; ask for a local ref or a patch file |
| `arc-idx` unhealthy | one repair per code-intel §1, then fallback search | 1 | continue on fallback, labeled |
| Query returns nothing | code-intel §5 widening | 5 rungs | `not found`; coverage gap |
| Scanner error or timeout | read `logs/<tool>.stderr.txt`; rerun with doubled `--timeout` or a narrower path | 1 | manual checklist; `manual-gap` with command and error |
| Install fails | record command and error | 1 | continue with available tools |
| Scanner and code disagree | re-trace the code; code wins | 1 per finding | `false-positive` with evidence, else `needs-triage` |
| Status differs from baseline | list the changed paths; revert nothing | 0 | stop and report to the user |
| Scope exceeds the size rules | deliver the Top-N and the coverage table | 1 pass | report the unchecked remainder |

## Completion Criteria

- [ ] `status-before.txt` and `status-after.txt` are identical.
- [ ] Every finding has ID, severity, status, `file:line`, evidence, fix direction and owner.
- [ ] No `confirmed` finding lacks a trace and a disproving check.
- [ ] Coverage table lists checked, `N/A`, not checked, manual gaps and sampled scope.
- [ ] Top-N list (N ≤ 10) with priorities from the contract mapping.
- [ ] Mode output present: review → symbol table, impact rows, verdict; health → 36-item screen; appsec → assets, data map, hypotheses, finding cards, manual gaps; scan → tool status table and triage of every signal group.
- [ ] Secrets redacted in every quoted output.
- [ ] Contract §9 report written with the Next handoff.
