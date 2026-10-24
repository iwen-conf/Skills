# Source: https://developer.lazycat.cloud/hello-world.md

# Hello World

下面用最短路径完成第一个可部署应用。

## 1. 创建项目

```bash
lzc-cli project create helloworld
```

在交互提示中：

1. 模板选择 `hello-vue (Vue基础模板)`。
2. 应用 ID 可使用默认值 `helloworld`。

创建完成后，会看到类似提示：

```bash
✨ 初始化项目 helloworld
✨ 初始化懒猫云应用
✨ 懒猫微服应用已创建成功 !
✨ First deploy and open the app once
   cd helloworld
   lzc-cli project deploy
   lzc-cli project info
```

进入项目后，你会看到这几类核心文件：

1. `lzc-manifest.yml`：应用运行结构与路由。
2. `package.yml`：静态包元数据，例如 `package`、`version`、`name`、`description`、`locales`、`author`、`license`。
3. `lzc-build.yml`：默认构建配置，也是 release 配置。
4. `lzc-build.dev.yml`：开发态覆盖配置，默认包含独立的 dev 包名覆盖（例如 `package_override.package: cloud.lazycat.app.helloworld.dev`）、空 `contentdir` 覆盖，与构建期变量 `envs`。

这样日常开发时，`project deploy`、`project info`、`project exec` 等 `project` 命令默认都会优先使用 `lzc-build.dev.yml`，从而落到独立的 dev 包名，不会影响 release 版本。
每个命令都会打印实际使用的 `Build config`；如果你要操作 `lzc-build.yml`，请显式加上 `--release`。

## 2. 先部署，再打开应用

```bash
cd helloworld
lzc-cli project deploy
lzc-cli project info
```

说明：

1. 首次部署若提示授权，按 CLI 提示打开浏览器完成授权即可。
2. `project` 命令默认会优先读取 `lzc-build.dev.yml`；命令输出里会打印实际使用的 `Build config`。
3. `project deploy` 会按 `buildscript` 自动安装依赖并构建前端，不需要额外先执行 `npm install`。
4. `project info` 运行中会输出 `Target URL`。
5. 如果你要查看或操作 release 配置，请显式使用 `--release`。
6. 如果应用尚未运行，可再执行：

```bash
lzc-cli project start
lzc-cli project info
```

然后直接打开应用：

1. 在微服客户端启动器点击应用图标。
2. 或在浏览器访问 `Target URL`。

对于 `hello-vue` 模板，应用页面会先进入 dev 模式入口：

1. 如果本地前端 dev server 还没启动，页面会直接提示下一步操作。
2. 页面会显示 inject 脚本实际等待的本地端口。
3. 你不需要先猜命令或先改 manifest。

## 3. 按页面提示启动前端开发

看到应用页面后，再执行：

```bash
npm run dev
```

然后刷新应用页面。

这样后续改动 `src/App.vue` 等前端文件时，LPK 会继续通过 request inject 把流量转到你本机的 dev server。

排查问题可用：

```bash
lzc-cli project log -f
```

## 4. 产出发布包

```bash
lzc-cli project release -o helloworld.lpk
```

安装发布包：

```bash
lzc-cli lpk install helloworld.lpk
```

`project release` 始终使用 `lzc-build.yml`，不会带上开发态的包名后缀和 dev-only `#@build` 分支。