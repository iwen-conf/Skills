# Docker Porting Pitfalls and Best Practices

When porting existing Docker images or `docker-compose.yml` files to Lazycat Micro-service (`lzc-manifest.yml`), developers often encounter issues at several key points. Refer to and apply these best practices when assisting users.

## 0. 排障写入边界：先判定是不是移植问题

普通 Lazycat 移植、打包、安装、上架、更新和安装后排障默认只允许修改包装层与运行时适配层。除非用户在当前任务明确说明“允许/需要修改业务代码”并点名范围，否则一律禁止修改上游业务源码。排障日志可以指向上游 bug，但不能自动授权修改上游业务源码。

**允许修改：** `package.yml`、`lzc-build.yml`、`lzc-manifest.yml`、`lzc-deploy-params.yml`、`Makefile`、`build.sh`、Docker wrapper、`runtime/`、启动脚本、`setup_script`、seed 脚本、配置模板、图标、商店素材和文档。

**禁止默认修改：** 上游前端页面/组件/路由/状态、后端 handler/service/domain/auth/agent/API 逻辑、数据库 schema/migration/model、测试、fixture，以及上游应用内部业务配置。

如果日志显示必须改上游业务源码才能修好，例如后端 `backend/app/...` 初始化崩溃、API handler 对缺失外部凭证没有降级、前端页面逻辑必须重写，处理方式是：

1. 先尝试非侵入方案：环境变量、`lzc-deploy-params.yml`、生成配置、bind、`setup_script`、wrapper entrypoint、seed 服务、OIDC、inject、upstream/Host 配置。
2. 如果非侵入方案不可行，输出 `Blocked by business-code change requirement`，说明具体日志、文件和符号。
3. 只有用户在当前任务明确说明“允许/需要修改业务代码”，并点名允许修改的业务范围后，才可以编辑这些业务源码；“修好”“跑起来”“可以处理问题”不算授权。

## 1. User & Permissions
**Problem:** Many third-party Docker images run as non-root users (e.g., `node`, `abc`) by default. However, in Lazycat Micro-service, persistent directories (`/lzcapp/var/`) and user document directories (`/lzcapp/run/mnt/home/`) require `root` privileges for read/write access by default, leading to `Permission denied` errors.

**Best Practices:**
- **Primary Solution:** Run the container as the `root` user whenever possible. This is the simplest path if the image does not strictly forbid it.
- **Secondary Solution (Root Forbidden):** If the application (e.g., certain databases) enforces a non-root policy, use `setup_script` to handle directory permissions with root privileges beforehand, or use `user: "1000"` (note: the UID must be a quoted string) in the `services` block and adjust permissions before startup.

## 2. Config File Initialization and R/W
**Problem:** Applications often require an initial configuration file (e.g., `config.yml`) packaged in the lpk (located at `/lzcapp/pkg/content/`), which the app needs to modify at runtime. Binding `/lzcapp/pkg/content/config.yml` directly via `binds` will fail because `/lzcapp/pkg/content` is **Read-Only**, causing the app to crash during modification attempts.

**Best Practices:**
- **Never** mount files under `/lzcapp/pkg/content/` directly as writable configurations.
- **Correct Approach:** Use `setup_script`. Before the container executes its original logic, check if the target writable path (e.g., `/lzcapp/var/config.yml`) exists. If not, copy the initial configuration from `/lzcapp/pkg/content/` to the writable location.
  
  ```yaml
  services:
    app:
      image: xxx
      binds:
        - /lzcapp/var/conf:/app/conf # Mount a writable directory
      setup_script: |
        if [ ! -f /app/conf/config.yml ]; then
            cp /lzcapp/pkg/content/default-config.yml /app/conf/config.yml
        fi
  ```

## 3. Startup Sequence & Health Checks
**Problem:** Heavyweight applications with databases may take a long time to initialize table structures during the first boot. Without proper health check configuration, the system may mark the container as `unhealthy` and kill it before initialization completes.

**Best Practices:**
- **Avoid hardcoded waits (`sleep`):** Arbitrarily extending the `start_period` is unreliable and provides a poor user experience.
- **Correct Approach:** Write semantically meaningful health check probes. For Web services, use `curl` to check actual API endpoints. For databases (e.g., MySQL), use actual SQL queries like `select 1` to determine if the service is truly ready.
- Use `services.[].healthcheck` (instead of the deprecated `health_check`) and properly configure `retries`, `interval`, and `start_period`.

### 3.1 Ingress Health Is Often a Secondary Symptom
**Problem:** Lazycat ingress logs may repeat `service "<name>" is not ready`, while the real failure is inside the routed business service or its dependency chain. A common pattern is: ingress waits for the business service -> business service starts early -> database is still `rejecting connections` -> business exits or becomes unhealthy -> ingress keeps reporting the route target as not ready.

**Best Practices:**
- Do not start by relaxing the ingress or application healthcheck. First inspect the route target service logs and dependency health.
- If business logs contain `connect: connection refused`, `database is starting up`, `rejecting connections`, `auth not ready`, `schema not found`, `topic missing`, or `bucket missing`, fix dependency readiness and seed order before changing the business healthcheck.
- Infra services need readiness probes that reflect actual readiness, not process liveness:
  - PostgreSQL: `pg_isready` can still report `rejecting connections` during first boot; give it a realistic `start_period` and verify logs.
  - MySQL: use `mysqladmin ping` or `SELECT 1` with the real credentials.
  - Redis: require `PONG` with auth when ACL/password is enabled.
- Business services that open DB/cache connections during startup must explicitly gate on every required infra service with `condition: service_healthy`.
- If the business still needs its own startup retry, keep it bounded and log the dependency being waited on. Do not rely only on Docker ordering.

### 3.2 One-shot Migration and Seed Containers
**Problem:** A migration or seed container may be modeled as a one-shot service with a terminal healthcheck marker such as `/tmp/done`. If it runs before PostgreSQL/MySQL/Redis is truly ready, it exits once with `connection refused`, never writes the marker, and every dependent service waits forever or starts with missing schema/config.

**Best Practices:**
- Keep the terminal marker contract: write `/tmp/done` (or the chosen marker) only after the migration/seed succeeds, then keep the container alive if downstream health gating depends on it.
- Add bounded retry/wait for transient dependency failures (`connection refused`, DB starting, temporary auth-not-ready). Permanent migration errors must still fail fast.
- Make seeds idempotent. Re-running after a partial install should produce `no change`, `already exists`, or equivalent success rather than corrupting state.
- Gate business services on the seed service with `condition: service_healthy`, not merely on the database.

### 3.3 Deployment Parameters and Generated Config
**Problem:** Apps using `lzc-deploy-params.yml` may look like they have a generic startup or 503 problem when the setup wizard was not completed, required params were not rendered, or generated config files are stale/missing.

**Best Practices:**
- Treat `need setup deploy params` as an installation/setup state, not a container startup failure. Complete the deployment-parameter wizard before starting or debugging the app.
- If a service needs a generated config file (for example `/app/.config/app.conf`), render it from environment/deploy params in `setup_script` on every startup so parameter changes apply after restart.
- During troubleshooting, inspect the rendered environment variables and generated file inside the container. Look for empty strings, placeholder values, unsupported provider names, or stale files from an older install.
- If deploy params were added after an earlier install, rebuild the LPK, reinstall, complete the setup wizard, then restart. Do not try to fix this by changing route healthchecks.

## 4. Privileged Mode & Capabilities
**Problem:** Some applications (e.g., sidecars, VPNs, or apps requiring FUSE mounts) depend on Docker's privileged mode (`privileged: true`) or specific capabilities (`cap_add`).

**Best Practices:**
- If the original app requires privileges, you can grant them in the Micro-service.
- Use the `compose_override` field in `lzc-build.yml` to inject these low-level Docker parameters (e.g., `privileged`, `cap_add`, `devices`).
- **Store Review:** Apps requiring these privileges **are allowed** for submission and review in the official Lazycat App Store, provided the functionality is justified.

## 5. LAN Access, CORS, and Host Header Validation
**Problem:** Container services often strictly validate the `Host` header of HTTP requests. Incorrect headers can lead to errors or CORS issues.

**Best Practices:**
- **Default Behavior:** Lazycat's `lzc-ingress` is highly intelligent and automatically handles Host headers and CORS in most cases; manual configuration is usually unnecessary.
- **Special Cases:** If the app reports domain or Host validation errors, configure forwarding rules in `application.upstreams` and add `use_backend_host: true` to ensure the upstream service sees its expected Host header.

## 6. 预构建镜像、运行时脚本与首启排障
**Problem:** 端口迁移类应用经常复用远程预构建镜像，同时只把 `runtime/`、`lzc-manifest.yml`、`docs/` 等文件打进 lpk。此时如果你修改了运行时脚本，并让脚本去调用镜像里原本不存在的新 Ruby 文件、新 rake task 或新应用代码，容器会在首启时直接退出，而不是单纯“启动太慢”。

**Best Practices:**
- **先区分“慢启动”还是“启动命令失败”：** 如果日志里出现 `container ... exited (1)`、`Unrecognized command`、`LoadError`、`NameError`，先按命令失败处理，不要先去调大 `start_period`。
- **明确包里真正带了什么：** 检查 `build.sh`/`lzc-build.yml`。如果打包过程只复制 `runtime/` 和 manifest，而没有构建新的应用镜像，就不要让 `runtime/*.sh` 依赖镜像中不存在的新增源码。
- **复用远程镜像时，优先把自定义逻辑写在包内可交付层：** 例如 `runtime/*.sh`、`setup_script`、已存在的镜像命令参数。只有当你真的同步构建并发布了新镜像时，才去依赖新增的应用源码或 rake task。
- **不要用业务代码修包装问题：** 如果报错来自路径、环境变量、启动顺序、默认账号、Host、健康检查或配置文件位置，先用 manifest、bind、`setup_script`、wrapper entrypoint、seed 服务、OIDC 或 inject 解决。普通移植任务中禁止为了让容器启动而直接改上游前端、后端、认证、schema、migration 或测试代码。
- **外部凭证缺失不是业务源码修改许可：** 如果接口 500 的根因是缺少 `OPENAI_API_KEY`、模型供应商 key、第三方服务地址或 webhook secret，优先用 `lzc-deploy-params.yml`、环境变量、生成配置或使用说明解决。不要为了上传、登录、健康检查或冒烟测试通过，直接给上游后端 agent/API handler 加懒初始化、兜底返回或跳过逻辑。
- **排查顺序要固定：**
  1. 看 `startup-log-tail`，判断是卡在 `Waiting` 还是已经 `exited (1)`。
  2. 看失败容器的最后日志，抓具体异常字符串。
  3. 对照 `build.sh`，确认报错涉及的文件或命令是否真的会进入最终包或镜像。
  4. 只有在命令存在且进程仍存活时，才继续分析 healthcheck 与冷启动耗时。
- **缩短首启时，先移走非关键前置任务：** 像 GeoIP 下载、演示数据准备、可延后缓存构建，不要阻塞 Web 服务首次通过健康检查。优先放到后台 worker、应用就绪后的异步任务，或做成“已存在即跳过”的幂等步骤。
- **避免重复应用引导：** 如果启动链里先 `bundle exec rails <task>`，又额外跑一次 `bundle exec rails runner`，通常意味着支付了两次完整 Rails 冷启动成本。能合并成一次就合并成一次。
- **经验规则：** “healthcheck 失败”不等于“一定要调 healthcheck”。先确认应用有没有真的启动起来，再确认它是不是被不必要的前置步骤拖慢。
