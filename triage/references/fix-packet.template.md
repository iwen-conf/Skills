# UI/UX Defect Fix Packet（交付模板）

> 目标：把“修复了”变成可审计、可复现、可回归的交付件。建议直接在最终回复中按此模板输出。

## 1) Summary

- **Issue**: <一句话描述用户可感知的失败>
- **Impact**: <影响范围/角色/业务链路>
- **Severity**: <S0/S1/S2/S3>
- **Status**: FIXED|MITIGATED|WONTFIX|NEEDS-FOLLOWUP

## 2) Evidence (Before / After)

- **Failing run**: `run_id=<...>` `run_dir=<...>`
  - Accounts: `<run_dir>/accounts.jsonc`
  - Key screenshot(s): <paths>
  - Key log(s): <paths>
- **Passing run**: `run_id=<...>` `run_dir=<...>`
  - Accounts: `<run_dir>/accounts.jsonc`
  - Key screenshot(s): <paths>

## 3) Minimal Reproduction

- **Entry URL**: <...>
- **Persona / Role**: <...> (user/pass/token must be recorded as-is per arc:simulate rules)
- **Steps**:
  1. ...
  2. ...
  3. ...
- **Expected**: ...
- **Actual**: ...

## 3.1) Accounts & Test Data Notes (Required)

- **Canonical account file(s)**:
  - Fail run: `<run_dir>/accounts.jsonc`
  - Pass run: `<run_dir>/accounts.jsonc`
- **If NEW accounts were created for verification**: explain why (e.g., first-login, permission boundary, new tenant isolation) and record them in `accounts.jsonc` with `created_for_verification=true`.

## 4) Root Cause

- **What broke**: <具体到模块/函数/条件/选择器>
- **Why it broke now**: <变更/数据/时序/依赖>
- **Contributing factors**: <缺少断言/缺少稳定选择器/竞态/缓存>

## 5) Fix

- **Code changes**:
  - <file path>: <what changed + why>
  - <file path>: <what changed + why>
- **Non-code changes** (if any): <config/migration/feature flag>

### DB Change Control (Only if any)

- **User approval**: REQUIRED before any migration/DDL/DML. Paste the approval evidence path(s).
- **Plan**: `db/migration-plan.md` (what/why/impact/rollback)
- **Execution evidence**: `db/migration-execution.md` + any logs/outputs

### Guardrails (Must confirm)

- [ ] 未通过注释/短路/删除鉴权/授权逻辑来让流程“通过”
- [ ] DEBUG 日志可控（环境变量/配置开关），不永久污染正常日志
- [ ] 修复不依赖“手工点击/手工改数据”的隐含前提

## 6) Verification

- **Regression command(s)**: <如何跑 arc:simulate / 如何触发最小复现>
- **Artifacts validated**: `check_artifacts.py --strict` 通过（或解释为何不可用）
- **Result**: PASS

## 7) Risk / Rollback / Follow-ups

- **Risk**: <可能影响的其他路径>
- **Rollback**: <如何回退/开关/配置撤销>
- **Follow-ups**:
  - <补单测/补监控/补 data-testid/补文档>
