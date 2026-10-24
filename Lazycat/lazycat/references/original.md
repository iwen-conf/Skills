# Lazycat Original App

## 职责范围

只处理原创应用或用户自有项目的懒猫工程基线。目标是让项目具备进入 ship 模式 的条件，不处理第三方开源迁移、许可证筛选、已安装应用更新或商店提审。

第三方 GitHub、自托管开源项目或搬运项目必须改用 intake 模式。

## 输入

- 项目想法、需求说明或已有原创项目路径。
- 目标平台：Web、移动端、桌面端、小程序或纯服务。
- 账号模型：无账号、内置账号、注册登录、OIDC 或其他。
- 文件能力：打开、保存、上传、下载、文件关联或无文件流程。
- 期望交付范围和可运行验证方式。
- 上架平台需求；除非用户明确要求移动端、智慧屏或其他平台，上架默认只准备桌面端。

## 凭据

按 [SKILL.md](../SKILL.md) 的凭据约定读取，只用环境变量，不输出明文。

## 输出

- 原创应用基线结论。
- 必要交付文件清单和实现状态。
- 登录、初始化、持久化、健康检查、文件能力方案。
- 构建或检查结果。
- 是否可交给 ship 模式。

## 前置条件

1. 已确认项目不是第三方迁移项目。
2. 已确认核心使用场景、目标用户和最小可用流程。
3. 如果包含前端或客户端，已确定平台类型；默认技术栈由 `arc:frontend` 决定，除非用户明确指定其他栈。

## 允许执行

- 创建或修改原创项目的业务代码、配置、文档和交付脚本。
- 创建或修改 `package.yml`、`lzc-manifest.yml`、`lzc-build.yml`、`build.sh`、`Makefile`。
- 为原创应用实现登录、初始化、持久化、健康检查和文件选择能力。
- 调用 `arc:frontend` 处理原创 Web、移动端、桌面端或小程序前端。
- 构建本地 `.lpk` 并执行包体和内嵌镜像检查。

## 禁止执行

- 不处理第三方项目许可证、查重或迁移可行性；改用 intake 模式。
- 不提交开发者中心、不发布商店、不处理审核状态；改用 ship 模式。
- 不通过 LPK Inspector 更新已安装应用；改用 update 模式。
- 不写入真实微服域名、微服/商店/开发者中心密码、API Key、Token 或内部未公开地址。应用内固定初始凭据写入 `package.yml.locales.usage`，交给 ship 模式 填开发者中心。
- 不使用 `lzc-build.yml.images`、`embed:<alias>`、包内 `images/` 或 `images.lock`。

## 执行规则

1. 建立最小需求记录：场景、用户、核心流程、账号模型、文件能力。
2. 建立或校准交付文件：`package.yml`、`lzc-manifest.yml`、`lzc-build.yml`、`build.sh`、`Makefile`。
3. 原创前端或客户端必须通过 `arc:frontend` 继承平台默认栈；用户指定例外时记录原因和边界。
4. 文件流程必须在原创业务 UI/代码中接入懒猫文件选择能力，并覆盖懒猫网盘；不要使用迁移用 inject 兜底。
5. 有登录页时实现免密且能进入已登录页（填充并提交）。固定初始账号写入 `locales.usage`。
6. 需要镜像时使用可拉取远程镜像；最终 `.lpk` 不得内嵌镜像。
7. 构建后检查 `.lpk`：大小 `<= 12,000,000` bytes，归档内无 `images/` 和 `images.lock`。
8. 交给 ship 模式 前准备 2-5 张互不相同的桌面 16:9 主流程截图，以及中英文上架文案分工。

## 后置条件

- `package.yml` 包含 `package`、`version`、`name`、`description`、`author`、`license`、`locales`。
- `version` 为严格 `x.x.x`。
- 除非用户明确要求并已验证移动端或智慧屏，交给 ship 模式 时按桌面端上架准备，不默认承诺 `ios`、`android` 或 `tvos` 支持。
- 项目有可执行构建入口和可描述的安装验证路径。
- 构建或检查失败时明确阻塞原因。
- 满足门禁后交给 ship 模式，不在本技能内提审。

## 输出格式

```text
Phase: Original App Baseline
Project: <path/name>

Inputs
- Scenario:
- Target platform:
- Store platform:
- Account model:
- Login lands authenticated:
- File capability:

Actions
- Delivery files:
- Frontend handoff:
- Runtime setup:
- Verification:

Decision: Ready for ship 模式 / Blocked
Blockers:
```
