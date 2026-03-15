# Task Todo

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
- [x] Keep the launcher stateless apart from the build cache under `${XDG_CACHE_HOME:-$HOME/.cache}/arc-serve`.
- [x] Update skill instructions and controller usage text to point at the launcher entrypoint.
- [x] Verify the launcher executes the existing Go controller and preserves tmux/registry semantics.

## Review

- `arc:serve` now avoids repetitive `go run` compile overhead while still using a short-lived Go controller for each action.
- The launcher only builds when the controller source hash changes, then `exec`s the cached binary directly on later runs.
