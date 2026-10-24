# Source: https://developer.lazycat.cloud/advanced-setupscript.md

# setup\_script

在应用移植时，偶尔会遇到进程权限和 lzcapp 默认权限 (root) 不一致，或需要在容器启动前执行一些初始化操作。

`manifest.yml:services[].setup_script` 字段可以处理相关问题。

::: tip 容器进程 UID
如果仅是用户权限问题，您可以使用 `manifest.yml:services[].user` 字段调整 UID
:::

`setup_script` 的原理是替换 docker image 本身的 entrypoint/command 参数，执行完 `setup_script` 后由 lzcos
自动在执行原始的 entrypoint/command 逻辑。

使用 `setup_script` 时需要注意其运行时机

1. 每次容器启动时都会重新执行一次。因此请注意不要重复进行有副作用的操作。
2. `manifest.yml:services[].binds` 是在容器进入 created 状态时就已经操作完毕，此时 `setup_script` 还未执行
3. 原始entrypoint/command是在 `setup_script` 之后执行，因此不能依赖容器镜像本身的一些初始化状态

比如

```yaml
services:
  cloudreve:
    image: registry.lazycat.cloud/xxxxx/cloudreve/cloudreve:a9e2373b7ca59bc4
    binds:
      # 先将应用 var 目录下的一个子目录 bind 到应用原生的存储目录，
      # 避免修改上游代码。(这里假设是 /conf/ 目录)
      - /lzcapp/var/cloudreve:/conf/

    # 然后把 lpk 里的预装配置拷贝到 /conf 目录
    setup_script: |     # 这个 "|" 是 yml 的字符串多行语法，
      if [ -z "$(find /conf -mindepth 1 -maxdepth 1)" ]; then
          cp -r /lzcapp/pkg/content/defaults/ /conf/
      fi
```