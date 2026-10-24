# Lazycat Installed App Update

## 职责范围

只处理用户微服中已安装应用的版本更新：从用户提供的 LPK Inspector 获取已安装应用和当前 `.lpk`，对比 GitHub 上游最新版本和镜像，重新生成可安装 `.lpk` 并在用户确认后安装验证。

它不做新应用上架、不做开发者中心提审、不做迁移候选发现。

## 输入

- 用户提供的 LPK Inspector URL；不得写入仓库文件或长期记录。
- 应用范围：全部已安装应用，或指定包名/应用名。
- 工作目录：用于下载、解包、clone、构建镜像、生成 `.lpk`。
- 镜像 registry：仅当需要自建镜像时使用；必须是用户明确授权可推送的公开 registry。
- 安装目标：如需安装验证，使用 `lzc-cli box default` 获取默认微服。

## 凭据

按 [SKILL.md](../SKILL.md) 的凭据约定读取，只用环境变量，不输出明文。

## 输出

- 已安装应用清单和当前版本。
- 当前 `.lpk` 下载、解包和 metadata 结果。
- GitHub 上游最新 release/tag、更新时间、镜像或 Dockerfile 情况。
- 更新决策、镜像路径、manifest 回写计划、数据兼容性和回滚方案。
- 新 `.lpk` 构建结果、包体门禁、安装验证结果。

## 前置条件

1. 用户提供 LPK Inspector 地址或等价的已安装应用清单来源。
2. 需要浏览器时必须使用 `agent-browser`；不可用时输出阻塞。
3. 需要安装覆盖现有应用前，必须取得用户确认。
4. 需要构建并推送自建镜像前，必须取得用户对目标公开 registry 的授权。

## 允许执行

- 只读打开 LPK Inspector 并下载当前 `.lpk`。
- 解包 `.lpk` 并读取 `package.yml`、manifest、`content.tar` 中的 metadata。
- 只读查询 GitHub release、tag、默认分支更新时间、Dockerfile、Compose、README 和镜像说明。
- 执行 `lzc-cli appstore copy-image <pullable-image>`。
- 在不修改业务代码的前提下构建镜像、推送用户授权的公开 registry，再执行 `copy-image`。
- 回写 manifest 镜像地址、更新包装层版本、重新打包 `.lpk`。
- 用户确认后安装新 `.lpk` 并验证运行状态。

## 禁止执行

- 不把真实 `*.heiyu.space` 域名、Token、Cookie、账号密码写入仓库文件、提交信息或长期报告。
- 不提交开发者中心、不发布商店；需要上架时改用 ship 模式。
- 不发现新迁移候选、不审查候选许可证；需要候选门禁时改用 intake 模式。
- 未经当前任务明确授权，不修改上游业务前端、后端、数据库 schema、测试或 fixture。
- 不对第三方 GitHub 仓库创建 issue、PR、fork、评论、star 或其他可见互动。
- 不使用 `lzc-build.yml.images`、`embed:<alias>`、包内 `images/` 或 `images.lock`。
- 不对 `latest` 镜像 tag 直接下可复现结论；必须记录 digest、release 关联或证据不足。

## 执行规则

1. 准备工作区，至少区分 `downloads/`、`unpacked/`、`repos/`、`dist/`。
2. 打开 LPK Inspector，记录应用名、包名、已安装版本、更新时间、下载链接、当前镜像、homepage/source URL。
3. 下载目标 `.lpk` 并运行 `lzc-cli lpk info <file.lpk>` 或等价检查。
4. 解包并读取 `package.yml`、manifest 和可见 metadata。
5. 定位 GitHub 上游；找不到时输出 `Blocked: Missing Source`。
6. 对比 release/tag、更新时间、当前镜像、推荐镜像、Dockerfile 和运行参数变化。
7. 选择更新路径：
   - 上游有可拉取镜像：`copy-image` 后回写 manifest。
   - 上游只有 Dockerfile：不改业务代码构建镜像，推送公开 registry，再 `copy-image`。
   - 无镜像更新：仅更新可验证的包装层内容或 metadata。
8. 更新 `package.yml.version`：严格 `x.x.x`；跟随上游 release/tag，或仅包装层更新时递增 patch 并记录原因。
9. 重打包 `.lpk`，保持原 LPK 结构或项目原构建流程。
10. 检查包体大小 `<= 12,000,000` bytes，归档内无 `images/` 和 `images.lock`，manifest 无 `embed:<alias>`。
11. 安装前输出更新计划、数据兼容性和回滚方案，并等待用户确认。
12. 安装后验证启动、核心页面、免密进入已登录页、重启后持久化、文件选择（含网盘）；查看容器状态时使用 `lzc-cli docker` 前缀命令。

## 决策

- `No Update Needed`: 当前版本、更新时间和镜像不落后于上游。
- `Ready: Copy Image`: 上游已有可拉取镜像，只需 `copy-image`、回写 manifest、重打包。
- `Ready: Build Image`: 上游无镜像但可不改业务代码构建；等待或使用用户授权 registry。
- `Ready: Repack Only`: 不涉及镜像，只需更新包装层内容或 metadata。
- `Needs Manual Review`: 上游、版本、镜像来源、数据兼容性或 LPK 解包证据不足。
- `Blocked: Missing Source`: 找不到 GitHub 上游或源码不可访问。
- `Blocked: Business Code`: 更新必须修改上游业务代码。
- `Blocked: Image Not Pullable`: 镜像不可被 `copy-image` 服务端拉取且无公开 registry 桥接。
- `Blocked: Package Gate`: 新 `.lpk` 超过 `12,000,000` bytes 或包含内嵌镜像产物。

## 后置条件

- 每个目标应用都有更新决策和证据。
- 如生成新 `.lpk`，必须通过包体、内嵌镜像和 `lzc-cli lpk info` 检查。
- 如安装覆盖，必须记录用户确认、安装结果和运行验证。
- 如阻塞，必须给出阻塞类型和缺失证据。

## 输出格式

```text
Phase: Installed App Update
LPK Inspector: <lpk-inspector-url>
Workdir: <path>

App
- Name:
- Package:
- Installed version:
- Installed updated at:
- Current LPK:
- Current image:

Upstream
- Repository:
- Latest release/tag:
- Latest updated at:
- Latest image:
- Runtime changes:

Decision: No Update Needed / Ready: Copy Image / Ready: Build Image / Ready: Repack Only / Needs Manual Review / Blocked: Missing Source / Blocked: Business Code / Blocked: Image Not Pullable / Blocked: Package Gate

Update Plan
- Image path:
- Manifest change:
- Version change:
- Data compatibility:
- Rollback:

Verification
- copy-image:
- Build/repack:
- LPK size:
- Embedded image check:
- LPK info:
- Install:
- Runtime:
- Login lands authenticated:
- Persistence:
- File picker / 网盘:
```
