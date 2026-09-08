# T01-TOOLS-04 飞书与安装验证

状态：`[x]`

- 上级：[T01-TOOLS](README.md)；所属端：本机集成；优先级：P1；难度：D2。
- 难度依据：源仓库飞书入口与运行时兼容入口版本不同，必须核对可用 CLI 和依赖来源，再验证实际加载路径。
- 依赖：01 安装实现、02 门禁、03 最终文档通过检查；下游：最终交付。
- 范围：飞书会议相关源文件、发布说明、安装器必要支持、本机对应技能；不调用远端业务接口。

<Context-Queries>

```bash
arc-idx files lark-minutes --format json
arc-idx files lark-meeting --format json
arc-idx files sync_skills.py --format json
```

</Context-Queries>

执行：核对已安装 Lark 兼容入口及 CLI 离线帮助；从真实来源补齐会议入口/资源或恢复兼容的完整源包；先暂存检查，再同步本机；核对两处技能根目录的名称去重与断链。不得编造接口、覆盖无关第三方技能或触发远端消息。

验收：所有修改入口和必需资源在实际安装面可读，别名名称与 UI 元数据一致；保存回滚位置；运行全套 pytest、技能校验、注册表生成和安装后检查；进度表记录真实终态。

当前验证：`lark-cli` 1.0.93 内嵌 `lark-meeting` 已导入；兼容入口声明依赖并指向它。本机 `~/.agents/skills` 为 dash 名（15 个 Arc + 会议包），`~/.codex/skills` 为 colon 名；会议包在 Codex 以符号链接可读。安装后校验本次包 0 错。回滚目录：`~/.agents/skills-backup-8nnsrt4r`、`~/.agents/skills-backup-_jv1sub0`、`~/.codex/skills-backup-b8aj9mem`。未提交、未调远端飞书。
