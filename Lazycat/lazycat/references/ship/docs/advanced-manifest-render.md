# Source: https://developer.lazycat.cloud/advanced-manifest-render.md

# manifest.yml 渲染

`manifest.yml` 的 render 只负责部署阶段才能确定的值，不再承载 dev/release 结构裁剪。

## 1. 渲染流程

1. 开发者在项目根目录下创建 [lzc-deploy-params.yml](https://developer.lazycat.cloud/spec/deploy-params) 并随 LPK 一起打包。
2. 安装或重新配置实例时，系统收集部署参数。
3. 系统使用 `text/template` 渲染 `/lzcapp/pkg/manifest.yml`。
4. 渲染结果写入 `/lzcapp/run/manifest.yml` 并作为最终运行配置。

## 2. 模板参数

当前只提供两类稳定参数：

1. `.UserParams(.U)`
   * 来源于 `lzc-deploy-params.yml`。
2. `.SysParams(.S)`
   * 系统相关参数，例如：
   * `.BoxName`
   * `.BoxDomain`
   * `.OSVersion`
   * `.AppDomain`
   * `.IsMultiInstance`
   * `.DeployUID`
   * `.DeployID`

不再提供包内 `envs` 注入的 `.E` / `.PkgEnvs`。

如果你要控制 dev/release 结构差异，请使用 `lzc-build.dev.yml` 与 manifest build 预处理，而不是部署阶段 render。

## 3. 内置模板函数

1. [sprig](https://masterminds.github.io/sprig/) 提供的函数（不含 `env` / `expandenv`）。
2. `stable_secret "seed"`
   * 用于生成稳定密码。
   * 同一个 seed 在同一台微服、同一个应用内保持稳定。
   * 不同应用或不同微服的结果不同。

## 4. 调试方式

可以临时把全部 render 上下文打印出来：

```yml
xx-debug: {{ . }}
```

也可以直接查看最终渲染文件：

```yml
application:
  route:
    - /m=file:///lzcapp/run/manifest.yml
```

或在容器内执行：

```bash
cat /lzcapp/run/manifest.yml
```

## 5. 示例

### 5.1 更安全的内部密码

```yml
services:
  mysql:
    environment:
      - MYSQL_ROOT_PASSWORD={{ stable_secret "root_password" }}
      - MYSQL_PASSWORD={{ stable_secret "admin_password" | substr 0 6 }}
  redmine:
    environment:
      - DB_PASSWORD={{ stable_secret "root_password" }}
```

### 5.2 启动参数由用户配置

```yml
# lzc-deploy-params.yml
params:
  - id: target
    type: string
    name: "target"
    description: "the target IP you want forward"

  - id: listen.port
    type: string
    name: "listen port"
    description: "the forwarder listen port, can't be 80, 81"
    default_value: "33"
    optional: true
```

```yml
# lzc-manifest.yml
application:
  subdomain: netmap
  upstreams:
    - location: /
      backend_launch_command: /lzcapp/pkg/content/netmap -target={{ .U.target }} -port={{ index .U "listen.port" }}
```

## 6. build 与 deploy 的边界

推荐按下面的原则理解：

1. build 阶段决定：包里有什么。
2. deploy 阶段决定：这些值在目标微服上取什么值。
3. request inject 阶段决定：当前请求如何路由。

也就是说：

1. dev/release 结构差异：放到 `lzc-build.yml` / `lzc-build.dev.yml` 与 `#@build`。
2. 用户部署输入：放到 `lzc-deploy-params.yml`。
3. 请求级动态行为：放到 inject script。