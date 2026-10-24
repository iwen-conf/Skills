# Lazycat App Shipping

## 职责范围

只处理已准备好的懒猫应用上架发布：打包、镜像同步、安装验证、开发者中心资料、提审和发布后检查。它不创建原创应用、不发现迁移候选、不做许可证门禁、不更新用户已安装应用。

## 输入

- 已准备好的原创应用项目，且已通过 original 模式 基线。
- 或已准备好的迁移项目，且已通过：
  - intake 模式
  - boundary 模式
- 项目路径、目标版本、上架资料、截图、测试账号或免密登录说明。
- 上架平台选择；除非用户明确要求并提供移动端、智慧屏等验证证据，默认只选择桌面端。
- 如需提审：开发者中心登录态或凭据变量。

## 凭据

按 [SKILL.md](../SKILL.md) 的凭据约定读取，只用环境变量，不输出明文。

## 输出

- 上架前检查结果。
- 镜像同步和 manifest 回写结果。
- `.lpk` 构建结果、包体大小和内嵌镜像检查结果。
- `lzc-cli lpk info <file.lpk>` 摘要。
- 安装验证、核心流程验证、平台选择、截图/资料状态。
- 提审状态或阻塞原因。

## 前置条件

1. 原创项目必须已完成 original 模式 后置条件。
2. 迁移项目必须有 intake 模式 和 boundary 模式 输出。
3. 迁移项目许可证不得为 `Unclear` 或 `Blocked`。
4. 迁移项目不得依赖未授权的业务代码修改。
5. 迁移项目必须带有原作者名称和源项目或代码地址，且证据来自许可证门禁记录或上游仓库。
6. 提交开发者中心前，所有必填资料必须完整，不得有占位符或待补内容。

## 允许执行

- 修改上架相关 metadata、包装层、manifest、构建脚本、图标、截图和文档。
- 执行 `lzc-cli appstore copy-image` 并把返回的 `registry.lazycat.cloud/...` 写回源 manifest。
- 执行 `make build`、`lzc-cli project release` 或项目等价打包命令。
- 执行 `lzc-cli lpk info <file.lpk>`。
- 经用户确认后执行安装、提审或发布相关命令。
- 审核失败时修改包装层、运行时适配层、上架资料和审核说明。

## 禁止执行

- 不接收许可证不明、不可商用、不可再分发或查重不可接受的迁移项目。
- 未经当前任务明确授权，不修改迁移项目上游业务代码。
- 不把截图、网页表单或本地开发环境当作真实安装验证。
- 不使用 `lzc-build.yml.images`、`embed:<alias>`、包内 `images/` 或 `images.lock`。
- 开发者中心只写应用内固定初始凭据或公开测试账号。微服、商店、开发者中心登录凭据、Token、内部地址和真实微服域名不得出现在资料中。
- 不上架色情、赌博、毒品、空投、破解软件或违法内容。
- 不上架没有完整应用内主流程的开发库、SDK 或中间件。
- 不处理已安装应用批量更新；改用 update 模式。

## 上架门禁

- `package.yml` 包含 `package`、`version`、`name`、`description`、`author`、`license`、`locales`。
- `version` 为严格 `x.x.x`。
- 除非用户明确要求并已验证移动端或智慧屏，开发者中心上架平台默认只选择桌面端。
- 桌面端默认上架时，`package.yml` 必须声明不支持移动端和智慧屏：

```yaml
unsupported_platforms:
  - ios
  - tvos
  - android
```

用户或旧文档称为 manifest 平台字段时，LPK v2 按 `package.yml` 静态元数据处理；只有旧版项目仍使用 manifest 字段时才同步保持一致。

- 如果 `package.yml.homepage` 是 GitHub URL，`package.yml.author` 必须逐字符等于 URL owner 段。
- `lzc-manifest.yml` 服务、路由、持久化、健康检查、环境变量完整。
- 镜像型项目已 `copy-image`，manifest 使用最终 `registry.lazycat.cloud/...` 地址。
- 最终 `.lpk` 大小 `<= 12,000,000` bytes。
- 最终 `.lpk` 不包含 `images/` 或 `images.lock`。
- 最终 `.lpk` 已在真实懒猫微服安装成功，应用可打开，核心业务能力和关键用户流程已验证正常；未验证或异常时必须停止提审。
- 审核员必须能只凭开发者中心资料和安装后的应用走完主流程。细则见 [store-publish.md](ship/lpk/store-publish.md) 第 0.4 节。
- 有登录页的应用：OIDC 或 inject 必须进入已登录页；inject 须填充并提交。固定初始账号必须写入开发者中心「应用描述」和「版本更新说明」的每一种上架语言。
- 有文件打开/保存/导入/导出/上传的应用：懒猫文件选择必须可用，且能选懒猫网盘。仅有网盘右键/`file_handler` 不够。
- `lzc-deploy-params.yml`：跳过安装向导后主流程仍须可用。缺少后会 500/404 的参数必须必填，或给出真正能用的默认值。
- 开发者中心截图 2-5 张，默认真实桌面端 16:9；文件 hash 两两不同，每张对应主流程不同步骤。迁移项目图标与上游官方图标一致。
- 简体中文上架字段用中文，英语上架字段用英文。英文界面必须在开发者中心填写英文简介、描述和更新说明；不能用 manifest/package 本地化代替。
- 截图、图标、描述来自真实运行版本。桌面端界面按桌面布局，不能是拉伸的移动端页。
- 开发者中心资料完整，审核单已挂上实际提审 `.lpk`，`lzc-cli lpk info` 来自该包。
- 使用 `compose_override` 时，上架描述说明用途，并在提审前提示用户向官方或开发者群报备。
- 迁移项目在开发者中心必须取消勾选“应用程序为本人原创开发或本人是源作者”，并填写原作者名称和源项目或代码地址。

## 执行规则

1. 确认项目来源：原创或迁移；读取对应前置结论。
2. 读取项目文件：`package.yml`、`lzc-manifest.yml`、`lzc-build.yml`、README、`docs/release-prep/`。
3. 同步或确认镜像，确保 manifest 引用最终可拉取镜像。
4. 构建 `.lpk`。
5. 检查包体大小和内嵌镜像产物。
6. 运行 `lzc-cli lpk info <file.lpk>` 并记录摘要。
7. 使用最终 `.lpk` 安装到真实懒猫微服，确认安装成功、应用可打开、启动正常。有登录页时确认免密进入已登录页，而不是停在已填充的登录表单。
8. 验证核心业务能力和关键用户流程，包括创建/读取/更新/删除等应用主要功能、重启后持久化、文件选择（含网盘）、卸载/升级风险；失败或未验证时停止提审。
9. 准备上架平台选择、2-5 张互不相同的桌面 16:9 截图、图标、中英文描述、更新说明中的应用内初始凭据、复现步骤、限制说明和 LPK 信息；默认只选择桌面端。
10. 桌面端默认上架时，确认 `package.yml.unsupported_platforms` 包含 `ios`、`tvos`、`android`。
11. 如果应用运行界面或主要用户文案为英文，提交前确认开发者中心上架页面已填写英文简介、描述和更新说明；不要把 manifest/package 本地化当作已完成上架页英文描述。
12. 若项目来源为迁移，提交前确认开发者中心未勾选“应用程序为本人原创开发或本人是源作者”，并已填写原作者名称和源项目或代码地址。
13. 填写开发者中心后、点提交前，回读「应用描述」「版本更新说明」、英文资料、截图、图标和已挂 LPK，对照 [store-publish.md](ship/lpk/store-publish.md) 第 0.4 节。
14. 提交开发者中心；记录版本、时间、状态和证据。
15. 审核通过后验证商店可见性和安装版本；审核失败按问题归类修复。

## 审核失败处理

按审核原话归类，只改对应层：

- `Packaging/runtime`: manifest、镜像、路由、健康检查、持久化、启动脚本、初始化。
- `Login/credentials`: OIDC/inject、自动提交、开发者中心描述和更新说明中的应用内初始凭据。
- `File capability`: 懒猫文件选择、网盘路径、`file_handler`。
- `Deploy params`: 必填项、可用默认值、向导不可被跳过的关键参数。
- `Store asset/docs`: 描述语言、截图去重、图标、版本、桌面布局、英文资料。
- `Privileged runtime`: `compose_override` 说明与官方报备。
- `Product validity`: 缺少应用内主流程的库/中间件、核心按钮无响应。
- `Business source required`: 迁移项目必须修改上游业务代码；停止普通上架，等待用户明确授权业务范围。

## 本地参考

- [store-publish.md](ship/lpk/store-publish.md)（上架流程与第 0.4 节审核员独立完成门禁）
- [docs/INDEX.md](ship/docs/INDEX.md)
- [lpk/](ship/lpk/)

## 后置条件

- 已产出通过包体和内嵌镜像检查的最终 `.lpk`，或明确阻塞原因。
- 已记录实际 `.lpk` 信息。
- 如已安装或提审，已记录目标、版本、时间和验证证据。

## 输出格式

```text
Phase: Shipping
Project: <path/name>
Source: Original / Migration

Preflight
- Metadata:
- Source gates:
- Image:
- Store platforms:
- Screenshots (count/hash uniqueness/16:9):
- Store materials:
- Listing credentials:
- English description:
- Originality checkbox:
- Original author:
- Source project/code:
- compose_override:
- Blockers:

Verification
- Build:
- LPK size:
- Embedded image check:
- LPK info:
- Install:
- Runtime:
- Login lands authenticated:
- Core flow:
- Persistence:
- File picker / 网盘:
- Business capability:

Submission
- Status:
- Evidence:
- Next:
```
