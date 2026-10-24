# Source: https://developer.lazycat.cloud/changelog.md

# 系统变更日志

本文汇总当前与开发者相关的系统版本变更，按版本倒序排列。

## v1.5.0（2026-4-7) {#v1-5-0}

### 功能调整

* 新增[脚本注入](./advanced-injects.md)机制，支持在 browser / request / response 三个阶段按规则注入脚本
* 新增[inject request 开发态 Cookbook](./advanced-inject-request-dev-cookbook.md)，方便前端 dev server 转发与后端开发态请求分流
* 新增基于 [lzc-cli project](./lzc-cli.md) 的统一开发工作流，覆盖 `project deploy`、`project info`、`project start`、`project exec`、`project cp`、`project log`、`project sync` 与 `project release`
* 新增[LPK v2](lpk-format.md) 格式说明，统一 tar-based LPK 的文件组织、`images/`、`images.lock` 与 `package.yml`，需配合 `lzc-cli v2.0.0+` (npm install -g @lazycatcloud/lzc-cli@2.0.0-pre.5)
* 新增[package.yml 规范](https://developer.lazycat.cloud/spec/package.md)，将静态包元数据从 `lzc-manifest.yml` 顶层拆分出来单独维护
* 新增[lzc-deploy-params.yml](deploy-params.md) 的随机默认值能力，`default_value` 支持 `$random(len=...)`（需 `lzcos v1.5.0+`）
* 新增[LightOS 场景](./advanced-lightos.md)，需通过应用商店下载对应入口 LPK；LightOS 拥有最高权限，切勿向非信任人员开放

### 兼容性调整

* 新增文稿访问路径 `/lzcapp/documents`
* 旧的用户文稿访问路径已标记为废弃（deprecated），后续仅保持兼容；新代码请统一使用 `/lzcapp/documents`

## v1.4.3（2026-02-04） {#v1-4-3}

### 功能调整

* clientfs支持
* 启动器内应用[多入口支持](./advanced-entries.md)
* custom/show-lzcapp-loading-time
* 新增[hc api\_auth\_token](./advanced-api-auth-token.md)

### 兼容性调整

* lpk容器的hostname从随机值改为基于appid的固定值
* fix timezone binding

## v1.4.2（2025-12-10） {#v1-4-2}

### 功能调整

* pkgm支持返回更详细的安装错误原因信息
* 预装systemd-container组件
* 支持挂载/lzcsys/run/clientfs特性，lpk层面的支持还需等待后续lzcos版本
* 修复多实例下public\_path异常问题

### 兼容性调整

进一步提高manifest.yml/services字段与docker compose spec的兼容性。

主要影响以下现存lpk:

1. `services.xx.user`字段如果是1000这种会报错，需要使用"1000"
2. `services.xxx.evironment`为空时会报错

## v1.4.1（2025-11-19） {#v1-4-1}

### 功能调整

* 新增`services.[].healthcheck`字段，与docker-compose保持100%兼容。
* `application.health_check`增加`timeout`字段
* ssh后 `timedatectl set-timezone xx`支持永久存储

### 兼容性调整

* 废弃`services.[].health_check`字段，新代码请使用`healthcheck`字段。
  若存量lpk里有写`health_check.start_period`(之前文档中只支持这一个时间相关的配置)，需要迁移到`healthcheck`版本
  并添加单位后缀。
  这个字段之前无论设置为多少，最终(不含app)service的health check会被强制设置为`start_period=300s, start_interval=1s` 也就是开发者设置的值被静默的废弃了。因此不做兼容性处理，以便尽快暴露问题。

* lzcos的rootfs视角，/data从真实挂载点调整为指向/lzcsys/data/的软连

## v1.3.9（2025-08-07） {#v1-3-9}

### 功能调整

* `UpstreamConfig`新增`disable_trim_location`选项，以便不要剥离浏览器发送过来的url路径
* 应用启动中界面显示实时启动日志，方便调试lpk的启动过程
* lpk的镜像下载流程从`安装lpk`推迟到`启动lpk`中(安装后会自动进入启动过程)
* 修复启动器中查看应用日志窗口抢占问题

## v1.3.8（2025-07-04） {#v1-3-8}

### 功能调整

* 新增在应用列表中编辑环境变量的支持(管理员在系统设置->应用列表中访问相关UI)
* 新增[应用部署机制](./advanced-manifest-render.md)
* 新增lzcapp运行时文件：/lzcapp/run/manifest.yml  (/lzcapp/pkg/manifest.yml为lpk中的原始内容)
* 新增部署时环境变量`LAZYCAT_DEPLOY_ID`
* 新增`manifest.yml:ext_config.default_prefix_domain`
* 新增tcp-ingress[转发80/443流量](./advanced-l4forward.md)的支持
* 新增`manifest.yml:application.file_handler`的[通配符支持](./advanced-mime.md)
* 新增`/lzcapp/document/`、`/lzcapp/media/`这个两个lzcapp运行时目录(lzcos同样新增了这两个目录以便系统和lzcapp可以使用统一的路径前缀)，对应之前的`/lzcapp/run/mnt/home`和`/lzcapp/run/mnt/media`
* 新增`manifest.yml:services[].mem_limit`
* 新增`manifest.yml:services[].shm_size`
* 新增[manifest.yml:UpstreamConfig](./advanced-route.md#upstreamconfig)以便支持基于域名前缀的分流，以及跳过TLS证书验证等功能
* 新增[sysbox-runc](https://developer.lazycat.cloud/spec/manifest.md#container-config)运行时，以便在不需要而且权限的前提下，运行dockerd,systemd等特权进程。
* 新增启动器中显示应用日志入口(需要安装开发者工具v0.3.0+)
* 修复/dev/shm/的权限为正常的1777
* 修复`setup_scripts`执行时的`HOME`环境变量
* 捕获更多lpk启动过程中出错的信息给前端界面
* `/lzcapp/var`目录权限调整为1777以便减少部分容器的适配工作

### 兼容性调整

* `/data/app/var/$pkg_id/$uid` 目录调整为 `/data/appvar/$deloy_id`
* pkgm/ss: 兼容 QueryApplication.TodoRemoveAppidList
* /data/system/pkgm/cfgs目录彻底废弃，相关信息统一存放到system/pkgm/deploy.{db,var}中
* 移除默认给所有容器添加的`seccomp=unconfined`和`apparmor=unconfined`
* `/data/document/`调整为readonly，避免应用错误在这里创建子目录。(部分应用在新系统上无法安装，已经安装的不受影响)
  已扫描到的相关应用官方正在联系开发者协助进行迁移。
* `application.background`字段调整为建议性作用，默认不开启自启动
* 移除`/data/system/pkgm/apps/$appid$/docker-compose.yml`这个临时文件

### 严重不兼容性说明

系统升级到v1.3.8+后如果执行系统降级操作，则所有应用数据会无法使用。(文稿数据不影响，且应用数据本身还在数据盘中)

原因是为后续更灵活的部署逻辑做支撑，appvar的逻辑从`$pkd_id/$uid`调整为`$deploy_id`了以便去掉uid的概念。
若回滚到低版本系统，则应用内部数据会被重新初始化创建。

## v1.3.7（2025-05-28） {#v1-3-7}

### 功能调整

* 支持systemd --user
* 默认镜像源从中国大陆调整为debian默认CDN. (中国大陆用户可以使用hc ustc临时切换回国内)

## v1.3.6（2025-05-09） {#v1-3-6}

### 功能调整

* 应用多域名机制调整 [附加域名](./advanced-secondary-domains.md)
* ssh中增加风扇控制相关操作`hc fanctl`
* 多实例应用会被分配独立域名 (浏览器等第三方访问时不在需要增加`uid@`这种操作)
* 多实例应用支持TCP/UDP转发
* 真实实现域名冲突解决(不同应用使用相同的`application.subdomain`，系统会自动修改后安装的应用域名)

## v1.3.4（2025-02-24） {#v1-3-4}

### 功能调整

* 支持一个lzcapp配置多个[附加域名](./advanced-secondary-domains.md)
* 支持全局禁用默认挂载用户文稿
* pkgm级别强制检测`min-os-version`
* 不论是否使用备份盘，都默认开启文稿快照(1小时一次，保留最近48小时)
* lpk支持应用使用grpc-web流量(需要开启`ext_config.disable_grpc_web_on_root`)
* ssh后的家目录调整为永久存储
* 修复lpk文件关联

在v1.5.0会默认禁用应用访问用户文稿数据的能力，应用如果确实需要访问用户文稿，
可以提前配置[application.ext\_config.enable\_document\_access](https://developer.lazycat.cloud/spec/manifest.md#ext_config)字段

如果需要测试效果，可以ssh后执行 `touch /lzcsys/var/runtime/config/disable_auto_mount_home` 提前开启此安全配置。

## v1.3.0（2025-02-17） {#v1-3-0}

### 功能调整

* 忽略manifest.yml中`services.depends_on`名称为"app"的条目。
  因为系统会强制等待所有容器就绪，因此不需要填写。反而因为填写后会出现循环等待导致超时。
* manifest.yml中services支持`init`字段，以便少量容器可以开启子进程回收
* lpk支持内嵌[compose.override.yml](./advanced-compose-override.md)文件来支持lpk暂时未覆盖的功能
* 上传日志时会在管理员网盘根目录下存放一份实际上传的日志文件
* lzcos的`vm.max_map_count`调整为1048576
* ssh增加resolvectl方便查询DNS状态
* 修复pg-docker重启后无法自动启动的问题
* 更新docker-ce到27.5.1
* 增加[国际化](https://developer.lazycat.cloud/spec/manifest.md#i18n)/本地化支持
* lzcapp强制开启网络隔离，禁止不同应用之间直接进行网络通讯。即通过`$service.$appid.lzcapp`的形式跨应用通讯。(应用内容器不受此限制)
  * 若之前配置了跨应用访问，需要迁移到端口转发工具上，将目标端口转发到"微服虚拟网卡"上进行间接访问.
  * 请更新lzc-cli到1.2.61+,否则可能因为老版本机制导致功能异常

## v1.2.0（2025-01-06） {#v1-2-0}

### 功能调整

1. 新增[应用使用须知](https://developer.lazycat.cloud/spec/manifest.md#basic-config)功能，方便做一些更强的应用使用提示
2. 新增[setup\_script](./advanced-setupscript.md)功能，方便做一些简单的初始化操作
3. 新增[user](https://developer.lazycat.cloud/spec/manifest.md#container-config)字段，兼容docker-compose语法
4. 新增[TCP/UDP动态端口转发](./advanced-public-api.md#tcp-udp-ingress)
   ::: tip 说明
   之前版本manifest.yml:application.ingress字段仅支持固定端口的转发，
   新版支持指定端口范围的动态转发
   :::
5. 新增[exclude publich\_path](./advanced-public-api.md)语法
6. 移除replace`$$`to`$`的转换操作。
   ::: tip 说明
   之前版本`lzc-manifest.yml`文件中出现`$$`会被强制替换为`$`
   :::
7. 部署时环境变量新增`LAZYCAT_APP_DOMAIN`避免困惑
   ::: tip 说明
   之前版本部署时的环境变量只有`LAZYCAT_APP_ORIGIN`，但运行时自动注入的环境变量又只有`LAZYCAT_APP_DOMAIN`。
   新版本在运行时和部署时均增加了`LAZYCAT_APP_DOMAIN`。
   :::

### bug修复

1. 修复重启后ssh密码丢失的问题
2. 修复系统干扰`http Authorization header`(比如应用使用了basic auth)
   ::: tip 说明
   之前版本的系统会将`http authorization header`解析为微服自身的账号密码导致应用无法使用此字段
   :::
3. 新增ssh后`hc data_convert`命令，支持在线转换raid0/raid1/plain
4. 修复pg-docker断电重启后无法自动拉起的问题

### 暂未实现的功能

1. 支持[动态渲染lzc-manifest.yml](./advanced-manifest-render.md)功能，允许部分参数在安装应用后由用户进行配置。
2. 实验性支持[部署向导](./advanced-user-deploy-guide.md)功能，允许部分参数在安装应用后由用户进行配置。