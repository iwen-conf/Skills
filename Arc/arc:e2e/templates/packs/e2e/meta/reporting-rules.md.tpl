# Reporting Rules (No Fluff, No Ambiguity)

## 禁止（直接视为无效信息）
- “可能/也许/大概/疑似/看起来/似乎/应该/差不多/正常/有点慢/偶尔”
- “页面没反应/按钮没用”（必须补充：点击目标、页面状态、期待结果、实际结果、证据）

## 必须（缺一不可）
- **Expected**：写“可验证”的条件（精确 UI 文案/元素状态/URL/列表行内容/数值）
- **Actual**：写“观测到”的事实（精确 UI 文案/元素状态/URL/列表行内容/数值）
- **Evidence**：截图/日志/DB 证据路径必须可打开（相对 run dir）
- **Repro**：失败时必须补齐可复现步骤（角色、前置条件、具体点击/输入）
- **Accounts**：本次测试使用的账号密码必须写入 `accounts.jsonc`，并在 `report.md` 中明文列出（用于开发复现）
- **Change Control**：若涉及数据库迁移/DDL/DML，必须先获得用户明确同意，并把同意与执行证据写入 `db/`
- **Artifacts Gate**：在最终交付前运行 `scripts/check_artifacts.py --strict`；`screenshot-manifest.md` 的 Path 必须是反引号包裹的 `screenshots/...`，不要保留 `<fill: ...>` 占位符

## 建议格式（Action Log）
```text
[STEP 0001][OBS] 当前 URL=...；页面存在 <element>; 未登录
[STEP 0001][THOUGHT] 需要登录 buyer，预期跳转 /dashboard 并出现 Toast "登录成功"
[STEP 0001][EXEC] agent-browser type "#username" "buyer_01"
[STEP 0001][EXEC] agent-browser type "#password" "secret123"
[STEP 0001][EXEC] agent-browser click "#btn-submit"
[STEP 0001][EXEC] agent-browser screenshot "screenshots/s0001_after-login.png"
[STEP 0001][VERIFY] URL=/dashboard 且页面文本包含 "待审批: 1" -> PASS
```
