# Source: https://developer.lazycat.cloud/advanced-depends.md

# 服务启动依赖

lzcapp 提供了两个和服务依赖相关的字段，用来处理应用启动时的时序问题

1. 简化版的 `depends_on` 字段
2. 根据 `application.routes` 字段自动注入的健康检测机制

## depends\_on

绝大部分情况, `depends_on` 仅需要在 `services.$service_name` 字段下使用，类型为 `[]string`
其中每一条填写一个 `$service_name`，当前 service 在启动时会等 `depends_on` 中所有的 service 的容器状态为 `healthy`

`service.$service_name.health_check` 字段会影响 service 进入 `healthy` 状态, 此字段与 docker-compose 的 healthCheck 语义一致，
但只支持以下 4 个字段

```go
type HealthCheckConfig struct {
	Test        []string      `yaml:"test"`
	TestUrl     string        `yaml:"test_url"`
	StartPeriod time.Duration `yaml:"start_period"`
	Disable     bool          `yaml:"disable"`
}
```

health\_check 示例:

```yml
services:
  wordpress:
    image: bitnami/wordpress:5.8.2
    health_check:
        test:
            - CMD-SHELL
            - "curl -f http://localhost:80/ || exit 1"
        # test_url: http://localhost:80 # 进行健康检查的 url，如果返回大于 500 则健康检查失败
        start_period: 190s
        disable: false
```

::: warning

> 1. lzcapp 中的对应字段为 `health_check` 而非 `healthCheck`
> 2. 即使 service 中没有填写 `health_check` 也会受 docker image 中对应字段影响
>    :::

## 自动注入的健康检测

lzcapp 主要是通过 `application.routes` 对外提供服务,因此系统会自动根据routes中的上游状态做智能的检测，
因此绝大部分情况下是不需要手动处理依赖关系，具体规则如下:

1. 检测并等待所有 service 对应的容器进入 running 状态
2. 检测并等待 `application.health_check.test_url` 中对应服务返回 200 状态(若有此配置)
3. 检测并等待所有上游就绪后 `application` 这个特殊 service 进入 healthy 状态
   具体方法为，扫描所有 routes 规则，并提取其中的 `http://$hostname:$port` 部分， exec 类型会自动转换为 `http://127.0.0.1:$port`
   使用 TCP dial `$hostname:$port`，如 dial 成功则表明此上游为就绪状态
4. 等待所有其他 service 全部进入 healty 状态, 页面中的"应用启动中"切换为实际服务内容

::: warning

> 1. 如果 `$hostname` 为一个公网 IP/域名 则步骤 3 时会忽略此上游服务，避免微服在没有互联网时无法启动此应用
> 2. 在 dial `$hostname:$port` 时并非使用 http 方式，因为部分上游正常状态时就是404或其他奇怪的状态
>    因此自动注入的检测逻辑仅确保 TCP 层面正常
> 3. 因为自动注入的存在，因此在 `services.$service_name.depends_on` 中千万不要填写 `app` 这个特殊 service\_name
> 4. 若您在开发阶段，遇到依赖相关问题,可以设置 `application.health_check.disable=true` 来禁用自动注入的健康检测,但强烈建议在正式发布应用时开启
>    :::