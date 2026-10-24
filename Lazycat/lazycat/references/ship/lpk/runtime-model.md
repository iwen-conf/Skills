# Lazycat Runtime Model and Porting Judgment

本文件用于帮助 AI 在迁移前理解懒猫微服的运行机理，并据此判断一个 Docker / Compose / 开源项目是否适合迁移、应该怎样迁移，以及哪些启动失败可以在设计阶段提前避免。

## 1. 运作模型

懒猫应用不是“把 docker-compose 原样丢给用户运行”。LPK 把应用身份、静态元数据、运行清单、静态内容和图标一起封装，安装后由微服平台负责标准化运行。本技能包强制禁止内嵌镜像；需要镜像时必须使用可拉取的远程镜像地址。

关键边界：

- `package.yml`：静态元数据。包括包名、版本、名称、描述、作者、许可证、`unsupported_platforms`、`locales`。
- `lzc-build.yml`：构建阶段配方。只在打包时生效，安装和运行时不会读取开发目录里的这个文件。
- `lzc-manifest.yml`：运行阶段清单。定义 `application`、`services`、路由、入口、环境变量、持久化目录、健康检查和初始化脚本。
- `.lpk`：构建后的交付物。安装后系统读取 LPK 内部的 `manifest.yml`、`package.yml`、`content.tar` 等产物；本技能包要求最终包体 `<= 12,000,000` bytes，且不得包含 `images/` 或 `images.lock`。

HTTP 访问路径：

1. 客户端请求进入微服平台的统一入口。
2. 平台按子域名定位应用实例，并处理登录态和 `public_path`。
3. 请求进入应用的 `application` / `lzcinit` 层。
4. `application.routes` 或 `application.upstreams` 再把请求转发到具体 service 和端口。

TCP/UDP 访问路径：

1. 只由 `application.ingress` 处理 4 层转发。
2. 不做 HTTP 语义解析，也不适合替代普通 Web 路由。
3. 不要默认接管 80/443，除非用户明确理解会绕过系统认证和路由。

## 2. 迁移前硬判断

开始写包装文件前，必须先完成以下判断。任何一项无法回答，都不能宣称迁移方案可靠。

### 2.1 交付形态

- 有官方镜像：优先直接使用官方镜像，迁移范围保持在 manifest/package/build/运行脚本。
- 有 `docker-compose.yml`：按 Compose 的服务、端口、卷、环境变量、依赖关系转换到 manifest。
- 只有 `Dockerfile`：构建 `linux/amd64` 镜像，再同步到 Lazycat 可拉取的 registry；禁止把镜像内嵌进 `.lpk`。
- 只有源码且无 Docker 化方案：先评估能否非侵入地补 Docker 包装层；不要默认改业务代码。如果必须改业务代码才能启动，应输出阻塞结论或请求明确的产品开发授权。
- 需要 GPU、KVM、dockerd、systemd、FUSE、特权能力：先标记为高级能力或普通迁移阻塞；不要纳入默认迁移路径。

### 2.2 网络入口

- 普通 Web/API：使用 `application.routes` 或 `application.upstreams`，目标写成 `http://service_name:port`。
- 需要保留路径前缀：用 `application.upstreams` 并设置 `disable_trim_location: true`。
- 后端校验 Host：用 `application.upstreams.use_backend_host: true`，不要先改业务代码。
- 非 HTTP 协议：使用 `application.ingress`，明确 `protocol`、`port`、`service` 和暴露边界。
- SPA 刷新 404：这是上游 Web server fallback 问题，不是 Lazycat 路由本身的问题。

### 2.3 存储与权限

- 持久化数据必须绑定到 `/lzcapp/var/...`。
- 缓存可用 `/lzcapp/cache/...`。
- 用户文件访问按能力使用 `/lzcapp/run/mnt/home` 或官方文件访问能力。
- `/lzcapp/pkg/content` 是包内容，只读；需要运行时可写的配置必须复制到 `/lzcapp/var` 后再让应用读写。
- 第三方镜像如果默认非 root，先判断是否会写 `/lzcapp/var`；必要时用 root、`user` 或 `setup_script` 处理权限。

### 2.4 启动依赖

必须把服务分层：

1. 基础设施：PostgreSQL、MySQL、Redis、MinIO、ZooKeeper、etcd。
2. 中间件：Nacos、Consul、Kafka、RocketMQ、搜索服务、队列服务。
3. 一次性初始化：迁移、schema 导入、bucket 创建、默认账号创建、配置推送。
4. 业务服务：gateway、auth、api、worker、web。

判断规则：

- 业务服务启动时连接的每个依赖，都必须出现在 `depends_on`。
- 依赖必须有真实 readiness healthcheck，不能只检查进程存在。
- 只有 Docker Compose 列表式 `depends_on` 时，迁移时必须升级为健康门控语义。
- 一次性 seed/migration 必须幂等，成功后写 terminal marker，失败时不要假装 healthy。
- 可选依赖不要使用强制 `service_healthy` 阻塞整个应用；应由应用自身重试或降级。

### 2.5 初始化与配置

- `setup_script` 每次容器启动都会执行，必须幂等。
- `setup_script` 在原始 entrypoint/command 之前执行，不能依赖应用已经启动后的状态。
- 部署参数必须通过安装向导完成渲染；`need setup deploy params` 是安装配置状态，不是容器崩溃。
- 由部署参数生成的配置文件，应在 `setup_script` 每次启动时从渲染后的环境变量重写或校验。
- 固定初始账号、OIDC、inject 免密登录、改密学习必须与启动顺序和健康检查一起设计。
- 如果初始化、登录、路由、健康检查或配置适配看起来需要改上游源文件，必须先穷尽环境变量、命令参数、`setup_script`、bind、生成配置、wrapper entrypoint、OIDC、inject 等非侵入方式。没有非侵入路径时，不要自行改业务代码。

## 3. 能否在懒猫上运行的判定门禁

判断一个应用“能不能在懒猫上跑”，不是判断它有没有源码或页面好不好看，而是判断它能否被映射成懒猫运行态。必须逐项给出证据，最后输出判定结论。

### 3.1 镜像与架构

- 是否已有可拉取镜像，或能构建出可发布镜像。
- 镜像架构是否匹配目标环境；迁移构建默认应产出 `linux/amd64` 可用镜像，除非目标能力明确支持其他架构。
- 镜像是否依赖宿主机目录、宿主 Docker socket、固定容器名、host network、特权设备或本机二进制。
- 如果镜像无法公开拉取，是否能通过测试 registry 或 `lzc-cli appstore copy-image` 转为 Lazycat 可拉取地址。
- 是否能在不使用内嵌镜像的前提下交付；`lzc-build.yml.images`、`embed:<alias>`、最终包内 `images/` 和 `images.lock` 都不允许。

判定：

- 镜像可拉取或可构建，且无不可替代宿主依赖：可继续。
- 依赖特权、GPU、KVM、dockerd、systemd、FUSE：转入高级能力判断，普通迁移默认不继续。
- 只有本机脚本、无 Docker 化路径、且必须改业务代码才能启动：高风险，不能直接宣称可迁移。
- 为了通过构建或启动而修改上游前端、后端、认证、schema、migration、测试或业务配置，不属于普通迁移范围；除非用户在当前任务明确说明“允许/需要修改业务代码”并点名范围，否则应标记为 `Blocked by business-code change requirement`。

### 3.2 进程模型

- 容器启动后是否有一个长期前台进程。
- 应用是否依赖 systemd、cron、后台 daemon、交互式 shell 或手工命令。
- 原始 entrypoint/command 是否能在 Lazycat service 中保持运行。
- `setup_script` 是否只做启动前准备，而不是替代长期运行进程。

判定：

- 单进程或明确的前台启动命令：可继续。
- 需要 systemd/dockerd/完整 init：必须评估 `sysbox-runc` 或专门包装层；普通迁移默认不继续。
- 只提供一次性 CLI，没有长期服务或静态入口：不应按普通 Web 应用迁移。

### 3.3 网络入口与监听

- 应用实际监听端口是什么，协议是 HTTP、WebSocket、gRPC、TCP 还是 UDP。
- 服务是否监听在容器内可访问地址。普通服务应能被同应用内服务名和端口访问。
- HTTP 应用能否通过 `application.routes` 或 `application.upstreams` 暴露。
- 非 HTTP 协议是否真的需要 `application.ingress`。
- 是否依赖固定公网域名、回调域名、Host header 或 HTTPS 终止。

判定：

- 有明确 HTTP 端口：可映射到 `routes` / `upstreams`。
- 需要保留 URL 前缀或 Host：用 `upstreams` 的对应能力处理。
- 只有随机端口、P2P 广播、LAN 发现、mDNS 或必须占用 80/443：高风险，先做网络适配判断。

### 3.4 文件系统与持久化

- 应用写入哪些目录：数据库、上传文件、配置、日志、缓存。
- 必须持久化的数据是否都能映射到 `/lzcapp/var`。
- 缓存是否可映射到 `/lzcapp/cache`。
- 运行时会修改的配置是否已经从只读 `/lzcapp/pkg/content` 复制到可写目录。
- 容器用户是否有权限写这些目录。

判定：

- 数据路径明确且可 bind 到 Lazycat writable 路径：可继续。
- 数据写入镜像层、系统目录或用户家目录且无法改配置：高风险。
- 非 root 镜像写 `/lzcapp/var` 报权限：用 root、`user` 或 `setup_script` 处理后再判断。

### 3.5 依赖与首启

- 是否依赖数据库、缓存、对象存储、消息队列、搜索服务或外部 API。
- 每个依赖是否在同一应用内可提供，或用户是否能稳定配置外部地址。
- 首次启动是否需要 schema migration、bucket 初始化、默认用户创建、配置导入。
- 业务服务是否会在启动阶段连接依赖。
- readiness 探针是否能证明服务真的可用，而不是只证明进程存在。

判定：

- 依赖清晰、可 health-gate、seed 幂等：可继续。
- 依赖未就绪会直接退出但无重试或健康门控：必须先修包装层。
- 依赖外部商业 SaaS、私有 API、不可公开凭证或用户无法获得账号：不适合普通上架迁移。

### 3.6 运行配置与账号

- 必需环境变量是否完整。
- 配置是否能通过 `lzc-deploy-params.yml`、`setup_script`、模板或默认值生成。
- 如果有内置登录，用户是否能免密进入，或是否有固定初始账号和改密学习机制。
- OIDC、`public_path`、inject 是否能非侵入实现。

判定：

- 必需配置可由部署参数或默认配置生成：可继续。
- 必须人工进入容器执行初始化或手抄随机密码：不算可交付运行。
- 登录路径未知、选择器/API 未验证：不能宣称免密登录完成。

## 4. 运行可行性结论

每次迁移判断必须输出以下四类之一：

- `Can Run`：证据完整，能映射为 Lazycat 的 image/service/route/bind/health/init/login。
- `Can Run After Packaging Fixes`：能运行，但必须先补包装层，例如镜像同步、路径 bind、healthcheck、seed、deploy params、Host/upstream 配置。
- `Cannot Determine Yet`：缺少关键事实，例如端口、启动命令、持久化路径、依赖、账号初始化方式；必须继续查 upstream docs、Compose、Dockerfile 或运行日志。
- `Not Suitable For Standard Lazycat Port`：核心运行方式无法合理映射，例如强依赖不可替代宿主能力、不可公开外部凭证、必须改业务代码才能启动、没有长期服务入口，或普通用户无法完成使用闭环。

结论必须引用证据，不允许只写“应该可以跑”。

## 5. Compose 到 Manifest 转换原则

| Compose / Docker 字段 | Lazycat 处理 |
| --- | --- |
| `image` | 写入 `services.<name>.image`，发布前同步为最终可拉取镜像 |
| `ports: ["8080:80"]` | HTTP 入口转为 `application.routes: ["/=http://service:80"]` |
| 非 HTTP 端口 | 转为 `application.ingress` |
| `volumes` | 左侧改为 `/lzcapp/var/...` 或 `/lzcapp/cache/...` |
| `environment` | 写入 service `environment` |
| `depends_on` | 保留并转换为健康门控依赖 |
| `healthcheck` | 保留并校准为真实 readiness |
| `restart` / `container_name` | 通常不需要迁移 |
| `privileged` / `cap_add` / `devices` | 进入高级能力判断，不要静默丢弃 |

## 6. 常见误判

- 只看到 Web 页面能跑，就忽略数据库首启、迁移、默认账号、持久化目录和健康检查。
- 把 ingress 的 `service not ready` 当成 ingress 配置错误，而不是先查目标 service 和依赖链。
- 认为 `depends_on` 表示“等数据库可用”；列表式依赖通常只表示启动顺序，不等于 readiness。
- 把需要构建进镜像的运行时代码放进 LPK 内容目录，然后让预构建镜像去调用它。
- 使用内嵌镜像或让 `.lpk` 超过 12 MB；正确做法是瘦身包内容并使用远程镜像桥接。
- 在 `make install` 中偷偷执行镜像构建、推送、`copy-image` 和 manifest 回写，导致安装入口不可预测。
- 为了迁移方便直接改上游认证、数据库 schema、前端页面、后端 API、测试或业务代码，而没有先尝试 wrapper、env、setup、seed、OIDC、inject。
- 用户已经说“不要修改业务代码”或任务只是普通移植时，仍然为了达成健康检查、登录、初始化或审核目标去改上游源码。
- 没有明确长期前台进程，就把一次性 CLI 或后台脚本包装成普通应用。
- 没有弄清真实监听端口，就直接写 `routes: /=http://service:80`。
- 没有验证普通用户是否能获得账号或配置，就认为“容器能启动”代表“应用能交付”。

## 7. 迁移可行性结论模板

```text
Runtime Model
- Delivery: <official image / compose / dockerfile / source only>
- LPK Package: <= 12,000,000 bytes, no embedded images
- Entry: <routes / upstreams / ingress>
- Persistence: <paths and permission plan>
- Dependencies: <infra -> seed -> business order>
- Init: <setup_script / seed / deploy params / account>
- Login: <none / OIDC / inject / fixed initial credentials>

Runability Gate
- Image/Arch: <evidence>
- Process: <evidence>
- Network: <evidence>
- Storage: <evidence>
- Dependencies: <evidence>
- Init/Login: <evidence>
- External Requirements: <none / user-configurable / blocking>

Risk Judgment
- Low: has official image, clear env/volumes/ports, no boot-time DB race.
- Medium: compose stack with DB/cache/seed but healthchecks are clear.
- High: source-only, undocumented init, fragile auth, privileged runtime, or unknown readiness.

Decision
- Can Run / Can Run After Packaging Fixes / Cannot Determine Yet / Not Suitable For Standard Lazycat Port
```
