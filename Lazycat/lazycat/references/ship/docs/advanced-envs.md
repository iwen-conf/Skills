# Source: https://developer.lazycat.cloud/advanced-envs.md

# 开发者环境变量

在应用配置 `lzc-manifest.yml` 的 `services` 中可以配置每个服务的 `environment`，如下配置 `bitnami/wordpress:5.8.2` 环境变量示例。

```
services:
  wordpress:
    image: bitnami/wordpress:5.8.2
    environment:
      - ALLOW_EMPTY_PASSWORD=yes
      - DISABLE_FEATURE_ABC=yes
```

## 运行时环境变量列表 {#runtime\_envs}

每个container在运行环境会自动注入以下环境变量, 其他变量需要开发者手动注入进去

| 变量名 | 示例值 | 描述 |
| -- | -- | -- |
|LAZYCAT\_APP\_DEPLOY\_UID| admin | 多实例应用下容器所属用户,若为空说明是单实例部署 (lzcos-v1.2引入)|
|LAZYCAT\_APP\_DOMAIN|l4test.snyht3.heiyu.space|应用分配到的域名，不要永久存储此值，后续版本重启后可能会变动|
|LAZYCAT\_APP\_ID|test.lzcos.l4ingress|应用的appid,等同/lzcapp/pkg/manifest.yml:Package字段|
|LAZYCAT\_APP\_SERVICE\_NAME|app|当前容器所属的service名称|
|LAZYCAT\_BOX\_DOMAIN|snyht3.heiyu.space|微服本身的主域名，不要永久存储此值，后续版本重启后可能会变动|
|LAZYCAT\_BOX\_NAME|snyht3|微服名称|

## 部署时环境变量列表  {#deploy\_envs}

在部署配置阶段(系统解析lzc-manifest.yml时)可以使用以下环境变量值, 在lzc-manifest.yml中使用`${ENV_NAME}`即可。

例如

```
services:
  iperf:
    image: registry.lazycat.cloud/snyh1010/some_nginx:2694e91b783def0b
    command: -my-domain ${LAZYCAT_APP_DOMAIN}
```

| 变量名 | 示例值 | 描述 |
| -- | -- | -- |
|LAZYCAT\_APP\_DEPLOY\_UID| admin | 多实例应用下容器所属用户,若为空说明是单实例部署 (lzcos-v1.2引入)|
|LAZYCAT\_APP\_DEPLOY\_ID| xx.yy.zz | 实例自身的ID (lzcos-v1.3.8引入)|
|LAZYCAT\_APP\_DOMAIN|l4test.snyht3.heiyu.space|应用分配到的域名，不要永久存储此值，后续版本重启后可能会变动.(lzc-os-v1.2引入)|
|LAZYCAT\_APP\_ID|test.lzcos.l4ingress|应用的appid,等同/lzcapp/pkg/manifest.yml:Package字段|
|LAZYCAT\_BOX\_DOMAIN|snyht3.heiyu.space|微服本身的主域名，不要永久存储此值，后续版本重启后可能会变动|
|LAZYCAT\_BOX\_NAME|snyht3|微服名称|
|LAZYCAT\_AUTH\_OIDC\_CLIENT\_ID|test.lzcos.l4ingress|oauth的client id|
|LAZYCAT\_AUTH\_OIDC\_CLIENT\_SECRET|a3deb9086885cbbc7|在安装阶段随机生成的oauth密钥，每次容器重启都会变动，因此不要保存在数据库中|
|LAZYCAT\_AUTH\_OIDC\_ISSUER\_URI|xxx|oauth的issuer地址|
|LAZYCAT\_AUTH\_OIDC\_AUTH\_URI|xxx|AUTH endpoint地址|
|LAZYCAT\_AUTH\_OIDC\_TOKEN\_URI|xxx|Token endpoint地址|
|LAZYCAT\_AUTH\_OIDC\_USERINFO\_URI|xxx|userinfo endpoint地址|

ps: oidc相关环境变量仅在`application.oidc_redirect_path`存在时才会注入