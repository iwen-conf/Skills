# Changelog

## Unreleased

### Arc 2.0.0 — architecture consolidation (breaking)

Design record: [ARC 技能架构重构评审](docs/01-系统设计/01-ARC技能架构重构评审.md).

- Replace the 16 Arc skills with 5, one per kind of work: `arc:explore` (from `arc:idx`), `arc:plan` (`arc:define` + `arc:clarify` + `arc:sdlc`), `arc:code` (`arc:build` + `arc:fix` + `arc:frontend` + `arc:arch` + `arc:comment` + `arc:trace`), `arc:verify` (`arc:test` + `arc:reliability` + `reliability-engineering-skill` + the external `arc:dbqual` package), and `arc:audit` (`arc:audit` + `arc:security`, plus a new diff/PR review mode). No aliases; the old → new map is in `Arc/README.md`, and the retired skills' trigger utterances stay in the corpus under their new owner.
- Move `arc:docs` to the Lark family as `lark-workflow-project-docs`; the `.lark.json` format is unchanged. Drop the thesis evidence checklist.
- Remove `arc:prewalk` (requires mid-session model swap that hosts do not provide); its design record moves to `docs/01-系统设计/prewalk.md`. Remove the external `ponytail` preflight; the simplicity intent is covered by `execution-truth.md` §9.
- Add the undiscoverable shared layer `Arc/_shared/`: `contract.md` (authority, T0–T2 tiers, scope lock, one vocabulary for verdicts/status/severity with a single severity → P0–P3 mapping, artifacts, environment, subagent delegation, Lark hook, commit format, 5-line report), `code-intel.md` (arc-idx preflight, query ladder, new 8-surface impact checklist, zero-hit widening, major-version-only check), `code-rot.md`, `api-contract.md`, `finding-campaign.md`, and `conventions/` (Go backend, frontend, color tokens, comments).
- Import the PostgreSQL release-qualification package as `arc:verify` db mode (`scripts/dbqual/`, gates G0–G31). Its 1,109-line guide is split into `references/db.md` and `references/db/*.md`; its Chinese README moves to `docs/dbqual/`. The package's unit suite runs in the repository test suite.
- Replace the six unreferenced `Arc/scripts/*.sh` with `arc:code/scripts/gate.sh`: baseline-aware completion gate that scans only lines changed since `gate.sh start`, runs non-mutating format checks and lint/typecheck for every detected stack, affected-package tests, and a scope check.
- Every Arc `SKILL.md` uses the fixed headings Trigger → Non-trigger → Goal → Inputs → Intent Router → Workflow → Tool Strategy → Decision Rules → Verification → Failure Recovery → Completion Criteria, stays within 3,500 estimated tokens, and names adjacent skills in a "Not for" clause. The validator enforces order, non-empty sections, token budgets for references and `_shared/`, retired-name references, nested `SKILL.md`, and Arc directory layout; vague wording is a warning. The root-cause gate applies to `arc:code`, `arc:verify` and `arc:audit`.
- Delete `Arc/templates/`, `docs/arc-routing-matrix.md`, `docs/arc-routing-cheatsheet.md`, `docs/conductor-pattern.md` and `docs/orchestration-contract.md`; `Arc/README.md` holds the single routing table. Move `docs/code-rot-taxonomy.md` to `Arc/_shared/code-rot.md` and archive `reliability-engineering-skill/` under `docs/archive/`.
- `gate.sh` behavior versus the old scripts: the file-count check warns above 20 files (was FAIL above 5) and fails only when `--scope` is given; formatters run in check mode only (`gofmt -l`, never `go fmt ./...`); whole-project lint (`npm run lint`, `ruff check .`, `staticcheck`) is replaced by typecheck of changed packages; a missing test command is `NOT_RUN`, not a failure; the destructive-command blocklist moves to contract §6 approval.
- `arc:audit` scan script: semgrep uses `--config p/default` (`auto` with `--metrics off` always exited 2, so semgrep never ran); schemathesis 4 uses `--url`; gitleaks uses `gitleaks git`. Default output moves from `.arc/security/<timestamp>/` to `.arc/artifacts/<task-slug>/security/` (`--task-slug`; a rerun with the same slug overwrites).
- `code-intel.md` query templates corrected against `arc-idx` 1.0.0: Go method callers use a text query (`ast` cannot match Go selector calls), JSON-tag and field-write regexes fixed, `--max` truncation and per-command zero-hit output documented. `arc-idx --help` now shows the real `ast` argument order and `--format` for `files`, `stats` and `doctor`.
- Frontend conventions updated to current toolchains: Tailwind v4 with `shadcn@latest init -t vite -b radix`, `sonner`/`field` components, NativeWind pinned to Tailwind 3, Taro CLI pinned to `@4`.

- Apply the 2026-10-05 full-audit disposition to all 80 entries: correct CNB base/partial-review contracts, R8 execution and missing-data failure, disabled Compose clicks, server-verified authentication, JSON stage logs, nested calendar conflicts and event completion semantics. Preserve the eight entries for which the audit recommends no substantive change.
- Consolidate default stack, workspace, Android execution, CNB runtime and Lark auth/authorization contracts; use scoped workflows, finite failure exits and references for optional detail. `arc:docs`, `arc:frontend`, `arc:prewalk` and `arc:trace` receive minor contract versions; other changed Arc entries receive patch versions. No runtime installation or publication is performed by this revision.
- Validate the standalone reliability source distribution separately: 80 checked entries, 79 default runtime packages. Add executable regression cases for the audit counterexamples and repair transitive packaging links; static checks and local examples do not claim remote/API or agent end-to-end verification.

- Add `arc:reliability` 1.1.0, adapted from the supplied reliability-engineering skill and 13 primary industry/expert sources: risk-based scope, independent business oracles, real transaction/fault/recovery evidence, mixed-version and rollback validation, and incident controls. Include source limits and a worked inventory/payment-notification example; reuse native execution and remote infrastructure without a source-runner dependency. Connect build/fix/audit/sdlc routing and lifecycle/root-cause registration.
- Upgrade `arc:test` to 1.2.0 with risk-based case design, explicit untested scope, generator/oracle review and bounded defect-sensitivity checks. Remove generic coverage/E2E percentages and test-type priority defaults; retain project-owned gates, independent functional/performance verdicts and required claims when flaky tests are quarantined. Add trigger and runtime scenarios for these decisions.
- `scripts/sync_skills.py` now stages a complete package (file links plus declared `metadata.requires.skills`), validates it, then replaces one host name (`--naming dash|colon`) and archives the other alias. Missing resources or publish failures leave the destination unchanged and restore from the backup. Shared docs are generated into `references/_shared/`.
- Keyword trigger tests are labeled as a static vocabulary gate. Runtime negation, multi-intent, scope, and authorization scenarios live in `schemas/behavior_corpus.yaml` and are not solved by hard-coding sentences into the scorer.
- Task tiering has a single authority in `arc:sdlc`: ordinary multi-step work stays on an in-session plan; only large / cross-session / tracked work creates persistent docs. Search fallback is `arc-idx` first; `rg`/`fd` only when the index cannot cover the target.
- Import CLI-embedded Lark meeting skills with `scripts/import_lark_skills.py`. Compatibility entries `lark-minutes` / `lark-note` / `lark-vc` / `lark-vc-agent` route to `lark-meeting`.
- Make repository Markdown API contracts the implementation and acceptance baseline; replace the mandatory Apifox source rule with explicit-request-only downstream sync. Add field/behavior/verification templates and connect `arc:build` 1.2.0, `arc:arch` 1.2.0, and `arc:frontend` 1.1.0 to the shared contract.
- `arc:sdlc` 1.2.0 requires Markdown task docs with business-group → separate frontend/backend work-package → executable-leaf hierarchy. Assess D1–D4 difficulty separately from P0–P4 priority, record reasons and dependency readiness, split complex work by acceptance boundaries, and isolate high-uncertainty exploration. Update task/progress/audit templates and status rules, including mandatory `<Context-Queries>` and independent frontend/backend verification.
- `arc-idx version` now prints the full identity line (`arc-idx 1.0.0 — local AI context engine (zoekt + ctags + ast-grep)`) instead of the bare semver, so AGENTS/CLAUDE identity checks can compare one command's output; `arc:idx` 1.1.0 makes the identity check, per-session `daemon status`/`start`, and the no-tree-walk red line explicit, and syncs the repo source with the stricter installed copy.
- Add cross-skill coding-discipline gates to `docs/execution-truth.md`: think-before-coding (state assumptions, present competing interpretations and simpler alternatives, ask/autonomy boundary, verifiable success criteria first), the simplicity ladder (skip > reuse > stdlib > platform-native > installed dependency > one line > minimal code, with `debt:` ceiling comments and never-simplify boundaries), surgical edits (no adjacent "improvements", orphan-only cleanup, per-line traceability, bounded elegance pass), and the self-improvement lessons loop (`tasks/lessons.md` + persistent memory).
- Wire the new gates into `arc:build`, `arc:fix` (autonomous evidence-driven repair, no user hand-holding), `arc:clarify` (present interpretations, ask/autonomy boundary), `arc:arch` (ladder vs required DIP boundaries), and add explicit task tiering (trivial / in-session bounded todo / persistent docs) plus re-plan-on-drift to `arc:sdlc`; bump the five skills to 1.1.0.
- Add a subagent usage strategy to `docs/orchestration-contract.md`: delegate read-heavy research/exploration to keep the main context clean, one subagent per task, no silent swallowing of failed/partial results.
- Add one evidence-first root-cause repair gate: confirm the issue before changing production behavior, allow only bounded evidence instrumentation while unconfirmed, trace the owning architecture and shared invariants, compare the real business contract, implement the smallest complete fix, and verify analogous paths.
- Require repair-capable Arc and vertical entrypoints to load that gate; preserve multi-inline-code router values during index generation.
- Enforce the 500-line progressive-disclosure budget and relative-link checks for bundled Android, HarmonyOS, CNB, and Lark entrypoints without forcing Arc frontmatter on them.
- Split the oversized HarmonyOS and Android Intent Security entrypoints into concise routers while preserving their complete guides under `references/`.
- Keep `skills.index.json.generated_at` stable when generated content has not changed, avoiding timestamp-only worktree and commit noise.

## 1.0.0 — 2026-08-17

Breaking Skill engineering upgrade. No historical compatibility.

- Treat every `SKILL.md` as a progressive-disclosure router: required `## Intent Router`, `## Red Lines`, `## When to Use`; body ≤ 500 lines.
- Descriptions are WHAT+WHEN, third person, 80–1024 characters, with mandatory trigger terms from `schemas/trigger_corpus.yaml`.
- Drop `enforce_arc_profile`, `expert_keywords`, and heading-laundry validation. Split oversized skills (`arc:sdlc`, `arc:arch`, `arc:docs`, `arc:comment`) into modules.
- Discovery index no longer embeds full section bodies. Automated trigger tests cover ≥8 positive and ≥3 negative utterances per skill.
- Authoring contract: [`docs/skill-engineering.md`](docs/skill-engineering.md).

## 0.2.0 — 2026-08-10

### Publish channel

- Add `scripts/sync_skills.py` to install SSOT `Arc/arc:*` into `~/.agents/skills` as both colon and dash aliases.
- Set `version: 0.2.0` on all Arc `SKILL.md` frontmatter.
- Document validate → index → sync flow in `README.md`.

### Content (anti-drift)

- Add [`docs/execution-truth.md`](docs/execution-truth.md): environment/branch/deploy truth, completion definition, scope lock, domain keys, paper-over bans, cheap-executor task authoring.
- `arc:sdlc`: completion definition, explicit non-goals, downstream task template gates.
- `arc:build` / `arc:fix`: environment surface preflight, scope lock, no matrix-only done claims.
- `arc:arch`: common drift patterns (domain services dump, temp var flags, type asserts, invented identity keys).
