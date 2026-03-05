---
name: "arc:retest"
description: "修复后必须重启服务时使用：执行失败-修复-回归的多轮闭环并沉淀证据。"
---

# tmux startup + arc:e2e regression closed loop (industrialization)

## Overview

Used for projects where "services must be restarted after repairs to take effect": connect service startup/restart, log forensics, regression testing, and failure recovery into a repeatable closed loop, and each round has deliverable evidence.

## Quick Contract

- **Trigger**: After repair, the service needs to be restarted and multiple rounds of regression closed-loop verification must be performed.
- **Inputs**: tmux service configuration, `arc:e2e` parameters, `max_iterations`.
- **Outputs**: Each round `run_id`, service log path, test results and failure diversion information.
- **Quality Gate**: The evidence chain and iteration control check of `## Quality Gates` must be passed before convergence.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:retest`, first pull up the service and establish a regression closed loop, and then verify it round by round."

## The Iron Law

```
NO RETEST LOOP WITHOUT SERVICE RESTART AND TRACEABLE EVIDENCE
```

Without restarting the confirmation and evidence chain, the regression results cannot be claimed to be valid.

## Workflow

1. Prepare the `tmux` startup configuration and record the baseline status.
2. Start/restart the service and download the logs.
3. Execute `arc:e2e` to produce this round of test artifacts.
4. FAIL will enter `arc:fix` for repair, and PASS will converge and deliver.

## Quality Gates

- Each round must output `run_id`, `run_dir`, log path and git status.
- Service logs and test reports must be correlatable in the same round.
- FAIL must be accompanied by evidence of root cause and repair, and "feeling repaired" is prohibited.
- When the iteration limit is reached, it must stop and output next step suggestions.

## Red Flags

- After repair, retest directly without restarting.
- The report passed but the corresponding log and run evidence could not be found.
- Infinite loop regression has no `max_iterations` upper limit.
- "Made PASS" by bypassing authentication/permissions logic.

## When to Use

- **首选触发**: Repair requires restarting the service and executing multiple rounds of fail→fix→retest closed loops.
- **典型场景**: It is necessary to unify the trinity evidence chain of run_id, log and report.
- **边界提示**: To locate the root cause of a single failure, `arc:fix` can be used directly.

## Context Budget (avoid Request too large)

- The service log is subject to placement (`logs/uxloop/<run_id>/...`); do not paste the entire log into the conversation. If necessary, only excerpt "a small number of lines near the failure time point" and indicate the source file path.
- Use the same `run_id` as the test report, so that the evidence chain is aligned only by paths (less talking, more paths).

## Definition of Done (must be produced in each round)

Each iteration is clearly output on stdout (easy to paste into ticket/PR/daily report):

- `iter`: 01..N
- `run_id` / `run_dir` (arc:e2e artifact directory)
- `result`: `PASS|FAIL` + one sentence failure type (selector/timing/auth/backend/env/data/unknown)
- `git`: `HEAD` + whether it is dirty (whether `git status --porcelain` is empty)
- `tmux`: `session_name` / `window_name` / Pane corresponding to each service
- `logs`: `logs_dir` + log_file for each service

and:

- FAIL → Must be handed over to `arc:fix` to output the root cause and repair evidence ("feeling repaired" is prohibited)
- PASS → Must give path to `run_id` + `report.md` via run

## Artifacts & Paths (where to put documents/files)

It is recommended to use the same `run_id`** for each iteration throughout the "service log + test report" so that the evidence can be directly aligned:

- **Service Log (this Skill/tmux)**: Default `logs/uxloop/<run_id>/<service>.log` (can be overridden with `uxloop_tmux.py --logs-dir ...`).
- **Test Report (arc:e2e)**: Default `reports/<run_id>/` (can be overridden with `scaffold_run.py --report-output-dir ...`).
- **Account management (arc:e2e)**: `reports/<run_id>/accounts.jsonc` (plain text account/password/Token; not allowed to be submitted to the database).

Example (the same `run_id` is used to connect logs+reports):

```bash
run_id="2026-02-01_14-00-00_abcd_iter01"
python arc:retest/scripts/uxloop_tmux.py --config uxloop.config.json --run-id "$run_id" --reset-window --wait-ready
# Then run arc:e2e with the same run_id and output reports/$run_id/
```

## Inputs (the caller needs to provide)

### A) Service startup configuration (recommended)

Prefer JSON config (reusable and reviewable):

- Reference: `assets/uxloop.config.example.json`
- Fields:
  - `session_name` (default `uxloop`)
  - `window_name` (default `svc`)
  - `services[]`：
    - `name` (for pane title and log file name)
    - `cwd` (optional)
    - `command`
    - `env` (optional, key values ​​are all strings)
    - `ready_check` (optional): `http|tcp|cmd`

### B) arc:e2e parameter

- `target_url`
- `test_objective`
- `personas`
-(optional)`validation_container`

### C) Iterative control

- `max_iterations` (default 5)

## Quick Start (scripted recommended)

1) Prepare configuration: copy `assets/uxloop.config.example.json` to the project (for example `uxloop.config.json`) and change `cwd/command/ready_check` according to the project.

2) Start/restart the service + log down (repeat this after each repair to ensure that the service and front-end bundle are up to date):

```bash
python arc:retest/scripts/uxloop_tmux.py \
  --config uxloop.config.json \
  --reset-window \
  --wait-ready
```

3) Enter iterative regression (see Loop Workflow below).

> Advanced tmux manipulation and forensics: see `references/tmux-runbook.md`.

## Markdown format verification (mandatory)

After generating or modifying any Markdown file, the format must be verified immediately:

- **Table column number alignment**: The number of header rows, separator rows, and data rows and columns must be exactly the same (for example, if the header has 7 columns, then each row has 7 columns).
- **Delimited row format**: Each column must be `---`, `:---`, `---:`, or `:---:` and cannot be empty or missing.
- **Special Character Escape**: Cells containing `|` must be escaped to `\|`; lines containing line breaks must be replaced with `<br>`.
- **Verification method**:
  1. `python arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict` (recommended)
  2. `mdformat --check <file.md>` (mdformat needs to be installed)
  3. Manually count the number of columns table by table
- **Verification failure must be repaired before continuing** and cannot be skipped.

## tmux convention (mandatory)

- The same closed loop uses the same session (default `uxloop`) to avoid multiple sessions being scattered and difficult to troubleshoot.
- One pane per service, and stdout/stderr must be visible.
- The log must be placed (`logs/uxloop/<loop_run_id>/...` is recommended), otherwise the evidence chain will be broken when FAIL occurs.
- Restart strategy:
  - Prioritize to stop in pane `Ctrl-C`, and then re-execute the start command
  - If the process is dead: locate the PID and kill it accurately; the last resort is `tmux kill-session`

## Loop Workflow (until PASS)

### 0) Pre-check (before each start)

- Confirm that `tmux` is available: `tmux -V`
- If you are not sure about the startup method: read the project startup script first (package scripts/Makefile/compose)
- Clean up conflicting ports (only stop relevant processes; don’t “solve” it by changing code to bypass permissions/authentication logic)
- Record git status:
  - `git rev-parse HEAD`
  - `git status --porcelain`

### 1) Start/restart service (tmux)

Prioritize the use of scripts (repeatable, diskable, and reusable):

- `python arc:retest/scripts/uxloop_tmux.py --config <cfg> --reset-window --wait-ready`

Wait for the service to be ready (ready_check/health check/open homepage/port connection), and write key observations into the `DEBUG:` log.

### 2) Execute a round of arc:e2e

- Generate a new `run_id` (it is recommended to bring the iteration number: `..._iter01`)
- Run arc:e2e and make sure the output/compile report is:
  - `python arc:e2e/scripts/scaffold_run.py ...`
  - `python arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python arc:e2e/scripts/compile_report.py --run-dir <run_dir> --in-place` (optional: add `--beautify-md` if `mdformat` is already installed in venv)

### 3) Judgment results

- PASS: Stop the loop and output the final run_id and report path
- FAIL: Enter repair
  - Take the current `run_dir` as input and locate and repair it according to the process of `arc:fix`
  - After repairing, return to step 1 (restart the service and test again; otherwise it is easy to "fix but not take effect")

### 4) Cover all exit conditions

- Reaching `max_iterations` and still failing: stop, summarize each run_id, failure type distribution, current most likely root cause and next step suggestions

## Resource & Cleanup (resource control and timely shutdown)

- Do not open sessions/services indefinitely: always reuse the same `session_name`, and use `max_iterations` to constrain the number of regressions.
- The log/report will continue to grow: it is recommended to use independent `run_id` for each round, and clean up `logs/uxloop/<run_id>/` and `reports/<run_id>/` as needed after the end.
- Close tmux and services promptly after completion (to prevent background processes from occupying CPU/memory/ports for a long time):

```bash
python arc:retest/scripts/uxloop_cleanup.py --session uxloop --window svc --kill-session
```
