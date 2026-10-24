# Source: https://developer.lazycat.cloud/advanced-file.md

# 文件访问

上一章我们讲了， 在应用内可以通过保存文件到 `/lzcapp/var` 目录下实现应用数据的持久化。

如果应用需要保存“应用文稿”类型数据，并且在 `package.yml.permissions` 中声明了 `document.private` 权限 id，则在代码中使用稳定路径 `/lzcapp/documents/${uid}`。这个权限只决定系统是否向当前应用提供它自己的应用文稿目录，不会让应用获得其他应用的应用文稿目录。

如果应用需要继续访问现有的用户文稿目录，则使用被废弃的兼容路径 `/lzcapp/run/mnt/home`。从 `v1.7.0` 开始，只有在管理员明确授权后，应用才能访问这个路径。

微服系统会根据兼容开关和权限声明，将相关目录挂载到应用容器，路径为：

* `/lzcapp/run/mnt/home`
* `/lzcapp/documents`

## 挂载点列表

| 挂载目录 | 描述 |
| -- | -- |
| /lzcapp/var | 应用存储目录，应用卸载后， 在勾选了 "并清理数据" 才会删除。|
| /lzcapp/cache | 应用缓存目录，该目录用于存放不重要的数据或文件，微服用户可以在应用管理里手动进行清理，比较适合存放一些日志文件或不必要持久化的临时文件|
| /lzcapp/run/mnt/home | 被废弃的兼容路径，当前保持原有用户文稿语义。单实例下对应全局用户文稿根目录；多实例下对应当前 owner 的用户文稿目录。从 `v1.7.0` 开始，只有在管理员明确授权后，应用才能访问这个路径。|
| /lzcapp/documents | 应用文稿根目录。实际文稿数据按用户隔离存放在 `/lzcapp/documents/<uid>`。不同应用的应用文稿目录始终彼此隔离；`document.private` 只用于声明当前应用是否需要自己的应用文稿目录。|
| /lzcapp/pkg | 应用静态资源目录，包含配置文件及应用打包时携带的资源。该目录为只读。|
| /lzcapp/pkg/content | `lzc-build.yml` 参数 `contentdir` 指定的 lpk 包内容 ，构建打包在 lpk 应用中的内容。该目录为只读。 |

## 使用约束

1. `/lzcapp/var`
   * 用于保存应用自己的持久化数据。

2. `/lzcapp/run/mnt/home`
   * 仅用于兼容旧的用户文稿语义。
   * 从 `v1.7.0` 开始，只有在管理员明确授权后，应用才能访问。

3. `/lzcapp/documents`
   * 应用文稿实际数据路径始终是 `/lzcapp/documents/<uid>`。
   * 不同应用的应用文稿目录始终彼此隔离。
   * 只有在 `package.yml.permissions` 中声明了 `document.private` 权限 id 时，系统才会向当前应用提供这个目录。

## 版本与使用约束

1. `/lzcapp/documents` 在 `v1.5.0` 引入。
2. 开发者不应该使用 `services.<name>.user` 字段处理目录权限问题，也不要使用 `setup_scripts` 执行 `chown` 一类操作。

::: warning

1. 目前 lzcapp 的 `lzc-manifest.yml` 配置文件中仅支持挂载上列挂载点列表中的文件夹。
2. 应用文稿实际数据路径始终是 `/lzcapp/documents/<uid>`。
3. 不同应用的应用文稿目录始终彼此隔离。
4. `/lzcapp/documents` 只有在 `package.yml.permissions` 中声明了 `document.private` 权限 id 时，系统才会向当前应用提供。
   :::

## 常见问题？

Q: 目录 `/lzcapp/documents` 在不同家庭成员使用同一个应用时会看到哪个用户的应用文稿目录？

A: 当应用声明了 `document.private` 权限 id 后，这里会有一个应用 **单实例** 和 **多实例** 的概念 (参考 [多实例](https://developer.lazycat.cloud/advanced-multi-instance) 相关文档)，两者的实际数据路径都统一为 `/lzcapp/documents/<uid>`。这只影响当前应用能看到哪些 UID 子目录，不会让它访问其他应用的应用文稿目录。

1. 在应用声明为 **多实例** 的情况下，应用只会看到当前实例所属用户的 UID 子目录。
2. 在应用声明为 **单实例** 的情况下，应用可能会看到多个 UID 子目录，此时应用需要自行决定将文件写入哪个 UID 目录。

Q: 如何在应用容器内访问懒猫网盘挂载的资源？

A: 在 `lzc-manifest.yml` 文件中配置 [enable\_media\_access: true](https://developer.lazycat.cloud/spec/manifest.html#ext_config) 即可在容器的 `/lzcapp/media/RemoteFS` 目录下找到你挂载的资源。