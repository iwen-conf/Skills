---
name: lark-shared
version: 1.0.0
description: "Use for lark-cli setup/auth tasks: auth login/status/logout, user vs bot identity, business-domain permissions (--domain, including all/docs/drive), missing scopes, revoking authorization, or handling _notice JSON."
---

# lark-cli 共享规则

本技能指导你如何通过lark-cli操作飞书资源, 以及有哪些注意事项。

## 按需读取

本会话首次使用飞书能力时读取本入口；未变化内容无需每次重读。

- 配置缺失、身份不明、缺 scope 或登录失效：读 [配置与认证](references/auth.md)。已有有效授权直接继续。
- 写入、删除、高风险 exit 10 或本地文件输入：读 [授权与路径](references/safety-and-paths.md)。既有同范围授权持续有效。
- 业务调用显式选择 `--as user` / `--as bot`；不得静默切身份掩盖权限错误。
- 参数不确定时查当前 CLI help / method schema；同版本已确认的参数可复用。

## 更新检查

lark-cli 命令执行后，如果检测到新版本，JSON 输出中会包含 `_notice.update` 字段（含 `message`、`command` 等）。

除非用户正在询问更新、版本或 notice，否则不要把 `_notice` 原样复制为当前任务的主要答案，也不要为了 notice 中断当前任务去反复查 help。

需要稳定 JSON 给脚本或机器读取时，可以在命令前设置：

```bash
LARKSUITE_CLI_NO_UPDATE_NOTIFIER=1 LARKSUITE_CLI_NO_SKILLS_NOTIFIER=1 <lark-cli command>
```

当你在输出中看到 `_notice.update` 时，先完成用户当前请求；如仍相关，再简短告知可运行：

```bash
lark-cli update
```

**重要**：始终使用 `lark-cli update` 更新，它会同时更新 CLI 和 AI Skills。

## JSON 输出契约

`--format json`（默认）下，成功与错误的信封结构不同：

成功信封写入 **stdout**（退出码 0）：

```json
{ "ok": true, "identity": "user", "data": { "guid": "..." }, "meta": { "count": 1 } }
```

错误信封写入 **stderr**（退出码非 0）：

```json
{ "ok": false, "identity": "user", "error": { "type": "authorization", "subtype": "missing_scope", "code": 99991679, "message": "...", "hint": "...", "missing_scopes": ["..."] } }
```

**判断 CLI 调用成功用 `ok == true` 并核对退出码，不要用 `code == 0`**：成功信封没有顶层 `code` / `msg` 字段，`code` 只出现在错误信封的 `error` 内，含义是上游 OpenAPI 的 numeric code。按 OpenAPI 老格式 `{"code": 0, "msg": "ok"}` 判断会把所有成功调用误判为失败；封装写入类命令（如 `task +create`）时尤其危险，误判会绕过幂等逻辑导致重复创建。


CLI 成功仅说明该调用成功：异步任务须查询终态，批量结果须检查每项与分页，事件流还须验证目标事件。输出区分完成、部分完成、BLOCKED、NOT_RUN；不得因退出码 0 宣称整个业务成功。
