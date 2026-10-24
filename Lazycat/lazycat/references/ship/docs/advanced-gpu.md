# Source: https://developer.lazycat.cloud/advanced-gpu.md

# GPU 加速

当我们开发多媒体应用时， GPU 加速就非常关键， 为应用开启 GPU 加速很简单， 只需要在 `lzc-manifest.yml` 文件中的 `application` 字段下加一个 `gpu_accel: true` 子字段即可， 举例：

```yml
application:
  gpu_accel: true
```