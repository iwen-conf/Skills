---
name: "arc:e2e"
description: "需要按真实用户路径做 E2E 验证时使用：自动执行流程并输出截图与证据工件。"
---
# **UI/UX Simulation & E2E Testing**

## Overview

This Skill gives Agent the ability of "Senior Automation Test Engineer". Agent will use the agent-browser binary tool to simulate the thinking logic and operating habits of real human users, and perform high-fidelity business flow closed-loop testing.

## Quick Contract

- **Trigger**: It is necessary to verify the end-to-end business flow according to the real user path and retain UI evidence.
- **Inputs**: `test_objective`, `personas`, `target_url`, optional validation and output parameters.
- **Outputs**: `report.md`, `action-log`, `screenshot-manifest`, `screenshots/` and account files.
- **Quality Gate**: The artifact integrity and format check of `## Quality Gates` must be passed before the result is declared.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:e2e`, which will perform E2E on the real user path and precipitate auditable evidence."

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

## Red Flags

- Bypassing UI action links via API/scripting.
- Report conclusions are inconsistent with screenshots/incident records.
- `accounts.jsonc` is missing or critical artifacts continue to be delivered.
- Performing write operations to the database without authorization.

## When to Use

- **首选触发**: E2E verification and UI evidence need to be precipitated according to the real user path.
- **典型场景**: Cross-account/cross-role process, key status transfer and screenshot evidence collection.
- **边界提示**: Please refer to `arc:fix` for failure root cause location and repair.

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
   * Description: List of user roles involved in the test. The account number and password must be included in **clear text** so that the Agent can simulate a real login.
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
   * Constraints: Even if `personas` is provided, the final account/password must be written to the file synchronously and listed in clear text in `report.md`.
   * Auxiliary: `python scripts/accounts_to_personas.py --accounts-file <...>` can be used to generate `personas` JSON for reruns.

## **Dependencies (environment dependencies)**

* **ace-tool (MCP)**: Required tool. Used to read project source code, API definitions and requirements documents before testing to accurately obtain page selectors (Selectors) and business logic.
* agent-browser: For browser automation (CLI).
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
3. **Plain Text Logging**
   * **Mandatory**: All sensitive information (password, Token, SessionID) in the debugging log **must be recorded in plain text** and desensitization is strictly prohibited so that developers can reproduce it directly.

4. **Accounts File (must be generated by unified account management file)**
   * **Mandatory**: The account/password/Token actually used in this test must be recorded in `<report_output_dir>/<run_id>/accounts.jsonc`.
   * **Mandatory**: If you must create a new account to verify the repair, you must mark `created_for_verification=true` in `accounts.jsonc` and indicate the reason and time, and explain it in the `Account Changes` section of `report.md`.

5. **Report Artifacts (report artifacts must be produced)**
   * **Mandatory**: Each business flow test, regardless of PASS/FAIL, must generate a deliverable test result document (see Phase 4).
   * **Mandatory**: All screenshots in the report must give **exact path + file name + image description** (see Screenshot Manifest).

6. **Do Not Commit Secrets**
   * Since this Skill requires clear text recording of account number/password/Token, the generated `reports/` (or the output directory you specify) must not be submitted to the code repository**.
   * `.gitignore` already in this Skill directory will ignore output directories such as `reports/` by default (but still subject to team constraints and review).

7. **Resource Control & Cleanup (resource control and timely shutdown)**
   * All waits/retries must have an upper limit (timeout/max attempts), and infinite loops or infinite waits are prohibited.
   * It is forbidden to start a background process that "does not exit after running" (such as a resident tail/listener) without closing it at the end.
   * If external tools such as tmux/container/browser are used: exit/close in time (such as detach + kill session) after this run is completed to avoid resource leakage and long-term occupation.

## **Instructions (execution process)**

### **Phase 0: Context Acquisition (Requirement Analysis)**

**Before starting any test, if you are not clear about the page structure, element selector (Selector) or specific business flow logic, you must obtain the context according to the following priorities: **

**Priority 0: Read shared context index (`.arc/context-hub/index.json`)**

1. Prioritize searching for the following products:
   * `codemap.md` (catalog responsibility and critical path)
   * `arc:audit` Snapshot/Diagnostic Report (Known Risk Points)
   * score output (generated by `score/` module, high risk dimension)
   * `arc:build` handoff (range of changes in this round)
2. Verify product freshness: `expires_at` + `content_hash`.
3. If the product is available, load it directly without repeating the full scan.
4. If the product fails, it will be updated according to `refresh_skill` reflow (`arc:init:update` / `arc:cartography` / `score` module refresh (triggered by `arc:release` arrangement) / `arc:audit`).

**Priority 1: Read project CLAUDE.md level index**

1. **Scan CLAUDE.md**: Use `find . -name "CLAUDE.md" -type f` to scan the hierarchical index of the project.
2. **Extract key information**:
   * **Root level CLAUDE.md**: project technology stack, running command, front-end entry path
   * **Module-level CLAUDE.md** (such as `frontend/CLAUDE.md`):
     - "Entry and Startup" Chapter: Front-end startup command, development server port
     - "External Interface" chapter: page routing, component selector mode, common data-testid specifications
     - "Coding Conventions" chapter: Selector naming convention (such as `button[data-testid="{action}-{component}"]`)
     - "Architecture Diagram" Chapter: Page Structure and Component Hierarchy
3. **Verify index freshness**: Check the "Change History" section of CLAUDE.md to confirm that the generation time is < 7 days.
4. **If the index is missing or out of date**: trigger `arc:init:update` (`arc:init:full` if necessary) before continuing.

**Priority 2: Use ace-tool to add details**

When CLAUDE.md provides insufficient information (such as lack of specific selectors and unclear page logic):

1. **Call ace-tool**: Read the project code base (especially front-end routing, component definition) and requirements analysis documents.
2. **Identify Elements**: Confirm the ID, Class or text identification of key interactive elements (buttons, input boxes) to avoid blindly guessing the selector.
3. **Understand Logic**: Understand the pre- and post-conditions of the business (for example: order status flow rules) and ensure that the test path conforms to the real business logic.

**Priority 3: Cache verification and error reporting**

During the test execution, if the information in CLAUDE.md is found to be inaccurate (such as the selector does not exist, the page structure changes):

1. **Flag Errors Immediately**: Document what is expected vs. what is actually happening.
2. **Return to ace-tool**: Use source code scanning to obtain correct information and continue testing.
3. **Generate error report**: Generate a cache verification failure report in the `<run_dir>/context-errors/` directory (see template below).
4. **Reflow update suggestions**:
   - CLAUDE index problem → `arc:init:update`
   - codemap issues → `arc:cartography` updated
   - Rating/review product issues → `score` module refresh (triggered by `arc:release` arrangement) / `arc:audit` update

**Cache Error Report Template** (`<run_dir>/context-errors/cache-error-YYYYMMDD-HHMMSS.md`):

```markdown
# Cache verification failure report

**Generation time**: <ISO 8601 timestamp>
**Run ID**: <run_id>
**Test step**: Step <step_number>

## Error details

- **Cache Source**: `<path-to-CLAUDE.md>`
- **Chapter**: <Chapter Name>
- **What to expect**:
  ```
<selector or information extracted from CLAUDE.md>
  ```
- **Actual situation**:
  - The selector does not exist in the page/The page structure has changed
  - Actual selector: `<actual-selector>`
  - Page URL: <current-url>
  - Discovery time: <ISO 8601 timestamp>

## Scope of influence

- Current test: Fallback to ace-tool scan, test execution continues
- Other tests: may affect all test cases that rely on this selector

## Suggested fix

1. **Fix Now** (Recommended):
   ```bash
   arc init --project-path <project-path>
   ```

2. **Manual Repair**:
Edit `<path-to-CLAUDE.md>`, update relevant chapters

## temporary patch

Already used ace-tool to get the correct information:
```
<ace-tool search result summary>
```
```

### **Phase 1: Strategy & Planning**

1. Based on the analysis of Phase 0, test_objective is disassembled into ordered sub-task queues (Sub-tasks).
2. Plan the complete user path: Login A -> Action -> Logout -> Login B -> Verify.

3. Evidence Plan:
   * Key nodes that must be screenshotted: after logging in, before and after clicking the key submit button, when a Toast/error pop-up window appears, and on the final results page.
   * Validation that must be done: UI validation text/element status; DB SELECT results if necessary.

### **Phase 2: Execution Loop**

Execute the following loop for each subtask:

1. **Check**: Confirm the current page status.
2. **Action**: Use agent-browser to perform atomic operations.
   * Command: agent-browser open|click|type|wait|screenshot ...
3. **Wait**: Wait for UI response (Loading ends, Toast appears).
4. **Verify (UI)**: Check page text or element status.
5. **Capture Evidence (Mandatory if capture_screenshots=true)**:  
   * Key nodes must be screenshotted and recorded **immediately**: screenshot absolute/relative path, file name, image description, current URL, and corresponding step number.

### **Phase 3: Deep Verification (Conditional)**

When UI feedback is unclear or data consistency needs to be confirmed:

* Execution: docker exec -t <container> <db_cmd> -e "SELECT ..."
* Verify that database fields are as expected.

### **Phase 4: Report & Artifacts (Mandatory)**

After each business flow test is completed (even if it fails midway), a test report and artifact directory must be generated.

**Output root directory**: `<report_output_dir>/<run_id>/`
If `run_id` is not provided, the Agent must generate and print it explicitly in the report.

**Directory structure (REQUIRED)**:

```text
<report_output_dir>/<run_id>/
  accounts.jsonc
  report.md
  action-log.md
  screenshot-manifest.md
  screenshots/
    s0001_login-page.png
    s0002_filled-form.png
  failures/
    failure-0001.md
  db/
    query-0001.txt
    result-0001.txt
  events.jsonl        (optional; when report_formats includes "jsonl")
```

> Note: The `db/` directory is recommended to always exist (even if it is empty) so that `scripts/check_artifacts.py --strict` can pass; the empty directory can be kept when no DB verification is done.

**Account management (required)**:

- The account/password/Token used in this test must be written in `accounts.jsonc` (unified source) and listed in clear text in the `Accounts & Secrets (PLAIN TEXT)` section of `report.md`.
- If you "must create a new account" for verification and repair (such as verifying first login, permission boundaries, new tenant isolation), you must:
  - Mark the account with `created_for_verification=true` in `accounts.jsonc` and write `why/created_at`
  - Explain "Why a new account is generated" in the `Account Changes` section of `report.md`

**Derived artifacts (generated by scripts, recommended to be retained)**:

- `scripts/scaffold_run.py`: Create `<report_output_dir>/<run_id>/` and `screenshots/`, `failures/`, `db/` (optional creation of `events.jsonl`)
- `scripts/compile_report.py`：
  - Output: `action-log.compiled.md`, `screenshot-manifest.compiled.md`
  - Report: `report.generated.md` is generated by default; if `--in-place` is used, the auto blocks of `report.md` are updated
- `scripts/beautify_md.py`: Format run_dir under Markdown (optional)

**Path specification (to avoid tool misjudgment)**:

- When referencing screenshots in `report.md` / `screenshot-manifest.md` / `failures/*.md`, the path must use the relative path `screenshots/...` and wrap it in backticks, for example: `screenshots/s0007_after-submit.png` (`check_artifacts.py` will parse and verify these paths).

**Screenshot naming rules (REQUIRED)**:

* Filename must be traceable to step number: `s<step_id>_<slug>.png`
  - `step_id` is a 4-digit number, such as `0007`
  - `slug` is short English/number/hyphen, for example `after-submit`
* Example: `screenshots/s0007_after-submit.png`

**Image Description Required (REQUIRED)**:

Each screenshot must contain in `screenshot-manifest.md`:
`step_id` / `path` / `captured_at` / `url` / `description` / `expectation` / `result(PASS/FAIL)`。

## **Output Schema (log specification)**

The output of this Skill is divided into two categories:

1) **Real-time log (stdout)**: used to watch while running.
2) **Placement report (artifacts)**: used for delivery, playback, and reproduction. **Must generate**.

The standardized Schema and templates are given below.

### **0. Run Report (Mandatory, report.md)**

`report.md` must contain the following chapters (the order is recommended to be fixed to facilitate diff and machine parsing):

```markdown
# E2E UI/UX Simulation Report

## Run Metadata
* **Run ID**: `<run_id>`
* **Objective**: `<test_objective>`
* **Target URL**: `<target_url>`
* **Start Time**: `YYYY-MM-DD HH:MM:SS`
* **End Time**: `YYYY-MM-DD HH:MM:SS`
* **Result**: `PASS|FAIL`

## Personas & Secrets (PLAIN TEXT)
> According to the iron law, the following information must be recorded clearly for development and reproduction.
* **buyer**: user=`buyer_01` pass=`secret123` token=`<access_token>`
* **manager**: user=`manager_01` pass=`secret456` token=`<access_token>`

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
[THOUGHT] Currently logged out, you need to enter your manager account password.
[EXEC] agent-browser type "#username" "manager_01"  
[EXEC] agent-browser type "#password" "secret123"  
[VERIFY] Login successful, Dashboard displays "Pending approval: 1" -> PASS
```

To ensure traceability, it is recommended to introduce step numbers and screenshot records in Action Log:

```markdown
[STEP 0007][THOUGHT] Prepare to click the submit button. A successful Toast is expected to appear and jump to the list page.
[STEP 0007][EXEC] agent-browser click "#btn-submit"
[STEP 0007][EXEC] agent-browser wait "text=Submission successful" 5000
[STEP 0007][EXEC] agent-browser screenshot "<report_output_dir>/<run_id>/screenshots/s0007_after-submit.png"
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

## Debug Artifacts (PLAIN TEXT)  
* **User**: `[plaintext_account]` / `[plaintext_password]`
* **Token**: `[plaintext_access_token]`
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
{"run_id":"<run_id>","step":1,"role":"buyer","kind":"exec","cmd":"agent-browser open \"<target_url>/login\"","ts":"YYYY-MM-DDTHH:MM:SS","result":"PASS"}
{"run_id":"<run_id>","step":1,"role":"buyer","kind":"screenshot","path":"screenshots/s0001_login-page.png","description":"Login page initial state","ts":"YYYY-MM-DDTHH:MM:SS"}
```
## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:e2e execution:**

### Test Execution Anti-Patterns

- **Screenshot Skipping**: Failing to capture screenshots at each step — reports are incomplete without visual evidence
- **Selector Guessing**: Using guessed selectors without verification — causes flaky tests
- **Happy Path Only**: Testing only success scenarios — must include error cases and edge conditions
- **State Blindness**: Ignoring page state before actions — causes cascade failures

### Evidence Anti-Patterns

- **Missing Manifest**: Not updating `screenshot-manifest.compiled.md` — breaks report traceability
- **Broken References**: Screenshot filenames not matching manifest entries — orphaned evidence
- **JSONL Corruption**: Malformed `events.jsonl` entries — breaks compilation

### Report Anti-Patterns

- **Preliminary Conclusion**: Marking test PASS before `check_artifacts.py --strict` validation — premature success declaration
- **Failure Suppression**: Skipping failed steps instead of recording them — hides real issues
- **Context Ignorance**: Not reading CLAUDE.md for expected behavior — tests wrong things
