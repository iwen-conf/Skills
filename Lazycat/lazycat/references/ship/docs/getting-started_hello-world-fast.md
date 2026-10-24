# Source: https://developer.lazycat.cloud/getting-started/hello-world-fast.md

# 5 分钟完成 Hello World 并多端验证 {#hello-world-fast}

## 目标 {#goal}

完成本篇后，你可以明确区分并验证这 3 件事：

1. 你创建并部署的应用，用户侧看到的主要是一个通过 HTTPS 访问的 Web App。
2. LPK 是应用最终交付和安装的包格式：代码与运行声明会被打成一个 `.lpk`，再安装到目标微服运行。
3. 同一份 `.lpk` 部署后，可在 Android、iOS、macOS、Windows、Web（浏览器）等多端一致访问，体现懒猫微服的跨平台能力。

## 前置条件 {#prerequisites}

1. 你已完成 [开发者环境搭建](https://developer.lazycat.cloud/env-setup.md)。
2. `lzc-cli box default` 已指向目标微服。

## 步骤 {#steps}

### 1. 创建项目 {#step-create-project}

在你自己的工作目录执行：

```bash
lzc-cli project create hello-lpk -t hello-vue
```

然后在应用 ID 提示里按回车使用默认值，或输入你自己的命名。

```bash
cd hello-lpk
```

模板默认会生成：

1. `lzc-build.yml`：默认构建配置，也是发布配置。
2. `lzc-build.dev.yml`：开发态覆盖配置，默认包含独立的 dev 包名覆盖（例如 `package_override.package: cloud.lazycat.app.helloworld.dev`）、空 `contentdir` 覆盖，以及 `DEV_MODE=1`。

这意味着 `project deploy`、`project info`、`project exec` 等 `project` 命令默认都会落到独立的 dev 包名，不会覆盖正式发布包。
命令输出里会打印一行 `Build config`，就是在告诉你“这次实际用了哪个构建配置文件”；如需操作发布配置，请显式加上 `--release`。

### 2. 先部署并确认访问入口 {#step-first-deploy}

```bash
lzc-cli project deploy
lzc-cli project info
```

说明：

1. 首次部署如果出现授权提示，按 CLI 输出打开浏览器完成授权即可。
2. `project` 命令默认会优先使用 `lzc-build.dev.yml`。
3. 每个命令都会打印 `Build config` 这一行。
4. `project deploy` 会按 `buildscript` 自动安装依赖并构建前端，不需要额外先执行 `npm install`。
5. `project info` 在应用已运行时会输出 `Target URL`。

### 3. 先打开应用，再看页面提示 {#step-open-app-first}

1. Android/iOS：打开懒猫微服客户端，在启动器中点击应用图标。
2. macOS/Windows：打开桌面客户端，在启动器中点击应用图标。
3. Web：复制 `project info` 输出里的 `Target URL`，在浏览器直接访问。

对于 `hello-vue` 模板，第一次打开应用时通常会先看到一个前端开发引导页。这是开发流程中的正常行为，表示：

1. 当前入口已经进入请求分流脚本（`request inject`）控制。
2. 页面会告诉你这个分流脚本正在等待的本地端口。
3. 如果 dev server 还没启动，页面会直接告诉你下一步该做什么。

### 4. 启动前端 dev server {#step-start-dev-server}

看到页面提示后，再执行：

```bash
npm run dev
```

然后刷新应用页面。

此时入口流量会继续通过应用域名进入，再由请求分流脚本代理到你开发机上的前端 dev server。

### 5. 修改源码并立即验证 {#step-modify-source}

打开 `src/App.vue`，把标题改成你自己的文案，例如把：

```text
Welcome to Lazycat Microserver
```

改为：

```text
Hello from my first LPK
```

保存后直接刷新页面，或等待前端 dev server 自动热更新。

排查问题可用：

```bash
lzc-cli project log -f
```

### 6. 查看 lpk 交付包（可选但推荐） {#step-inspect-lpk}

```bash
lzc-cli project release -o hello.lpk
lzc-cli lpk info hello.lpk
```

你会看到 `format`、`package`、`version` 等信息。
这一步用于确认：`.lpk` 就是这个应用最终交付和安装时使用的包。

## 验证 {#verification}

满足以下条件即通过：

1. `lzc-cli project info` 显示 `Current version deployed: yes`。
2. `lzc-cli project info` 显示 `Project app is running.`。
3. 首次打开应用时，你能看到默认页面或前端开发引导页。
4. 启动 `npm run dev` 后，浏览器与客户端都能进入前端页面。
5. 修改 `src/App.vue` 后，刷新页面或等待热更新即可看到新文案。

## 常见错误与排查 {#troubleshooting}

### 1. `Project app is not running. Run "lzc-cli project start" first.` {#error-app-not-running}

处理：

```bash
lzc-cli project start
```

### 2. `Target URL` 未显示 {#error-target-url-missing}

原因：应用尚未运行，或 `app` 容器未就绪。

处理：

```bash
lzc-cli project start
lzc-cli project info
```

### 3. 页面提示前端 dev server 未就绪 {#error-frontend-not-ready}

处理顺序：

1. 先确认当前页面显示的端口。
2. 在项目目录执行 `npm run dev`。
3. 刷新应用页面。
4. 如仍失败，确认是从执行过 `lzc-cli project deploy` 的那台开发机上启动的 dev server。

## 下一步 {#next}

继续阅读：[开发流程总览](https://developer.lazycat.cloud/dev-workflow.md)