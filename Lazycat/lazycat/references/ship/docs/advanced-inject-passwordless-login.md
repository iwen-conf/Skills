# Source: https://developer.lazycat.cloud/advanced-inject-passwordless-login.md

# 免密登录

## 目标

本文完成后，你可以实现两类常见场景：

1. 简单场景：使用 `builtin://simple-inject-password` + 部署参数，自动填充半固定账号密码。
2. 高级场景：三阶段联动（request/response/browser），自动记录用户设置的新密码，并在登录页和修改密码页自动填充。

## 常见方式

在微服应用里，常见的“免密/弱感知登录”大致有以下几种方式：

1. 直接基于 ingress 注入的用户身份 header 做用户管理（见 [HTTP Headers](./http-request-headers.md)）。
2. 基于 OIDC 标准登录流打通应用账号体系（见 [对接 OIDC](./advanced-oidc.md)）。
3. 基于部署参数或部署时环境变量注入固定凭据（见 [manifest.yml渲染](./advanced-manifest-render.md) 与 [环境变量](./advanced-envs.md)）。
4. 自动注入 Basic Auth Header（见 [示例一：自动注入 Basic Auth Header](#example-1-basic-auth-header)）。
5. 基于 inject 在不修改上游源码的前提下进行行为改写（本文重点）。

## 前置条件

1. lzcos 版本满足 inject 功能要求。
2. 已阅读 [脚本注入（injects）](./advanced-injects.md) 和 [manifest inject 规范](https://developer.lazycat.cloud/spec/manifest.md#injects)。
3. 已掌握部署参数渲染（见 [manifest.yml渲染](./advanced-manifest-render.md)）。

## 示例一：自动注入 Basic Auth Header

适用场景：

* 上游服务本身使用 Basic Auth。
* 希望在不改上游服务配置的前提下，由网关注入认证头。

示例：

```yml
application:
  injects:
    - id: inject-basic-auth-header
      on: request
      auth_required: false
      when:
        - /api/*
      do: |
        ctx.headers.set("Authorization", "Basic " + ctx.base64.encode("lazycat:Lazycat@2026"));
```

## 示例二：部署参数 + simple-inject-password

适用场景：

* 应用登录账号基本固定，或者账号由部署参数提供。
* 只需要自动填充，不需要自动学习用户后续改密结果。

## 步骤

1. 定义部署参数（`lzc-deploy-params.yml`）：

```yml
params:
  # 固定默认用户名，减少部署后首次登录心智负担
  - id: login_user
    type: string
    name: "Login User"
    description: "Default login username"
    default_value: "lazycat"

  # 密码默认随机生成，避免弱口令
  - id: login_password
    type: secret
    name: "Login Password"
    description: "Default login password"
    default_value: "$random(len=20)"
```

2. 在 `lzc-manifest.yml` 中增加 browser inject：

```yml
application:
  injects:
    # browser 阶段：仅在登录相关 hash 路由执行
    - id: login-autofill
      when:
        - /#login
        - /#signin
      do:
        - src: builtin://simple-inject-password
          params:
            # 从部署参数渲染得到固定用户名
            user: "{{ index .U \"login_user\" }}"
            # 从部署参数渲染得到随机初始密码
            password: "{{ index .U \"login_password\" }}"
```

## 验证

1. 安装应用时填写部署参数（不填写时，用户名使用 `lazycat`，密码使用 `default_value: "$random(...)"` 生成值）。
2. 打开登录页（命中 `when`）。
3. 账号和密码输入框被自动填充。

## 示例三：三阶段联动（Jellyfin）

适用场景：

* 应用首次使用由用户自己创建管理员账号和密码。
* 后续用户可能会在应用内修改密码，希望 inject 自动跟随。

## 核心思路

1. request 阶段：观察创建/登录/改密请求，把候选用户名和密码写入 `ctx.flow`。
2. response 阶段：仅在响应成功时，把 `ctx.flow` 里的值提交到 `ctx.persist`。
3. browser 阶段：登录页从 `ctx.persist` 读取并自动填充；改密页自动填充“当前密码”。

配置示例（Jellyfin 三阶段示例）

## 以下仅展示 `lzc-manifest.yml`；包名、版本等静态信息请写在 `package.yml`。

```yml
application:
  subdomain: jellyfininject
  public_path:
    - /
  routes:
    - /=http://jellyfin:8096
  gpu_accel: true
  injects:
    # request 阶段：抓取首次初始化、登录、改密请求里的候选用户名/密码
    - id: jellyfin-capture-password
      auth_required: false
      on: request
      when:
        - /Startup/User
        - /Users/*
      do: |
        const path = String(ctx.request.path || "");
        const method = String(ctx.request.method || "").toUpperCase();

        const isSetup = path === "/Startup/User" && method === "POST";
        const isUserAuth = /^\/Users\/AuthenticateByName$/i.test(path) && method === "POST";
        const isPasswordUpdate = /^\/Users\/[^/]+\/(Password|EasyPassword)$/i.test(path) && (method === "POST" || method === "PUT");
        if (!isSetup && !isUserAuth && !isPasswordUpdate) return;

        let payload = null;
        try {
          payload = ctx.body.getJSON();
        } catch {
          payload = null;
        }
        if (!payload || typeof payload !== "object") return;

        const pickString = (...values) => values.find((v) => typeof v === "string" && v.length > 0) ?? "";
        const username = pickString(payload.Name, payload.Username, payload.UserName, payload.userName);
        const password = pickString(
          payload.NewPw,
          payload.NewPassword,
          payload.newPw,
          payload.newPassword,
          payload.Password,
          payload.password,
          payload.Pw,
          payload.pw,
        );

        if (username) ctx.flow.set("jf_pending_username", username);
        if (password) ctx.flow.set("jf_pending_password", password);

    # response 阶段：仅在请求成功时提交到持久化存储
    - id: jellyfin-commit-password
      auth_required: false
      on: response
      when:
        - /Startup/User
        - /Users/*
      do: |
        if (ctx.status < 200 || ctx.status >= 300) return;

        const username = ctx.flow.get("jf_pending_username");
        const password = ctx.flow.get("jf_pending_password");
        if (typeof username === "string" && username.length > 0) {
          ctx.persist.set("jellyfin.username", username);
        }
        if (typeof password === "string" && password.length > 0) {
          ctx.persist.set("jellyfin.password", password);
        }

    # browser 阶段：登录页自动填充用户名和密码
    - id: jellyfin-login-autofill
      when:
        - /web/*#/login.html*
        - /web/*#/startup/login*
      do:
        - src: builtin://simple-inject-password
          params:
            user:
              $persist: jellyfin.username
            password:
              $persist: jellyfin.password
            userSelector: "#txtManualName"
            passwordSelector: "#txtManualPassword"

    # browser 阶段：改密页自动填充“当前密码”，但不自动提交
    - id: jellyfin-userprofile-current-password
      when:
        - /web/*#/userprofile.html*
      do:
        - src: builtin://simple-inject-password
          params:
            password:
              $persist: jellyfin.password
            passwordSelector: "#txtCurrentPassword"
            autoSubmit: false

services:
  jellyfin:
    image: registry.lazycat.cloud/nyanmisaka/jellyfin:250503-amd64
    binds:
      - /lzcapp/var/config:/config
      - /lzcapp/var/cache:/cache
      - /lzcapp/run/mnt/media:/media/
```

## 验证

1. 首次初始化管理员账号后，退出并回到登录页，验证自动填充生效。
2. 进入用户资料页修改密码，验证“当前密码”输入框自动填充旧密码。
3. 改密成功后退出登录，验证下次登录页自动填充为新密码。

## 常见错误

1. `on=request/response` 时写了 hash 规则（`#...`），导致规则不生效。
2. request 阶段直接把密码写入 `persist`，未在 response 成功后再提交，导致失败请求污染数据。
3. `simple-inject-password` 未指定选择器，页面字段命名特殊时可能只填充部分输入框。

## 下一步

1. 需要跨页面复用更多状态时，可继续引入 `ctx.persist.list(prefix)` 做批量管理。
2. 需要请求级调试时，可在 request/response 阶段使用 `ctx.dump.request()` 与 `ctx.dump.response()` 输出排障日志。

## 附录：`builtin://simple-inject-password` 参数说明

| 参数 | 类型 | 说明 |
| ---- | ---- | ---- |
| `user` | `string` | 账号值，默认空 |
| `password` | `string` | 密码值，默认空 |
| `requireUser` | `bool` | 是否必须找到账号输入框；默认逻辑：若 `allowPasswordOnly=true` 则为 `false`，否则当 `user` 非空时为 `true` |
| `allowPasswordOnly` | `bool` | 允许仅填充密码，默认 `false` |
| `autoSubmit` | `bool` | 是否自动提交，默认 `true` |
| `submitMode` | `string` | 提交模式：`auto`/`requestSubmit`/`click`/`enter`，默认 `auto` |
| `submitDelayMs` | `int` | 自动提交前延迟（毫秒），默认 `50`，最小 `0` |
| `retryCount` | `int` | 自动提交重试次数，默认 `10` |
| `retryIntervalMs` | `int` | 自动提交重试间隔（毫秒），默认 `300` |
| `observerTimeoutMs` | `int` | DOM/状态观察超时（毫秒），默认 `8000` |
| `debug` | `bool` | 开启调试日志，默认 `false` |
| `userSelector` | `string` | 显式指定账号输入框选择器 |
| `passwordSelector` | `string` | 显式指定密码输入框选择器 |
| `formSelector` | `string` | 限定在指定容器内搜索输入框 |
| `submitSelector` | `string` | 显式指定提交按钮选择器 |
| `allowHidden` | `bool` | 允许填充不可见输入框，默认 `false` |
| `allowReadOnly` | `bool` | 允许填充只读输入框，默认 `false` |
| `onlyFillEmpty` | `bool` | 仅当输入框为空时才填充，默认 `false` |
| `allowNewPassword` | `bool` | 允许填充 `autocomplete=new-password` 的密码框，默认 `false` |
| `includeShadowDom` | `bool` | 是否搜索开放的 Shadow DOM，默认 `false` |
| `shadowDomMaxDepth` | `int` | Shadow DOM 最大递归深度，默认 `2` |
| `preferSameForm` | `bool` | 优先选择与密码框同一表单内的账号框，默认 `true` |
| `eventSequence` | `string` 或 `[]string` | 触发事件序列，默认 `input,change,keydown,keyup,blur` |
| `keyValue` | `string` | 触发键盘事件时的按键值，默认 `a` |
| `userKeywords` | `string` 或 `[]string` | 追加账号字段关键词（逗号分隔或数组） |
| `userExcludeKeywords` | `string` 或 `[]string` | 追加账号字段排除关键词 |
| `passwordKeywords` | `string` 或 `[]string` | 追加密码字段关键词 |
| `passwordExcludeKeywords` | `string` 或 `[]string` | 追加密码字段排除关键词 |
| `submitKeywords` | `string` 或 `[]string` | 追加提交按钮关键词 |