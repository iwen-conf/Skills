# Source: https://developer.lazycat.cloud/network-config.md

# 如何配置微服的网络

除了通过系统设置页面修改网络之外, 针对部分开发者, 微服也提供了更为灵活的方式进行网络配置. 本文将介绍如何通过配置文件的方式修改网络配置.

首先需要获取微服的[ssh权限](https://developer.lazycat.cloud/ssh), 一旦通过ssh登录上微服之后, 可以使用 `nmtui` 和 `nmcli` 命令来修改网络配置.下面重新配置有线使用静态ip为例

## 设置使用静态ip

执行`nmtui`命令后, 会进入一个交互式的界面, 选择`Edit a connection`选项, 选择需要修改的连接, 比如`Wired connection 1`, 然后选择`Edit`, 在弹出的界面中, 选择`IPv4 CONFIGURATION`, 将`Method`设置为`Manual`, 然后添加`Address`, `Netmask`, `Gateway`, `DNS Servers`等信息, 最后选择`OK`保存设置.

![nmtui](https://developer.lazycat.cloud/public/nmtui.png)

完成之后可以通过`nmcli`命令重新加载网络配置, 例如:

```
nmcli device reapply enp2s0
```