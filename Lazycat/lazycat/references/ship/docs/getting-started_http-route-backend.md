# Source: https://developer.lazycat.cloud/getting-started/http-route-backend.md

# 有后端时如何通过 HTTP 路由对接 {#http-route-backend}

## 目标 {#goal}

完成本篇后，你可以明确区分并验证这 3 件事：

1. `exec://...` handler 会“启动进程 + 转发流量”，适合单容器后端快速起步。
2. `http://...` handler 只负责转发，不会帮你启动进程；更适合接入已经存在的独立 service。
3. 你会理解默认鉴权、`application.public_path`（哪些路径允许未登录访问）最小放开规则，以及后端数据应写入 `/lzcapp/var`。

## 前置条件 {#prerequisites}

1. 你已完成 [5 分钟完成 Hello World 并多端验证](https://developer.lazycat.cloud/hello-world-fast.md)。
2. `lzc-cli box default` 已指向目标微服。
3. 你本机可以执行 `curl`（用于可选验证）。

## 步骤 {#steps}

### 1. 创建一个带后端的模板项目 {#step-create-backend-template}

```bash
lzc-cli project create hello-api -t todolist-golang
```

然后在应用 ID 提示里按回车使用默认值，或输入你自己的命名。进入项目目录：

```bash
cd hello-api
```

### 2. 先部署并验证默认 `exec` handler {#step-verify-default-exec-handler}

先部署：

```bash
lzc-cli project deploy
lzc-cli project info
```

默认情况下，`project` 命令会优先使用 `lzc-build.dev.yml`，并打印实际的 `Build config`，也就是“当前实际命中的构建配置文件”。
如果你要操作发布配置，请显式加上 `--release`。

继续后续步骤前，请先确认 `lzc-cli project info` 输出里出现 `Project app is running.`。\
如果还在启动中，等待几秒后再次执行 `lzc-cli project info`。

在浏览器打开 `Target URL` 并完成登录，然后在开发者工具 Console 执行：

```javascript
fetch('/api/health').then((r) => r.json()).then(console.log);
fetch('/api/todos').then((r) => r.json()).then(console.log);
```

期望结果：

1. `/api/health` 返回 `{"status":"ok"}`。
2. `/api/todos` 返回 `items` 列表（初次通常为空）。

说明：

1. 模板默认路由是 `/=exec://3000,/app/run.sh`。这里的路由规则可以先理解成“某个请求该转给谁处理”。
2. 这条规则会执行 `/app/run.sh` 并把流量转发到 `127.0.0.1:3000`。
3. 这就是 `exec` handler 的核心：启动进程 + 转发请求。

### 3. 新增第三方 service，演示 `http` handler {#step-add-third-party-service}

编辑 `lzc-manifest.yml`，在保留默认 `exec` 路由的前提下，新增 `/inspect/` 路由与 `whoami` service：

```yml
application:
  image: embed:app-runtime
  routes:
    - /inspect/=http://whoami:80/
    - /=exec://3000,/app/run.sh

services:
  whoami:
    image: registry.lazycat.cloud/traefik/whoami
```

说明：

1. `whoami` 是一个真实第三方服务，用来演示 `http` 路由转发目标。
2. `/inspect/` 走 `http://whoami:80/`，不会触发 `/app/run.sh`。
3. `"/=exec://..."` 保持不变，原本 Todo 功能仍可用。
4. `/inspect/` 规则要放在 `"/="` 之前，避免被更宽泛规则先匹配。

### 4. 部署并验证 `http` handler {#step-verify-http-handler}

```bash
lzc-cli project deploy
lzc-cli project info
```

在已登录浏览器页面的 Console 执行：

```javascript
fetch('/inspect/').then((r) => r.text()).then(console.log);
```

期望结果：输出里可看到 whoami 返回的请求信息（例如 `Hostname`、`RemoteAddr`、`Headers`）。

通过这一步你可以确认：`http` handler 只做转发，目标响应来自 `services.whoami`。

### 5. （可选）配置 `application.public_path` 后用 curl 验证 {#step-configure-public-path}

默认情况下，微服应用 HTTPS 路径受登录态保护。未登录请求直接 `curl` 通常会被拦截，这是预期行为。

如果你希望放开健康检查，可在 `lzc-manifest.yml` 添加：

```yml
application:
  public_path:
    - /api/health
```

重新部署后验证：

```bash
lzc-cli project deploy
lzc-cli project info
curl "<Target URL>/api/health"
```

说明：`public_path` 应只放开必要路径，避免过度暴露接口。

### 6. 查看 Todolist 的数据目录（`/lzcapp/var`） {#step-check-lzcapp-var}

先回到应用页面，手动新增 1-2 条待办事项（Todo），让应用先产生数据文件。

先进入应用容器：

```bash
lzc-cli project exec -s app /bin/sh
```

进入容器后执行：

```bash
ls -la /lzcapp/var
```

这里对应的就是应用容器内的 `/lzcapp/var`。\
对于 Todolist 这类有后端数据的应用，数据文件应放在这个路径下（例如 `/lzcapp/var/todos.json`）。

重要提示：仅 `/lzcapp/var/` 目录下的内容会在重启应用后保留。

## 验证 {#verification}

满足以下条件即通过：

1. `project info` 显示 `Current version deployed: yes`。
2. `/api/health` 返回 `status: "ok"`。
3. `/inspect/` 返回 whoami 的请求信息。
4. 你能通过 `project exec` 在容器内确认 Todolist 的持久化目录 `/lzcapp/var`（例如看到 `todos.json`）。

## 常见错误与排查 {#troubleshooting}

### 1. `/inspect/` 返回 `404` 或 `502` {#error-inspect-404-502}

排查顺序：

1. 检查路由是否为 `/inspect/=http://whoami:80/`。
2. 检查该路由是否位于 `"/="` 规则之前。
3. 检查 `services.whoami.image` 是否为 `registry.lazycat.cloud/traefik/whoami`。
4. 执行 `lzc-cli project log -s whoami -f` 查看 whoami 容器日志。

### 2. `curl <Target URL>/api/health` 被拦截或跳转登录 {#error-health-api-blocked}

原因：默认鉴权开启，未登录请求会被拦截。

处理：

1. 用已登录浏览器执行 `fetch('/api/health')` 验证。
2. 或按需配置 `application.public_path` 后重新 `project deploy`。

### 3. 不确定 Todolist 数据该放哪里 {#error-where-to-store-data}

处理：统一放在 `/lzcapp/var` 下；仅该目录内容会在重启应用后保留。

### 4. 部署后结果没变化 {#error-no-change-after-deploy}

处理：

```bash
lzc-cli project deploy
lzc-cli project info
```

确认 `Current version deployed: yes`。

## 下一步 {#next}

继续阅读：[LPK 如何工作：精简机制与最小规范](https://developer.lazycat.cloud/lpk-how-it-works.md)