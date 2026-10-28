# Changelog

## Unreleased

### arc-policy — drop the OMP / PI host adapter

- Removed `hosts/omp/`, `.omp-plugin/`, and the OMP/PI `package.json` native-extension manifest. Remaining hosts are Claude Code, Codex, Grok, Cursor Agent, and DeepSeek Harness.
- Plugin-root discovery no longer reads `OMP_PLUGIN_ROOT`.

### arc-policy — tasks follow the edited file's repository

- Fix: `PreToolUse` looked up the task in the session directory's repository, not the edited file's. A Grok session in the non-Git workspace `ditie` was denied `search_replace` on `A133BMS/…` although `A133BMS` had an active task, then wrote the files through Python. The reverse leaked too: a task in the working repository allowed edits in any other repository.
- Edit gates now resolve every target (`file_path`, `path`, `notebook_path`, `target_file`, apply_patch headers) to its Git repository and need a task in each; paths outside every Git work tree are not gated, because `task begin` needs Git. Denials name the repository and give `arc-policy --cwd <repo> task begin …`.
- In a non-Git parent workspace, `SessionStart` lists the active tasks of child repositories and `Stop` checks them; Stop reminders name the repository and its `--cwd` command. The plugin skill, DSH and OMP prompts state that a denied edit must not be routed through the shell.

### arc-policy — closing the remaining bypasses

- Git-level gate: `task begin` (or `arc-policy install-git-hooks`) registers config-based `pre-commit` / `pre-push` hooks (Git 2.54+) beside existing hook files. When an agent marker is set (`CLAUDECODE`, `GROK_AGENT`, `CODEX_THREAD_ID`, `CODEX_CI`, `CURSOR_AGENT`, `PI_CODING_AGENT`, `AI_AGENT`, `ARC_POLICY_AGENT`; each confirmed in host sessions or source), a commit or push by any route needs a matching receipt. Humans are not gated.
- Stop covers every repository the session touched: `PreToolUse` records edit targets and shell-reachable repositories (directory, `cd` / `-C` targets, absolute paths in commands and script bodies) with first-touch digests under `~/.cache/arc-policy/sessions/`. Changes written through Python or the shell, or before a late `task begin`, now block a delivery claim; repositories without a task are reported. Cursor `workspace_roots` are all checked; the strictest `stop.mode` wins.
- Shell parser: heredoc bodies are data. A body such as `Don't` used to deny the whole command with "No closing quotation". Command substitutions (`$(…)`, backticks, unquoted heredoc substitutions), `sh -c` / `bash -c`, `eval`, and prefix wrappers (`timeout`, `nohup`, `sudo`, `nice`, `time`, `exec`, `xargs`) are now parsed for commits and migrations. Grok input truncated past 128 KiB is denied.
- Grok `hashline_edit` and `EditNotebook` (`target_notebook`) edits are gated; before this they went through unchecked.
- Fail closed: the hook entry denies edits and controlled operations when the engine cannot load (for example, PyYAML missing) or crashes. The OMP and DSH adapters deny when the engine cannot answer at all; until now they allowed. They also send a per-process session id and export `ARC_POLICY_AGENT=1`. `PreToolUse` timeouts are raised from 10 s to 30 s, because Claude Code, Codex and Grok ignore a hook that times out. During a 15:00–15:14 system stall on 2026-10-24, Grok skipped seven calls this way.

### arc:audit 2.6.0 — learning from Cloudflare security-audit

- Compared `arc:audit` appsec with Cloudflare's `security-audit` skill (`cloudflare/security-audit-skill` at `c1c8a8c`, MIT) and adapted its method: asset rows name the trust boundary and strongest control; Phase 4 works from a concrete invariant with sibling-path equivalence, sad inputs, variant search and a candidate gate (principal, input, control, boundary, affected resource, concrete result; no claim stronger than the evidence); off-repository facts and unsandboxed reproduction become `manual-gap`; only `confirmed`/`likely` cards carry severity, with a High/Medium discriminator; cards gain a stable fingerprint and boundary line; fixes name the invariant at the last trusted decision point; `coverage.md` becomes per-row `surface × boundary × class` with prior-run carry-forward rules; anti-patterns listed.
- New `references/attack-classes.md`: nine ordinary classes and ten boundary-selected domains (HTTP and identity, browser, data isolation and lifecycle, supply chain and CI, cloud, RPC and messaging, resource exhaustion, desktop/mobile IPC, memory safety, AI/LLM agents). Upstream MIT license kept at `references/upstream/security-audit-LICENSE`.
- Description adds threat model, source-first pen test and the surface list; health mode now screens all 41 code-rot items (was 36). `appsec-campaign` load ceiling 15,000 → 19,000 for the new reference.
- `docs/01-系统设计/04-安全审计上游学习.md` records the comparison, what was absorbed or deliberately left out (JSON findings schema and Node validators, multi-agent budgets and artifact promotion, out-of-repo output) and the procedure to re-check upstream changes.

### Installed skill cleanup

- Removed `yab-browser` (app missing; MCP server removed from user config), upstream `lark-suite` and the standalone `lark-workflow-project-docs` copy (repository `lark` router installed instead), the `open-source-vetting-kit/` source (superseded by `arc:vet`), and the disabled `typesafe@typesafe-ai` and `ponytail@ponytail` Claude plugins with their marketplaces and caches.
- Cloudflare `security-audit` withdrawn from Claude, Codex, Grok, DSH and Cursor and its local copy deleted; upstream GitHub stays the learning source for `arc:audit`, and `~/.dsh/AGENTS.md` routes explicit security audits and pen tests to `arc:audit` appsec. Retired copies `~/.agents/skills-retired-arc-*` and `~/.agents/waza-retired-*` deleted.
- `host_profiles.yaml` drops the unused Gemini and Pi hosts; `link_hosts.py --check` is OK for Claude (19), Codex (21) and Grok (9). Skill backups (`~/.agents/skills-backup-*`, `~/.skills-backup/`) and the stale Claude `arc-policy` cache deleted; README states backups are cleaned manually.

### arc:vet 1.0.0 — open-source due diligence

- New sixth Arc skill `Arc/arc:vet/` (installed as `arc-vet`), rewritten from the standalone `open-source-vetting-kit` into the Arc structure: select · check · replace modes; local reuse first via `arc-idx`; at least three comparable candidates or a stated shortfall; dated, linked evidence per candidate; maintenance tiers (12 / 24 / 36 months) with vetoes for archived, deprecated, unlicensed and unfixed-advisory projects; an evidenced stale exception that never applies to cryptography, authentication, payment or network-facing parsers; a decision ladder (reuse → adopt → borrow the design → build); records at the project ADR location or `docs/10-复盘决策/`; handoff to `arc:code` in the same turn when implementation is authorized.
- `references/search-playbook.md` (keyword families, `gh api` queries, manual confirmation, successors, supply-chain hygiene) and `references/decision-template.md`.
- `scripts/github_health.py`: standard-library GitHub precheck with `--as-of`, `GITHUB_TOKEN`/`GH_TOKEN`, bot-commit exclusion, maintenance tier, deprecation hint, license and release warnings; a pure `assess()` covered offline by `tests/test_arc_vet.py`.
- Contract §1 authority row and §2 rule (a new third-party dependency needs an `arc:vet` record first); workflow owner row and `vet → code → verify` path; `LIFECYCLE_PHASES` gains `select`; new `vet-select` load ceiling (9,000); trigger, route and behavior corpora, host profiles (core, Grok) and registry updated.
- Remove `apifox-cli`: host profile external target and Codex extra, route corpus roster and case, skill catalog row; Arc API-contract, task-docs, docs-layout and task-template text no longer name Apifox (the rule stays: no API-platform sync unless the user asks; the Markdown contract is the authority).
- arc-policy: `arc-vet` accepted by `task begin` (alias `open-source-vetting`), listed in deny hints, the DSH prompt and the plugin skill, and exempt from the stack-skill architecture check like the other non-coding phases.

### arc-policy 0.1.0 — Coding Agent Policy Runtime

- New plugin at `plugins/arc-policy/`: portable `plugin.json`, Claude `.claude-plugin/plugin.json`, Codex `.codex-plugin/plugin.json`, Grok/Claude/Codex marketplaces.
- Policy engine (`lib/arc_policy`) plus CLI `bin/arc-policy`: `task begin|status|verify|end`, `detect`, `doctor`, `hook`.
- Default mode is free source edits after `task begin`; `git commit`/`push`, `--no-verify`, and schema-migration CLIs need a tree-bound receipt from `task verify`.
- Host hooks: `SessionStart` injects stack and skill, `PreToolUse` denies, `Stop` reminds or blocks delivery claims. Same-user hooks are documented as process control, not a security boundary.
- Host adapters for OMP (`hosts/omp/arc-policy.ts` + `.omp-plugin/`), Cursor Agent (`hosts/cursor/hooks.json`, Cursor `permission` replies), and DeepSeek Harness (`hosts/dsh` Cordis plugin). Hook JSON dual-emits Claude `permissionDecision` and Cursor `permission`.

### arc:verify 3.2.0 — properties and model conformance

- New `references/model-conformance.md` and Intent Router row (write mode): pick the cheapest disproving method (property, exhaustive enumeration, stateful model, conformance, linearizability, bounded proof); property shapes derived from contract and closure-spec invariants with their traps; generator, shrinking, seed and replay rules per cadence; property and state-machine APIs for rapid, fast-check, Hypothesis, proptest/proptest-state-machine and jqwik.
- Closure spec → tests: states, transitions, guards, `ignores`, `race_policy`, resources, invariants and timers each map to a command or assertion; the model gate must pass first, a transition hit ledger replaces iteration counts, and divergences split into `FAIL` (to `arc:code`), `SPECIFICATION GAP` (to `arc:plan`) or adapter defects. Existing TLA+ (trace validation), Quint (ITF traces) and Alloy (instances as fixtures) specs feed the same adapter; model-check and code-conformance verdicts stay separate.
- Concurrent conformance via linearizability checking (Porcupine `Unknown` is `BLOCKED`); Kani bounded proofs for Rust with unwinding failures `BLOCKED`; Verus/Dafny stay with `arc:code` because they change production source.
- `case-design.md` §4, `flow-model.md` §6 and `stacks.md` link the new reference; Go `testing/quick` is marked frozen and non-shrinking. Description gains 属性测试 and 模型一致性 (trigger corpus updated); new behavior scenario `model-divergence-is-not-a-model-edit`.
- SKILL.md keeps the cross-mode procedure and leaves mode detail to the mode references (3,496 → 3,345 tokens; `pg-reliability` load 15,498 → 15,347): the release waiver rule and GO/NO-GO, flaky and fuzz evidence rows defer to release-ops §2–§3 and fuzz-campaign §8, which already state them; the gap-terminal rule defers to flow-model §3; the ledger-failure recovery row and flow evidence move into the flow-model §1 Check row; the PostgreSQL routing rule folds into the db router row; the browser fallback defers to web-e2e and stacks. A "Mode evidence" Verification row points to each mode's verdict section.

### arc:verify 3.1.0 — business-flow-first testing

- New flow mode: T1/T2 test work starts from the business flows a change reaches. `references/flow-model.md` (catalog at `docs/07-测试质量/业务流/`, discovery from real entry points, Mermaid chart subset with `Nxx` decisions, `Bxx` branches and `Txx` terminals, closure kinds 正常/拒绝/失败/恢复/异步/缺口, flow card, mapping to closure-spec IDs) and `references/flow-matrix.md` (risk ranking, A–H dimensions, branch → case, pre/post state diff, branch ledger, layers, change impact, gap routing).
- `scripts/flow_ledger.py check <catalog>` fails on unlinked or missing flow files, charts with unreachable or dead-end nodes, decisions with fewer than two branches or bare yes/no conditions, ledger rows that differ from the chart, illegal verdicts, and verdicts without reason or evidence; warns on gap terminals and Critical/High branches without PASS. `tests/test_flow_ledger.py` (17 cases, 12 structural mutations).
- Workflow step 2 is "flows first"; generating T1/T2 cases from an endpoint, function or file list is forbidden; unimplemented recovery is charted as a gap terminal. Contract and workflow give verify ownership of the flow catalog; arc:plan closure-design and docs-layout point to it. New `verify-flow` load ceiling (14,000).
- ARC's own toolchain is the first catalog: 16 flows inventoried, 5 modeled (install, host links, closure gate, completion gate, database release gate) with 83 ledger branches. It records a confirmed defect: `dbqual/scripts/release_gate.py` returns PASS with exit 0 when the policy declares no required evidence or metric rule (empty or misspelled key), while `canary_evaluator.py` treats the same case as BLOCKED.
- Fix dbqual gates that check nothing: `release_gate.py` validates the policy against `schemas/gate-policy.schema.json` and fails (`invalid_policy`, `empty_policy`, exit 2) on unknown sections or selector fields, non-list sections and policies without any assertion; before, `{}` or a misspelled section passed with exit 0 and an object-shaped section crashed. `qualification_coverage_gate.py` fails a manifest without claims. db.md maps `empty_policy` to Arc `BLOCKED`.
- `sync_skills.py`: a rollback that fails itself now raises an error naming the backup that still holds the previous entries. `link_hosts.py --apply` stops at the first OSError and prints the entry and backup path instead of a traceback.
- New tests: `tests/test_link_hosts.py` (5), overlap/empty-selection/dry-run/argument cases in `tests/test_sync_skills.py`, red-without-baseline, vanished baseline head and exit-127 cases in `tests/test_gate_script.py`, unusable-input and render cases in the closure selftest. Each characterization test was mutation-checked in a disposable copy. All 83 ledger branches of the 5 modeled flows now PASS.

### arc:verify 3.0.0, arc:code 2.6.0 — business-expectation testing and fail-closed evidence

- Fix two false `PASS` paths in the bundled dbqual package (reproduced offline, before `PASS` → after `FAIL`/`BLOCKED`): `run_suite.py` archives earlier evidence and writes `run-manifest.json` (run id, HEAD, changed-path and config digests); every record carries `run_id`; `release_gate.py --root` re-checks the candidate fingerprint and rejects other-run, unreadable or duplicate evidence; `regression_compare.py` is `BLOCKED` on a missing baseline or manifest, missing environment snapshot, absent workload or zero effective comparisons. Siblings closed: coverage gate no longer skips unreadable files, baseline promotion only takes the gated run, canary with no metrics is `BLOCKED`, pgbench parses only the current invocation's logs. 61/61 package tests, selftest with negative controls.
- `arc:verify` states its purpose up front (tests prove business expectations; green is not the goal) and adds red mode (acceptance tests before code), test-value review (`references/test-value.md`), browser E2E with `scripts/with_server.py` and recon-before-act (`references/web-e2e.md`), and hostile-input fuzz campaigns (`references/fuzz-campaign.md`). Reliability minimums move to `txn-faults.md` §0.
- SQL verification: new `sql_static_audit.py` (evidence `sql.static`) parses and lints SQL with SQLFluff in the configured dialect and lints migrations with Squawk; parse errors, gated rules and unexcluded Squawk findings fail, nothing checked is `BLOCKED`. `gate.sh check` gains `lint(sql)` for changed `.sql` files under the project's sqlfluff config (and Squawk with `.squawk.toml`). New `references/sql-static.md` (what parse, lint, schema-bound EXPLAIN and migration lint each prove) and `references/txn-design.md` (transaction boundaries, forbidden anomaly → mechanism, per-engine isolation semantics for PostgreSQL, InnoDB, SQL Server, Oracle, CockroachDB, Spanner, SQLite; retry ownership). Verified against real sqlfluff 4.4.0 and squawk 2.68.0.
- `arc:code` adopts the red-before-code iron law (`Arc/_shared/test-first.md`): `gate.sh red -- <cmd>` refuses when production files changed before the first assertion-level red or when the command passes; `gate.sh check --require-red` re-runs every red and warns on oversized stubs and tests edited after their red.

### T08 full-catalog consolidation — Arc 2.5.0 (breaking)

- Collapse 162 installed skill directories (204 on-disk copies) into 22 entrypoints: Arc 5, check, health, research-write, routers `lark`, `android`, `ui-design`, `cloudflare`, `threejs`, merged `cnb`, `harmonyos`, `lazycat`, and six external tools. Design and probe data: `docs/01-系统设计/02-全量技能架构评审.md`.
- Merge `arc:closure` into `arc:plan` model mode; closure docs move to `docs/01-系统设计/业务闭环/` with legacy `docs/arc/` kept; checker under `arc:plan/scripts/closure/`.
- Remove the body Trigger section from Arc skills; demote `workflow.md` to handoff-only; split `coding-discipline.md` out of `execution-truth.md` (§1–§4, §6–§7, §11 keep their numbers) and `go-backend-structure.md` out of `go-backend.md`; add `ARC_LOAD_SCENARIOS` ceilings (Go repair 18.1K → 15.1K, full-stack API 22.3K → 17.5K estimated tokens).
- Add code-rot family G (over-engineering, from ponytail) and `arc:verify/references/web-perf.md`; arc:code consumes `DESIGN.md` and routes semantic-judgment features to TypeSafe.
- Waza: read, learn, write and AnySearch become `research-write` (fetch/search/research/edit); `check` covers branches and PRs on any host, absorbing cnb-code-commit's safe-branch steps.
- Packager: `metadata.library` routers bundle sources as `library/<name>/ENTRY.md`, refuse standalone installs of members, reject nested `SKILL.md`, skip `node_modules`, and tolerate directory links. Fix cnb-pipeline's validator and references that were never packaged.
- Add `scripts/link_hosts.py` + `schemas/host_profiles.yaml` (host install governance) and `scripts/route_probe.py` + `schemas/route_corpus.yaml` (63-case cross-family routing regression via TypeSafe Jev; 63/63 passing).
- Rewrite global instructions to current names (6.6K → 2.4K tokens); retire fixtures, stubs, ponytail, stale copies and drift copies to `~/.skills-backup/2026-10-08-T08/`.

### Earlier unreleased

### Arc 2.4.0 native client direction

- Replace Expo/mobile, Tauri/desktop and Taro/mini-program defaults with a single target-native policy shared by planning, implementation, audit and verification; scope Web state, theme, API and scaffold rules to Web.
- Preserve scoped maintenance of existing cross-platform products; require explicit migration scope. Add native Apple/desktop/mini-program verification guidance, retain native Android/HarmonyOS tools, and remove ArkUI-X as a default direction.

### Arc 2.3.0 API design review

- Extend arc:audit with api-design mode for contracts and implementations; verify primary standards online and distinguish design defects, implementation defects, specification gaps and evidence gaps.
- Share sourced HTTP, JSON field and upload rules with planning, implementation and verification. Record units, provenance, enforcement and failure behavior for limits; keep protocol requirements, security guidance and project choices distinct.
- Add API boundary evidence guidance and behavior scenarios without creating another skill, scanner or arbitrary global limits. Plan/code/audit/verify move to 2.3.0; explore/closure retain 2.1.0.

### Waza selection and Arc 2.2.0 references

- Review Waza at `d8fa34ff50a37043d40bbd3b9e622029a12dd37b`; manage adapted read/write/learn/check/health sources in `Waza/` with MIT license closure. Keep check for authorized delivery and health for agent configuration; omit duplicated source scanners and implicit conversation collection.
- Merge think/hunt/ui increments into Arc planning, regression evidence and screenshot iteration. Retain the existing Arc authority, root-cause and runtime evidence gates; retire the three competing installed entries with backups.
- Connect explicit read network proxy configuration to the local extractor without enabling third-party extraction. Preserve Gollum wikilinks and add an explicit Obsidian dialect to punctuation checks. Bundle read/write when installing learn.

### Arc 2.1.0 — business closure and shared handoffs

- Add `arc:closure` from `Arc/arc-closure`, with design/change/audit/gate modes and complete packaged assets and scripts.
- Connect all six skills through one handoff contract: preserve requirement/model IDs, source facts, task status, evidence freshness and existing authorization; route gaps back to their owner without recursive skill loading.
- Keep read-only closure audits in artifacts; distinguish finite-model checks, business decisions, implementation and runtime qualification. Generic planning and cosmetic edits do not require a closure model.
- Close warning/waiver gate bypasses; bind model evidence to current spec and checker fingerprints and require complete enumeration. Provide explicit checker-copy refresh without overwriting existing project documents.
- Extend lifecycle/trigger/behavior corpora and test real installed closure initialization, checking and gating under dash and colon names. Install the six-skill family through the existing atomic backup/rollback packager.

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
