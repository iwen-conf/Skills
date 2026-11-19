---
name: arc-vet
version: 2.0.0
description: >-
  Open-source due diligence before adding a dependency or generic component (开源选型, 技术选型,
  引入依赖, 不要重复造轮子, build vs buy): checks local code and standard library first, compares
  ≥3 GitHub candidates on substantive maintenance, stable releases, license compatibility (许可证),
  advisories and integration cost with dated source URLs, rejects archived or ≥36-month-stale repos
  (停更), and writes a decision record (选型记录). Not for implementation (use arc-code), CVE scans
  (use arc-audit), or research articles (use research-write).
---

# arc-vet

Evaluate reuse, adopt, borrow or build for a capability with dated, source-linked evidence per candidate; write only the decision record, then hand off to `arc-code`.

## Non-trigger

- Implement/pin chosen library → `arc-code`; CVE/secret scan → `arc-audit`; general research article → `research-write`; feature PRD/DAG → `arc-plan`.

## Goal

Deliver a decision record where every surviving candidate has dated URLs for fit, license, human maintenance, releases, advisories and transitive weight, every rejection cites its veto, and the chosen option specifies pin, proof test and exit plan.

## Inputs

- `need` (required); `constraints` (lang/version/platforms/project license); `candidates` (optional); `as_of` (default today); `mode` (`select|check|replace`).

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Existing dependency is stale, archived, relicensed or vulnerable (停更, 换库) | `replace` | [search-playbook.md](references/search-playbook.md) §4 |
| Judge named repositories/packages (这个库能不能用) | `check` | [search-playbook.md](references/search-playbook.md) §2–§3 |
| New capability / build vs buy without preselected repos (开源选型, 技术选型) | `select` | [search-playbook.md](references/search-playbook.md) |

## Workflow

1. Read [contract.md](../_shared/contract.md). Record need, constraints, project license, mode and `as_of` (new dependency = T2).
2. Check local reuse first per [code-intel.md](../_shared/code-intel.md): `arc idx files 'go.mod|package.json|Cargo.toml|pyproject.toml'` + `arc idx search` + target stdlib.
3. For `select`/`replace`, query ≥2 keyword families across GitHub and the ecosystem registry to gather ≥3 comparable candidates (exclude forks/demos/archives).
4. Run `python3 -I <skill-dir>/scripts/github_health.py --as-of <as_of> OWNER/REPO ...` and verify core non-bot commits, latest stable release, SPDX license (repo + registry), advisories (GHSA/OSV) and transitive deps. Save raw JSON under `.arc/artifacts/<slug>/reports/`.
5. Apply vetoes & maintenance tiers (≤12m pass; 12–24m `维护关注`; >24m `高维护风险`; ≥36 months reject by default).
6. Decide down the ladder: local/stdlib → maintained library → borrow design → small in-house build. Write record using [decision-template.md](references/decision-template.md) at ADR path or `docs/10-复盘决策/NN-选型-<主题>.md`.
7. Report per [contract.md](../_shared/contract.md) §9 and hand off to `arc-code`.

## Tool Strategy

- **Preferred**: `arc idx`, `gh search repos`, `gh api`, `scripts/github_health.py`, GHSA/OSV.
- **Forbidden**: using stars/downloads/`pushed_at`/bot commits as maintenance proof, executing candidate install scripts, copying incompatible/unknown licenses, editing source/manifests/lockfiles.

## Decision Rules

- IF stdlib or an installed dependency fits → decide reuse; search externally only if asked.
- IF archived, deprecated, incompatible/missing license (`NOASSERTION`/`OTHER`) or unfixed severe advisory → reject.
- IF domain is crypto, auth, payment or network parser → no stale exception (>24m rejects).
- IF offline/unreachable → mark external facts `未核实`; never recommend unverified libraries for production.

## Verification

| Claim | Evidence |
|---|---|
| Maintenance & license | Last non-bot core commit URL/date, latest release URL, SPDX + LICENSE URL |
| Security & rejections | Advisory query source/result; named veto rule + evidence link |
| Decision | Ladder rung, pinned version, proof test, exit plan, record path |

## Failure Recovery

| Failure | Action | Stop |
|---|---|---|
| `gh` missing or 403/429 | Run `github_health.py` / export `GITHUB_TOKEN` (1 retry) | Mark unreachable fields `未核实` |
| <3 candidates found | Add keyword family & registry source (1 retry) | Record queries and shortfall reason |
| Offline | Inspect local lockfiles/docs only | List re-run queries; no production pick |

## Completion Criteria

- [ ] Local/stdlib reuse checked with `arc idx`; ≥3 candidates compared (or shortfall documented).
- [ ] Every row has check date + source URLs (or `未核实` + reason); zero unwaived archived/≥36m picks.
- [ ] Decision record written; source/manifests untouched; next owner (`arc-code`) named.
