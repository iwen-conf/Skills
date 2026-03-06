---
name: arc:fix
description: "故障修复闭环：定位根因、实施修复并回归验证；当用户说“线上故障/bug 修复/incident triage/fix broken flow”时触发。"
---

# UI/UX defect troubleshooting and repair (based on arc:e2e, industrialization)

## Overview

Convert arc:e2e failure artifacts or online fault clues into: minimum recurrence/stop loss → root cause location → repair and recovery → passing evidence.

Unified mode:
- Use `arc:fix --mode retest-loop` when the service must be restarted and validated in multiple fail-fix-retest rounds.

## Quick Contract

- **Trigger**: `arc:e2e` FAIL, CI test intermittent failure, or online failure requires stop loss recovery.
- **Inputs**: Failure `run_dir` or CI failure samples, logs and time series, test targets and account context if necessary.
- **Outputs**: Root cause analysis, fix instructions, fail/pass or stability control evidence and `fix-packet`.
- **Quality Gate**: The fail-to-pass evidence chain check of `## Quality Gates` must be passed before closing the issue.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
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

## Expert Standards

- Fault severity classification must follow `SEV-1/SEV-2/SEV-3` grading with defined response SLAs.
- Root cause analysis needs to be combined with `5 Whys + Fault Tree` to avoid staying at the superficial causes.
- Repair verification must quantify `MTTA/MTTR` with post-recovery stability window (e.g. 24h/72h).
- It is recommended to use `canary/feature flag` first and bind the rollback threshold for online repair.
- Case closing must produce `Blameless Postmortem` and CAPA (Corrective/Preventive Action) items.

## Scripts & Commands

- Triage summary: `python3 arc:fix/scripts/triage_run.py <run_dir> --json-out <triage.json> --md-out <triage.md>`
- Runtime main command: `arc fix`
- Multiple loop regression mode: `arc fix --mode retest-loop`
- Recommended link: `arc e2e` (produce evidence) → `arc fix` (fix closed loop)

## Red Flags

- Only a "repaired" description is given, with no evidence of recurrence.
- Avoid failures by deleting authentication/authorization logic.
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
python arc:fix/scripts/triage_run.py <run_dir>
```

Triage decision tree (for quick judgment of "product defects vs test false positives vs environment/data/flake"): see `references/triage-decision-tree.md`.

## Artifacts & Paths (where to put documents/files)

All evidence is rooted in `run_dir` of arc:e2e:

- **Failing run**:`reports/<fail_run_id>/`
- **Passing run**:`reports/<pass_run_id>/`
- **Accounts file (unified account management)**: `<run_dir>/accounts.jsonc` (plain text account/password/Token; not allowed to be submitted to the database)

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
python arc:fix/scripts/triage_run.py <run_dir> \
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
     1. `python arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict` (recommended)
     2. `mdformat --check <file.md>` (mdformat needs to be installed)
     3. Manually count the number of columns table by table
   - **Verification failure must be repaired before continuing** and cannot be skipped.

1. **It is prohibited to bypass permission (authentication/authorization) verification to "repair"**
   - The "permissions" here refer to **business permissions/authentication/authorization** (not file system chmod/chown).
   - It is forbidden to comment/delete/short-circuit permission verification in the code (for example, change "no permission should fail" to directly return success, or remove the judgment that roles/ACLs are required).
   - If the problem is indeed a defect in permission rules/role mapping/policy configuration: Fix it to **in line with business expectations**, and make it back to cover both sides of the "should be allowed/should be denied" boundary.

2. **Lots of DEBUG output (forced)**
   - During the troubleshooting process, a large number of logs starting with `DEBUG:` must be output to the terminal, covering: input, key branches, key variables, external request/response summary, and exception stack.
   - Prioritize adding **controllable** DEBUG logs to your code (it is recommended to use environment variables/configuration switches, such as `DEBUG_UI_UX_FIX=1`) to avoid permanently contaminating normal logs.

3. **Evidence driven**
   - "Feeling fixed" is not allowed: evidence must be given for each fix (at least one failed reproduction run + one regression via the run_id/report path of the run).

4. **Comply with arc:e2e's log and artifact specifications**
   - If you need to generate/update a report, give priority to using the script under `arc:e2e/scripts/` instead of creating your own format.

5. **DB Migration/DDL/DML Change Control**
   - Any database migration/DDL/DML (including but not limited to migrate, ALTER, INSERT/UPDATE/DELETE, backfill data) must first obtain the user's explicit consent.
   - Before agreeing: only read-only verification (SELECT) and code troubleshooting can be done; "migrate first and then add tickets" is not allowed.
   - After consent: Write the consent and implementation evidence into `run_dir/db/` (see suggestion document above) and clearly state it in the final Fix Packet.

6. **New accounts (used to verify repairs) must be auditable**
   - If a new account must be created in order to verify the repair (such as verifying first login, permission boundaries, new tenant isolation), you must write:
     - `accounts.jsonc`: Mark the account with `created_for_verification=true` and write `why/created_at`
     - `report.md`: `Account Changes` Chapter explains "Why a new account is generated"

7. **Resource Control & Cleanup (resource control and timely shutdown)**
   - Running "unlimited" retry/regression loops is prohibited; must have `max_iterations`/timeout.
   - If additional tools (browser recording, long-term tail, temporary container, tmux session, etc.) are started for troubleshooting, close them promptly after obtaining the evidence to avoid resource leakage and long-term occupation.

## Workflow (recommended to execute in order)

### 0) If run_dir is missing: run arc:e2e first to produce failure artifacts

- Create a directory skeleton with scaffolding (recommended `--pack full-process`, at least implicitly including `e2e`):
  - `python arc:e2e/scripts/scaffold_run.py --help`
  - `python arc:e2e/scripts/scaffold_run.py --pack full-process --objective "<objective>" --target-url "<url>" --personas "<json-or-path>"`
- Press `arc:e2e/SKILL.md` to execute the test to ensure disk placement:
  - `report.md`,`action-log.md`,`screenshot-manifest.md`,`screenshots/`
-(optional)`events.jsonl`
- After failure, use the output `run_dir` as the input of this Skill to continue with the subsequent steps.

### 1) Obtain failure evidence and compile a report

- If `run_dir` already exists: verify and compile first
  - `python arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python arc:e2e/scripts/compile_report.py --run-dir <run_dir> --in-place` (optional: add `--beautify-md` if `mdformat` is already installed in venv)
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

**Priority 1: Check `.arc/arc:audit/` Architecture Analysis**
1. Find review artifacts: Check `.arc/arc:audit/<project-name>/` for the presence of an architectural analysis document
2. Verify freshness: check that `project-snapshot.md` or `diagnosis-report.md` was generated < 7 days ago
3. Extract key information: module dependencies, common defect patterns, repair strategy suggestions
4. If the review product does not exist or has expired: trigger the `arc:audit` update first and then continue.

**Priority 2: Check `.arc/arc:fix/known-patterns.md`**
1. Find historical defect patterns: read `.arc/arc:fix/known-patterns.md` if present
2. Match current failure: Compare current failure symptoms to known patterns
3. Reuse repair strategy: If the match is successful, apply the verified repair solution directly

**Priority 3: Use ace-tool to search source code**
When the cache information is insufficient, use ace-tool to search the project source code to locate the root cause of the problem.

**Priority 4: Cache Verification and Error Reporting**
During the diagnostic process, if you discover that the information in `.arc/arc:audit/` or CLAUDE.md is inaccurate:
1. Tag cache errors: logging expected vs. actual
2. Falling back to source code search: use ace-tool to get the correct information
3. Generate error report: Generate cache validation failure report at `<run_dir>/analysis/context-errors/`
4. Reflow update:
   - Review data issues → `arc:audit`
   - CLAUDE index problem → `arc:init --mode update`
   - Code map issue → `arc:cartography`

**Priority 5: Update known defect pattern library**
After successfully fixing the defect, append the newly discovered defect pattern to `.arc/arc:fix/known-patterns.md`.

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

- Re-execute arc:e2e, generate a new `run_dir`, and again:
  - `python arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python arc:e2e/scripts/compile_report.py --run-dir <run_dir> --in-place` (optional: add `--beautify-md` if `mdformat` is already installed in venv)
- Which gives in the final output:
  - **Root Cause**
  - **Fix (repair point)**: involving files/modules/key logic
  - **Verification**: failed run_id + passed run_id + reporting path
  - **Risk & Follow-ups**: possible regression points, single tests/monitoring that need to be supplemented

> Final delivery template: see `references/fix-packet.template.md`.

## Defect file (optional but recommended)

When a clear failed step is located, a defect file is generated for easy tracking:

```bash
python arc:e2e/scripts/new_defect.py \
  --run-dir <run_dir> \
  --step 0007 \
--title "500 appears after clicking submit" \
  --role buyer \
  --url "http://localhost:5173/order/new" \
  --user "buyer_01" \
  --password "secret123" \
  --screenshot "screenshots/s0007_after-submit.png" \
  --severity S1
```

Note: arc:e2e allows clear text recording of account numbers/passwords; therefore `reports/` should not be submitted to the warehouse.
## Anti-Patterns

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
- **Context Skip**: Not reading `.arc/arc:e2e/` failure report before triage — missing context
- **Session Waste**: Starting fresh instead of continuing with `task_ref` — loses diagnosis progress
