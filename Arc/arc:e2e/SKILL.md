---
name: arc:e2e
description: "真实路径 E2E 验证与证据沉淀；当用户说“端到端测试/用户流程回归/e2e test/browser journey”时触发。"
---
# arc:e2e — evidence-based E2E validation

## Overview

`arc:e2e` validates real user journeys through the UI and preserves auditable evidence. It is capable of intelligently selecting the most appropriate browser automation tool based on task requirements:
- **`chrome-cdp` (`mcp_chrome-devtools_*`)**: A lightweight local debugging tool that connects via WebSocket to an existing Chrome browser. Best for single-point validation, debugging already-open pages, and reusing existing login states.
- **`agent-browser` (Skill)**: A full-featured AI agent browser framework that manages its own browser lifecycle and session isolation. Best for complex end-to-end automation, multi-step flows (like login), and independent environments.

By choosing the right tool, it follows the actual interaction path, captures screenshots and action logs, and produces a reproducible PASS/FAIL report without bypassing the UI.

## Quick Contract

- **Trigger**: It is necessary to verify the end-to-end business flow according to the real user path and retain UI evidence.
- **Inputs**: `test_objective`, `personas`, `target_url`, optional validation and output parameters.
- **Outputs**: `report.md`, `action-log.md`, `screenshot-manifest.md`, `screenshots/`, and account files.
- **Quality Gate**: The artifact integrity and format check of `## Quality Gates` must be passed before the result is declared.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:e2e`, which will perform E2E on the real user path and precipitate auditable evidence."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO E2E PASS CLAIM WITHOUT UI EVIDENCE AND ARTIFACT INTEGRITY
```

Without UI evidence and artifact integrity verification, it shall not be declared passed.

## Workflow

1. Obtain the test target, account role and running parameters and create a file.
2. Execute business flow according to UI operation path and record events and screenshots.
3. Summary reporting, verifying artifact integrity and Markdown formatting.
4. Output PASS/FAIL evidence, reflowing `arc:fix` if necessary.

## Quality Gates

- Each run must produce `run_id` and a directory of standardized artifacts.
- The failed steps in the report must be mapped to screenshot/log evidence.
- Account and session switching must comply with session isolation rules.
- All Markdown products must pass format verification.
- Chat delivery should keep only compact FAIL/PASS snapshots, step summaries, or artifact inventories; when these are naturally tabular, prefer `terminal-table-output` instead of Markdown tables.

## Expert Standards

- Scenario design aligns `ISTQB` terminology and test levels, covering the main path, exception path, and boundary path.
- Key business flows must define the `E2E assertion golden thread` (the four categories of status, event, UI, and log are consistent).
- Authentication and permission flows need to be aligned with `OWASP ASVS` key control points for verification.
- Accessibility related pages should check the applicable `WCAG 2.2 AA` basics (focus, contrast, semantics) when those pages are in scope.
- When stability is under review, the report must include flaky-rate observations and the resulting judgment.

## Scripts & Commands

- Run directory scaffolding: `python3 Arc/arc:e2e/scripts/scaffold_run.py --run-id <run_id> --target-url <url>`
- Convert accounts to personas: `python3 Arc/arc:e2e/scripts/accounts_to_personas.py --accounts-file <accounts.jsonc> --out <personas.json>`
- Artifact check: `python3 Arc/arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --strict`
- Artifact check (with visual baselines): `python3 Arc/arc:e2e/scripts/check_artifacts.py --run-dir <run_dir> --check-baselines --strict`
- Report compilation: `python3 Arc/arc:e2e/scripts/compile_report.py --run-dir <run_dir> --in-place --beautify-md`
- Defect registration: `python3 Arc/arc:e2e/scripts/new_defect.py --run-dir <run_dir> --title \"<defect_title>\"`
- Visual diff (single file): `python3 Arc/arc:e2e/scripts/visual_diff.py --baseline <base.png> --current <curr.png> --json --fail-on-diff`
- Baseline init: `python3 Arc/arc:e2e/scripts/baseline_manager.py init --run-dir <run_dir> --baseline-dir baselines/<proj>`
- Baseline compare: `python3 Arc/arc:e2e/scripts/baseline_manager.py compare --run-dir <run_dir> --baseline-dir baselines/<proj> --fail-on-diff`
- Baseline update: `python3 Arc/arc:e2e/scripts/baseline_manager.py update --run-dir <run_dir> --baseline-dir baselines/<proj> --files <file1,file2> --reason "<reason>"`
- Contrast check: `python3 Arc/arc:e2e/scripts/contrast_check.py --input <screenshot.png> --json`
- Runtime main command: `arc e2e`

## Red Flags

- Bypassing UI action links via API/scripting.
- Report conclusions are inconsistent with screenshots/incident records.
- `accounts.jsonc` is missing or critical artifacts continue to be delivered.
- Performing write operations to the database without authorization.

## When to Use

- **Primary Trigger**: E2E verification and UI evidence need to be precipitated according to the real user path.
- **Typical scenario**: Cross-account/cross-role process, key status transfer and screenshot evidence collection.
- **Boundary Tip**: Please refer to `arc:fix` for failure root cause location and repair.

## Context Budget (avoid Request too large)

- By default, only **1** `run_id` (corresponding to **1** `run_dir`) is run/analyzed at a time.
- Do not paste the complete `report.md` / `events.jsonl` / large section of the log into the conversation; only provide: **FAIL row** in the failed step table + corresponding `screenshots/...png` + a small amount of necessary key information.
- If you encounter `Request too large`: Divide the work into multiple small rounds (for example: first determine the failure step you want to see → then add evidence for that step → then repair/verify).

## Helper Scripts Available

> Follow the pattern of `webapp-testing/`: first call `scripts/` directly as a black box; run `--help` first and then decide whether you need to read the source code.

- `scripts/scaffold_run.py`: One-click generation of `<report_output_dir>/<run_id>/` directory skeleton + report/artifact template (supports pack: `e2e` / `full-process`)
- `scripts/compile_report.py`: Compile and output `action-log.compiled.md` / `screenshot-manifest.compiled.md` from `events.jsonl`; supports table backends and column width control such as `tabulate`/`py-markdown-table`/`pandas(df.to_markdown)`; can update the step table (auto markers) of `report.md`
- `scripts/new_defect.py`: generates `failures/failure-XXXX.md` and can be appended to `execution/defect-log.md`
- `scripts/check_artifacts.py`: Quality access control (required files/directories, screenshot reference existence, accounts.jsonc parsable, JSONL parsable, etc.)
- `scripts/beautify_md.py`: Beautify existing Markdown with one click (based on `mdformat`, can fully format the run dir)
- `scripts/accounts_to_personas.py`: Generate `personas` JSON (role/user/pass/token) from `accounts.jsonc` to facilitate repeated regression runs
- `scripts/visual_diff.py`: Pixel-level screenshot comparison with diff visualization (Pillow-based, supports mask regions)
- `scripts/baseline_manager.py`: Baseline lifecycle management (init / compare / update / status)
- `scripts/contrast_check.py`: WCAG contrast ratio batch scanning on screenshots (Pillow-based)
- `scripts/js/interactability_checks.js`: Browser-injectable JS for DOM interactability and contrast verification

## Reference Files

- `templates/`: Deliverable templates and packs (the whole process of "testing company level")
- `examples/`: Scaffolding, event samples and usage examples
- `requirements.txt`: Report beautification/table tool dependency (mdformat/tabulate/py-markdown-table/pandas)
  - Installation is not forced by default: it can run with zero dependencies when `--beautify-md` is not used; it is recommended to install the required packages only when needed and in **venv** to avoid contaminating the system Python.

## **Input Arguments**

When calling this Skill, the following parameters must be explicit in the context:

1. **test_objective** (string, required)  
   * Description: The macro business goal of this test.
   * Example: "Verify the complete process from ordinary user submission of purchase requisition to manager approval"
2. **personas** (array, required)
   * Description: List of user roles involved in the test. Credentials may be supplied here so the Agent can complete a real login, but they should be copied only into `accounts.jsonc` for controlled reuse and must not be echoed back into reports/logs without redaction.
   * Format: [{"role": "buyer", "user": "u1", "pass": "p1"}, ...]
3. **target_url** (string, required)  
   * Description: Entry URL of the test environment.
4. **validation_container** (string, optional)  
   * Description: Docker container name to use for data layer validation (read-only operations only).
5. **run_id** (string, optional but recommended)  
   * Description: The unique identifier of this execution, used to associate reports and artifact directories. When not provided, the Agent must generate it itself.
   * Recommended format: `YYYY-MM-DD_HH-mm-ss_<short>`
6. **report_output_dir** (string, optional)  
   * Description: Report output root directory (either relative path or absolute path).
   * Default: `reports/`
7. **report_formats** (array, optional)  
   * Description: Report output format.
   * Default: `["markdown"]`
   * Optional: `"jsonl"` (one line of JSON for each step to facilitate machine summary)
8. **capture_screenshots** (boolean, optional)  
   * Description: Whether to force screenshots to be used as evidence artifacts.
   * Default: `true`
9. **accounts_file** (string, optional but recommended)  
   * Description: Unified account management file path (suggestion: `<report_output_dir>/<run_id>/accounts.jsonc`).
   * Constraints: Even if `personas` is provided, the final credentials used for execution should be written to the file synchronously. `report.md` should identify the role/account used, while secrets and tokens stay redacted outside `accounts.jsonc`.
   * Auxiliary: `python scripts/accounts_to_personas.py --accounts-file <...>` can be used to generate `personas` JSON for reruns.

## **Dependencies (environment dependencies)**

* **ace-tool (MCP)**: Required tool. Used to read project source code, API definitions and requirements documents before testing to accurately obtain page selectors (Selectors) and business logic.
* **Browser Automation Tools** (choose based on task):
  * **chrome-cdp** (`mcp_chrome-devtools_*`): For native, lightweight local browser automation, attaching to an already-running session.
  * **agent-browser** (Skill): For full-featured AI agent browser automation, handling isolated sessions and complex multi-step navigation.
* docker: for database read-only authentication (CLI).

## **Critical Rules**

The following constraints must be strictly adhered to when executing tests, any violation will be deemed Task Failed:

0. **Markdown Format Validation (Markdown format validation - highest priority)**
   * **Mandatory**: Format verification must be performed immediately after generating any Markdown file (`report.md`, `screenshot-manifest.md`, `action-log.md`, `failures/*.md`, etc.).
   * **Table column number alignment**: The number of columns in the **header row, separator row, and data row** of the table must be exactly the same. For example, if the header has 7 columns, the delimited rows and each row of data must have 7 columns.
   * **Delimited row format**: Each column of a delimited row must be one of `---`, `:---`, `---:`, or `:---:`, and cannot be empty or missing.
   * **Special Character Escape**: If the cell content contains `|` characters, it must be escaped to `\|`; if it contains line breaks, it must be replaced with `<br>` or split into multiple lines.
   * **Verification method (choose one)**:
     1. Use `scripts/check_artifacts.py --run-dir <run_dir> --strict` (recommended)
     2. Use `mdformat --check <file.md>` (mdformat needs to be installed)
     3. Manual check: Count the number of columns in tables one by one to ensure that the number of table headers, separated rows, and all data rows and columns is the same
   * **If the verification fails, repair it before continuing**: If a format error is found, it must be repaired immediately and re-verified and cannot be skipped.

1. **Human Simulation**
   * **BANNED** Using curl/API scripts to bypass the UI. Must simulate click (click) and input (type).
   * **Thought Chain**: "Observation -> Thought -> Action" must be output before operation.
   * **Session Isolation**: You must click "Logout" on the UI before switching accounts.
2. **Read-Only Backend (backend read-only)**
   * **Prohibited** Modification of backend code or configuration files.
   * **BANNED** "Hand-fake data" via SQL INSERT/UPDATE/DELETE to make tests pass.
   * **Allow** to use SQL SELECT to validate the results of UI operations.
   * If you really need to perform database migration/DDL/DML (for example, application repair requires upgrading the schema or executing the official migration script): you must first obtain the user's explicit consent, and write the consent and execution evidence into `<run_dir>/db/`; without consent, it will be regarded as blocked and may not be executed without authorization.
3. **Real Update / OTA Path Integrity**
   * **BANNED** Temporarily switching OTA/update/install targets to a writable directory, changing package source paths, flipping feature flags, editing local configuration, or mutating any other underlying runtime target just to make the page button succeed.
   * **BANNED** "Change target first → publish/update → click the page button → restore the config". That bypasses the real product logic and invalidates the test.
   * If a writable sandbox or test feed is legitimately required, it must already be the declared test environment before the run starts, be user-approved, and be recorded in `report.md`; it must not be silently switched during the verification flow.
4. **Protected Credential Handling**
   * **Mandatory**: Sensitive values such as passwords, tokens, and session identifiers may be stored in `accounts.jsonc` when required for reproducible reruns, but they must be redacted in delivery-facing logs, reports, and failure summaries.

5. **Accounts File (must be generated by unified account management file)**
   * **Mandatory**: The account/credential set actually used in this test must be recorded in `<report_output_dir>/<run_id>/accounts.jsonc`.
   * **Mandatory**: If you must create a new account to verify the repair, you must mark `created_for_verification=true` in `accounts.jsonc` and indicate the reason and time, and explain it in the `Account Changes` section of `report.md`.

6. **Report Artifacts (report artifacts must be produced)**
   * **Mandatory**: Each business flow test, regardless of PASS/FAIL, must generate a deliverable test result document (see Phase 4).
   * **Mandatory**: All screenshots in the report must give **exact path + file name + image description** (see Screenshot Manifest).

7. **Do Not Commit Secrets**
   * Since this Skill may require credentials to be stored in `accounts.jsonc` for reruns, the generated `reports/` (or the output directory you specify) must not be submitted to the code repository.
   * `.gitignore` already in this Skill directory will ignore output directories such as `reports/` by default (but still subject to team constraints and review).

8. **Resource Control & Cleanup (resource control and timely shutdown)**
   * All waits/retries must have an upper limit (timeout/max attempts), and infinite loops or infinite waits are prohibited.
   * It is forbidden to start a background process that "does not exit after running" (such as a resident tail/listener) without closing it at the end.
   * If external tools such as tmux/container/browser are used: exit/close in time (such as detach + kill session) after this run is completed to avoid resource leakage and long-term occupation.

## **Instructions (execution process)**

This section has been moved to a reference file to reduce context bloat.

👉 **Please see [Detailed Execution Instructions](references/execution-instructions.md) for the full details.**

## **Output Schema (log specification)**

This section has been moved to a reference file to reduce context bloat.

👉 **Please see [Output Schema and Log Specifications](references/output-schema.md) for the full details.**

# E2E UI/UX Simulation Report

## Run Metadata
* **Run ID**: `<run_id>`
* **Objective**: `<test_objective>`
* **Target URL**: `<target_url>`
* **Start Time**: `YYYY-MM-DD HH:MM:SS`
* **End Time**: `YYYY-MM-DD HH:MM:SS`
* **Result**: `PASS|FAIL`

## Accounts Used (redacted in report)
> Record the actual role/account identifiers used for this run. Keep passwords, tokens, and session identifiers only in `accounts.jsonc`.
* **buyer**: user=`buyer_01` pass=`[redacted]` token=`[redacted]`
* **manager**: user=`manager_01` pass=`[redacted]` token=`[redacted]`

## Scenario Outline
1. Login buyer -> create request
2. Logout -> login manager -> approve
3. Verify final status

## Step-by-step Execution
> Each step must have: operation, expectation, actual, conclusion, evidence (screenshot path).

| Step | Role | Action | Expected | Actual | Evidence | Result |
|------|------|--------|----------|--------|----------|--------|
| 0001 | buyer | open `/login` | Login form visible | Login form visible | `screenshots/s0001_login-page.png` | PASS |
| 0002 | buyer | type username/password | Fields filled | Fields filled | `screenshots/s0002_filled-form.png` | PASS |
| 0003 | buyer | click `#btn-submit` | Redirect to dashboard | Redirected to dashboard | `screenshots/s0003_dashboard.png` | PASS |

## Screenshot Manifest
See: `screenshot-manifest.md`

## DB Verification (Optional)
* Query: `db/query-0001.txt`
* Result: `db/result-0001.txt`

## Failure Summary (Only if FAIL)
* Primary failure: `<one-line>`
* See: `failures/failure-0001.md`
```

### **1. Action Log (Standard, action-log.md and/or stdout)**

```markdown
[ANALYSIS] Use ace-tool to read src/pages/Login.tsx and confirm that the login button ID is #btn-submit-v2
[PLAN] Switch to the approval manager account
[THOUGHT] Currently logged out, you need to enter the manager account credentials.
[EXEC] mcp_chrome-devtools_fill "#username" "manager_01"
[EXEC] mcp_chrome-devtools_fill "#password" "<redacted-at-report-time>"
[VERIFY] Login successful, Dashboard displays "Pending approval: 1" -> PASS
```

To ensure traceability, it is recommended to introduce step numbers and screenshot records in Action Log:

```markdown
[STEP 0007][THOUGHT] Prepare to click the submit button. A successful Toast is expected to appear and jump to the list page.
[STEP 0007][EXEC] mcp_chrome-devtools_click "#btn-submit"
[STEP 0007][EXEC] mcp_chrome-devtools_wait_for "Submission successful" 5000
[STEP 0007][EXEC] mcp_chrome-devtools_take_screenshot "<report_output_dir>/<run_id>/screenshots/s0007_after-submit.png"
[STEP 0007][VERIFY] Toast appears and the page jumps to /requests -> PASS
```

### **2. Screenshot Manifest (Mandatory, screenshot-manifest.md)**

The screenshot list must contain the exact path + file name + image description (and be consistent with the `report.md` reference).

```markdown
# Screenshot Manifest

| Step | Path | Captured At | URL | Description | Expectation | Result |
|------|------|-------------|-----|-------------|-------------|--------|
| 0001 | `screenshots/s0001_login-page.png` | YYYY-MM-DD HH:MM:SS | `<url>` | Initial state of login page: username/password input box is visible | Show login form | PASS |
| 0002 | `screenshots/s0002_filled-form.png` | YYYY-MM-DD HH:MM:SS | `<url>` | Username/password has been filled in, no login has been clicked | Field content is correct | PASS |
| 0007 | `screenshots/s0007_after-submit.png` | YYYY-MM-DD HH:MM:SS | `<url>` | After clicking Submit, a successful Toast appears, and the new record is displayed in the first row of the list page. | Toast + list update | PASS |
```
### **3. Failure Report (Mandatory on Error, failures/failure-XXXX.md)**

If the test fails, the following Markdown block must be output:
```markdown
# 🛑 E2E Test Failure Report

## Context  
* **Task**: [Current subtask name]
* **Time**: [YYYY-MM-DD HH:MM:SS]

## Debug Artifacts (sanitized for report)
* **User**: `[account_identifier]`
* **Credential Ref**: `accounts.jsonc`
* **Token**: `[redacted]`
* **URL**: `[current_url]`

## Screenshot Evidence
* **Step**: `0007`
* **Path**: `screenshots/s0007_after-submit.png`
* **Description**: After clicking submit, an error pop-up window "Insufficient Permissions" appears, and the page does not jump.

## Reproduction Steps  
1. [Operation step 1]
2. [Operation step 2]
3. [Operation that caused the error]

## Evidence  
* **UI State**: [Error message or screenshot description]
* **DB State**:   
  > Query: `SELECT status FROM orders ...`  
  > Result: `[database_query_result]`
```

### **4. Optional: events.jsonl (Machine-readable)**

When `report_formats` contains `"jsonl"`, each step must append a line of JSON containing at least these fields:

```json
{"run_id":"<run_id>","step":1,"role":"buyer","kind":"exec","cmd":"mcp_chrome-devtools_navigate_page \"<target_url>/login\"","ts":"YYYY-MM-DDTHH:MM:SS","result":"PASS"}
{"run_id":"<run_id>","step":1,"role":"buyer","kind":"screenshot","path":"screenshots/s0001_login-page.png","description":"Login page initial state","ts":"YYYY-MM-DDTHH:MM:SS"}
```
## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:e2e execution:**

### Test Execution Anti-Patterns

- **Screenshot Skipping**: Failing to capture screenshots at required key steps — reports are incomplete without visual evidence
- **Selector Guessing**: Using guessed selectors without verification — causes flaky tests
- **Happy Path Only**: Testing only success scenarios — must include error cases and edge conditions
- **State Blindness**: Ignoring page state before actions — causes cascade failures

### Evidence Anti-Patterns

- **Missing Manifest**: Missing `screenshot-manifest.md` or any compiled manifest used for delivery — breaks report traceability
- **Broken References**: Screenshot filenames not matching manifest entries — orphaned evidence
- **JSONL Corruption**: Malformed `events.jsonl` entries — breaks compilation
- Temporarily changing OTA/update targets, writable directories, feed URLs, or local config so the UI button can "pass", then restoring the original value afterward

### Report Anti-Patterns

- **Preliminary Conclusion**: Marking test PASS before `check_artifacts.py --strict` validation — premature success declaration
- **Failure Suppression**: Skipping failed steps instead of recording them — hides real issues
- **Context Ignorance**: Not reading CLAUDE.md for expected behavior — tests wrong things
