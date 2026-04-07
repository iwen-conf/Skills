---
name: arc:fix
description: "故障修复闭环：定位根因、实施修复并回归验证；当用户说“线上故障/bug 修复/incident triage/fix broken flow”时触发。"
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

# arc:fix — evidence-based fault repair loop

## Overview

`arc:fix` turns failure artifacts or online fault clues into a closed repair loop: reproduce or stabilize the failure, locate the root cause, implement the minimum safe fix, and return fail-to-pass evidence.

Mode note:
- Use `arc:fix --mode retest-loop` when the service must be restarted and validated through multiple fail-fix-retest rounds.

## Quick Contract

- **Trigger**: `arc:e2e` FAIL, CI test intermittent failure, or online failure requires stop loss recovery.
- **Inputs**: Failure `run_dir` or CI failure samples, logs and time series, test targets and account context if necessary.
- **Outputs**: Root cause analysis, fix instructions, fail/pass or stability control evidence and `fix-packet`.
- **Quality Gate**: The fail-to-pass evidence chain check of `## Quality Gates` must be passed before closing the issue.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:fix` to fix the failure evidence first, then locate the root cause and verify the fix."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO FIX CLAIM WITHOUT FAIL-TO-PASS EVIDENCE CHAIN
```

Without the evidence chain of "failure→repair→pass", it is not allowed to claim that the problem has been solved.

## Mandatory Hypothesis

**Do not touch code until you can state the root cause in one sentence.**

Before going further, commit to a testable claim:
> "I believe the root cause is [X] because [evidence]."

The claim must name a specific file, function, line, or condition. "A state management issue" is not testable. If you cannot be that specific, you do not have a hypothesis yet.

## Rationalization Watch

These phrases are diagnostic failures in disguise. When one surfaces in your thoughts, stop and re-examine:

| What you're thinking | What it actually means | Rule |
|---|---|---|
| "I'll just try this one thing" | No hypothesis, random-walking | Stop. Write the hypothesis first. |
| "I'm confident it's X" | Confidence is not evidence | Run an instrument that proves it. |
| "Probably the same issue as before" | Treating a new symptom as a known pattern | Re-read the execution path from scratch. |
| "It works on my machine" | Environment difference IS the bug | Enumerate every env difference before dismissing it. |
| "One more restart should fix it" | Avoiding the error message | Read the last error verbatim. Never restart more than twice without new evidence. |

## Hard Stops

**Same symptom after a fix is a hard stop.** If the user reports the same symptom after a patch was applied, do not patch again. Treat it as a new investigation: the previous hypothesis was wrong. Re-read the execution path from scratch. Three rounds of "fixed but still broken" in the same area means the abstraction is wrong, not the specific line.

**Never state environment details from memory.** Before diagnosing OS, compiler, SDK, or tool version issues, run the detection command first: `sw_vers`, `node --version`, `rustc --version`, etc. State the actual output. A diagnosis built on an assumed version is not a diagnosis.

## Workflow

1. Fixed minimal reproducibility evidence and context for failed runs.
2. Quickly triage and locate the root cause (product/test/environment/data).
3. Implement minimal fixes and keep permission/authentication semantics from being bypassed.
4. Regression verification and output deliverable Fix Packet.

## Quality Gates

- Both failing and passing run evidence must be provided.
- The root cause must be specific to a module, condition, or timing point.
- Remediation instructions must include impact surfaces and regression commands.
- DEBUG clues and artifact paths need to be retained throughout the process.
- Medium/High-risk repairs must record a recoverable boundary before code changes: pushed remote branch, snapshot, or explicit blocker.
- Fail/pass snapshots, impact summaries, and regression checklists delivered in chat should prefer `terminal-table-output` when the content stays compact; the Fix Packet files stay in their native Markdown form.

## Expert Standards

- For online incidents, fault severity classification should follow `SEV-1/SEV-2/SEV-3` grading with the team's defined response expectations.
- Root cause analysis needs to be combined with `5 Whys + Fault Tree` to avoid staying at the superficial causes.
- For online recovery, verification should record `MTTA/MTTR` and the agreed post-recovery stability window.
- For online repairs, it is recommended to use `canary/feature flag` first and bind the rollback threshold.
- High-impact incident closure should produce `Blameless Postmortem` and CAPA (Corrective/Preventive Action) items.
- Strict adherence to the `Mandatory Hypothesis` before touching any code.
- Actively monitor internal assumptions using the `Rationalization Watch` to prevent diagnostic failures.

## Scripts & Commands

- Triage summary: `python3 Arc/arc:fix/scripts/triage_run.py <run_dir> --json-out <triage.json> --md-out <triage.md>`
- Runtime main command: `arc fix`
- Multiple loop regression mode: `arc fix --mode retest-loop`
- Recommended link: `arc e2e` (produce evidence) → `arc fix` (fix closed loop)

## Red Flags

- Only a "repaired" description is given, with no evidence of recurrence.
- Avoid failures by deleting authentication/authorization logic.
- Temporarily switching OTA/update targets, writable directories, feed URLs, or local config just to make a user-facing update path appear healthy.
- Treat test false positives as product defects and directly modify the code.
- The ticket will be closed without retesting.

## When to Use

- **Primary Trigger**: FAIL evidence or online fault signal has been obtained, and the root cause needs to be located and recovery promoted.
- **Typical Scenario**: Intermittent failure, unstable regression, surge in online errors, repair of critical business interruptions.
- **Boundary Note**: Execute `arc:e2e` first when there is no reproducible failed artifact; only use `arc:audit` for quality review conclusions.

## Context Budget (must be split to avoid too long context)

- By default, only **1** `run_dir` is triageed at a time; in-depth analysis can focus on up to **1~2** FAIL steps.
- The evidence should be placed in `<run_dir>/analysis/` first, and only the path and key conclusions should be quoted in the conversation; do not paste the entire report/log.
- If you encounter `Request too large`: first generate `analysis/triage.md` (overview) → then press step to process one by one (only look at the minimum necessary fragment each time).

## Definition of Done (Must be delivered eventually)

The final output must contain (it is recommended to organize directly by `references/fix-packet.template.md`):

- **Failing evidence**: At least 1 set of failure evidence (`run_id/run_dir` or CI log sample)
- **Passing evidence**: At least 1 set of passing/recovery evidence (regression run or stability statistics)
- **Root Cause**: Specific to module/condition/selector/concurrency/permission policy
- **Fix**: Involved files/key logic and why it was changed
- **Verification**: How to reproduce failure + how to run regression (including commands/parameters)
- **Guardrails**: Explicitly stated not "fixed" by short-circuiting/removing authentication authorization

## Inputs (need to be provided by the caller/or generated by you)

Work on the failed artifact first:

- `run_dir`: e.g. `reports/2026-02-01_14-00-00_abcd/`
- `failing_tests`: e.g. `["tests/api/test_order.py::test_retry"]`
- Without `run_dir`, arc:e2e's core parameters are required: `test_objective`, `personas`, `target_url` (and optionally `validation_container`)

## Quick Triage (recommended to run first)

First use a script to do a best-effort summary (it does not replace manual analysis, but it can speed up positioning):

```bash
python Arc/arc:fix/scripts/triage_run.py <run_dir>
```

Triage decision tree (for quick judgment of "product defects vs test false positives vs environment/data/flake"): see `references/triage-decision-tree.md`.

## Artifacts & Paths (where to put documents/files)

All evidence is rooted in `run_dir` of arc:e2e:

- **Failing run**:`reports/<fail_run_id>/`
- **Passing run**:`reports/<pass_run_id>/`
- **Accounts file (unified account management)**: `<run_dir>/accounts.jsonc` (store the credential set used for reproduction; keep secrets redacted outside this file and do not commit it to the repository)

It is recommended to put "Analyze/Repair Deliverables" in `run_dir/analysis/` (does not affect `check_artifacts.py --strict`):

- `<run_dir>/analysis/triage.md` / `<run_dir>/analysis/triage.json` (generated from `triage_run.py`)
- `<run_dir>/analysis/fix-packet.md` (fill in as `references/fix-packet.template.md`, refer to fail+pass two runs)

If database migration/DDL/DML is involved (user consent must be obtained first), it is recommended to put the change control evidence in `<run_dir>/db/`:

- `<run_dir>/db/migration-approval.md` (paste the original text/screenshot path agreed by the user)
- `<run_dir>/db/migration-plan.md` (change content, scope of impact, rollback strategy)
- `<run_dir>/db/migration-execution.md` (actual execution command and output summary)

Example (drop triage to run_dir):

```bash
mkdir -p <run_dir>/analysis
python Arc/arc:fix/scripts/triage_run.py <run_dir> \
  --md-out <run_dir>/analysis/triage.md \
  --json-out <run_dir>/analysis/triage.json
```

## Core iron rules (must be followed)

0. **Markdown format verification (highest priority)**
   - **Mandatory**: After generating or modifying any Markdown file, the format must be verified immediately.
   - **Table column number alignment**: The number of header rows, separator rows, and data rows and columns must be exactly the same (for example, if the header has 7 columns, then each row has 7 columns).
   - **Delimited row format**: Each column must be `---`, `:---`, `---:`, or `:---:` and cannot be empty or missing.
   - **Special Character Escape**: Cells containing `|` must be escaped to `\|`; lines containing line breaks must be replaced with `<br>`.
   - **Verification method**:
     1. `python Arc/arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict` (recommended)
     2. `mdformat --check <file.md>` (mdformat needs to be installed)
     3. Manually count the number of columns table by table
   - **Verification failure must be repaired before continuing** and cannot be skipped.

1. **It is prohibited to bypass permission (authentication/authorization) verification to "repair"**
   - The "permissions" here refer to **business permissions/authentication/authorization** (not file system chmod/chown).
   - It is forbidden to comment/delete/short-circuit permission/auth middleware or verification in the code (for example, change "no permission should fail" to directly return success, or remove the judgment that roles/ACLs are required).
   - **Diagnosis over Bypass**: When facing an error in a permission-protected route or encountering a bug, you MUST NOT bypass the permission/auth logic to "make it work" or "test inner logic". Instead, you must use extensive logging, console printing, and error collection solutions to diagnose the root cause *within the context of the real business logic*.
   - If the problem is indeed a defect in permission rules/role mapping/policy configuration: Fix it to **in line with business expectations**, and make it back to cover both sides of the "should be allowed/should be denied" boundary.
   - For OTA/update/install defects, do not mutate the underlying target directory, package source, local config, or feature flag just to let the page button succeed. If the real user path fails, fix that path instead of rewriting the environment underneath it.

2. **It is strictly prohibited to bypass type safety or business logic to "repair" (Type & Logic Bypass is FORBIDDEN)**
   - **Type Suppression**: Do NOT change types to `any`, use `@ts-ignore`, `# type: ignore`, or use explicit type casting just to silence compiler errors. This hides the root cause and leads to instability and unmaintainability.
   - **Logic Bypass**: Do NOT comment out throwing errors, delete assertions, or forcefully return a mock value just to make a test pass or an error disappear. This violates the original intent of the business logic.
   - **Root Cause Focus**: You MUST understand and fix the underlying issue. The code should remain maintainable, strictly typed, and logically sound after the fix.

3. **Lots of DEBUG output (forced)**
   - During the troubleshooting process, a large number of logs starting with `DEBUG:` must be output to the terminal, covering: input, key branches, key variables, external request/response summary, and exception stack.
   - Prioritize adding **controllable** DEBUG logs to your code (it is recommended to use environment variables/configuration switches, such as `DEBUG_UI_UX_FIX=1`) to avoid permanently contaminating normal logs.

4. **Evidence driven**
   - "Feeling fixed" is not allowed: evidence must be given for each fix (at least one failed reproduction run + one regression via the run_id/report path of the run).

5. **Comply with arc:e2e's log and artifact specifications**
   - If you need to generate/update a report, give priority to using the script under `Arc/arc:e2e/scripts/` instead of creating your own format.

6. **DB Migration/DDL/DML Change Control**
   - Any database migration/DDL/DML (including but not limited to migrate, ALTER, INSERT/UPDATE/DELETE, backfill data) must first obtain the user's explicit consent.
   - Before agreeing: only read-only verification (SELECT) and code troubleshooting can be done; "migrate first and then add tickets" is not allowed.
   - After consent: Write the consent and implementation evidence into `run_dir/db/` (see suggestion document above) and clearly state it in the final Fix Packet.
   - This same rule applies to update channels, writable targets, local OTA directories, and other runtime configuration: do not change them as a hidden shortcut during verification unless the user explicitly approves that infrastructure change as part of the real fix.

7. **New accounts (used to verify repairs) must be auditable**
   - If a new account must be created in order to verify the repair (such as verifying first login, permission boundaries, new tenant isolation), you must write:
     - `accounts.jsonc`: Mark the account with `created_for_verification=true` and write `why/created_at`
     - `report.md`: `Account Changes` Chapter explains "Why a new account is generated"

8. **Resource Control & Cleanup (resource control and timely shutdown)**
   - Running "unlimited" retry/regression loops or zero-delay polling (Busy Loop) is **strictly prohibited**; must have `max_iterations`/timeout and explicit delays (e.g., `sleep`).
   - Whether using Python or Node/Bun, you MUST prevent CPU/Memory crashes by avoiding unbatched mass-concurrency (e.g., thousands of uncontrolled Promises or excessive multi-processing) and avoiding scripted deep-directory traversal (use `rg`/`fd` instead).
   - If additional tools (browser recording, long-term tail, temporary container, tmux session, etc.) are started for troubleshooting, close them promptly after obtaining the evidence to avoid resource leakage and long-term occupation.

9. **Checkpoint before risky repairs**
   - Before Medium/High-risk hotfixes, confirm the recoverable boundary first: pushed remote branch, local snapshot, or a clearly documented blocker.
   - If the repository has no safe remote and the target host is Gitea, coordinate with `agent-browser` to create a repository at `https://gitea.ezer.heiyu.space/`, using `lazycat_gitea_account` and `lazycat_gitea_password`, choose visibility according to user intent or existing policy, then backfill the SSH URL into local `.git` remote and push the current branch.
   - For scans, polling, or browser glue during triage, follow a CLI-first ladder: `rg/git/sed/awk/fd/find` → `jq` and shell pipelines → `mcp_chrome-devtools_*` tools or `agent-browser` → `gh` / `just` when applicable → **Go** (`go run`) for high-performance/CPU-intensive tasks to prevent crashes → JS tools such as `bun` (with batching) → `tmux` for long-running sessions → repo-native scripts → Python only when nothing lighter fits.

10. **Proactive Extrapolation (举一反三) & Anti-NPC Debugging (强制要求)**
   - **No blind retries**: If a fix fails 2 times, you MUST stop tweaking parameters and perform a 7-step check: read full logs, search codebase, read 50 lines of context, verify assumptions, reverse assumptions, isolate, and change approach.
   - **Fix Extension**: When the immediate bug is fixed, you MUST check if the same problematic pattern exists elsewhere in the project. The final Fix Packet MUST include a "Proactive Extension" section documenting these checks and additional preventative fixes.

## Workflow (recommended to execute in order)

### 0) If run_dir is missing: run arc:e2e first to produce failure artifacts

- Create a directory skeleton with scaffolding (recommended `--pack full-process`, at least implicitly including `e2e`):
  - `python Arc/arc:e2e/scripts/scaffold_run.py --help`
  - `python Arc/arc:e2e/scripts/scaffold_run.py --pack full-process --objective "<objective>" --target-url "<url>" --personas "<json-or-path>"`
- Press `arc:e2e/SKILL.md` to execute the test to ensure disk placement:
  - `report.md`,`action-log.md`,`screenshot-manifest.md`,`screenshots/`
-(optional)`events.jsonl`
- After failure, use the output `run_dir` as the input of this Skill to continue with the subsequent steps.

### 1) Obtain failure evidence and compile a report

- If `run_dir` already exists: verify and compile first
  - `python Arc/arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python Arc/arc:e2e/scripts/compile_report.py --run-dir <run_dir> --in-place` (optional: add `--beautify-md` if `mdformat` is already installed in venv)
- Extract "minimum reproducible information" from the following files:
  - `report.md` (failed step table)
  - `failures/*.md` (defect description)
  - `action-log.md` / `events.jsonl` (operation track)
  - `screenshots/` (visual evidence)

### 2) Reproduction failed (reduced to the smallest steps)

- Goal: Convergence failure into "from which page, what was clicked, what was expected, what was actual, and where did it start to deviate".
- Output `DEBUG:` log (contains at least):
  - Recurrence entrance URL, account role, step number step_id, page URL
  - Key DOM/interface request observations (e.g. toast copy, HTTP status codes, key fields)


**Get context before starting diagnostics in the following priority: **

**Priority 0: Read shared context index (`.arc/context-hub/index.json`)**
1. Retrieve reusable products: `codemap.md`, `arc:audit` snapshot, score results (generated by `score/` module), `arc:build` handoff
2. Verify `expires_at`, `content_hash`, file existence
3. If available, load it directly and narrow down the scope of investigation.
4. If it fails, press `refresh_skill` reflow to trigger the update (`arc:init --mode update` / `arc:cartography` / `arc:audit` / `score` module refresh (triggered by `arc:gate` orchestration))

**Priority 1: Check `.arc/audit/` Architecture Analysis**
1. Find review artifacts: Check `.arc/audit/<project-name>/` for the presence of an architectural analysis document
2. Verify freshness: check that `project-snapshot.md` or `diagnosis-report.md` was generated < 7 days ago
3. Extract key information: module dependencies, common defect patterns, repair strategy suggestions
4. If the review product does not exist or has expired: trigger the `arc:audit` update first and then continue.

**Priority 2: Check `.arc/fix/known-patterns.md`**
1. Find historical defect patterns: read `.arc/fix/known-patterns.md` if present
2. Match current failure: Compare current failure symptoms to known patterns
3. Reuse repair strategy: If the match is successful, apply the verified repair solution directly

**Priority 3: Use ace-tool to search source code**
When the cache information is insufficient, use ace-tool to search the project source code to locate the root cause of the problem.

**Priority 4: Cache Verification and Error Reporting**
During the diagnostic process, if you discover that the information in `.arc/audit/` or CLAUDE.md is inaccurate:
1. Tag cache errors: logging expected vs. actual
2. Falling back to source code search: use ace-tool to get the correct information
3. Generate error report: Generate cache validation failure report at `<run_dir>/analysis/context-errors/`
4. Reflow update:
   - Review data issues → `arc:audit`
   - CLAUDE index problem → `arc:init --mode update`
   - Code map issue → `arc:cartography`

**Priority 5: Update known defect pattern library**
After successfully fixing the defect, append the newly discovered defect pattern to `.arc/fix/known-patterns.md`.

---

### 3) Diversion diagnosis (prioritize the elimination of "test false alarms")

Quickly judge by priority:

- **Selector/Copy Change**: Element does not exist/role changes/text changes
- **Timing issues**: slow loading, incorrect waiting conditions, race condition
- **Permissions/Authentication**: Login status lost, token expired, cross-account not logged out
- **Backend exception**: 500/timeout/data inconsistency
- **Environmental issues**: service not started, port conflict, database unreachable

If it is judged to be a false positive by the test script/selector: first correct the steps and selector of arc:e2e, and then go back again to confirm whether the real defect still exists.

### 4) Enhance observability (add DEBUG to the code)

- First add the "observation point" and then move to the "business logic".
- It is recommended to unify the format to facilitate grep:
  - Backend: `DEBUG_UI_UX_FIX` + `run_id` + `step_id` + `request_id`
  - Frontend: `console.debug('[DEBUG_UI_UX_FIX]', { runId, stepId, ... })`
- Record "Input → Intermediate → Output" to avoid just typing "Arrived here".

### 5) Repair (submit in small steps to avoid damaging the surface)

- Prioritize root causes: state machine, verification, boundary conditions, concurrency, selector stability, etc.
- It is forbidden to use permission modification to cover up problems (see Iron Rule 1).
- Run the minimum recurrence path after each change; run the complete regression after confirming that the failure has disappeared.

### 5.1) Engineering suggestions for UI related repairs (common industrial-grade practices)

- Prioritize the introduction/use of stable selectors: `data-testid`, ARIA role + label; avoid relying on volatile copywriting and complex CSS.
- Avoid `sleep`-style waits: use deterministic conditions such as "element appears/can be clicked/request completed/Toast appears".
- Evidence is required for Flake: the same commit must be run at least 2 to 3 times and still pass stably before it can be judged as "fixed".

### 6) Regression verification and production of deliverable repair solutions

- **Code Quality Check (Mandatory)**: You MUST run `bash Arc/scripts/check-placeholders.sh` to ensure no lazy placeholder code was committed to the diff.
- **Project Verification (Mandatory)**: Before generating the final fix packet, you MUST run the standard project verification script to ensure no regressions were introduced to the type system or test suite:
  - `bash Arc/scripts/verify-project.sh` (or the equivalent project-specific test command)
- Re-execute arc:e2e, generate a new `run_dir`, and again:
  - `python Arc/arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python Arc/arc:e2e/scripts/compile_report.py --run-dir <run_dir> --in-place` (optional: add `--beautify-md` if `mdformat` is already installed in venv)
- Which gives in the final output:
  - **Root Cause**
  - **Fix (repair point)**: involving files/modules/key logic
  - **Verification**: failed run_id + passed run_id + reporting path
  - **Risk & Follow-ups**: possible regression points, single tests/monitoring that need to be supplemented

> Final delivery template: see `references/fix-packet.template.md`.

## Defect file (optional but recommended)

When a clear failed step is located, a defect file is generated for easy tracking:

```bash
python Arc/arc:e2e/scripts/new_defect.py \
  --run-dir <run_dir> \
  --step 0007 \
--title "500 appears after clicking submit" \
  --role buyer \
  --url "http://localhost:5173/order/new" \
  --user "buyer_01" \
  --password "<store-in-accounts-jsonc-only>" \
  --screenshot "screenshots/s0007_after-submit.png" \
  --severity S1
```

Note: arc:e2e may store reusable credentials in `accounts.jsonc`; therefore `reports/` should not be submitted to the repository.
## Gotchas

**CRITICAL: The following behaviors are FORBIDDEN in arc:fix execution:**

### Diagnosis Anti-Patterns

- **Shotgun Debugging**: Randomly changing code hoping something works — fix root causes, not symptoms
- **Symptom Treatment**: Patching error messages without understanding underlying cause — guarantees regression
- **Cache Blame**: Assuming cache is wrong without verification — check timestamps and hashes first
- **Log Ignorance**: Not reading full error context before diagnosing — missing critical clues

### Fix Anti-Patterns

- **Refactor-While-Fix**: Refactoring unrelated code while fixing a bug — scope creep causes new bugs
- **Type Suppression**: Using `as any`, `@ts-ignore`, `@ts-expect-error` to silence errors — forbidden
- **Test Deletion**: Deleting failing tests to make suite pass — never acceptable
- **Skip Shortcuts**: Commenting out code or adding `if(false)` to skip logic — proper fix required

### Verification Anti-Patterns

- **Premature PASS**: Declaring fix complete before re-running failed test — must verify
- **Partial Verification**: Only testing the fixed path, not related paths — regression risk
- **Context Skip**: Not reading `.arc/e2e/` failure report before triage — missing context
- **Session Waste**: Starting fresh instead of continuing with `task_ref` — loses diagnosis progress

## Sign-off

```text
files changed:    N (+X -Y)
scope:            on target / drift: [what]
hard stops:       N found, N fixed, N deferred
signals:          N noted
verification:     [command] → pass / fail
```
