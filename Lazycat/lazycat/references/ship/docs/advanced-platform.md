# Source: https://developer.lazycat.cloud/advanced-platform.md

# 平台支持

微服官方应用天然支持 Linux/Windows/macOS/Android/iOS/鸿蒙等 6 个平台， 当您开发的应用并不想支持某些平台时， 可以在 `lzc-manifest.yml` 文件中加入 `unsupported_platforms` 字段即可：

```yml
unsupported_platforms:
  - ios
```

上面的配置的意思是， 您的应用不支持 iOS 平台， 当用户在 iOS 平台下点击应用图标， 微服系统会弹出 `您的应用不支持当前平台` 的提示。

相关可用参数如下:

| 参数    | 平台                                            |
|---------|-------------------------------------------------|
| ios     | 不支持 iOS 和 iPad 移动端                       |
| android | 不支持 Android 移动端                           |
| linux   | 不支持 Linux 桌面端                             |
| windows | 不支持 Windows 桌面端                           |
| macos   | 不支持 Macos 桌面端                             |
| tvos    | 不支持 懒猫智慧屏 平台端 (要求系统 1.0.18 以上) |