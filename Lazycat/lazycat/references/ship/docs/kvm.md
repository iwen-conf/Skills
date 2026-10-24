# Source: https://developer.lazycat.cloud/kvm.md

# KVM 模式

## KVM 虚拟机的优势

前面几章讲的都是基于 lzcapp 模式下的应用开发细节和技巧。

其实在日常的学习研究中， 我们大部分时间并不是在开发应用， 而是在做技术实验。

这时候， KVM 模式就要比 lzcapp 模式更加灵活方便：

1. 折腾简单： KVM 并不像 lzcapp 有那么多限制， 折腾起来更像公有云服务器
2. 网络穿透： KVM 直接可以利用懒猫微服的网络穿透能力， 在 KVM 起一些实验的服务， 外网马上可以访问到
3. 云端虚拟化： 可以虚拟化 ArchLinux、 Windows 等不同的系统， 可以在云端跑迅雷、 QQ 等 Windows 软件， 不管作为云电脑办公还是环境测试， 都是很方便的

从使用体感来说， KVM 模式更像公有云服务器的操作手感， 唯一不一样的是， 懒猫微服的硬件计算资源和网络资源是独占的， 不受公有云高峰期的资源限制。

## 虚拟机和开发库安装

使用方法举例， 我们以 ArchLinux 为例：

1. 首先要安装 ArchLinux 虚拟机， 请参考[攻略](https://lazycat.cloud/playground/guideline/423)
2. 在虚拟机中用包管理器安装对应的开发库， 可使用 pacman 或 aur
3. 编写后台服务脚本并启动, 后台服务可以依托于微服的网络穿透， 对外网提供服务

## 对外访问端口

目前虚拟机已支持动态端口转发，除部分端口被用于`vnc`等服务外，其他端口都已正常开放。如果您发现部分端口无法访问，请检查
虚拟机内防护墙设置，如果端口未被拦截，则可能是该端口已被占用或暂不支持进行转发，可以参考[使用`SSH`转发虚拟机内端口](#使用ssh转发虚拟机内端口)章节，利用SSH进行转发。

### 暂不支持的端口

| Protocol |       |       |       |       |       |
| :---:    | :---: | :---: | :---: | :---: | :---: |
|   TCP    | 49199 | 49200 |  445  | 8006  | 5700  |
|   UDP    | 49199 |   -   |   -   |   -   |   -   |

### 使用`SSH`转发虚拟机内端口

如果所需端口被占用或无法转发，还可以尝试在虚拟机中使用`SSH forward`将需要的端口转发至本地，或将本地端口转发至虚拟机中。
相关教程可参考[ArchWiki-OpenSSH](https://wiki.archlinux.org/title/OpenSSH#Forwarding_other_ports)，Windows用户也可利用
[Putty](https://apps.microsoft.com/detail/xpfnzksklbp7rj?hl=en-US\&gl=US)等GUI工具使用SSH转发。

示例(在Ubuntu虚拟机中开启SSH转发):

* 如果没有安装`openssh-server`，可以使用`apt install openssh-server`安装`OpenSSH`服务
* 使用指令`sudo useradd -M -U -s /bin/false forward`创建一个用于转发的用户
* 使用指令`sudo passwd forward`设置该用户的密码
* 在`/etc/ssh/sshd_config.d/`中创建文件`30-forward.conf`并填写下面的配置:

```bash
Match User forward
   AllowTcpForwarding yes # 开启TCP转发
   PermitTTY no # 禁用ptty
   X11Forwarding no # 禁用X11转发
   ForceCommand echo 'This account is restricted to port forwarding only.' # 提示
   PasswordAuthentication yes # 允许使用密码登陆
```

* 使用`sudo systemctl restart sshd`重启`sshd`服务
* 在本地计算机上执行`ssh -N -L <端口>:localhost:<端口> forward@ubuntu.<微服名称>.heiyu.space`即可将虚拟机中端口转发到本地计算机上。

## 外网服务连接方式

ArchLinux 应用的子域名是 archlinux， 假设您的设备名为 devicename, 对外的 TCP 端口为 10086， 您就可以通过访问 archlinux.devicename.heiyu.space:10086 来访问对外提供的 TCP 服务啦。