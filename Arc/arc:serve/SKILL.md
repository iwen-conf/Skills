---
name: arc:serve
description: "本地服务启动与 tmux 会话托管：当用户说“启动前后端/启动 dev server/重启本地服务/重跑端口服务”时触发；用于用 tmux 启停本地长时服务、维护 sessions.json，并在重启前先检查并关闭同名 session，避免重复占用 CPU、内存和端口。"
version: 1.0.0
allowed_tools:
  - Bash
  - Read
  - Edit
  - Write
  - Grep
  - Glob
hooks:
  PreToolUse:
    - matcher: Bash
      hooks:
        - type: command
          command: "bash ${ARC_SKILL_DIR}/scripts/check-destructive.sh"
          statusMessage: "Checking for destructive commands..."
---

# arc:serve — local service session orchestration

## Overview

`arc:serve` is responsible for starting, restarting, stopping, and checking local long-running project services through `tmux`.

The skill is intentionally strict:

- Treat `tmux` as the control plane for local frontend/backend/dev servers.
- Persist service metadata into `<project_path>/.arc/serve/tmux-sessions.json`.
- Reconcile JSON state with `tmux ls` before each action.
- Use a cached Go binary launcher so the first call builds once and later calls reuse the same one-shot controller without paying `go run` compile cost every time.
- Prevent duplicate CPU, memory, and port consumption by enforcing one managed session per project/service pair.

## Quick Contract

- **Trigger**: The user asks to start, restart, stop, or inspect a local frontend/backend/dev server or another long-running project service.
- **Inputs**: `project_path`, `service_name`, `action`, and for start/restart a `command`; optional `working_directory`, `ports`, and `session_name`.
- **Outputs**: Managed tmux session state plus refreshed `<project_path>/.arc/serve/tmux-sessions.json`.
- **Quality Gate**: The same service must not end up with duplicate running tmux sessions; restart must kill the old session before launching the new one.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:serve` to reconcile the existing tmux session first, and then start/restart the local service without duplicates."

## The Iron Law

```text
NO DUPLICATE LOCAL SERVICE SESSION
```

Before launching any long-running local service, check the project-local registry and live `tmux` state first.

## Workflow

1. Normalize the request into one action: `start`, `restart`, `stop`, `status`, or `cleanup`.
2. Choose a stable `service_name` such as `frontend`, `backend`, `api`, or `worker`, then resolve the registry path `<project_path>/.arc/serve/tmux-sessions.json`.
3. Run `tmux ls` / `tmux has-session` and reconcile the target registry entry before changing anything.
4. For `start`, reuse the existing live session instead of launching a duplicate; for `restart`, kill the existing session first and only then create a new one.
5. Update JSON state with `session_name`, `cwd`, `command`, declared `ports`, timestamps, and last action so future turns can reuse the same session contract.
6. After `start` or `restart`, verify the tmux session still exists and report the exact session name back to the user.

## Quality Gates

- The managed service must have a stable `service_name` and deterministic `session_name`.
- `restart` must prove the old session has been terminated before the replacement is launched.
- `tmux-sessions.json` must be updated after each action and stale entries must be reconciled against live tmux state.
- If the user supplied ports, the registry must preserve those `ports` for later conflict checks and follow-up turns.
- Long-running project services must not be left in the raw foreground shell when `tmux` is available.

## Expert Standards

- Treat `tmux` as the authoritative runtime wrapper for local long-running services and keep the session name deterministic across turns.
- Enforce `single-instance` service ownership for each project/service pair; do not blindly launch a second frontend or backend process.
- Maintain a JSON `registry` and reconcile it with live tmux state before every action; the registry is memory, tmux is liveness.
- Record declared `port` values with the service metadata so later turns can detect likely conflicts and reuse the right session.
- Prefer `graceful shutdown` when feasible, but if the user explicitly asks for restart, the previous tmux session must be stopped before replacement.
- For Cloudflare-based services, invoke the locally installed `wrangler` command directly; NEVER use `npx wrangler`.

## Scripts & Commands

- Start service: `Arc/arc:serve/scripts/tmux_service_ctl start --project-root <project_path> --service frontend --cwd <cwd> --ports 3000,5173 --command "pnpm dev --host 0.0.0.0"`
- Restart service: `Arc/arc:serve/scripts/tmux_service_ctl restart --project-root <project_path> --service backend --cwd <cwd> --ports 8080 --command "go run ./cmd/server"`
- Stop service: `Arc/arc:serve/scripts/tmux_service_ctl stop --project-root <project_path> --service frontend`
- Check one service: `Arc/arc:serve/scripts/tmux_service_ctl status --project-root <project_path> --service frontend`
- Reconcile all remembered sessions: `Arc/arc:serve/scripts/tmux_service_ctl cleanup --project-root <project_path>`
- Inspect live sessions directly: `tmux ls`
- Inspect recent logs: `tmux capture-pane -p -t <session_name>`

## Red Flags

- Starting frontend/backend dev servers directly in the foreground shell and leaving them orphaned.
- Restarting a service without checking `tmux ls` and the JSON registry first.
- Using a random session name every turn so the same service cannot be matched and reused.
- Writing `tmux-sessions.json` optimistically without reconciling actual tmux liveness.
- Launching duplicate services for the same project/service/port tuple and wasting CPU or memory.

## When to Use

- **Primary Trigger**: The user asks to start, restart, stop, or inspect a local frontend, backend, worker, API server, or other long-running project service.
- **Typical Scenario**: Local development commands such as `pnpm dev`, `npm run dev`, `vite`, `next dev`, `go run`, `air`, `uvicorn`, `cargo run`, or paired frontend/backend services that should stay alive across turns.
- **Boundary Note**: Do not use `arc:serve` for production deployment, Docker Compose/Kubernetes orchestration, or one-shot commands like build/test/lint that should exit immediately.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the target project root |
| `service_name` | string | yes | Stable logical service label, for example `frontend` or `backend` |
| `action` | enum | yes | `start` / `restart` / `stop` / `status` / `cleanup` |
| `working_directory` | string | no | Working directory for the service command; defaults to `project_path` |
| `command` | string | no | Required for `start` and `restart` |
| `ports` | string | no | Comma-separated port list, for example `3000,5173` |
| `registry_path` | string | no | Override the default registry path `<project_path>/.arc/serve/tmux-sessions.json` |
| `session_name` | string | no | Optional exact tmux session name override |

## Instructions

### Phase 1: Resolve the service contract

1. Detect whether the user means frontend, backend, API, worker, or another long-running local service.
2. Choose a stable `service_name` and reuse it on later turns so the same tmux session can be found again.
3. Derive `working_directory` and `command` from the project structure, and keep the command string explicit in the registry.

### Phase 2: Reconcile state before action

1. Read `<project_path>/.arc/serve/tmux-sessions.json` if it exists.
2. Run `tmux ls` and `tmux has-session -t <session_name>`.
3. If the registry says `running` but tmux no longer has the session, mark the entry stale or missing before proceeding.
4. If the user said `restart`, stop the existing session before starting the replacement.

### Phase 3: Execute through tmux

1. Start with `tmux new-session -d -s <session_name> -c <cwd> "<command>"`.
2. Stop with `tmux kill-session -t <session_name>`.
3. Never create a duplicate live session for the same logical service.
4. Prefer the bundled launcher so it can reuse a cached Go binary, keep the JSON registry consistent, and still exit immediately after the control action:

```bash
Arc/arc:serve/scripts/tmux_service_ctl start \
  --project-root <project_path> \
  --service frontend \
  --cwd <cwd> \
  --ports 3000 \
  --command "pnpm dev"
```

The launcher builds `tmux_service_ctl.go` into `${XDG_CACHE_HOME:-$HOME/.cache}/arc:serve/...` on first use or after the source changes, then reuses that binary on later calls.

### Phase 4: Persist and report

1. Refresh `<project_path>/.arc/serve/tmux-sessions.json`.
2. Report the exact `session_name`, `service_name`, `cwd`, `command`, and recorded `ports`.
3. On the next related turn, check the registry first and confirm whether the same session is still live before launching anything.

## Outputs

```text
<project_path>/.arc/serve/
├── tmux-sessions.json
└── (managed tmux sessions referenced by name in the registry)
```

## Sign-off

```text
files changed:    N (+X -Y)
scope:            on target / drift: [what]
hard stops:       N found, N fixed, N deferred
signals:          N noted
verification:     [command] → pass / fail
```
