# Source: https://developer.lazycat.cloud/lzc-cli.md

# 开发环境搭建

懒猫微服应用开发支持 Windows、Linux、macOS。下面是最新的环境搭建步骤。

## 1. 安装基础依赖

1. 安装 [Node.js](https://nodejs.org/zh-cn) 18 及以上版本（建议 LTS）。
2. 安装并登录 [懒猫微服客户端](https://lazycat.cloud/download)。
3. 在应用商店安装 [懒猫开发者工具](https://appstore.lazycat.cloud/#/shop/detail/cloud.lazycat.developer.tools)。

## 2. 安装 lzc-cli

[lzc-cli](https://www.npmjs.com/package/@lazycatcloud/lzc-cli) 是开发者终端工具，用于项目创建、部署、调试和发布。

```bash
npm install -g @lazycatcloud/lzc-cli
lzc-cli --version
```

## 3. 安装系统依赖

根据你的操作系统安装依赖：

* **Ubuntu / Debian**

  ```bash
  sudo apt update
  sudo apt install openssh-client rsync
  ```

* **macOS**

  ```bash
  brew install rsync
  brew install openssh
  ```

  `macOS` 自带 `rsync` 版本较旧，建议使用 Homebrew 版本（v3.2.0+）。

* **Windows**

  Windows 10+ 已内置 `ssh.exe`，通常无需额外安装。

* **Windows (WSL)**

  可参考 [lzc-cli-wsl](https://developer.lazycat.cloud/lzc-cli-wsl)。

  ```bash
  sudo apt update
  sudo apt install openssh-client rsync
  ```

## 4. 准备 SSH Key（仅首次）

Linux/macOS/Git Bash：

```bash
[ -f ~/.ssh/id_ed25519.pub ] || ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
```

## 5. 选择默认目标微服

```bash
lzc-cli box list
lzc-cli box switch <boxname>
lzc-cli box default
```

说明：

1. 如果不是 WSL/LightOS 这类无法直接使用 hclient API 的环境，且你只有一台微服，可以跳过这一步。
2. 如果你有多台微服，建议使用 `lzc-cli box switch <boxname>` 明确切到开发目标微服。
3. 如果你在 WSL/LightOS 等环境中开发，可使用 SSH 方式添加目标微服：

```bash
lzc-cli box add-by-ssh <loginUser> <address>
```

该方案需要目标微服已开通 SSH 功能。

## 6. 上传公钥到开发者工具（仅首次）

```bash
lzc-cli box add-public-key
```

执行后会输出授权链接，使用浏览器打开并完成授权。

## 7. 验证 CLI 能力

```bash
lzc-cli project --help
```

你应能看到以下核心命令：

1. `project deploy`
2. `project info`
3. `project start`
4. `project exec`
5. `project cp`
6. `project log`
7. `project release`
8. `project sync`

## 8. 构建配置文件约定

推荐项目只保留两个构建配置文件：

1. `lzc-build.yml`：默认构建配置，也是 release 配置。
2. `lzc-build.dev.yml`：可选的开发态覆盖配置，只放 dev 相对默认配置的差异项。

默认命令行为：

1. `lzc-cli project deploy`：优先使用 `lzc-build.dev.yml`，不存在时使用 `lzc-build.yml`。
2. `lzc-cli project info/start/exec/cp/log/sync`：默认也遵循同样规则，优先使用 `lzc-build.dev.yml`。
3. `lzc-cli project release`：使用 `lzc-build.yml`。
4. `lzc-cli project build`：默认使用 `lzc-build.yml`。
5. 所有 `project` 命令都会打印实际使用的 `Build config`；如需显式操作 release 配置，可加 `--release`。

开发态常见做法：

```yml
# lzc-build.dev.yml
package_override:
  package: cloud.lazycat.app.helloworld.dev
contentdir:
envs:
  - DEV_MODE=1
```

这样 `project deploy` 默认部署的是独立的 dev 包名，不会影响 release 版本的安装与使用。

补充说明：

1. 上游 `images` 机制用于决定哪些 embed image 会被打包进当前 LPK；本仓库禁用该机制。
2. dev 和 release 都不得使用 `lzc-build.yml.images` 或 `embed:<alias>`。
3. dev 和 release 都应使用普通 remote image，或留空使用系统默认镜像。
4. 若 release 配置了 `contentdir`，dev 可通过空 `contentdir:` 覆盖它，避免误打包本地未构建产物。
5. request inject 等开发态行为由 manifest build 预处理阶段决定是否进入最终包。

## 9. 统一开发模型

建议按下面的模型理解开发流程：

1. 前端开发：`lzc-build.dev.yml + request inject + 开发机 dev server`
2. 后端开发：`lzc-build.dev.yml + request inject + project sync --watch`
3. 发布：`lzc-build.yml + 最终产物镜像`

前端模板建议顺序：

1. 先执行 `lzc-cli project deploy`。
2. 再执行 `lzc-cli project info`，从 CLI 输出里确认实际的 `Build config` 与 `Target URL`。
3. 先打开 LPK。
4. 如果前端 dev server 未启动，应用页面会直接提示端口和下一步命令。
5. 再执行 `npm run dev`。

后端模板建议顺序：

1. 先执行 `lzc-cli project deploy`。
2. 再执行 `lzc-cli project info`，从 CLI 输出里确认实际的 `Build config` 与 `Target URL`。
3. 先打开 LPK。
4. 如果后端服务未启动，应用页面会提示你同步或拷贝代码，并显式启动后端进程。
5. 推荐使用 `lzc-cli project sync --watch` 配合 `lzc-cli project exec /bin/sh`。
6. 如需操作 release 版本，显式加上 `--release`。

这套模式的重点是：

1. request inject 负责开发态入口控制。
2. 开发机服务未就绪时，直接返回静态引导内容，不默认 fallback。
3. release 不携带这些开发态行为。

更完整的说明见：[开发流程总览](https://developer.lazycat.cloud/getting-started/dev-workflow.md)

## 下一步

继续阅读 [Hello World](./hello-world.md)。
