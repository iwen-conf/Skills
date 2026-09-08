# Changelog

## Unreleased

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
