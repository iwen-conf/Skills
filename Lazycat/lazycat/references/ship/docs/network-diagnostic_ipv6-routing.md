# Source: https://developer.lazycat.cloud/network-diagnostic/ipv6-routing.md

# IPv6 网络异常

您当前的IPv6网络环境启用了TUN或者TPROXY模式代理，微服连接可能会被干扰。您需要将`6.6.6.6/32`以及`2000::6666/128`这两个 ip 添加至TUN旁路由规则中

由于不同系统和软件的设置方法不同，您可以参考以下链接进行更详细的设置:

[配置指南](https://github.com/wlabbyflower/peppapigconfigurationguide)

::: tip 6.6.6.6

6.6.6.6以及2000::6666这两个IP并非官方服务器，也不会实际向这两个IP发起任何网络请求。

这两个IP仅仅是普通的公网IP，微服客户端会利用`UDP Dial 6.6.6.6`的形式去检测本地网络出口，以最低成本来判断当前是否有局域网。

在存在第三方代理软件时，此IP还可以作为桥梁，来让用户有针对性的进行规则配置，以便让微服客户端能正确获取局域网出口IP等，否则会导致所有打洞逻辑全部失效。

:::