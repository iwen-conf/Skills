# Lazycat 远程镜像桥接策略

当仓库里有 `Dockerfile`，但没有现成的公网镜像，或者你需要把本地改动打进自定义镜像时，优先使用本文档定义的“远程镜像桥接”路线。

强约束：最终 `.lpk` 必须小于或等于 12 MB（按 `12,000,000` bytes 检查），且禁止内嵌镜像。不得使用 `lzc-build.yml.images`、`embed:<alias>`、包内 `images/` 或 `images.lock`。

## 决策矩阵

| 维度 | 路线 A：本地构建并推送远程镜像 | 路线 B：直接把源码装进 LPK |
| --- | --- | --- |
| 适用复杂度 | 高，多阶段构建、复杂依赖、现成 Docker 运行成熟 | 低，单二进制、静态站点、小工具 |
| 构建耗时 | 长，通常大于 5 分钟 | 短，通常小于 2 分钟 |
| 交付稳定性 | 高，镜像可先在本地验收 | 中，依赖 LPK 内部构建 |
| 默认建议 | 生产级应用、SaaS、复杂移植项目 | 极简工具、脚本、纯静态页面 |

## 路线 A：远程镜像桥接

复杂项目不要把主镜像构建塞进 LPK。标准做法是：

1. 本地构建 `linux/amd64` 镜像。
   ```bash
   docker buildx build --platform linux/amd64 -t your-hub-user/app-name:v1.0 --load .
   ```
2. 将镜像推送到 Docker Hub。
   ```bash
   docker push your-hub-user/app-name:v1.0
   ```
3. 用懒猫 CLI 同步到官方镜像仓。
   ```bash
   lzc-cli appstore copy-image your-hub-user/app-name:v1.0
   ```
4. 将返回的 `registry.lazycat.cloud/...` 镜像地址回填到 `lzc-manifest.yml`。
5. `lpk` 只保留这些内容：
   - `package.yml`
   - `lzc-build.yml`
   - `lzc-manifest.yml`
   - `runtime/` 脚本
   - 图标和静态资源
6. 构建后检查 `.lpk` 大小不超过 `12,000,000` bytes，并确认归档内没有 `images/` 或 `images.lock`。

### 为什么默认选这条路

- 上游 Dockerfile 的行为与生产环境更接近，减少“LPK 内再造一套构建逻辑”的偏差。
- 镜像可以先在本地启动和验收，再进入 `copy-image`。
- `copy-image` 只接受服务端可拉取的镜像地址，本地镜像名本身无法直接同步。
- 大部分复杂项目真正变化的是镜像，不是 LPK 静态内容。

## 路线 B：直接源码集成

如果项目非常简单，可以忽略 `Dockerfile`，直接用 Lazycat 原生入口：

1. 二进制模式：在 `build.sh` 中产出可执行文件，用 `exec://` 暴露入口。
2. 通用基础镜像：使用 `alpine`、`node:alpine` 等基础镜像，并把运行所需文件放进 LPK。

路线 B 只适用于能保持 `.lpk` 小于或等于 `12,000,000` bytes 的极简交付；超过上限时必须改为远程镜像桥接或进一步瘦身。

## 风险提示

- 如果 `Dockerfile` 依赖私有基础镜像、私有包源或构建密钥，必须先补足可公开或可代理的构建链路，再走路线 A。
- 一定要显式构建 `linux/amd64`。仅有 `arm64` 的镜像在多数 Lazycat 节点上不可用。
- 不要把应用主镜像层塞进 LPK。那会让交付链路更重，也违反本仓库禁止内嵌镜像和 12 MB 包体上限。
