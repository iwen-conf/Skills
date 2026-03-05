# E2E UI/UX Simulation Report

## Run Metadata
* **Run ID**: `{{RUN_ID}}`
* **Objective**: {{OBJECTIVE}}
* **Target URL**: `{{TARGET_URL}}`
* **Start Time**: {{START_TIME}}
* **End Time**: {{END_TIME}}
* **Result**: {{RESULT}}

## Accounts & Secrets (PLAIN TEXT)
* **Canonical account file**: `accounts.jsonc`
> 按 Skill 铁律要求，以下信息必须明文记录，用于开发复现。
{{PERSONAS_MARKDOWN}}

## Account Changes (Only if any)
> 如果为了验证修复“必须创建新账号”（例如验证首登/权限边界/新租户隔离），必须在这里写清楚原因，并同步更新 `accounts.jsonc`（created_for_verification=true）。
- _<none>_

## Environment
* **Env Name**: {{ENV_NAME}}
* **Build/Commit**: {{BUILD_REF}}
* **Browser**: chromium (headless)
* **Validation Container (optional)**: {{VALIDATION_CONTAINER}}

## Scenario Outline
1. {{SCENARIO_1}}
2. {{SCENARIO_2}}
3. {{SCENARIO_3}}

## Step-by-step Execution
<!-- AUTO_STEPS_TABLE_START -->
| Step | Role | Action | Expected | Actual | Evidence | Result |
|------|------|--------|----------|--------|----------|--------|
| 0001 | - | - | - | - | - | - |
<!-- AUTO_STEPS_TABLE_END -->

## Screenshot Manifest
详见：`screenshot-manifest.md`（或 `screenshot-manifest.compiled.md`）

## DB Verification (Optional)
* Container: `{{VALIDATION_CONTAINER}}`
* Evidence: `db/`

## DB Change Control (Only if any)
> 默认只允许只读 SELECT。若需要执行数据库迁移/DDL/DML，必须先获得用户明确同意，并把证据写入：
> - Approval: `db/migration-approval.md`
> - Plan: `db/migration-plan.md`
> - Execution evidence: `db/migration-execution.md` / `db/`
- _<no migrations executed>_

## Defects / Failures
<!-- AUTO_FAILURE_SUMMARY_START -->
- _No failures recorded._
<!-- AUTO_FAILURE_SUMMARY_END -->

## Notes
- Generated at: {{GENERATED_AT}}
- Report pack(s): {{PACKS}}
