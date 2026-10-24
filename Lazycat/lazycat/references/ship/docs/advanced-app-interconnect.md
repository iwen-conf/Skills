# Source: https://developer.lazycat.cloud/advanced-app-interconnect.md

# 应用间访问

## 目标

本文说明 lzcapp 如何在懒猫微服内以“代表当前真实用户”的语义访问其他 lzcapp 的 HTTP 服务。

当前能力适用场景：

1. 一个应用的前端访问另一个应用的后端。
2. 一个应用访问其他应用暴露的 MCP HTTP 服务。
3. 应用通过统一入口访问自己的 canonical HTTP 面。

最低版本要求：

1. `lzcos >= v1.5.2`

## 核心概念

当前应用间访问能力只覆盖 HTTP 场景。

访问目标统一使用：

```text
app.<target-pkg-id>.lzcx
```

例如：

```text
http://app.cloud.lazycat.app.todo.lzcx/api/tasks
```

说明：

1. `.lzcx` 是 app 级入口，不提供 service 级选择能力。
2. 系统会复用正常 `heiyu.space` 访问所使用的 ingress 路由语义。
3. 如果目标应用是多实例应用，系统会根据真实用户 `uid` 自动路由到对应实例。
4. 应用访问自身时，也使用同一套入口，例如 `http://app.<self-pkg-id>.lzcx/...`。

## 权限模型

当前版本有两个相关权限：

### `lzcapp.self_delegate`

用于允许系统向当前应用下发用户票据。

它只适用于：

1. 应用访问自己的 `app.<self-pkg-id>.lzcx`

它不适用于：

1. 访问其他应用

### `lzcapp.user_delegate`

用于允许应用代表当前真实用户访问其他应用。

它适用于：

1. 访问其他应用的 `app.<target-pkg-id>.lzcx`
2. 访问 `app.home.system.lzcx`

## 最小配置

如果应用只需要访问自己：

```yml
package: cloud.lazycat.app.demo
version: 0.0.1
name: Demo

permissions:
  required:
    - lzcapp.self_delegate
```

如果应用需要访问其他应用：

```yml
package: cloud.lazycat.app.demo
version: 0.0.1
name: Demo

permissions:
  required:
    - lzcapp.user_delegate
```

## 当前使用方式

当前版本里，系统可能会在真实用户访问应用时，向应用注入 `X-HC-USER-TICKET`。

## 如何获取 `X-HC-USER-TICKET`

当前版本中，应用没有独立的“主动申请票据”接口。

实际获取方式是：

1. 应用先在 `package.yml` 中声明 `lzcapp.self_delegate` 或 `lzcapp.user_delegate`。
2. 真实用户通过正常微服 HTTP 入口访问该应用。
3. 系统在将这次真实用户请求转发给应用时，可能会在请求 header 中注入 `X-HC-USER-TICKET`。
4. 应用从自己的入站 HTTP 请求 header 中读取该值，并按自身需要决定是否保存。

也就是说，当前版本里 `X-HC-USER-TICKET` 的来源不是独立 API，而是“真实用户访问应用时，由 ingress 附带注入到应用收到的请求里”。

一个典型流程是：

1. 用户先打开应用页面。
2. 应用后端从这次请求的 header 里读取 `X-HC-USER-TICKET`。
3. 应用把该值保存到自己的会话、数据库或其他状态里。
4. 后续应用再用这个值访问 `app.<target-pkg-id>.lzcx`。

应用拿到该值后，可以在后续请求中带上它：

```bash
curl \
  -H "X-HC-USER-TICKET: <ticket>" \
  http://app.cloud.lazycat.app.todo.lzcx/api/tasks
```

访问自身时同理：

```bash
curl \
  -H "X-HC-USER-TICKET: <ticket>" \
  http://app.cloud.lazycat.app.demo.lzcx/api/profile
```

## 当前版本的重要限制

`X-HC-USER-TICKET` 的获取方式目前只是临时方案。

请明确遵守以下约束：

1. 当前版本不对“系统会默认下发 `X-HC-USER-TICKET`”做兼容性保障。
2. 应用不能把“首次真实用户请求里一定能拿到该 header”视为长期稳定接口。
3. `lzcapp.self_delegate` 只适用于应用访问自己；需要访问其他应用时，仍应声明 `lzcapp.user_delegate`。
4. 预计在 `lzcos v1.7.x`，系统会改为只有用户明确授权后，应用才能获取可用于应用间访问的票据。
5. 新应用设计时，应预留后续显式授权接入能力，不要把当前临时行为固化为唯一方案。

## 转发后的请求语义

当目标应用收到系统转发的请求时，可以信任以下 header：

1. `X-HC-User-ID`
2. `X-HC-User-Role`
3. `X-HC-SOURCE`
4. `X-HC-ORIGINAL-HOST`（仅在 ingress 为路由改写了目标 host 时出现）

其中：

1. `X-HC-SOURCE=client` 表示请求来自真实客户端访问。
2. `X-HC-SOURCE=app:self` 表示应用在代表当前真实用户访问自己。
3. `X-HC-SOURCE=app:<pkg-id>` 表示请求来自其他应用。

需要注意：

1. 目标应用看不到上游传入的 `X-HC-USER-TICKET`，系统会在转发前消费并移除它。
2. 委托请求下，系统不会向目标应用注入 `X-HC-Device-ID`、`X-HC-Device-PeerID`、`X-HC-Device-Version`、`X-HC-Login-Time` 这类客户端设备上下文。

## 开发建议

1. 业务鉴权应继续基于 `X-HC-User-ID` 判断真实用户，不要依赖目标应用自己解析 `X-HC-USER-TICKET`。
2. 如果应用需要区分“真实客户端访问”与“其他应用代用户访问”，请基于 `X-HC-SOURCE` 判断。
3. 如果当前只是后端通过统一入口回访自己，请优先使用 `lzcapp.self_delegate`，不要直接申请 `lzcapp.user_delegate`。

## 相关文档

1. [HTTP Headers](./http-request-headers.md)
2. [package.yml 规范](https://developer.lazycat.cloud/spec/package.md)