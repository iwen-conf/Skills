# package.yml Specification Document (LPK v2)

Starting from LPK v2, static metadata for Lazycat applications is unified in `package.yml`. `lzc-manifest.yml` remains focused on the runtime execution structure.

## I. Data Structure

The following fields are mandatory or recommended for inclusion in `package.yml`:

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `package` | `string` | **Mandatory**. Unique application ID. Must be globally unique (e.g., `com.example.myapp`). |
| `version` | `string` | **Mandatory**. Semantic versioning. The format MUST strictly follow `x.x.x` (e.g., `1.0.0`). |
| `name` | `string` | **Mandatory**. Application display name. |
| `description` | `string` | **Mandatory**. Brief application description. |
| `author` | `string` | **Mandatory**. Author or organization name. If `homepage` is a GitHub repository URL in the form `https://github.com/<owner>/<repo>`, this MUST exactly equal the `<owner>` segment character-for-character, including case and symbols. Example: `homepage: https://github.com/Sliverkiss/mimocode2api` requires `author: Sliverkiss`; `sliverkiss`, `SliverKiss`, or any other mismatch will be rejected. For non-GitHub ported apps, this MUST exactly match the original project's author. |
| `license` | `string` | **Mandatory**. Software license (e.g., `MIT`, `GPL-3.0`). |
| `homepage` | `string` | **Optional**. Official website or repository URL. |
| `locales` | `map` | **Optional**. Localization for name, description, and usage. |
| `min_os_version` | `string` | **Optional**. Minimum required Lazycat OS version (e.g., `1.3.0`). |
| `unsupported_platforms`| `[]string`| **Optional but required by this skill for default desktop-only store submissions**. List of unsupported platforms. Unless the user explicitly requests and verifies mobile or TV support, declare `ios`, `tvos`, and `android`. |

## II. Localization (`locales`)

Localization keys follow the [BCP 47 standard](https://en.wikipedia.org/wiki/IETF_language_tag). Prefer concrete tags such as `zh-CN` and `en-US` for store-facing packages.

| Key | Description |
| ---- | ---- |
| `name` | Localized name. |
| `description` | Localized description. |
| `usage` | Localized usage instructions. |

### Example `package.yml`:

```yaml
package: cloud.lazycat.app.demo
version: 1.0.2
name: Demo App
description: A high-performance demo application for Lazycat OS.
author: Lazycat Team
license: MIT
homepage: https://github.com/lazycat-cloud/demo-app
min_os_version: 1.3.5
unsupported_platforms:
  - ios
  - tvos
  - android
locales:
  zh-CN:
    name: "演示应用"
    description: "一个用于 Lazycat OS 的高性能演示程序。"
    usage: "打开应用后会自动使用预置账号登录。默认账号：lazycat@lazycat.com，默认密码：请查看使用说明。"
  en-US:
    name: "Demo App"
    description: "A high-performance demo application for Lazycat OS."
    usage: "Open the app and it signs in automatically with the preset account. Default account: lazycat@lazycat.com. Default password: see usage notes."
```
