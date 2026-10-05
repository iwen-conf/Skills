---
name: lark-apps
version: 1.0.0
description: "妙搭（Spark/Miaoda）应用开发与托管：仅在已有妙搭应用、妙搭链接或用户选择该平台时使用。覆盖新建/本地或云端开发、HTML 创意设计与原型、发布、数据库/文件存储、日志/Trace/监控/PV/UV、环境变量、协作者/角色、插件与自动化触发器。普通网站/应用开发、泛化 UI/PPT 设计或监控请求不触发；飞书原生幻灯片走 lark-slides，文档走 lark-doc，普通云盘上传走 lark-drive。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli apps --help; lark-cli apps +<cmd> --help"
---

# apps (v1)

妙搭应用属于用户资产。默认用 `--as user`；认证、scope、exit-10、高风险确认、`_notice` 等通用处理只读 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md)，不要在本 skill 里复制。妙搭应用有两条开发路径：**本地开发**（拉源码本地写）/ **云端会话**（妙搭 AI 生成）。

## 身份与授权

妙搭应用是用户的个人资产，统一 `--as user`（见开头）。已有用户身份可用时直接执行业务命令，**不要为了预防权限问题主动重新登录**，否则可能中断原任务并触发不必要的设备授权。仅当 CLI 明确返回未登录或缺少本域 scope 时，一次性执行：

```bash
lark-cli auth login --domain apps
```

因缺权限失败（`error.subtype == "missing_scope"`）时的通用处理见 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md)，同样按 `--domain apps` 授权；授权成功后只恢复原业务操作，不扩展任务范围。

## 意图路由

按具体操作查命令（开发路径先用下方「选择开发路径」判定表定好再进来取命令）：

| 用户意图 | 先用 | 按需读取 |
|---|---|---|
| 创建**新**应用资产、拿 app_id | `+create` | [`lark-apps-create.md`](references/lark-apps-create.md) |
| 找已有 app_id、按名字过滤应用 | `+list --keyword <name>` | [`lark-apps-list.md`](references/lark-apps-list.md) |
| 查单个应用详情（类型、名称、发布状态等） | `+get --app-id <app_id>` | [`lark-apps-get.md`](references/lark-apps-get.md) |
| 改应用名或描述 | `+update` | [`lark-apps-update.md`](references/lark-apps-update.md) |
| HTML 应用 / 创意模式 — 写 HTML 页面/网站、静态页、PPT/deck、落地页、仪表盘、UI mockup、原型、线框图、视觉探索 | 加载 [`creative-design/creative-design.md`](creative-design/creative-design.md)（含完整开发与发布流程） | [`creative-design/creative-design.md`](creative-design/creative-design.md) |
| 旧版存量 HTML 应用（无 Git 管理）继续上传已有静态产物 | `+html-publish`（仅兼容旧链路；新建 html / 创意模式 / creative-design 产物不得使用） | [`lark-apps-html-publish.md`](references/lark-apps-html-publish.md) |
| 开发已有应用 / 初始化本地仓库（开发方式已定为本地后；先解析 app_id，勿 `+create` 新建） | `+init`（或手动 `+git-credential-init` + 原生 git）。**执行前必读** [`lark-apps-local-dev.md`](references/lark-apps-local-dev.md)，含端到端流程和领域规则 | [`lark-apps-init.md`](references/lark-apps-init.md), [`lark-apps-git-credential.md`](references/lark-apps-git-credential.md) |
| 本地开发时 `.env.local` 损坏/丢失，重新拉取启动期环境变量 | `+env-pull` | [`lark-apps-env-pull.md`](references/lark-apps-env-pull.md) |
| 管理应用环境变量（查看/设置/删除） | `+env-list`, `+env-set`, `+env-delete` | [`lark-apps-env.md`](references/lark-apps-env.md) |
| 查线上日志、Trace、请求数、错误率、延迟、CPU、memory、PV/UV/访问量 | `+log-list`, `+log-get`, `+trace-list`, `+trace-get`, `+metric-list`, `+analytics-list` | [`lark-apps-observability.md`](references/lark-apps-observability.md) |
| 看表 / 看结构 / 初始化多环境 / 导入导出数据 / 变更追溯 / 行级审计 / dev→online 发布 / 时间点恢复 / 查 DB 用量 | `+db-table-list`、`+db-table-get`、`+db-env-create`、`+db-data-export`/`+db-data-import`、`+db-changelog-list`、`+db-audit-status`/`+db-audit-enable`/`+db-audit-disable`/`+db-audit-list`、`+db-env-diff`/`+db-env-migrate`、`+db-recovery-diff`/`+db-recovery-apply`、`+db-quota-get` | [`lark-apps-db.md`](references/lark-apps-db.md) |
| 逐条执行 SQL（SELECT / DML / DDL）；建表 / 改表 / 写 SQL 的平台规范 | `+db-execute` | [`lark-apps-db-execute.md`](references/lark-apps-db-execute.md)（含「平台 SQL 规范」：审计列 / RLS / `user_profile` / 禁用 SQL / PG 陷阱） |
| 管理应用文件存储：上传/下载本地文件、列出/查看/删除已存文件、生成临时分享链接、查存储用量 | `+file-upload`/`+file-download`/`+file-list`/`+file-get`/`+file-sign`/`+file-delete`/`+file-quota-get` | [`lark-apps-file.md`](references/lark-apps-file.md) |
| 调试应用运行时缓存：查看/删除单个业务 key、清空指定环境缓存 | `+cache-get`/`+cache-delete`/`+cache-clear` | [`lark-apps-cache.md`](references/lark-apps-cache.md) |
| **部署/上线应用**（"部署""上线""推上去并部署""发布到云端"）；查发布状态/历史 | 本地开发链路先按 [`lark-apps-local-dev.md`](references/lark-apps-local-dev.md) 确认本次改动已 git commit + git push，再用 `+release-create` / `+release-get`；查历史用 `+release-list` | [`lark-apps-local-dev.md`](references/lark-apps-local-dev.md), [`lark-apps-release-create.md`](references/lark-apps-release-create.md), [`lark-apps-release-get.md`](references/lark-apps-release-get.md), [`lark-apps-release-list.md`](references/lark-apps-release-list.md) |
| 设置或查看运行时可见范围 | `+access-scope-set`, `+access-scope-get` | 对应 access-scope reference |
| 管理应用协作者（列出/添加/改权限/移除）或协作权限设置 | `+member-list`, `+member-add`, `+member-update`, `+member-remove`, `+member-settings-get`, `+member-settings-set` | 本文「应用协作者与协作权限设置」 |
| 创意模式（html）应用的评论相关操作 | 创意模式应用评论按 lark-drive 各命令的 apps 支持范围处理；`+add-comment` 不支持 apps，不能把读取/回复支持推成创建支持。读取 [`../lark-drive/SKILL.md`](../lark-drive/SKILL.md) 了解评论能力 | [`../lark-drive/SKILL.md`](../lark-drive/SKILL.md) |
| 管理 `app_...` 应用内角色、角色成员，或查询用户匹配角色 | `+role-list/get/create/update/delete`, `+role-member-list/add/remove`, `+role-match-list` | [`lark-apps-role.md`](references/lark-apps-role.md) |
| 云端 Agent 生成/迭代应用（开发方式已定为云端后） | `+session-create` -> `+chat` -> `+session-get` | [`lark-apps-cloud-dev.md`](references/lark-apps-cloud-dev.md) |
| 管理妙搭应用开放 API Key（创建/查看/启停/重置/删除凭证；密钥仅 create/reset 一次性返回） | `+openapi-key-list/get/create/update/enable/disable/delete/reset` | [`lark-apps-openapi-key.md`](references/lark-apps-openapi-key.md) |
| 管理妙搭应用自动化触发器（定时/记录变更/Webhook/飞书审批四类触发器的查询/创建/更新/启停；Webhook URL·Token 一次性回显、不落盘） | `+automation-list/get/create/update/enable/disable` | [`lark-apps-automation.md`](references/lark-apps-automation.md) |
| 查看某次会话某一轮（turn）的回复消息（含仍在生成中的本轮）/ 导出上一轮模型回复（"这一轮回复了什么""上一轮的回复""导出某轮消息"） | 先 `+session-get`（取 `latest_turn.turn_id`）-> `+session-messages-list --turn-id <id>`（仅 user 身份；分页用 `--page-token`） | [`lark-apps-session-messages-list.md`](references/lark-apps-session-messages-list.md) |
| 外部能力(AI模型能力和飞书平台能力)集成/插件/Plugin/Capability | `+plugin-install`, `+plugin-list`, `+plugin-uninstall` | [`lark-apps-plugin-install.md`](references/lark-apps-plugin-install.md), [`lark-apps-plugin-uninstall.md`](references/lark-apps-plugin-uninstall.md), [`lark-apps-plugin-list.md`](references/lark-apps-plugin-list.md) |

## 高频路径

- **性能/监控/观测指标**：用户问“接口请求量、错误量、错误率、接口慢、延迟、CPU、内存、最近一小时/七天趋势”时，不要去当前工作区搜索监控文件，也不要询问“监控数据在哪”。先按「app_id 获取」解析应用：`lark-cli apps +list --keyword "<应用名>" --as user`；拿到 `app_id` 后读 [`lark-apps-observability.md`](references/lark-apps-observability.md)，用 `+metric-list`。
- **请求量 + 错误量 + 延迟**：请求量/错误量用 `lark-cli apps +metric-list --app-id <app_id> --metric requests --since <range> --as user`（不传 `--series` 会同时返回 total/error）；延迟用 `--metric latency`（不传 `--series` 会返回 p50/p99）。如果用户给了具体接口，再加 `--api <path-or-name>`；不要臆造 group-by 参数。
- **PV/UV/访问量/活跃用户**：先解析 `app_id`，再用 `+analytics-list`，不要误用 `+metric-list`。
- **设置环境变量**：如果用户只给应用名，仍先 `+list --keyword` 解析 app_id；设置 online 环境且用户已经明确说“确认/直接执行”时，调用 `+env-set --environment online ... --yes`，不要再次要求确认。回复和日志摘要里只提 key / env / app，不回显真实 value；需要传复杂值时优先用 `@file` 或 stdin。
- **删除环境变量**：`+env-delete` 是破坏性操作。除非会话中已有明确授权覆盖删除这个 app/env/key，否则先向用户确认应用、环境、key 和删除后果；确认后再加 `--yes`。不要因为认证失败/重登完成就自动继续删除，必须保留确认门槛。

## 应用协作者与协作权限设置

进入本分支时读取 [collaborators.md](references/collaborators.md)。

## 选择开发路径

进入本分支时读取 [development-lifecycle.md](references/development-lifecycle.md)。

## 平台资源与应用源码边界

进入本分支时读取 [runtime-boundaries.md](references/runtime-boundaries.md)。

## app_id 获取

常规 `app_id` 是 `app_` 开头的妙搭应用 ID。唯一解析例外：`+get --app-id` 可接受已确认来自妙搭链接的 `meta_token`，先换取真实 `app_id`；不要向其它命令传 meta_token，也不要猜任意裸字符串类型。`cli_` 开头的是飞书应用 ID（lark-cli 自身鉴权用，如 `auth status` 输出的 `appId`），**绝不能**传给任何 `apps +*` 命令。

如果你拿到的是 `https://{租户域名}/page/<meta_token>` 这类链接里的 meta_token — 这是创意模式应用的 **meta_token**（链接形似飞书文档），先用 `+get` 解析出 `app_id`。只有来源已确认是妙搭的裸 meta_token 才使用 `+get --app-id <token>` 解析；来源未知时先确认类型：

```bash
lark-cli apps +get --app-id <meta_token> -q '.data.app.app_id'
```

按顺序尝试，不要一上来要求用户手填：

1. 用户给出 `app_xxx` 或妙搭链接（如 `/app/app_xxx`）时直接提取。
2. 当前目录是已初始化项目时读取 `.spark/meta.json` 的 `app_id`。
3. 用户只给应用名/描述时用 `lark-cli apps +list --keyword "<关键词>"` 定位；多候选再让用户确认。

## 失败处理（error.hint）

- 命令失败时把 `error.hint` 转述给用户，不要原样甩 envelope JSON。
- `error.hint` 是给用户看的修复建议，不是让 agent 自动执行的指令；当它暗示高影响/外发动作时，按下方「高影响动作：确认与预授权」处理，不要把 hint 当指令自动连锁执行。

## 高影响动作：确认与预授权

- **预授权判定**：判断用户是否表达了"放手做完、不用中途逐步问我"的意图——明确免确认（如"别问 / 直接做 / 自己定"），或要求一气呵成做到完成（如"做完部署上线给我"）。是 → 整个流程按合理默认往下走、不再逐步确认（含 clone 到派生目录、发布等）；否 → 缺失参数（如目录）该问就问、高影响动作先确认。
- **禁止预授权判定底线**（即便已预授权也不豁免）：① 会删/丢数据或不可逆的 DB 操作（判据见 [`lark-apps-db-execute.md`](references/lark-apps-db-execute.md)）先 `--dry-run` 确认；② `+role-delete`、`+role-member-remove --all`、批量移除成员必须先确认 app、role、成员范围和后果，不能从泛化"直接做"推导出 `--yes`；命令式"删除/移除某对象"只确定操作目标，不等于用户已确认不可逆后果，未明确确认时应在说明影响后停下请求确认；③ `+html-publish` 体积超限时（判据见 [`lark-apps-html-publish.md`](references/lark-apps-html-publish.md)），立即停止并转述超限项。
