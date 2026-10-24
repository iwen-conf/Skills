# Source: https://developer.lazycat.cloud/dockerd-support.md

# Dockerd 开发模式

部分开发者希望微服能提供完整的 docker 套件功能, 针对于这一类用户, 微服系统 提供了一个独立的 Docker 守护进程以供开发者使用。

## 获取并安装Dockge应用

虽然非开发者用户可以使用独立 docker 套件，但具有`privileged`属性或赋予`CAP_SYS_ADMIN`等权限的容器可以读写并访问懒猫微服中的所有文件数据，甚至对系统造成无法修复的错误。因此请在启用独立 docker 套件后，仔细阅读本文的剩余内容。

\<button :class="$style.button" @click="downloadFile">下载Dockge应用LPK

* 点击上方按钮，下载`Dockge`应用lpk。
* 将lpk上传至`懒猫网盘`，并右键安装。
* 安装完成后重启懒猫微服，系统将自动激活独立 Docker 守护进程。
* (可选)若需要重启后自动启动dockerd，请在应用列表中将dockage设置为后台运行

## 使用说明

开发者可以通过以下两种方式运行自己的容器

### Dockge

该应用可以在应用列表中访问到, 通过 `dockge` 应用, 用户可以自己编写docker-compose 文件, 部署并且测试

![dockge](https://developer.lazycat.cloud/public/dockge.png)

### pg-docker

ssh 登录到微服之后, 用户可以直接使用 `pg-docker` 命令执行docker相关的命令, 通过pg-docker暴露的端口, 能够直接在内网中访问

### 关于docker存储位置

在独立 docker 套件中创建的容器将默认使用机械硬盘作为存储空间，容器在重启后内容将持久化存储。

### 将用户数据文件映射至容器内

docker容器默认状况下将与系统隔离，可以使用下面的compose表达式将用户磁盘数据绑定至容器内。

```yaml
service:
  example:
    volumes:
      - /data/document/{用户名}:/容器/内/路径
```

### 映射容器内端口

使用下面的compose表达式即可将容器内`2222`端口转发到外部的`3333`端口。如须访问，可通过懒猫微服`局域网ip:端口号`进行访问。

```yaml
service:
  example:
    ports:
      - 3333:2222
```

### 带有权限的容器

为容器添加`privileged`权限或某些[特权](https://man.archlinux.org/man/core/man-pages/capabilities.7.en)（例如`CAP_SYS_ADMIN`）会赋予该容器极高的系统权限。这样一来，容器可能会对懒猫微服的系统资源产生很大影响，甚至可能造成严重的安全风险。特别是在将容器的高风险端口暴露给外部网络时，容器可能成为攻击的目标。

如果容器内运行恶意程序，这些程序可能会影响系统的正常运行，甚至导致数据丢失或损坏。因此，在使用别人提供的Compose文件时，需要特别留意文件中是否会授予容器过多的权限，避免潜在的安全隐患。

```yaml
service:
  example:
    privileged: true
    cap_add:
      - SYS_ADMIN
      - NET_ADMIN # 开放所有网络相关权限
```

### 安装Dockge后无法创建容器

在安装Dockge后，如果您发现创建容器失败，且右下角有此提示弹出时。请确保第一次安装后已经重启微服。在不安装Dockge应用的情况下系统为保证安全将不会启用独立 docker 守护进程。
![右下角错误提示](https://developer.lazycat.cloud/public/dockge-error.png)

### pg-docker配置文件

目前 `pg-docker` 的 `daemon.json` 文件位于 `/lzcsys/var/playground/daemon.json` 目录下，该配置修改后不会回滚，不过下面的配置项会被系统强行配置：

* `bridge` 懒猫微服网络环境相关
* `cgroup-parent` 懒猫微服进程调度相关