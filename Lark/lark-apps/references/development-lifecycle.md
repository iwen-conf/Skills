## 选择开发路径（先区分已有与新建）

已有应用先使用现有本地项目、用户已选路径或上下文约定；下面未表达偏好必须提问的条目仅适用于新建且会改变执行平台的选择。

新建必先定 **app_type** 和**开发方式**两件正交的事；修改已有先按「app_id 获取」指认到 app，指认不到就问用户，不擅自 `+create`。开发方式（本地 vs 云端）只看用户对"谁来写代码"的偏好，与应用复杂度、要不要数据库无关。

**app_type 三类边界**（先判"要不要把数据存到服务端"，再判"纯展示还是有交互"）：

| 信号 | 判定 |
|---|---|
| 含数据库 / 后端持久化：登录 / 增删改查 / 报名·投票·站会存记录 / 多人协作 / 泛称"系统·工具"且明确要存数据 | `app_type=full_stack` |
| 纯静态展示（给人"看"的物料，无 JS 交互）：PPT/deck / demo / 落地页 / 海报 / UI mockup / 线框图 / 静态仪表盘 / 视觉探索 | `app_type=html`，加载 [`creative-design/creative-design.md`](../creative-design/creative-design.md)（含完整开发与发布流程） |
| 有 JS 交互但无数据库（给人"用"的前端应用）：可交互原型 / SPA / 表单校验 / 动态计算 / 调用外部 API / 泛称"工具·系统"但未明确要存数据 | `app_type=frontend`（**默认倾向**：用户未明确提出数据库需求时默认引导 frontend，不默认 full_stack） |
| 类型模糊（尤其"要不要存数据"不清） | **追问**，话术偏向 frontend，例："看起来是个前端应用，需要保存数据吗？"；确认要存数据再转 full_stack，确认纯展示再转 html |
| 用户要自己写 / 本地 IDE·code agent / 拉源码到本地 / 交研发 | 本地开发，读 [`lark-apps-local-dev.md`](../references/lark-apps-local-dev.md) |
| 让妙搭 AI 云端生成 / 对话式 / 自己不碰代码 | 云端会话，读 [`lark-apps-cloud-dev.md`](../references/lark-apps-cloud-dev.md) |
| 未表达"谁来写"偏好 | **必须先问**（本地代码开发 vs 云端 AI 生成）；选定前不擅自选边、不暗示默认，不得以"需求不模糊"为由跳过提问直接 `+init` / `git clone` / `+session-create` / 首轮 `+chat` |
| 修改已有 + 当前目录是 `.spark/meta.json` 项目 | 直接继续本地按意图路由，不必问也不必判云端 |
| 修改已有 + 有云端偏好 | 云端会话；未表达偏好且非本地项目 → 默认本地；判不准先问 |

**类型升级**：`frontend` 应用后续需要数据库/后端能力时，本地 CLI 不提供类型升级；引导用户到云端会话（打开 `https://miaoda.feishu.cn/app/{app_id}`），用自然语言描述后端需求（如"给这个应用加登录和数据存储"）即可触发升级，无需特殊指令。

## 发布态护栏

- **发布意图判定**：用户要"可访问 / 线上 / 分享 / 新链接 / 上线" = 发布意图，先走发布链路、确认完成再给链接。
- 完成 ≠ 发布：云端会话完成 / `+list is_published=true` 都不代表最新内容已部署。
- 开发态链接 `https://miaoda.feishu.cn/app/{app_id}`（full_stack / frontend 应用）：进应用编辑/开发态、管理与继续开发应用的入口，也是 frontend 升级为 full_stack 的入口（云端会话）。创意模式（html）应用开发态和发布态是同一个链接，无需额外提供开发态链接。
- 发布态链接来源：`+release-get` 轮询 `finished` 给 `online_url` / `failed` 给 `error_logs`（html / frontend / full_stack 统一走 `+release-get`）。
- html 应用的主链路是创意模式开发方式：按 [`lark-apps-local-dev.md`](../references/lark-apps-local-dev.md) 初始化仓库、在仓库内产出 HTML 及关联文件，并通过 git commit / git push / `+release-create` / `+release-get` 发布部署。仅 git 错误明确指向认证/凭证失效时，才执行 `lark-cli apps +git-credential-init --app-id <app_id> --as user` 刷新本地 Git 凭证，再重试原 git 命令一次。网络错误有限退避，冲突或 non-fast-forward 检查实际分支与差异，路径/权限错误核对环境；这些错误不触发凭证刷新。如果刷新凭证也失败，**停止并向用户报告**：原始 git 错误、凭证刷新失败原因，以及是否可能是当前环境（操作系统、沙箱）限制导致（如 macOS Keychain 在沙箱中不可用、Linux 加密文件目录不可写等）。不要改走 `+html-publish`，也不要把 `+html-publish` 当作本地开发链路的 fallback。
- 创意模式（html）应用的链接格式为 `https://{租户域名}/page/{meta_token}`，**开发态和发布态是同一个链接**（区别于 full_stack 应用两者分开）。此链接形似飞书文档链接。`+get --app-id <meta_token>` 可获取应用信息（含 `app_id`），`+get --app-id <app_id>` 可获取 `meta_token`。看到 `/page/xxx` 链接时，它是妙搭创意模式应用，不要当成飞书文档跳过。

