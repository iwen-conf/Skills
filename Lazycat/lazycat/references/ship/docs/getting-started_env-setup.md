# Source: https://developer.lazycat.cloud/getting-started/env-setup.md

# 开发者环境搭建 {#env-setup}

## 目标 {#goal}

完成本篇后，你的本地机器可以直接使用 `lzc-cli` 连接目标微服，并具备后续构建与部署能力。

## 前置条件 {#prerequisites}

1. 你有一台可访问的懒猫微服。
2. 你的账号可以登录懒猫微服客户端。
3. 你的终端可执行 `node` 和 `npm`。

## 步骤 {#steps}

### 1. 安装基础依赖 {#step-install-dependencies}

1. 安装 Node.js 18+（建议 LTS 版本）。
2. 安装并登录懒猫微服客户端。
3. 在微服应用商店安装“懒猫开发者工具”。

### 2. 安装 lzc-cli {#step-install-lzc-cli}

```bash
npm install -g @lazycatcloud/lzc-cli
lzc-cli --version
```

如果 `lzc-cli --version` 能输出版本号，说明安装成功。

### 3. 准备 SSH Key（仅首次） {#step-prepare-ssh-key}

Linux/macOS/Git Bash 可直接执行：

```bash
[ -f ~/.ssh/id_ed25519.pub ] || ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
```

### 4. 选择默认目标微服 {#step-select-default-target}

```bash
lzc-cli box list
lzc-cli box switch <boxname>
lzc-cli box default
```

将 `<boxname>` 替换成你自己的目标微服名。

说明：

1. 如果不是 WSL/LightOS 这类无法直接使用 hclient API 的环境，且你只有一台微服，可以跳过这一步。
2. 如果你有多台微服，建议使用 `lzc-cli box switch <boxname>` 明确切到开发目标微服。
3. 如果你在 WSL/LightOS 等环境中开发，可使用 SSH 方式添加目标微服：

```bash
lzc-cli box add-by-ssh <loginUser> <address>
```

该方案需要目标微服已开通 SSH 功能。

### 5. 上传公钥到开发者工具（仅 hclient 接入模式首次） {#step-upload-public-key}

仅当你通过 hclient 模式接入目标微服时，才需要执行这一步：

```bash
lzc-cli box add-public-key
```

执行后会输出一个授权链接，使用浏览器打开并完成授权。

如果你是通过 `lzc-cli box add-by-ssh <loginUser> <address>` 添加的目标微服，请跳过这一步。\
`add-by-ssh` 模式不需要、也无法成功执行 `lzc-cli box add-public-key`。

### 6. 创建本地工作目录 {#step-create-workspace}

```bash
mkdir -p <your-workspace-dir>
```

## 验证 {#verification}

执行以下检查：

```bash
lzc-cli --version
lzc-cli box default
lzc-cli project --help
```

满足以下条件即通过：

1. `lzc-cli` 版本号可见。
2. `box default` 能输出默认微服名。
3. `project --help` 能看到 `deploy/start/info/exec/cp/log/release` 等命令。

## 常见错误与排查 {#troubleshooting}

### 1. `No default box configured` {#error-no-default-box}

原因：还没有默认微服。

处理：

```bash
lzc-cli box list
lzc-cli box switch <boxname>
```

### 2. `permission denied (publickey)` {#error-publickey-denied}

原因：当前 SSH 公钥未被目标微服接受（常见于 `add-by-ssh` 模式）。

处理：

1. 如果你使用的是 hclient 模式，执行 `lzc-cli box add-public-key`，并按提示打开浏览器完成授权。
2. 如果你使用的是 `add-by-ssh` 模式，不要执行 `box add-public-key`；请改为检查目标微服的 SSH 公钥授权配置是否正确。

### 3. `lzc-cli: command not found` {#error-cli-not-found}

原因：全局 npm bin 目录不在 PATH。

处理：确认 npm 全局安装路径已加入 shell 的 PATH，或重新打开终端。

## 下一步 {#next}

继续阅读：[5 分钟完成 Hello World 并多端验证](https://developer.lazycat.cloud/hello-world-fast.md)