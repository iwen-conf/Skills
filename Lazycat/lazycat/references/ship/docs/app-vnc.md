# Source: https://developer.lazycat.cloud/app-vnc.md

# 第一个VNC应用

![image-20250612182342560](https://lzc-playground-1301583638.cos.ap-chengdu.myqcloud.com/guidelines/439/image-20250612182342560.png?imageSlim)

> 按照本教程流程走，你将在懒猫上安装上自己的VNC应用，效果如上图。支持读写懒猫网盘内容，支持开机自启应用。
>
> 本教程只是一个基础配置教程,所有内容都在[lzc-vnc-sample](https://github.com/00longxiaoyi/lzc-vnc-sample)，如果需要看更复杂的配置请看[lzc-im-all-in-one](https://github.com/linakesi-xiaoe/lzc-im-all-in-one/tree/no-telegram)

### 📂 1. 准备配置文件

#### 1.1 DOCKERFILE文件

```dockerfile
# Pull base image.
FROM registry.lazycat.cloud/kasm-debian-bookworm:0.0.1
USER root

# 新增用户和用户组
RUN usermod -l lazycat kasm-user
# 给新增的用户添加权限
RUN echo 'lazycat ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers 
ENV HOME /home/lazycat
WORKDIR $HOME

# 安装应用
RUN apt-get update; apt install -y x11-apps;

# 修改原有脚本内容，将脚本中原有的 kasm_user 字符串替换成 lazycat，修改为新建的用户
RUN sed -i 's/kasm_user/lazycat/g' /dockerstartup/vnc_startup.sh
RUN sed -i '5i sudo chown -R lazycat:kasm-user /home/lazycat/' /dockerstartup/kasm_default_profile.sh

RUN cat /dockerstartup/kasm_default_profile.sh

# 复制内容并修改文件的拥有者，文件内容看具体路径下对应的文件
COPY --chown=lazycat:kasm-user kasmvnc.yaml /home/lazycat/.vnc/kasmvnc.yaml
COPY --chown=lazycat:kasm-user desktop/X11-Xeyes.desktop /home/lazycat/Desktop/
COPY --chown=lazycat:kasm-user mount-mappied /home/lazycat/

# autostart 实现开机自启软件和开机自启脚本
COPY --chown=lazycat:kasm-user desktop/X11-Xeyes.desktop /home/lazycat/.config/autostart/
COPY --chown=lazycat:kasm-user desktop/startup-script.desktop /home/lazycat/.config/autostart/
COPY --chown=lazycat:kasm-user startup-script.sh /home/lazycat/.config/autostart/

# 修改对应脚本的权限，权限不对时将无法正常运行脚本（请检查好权限问题）
RUN chmod +x /home/lazycat/mount-mappied
RUN chmod +x /home/lazycat/.config/autostart/startup-script.sh
RUN chmod +x /home/lazycat/Desktop/*.desktop
RUN chmod +x /home/lazycat/.config/autostart/*.desktop

# 禁用验证 优化远程桌面体验
ENV VNCOPTIONS "-PreferBandwidth -disableBasicAuth -DynamicQualityMin=4 -DynamicQualityMax=7 -DLP_ClipDelay=0 -sslOnly=0"
ENV VNC_PW lazycat

USER lazycat
```

#### 1.2 startup-script.sh

```sh
#!/bin/bash
set -ex

mkdir -p /home/lazycat/lzc-home

# 映射权限：详细请看：https://github.com/brauner/mount-idmapped
sudo /home/lazycat/mount-mappied --map-mount b:0:1000:1 /lzcapp/run/mnt/home /home/lazycat/lzc-home

# 映射对应目录到桌面
ln -svfn /lzcapp/var '/home/lazycat/Desktop/本应用数据(支持读写)'
ln -svfn /home/lazycat/lzc-home '/home/lazycat/Desktop/懒猫网盘数据(支持读写，请谨慎操作)'
```

#### 1.3 kasmvnc.yaml

```yaml
network:
  ssl:
    pem_certificate: ${HOME}/.vnc/self.pem
    pem_key: ${HOME}/.vnc/self.pem
    require_ssl: false
  udp:
    public_ip: 127.0.0.1

runtime_configuration:
  allow_override_standard_vnc_server_settings: false
  allow_client_to_override_kasm_server_settings: false
  allow_override_list: []  # 禁止客户端覆盖任何选项

encoding:
  max_frame_rate: 60  # 最高帧率
  full_frame_updates: 60  # 始终全帧更新，确保无损一致性

  video_encoding_mode:
    jpeg_quality: -1  # 禁用视频编码模式
    webp_quality: -1
    max_resolution:
      width: 1920
      height: 1080
    scaling_algorithm: progressive_bilinear  # 不缩放图像

  compare_framebuffer: off  # 关闭帧缓冲对比，强制发送全部数据以保证无损
  zrle_zlib_level: 0  # 关闭压缩以避免损失
  hextile_improved_compression: false  # 禁用增强压缩以避免画质损失

desktop:
  gpu:
    hw3d: false
    drinode: /dev/dri/renderD128
```

### 🚀 2.在微服上构建测试镜像，使用测试镜像进行测试

#### 2.1 上传文件到微服网盘中

![image-20250612174456805](https://lzc-playground-1301583638.cos.ap-chengdu.myqcloud.com/guidelines/439/image-20250612174456805.png?imageSlim)

#### 2.2 构建测试镜像

* 通过SSH登录上微服（[SSH开启教程](https://developer.lazycat.cloud/ssh.html#%E5%BC%80%E5%90%AF-ssh)），找到刚上传的文件，网盘对应的目录在`/data/document/用户名/`下。

![image-20250612175015450](https://lzc-playground-1301583638.cos.ap-chengdu.myqcloud.com/guidelines/439/image-20250612175015450.png?imageSlim)

* 构建镜像

```shell
# 构建镜像 
lzc-docker build -t dev.微服名.heiyu.space/镜像名:版本号 .

# 查看镜像
lzc-docker images | grep dev.微服名.heiyu
```

![image-20250612180605983](https://lzc-playground-1301583638.cos.ap-chengdu.myqcloud.com/guidelines/439/image-20250612180605983.png?imageSlim)

> 1. 如果是手动构建测试镜像，一定需要把镜像的 tag 标记为 `dev.$BOXNAME.heiyu.space` 地址，`$BOXNAME` 为目标微服名。推荐优先使用 `lzc-build.yml` 的 `images` 机制，详见 [lzc-build.yml 镜像构建](./build.md#images)。
>
> 2. 为什么用lzc-docker，请看[微服一台机器跑三套 Docker？](https://mp.weixin.qq.com/s/_dXE0CxWvLgA5EX1sIft8Q)

* 上传镜像

```shell
lzc-docker push 镜像名
```

![image-20250612180825422](https://lzc-playground-1301583638.cos.ap-chengdu.myqcloud.com/guidelines/439/image-20250612180825422.png?imageSlim)

当完成上传之后，就可以将 `lzc-manifest` 里面的 image 地址换成刚 push 的镜像了。静态元数据放在 `package.yml`，下面展示 `lzc-vcn-sample` 的配置：

```yml
# package.yml
package: ltp.lzcapp.vnc
version: 0.0.1
name: lzc-vnc-test
description:
license: https://choosealicense.com/licenses/mit/
homepage:
author:
```

```yml
# lzc-manifest.yml
lzc-sdk-version: 0.1
application:
  subdomain: im2
  routes:
    - /=http://imallinone.ltp.lzcapp.vnc.lzcapp:6901
  depends_on:
    - imallinone
  multi_instance: true # 是否启用多实例

services:
  imallinone:
    image: 替换为刚push的镜像
```

通过`lzc-cli project build`构建lpk包 ，`lzc-cli lpk install xxx.lpk`安装镜像

> 如果对构建lpk包不熟悉的，请看开发者手册 [开发自己的第一个lpk应用](https://developer.lazycat.cloud/app-example-go.html)

### 😃 3.打开应用验证

如果你是按照教程一步一步来的，你可以在自己的启动托盘上看到一个名为：lzc-vnc-test 的应用

![image-20250612182342560](https://lzc-playground-1301583638.cos.ap-chengdu.myqcloud.com/guidelines/439/image-20250612182342560.png?imageSlim)