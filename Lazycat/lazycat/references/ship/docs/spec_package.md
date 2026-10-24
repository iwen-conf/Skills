# Source: https://developer.lazycat.cloud/spec/package.md

# package.yml 规范文档

## 一、概述

`package.yml` 用于定义 LPK 的静态包元数据，以及开发者声明的权限需求范围。

说明：

1. 本文描述的 `package.yml` 新流程以 `lzcos v1.5.0+` 为前提。
2. 如需实际构建对应的 `LPK v2` 包，请配合 `lzc-cli v2.0.0+`。
3. 当前 `permissions` 只定义微服侧权限，不包含客户端权限，也不包含资源配额。

自 LPK v2 起，以下静态字段应统一放在 `package.yml`，不再写到 `lzc-manifest.yml` 顶层：

1. `package`
2. `version`
3. `name`
4. `description`
5. `author`
6. `license`
7. `homepage`
8. `admin_only`
9. `min_os_version`
10. `unsupported_platforms`
11. `locales`

## 二、顶层数据结构 `PackageConfig`

| 字段名 | 类型 | 描述 |
| ---- | ---- | ---- |
| `package` | `string` | 必填；应用唯一包 ID，需满足合法 package name 规则 |
| `version` | `string` | 必填；应用版本 |
| `name` | `string` | 可选；应用名称 |
| `description` | `string` | 可选；应用描述 |
| `author` | `string` | 可选；作者或维护者 |
| `license` | `string` | 可选；许可证标识或链接 |
| `homepage` | `string` | 可选；主页或反馈地址 |
| `admin_only` | `bool` | 可选；是否仅管理员可见 |
| `min_os_version` | `string` | 可选；要求的最低系统版本 |
| `unsupported_platforms` | `[]string` | 可选；不支持的平台列表 |
| `locales` | `map[string]PackageLocaleConfig` | 可选；多语言元数据 |
| `permissions` | `PermissionsConfig` | 可选；开发者声明的权限需求范围 |

说明：

1. `package` 与 `version` 是最小必填字段。
2. `package` 是唯一包 ID，`application.subdomain` 是默认访问域名，两者语义不同。
3. 构建阶段可在 `lzc-build.yml` 里使用 `package_override` 覆盖最终写入 `package.yml` 的顶层字段；同名字段按顶层整体覆盖，不做递归 merge；顶层写空值表示清空对应字段。

## 三、`locales`

`locales` 用于声明多语言元数据。

| 字段名 | 类型 | 描述 |
| ---- | ---- | ---- |
| `name` | `string` | 当前语言下的应用名称 |
| `description` | `string` | 当前语言下的应用描述 |

约束：

1. `locales` 的 key 建议使用 BCP 47 语言标签，例如 `zh-CN`、`en-US`。
2. `locales.<lang>.name` 与 `locales.<lang>.description` 都是可选字段。
3. 未命中某个语言时，系统回退到顶层 `name` / `description`。

## 四、`PermissionsConfig`

`permissions` 表达的是开发者声明，不是最终授权结果。

| 字段名 | 类型 | 描述 |
| ---- | ---- | ---- |
| `required` | `[]string` | 应用正常运行必需的权限 id 列表 |
| `optional` | `[]string` | 应用可选使用的权限 id 列表 |

约束：

1. `required` 和 `optional` 中的元素必须是合法权限 id。
2. 同一个权限 id 不应同时出现在 `required` 和 `optional` 中。
3. 未知权限 id 视为非法配置。
4. 未声明的权限不应被应用使用。

## 五、`permissions` 权限 id

### 5.1 网络

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `net.internet` | 访问互联网 | 允许应用访问公网网络资源 |
| `net.lan` | 访问局域网 | 允许应用访问当前局域网内的设备与服务 |
| `net.host` | 使用宿主网络 | 允许应用使用宿主网络 |
| `net.admin` | 网络管理 | 允许应用执行高危网络管理操作 |

### 5.2 文稿与媒体

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `document.private` | 私有文稿 | 允许应用使用 `/lzcapp/documents/$uid` 私有文稿目录。实际文稿数据按用户隔离存放，应用卸载后默认不会删除 |
| `document.read` | 读取文稿 | 允许应用读取用户文稿目录中的内容 |
| `document.write` | 写入文稿 | 允许应用修改或写入用户文稿目录中的内容 |
| `media.read` | 读取媒体 | 允许应用读取系统媒体目录中的内容 |
| `media.write` | 写入媒体 | 允许应用修改或写入系统媒体目录中的内容 |

### 5.3 设备与挂载

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `device.dri.render` | 访问 DRI Render | 允许应用访问 DRI render 设备 |
| `device.dri.master` | 访问 DRI Master | 允许应用访问 DRI master 设备 |
| `device.usb` | 访问 USB 设备 | 允许应用访问连接到微服的 USB 设备 |
| `device.kvm` | 访问 KVM 设备 | 允许应用访问 KVM 相关设备 |
| `device.block` | 访问块设备 | 允许应用访问块设备相关能力 |
| `fuse.mount` | 挂载 FUSE 文件系统 | 允许应用自行挂载 FUSE 文件系统 |

### 5.4 跨应用数据

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `appvar.other.read` | 读取其他应用数据 | 允许应用读取其他应用实例的 `appvar` 数据 |
| `appvar.other.write` | 写入其他应用数据 | 允许应用修改其他应用实例的 `appvar` 数据 |

### 5.5 高危兼容能力

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `compose.override` | 高危运行时覆盖 | 允许应用通过 Compose Override 覆盖最终运行时配置，可能绕过常规权限边界，风险接近最高权限 |

### 5.6 电源控制

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `power.shutdown.inhibit` | 阻止关机 | 允许应用在关键运行期间申请阻止系统正常关机 |

### 5.7 LightOS

| 权限 id | 名称 | 描述 |
| ---- | ---- | ---- |
| `lightos.use` | 使用 LightOS | 允许应用创建、删除和修改当前 LPK 独享的 LightOS 实例 |
| `lightos.manage` | 管理 LightOS | 允许应用管理全局 LightOS 实例，且可以创建超级权限的实例 |

## 六、最小示例

```yml
package: cloud.lazycat.app.demo-app
version: 0.0.1
name: Demo App
description: Demo application

permissions:
  required:
    - net.internet
    - document.read
  optional:
    - document.write
    - device.dri.render
    - power.shutdown.inhibit
```

## 七、完整示例

```yml
package: cloud.lazycat.app.demo-app
version: 0.0.1
name: Demo App
description: Demo application
author: Demo Team
license: MIT
homepage: https://example.com
min_os_version: 1.2.3
unsupported_platforms:
  - linux/386
locales:
  zh-CN:
    name: 示例应用
    description: 示例应用描述
  en-US:
    name: Demo App
    description: Demo application
permissions:
  required:
    - net.internet
    - document.read
  optional:
    - document.write
    - media.read
    - device.dri.render
    - power.shutdown.inhibit
```