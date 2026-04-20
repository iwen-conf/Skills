# Task Todo

## 2026-04-18 graduation-doc-support generic skill

- [x] Confirm the correct placement and allowlist path for a new non-`arc:*` skill that captures the thesis-support-doc workflow.
- [x] Add a reusable root-level generic skill for evidence-driven graduation thesis support documents.
- [x] Wire the new skill into repository validation, discovery, and README guidance.
- [x] Rebuild generated registry artifacts and run repository validation/tests.

## Review

- Added root skill `graduation-doc-support/` with a generic-skill contract for evidence-driven graduation thesis support docs.
- Added `graduation-doc-support` to the generic allowlist in `src/arc_core/skill_validation.py`, so the registry will index it.
- Updated `README.md` to document the new root-level generic skill alongside `terminal-table-output`.
- Updated registry tests to assert the new skill is discovered and emitted into `skills.index.json`.
- Rebuilt generated registry artifacts: `skills.index.json`, `.arc/context-hub/index.json`, `.arc/registry/manifest.json`.
- Verification results:
  - `uv run pytest tests/test_skill_registry.py tests/test_skill_validation.py -q` -> pass (`19 passed`)
  - `PYTHONPATH=src uv run python - <<'PY' ... validate_file(graduation-doc-support/SKILL.md) ...` -> pass (`errors=[]`, `warnings=[]`)
  - `uv run python scripts/build_skills_index.py` -> pass
  - `uv run python scripts/validate_skills.py` -> fail, but on pre-existing repository issue: `Arc/arc:brand-brief/SKILL.md` missing required expert keyword `project architecture`, not caused by this task

## 2026-04-15 arc:uml layout hardening

- [x] Audit why sequence and deployment diagrams produced by AI drift away from draw.io shapes and connectors.
- [x] Tighten the `arc:uml` skill contract so sequence diagrams no longer rely on raw draw.io XML arrows and deployment diagrams carry explicit geometry rules.
- [x] Update the UML scaffold so sequence sources land in `diagrams/sequence.mmd` and deployment briefs carry anti-misalignment guidance.
- [x] Add deployment-specific drawio validation for floating edges, manual endpoint coordinates, and off-grid node layout.
- [x] Add regression tests and run validation / smoke coverage for the new diagram workflow.

## Review

- Sequence diagrams now default to `Mermaid` source under `diagrams/sequence.mmd`, which removes the highest-risk failure mode: LLM-generated raw draw.io message arrows with broken coordinates.
- Deployment diagrams now have an explicit layout contract instead of only semantic hints. The skill, notation reference, scaffold, and validator all agree on node alignment and connector rules.
- New automated validation blocks the most damaging deployment failure directly: connectors that do not actually attach to nodes.
- The workflow no longer stops at validation. `generate_deployment_drawio.py` now turns structured deployment specs into aligned draw.io output, and `render_beautiful_mermaid_svg.mjs` is back in the source tree so `sequence.mmd` can render into a clean deliverable SVG.
- The workflow now also reduces handwork: `draft_deployment_spec.py` converts a compact seed text into `deployment.json`, and `generate_sequence_drawio.py` wraps `sequence.mmd` into a stable `sequence.drawio` that still validates against embedded Mermaid source.
- `review_uml_pack.py` now adds a true repeat-check loop: it validates existing sources, regenerates temporary artifacts, reruns validation on regenerated outputs, and checks workspace artifacts for drift before the task can be considered finished.

## 2026-03-15 arc:init baseline rebuild

- [x] Confirm `arc:init` should be executed as a local skill workflow, not inferred from shell PATH alone.
- [x] Re-check the `server/.arc` baseline and route to `mode=update`.
- [x] Rebuild `.arc/init/context/module-fingerprints.json` for the current `apps/` + `tools/` topology.
- [x] Refresh `.arc/context-hub/index.json` to point at the regenerated baseline.
- [x] Replace the stale fallback wording in `.arc/init/context/update-report.md`.
- [x] Verify JSON integrity with `jq`.
- [x] Verify the resulting file set with `git status` and `git diff`.

## Review

- The fix stayed inside `server/.arc/` and did not touch business code.
- The old `cloud/` and `sdk/` entries were removed from the active baseline because they no longer match the current git-backed topology of `server/`.
- The remaining out-of-scope repository is the standalone SDK, which still requires its own `arc:init` run if indexing is needed.

## 2026-03-15 arc skill load diagnosis

- [x] Check whether Arc skills explicitly invoke Python-based helper scripts.
- [x] Check whether current Python activity looks like one-shot scripts or lingering background processes.
- [x] Confirm that the visible resident Python processes are mainly tool-layer `cocoindex-code` services rather than `arc:init` itself.
- [x] Tighten `arc:init` and `arc:cartography` guidance to prefer CLI-first execution and forbid unnecessary background helpers.

## Review

- The heaviest Arc-side risk is repository-scale scanning, especially codemap rebuilds, not an obvious `while True` loop in the shipped helper scripts.
- The current resident Python processes are mostly `cocoindex-code` index/search workers, which are tool-layer services and may stay alive across tasks.

## 2026-03-15 arc:serve skill creation

- [x] Confirm the new skill boundary, naming, and tmux command surface.
- [x] Create `Arc/arc:serve/` with a tmux-first `SKILL.md`.
- [x] Add a lightweight session-registry script that writes `<project_path>/.arc/serve/tmux-sessions.json`.
- [x] Enforce restart-before-start semantics so the same service session is never duplicated.
- [x] Wire `arc:serve` into routing/validation/index generation.
- [x] Verify shell syntax, rebuild `skills.index.json`, and exercise `start → restart → stop` against a temporary tmux session.

## Review

- `arc:serve` now routes local frontend/backend/dev-service start-restart-stop requests into a deterministic `tmux` session contract instead of raw foreground processes.
- The runtime implementation stays on `tmux + jq + shell`; no Python helper was introduced into the steady-state serving path.
- Real `tmux` smoke validation exposed a `jq` reconciliation bug in `status/restart/stop`; it was fixed before completion, and the final `start → status → restart → stop` flow passed with registry updates preserved in `.arc/serve/tmux-sessions.json`.
- The skill is now visible to validation, routing docs, README guidance, and the generated `skills.index.json` / `.arc/context-hub/index.json` artifacts.

## 2026-03-15 arc:serve Go runtime migration

- [x] Replace the `jq + shell` control script with a one-shot Go CLI for tmux session orchestration.
- [x] Preserve the existing registry contract at `<project_root>/.arc/serve/tmux-sessions.json`.
- [x] Keep `start/restart/stop/status/cleanup` semantics and duplicate-session protection unchanged.
- [x] Update the skill instructions to point at the Go entrypoint instead of the removed shell script.
- [x] Re-run Go formatting, skill validation, registry rebuild, and tmux smoke coverage.

## Review

- `arc:serve` now executes through a short-lived Go control process, which exits after reconciling tmux and the JSON registry.
- The steady-state path no longer depends on `jq` or shell state mutation for registry writes; tmux remains the only long-lived runtime wrapper.
- The original no-Python runtime guarantee still holds, and the control surface is now closer to the user's preferred "run and destroy" model.

## 2026-03-15 arc:serve cached launcher

- [x] Add a thin launcher that caches the compiled Go controller instead of relying on `go run` for every invocation.
- [x] Keep the launcher stateless apart from the build cache under `${XDG_CACHE_HOME:-$HOME/.cache}/arc:serve`.
- [x] Update skill instructions and controller usage text to point at the launcher entrypoint.
- [x] Verify the launcher executes the existing Go controller and preserves tmux/registry semantics.

## Review

- `arc:serve` now avoids repetitive `go run` compile overhead while still using a short-lived Go controller for each action.
- The launcher only builds when the controller source hash changes, then `exec`s the cached binary directly on later runs.

## 2026-03-15 remove vendor-specific CI references

- [x] Confirm the repository does not contain actual vendor CI workflow files.
- [x] Locate source files that still mention vendor-specific CI paths or runner assumptions.
- [x] Remove vendor-specific CI examples from skill docs and reference material.
- [x] Rewrite report generator text to use generic CI wording instead of host-specific paths and permissions.
- [x] Rebuild generated indexes and audit artifacts.
- [x] Verify there are no remaining vendor-specific CI references in tracked repository files.

## Review

- No real vendor-specific workflow files existed in this repository; the cleanup scope is residual documentation, reference heuristics, and generated report text.
- The cleanup keeps generic CI guidance where it still adds value, but removes host-specific filenames, actions, and permission assumptions.

## 2026-03-18 arc:uml beautiful-mermaid SVG rendering

- [x] Confirm the actual `beautiful-mermaid` package API and renderer coverage.
- [x] Add a local `arc:uml` render script that turns Mermaid `.mmd` files into `.svg`.
- [x] Update the `arc:uml` skill contract so rendered SVG becomes the default delivery.
- [x] Rebuild generated registry artifacts and verify the new render path with a smoke run.

## Review

- `arc:uml` now defaults to Mermaid source plus SVG delivery, with `beautiful-mermaid` called out as the standard renderer and `timeline`-style unsupported headers explicitly marked `svg-skipped`.
- The new `render_beautiful_mermaid_svg.mjs` script caches `beautiful-mermaid@1.1.3` under the local user cache on first run, then renders single files or whole diagram directories without adding a repo-level Node dependency.
- Validation passed through `uv run --group dev python scripts/validate_skills.py`, the skill registry artifacts were rebuilt, and a smoke run rendered valid `classDiagram` + `sequenceDiagram` samples while correctly skipping a `timeline` sample.

## 2026-03-25 terminal-table-output fusion skill

- [x] Confirm how the repository recognizes non-`arc:*` fusion/generic skills.
- [x] Add a reusable `terminal-table-output` skill for compact chat tables with box-drawing borders.
- [x] Wire the generic skill into validation, discovery, and registry generation.
- [x] Document the output-paradigm composition rule in the shared orchestration/fusion guidance.
- [x] Reference the new output paradigm from key table-heavy Arc skills.
- [x] Expand the explicit composition hint to more output-heavy skills (`arc:e2e`, `arc:fix`, `arc:test`, `arc:ip-check`).
- [x] Run skill validation, targeted tests, and rebuild generated registry artifacts.
- [x] Add a repository-level guard that rejects `.github/workflows/` so GitHub Actions cannot be reintroduced silently.
- [x] Verify the remote repository has Actions disabled and purge historical workflow runs from GitHub.

## Review

- `terminal-table-output` is introduced as a fusion-style generic skill for chat presentation only; it does not replace Markdown/JSON/text artifacts written to disk by existing Arc workflows.
- Shared guidance is updated so Arc skills can compose this output profile when a final answer is naturally a compact two-dimensional summary.
- The explicit composition hint now also covers evidence-heavy delivery skills where final chat output frequently collapses to short PASS/FAIL tables, risk matrices, or generated-file inventories.
- `uv run python scripts/validate_skills.py` passed for 19 indexed skills, `uv run pytest tests/test_skill_validation.py tests/test_skill_registry.py -q` passed with 18 tests, and `uv run python scripts/build_skills_index.py` refreshed the generated registry artifacts.
- `validate_skills.py` now also enforces a repository policy: `.github/workflows/` is forbidden in this Skills repository, so accidental GitHub Actions reintroduction fails validation immediately.
- Remote GitHub state was also cleaned up: the repository Actions permission is `enabled=false`, there is only one branch (`main`), and historical workflow runs were deleted until the Actions runs count returned `0`.

## 2026-04-10 arc:uml drawio hard switch

- [x] Replace the repository `arc:uml` skill contract so formal diagram delivery must call the `drawio` skill and no longer defaults to Mermaid.
- [x] Add or refresh `arc:uml` reference material for China university-style UML / Chen E-R review points, including activity diagrams.
- [x] Replace the UML scaffold script so it produces `.drawio` skeletons and modeling briefs instead of Mermaid `.mmd` files.
- [x] Rebuild generated skill index artifacts and run repository validation.
- [x] Verify the new scaffold output paths and file names match the updated skill contract.

## Review

- `Arc/arc:uml/SKILL.md` now makes `drawio` the mandatory formal delivery path and explicitly links `notation-standards.md`, `diagram-catalog.md`, and the new `china-university-diagram-guidelines.md`.
- The scaffold script now produces `.drawio` source files, `diagram-briefs/`, `context/project-snapshot.md`, `diagram-plan.md`, `diagram-index.md`, and `validation-summary.md`, matching the updated contract.
- The obsolete Mermaid renderer under `Arc/arc:uml/scripts/` was removed so the skill directory no longer advertises two competing delivery paths.
- Repository validation passed with `uv run python scripts/validate_skills.py`, targeted validation tests passed with `uv run pytest tests/test_skill_validation.py -q`, and `uv run python scripts/build_skills_index.py` refreshed `skills.index.json`.
