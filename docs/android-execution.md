# Android 执行与验证

在构建、设备运行或视觉验证时按需应用；同会话未变化不重复读取。

- 仓内发现按宿主 arc-idx；依赖以目标 variant 的 resolved artifacts 为准，版本目录是声明而非解析结果。保留既有栈，只做本任务授权升级。
- 使用项目 wrapper、实际存在的 variant assemble/test 任务。help/dry-run 仅验证配置和任务图，不能代替编译或业务验证；依赖/设备/网络不可用标 NOT_RUN/BLOCKED。
- 设备启停服从用户及项目环境策略：当前用户默认远程 192.168.0.63 redroid（§1）；本机 AVD 只走 avdslim（§2）。SDK/文档 CLI 不接管 emulator 生命周期；adb 继续用于装包、日志和截图。redroid 无 GMS，Google 登录/FCM/凭据等需 GMS 的验证转已授权真机或本机 AVD，不能将模拟结果算通过。
- 视觉变更保留原基线，生成候选与 diff；先检查已有资产和可用截图工具，仅缺必要素材才找用户。基线缺失时视觉比较 NOT_RUN，不宣称像素一致。只有用户授权或已确立项目基线更新流程允许更新时才替换 reference；行为测试独立验证点击、禁用、恢复、导航。
- 工具/样例已确认版本与路径在会话中复用；文档/API 不匹配时查目标版本权威来源，禁止靠猜参数或降级依赖消除错误。

## 1. 远程 redroid（默认测试面）

63 主机：Ubuntu 26.04 x86_64，57 GB 内存、26 核，可并行多个实例；本机只做编译与 adb 客户端。连接 `ssh luwen@192.168.0.63`（公钥免密）。sudo 口令不入仓：从用户全局指令取值，放进当前 shell 的 `REMOTE63_SUDO`，以 `printf '%s\n' "$REMOTE63_SUDO" | sudo -S -p "" <cmd>` 执行（下文写作 `SUDO <cmd>`）。远端 shell 先 `export ANDROID_HOME=$HOME/android-sdk`，PATH 加 `platform-tools`、`cmdline-tools/latest/bin`；`JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64`。

63 是 VMware 虚拟机，没有 `/dev/kvm`，AOSP emulator 起不来；只用 redroid 容器：

```bash
# 1) binder（宿主重启后重做）：必须 binderfs，mknod 出来的节点会 ENODEV，容器内 servicemanager/zygote 全挂
SUDO modprobe binder_linux
SUDO mkdir -p /dev/binderfs
SUDO mount -t binder binder /dev/binderfs
# 对 /dev/binderfs/binder-control 发 BINDER_CTL_ADD（_IOWR(0x62,1,264) = 0xC1086201）建 binder/hwbinder/vndbinder，
# 再把 /dev/binder* 软链到 /dev/binderfs/*；本机脚本 ~/.cache/remote63-redroid-up.sh 已封装此步
# 2) 起实例 N：容器名、宿主端口（5555+N-1）、/data 目录各自独立
SUDO docker run -d --name redroid-1 --privileged -p 5555:5555 \
  -v /dev/binderfs:/dev/binderfs -v /data/redroid-1:/data \
  redroid/redroid:14.0.0_64only-latest \
  androidboot.hardware=redroid androidboot.redroid_width=1080 androidboot.redroid_height=1920 androidboot.redroid_dpi=320
# 3) 就绪：返回 1（约 30–60 秒）
SUDO docker exec redroid-1 getprop sys.boot_completed
# 4) 本机访问
ssh -f -N -L 15555:127.0.0.1:5555 luwen@192.168.0.63 && adb connect 127.0.0.1:15555
```

用完 `SUDO docker rm -f redroid-<N>`。`64only` 镜像无 Google Play/GMS；UI、导航、网络、业务流可验。

## 2. 本机 AVD（只准 avdslim）

只在用户要求本机 AVD 或必须验证 GMS 时使用。启停、瘦身、快照一律走 `avdslim`（`~/.local/bin/avdslim`）；禁止直接调 SDK `emulator`、手写 `-lowram`/`-gpu`/`-no-snapshot-save`。

| 需要 | 命令 |
|---|---|
| 开工体检（版本或环境不符先报告） | `avdslim doctor`、`avdslim --version` |
| 启动 / 无人值守 / 停止 | `avdslim start [<n>] [--headless]`；`avdslim stop [--snap] [-f]` |
| 运行中瘦身 / 恢复原厂 GMS | `avdslim on [--aggressive] [--keep=<pkg>] [--skip=<group>]`；`avdslim restore` |
| Android Studio 启动接管（SDK 升级后重装） | `avdslim install-shim`；`avdslim watch` |
| 黄金快照 | `avdslim bake`、`avdslim snapshot`、`avdslim unbake` |
| 调优与测量 / 黑屏 | `avdslim tune-avd <name> --ram=1536 --heap=256`、`measure`、`bench`；`avdslim repair` |

系统镜像只选 Google APIs（4 KB 页）：不选 Google Play（锁 `adb root`、多耗约 40% 内存）和 16 KB 页（QEMU 硬性 4 GB）。固定参数写 `~/Library/Application Support/avdslim/defaults`。
