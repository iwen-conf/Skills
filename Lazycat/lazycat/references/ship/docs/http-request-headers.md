# Source: https://developer.lazycat.cloud/http-request-headers.md

# http headers

所有从客户端发起的https/http的流量会先进入`lzc-ingress`这个组件进行分流。

lzc-ingress主要处理以下任务

* 对http请求进行鉴权，若未登录则跳转到登录页面
* 根据请求的域名分流到不同的lzcapp后端

在鉴权成功转发给具体的lzcapp前，`lzc-ingress`会设置以下额外的http headers

* `X-HC-User-ID`        登录的UID(用户名)
* `X-HC-Device-ID`      客户端位于本微服内的唯一ID， 应用程序可以使用这个作为设备标识符
* `X-HC-Device-PeerID`  客户端的peerid， 仅内部使用。
* `X-HC-Device-Version` 客户端的内核版本号
* `X-HC-Login-Time` 微服客户端最后一次的登录时间， 格式为unix时间戳(一个int32的整数)
* `X-HC-User-Role`  普通用户为:"NORMAL"， 管理员用户为: "ADMIN"
* `X-HC-SOURCE` 请求来源语义。当前可能取值为 `client`、`app:self`、`app:<pkg_id>`、`system`
* `X-Forwarded-Proto` 固定为"https"，以便少量强制https的应用可以正常工作
* `X-Forwarded-By`  固定为"lzc-ingress"
* `X-HC-User-Ticket`  用户票据。当前版本中，系统可能在已登录请求上下文中默认提供；未来版本将改为只能通过显式权限申请获得。应用不应假设该 header 会永久默认存在。

`lzc-ingress`是通过`HC-Auth-Token`这个cookie来进行鉴权的(客户端内是通过其他内部方式完成鉴权)。

当`lzc-ingress`遇到此cookie值无效或为空时，且目标地址不是`public_path`，则会跳转到登录页面。

当目标地址为`public_path`时， `lzc-ingress`依旧会进行一次鉴权，但不会跳转到登录页面。

* 如果鉴权失败，则会清空上述`X-HC-XX`的header，避免一些安全风险
* 如果鉴权成功，则会带上上述`X-HC-XX`的header。

也就是lzcapp开发者在编写后端代码时，不用考虑是否为`public_path`， 直接信任`X-HC-User-ID`即可。

## `X-HC-User-Ticket`

`X-HC-User-Ticket` 用于表达“应用以某个真实用户身份继续访问自身或其他 lzcapp 服务”的用户票据语义。

约束：

1. 当前能力最低要求 `lzcos v1.5.2`。
2. 当前版本中，`X-HC-User-Ticket` 可能由系统在已登录请求上下文中默认提供。
3. 当前默认提供方式只是临时行为，不做兼容性保障。
4. 预计在 `lzcos v1.7.x`，系统会改为只有用户明确授权后，应用才能获取该票据。
5. 应用不应把“当前版本默认存在”视为长期兼容承诺。
6. 应用间访问场景请同时参考 [应用间访问](./advanced-app-interconnect.md)。
7. 新能力设计应统一以 `X-HC-User-Ticket` 为最终字段名，不再使用 `X-HC-User-Delegation`。

## `X-HC-SOURCE`

`X-HC-SOURCE` 用于表达当前请求的业务来源语义。

当前可能取值：

1. `client`
2. `app:self`
3. `app:<pkg_id>`
4. `system`

说明：

1. `client` 表示真实客户端访问。
2. `app:self` 表示应用代表当前真实用户访问自己。
3. `app:<pkg_id>` 表示其他应用代表当前真实用户访问当前应用。
4. 该 header 由系统生成，应用不应信任客户端自行传入的值。