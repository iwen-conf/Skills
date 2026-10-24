# Source: https://developer.lazycat.cloud/getting-started.md

# 入门路线 {#getting-started-overview}

本栏目面向第一次开发懒猫微服应用的开发者，目标是用最短路径完成一次真实部署：

本仓库覆盖规则：虽然上游文档介绍了 LPK v2 内嵌镜像能力，但本技能包禁止使用 `lzc-build.yml.images`、`embed:<alias>`、包内 `images/` 和 `images.lock`。所有最终 `.lpk` 必须小于或等于 `12,000,000` bytes；需要镜像定制时走远程镜像桥接和 `copy-image`。

说明：

* 本栏目默认以 `lzcos v1.5.0+` 为前提。
* 若你要跟随后续文档体验 `LPK v2` / `package.yml` 新流程，请配合 `lzc-cli v2.0.0+`。

1. 先部署成功并在多端看到结果。
2. 理解前端开发、后端开发与发布包产出的分工。
3. 再接入后端与 HTTP 路由。
4. 最后理解 LPK 运行机制和进阶镜像构建。

## 适用范围 {#scope}

* 你第一次接触懒猫微服应用开发。
* 你希望先跑通完整链路，再学习细节。
* 你使用 `lzc-cli` 进行本地开发与部署。

## 学习顺序 {#learning-path}

| 阶段 | 文档 | 你会得到什么 |
| --- | --- | --- |
| 1 | [开发者环境搭建](https://developer.lazycat.cloud/env-setup.md) | 本机开发环境可用，`lzc-cli` 可连目标微服 |
| 2 | [5 分钟完成 Hello World 并多端验证](https://developer.lazycat.cloud/hello-world-fast.md) | 第一个应用部署成功，可在 Android/iOS/macOS/Windows/Web 查看 |
| 3 | [开发流程总览](https://developer.lazycat.cloud/dev-workflow.md) | 理解 `lzc-build.dev.yml`、请求分流脚本（`request inject`）和 `project sync --watch` 如何组成完整开发流程 |
| 4 | [有后端时如何通过 HTTP 路由对接](https://developer.lazycat.cloud/http-route-backend.md) | 理解 `application.routes`（请求该转到哪里）与后端联调边界 |
| 5 | [LPK 如何工作：精简机制与最小规范](https://developer.lazycat.cloud/lpk-how-it-works.md) | 形成构建、安装、运行的整体心智模型 |
| 6 | 远程镜像桥接与上游定制 | 本仓库禁用内嵌镜像；基于 Dockerfile 定制镜像后推送到公开 registry，再用 `copy-image` 写回 manifest |

## 你需要准备什么 {#what-to-prepare}

1. 一台已安装懒猫微服客户端的设备。
2. 一个可访问的目标微服（局域网或互联网都可以）。
3. 本地可用的 Node.js 开发环境。

## 成功标准 {#success-criteria}

完成本栏目后，你应该能做到：

1. 独立创建一个 LPK 项目并部署到目标微服。
2. 理解 `project` 命令默认优先使用 `lzc-build.dev.yml`，并通过输出里的 `Build config` 这一行确认“这次实际用了哪个构建配置文件”。
3. 理解前端开发如何通过请求分流脚本转到开发机，后端开发如何通过 `project sync --watch` 落到真实运行环境。
4. 在客户端和浏览器两条路径都能验证应用可用。
5. 在需要后端时，正确配置 HTTP 路由。
6. 在需要镜像定制时，使用远程镜像桥接和 `copy-image`，不要使用内嵌镜像机制。
7. 使用 `lzc-cli project release` 产出可分发的 `.lpk` 发布包。
