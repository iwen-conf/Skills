## 应用协作者与协作权限设置

这组命令管理妙搭应用的开发协作者和协作策略，不等同于 `+access-scope-*` 的运行时访问范围，也不等同于 `+role-*` 的应用内业务角色。所有命令使用 `app_...` 应用 ID 和 `--as user`。不要读取或判断 `app_type` 来预判支持范围，直接调用对应的协作者命令。

- `+member-list`、`+member-settings-get` 是只读命令，需要 `spark:app:read`。
- `+member-add`、`+member-update`、`+member-remove`、`+member-settings-set` 是高风险写命令，需要 `spark:app:write`。先用 `--dry-run` 核对目标、URL 和请求体；dry-run 不需要 `--yes`。用户已确认具体应用、成员/设置及影响，或已按[入口授权协议](../SKILL.md#高影响动作确认与预授权)对整条流程明确预授权时，真实执行加 `--yes`；否则在 dry-run 后停下请求确认。批量移除成员仍执行[入口授权底线](../SKILL.md#高影响动作确认与预授权)，不能从泛化的“直接做”推导出 `--yes`。
- 添加、更新、移除成员时必须显式提供匹配的外部 ID 类型，禁止传内部数字 ID、猜测类型或做隐式转换：用户 `--member-type openid --member-id ou_...`；群组 `--member-type openchat --member-id oc_...`；部门 `--member-type opendepartmentid --member-id od-...`。
- `+member-list --member-type` 的筛选枚举是响应对象类型 `user` / `department` / `chat`，与写命令的 ID 类型枚举不同。可再用 `--role view|edit|full_access` 筛选。
- `+member-list` 一次返回应用的全部直接协作者，不提供分页参数；可用 `--member-type` 和 `--role` 缩小结果范围。
- 成员响应不包含应用详情。需要名称、类型或发布状态时单独调用 `+get --app-id <app_id>`，不要期待成员分页重复返回 `app`。
- 收到 subtype `feature_not_available`（OpenAPI code `3340005`；直连服务可能为 `40005`）时，立即停止 CLI 自动化，不切换 `app_type`，也不尝试用 access scope、应用角色或其它成员命令绕过。向用户说明该应用暂不支持通过 lark-cli 设置协作者，并引导其在妙搭后台的权限设置中操作。
- `external_invite` 只在 `+member-settings-get` 的响应中读取，不能独立设置；它会跟随 `external_access`。CLI 不注册 `--external-invite`，需要改变外部协作能力时只设置 `--external-access`。
- `copy_download_by` 也只在 `+member-settings-get` 的响应中读取。CCM 当前明确不支持为妙搭对象写入复制、打印和下载权限，因此 CLI 不注册 `--copy-download-by`。保留读取结果，不要尝试写入，也不要改用其它权限字段模拟。

```bash
# 读取协作者和当前协作策略
lark-cli apps +member-list --app-id <app_id> --as user
lark-cli apps +member-settings-get --app-id <app_id> --as user

# 写操作先预览精确的 typed-ID 字段；确认后把 --dry-run 换成 --yes
lark-cli apps +member-add --app-id <app_id> --member-type openid --member-id ou_xxx --perm view --dry-run --as user
lark-cli apps +member-update --app-id <app_id> --member-type openchat --member-id oc_xxx --perm edit --dry-run --as user
lark-cli apps +member-remove --app-id <app_id> --member-type opendepartmentid --member-id od-xxx --dry-run --as user
lark-cli apps +member-settings-set --app-id <app_id> --external-access disabled --comment-by viewer --dry-run --as user
```

