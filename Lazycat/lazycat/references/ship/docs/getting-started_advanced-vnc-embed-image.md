# Source: https://developer.lazycat.cloud/getting-started/advanced-vnc-embed-image.md

# 高级实战：内嵌镜像与上游定制 {#advanced-vnc-embed-image}

本仓库覆盖规则：本页是上游内嵌镜像能力说明，但本技能包禁止采用该路径。最终 `.lpk` 必须小于或等于 `12,000,000` bytes，且不得使用 `lzc-build.yml.images`、`embed:<alias>`、包内 `images/` 或 `images.lock`。需要上游定制时，改用远程镜像桥接：构建镜像、推送公开 registry、执行 `lzc-cli appstore copy-image`、写回 `registry.lazycat.cloud/...`。

## 目标 {#goal}

完成本篇后，你可以明确区分并验证这 3 件事：

1. 理解持久化边界：`/lzcapp/var` 会在重启后保留；运行态目录（例如 `/home/lazycat`）的手工改动会丢失，需要通过镜像/启动脚本固化。
2. 区分被废弃的旧路径 `/lzcapp/run/mnt/home` 与 `/lzcapp/documents`：前者保持原有用户文稿语义，且从 `v1.7.0` 开始只有在管理员明确授权后应用才能访问；后者是当前应用自己的应用文稿根目录，实际数据路径为 `/lzcapp/documents/<uid>`，只有在 `package.yml.permissions` 中声明了 `document.private` 后系统才会提供，且不会因此获得其他应用的应用文稿目录。
3. 掌握 LPK v2 embed image 的基本流程：基于上游镜像做小改动并发布。

## 前置条件 {#prerequisites}

1. 你已完成 [LPK 如何工作：精简机制与最小规范](https://developer.lazycat.cloud/lpk-how-it-works.md)。
2. 目标系统支持 LPK v2 embed image（建议 lzcos 1.5.0+）。
3. 你已经能正常执行 `lzc-cli project deploy`。

## 步骤 {#steps}

### 1. 使用 gui-vnc 模板创建项目 {#step-create-gui-vnc-project}

```bash
lzc-cli project create embed-demo -t gui-vnc
```

然后在应用 ID 提示里按回车使用默认值，或输入你自己的命名。进入目录：

```bash
cd embed-demo
```

模板默认会生成：

1. `lzc-build.yml`：默认构建配置，也是 release 配置。
2. `lzc-build.dev.yml`：开发态覆盖配置，默认包含独立的 dev 包名覆盖（例如 `package_override.package: cloud.lazycat.app.embed-demo.dev`）。

### 2. 查看模板关键配置 {#step-check-template-config}

查看 `lzc-build.yml`：

```bash
cat lzc-build.yml
```

你会看到：

1. `images.app-runtime` 使用 `images/Dockerfile` 构建。
2. 配置了 `upstream-match: registry.lazycat.cloud`。
3. `lzc-manifest.yml` 里通过 `embed:app-runtime` 引用该镜像。

这就是 embed image 的最小可用结构。

如需查看开发态差异，可再看：

```bash
cat lzc-build.dev.yml
```

默认情况下，它只用于 dev 部署与包名隔离。
相关 `project` 命令默认也会优先使用它，并在输出里打印实际的 `Build config`；如需操作 release，请显式加上 `--release`。

### 3. 首次部署并验证可访问 {#step-first-deploy-and-verify}

```bash
lzc-cli project deploy
lzc-cli project info
# Wait until the app reaches running state, then continue.
```

记录 `project info` 输出中的 `Target URL`，在浏览器打开确认可进入桌面页面。

### 4. 先把兼容文稿目录挂到桌面（手动验证） {#step-manual-link-documents}

先进入容器：

```bash
lzc-cli project exec -s desktop /bin/sh
```

进入容器 shell 后，手动创建文稿目录软链接：

```bash
mkdir -p /home/lazycat/Desktop
ln -svfn /lzcapp/run/mnt/home "/home/lazycat/Desktop/Documents (ReadWrite)"
ls -la /home/lazycat/Desktop
ls -la /lzcapp/run/mnt/home
exit
```

完成后在 VNC 页面刷新桌面，你应能看到：

1. `Documents (ReadWrite)` 入口。
2. 打开后可以访问 `/lzcapp/run/mnt/home` 对应的文稿数据。

### 5. 重启应用，观察软链接丢失 {#step-restart-and-observe-link-loss}

执行：

```bash
lzc-cli project start --restart
```

然后再次进入容器或刷新桌面查看。你会发现刚才手动创建在 `/home/lazycat/Desktop` 下的软链接重启后一定会丢失。

这是预期行为：`/lzcapp/var` 之外的数据在重启后不保证保留。

延伸阅读：

1. [文件访问（`/lzcapp/var`、`/lzcapp/run/mnt/home` 与 `/lzcapp/documents`）](advanced-file.md)

### 6. 用 Dockerfile 把软链接逻辑固化到镜像 {#step-persist-link-via-dockerfile}

为了让桌面入口每次启动都自动存在，建议把创建软链接的逻辑放入镜像启动脚本（而不是手工执行）。

`gui-vnc` 模板已包含 `images/startup-script.sh`，请确认脚本里有类似逻辑：

```bash
mkdir -p /home/lazycat/Desktop
ln -svfn /lzcapp/run/mnt/home "/home/lazycat/Desktop/Documents (ReadWrite)"
```

并确认 `images/Dockerfile` 中复制了启动脚本；`startup-script.desktop` 这一行默认是注释状态，你需要手动反注释后再部署：

```dockerfile
COPY --chown=lazycat:kasm-user startup-script.sh /home/lazycat/.config/autostart/startup-script.sh
# COPY --chown=lazycat:kasm-user startup-script.desktop /home/lazycat/.config/autostart/startup-script.desktop
RUN chmod +x /home/lazycat/.config/autostart/startup-script.sh
```

把 `COPY --chown=lazycat:kasm-user startup-script.desktop ...` 前面的 `#` 去掉后，保存文件。

重新部署后直接验证（`project deploy` 会自动启动应用）：

```bash
lzc-cli project deploy
lzc-cli project info
```

等待 `project info` 显示应用进入 running 状态后，再继续。

然后在 VNC 桌面确认：`Documents (ReadWrite)` 会自动出现。

说明：LPK 的定位是把应用封装给他人“一键即用”。为保证交付环境始终一致、可复现，系统只会持久化显式声明的数据目录，其他路径的文件改动在重启后都会被清理，避免出现意外的永久存储。

延伸阅读：

1. [LightOS 场景（占位）](advanced-lightos.md)

### 7. 验证产物包含 embed image {#step-verify-embedded-image-output}

```bash
lzc-cli project release -o embed-demo.lpk
lzc-cli lpk info embed-demo.lpk
```

你应能看到：

1. 输出里 `image_count` 大于 `0`。
2. `images:` 列表中存在 `app-runtime` 别名与层来源摘要（`embedded_layers` / `upstream_layers`）。

## 验证 {#verification}

满足以下条件即通过：

1. `project info` 显示 `Current version deployed: yes`。
2. `Target URL` 可打开桌面页面。
3. `Documents (ReadWrite)` 可在重启后继续出现（通过镜像启动脚本自动创建）。
4. `lzc-cli lpk info embed-demo.lpk` 显示包含 embed image 摘要信息。

## 常见错误与排查 {#troubleshooting}

### 1. `Cannot resolve embedded image alias` {#error-embedded-alias-not-found}

原因：`embed:<alias>` 与 `images.<alias>` 不一致。

处理：检查 `lzc-manifest.yml` 和 `lzc-build.yml` 的 alias 后重新 `project deploy`。

### 2. 构建很慢 {#error-build-too-slow}

排查建议：

1. 第一次构建慢是正常现象（需要拉取上游层）。
2. 第二次仅小改 Dockerfile 时，通常会明显快于首次。
3. 用 `lzc-cli project log -f` 观察是否反复拉取相同层。

### 3. 页面无法打开或黑屏 {#error-page-black-or-unreachable}

处理顺序：

1. 检查 `application.routes` 是否指向 `http://desktop:6901/`。
2. 执行 `lzc-cli project log -s desktop -f` 查看桌面服务日志。
3. 执行 `lzc-cli project exec -s desktop /bin/sh` 确认进程是否正常。

### 4. 看不到文稿目录或桌面入口 {#error-documents-link-missing}

处理：

1. 检查 `images/startup-script.sh` 是否包含创建软链接命令。
2. 检查 `images/Dockerfile` 里 `COPY startup-script.desktop ...` 是否已反注释。
3. 检查 `images/startup-script.desktop` 是否存在且 `Exec` 指向 `startup-script.sh`。
4. 重新执行 `lzc-cli project deploy`，然后用 `lzc-cli project info` 确认进入 running。

## 延伸阅读 {#further-reading}

1. [lzc-build.yml 规范](build.md)
2. [lzc-manifest.yml 规范](https://developer.lazycat.cloud/spec/manifest.md)
3. [文件访问（`/lzcapp/var`、`/lzcapp/run/mnt/home` 与 `/lzcapp/documents`）](advanced-file.md)
4. [LightOS 场景（占位）](advanced-lightos.md)
