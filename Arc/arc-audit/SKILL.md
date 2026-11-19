---
name: arc-audit
version: 3.0.0
description: >-
  Read-only evaluation (只读) producing ranked, evidence-backed code findings without editing code:
  diff/PR review (代码评审, 能不能合), project health, code rot and tech debt (体检, 技术债, 过度设计),
  HTTP API design review (HTTP API, 接口设计, 上传约束, JSON字段限制), AppSec threat model and pen test
  (AppSec, 威胁建模, 渗透测试), security/secret/dependency SAST scans (安全扫描, Semgrep, 漏洞) and
  authorized DAST. Not for fixes (use arc-code), tests (use arc-verify), or CNB PR comments (use cnb).
---

# arc-audit

Read-only review producing ranked, disproof-checked findings across diffs, API contracts, code health, AppSec, race windows and ledger invariants; edit zero project files.

## Non-trigger

- Code fixes → `arc-code`; test/storyline execution → `arc-verify`; remediation DAG/tasks → `arc-plan`; code search → `arc-explore`; CNB PR comments → `cnb` (review mode).

## Goal

Deliver evidence-backed, deduplicated findings with `file:line`, disproof checks, coverage table and an untouched worktree.

## Inputs

- `target` (default `<base>...HEAD` for review, else repo root); `mode` (`review|api-design|health|appsec|scan`); `DAST authorization` (required for DAST).

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| HTTP API, upload caps or JSON field bounds (HTTP API, 接口设计, 上传约束, JSON字段限制) | `api-design` | [api-design-review.md](references/api-design-review.md), [api-contract.md](../_shared/api-contract.md) |
| Diff, PR, branch or patch review (代码评审) | `review` | [diff-review.md](references/diff-review.md); re-review: [review-loop.md](references/review-loop.md) |
| Health, code rot, over-engineering or tech debt (体检, 技术债) | `health` | [code-rot.md](../_shared/code-rot.md) |
| Security, AppSec, AuthZ, threat model or race/ledger invariants (AppSec, 漏洞) | `appsec` | [appsec.md](references/appsec.md), [attack-classes.md](references/attack-classes.md) |
| Scanner run: Semgrep/Trivy/secrets/DAST (安全扫描, Semgrep, DAST) | `scan` | [scanners.md](references/scanners.md); `scripts/security-scan.py` |

## Workflow

1. Read [contract.md](../_shared/contract.md) and [code-intel.md](../_shared/code-intel.md). Save read-only worktree baseline: `git status --porcelain --untracked-files=all -- . ':(exclude).arc/artifacts' > .arc/artifacts/<slug>/logs/status-before.txt`.
2. Use `arc idx skeleton`, `arc idx impact` and `arc idx ast` to trace entry → sink paths, cross-entity invariants, AuthZ boundaries and concurrency race windows (e.g. concurrent callback vs cancellation lock).
3. Run the matched Intent Router mode (and `python3 <skill-dir>/scripts/security-scan.py` for `scan` mode).
4. Validate every suspected defect through the evidence-first root-cause repair gate in [execution-truth.md](../../docs/execution-truth.md) §6: observed vs expected, disproof attempt, root cause and sibling paths.
5. Write deduplicated findings to `.arc/artifacts/<slug>/reports/findings.md` and coverage to `reports/coverage.md` (Top-N ≤ 10 ranked by severity × impact).
6. Verify `status-after.txt` equals `status-before.txt`; hand off to `arc-code` (single fix), `arc-plan` (≥3 findings campaign) or `arc-verify` (runtime/race proof).

## Tool Strategy

- **Preferred**: `arc idx skeleton/impact/search/ast`, read-only `git diff/log`, `scripts/security-scan.py`.
- **Forbidden**: editing any file outside `.arc/artifacts/<slug>/`, mutating git commands, unauthorized DAST/fuzzing, printing raw secrets, or marking `confirmed` without a disproof check.

## Decision Rules

- IF entry → sink has full `file:line` evidence AND disproof fails → `confirmed`; if one hop is dynamic/unverified → `likely`; if disproof succeeds → `false-positive`.
- IF review finds confirmed/likely `new` correctness, security, data-loss or contract-break issues → `FAIL`; pre-existing/style are advisory.
- IF DAST has no written target authorization → `BLOCKED`.
- IF nothing found → state "no findings in checked scope" with the coverage table; never claim "safe".

## Verification

| Claim | Evidence |
|---|---|
| Worktree untouched | `diff status-before.txt status-after.txt` empty |
| Finding `confirmed` | `file:line` trace + disproof check result |
| Scan / coverage | `security-summary.json` exit codes + `reports/coverage.md` |

## Failure Recovery

| Failure | Action | Stop |
|---|---|---|
| Missing scanner | Use manual checklist in [scanners.md](references/scanners.md) | Label `manual-gap` |
| Scope > 1,500 lines / 50 entries | Prioritize top 20 hotspots by exposure/churn | List unchecked scope in `coverage.md` |
| Worktree modified unexpectedly | List changed paths; revert nothing | Stop and report |

## Completion Criteria

- [ ] `status-before.txt` and `status-after.txt` match identically; zero secrets printed.
- [ ] Every finding has ID, severity, status, `file:line`, disproof evidence, fix direction and owner.
- [ ] Top-N list (≤10), `coverage.md` and contract §9 handoff complete.
