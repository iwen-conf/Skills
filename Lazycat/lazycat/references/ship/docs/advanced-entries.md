# Source: https://developer.lazycat.cloud/advanced-entries.md

# 多入口支持

当一个应用需要提供多个入口（例如主界面与管理界面）时，可以在 `lzc-manifest.yml` 中使用 `application.entries` 声明多个入口。启动器会为同一应用(右键菜单)展示多个入口按钮，方便用户直达不同页面。

需要 lzcos v1.4.3+。

```yml
# lzc-manifest.yml
application:
  subdomain: demoapp
  entries:
    - id: main
      title: "Main"
      path: /
    - id: admin
      title: "Admin"
      path: /admin
      prefix_domain: admin
```

```yml
# package.yml
locales:
  en:
    entries.main.title: "Main"
    entries.admin.title: "Admin Console"
```

* `id`：入口唯一标识，必须在应用内保持唯一
* `title`：入口显示名称，可通过 `locales` 的 `entries.<entry_id>.title` 做多语言本地化
* `path`：入口路径，通常以 `/` 开头
* `prefix_domain`：可选，入口域名前缀，最终域名表现为 `<prefix>-<subdomain>.<rootdomain>`

建议只为真正需要被用户直接访问的页面创建入口，避免入口过多造成使用混淆。