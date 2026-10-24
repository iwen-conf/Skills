# T07 原生平台方向与安装

状态：`[x]`。需求：[原生平台方向](../../../../00-产品需求/05-原生平台方向.md)。

Mode: implement · Tier: T2（跨技能合同）· Scope: Arc 原生/前端/规划/验证规则、鸿蒙入口与栈说明、语料和文档 · Task: native-platform-policy。

1. `[x]` 定位旧跨平台默认与脚手架；Android 原生技能无需更换，互操作标准的 cross-platform 文案不属于 UI 选型冲突。
2. `[x]` 建立单一原生合同，移除旧默认和 Web 规则外溢，同步规划/测试与鸿蒙；75 入口结构校验通过。
3. `[x]` 12 包安装、332 文件内容/权限一致，原生参考与 Codex/Claude 各 12 个同目标链接回读通过。

不迁移任何产品、不启动设备或服务、不安装框架、不删用户外部技能、不提交推送；保留先前所有未提交改动。

<Context-Queries>

```bash
arc-idx search '跨平台' --context 2 --max 40 --format ai-md
arc-idx search 'cross-platform' --context 2 --max 40 --format ai-md
arc-idx search '(Flutter or "React Native" or Tauri or Electron or ArkUI-X or multiplatform)' --context 2 --max 50 --format ai-md
arc-idx search 'f:Arc (Expo or Taro or NativeWind or Tauri)' --context 1 --max 20 --format ai-md
```

</Context-Queries>

影响：frontend 为旧默认定义；contract/brief/code/audit/verify 为使用方；HarmonyOS full-guide 为独立冲突源。无实际产品字段/数据/网络协议迁移。共享引用的安装包需同步，包括引用 contract 的 Waza 包。

验收：`scripts/validate_skills.py`、原生方向行为语料、`gate.sh check --task native-platform-policy --max-files 24 --strict`、Arc/Waza/harmonyos-development 完整包及安装路径校验。实际平台构建与设备测试不属于本次文档更新。

## 证据

- 原生规则：`Arc/_shared/conventions/native-platforms.md`，通过共享 contract、frontend、brief 和鸿蒙入口加载。Web 颜色/状态/API/脚手架范围已明确；旧 Expo/Tauri/Taro 初始化命令从默认规则移除。
- 版本：plan/code/audit/verify 为 2.4.0；explore/closure 的入口版本仍为 2.1.0。Waza 只同步受影响的共享依赖；鸿蒙入口及 ArkUI 栈说明更新。
- 验证：75 入口通过；严格门禁 `.arc/artifacts/native-platform-policy/reports/gate-20261006T151340Z.txt` 为 `0 FAIL / 2 NOT_RUN / 0 WARN`，测试 `122 passed / 1 skipped`。新增 3 条行为语料描述原生双端、旧 Expo 局部维护和原生 macOS 证据边界；静态语料校验不冒称独立 Agent 行为实验。
- 安装：`/Users/iluwen/.agents/skills`，12 个选定包；原包保存在 `/Users/iluwen/.agents/skills-backup-l3mt9h6y`。`.arc/artifacts/native-platform-policy/reports/installed-proof.json` 记录 332 个文件 SHA-256/权限一致、5 个原生策略使用入口的参考可达，以及 Codex/Claude 各 12 个链接解析正确。
- 限制：仅技能文档/语料改变；格式检查因无已知 formatter 对应改动而 NOT_RUN，Python lint 因无 Python 改动而 NOT_RUN；既有 R8 protobuf 环境用例跳过。没有构建产品、启动模拟器或运行任何原生设备用例，包通过不表示具体应用的原生行为已验证。

用户下轮可直接使用新版技能。任务未提交或推送。
