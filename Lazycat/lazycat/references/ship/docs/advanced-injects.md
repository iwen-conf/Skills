# Source: https://developer.lazycat.cloud/advanced-injects.md

# 脚本注入（injects）

## 概述

`injects` 用于在不修改 OCI image 或应用源码的前提下，按规则注入脚本，覆盖浏览器行为、请求行为和响应行为。
字段定义见 [manifest.md#injects](https://developer.lazycat.cloud/spec/manifest.md#injects)。本文聚焦运行机制、调试方式和实践建议。
如果你要直接落地开发态 request inject 模板，继续看 [inject request 开发态 Cookbook](./advanced-inject-request-dev-cookbook.md)。

## 适用场景

* 密码填充与自动登录：见 [免密登录](./advanced-inject-passwordless-login.md)。
* CORS/CSP 微调：按路径精确增删响应头，适配 iframe、前端调试等场景。
* 替换浏览器文件对话框到懒猫网盘：拦截前端交互并接入自定义文件选择流程。
* 隐藏或修改部分页面元素：在不改上游源码的前提下做 UI 适配和运营开关。
* 高级路由：在 request/response 阶段结合 `ctx.proxy` 做动态反向代理。
* 请求头/响应头兼容修正：例如补充鉴权头、修正 WebSocket 相关头、清理冲突头。
* 按用户持久化行为：通过 `ctx.persist` 保存用户侧配置，跨请求复用。
* 请求级诊断与排障：用 `ctx.dump` 输出关键请求/响应信息，快速定位问题。

## 阶段与执行环境

每个 inject 只属于一个阶段：

* `on=browser`：脚本在应用页面真实浏览器环境执行。
* `on=request`：脚本在 lzcinit 沙盒中执行，时机是请求转发到 upstream 前。
* `on=response`：脚本在 lzcinit 沙盒中执行，时机是收到 upstream 响应后。

执行顺序：

* 先按 `application.injects` 的声明顺序。
* 再按同一 inject 的 `do[]` 声明顺序。
* 命中策略是 `all-match-run`，即同阶段所有命中 inject 都执行。

中断行为：

* `request/response` 阶段里，`ctx.response.send(...)` 或 `ctx.proxy.to(...)` 生效后立即短路，停止当前阶段后续脚本。
* 任一脚本报错，当前阶段立即终止并返回错误。

## 匹配规则

匹配字段：

* `when`：命中条件（OR），任意一条命中即可进入候选。
* `unless`：排除条件（OR），任意一条命中即排除。
* `prefix_domain`：仅匹配 `<prefix>-` 前缀域名请求。
* `auth_required`：默认 `true`。请求没有合法 `SAFE_UID` 时跳过当前 inject。

单条规则格式：

`<path-pattern>[?<query>][#<hash-pattern>]`

规则语义：

* 仅支持后缀 `*` 作为前缀匹配；无 `*` 时为精确匹配。
* `query` token 支持 `key` 或 `key=value`，单条规则内为 AND。
* `#hash` 仅 browser 阶段支持，request/response 阶段不支持 hash 规则。

示例：

* `"/"`：仅匹配根路径。
* `"/*"`：匹配任意路径。
* `"/api/*?v=2"`：匹配 `/api/` 前缀且 query 包含 `v=2`。
* `"/#login"`：匹配 hash 为 `login`（仅 browser）。

## 清单示例

```yml
application:
  injects:
    - id: login-autofill
      when:
        - /#login
        - /#signin
      do:
        - src: builtin://hello
          params:
            message: "hello world"

    - id: inject-basic-auth-header
      auth_required: false
      on: request
      when:
        - /api/*
      do: |
        ctx.headers.set("Authorization", "Basic " + ctx.base64.encode("lazycat:Lazycat@2026"));

    - id: remove-cors
      on: response
      when:
        - /api/*
      unless:
        - /api/admin/*
      do: |
        ctx.headers.del("Access-Control-Allow-Origin");
        ctx.headers.del("Access-Control-Allow-Credentials");
```

## `do` 写法

`do` 支持两种写法：

* short syntax：`do` 直接写脚本字符串（只对应一条脚本）。
* long syntax：`do` 写 `[]InjectScriptConfig`（可配置多条脚本与 `params`）。

## 动态参数 `$persist`

`params` 支持用 `$persist` 动态取值，按当前请求的 `SAFE_UID` 解析：

* `{ $persist: "key" }`
* `{ $persist: "key", default: <any> }`

行为说明：

* 命中持久值时使用持久值。
* 未命中时，若配置 `default` 则返回默认值。
* 未命中且无默认值时返回 `null`。

## `ctx` 能力概览

所有阶段都提供：

* `ctx.id` / `ctx.src` / `ctx.phase` / `ctx.params` / `ctx.safe_uid`
* `ctx.request.host` / `ctx.request.path` / `ctx.request.raw_query`

browser 阶段常用：

* `ctx.request.hash`
* `ctx.runtime.executedBefore`
* `ctx.runtime.executionCount`
* `ctx.runtime.trigger`
* `ctx.persist`（Promise 接口）

request/response 阶段常用：

* `ctx.headers`：读写 HTTP 头
* `ctx.body`：读取与改写 body
* `ctx.flow`：同一请求内 request -> response 临时共享
* `ctx.persist`：跨请求持久化
* `ctx.response`：直接返回响应（短路）
* `ctx.proxy`：动态改写反代目标
* `ctx.base64` / `ctx.fs` / `ctx.dump`

## 内置脚本

内置脚本列表和参数说明见：

* [manifest inject 规范](https://developer.lazycat.cloud/spec/manifest.md#injects)

当前最常用的是：

* `builtin://simple-inject-password`

完整免密登录专题见：

* [免密登录](./advanced-inject-passwordless-login.md)

## 验证与排错

验证建议：

1. 先用 short syntax 写最小脚本（例如 `console.log`）确认匹配生效。
2. 再引入 `ctx.headers` 或 `ctx.body` 做单点改写。
3. 最后再叠加 `ctx.flow` 和 `ctx.persist` 做阶段协作。

常见错误：

* `when` 写了 `#hash`，但 `on=request/response`：该规则不会生效。
* 没有 `SAFE_UID` 且 `auth_required=true`：inject 会被跳过。
* `ctx.body.getJSON()` 直接解析失败：请求体并非合法 JSON，需要先判断或捕获异常。

## 下一步

* 需要做免密登录流程，请继续阅读：[免密登录](./advanced-inject-passwordless-login.md)