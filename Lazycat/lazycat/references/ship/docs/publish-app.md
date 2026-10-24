# Source: https://developer.lazycat.cloud/publish-app.md

# 发布自己的第一个应用

1. 在发布应用前， 您需要 [注册](https://lazycat.cloud/login?redirect=https://developer.lazycat.cloud/) 社区账号，并访问 [开发者中心](https://developer.lazycat.cloud/manage)，根据界面上的引导提交审核（提交申请后建议在客服群或者尝试 [联系我们](https://lazycat.cloud/about?navtype=AfterSalesService) 快速审核），申请完成后成为懒猫微服的开发者。

2. 提交应用到商店审核之前请阅读[应用上架审核指南](./store-submission-guide.md)。

3. 提交应用到商店审核:

   * 或者通过命令行工具 `lzc-cli` (1.2.54 及以上版本) 提交审核， 如何安装 `lzc-cli` 请参考 [开发环境搭建](https://developer.lazycat.cloud/lzc-cli.html)

     ```bash
     lzc-cli project build
     lzc-cli appstore publish ./your-app.lpk
     ```

# 推送镜像到官方仓库

docker hub 的网络质量不太稳定，因此懒猫官方提供了一个稳定的 registry 供大家使用

:::tip
开发者在最终上传商店前，需要将 lpk 中用的镜像推送到官方 registry，上传完毕后需要手动调整 manifest.yml 中的相关引用 (否则可能会使应用审核人员无法安装应用导致 **上架审核失败** )
:::

```
$ lzc-cli appstore copy-image <公网可以访问的镜像名称>
# 上传完成后将打印  registry.lazycat.cloud/<community-username>/<镜像名称>:<hash版本>
```

比如

```
lzc-cli appstore copy-image alpine:3.18
Waiting ... ( copy alpine:3.18 to lazycat offical registry)
lazycat-registry: registry.lazycat.cloud/snyh1010/library/alpine:d3b83042301e01a4

```

注意 `registry.lazycat.cloud` 的使用存在以下限制

1. 为了保证 LPK 引用镜像本身的稳定性，生成镜像tag会替换成IMAGE\_ID，每次执行 `copy-image` ，服务端都会强制执行一次 `docker pull`
2. 被上传的镜像必须是公网存在的，`pull` 操作是在服务端进行的，因此仅在开发者本地存在的镜像无法被 `copy-image`
3. 被上传镜像必须被至少一个商店应用引用，仓库会定期进行垃圾回收操作
4. `registry.lazycat.cloud` 仅供微服内部使用，在微服外部使用会有黑科技**限速**