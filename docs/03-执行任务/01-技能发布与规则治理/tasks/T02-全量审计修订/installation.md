# 最新技能本机安装记录

日期：2026-10-05。状态：`[x]`。用户在全量审计修订交付后明确要求安装最新版，本次安装来源为当前工作区，包括尚未提交的修订。

## 安装结果

- 79 个运行技能安装到 `/Users/iluwen/.agents/skills`，Arc 使用 `arc-*` 名称。
- `/Users/iluwen/.codex/skills` 与 `/Users/iluwen/.claude/skills` 各有 79 个对应符号链接，引用同一份安装内容。
- 两个宿主各归档 16 个旧 Arc 冒号入口。Codex 新建 51 个链接、保留 28 个原有链接；Claude 新建 52 个链接、保留 27 个原有链接。
- 独立可靠性来源包不默认安装；其他来源技能、系统技能、插件缓存未修改。
- 本次 79 个受管技能在三个根目录中均解析到唯一真实入口，受管范围没有独立重复副本。既有的其他来源重复项与 OfficeCLI 断链不属于本次安装验收范围，也未作清理。

## 验证

- 源入口校验：80/80 通过（79 runtime + 1 source）。
- 仓库测试：82 passed，无跳过；protobuf 测试解释器为 `/tmp/skills-audit-proto-venv/bin/python`。
- 完整 family dry-run：79 个包通过。
- 三个实际安装根的 `sync_skills.py --check`：均通过，检查全部五个 family。
- 重新从当前源码构建临时包，与实际安装逐文件比对：798 个文件的 SHA-256 与可执行权限一致。
- 上述是安装内容、引用、命名和依赖验收；当前会话技能缓存不作为下轮发现或真实 Agent 行为证据。原生平台与远程 API 的未验证范围沿用修订报告。

证据：[安装校验](evidence/installation-verification.json)、[宿主链接及恢复清单](evidence/installation-hosts.json)。

## 备份与恢复

| 内容 | 备份目录 |
| --- | --- |
| shared 原有 44 个包 | `/Users/iluwen/.agents/skills-backup-7k5wujii` |
| Codex 原有 16 个 Arc 入口及 manifest | `/Users/iluwen/.codex/skills-link-backup-6by1jrbg` |
| Claude 原有 16 个 Arc 入口及 manifest | `/Users/iluwen/.claude/skills-link-backup-eekq8ill` |

如需整体恢复，先保留当前安装的副本，再将本次 79 个 shared 包移出生效目录并恢复 shared 备份中的原有 44 个包；分别依据两个宿主 `manifest.json` 的 `created` 移出本次新建链接、依据 `saved` 恢复旧入口，`retained` 保持原样。这样会同时移出本次新增的 35 个 shared 包，不留下混合版本。本次未执行恢复。

未提交、未推送、未发布到远程。新技能预计在下一轮对话重新发现后可用。
