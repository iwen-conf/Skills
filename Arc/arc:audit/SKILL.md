---
name: arc:audit
version: 2.7.0
description: >-
  Read-only evaluation (只读) that produces ranked, evidence-backed code findings and never edits code:
  review any diff, PR or branch (代码评审, 能不能合), project health, code rot, over-engineering and
  tech debt (体检, 技术债, 过度设计, 可删代码), HTTP API design review (接口设计, 上传约束, JSON字段限制),
  AppSec audit, threat model and source-first pen test (威胁建模, 渗透测试) across web, identity,
  client, cloud, CI, RPC, mobile and LLM-agent surfaces, secret, dependency and SAST scans (安全扫描,
  Semgrep, 漏洞), and authorized DAST. Business lifecycle gaps (状态机漏洞, 闭环检查) go to arc:plan model mode. Not for
  fixes (use arc:code), tests (use arc:verify), or posting comments on CNB pull requests (use cnb).
---
# arc:audit

Read-only review producing ranked, evidence-backed findings.

## Non-trigger

- Fix a defect or add a control → `arc:code`.
- Write or run tests, prove a fix, release evidence → `arc:verify`.
- Turn findings into task documents or a tracked campaign → `arc:plan`.
- Map unfamiliar code or run index operations without judging it → `arc:explore`.
- Post review comments on a CNB pull request → `cnb` (review mode).
- Business lifecycle gaps (dead ends, missing compensation, illegal transitions) → `arc:plan` model mode.
- Probe third-party or production systems without written authorization → refuse.

## Goal

Deliver located findings, evidence, severity/status and coverage; preserve source and supplied documents.

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
| HTTP API design, upload or JSON contract | api-design | [api-design-review.md](references/api-design-review.md); also review if a diff is supplied |
| Diff, PR, branch, commit range or patch file | review | [references/diff-review.md](references/diff-review.md) |
| Health, code rot, over-engineering (过度设计), tech debt, maintainability, architecture or release readiness; "审计/audit" with no security word | health | [../_shared/code-rot.md](../_shared/code-rot.md) |
| Security, vulnerability, AppSec, threat model, pen test, data exposure, AuthZ | appsec | [references/appsec.md](references/appsec.md), [attack-classes.md](references/attack-classes.md) |
| Scanner name, secrets/dependency/SAST scan, DAST, re-scan | scan | [references/scanners.md](references/scanners.md) |
| AppSec audit plus tool scans | appsec, then scan | [references/appsec.md](references/appsec.md), [references/scanners.md](references/scanners.md) |

## Workflow

Graph evidence review: [workflow §6](../_shared/workflow.md#6-graph-based-business-chain-verification-across-arc-skills), `arc-verify handoff --to arc-audit --graph <id>` (read-only). NOT_FOUND and RISK are not confirmed defects.

Document-only API reviews cite sections/lines; without a repository skip git/index steps.

1. Read [../_shared/contract.md](../_shared/contract.md) and [../_shared/code-intel.md](../_shared/code-intel.md); run the code-intel preflight. Record the baseline:
   `mkdir -p .arc/artifacts/<slug>/logs && git status --porcelain --untracked-files=all -- . ':(exclude).arc/artifacts' > .arc/artifacts/<slug>/logs/status-before.txt`.
   IF `.lark.json` exists THEN read its prior risk and audit records. Output: baseline file, mode, scope.
2. Read the rubric for each stack in scope: [go-backend.md](../_shared/conventions/go-backend.md) + [go-type-safety.md](../_shared/conventions/go-type-safety.md), [frontend.md](../_shared/conventions/frontend.md), [api-contract.md](../_shared/api-contract.md) when routes or DTOs are in scope, [comments.md](../_shared/conventions/comments.md) when comments are reviewed. Output: rubric list in the report.
3. Run the mode:
   - **review**: [diff-review.md](references/diff-review.md) §1–§8. Output: symbol table, impact rows, verdict.
   - **api-design**: [api-design-review.md](references/api-design-review.md): sourced constraints, findings, gaps and cases.
   - **health**: pick hotspots from `arc-idx stats --format json` and `git log --since=12.months --name-only --format= | sort | uniq -c | sort -rn | head -30`. Screen all 41 [code-rot](../_shared/code-rot.md) items with their Detect queries, marking each `finding`, `clean` or `N/A`; also check architecture boundaries against the conventions, dependency health, tests, state/data risks and release readiness. Output: 41-row screen table.
   - **appsec**: [appsec.md](references/appsec.md) Phases 0–6. Output: assets, data map, hypotheses, finding cards, manual gaps, coverage rows.
   - **scan**: [scanners.md](references/scanners.md): asset snapshot, `security-scan.py` quick mode, DAST only under the authorization rule, triage of every signal group. Output: `security/` reports and a triage table.
4. Diagnose every suspected bug or security finding through the evidence-first root-cause repair gate in [execution-truth.md](../../docs/execution-truth.md) §6: observed vs expected, hypothesis, a check that could disprove it, root cause, affected siblings. Mark each analogous path sharing the boundary `checked`, `out of scope` or `unverified`. Output: status per finding.
5. Write finding cards to `.arc/artifacts/<slug>/reports/findings.md`: ID, severity, status, `file:line`, evidence (query or command with result), impact, fix direction, fix owner, verification suggestion; appsec cards add permission class and data yield.
6. Merge findings with one root cause into one card listing every location. Rank review findings by blocking, severity, tier; other modes by severity, then data yield. Write `reports/coverage.md`: checked, `N/A`, not checked (with reason), manual gaps, sampled scope. Output: Top-N list (N ≤ 10).
7. Hand off per Decision Rules; Lark status follows contract §7.
8. Re-run the step 1 status command into `logs/status-after.txt`; `diff` it against `status-before.txt`. Report per contract §9.

## Tool Strategy

- **Preferred**: code-intel, project inventories (routes, OpenAPI, migrations, env/deploy config), read-only git and scan scripts.
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
  - Design contradictions use api-design's document-evidence rule; confirmation does not imply runtime exploitability.
  - IF a reproduction ran, or an entry → sink trace has `file:line` for every hop, AND a disproving check failed to disprove it THEN `confirmed`.
  - IF code evidence holds but one hop is unverified (dynamic dispatch, runtime config, environment) THEN `likely`.
  - IF the only evidence is a scanner signal, a pattern match or inference THEN `needs-triage`.
  - IF the disproving check succeeds (unreachable, fixture, sanitized upstream, mitigated elsewhere) THEN `false-positive` with that evidence.
  - IF it cannot be decided statically (AuthZ matrix, payment flow, race, live config) THEN `manual-gap`.
- **Severity**: security per [appsec.md](references/appsec.md) Phase 5. Otherwise Critical = data loss, corruption or outage on a main path; High = wrong result on a main path or a contract break for existing clients; Medium = edge-case defect, missing test for changed behavior, partial exposure; Low = maintainability defect with a named code-rot or conventions rule; Info = hardening or doc mismatch. Priority comes only from the contract §4 mapping.
- IF scanner severity differs from reachability, auth context or data yield THEN re-rank and record both.
- IF a finding has no source rule (contract, conventions, api-contract, code-rot) and no defect THEN it is not a finding. An established frontend stack that differs from the default is not a defect; same-duty library duplication, mixed state layers and undocumented exceptions on new surfaces are.
- Fix directions name the layer and change; "strengthen validation" is insufficient.
- **Review**: confirmed/likely correctness, security, data-loss or contract-break findings → `FAIL`; style is advisory.
- IF nothing is found THEN write "no findings in checked scope" beside the coverage table; never "safe".
- **Size**: IF a diff exceeds 1,500 changed lines THEN use the risk tiers in diff-review.md §2. IF health or appsec scope exceeds 50 entrypoints or 2,000 source files THEN deep-review the top 20 entrypoints or hotspots by data yield × exposure (appsec) or churn (health) and list the rest as not checked.
- **Scan volume**: IF more than 500 raw signals THEN group by rule and root cause, triage the top 50 groups by severity × yield, and leave the rest `needs-triage`. Run `quick` before `full` on any monorepo.
- **DAST**: IF the user has not written an authorization naming the target and environment THEN do not pass `--target-url` or `--mode full`; mark DAST `BLOCKED`.
- **Installs**: without install permission, use the missing tool's manual checklist.
- **Release gate**: the scan report is evidence, not a gate. IF the project has no explicit gate policy THEN report "no gate decision"; absent or unknown evidence never passes a gate.
- **Handoff**: IF the user asks for remediation tasks THEN write the handoff package at the path and in the format of [finding-campaign.md](../_shared/finding-campaign.md) §2 and name `arc:plan`. IF three or more `confirmed`/`likely` findings need fixes across sessions THEN recommend `arc:plan` in Next. IF one confirmed finding sits in one module THEN hand the card to `arc:code`; a small scoped fix needs no task tree. Proof of a fix goes to `arc:verify`.

## Verification

| Claim | Evidence |
|---|---|
| Worktree status preserved | Before/after status agrees; this alone does not prove content unchanged |
| Finding `confirmed` | Source trace or api-design document evidence, plus disproof/result |
| Impact covered (review) | eight code-intel rows per changed public symbol |
| Health screened | 41 rows, each `finding`, `clean` or `N/A` with the query run |
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
- [ ] Confirmed findings have source-trace or design-document evidence and disproof.
- [ ] Coverage table lists checked, `N/A`, not checked, manual gaps and sampled scope.
- [ ] API design: sourced constraints; design/implementation defects and unknowns separated.
- [ ] Top-N list (N ≤ 10) with priorities from the contract mapping.
- [ ] All selected-mode outputs from Workflow step 3 are present.
- [ ] Secrets redacted in every quoted output.
- [ ] Contract §9 report written with the Next handoff.
