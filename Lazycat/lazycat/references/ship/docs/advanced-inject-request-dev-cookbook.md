# Source: https://developer.lazycat.cloud/advanced-inject-request-dev-cookbook.md

# inject request 开发态 Cookbook

本文不重复解释 inject 的基础语法，而是直接给出当前 `lzc-cli` 开发流程里最常用的 request inject 写法。

适用目标：

1. 前端开发时，把 LPK 入口代理到开发机的 dev server。
2. 后端开发时，服务未 ready 就返回静态引导页；ready 后再转发到真实后端。
3. 所有这些行为都只在 dev 模式下生效，不污染 release。

相关基础文档：

1. [开发流程总览](https://developer.lazycat.cloud/getting-started/dev-workflow.md)
2. [inject.ctx 规范](https://developer.lazycat.cloud/spec/inject-ctx.md)
3. [脚本注入（injects）](./advanced-injects.md)

## 1. 只在 dev 模式下启用 {#dev-only}

推荐把 dev 专属行为放进 `lzc-manifest.yml` 的 build 预处理块里，而不是在脚本内部自己判断一堆环境变量。

示例：

```yml
application:
  routes:
    - /=file:///lzcapp/pkg/content/dist
#@build if env.DEV_MODE=1
  injects:
    - id: frontend-dev-proxy
      on: request
      auth_required: false
      when:
        - "/*"
      do:
        - src: |
            // dev only request inject here
#@build end
```

这样 release 渲染结果里根本不会带这段 inject。

## 2. 前端开发：代理到开发机 dev server {#frontend-dev}

适用场景：

1. `vite`
2. `webpack dev server`
3. 任何跑在开发机本地端口上的前端服务

说明：`lzc-cli project deploy` 会把当前开发机的 `dev.id` 同步到应用实例；inject 脚本再通过 `ctx.dev.id` 与 `ctx.net.via.client(...)` 把流量转到对应开发机。

示例：

```yml
application:
  routes:
    - /=file:///lzcapp/pkg/content/dist
#@build if env.DEV_MODE=1
  injects:
    - id: frontend-dev-proxy
      on: request
      auth_required: false
      when:
        - "/*"
      do:
        - src: |
            const devPort = 3000;
            const contentType = "text/html; charset=utf-8";

            function renderDevPage(title, subtitle, steps) {
              const items = steps.map(function (step) {
                return "<li>" + step + "</li>";
              }).join("");
              return [
                "<!doctype html>",
                "<html lang=\"en\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>Frontend Dev</title></head>",
                "<body><h1>", title, "</h1><p>", subtitle, "</p><div>Expected local port: ", String(devPort), "</div><ol>", items, "</ol></body></html>",
              ].join("");
            }

            if (!ctx.dev.id) {
              ctx.response.send(200, renderDevPage(
                "Dev machine is not linked yet",
                "This app is waiting for a frontend dev server from the machine that runs lzc-cli project deploy.",
                [
                  "Run <code>lzc-cli project deploy</code> on your dev machine.",
                  "Start your local dev server with <code>npm run dev</code>.",
                  "Refresh this page after port <code>" + String(devPort) + "</code> is ready.",
                ]
              ), { content_type: contentType });
              return;
            }

            const via = ctx.net.via.client(ctx.dev.id);
            if (!ctx.dev.online()) {
              ctx.response.send(200, renderDevPage(
                "Dev machine is offline",
                "The linked dev machine is not reachable right now.",
                [
                  "Bring the dev machine online.",
                  "Start your local dev server with <code>npm run dev</code>.",
                  "Refresh this page after port <code>" + String(devPort) + "</code> is ready.",
                ]
              ), { content_type: contentType });
              return;
            }

            if (!ctx.net.reachable("tcp", "127.0.0.1", devPort, via)) {
              ctx.response.send(200, renderDevPage(
                "Frontend dev server is not ready",
                "This app is running in dev mode and will proxy requests to your linked dev machine.",
                [
                  "Start your local dev server with <code>npm run dev</code>.",
                  "Make sure the dev server listens on port <code>" + String(devPort) + "</code>.",
                  "Refresh this page after the server is ready.",
                ]
              ), { content_type: contentType });
              return;
            }

            ctx.proxy.to("http://127.0.0.1:" + String(devPort), {
              via: via,
              use_target_host: true,
            });
#@build end
```

关键点：

1. 先判断 `ctx.dev.id`，避免实例没有同步到开发机时直接报错。
2. 再看 `ctx.dev.online()`，避免无意义地实时拨测已离线的开发机。
3. 用 `ctx.net.reachable(...)` 做最终端口探测。
4. 用 `ctx.proxy.to(..., { via })` 把流量送到开发机。

## 3. 后端开发：未就绪返回引导页 {#backend-dev}

适用场景：

1. Go / Java 这类更适合手动构建、手动重启的服务
2. 任何必须依赖真实 LPK 容器环境的服务

示例：

```yml
application:
#@build if env.DEV_MODE!=1
  routes:
    - /=exec://3000,/app/run.sh
#@build end
#@build if env.DEV_MODE=1
  injects:
    - id: backend-dev-proxy
      on: request
      auth_required: false
      when:
        - "/*"
      do:
        - src: |
            const backendPort = 3000;
            const backendURL = "http://127.0.0.1:" + String(backendPort);
            const contentType = "text/html; charset=utf-8";

            function renderDevPage(steps) {
              const items = steps.map(function (step) {
                return "<li>" + step + "</li>";
              }).join("");
              return [
                "<!doctype html>",
                "<html lang=\"en\"><head><meta charset=\"UTF-8\"><title>Backend Dev</title></head>",
                "<body><h1>Backend dev service is not ready</h1><ol>", items, "</ol></body></html>",
              ].join("");
            }

            if (!ctx.net.reachable("tcp", "127.0.0.1", backendPort)) {
              ctx.response.send(200, renderDevPage([
                "Sync your latest code into the container with <code>lzc-cli project sync --watch</code> or copy it with <code>lzc-cli project cp</code>.",
                "Open a shell with <code>lzc-cli project exec /bin/sh</code> and start or restart the backend process with <code>/app/run.sh</code>.",
                "Refresh this page after port <code>" + String(backendPort) + "</code> is ready.",
              ]), { content_type: contentType });
              return;
            }

            ctx.proxy.to(backendURL, {
              use_target_host: true,
            });
#@build end
```

关键点：

1. dev 模式下不写 `routes`，避免自动启动把策略绕过去。
2. 服务不在线时直接给出静态引导，不默认 fallback。
3. 服务 ready 后再切到真实后端。

## 4. 访问 lzcos host network 或指定客户端 {#via-network}

`ctx.proxy.to(...)` 和 `ctx.net.reachable(...)` 都支持 `via`。

访问 lzcos host network：

```js
ctx.proxy.to("http://127.0.0.1:8080", {
  via: ctx.net.via.host(),
  use_target_host: true,
});
```

访问指定客户端节点：

```js
if (ctx.dev.id) {
  ctx.proxy.to("http://127.0.0.1:3000", {
    via: ctx.net.via.client(ctx.dev.id),
    use_target_host: true,
  });
}
```

## 5. 什么时候直接返回静态页面 {#when-send-page}

推荐直接 `ctx.response.send(...)` 的场景：

1. 开发机未绑定
2. 开发机离线
3. dev server / backend service 未启动
4. 你希望给出明确的下一步引导，而不是给用户一个模糊的 502

不建议默认 fallback 到 release 路由，因为这会掩盖开发态状态，让开发者误以为当前流量已经进入 dev 逻辑。

## 6. 调试顺序建议 {#debug-order}

建议按这个顺序排查：

1. inject 是否真的在当前路径命中
2. `ctx.dev.id` 是否为空
3. `ctx.dev.online()` 是否为 `true`
4. `ctx.net.reachable(...)` 是否为 `true`
5. `ctx.proxy.to(...)` 的 `via` 是否指向了正确网络

必要时可以临时加 header 辅助确认：

```js
ctx.headers.set("X-Debug-Dev-ID", ctx.dev.id || "");
ctx.headers.set("X-Debug-Dev-Online", String(ctx.dev.online()));
```

## 7. 推荐组合 {#recommended-combos}

推荐把下面这几个能力组合使用：

1. `#@build if env.DEV_MODE=1`
   * 在打包阶段裁剪 dev-only inject
2. `ctx.dev`
   * 判断是否已绑定开发机、开发机是否在线
3. `ctx.net.reachable`
   * 判断目标服务是否真实 ready
4. `ctx.net.via.*`
   * 把流量送到容器外的目标网络
5. `ctx.response.send`
   * 未 ready 时返回明确引导页
6. `ctx.proxy.to`
   * ready 后切到真实目标

## 下一步 {#next}

1. 如果你要看 API 规范：继续阅读 [inject.ctx 规范](https://developer.lazycat.cloud/spec/inject-ctx.md)
2. 如果你要看总体开发流：继续阅读 [开发流程总览](https://developer.lazycat.cloud/getting-started/dev-workflow.md)
3. 如果你要看内置脚本与匹配机制：继续阅读 [脚本注入（injects）](./advanced-injects.md)