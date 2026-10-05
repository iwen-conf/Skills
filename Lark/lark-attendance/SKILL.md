---
name: lark-attendance
version: 1.0.0
description: "飞书考勤打卡：查询自己的考勤打卡记录"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli attendance --help"
---

# attendance (v1)

**CRITICAL — 开始前 MUST 先用 Read 工具读取 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md)，其中包含认证、权限处理**

## 查询合同

本技能仅覆盖查询自己的 `user_tasks.query`，显式 `--as user`。首次使用该 method 时读取 `lark-cli schema attendance.user_tasks.query`；同版本已确认的 schema 可复用。参数位置、支持字段和空数组含义须由当前 schema / 官方文档确认，不向任何 API 无差别注入固定值。

如果当前 method 合同明确支持以 `employee_type="employee_no"` 和空 `user_ids` 查询本人，再按规定位置使用下列合法 JSON 片段；它们不是完整请求，日期等必填字段须按实际 schema 加入：

```json
{"employee_type":"employee_no"}
```

```json
{"user_ids":[]}
```

若当前版本不支持这些约定，使用已查询到的当前用户标识，不能猜测空数组表示自己、全部人或零人。

## API Resources

```bash
lark-cli schema attendance.<resource>.<method>   # 调用 API 前必须先查看参数结构
lark-cli attendance <resource> <method> [flags]  # 调用 API
```

> **重要**：使用原生 API 时，必须先运行 `schema` 查看 `--data` / `--params` 参数结构，不要猜测字段格式。

### user_tasks

- `query` — 查询用户考勤打卡记录

## 权限表

| 方法 | 所需 scope |
|------|-----------|
| `user_tasks.query` | `attendance:task:readonly` |

