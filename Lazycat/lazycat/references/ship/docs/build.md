# Source: https://developer.lazycat.cloud/build.md

# lzc-build.yml 规范文档

本仓库覆盖规则：最终 `.lpk` 必须小于或等于 `12,000,000` bytes，且禁止内嵌镜像。不得使用顶层 `images` 配置、`embed:<alias>` 引用，最终包内不得包含 `images/` 或 `images.lock`。需要镜像时使用可拉取远程镜像，并在提审前通过 `lzc-cli appstore copy-image` 写回 `registry.lazycat.cloud/...`。

## 一、概述

构建配置只分为两个层次：

1. `lzc-build.yml`：默认构建配置，也是 release 配置。
2. `lzc-build.dev.yml`：可选的开发态覆盖配置，只放 dev 相对默认配置的差异项。

推荐项目仅保留这两个文件。

`package_override` 的语义如下：

1. 只覆盖最终产物里的 `package.yml`。
2. 同名字段按顶层整体覆盖，不做递归 merge。
3. 顶层写空值表示清空对应字段。
4. 若 `lzc-build.dev.yml` 里也定义了 `package_override`，它会整体覆盖 `lzc-build.yml` 里的 `package_override`。

默认命令约定：

1. `lzc-cli project deploy`：优先读取 `lzc-build.dev.yml`，不存在时读取 `lzc-build.yml`。
2. `lzc-cli project info/start/exec/cp/log/sync`：默认也遵循同样规则，优先读取 `lzc-build.dev.yml`。
3. `lzc-cli project release`：读取 `lzc-build.yml`。
4. `lzc-cli project build`：默认读取 `lzc-build.yml`，也可通过 `-f` 显式指定其他文件。
5. 所有 `project` 命令都会打印实际使用的 `Build config`；如果要显式操作正式配置，可加 `--release`。

## 二、顶层数据结构 `BuildConfig`

### 2.1 基本信息 {#basic-config}

| 字段名 | 类型 | 描述 |
| ---- | ---- | ---- |
| `buildscript` | `string` | 可以为构建脚本的路径地址或者 sh 的命令 |
| `manifest` | `string` | 指定 lpk 包的 `lzc-manifest.yml` 文件路径 |
| `contentdir` | `string` | 可选；指定打包的内容目录。未配置或显式写空值时不会生成 `content.tar` / `content.tar.gz` |
| `pkgout` | `string` | 指定 lpk 包的输出路径 |
| `icon` | `string` | 指定 lpk 包 icon 的路径，如果不指定将会警告，目前仅允许 png 后缀的文件 |
| `package_override` | `map[string]any` | 可选；按顶层字段整体覆盖最终 `package.yml`，不做递归 merge；顶层写空值表示清空对应字段；其中 `package_override.package` 会参与最终 LPK 文件名与包名校验 |
| `envs` | `[]string` | 可选；构建期变量列表，支持 `KEY=VALUE` 字符串数组 |
| `images` | `map[string]ImageBuildConfig` | 上游 LPK v2 内嵌镜像能力；本仓库禁用，不得用于最终 `.lpk` |
| `compose_override` | `ComposeOverrideConfig` | 高级 compose override 配置，**需要更新 lzc-os 版本 >= v1.3.0** |

### 2.2 文件组织约定 {#file-layout}

推荐项目结构：

```text
.
├── lzc-build.yml
├── lzc-build.dev.yml
├── lzc-manifest.yml
└── package.yml
```

约定如下：

1. `lzc-build.yml` 保存默认构建配置，也就是正式发布时使用的配置。
2. `lzc-build.dev.yml` 只保存开发态差异，例如：
   * `package_override.package: cloud.lazycat.app.demo-app.dev`
   * 若 release 配置了 `contentdir`，dev 可显式写 `contentdir:` 空值覆盖
   * 开发态专用 `buildscript`
   * 开发态专用 `envs`
3. image-only release 场景可以完全省略 `contentdir`；此时最终 LPK 不会生成 `content.tar` / `content.tar.gz`。
4. 不要把完整配置重复拷贝到 `lzc-build.dev.yml`；该文件应尽量只保留差异项。

## 三、构建期变量 `envs` {#envs}

`envs` 是一组只在构建阶段生效的变量。

行为如下：

1. `envs` 必须是 `KEY=VALUE` 字符串数组。
2. `KEY` 必须符合环境变量命名规则：`^[A-Za-z_][A-Za-z0-9_]*$`。
3. 不允许重复 key。
4. 这些变量会注入 `buildscript` 进程环境。
5. 这些变量也会参与 manifest build 预处理。
6. 它们不会写入最终 LPK，也不会参与部署阶段 manifest render。

示例：

```yml
# lzc-build.dev.yml
package_override:
  package: cloud.lazycat.app.demo-app.dev
contentdir:
envs:
  - DEV_MODE=1
  - FRONTEND_PORT=3000
```

## 四、manifest build 预处理 {#manifest-build}

`lzc-manifest.yml` 在打包前会先经过一层轻量的 build 预处理。

指令都必须写在 YAML 注释里，格式固定为 `#@build <directive>`。

当前支持五条指令：

1. `#@build if profile=dev`
2. `#@build if env.DEV_MODE=1`
3. `#@build else`
4. `#@build end`
5. `#@build include ./relative/path.yml`

求值规则：

1. `if` 会开启一个条件块，直到遇到同层的 `else` 或 `end`。
2. `else` 是可选分支；未命中 `if` 时才会保留 `else` 后面的文本。
3. `end` 用于结束当前条件块。
4. `profile=dev` / `profile=release` 用于按当前构建配置来源判断。
5. `env.KEY=VALUE` 使用 `build.yml:envs` 里的构建期变量做精确字符串比较；未定义视为不匹配。
6. build 预处理发生在打包前，因此它只决定哪些文本进入最终 `manifest.yml`。
7. 后续部署阶段的 manifest render 仍然只处理 `.U` / `.S`，不会再处理 `#@build`。

约束：

1. `include` 只支持纯文本片段。
2. 被 include 的文件里不允许再出现 `#@build` 指令。
3. `include` 路径相对主 `lzc-manifest.yml` 所在目录解析。
4. build 预处理只决定哪些文本进入最终 `manifest.yml`；部署阶段 render 仍只负责 `.U` / `.S` 取值。
5. `#@build` 适合做开发态/发布态文本裁剪，不适合承载部署期动态逻辑。

示例：

```yml
#@build if env.DEV_MODE=1
application:
  injects:
    - id: frontend-dev-proxy
      on: request
      when:
        - "/*"
      do:
        - src: |
            if (ctx.dev.id) {
              ctx.proxy.to("http://127.0.0.1:3000", {
                via: ctx.net.via.client(ctx.dev.id),
                use_target_host: true,
              });
            }
#@build else
application:
  routes:
    - /=file:///lzcapp/pkg/content/dist
#@build end
```

`include` 示例：

```yml
application:
  routes:
    - /=file:///lzcapp/pkg/content/dist
#@build if profile=dev
#@build include ./manifest.dev.inject.yml
#@build end
```

## 五、镜像构建 `ImageBuildConfig` {#images}

`images` 是上游 LPK v2 的内嵌镜像能力；本仓库禁用该字段，不得用它生成最终 `.lpk`。

需要镜像定制时，改用远程镜像桥接：本地构建并推送公开镜像，执行 `lzc-cli appstore copy-image`，然后将返回的 `registry.lazycat.cloud/...` 地址写入 `lzc-manifest.yml`。

| 字段名 | 类型 | 描述 |
| ---- | ---- | ---- |
| `dockerfile` | `string` | 可选，Dockerfile 文件路径，与 `dockerfile-content` 二选一 |
| `dockerfile-content` | `string` | 可选，Dockerfile 内容，与 `dockerfile` 二选一 |
| `context` | `string` | 可选，构建上下文目录；`dockerfile` 模式默认 `dockerfile` 所在目录，`dockerfile-content` 模式默认项目根目录 |
| `upstream-match` | `string` | 可选，按前缀匹配上游镜像，默认 `registry.lazycat.cloud` |

说明：

1. `dockerfile` 与 `dockerfile-content` 必须二选一，不能同时设置。
2. 构建器会沿父镜像链查找 `upstream-match` 指定前缀的镜像作为上游。
3. 若找到上游，则生成混合分发（部分层走 upstream、部分层内嵌）。
4. 若未找到上游，则该镜像自动转为全量内嵌。

## 六、高级 compose override 配置 `ComposeOverrideConfig` {#compose-override}

1. compose override 是 lzc-cli@1.2.61 及以上版本支持的特性，用于在构建时指定 compose override 的配置。
2. compose override 属于 lzcos v1.3.0+ 后，针对一些 lpk 规范目前无法覆盖到的运行权限需求的配置。

详情见 [compose override](./advanced-compose-override.md)
