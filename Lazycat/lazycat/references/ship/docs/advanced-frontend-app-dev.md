# Source: https://developer.lazycat.cloud/advanced-frontend-app-dev.md

# 客户端能力前端接入文档

## 1. 接入前准备

### 1.1 推荐的环境判断方式

推荐先判断当前页面是否运行在客户端 WebShell 中，再决定是否调用宿主能力。

```js
import base from "@lazycatcloud/sdk/dist/extentions/base"

export const isIosWebShell = () => base.isIosWebShell()
export const isAndroidWebShell = () => base.isAndroidWebShell()
export const isClientWebShell = () => isIosWebShell() || isAndroidWebShell()
```

### 1.2 推荐的调用原则

* 有 SDK 封装的能力，优先用 SDK
* 没有 SDK 封装的能力，再直接调用宿主注入对象
* 所有宿主调用都要先做能力存在性判断

```js
export function safeCall(fn, fallback) {
  try {
    return fn ? fn() : fallback
  } catch (error) {
    console.warn("[client-api] call failed", error)
    return fallback
  }
}
```

## 2. 能力矩阵

| 功能 | 支持平台 | 推荐入口 |
| --- | --- | --- |
| 打开轻应用 | iOS / Android | `AppCommon.LaunchApp` |
| 页面禁用暗黑模式 | iOS / Android | `meta[name="lzcapp-disable-dark"]` |
| 轻应用全屏控制 | iOS / Android | `AppCommon.SetFullScreen` / `CancelFullScreen` / `GetFullScreenStatus` |
| 文件分享 | iOS / Android | `AppCommon.ShareWithFiles` |
| 媒体分享 | iOS / Android | `AppCommon.ShareMedia` |
| 文件打开方式 | iOS / Android | `AppCommon.OpenWith` |
| 轻应用临时窗口打开 | iOS | `AppCommon.OpenAppTemporaryWindow` |
| 轻应用关闭按钮显示控制 | iOS | `window.webkit.messageHandlers.SetCloseBtnShowStatus` |
| 轻应用关闭按钮布局 CSS 变量 | iOS | 宿主注入的 CSS 变量 |
| 轻应用导航栏 / 状态栏 meta | iOS | `lzcapp-navigation-bar-scheme` 等 meta |
| 打开客户端主题模式页 | iOS | `client_OPENThemeMode` |
| 通用系统分享 | iOS | `SystemShare` |
| 音量 / 亮度控制 | iOS | `AppCommon.GetDeviceVolume` 等 |
| 锁屏音乐控制 | Android | `MediaSession.*` |
| 图片原生分享 | Android | `AppCommon.ShareImage` |
| 状态栏颜色 | Android | `lzc_window.SetStatusBarColor` |
| 控制栏显隐 | Android | `lzc_tab.SetControlViewVisibility` |
| 获取控制栏状态 | Android | `lzc_tab.GetControlViewVisibility` |
| WebView 跟随键盘 resize | Android | `lzc_window.EnableWebviewResize` |
| 客户端主题模式 | Android | `lzc_theme.getThemeMode` / `setThemeMode` |

## 3. iOS / Android 都支持

### 3.1 打开轻应用

推荐入口：

* `AppCommon.LaunchApp(url, appid, options?)`

推荐场景：

* 从一个页面跳转到另一个轻应用
* 打开文件处理应用
* 打开应用商店中的应用页面

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

export async function openPhotoApp(boxDomain: string) {
  const url = `https://photo.${boxDomain}`
  await AppCommon.LaunchApp(url, "cloud.lazycat.app.photo", {
    forcedRefresh: true,
  })
}
```

说明：

* `forcedRefresh: true` 适合你希望宿主强制刷新目标应用页面的场景
* 如果当前不在客户端环境，SDK 会按浏览器逻辑兜底

### 3.2 页面禁用暗黑模式

支持平台：

* iOS
* Android

接入方式：

在页面 `<head>` 中声明：

```html
<meta name="lzcapp-disable-dark" content="true">
```

推荐真值：

* `true`
* `1`
* `yes`
* `on`

完整示例：

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="lzcapp-disable-dark" content="true" />
    <title>My App</title>
  </head>
  <body>
    <div id="app"></div>
  </body>
</html>
```

说明：

* 该 meta 适合在你的页面本身已经有完整亮色设计时使用
* 推荐在首屏 HTML 里直接声明
* iOS 当前代码支持在页面运行时更新该 meta 后自动生效；Android 侧文档按已支持记录

### 3.3 轻应用全屏控制

推荐入口：

* `AppCommon.SetFullScreen()`
* `AppCommon.CancelFullScreen()`
* `AppCommon.GetFullScreenStatus()`

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

export async function enterFullScreen() {
  await AppCommon.SetFullScreen()
}

export async function exitFullScreen() {
  await AppCommon.CancelFullScreen()
}

export async function toggleFullScreen() {
  const isFull = await AppCommon.GetFullScreenStatus()
  if (isFull) {
    await AppCommon.CancelFullScreen()
  } else {
    await AppCommon.SetFullScreen()
  }
}
```

浏览器原生事件示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

let isFull = false

document.getElementById("fullscreen-btn").addEventListener("click", async function () {
  if (isFull) {
    await AppCommon.CancelFullScreen()
    isFull = false
    this.textContent = "进入全屏"
  } else {
    await AppCommon.SetFullScreen()
    isFull = true
    this.textContent = "退出全屏"
  }
})

AppCommon.GetFullScreenStatus().then(function (value) {
  isFull = !!value
  document.getElementById("fullscreen-btn").textContent = isFull ? "退出全屏" : "进入全屏"
})
```

说明：

* 这是宿主容器级全屏，不是 DOM `requestFullscreen()`
* 更适合轻应用页面整体进入全屏展示

### 3.4 文件分享

推荐入口：

* `AppCommon.ShareWithFiles(path?, paths?, mediaIds?)`

支持平台：

* iOS
* Android

适用场景：

* 分享单个离线文件
* 分享多个离线文件
* 分享媒体资源
* 文件和媒体资源混合分享

单文件示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.ShareWithFiles("/private/path/to/report.pdf")
```

多文件示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.ShareWithFiles(
  undefined,
  [
    "/private/path/to/a.pdf",
    "/private/path/to/b.docx",
  ]
)
```

混合分享示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.ShareWithFiles(
  undefined,
  ["/private/path/to/a.pdf"],
  ["media-id-1", "media-id-2"]
)
```

真实项目参考：

* `lzc-files/ui/src/mobile/store/sharewith.ts`

说明：

* 三个参数 `path`、`paths`、`mediaIds` 不能同时为空
* 移动端优先用这个接口，不建议再接旧的 `ShareWith`

### 3.5 媒体分享

推荐入口：

* `AppCommon.ShareMedia({ actionName?, ids?, id? })`

支持平台：

* iOS
* Android

平台差异：

* iOS
  * 只需要传 `id` 或 `ids`
* Android
  * 需要传 `actionName`
  * `actionName` 由包名和类名组成

iOS 示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.ShareMedia({
  ids: ["media-id-1", "media-id-2"],
})
```

Android 示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.ShareMedia({
  actionName: "com.example.app:com.example.app.ShareActivity",
  ids: ["media-id-1"],
})
```

### 3.6 文件打开方式

推荐入口：

* `AppCommon.OpenWith(boxName, path, appid)`

支持平台：

* iOS
* Android

说明：

* 这是“用其他应用打开文件”能力
* 经常和分享一起使用，但语义上更接近系统“打开方式”

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.OpenWith("", "/private/path/to/demo.pdf", "")
```

## 4. 仅 iOS 支持

### 4.1 轻应用临时窗口打开

推荐入口：

* `AppCommon.OpenAppTemporaryWindow(url)`

适用场景：

* 打开应用商店详情页
* 打开升级页、说明页、外部跳转页
* 希望以“临时窗口”方式打开，而不是常规轻应用栈

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"
import base from "@lazycatcloud/sdk/dist/extentions/base"

export async function openAppStoreDetail(pkgId) {
  const domain = window.location.host.split(".").slice(1).join(".")
  const targetUrl = `https://appstore.${domain}/?#/shop/detail/${pkgId}`

  if (base.isIosWebShell()) {
    await AppCommon.OpenAppTemporaryWindow(targetUrl)
  } else {
    window.location.href = targetUrl
  }
}
```

### 4.2 轻应用关闭按钮显示控制

当前没有统一 SDK 包装，直接通过 iOS JSBridge 调用。

接入方式：

```js
function setIosCloseButtonVisible(visible) {
  const bridge = window?.webkit?.messageHandlers?.["SetCloseBtnShowStatus"]
  if (!bridge?.postMessage) return

  bridge.postMessage({
    params: [visible],
  })
}
```

页面事件示例：

```js
function setIosCloseButtonVisible(visible) {
  const bridge = window?.webkit?.messageHandlers?.["SetCloseBtnShowStatus"]
  if (!bridge?.postMessage) return

  bridge.postMessage({
    params: [visible],
  })
}

document.getElementById("open-preview").addEventListener("click", function () {
  setIosCloseButtonVisible(false)
})

document.getElementById("close-preview").addEventListener("click", function () {
  setIosCloseButtonVisible(true)
})
```

说明：

* 这个能力影响的是宿主层关闭按钮
* 如果你的页面本身需要给关闭按钮留出额外空间，建议结合下一节的 CSS 变量一起用

### 4.3 轻应用关闭按钮布局 CSS 变量

宿主会注入以下 CSS 变量：

* `--lzc-client-safearea-view-width`
* `--lzc-client-safearea-view-height`
* `--lzc-client-safearea-view-top`
* `--lzc-client-safearea-view-right`

推荐用途：

* 给页面右上角标题 / 操作区让位
* 给吸顶 header 增加安全区 padding

示例：

```css
.page-header {
  padding-top: calc(var(--lzc-client-safearea-view-top, 0px) + 8px);
  padding-right: calc(var(--lzc-client-safearea-view-right, 0px) + var(--lzc-client-safearea-view-width, 0px) + 12px);
}

.floating-toolbar {
  top: calc(var(--lzc-client-safearea-view-top, 0px));
  right: calc(var(--lzc-client-safearea-view-right, 0px));
}
```

### 4.4 轻应用导航栏 / 状态栏 meta

#### 4.4.1 导航栏样式

支持的 meta：

```html
<meta name="lzcapp-navigation-bar-scheme" content="sink" />
```

当前可用值：

* `default`
* `sink`
* `hidden`
* `statusBarOnly`

建议理解：

* `default`
  * 显示顶部导航区域
  * 显示关闭按钮
* `sink`
  * 隐藏顶部导航区域
  * 显示关闭按钮
* `hidden`
  * 隐藏顶部导航区域
  * 隐藏关闭按钮
* `statusBarOnly`
  * 隐藏顶部导航区域
  * 显示关闭按钮
  * 仅保留状态栏区域

示例：

```html
<meta name="lzcapp-navigation-bar-scheme" content="statusBarOnly" />
```

#### 4.4.2 状态栏样式

可配置 meta：

```html
<meta name="lzcapp-state-bar-scheme" content="hidden" />
<meta name="lzcapp-state-bar-color" content="#F7F8FA" />
<meta name="lzcapp-state-bar-color-dark" content="#111111" />
```

示例 1：沉浸式页面

```html
<meta name="lzcapp-navigation-bar-scheme" content="sink" />
<meta name="lzcapp-state-bar-scheme" content="sink" />
```

示例 2：保留状态栏颜色

```html
<meta name="lzcapp-navigation-bar-scheme" content="statusBarOnly" />
<meta name="lzcapp-state-bar-color" content="#FFFFFF" />
<meta name="lzcapp-state-bar-color-dark" content="#111111" />
```

说明：

* 推荐把这些 meta 放在页面初始 HTML 中
* 如果页面会运行时动态改 meta，iOS 当前实现支持宿主监听刷新

### 4.5 通用系统分享

推荐入口：

* `SystemShare(contentOrPath)`

支持平台：

* iOS

适用场景：

* 分享一段文本
* 分享一个本地文件路径
* 分享 `file://` 文件 URL

分享文本示例：

```js
const sdk = window.lzcSDK || window.sdk

if (sdk && typeof sdk.call === "function") {
  sdk.call("SystemShare", "https://lazycat.cloud")
}
```

分享文件示例：

```js
const sdk = window.lzcSDK || window.sdk

if (sdk && typeof sdk.call === "function") {
  sdk.call("SystemShare", "/private/path/to/report.pdf")
}
```

说明：

* 这是当前 iOS 已明确实现的原生系统分享面板能力
* 传入本地路径时会走文件分享
* 传入普通字符串时会走文本分享

### 4.6 音量 / 亮度控制

推荐入口：

* `AppCommon.GetDeviceVolume()`
* `AppCommon.SetDeviceVolume(value)`
* `AppCommon.GetScreenBrightness()`
* `AppCommon.SetScreenBrightness(value)`

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

export async function setHalfBrightness() {
  await AppCommon.SetScreenBrightness(0.5)
}

export async function setHalfVolume() {
  await AppCommon.SetDeviceVolume(0.5)
}

export async function readDeviceState() {
  const [volume, brightness] = await Promise.all([
    AppCommon.GetDeviceVolume(),
    AppCommon.GetScreenBrightness(),
  ])

  return { volume, brightness }
}
```

### 4.7 监听音量键

推荐入口：

* `AppCommon.OnVolumeKeyEvent(callback)`

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

const dispose = AppCommon.OnVolumeKeyEvent((event) => {
  console.log("volume key event", event)
})

// 页面卸载时取消监听
// dispose()
```

说明：

* 当前 iOS 原生和 SDK 封装的事件链路仍建议你在真实设备上做一次联调验证

## 5. 仅 Android 支持

### 5.1 锁屏音乐控制（MediaSession）

推荐入口：

* `MediaSession`
* `isMediaSessionAvailable()`

导入方式：

```js
import {
  MediaSession,
  isMediaSessionAvailable,
} from "@lazycatcloud/sdk/dist/extentions/mediasession/index"
```

基础示例：

```js
export async function setupPlayerMediaSession(audio, title) {
  if (!isMediaSessionAvailable()) return

  await MediaSession.setMetadata({
    title,
  })

  await MediaSession.setPlaybackState({
    playbackState: audio.paused ? "paused" : "playing",
  })

  await MediaSession.setPositionState({
    position: audio.currentTime,
    duration: audio.duration,
    playbackRate: audio.playbackRate,
  })

  await MediaSession.setActionHandler({ action: "play" }, () => audio.play())
  await MediaSession.setActionHandler({ action: "pause" }, () => audio.pause())
  await MediaSession.setActionHandler({ action: "nexttrack" }, () => {
    console.log("play next")
  })
  await MediaSession.setActionHandler({ action: "previoustrack" }, () => {
    console.log("play previous")
  })
}
```

页面接入示例：

```js
import {
  MediaSession,
  isMediaSessionAvailable,
} from "@lazycatcloud/sdk/dist/extentions/mediasession/index"

const audio = document.getElementById("audio")

async function bindMediaSession() {
  if (!isMediaSessionAvailable()) return

  await MediaSession.setMetadata({ title: "示例音频" })

  await MediaSession.setActionHandler({ action: "play" }, function () {
    audio.play()
  })

  await MediaSession.setActionHandler({ action: "pause" }, function () {
    audio.pause()
  })

  audio.addEventListener("play", function () {
    MediaSession.setPlaybackState({ playbackState: "playing" })
  })

  audio.addEventListener("pause", function () {
    MediaSession.setPlaybackState({ playbackState: "paused" })
  })
}

bindMediaSession()
```

该示例里实际使用了：

* `setMetadata`
* `setPlaybackState`
* `setPositionState`
* `setActionHandler(nexttrack / previoustrack / play / pause / seekto / stop / seekforward / seekbackward)`

### 5.2 图片原生分享

推荐入口：

* `AppCommon.ShareImage(data)`

支持平台：

* Android

适用场景：

* Android WebView 中无法直接用 `navigator.share` 分享图片
* 你已经拿到了图片二进制数据

示例：

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

async function shareCanvasImage(canvas) {
  const blob = await new Promise(function (resolve) {
    canvas.toBlob(resolve, "image/png")
  })

  const arrayBuffer = await blob.arrayBuffer()
  const data = new Uint8Array(arrayBuffer)

  await AppCommon.ShareImage(data)
}
```

### 5.3 状态栏颜色

Android 客户端通常会注入 `lzc_window` namespace。

建议封装：

```js
export function setAndroidStatusBarColor(color) {
  const api = window.lzc_window
  if (!api?.SetStatusBarColor) return
  api.SetStatusBarColor(color)
}
```

使用示例：

```js
setAndroidStatusBarColor("#FFFFFF")
setAndroidStatusBarColor("#D6E7EE")
```

说明：

* 颜色必须是 `#` 开头的字符串
* 建议仅在 Android 客户端环境下调用

### 5.4 控制栏显隐

建议封装：

```js
export async function setAndroidControlViewVisible(visible) {
  const api = window.lzc_tab
  if (!api?.SetControlViewVisibility) return
  api.SetControlViewVisibility(visible)
}

export async function getAndroidControlViewVisible() {
  const api = window.lzc_tab
  if (!api?.GetControlViewVisibility) return true
  return api.GetControlViewVisibility()
}
```

使用示例：

```js
await setAndroidControlViewVisible(false)

const oldVisible = await getAndroidControlViewVisible()
console.log("control view visible:", oldVisible)
```

### 5.5 WebView 跟随键盘 resize

建议封装：

```js
export function enableAndroidWebviewResize(enable) {
  const api = window.lzc_window
  if (!api?.EnableWebviewResize) return
  api.EnableWebviewResize(enable)
}
```

使用示例：

```js
enableAndroidWebviewResize(true)
```

适用场景：

* 输入框聚焦时，希望页面跟随键盘压缩布局
* 聊天页、表单页、编辑器页

### 5.6 客户端主题模式

建议封装：

```js
const ThemeMode = {
  DAY_MODE: 0,
  NIGHT_MODE: 1,
  FOLLOW_SYSTEM: 2,
}

export function getAndroidThemeMode() {
  return window.lzc_theme?.getThemeMode?.()
}

export function getAndroidThemeApplyMode() {
  return window.lzc_theme?.GetThemeApplyMode?.()
}

export function setAndroidThemeMode(mode, themeApplyMode) {
  window.lzc_theme?.setThemeMode?.(mode, themeApplyMode)
}
```

使用示例：

```js
const mode = getAndroidThemeMode()
const applyMode = getAndroidThemeApplyMode()

console.log({ mode, applyMode })

setAndroidThemeMode(ThemeMode.FOLLOW_SYSTEM, applyMode ?? 0)
```

## 6. 推荐接入顺序

如果你是新页面，建议按这个顺序接入：

1. 先加平台判断
2. 再加 `lzcapp-disable-dark`
3. 如果页面有分享诉求，优先接 `ShareWithFiles`
4. 如果是 iOS 轻应用，再补导航栏 / 状态栏 meta
5. 如果页面需要沉浸展示，再接全屏接口
6. 如果页面是播放器，再接 Android `MediaSession`
7. 如果页面是 Android 宿主内页面，再按需接状态栏颜色、控制栏显隐、主题模式

## 7. 常见组合示例

### 7.1 沉浸式详情页

适用平台：

* iOS / Android

建议组合：

* 两端：
  * `lzcapp-disable-dark`
  * `AppCommon.SetFullScreen`
* iOS：
  * `lzcapp-navigation-bar-scheme=sink`
  * `lzcapp-state-bar-scheme=sink`

示例：

```html
<meta name="lzcapp-disable-dark" content="true" />
<meta name="lzcapp-navigation-bar-scheme" content="sink" />
<meta name="lzcapp-state-bar-scheme" content="sink" />
```

```js
import { AppCommon } from "@lazycatcloud/sdk/dist/extentions"

await AppCommon.SetFullScreen()
```

### 7.2 iOS 文件选择 / 图片预览页

建议组合：

* iOS 关闭按钮显隐
* iOS 关闭按钮布局 CSS 变量

示例：

```js
function onPreviewOpen() {
  const bridge = window?.webkit?.messageHandlers?.["SetCloseBtnShowStatus"]
  bridge?.postMessage?.({ params: [false] })
}

function onPreviewClose() {
  const bridge = window?.webkit?.messageHandlers?.["SetCloseBtnShowStatus"]
  bridge?.postMessage?.({ params: [true] })
}
```

```css
.preview-toolbar {
  padding-right: calc(
    var(--lzc-client-safearea-view-right, 0px) +
    var(--lzc-client-safearea-view-width, 0px) +
    12px
  );
}
```

### 7.3 Android 音频播放页

建议组合：

* `MediaSession.setMetadata`
* `MediaSession.setPlaybackState`
* `MediaSession.setPositionState`
* `MediaSession.setActionHandler`

示例：

```js
import {
  MediaSession,
  isMediaSessionAvailable,
} from "@lazycatcloud/sdk/dist/extentions/mediasession/index"

export async function bindAudioSession(audio, title) {
  if (!isMediaSessionAvailable()) return

  await MediaSession.setMetadata({ title })
  await MediaSession.setActionHandler({ action: "play" }, () => audio.play())
  await MediaSession.setActionHandler({ action: "pause" }, () => audio.pause())

  audio.addEventListener("play", () => {
    MediaSession.setPlaybackState({ playbackState: "playing" })
  })

  audio.addEventListener("pause", () => {
    MediaSession.setPlaybackState({ playbackState: "paused" })
  })
}
```

## 8. 兼容性建议

* 所有宿主能力都做存在性判断，不要假设每个环境都已注入
* meta 类能力推荐在页面初始 HTML 中直接声明
* Android 侧的 `lzc_window` / `lzc_tab` / `lzc_theme` 更适合做一层项目内封装后再使用
* iOS 侧 `window.webkit.messageHandlers` 直接调用时，建议统一封装到 hook / util 中
* 需要同时兼容普通浏览器时，一定要给出 fallback 行为