# T01-TOOLS-06 本机安装去重

状态：`[/]`

- 上级：[T01-TOOLS](README.md)；所属端：本机工具；优先级：P1；难度：D2。
- 难度依据：同一宿主同时发现 agents/codex 两根；需要区分同目标链接、独立副本与内容分叉，并保留恢复能力。
- 依赖：清理清单经用户确认才移出安装项；最终安装依赖 05 门禁通过。
- 范围：`scripts/sync_skills.py`、`tests/test_sync_skills.py`、本机安装项、安装维护文档。

<Context-Queries>

```bash
arc-idx files sync --format json
arc-idx symbol validate_installation --format json
arc-idx files skill-engineering.md --format json
```

</Context-Queries>

执行：给现有发布检查增加多根重复检测（归一 Arc 冒号/短横线别名；同真实路径的链接不算独立副本）；用隔离临时目录验证。不新建安装框架。用户确认后归档示例、断链、独立重复副本；共享安装统一到 agents，保留已有其他宿主链接。分叉的 Apifox/蓝湖不自动覆盖。

验收：重复副本和空安装检查失败，同目标链接检查成功；实际引用校验通过；归档有清单、原路径和恢复说明。不得把当前会话缓存的技能目录当作重新发现后的行为证据。

当前证据：仓库 80 个 SKILL.md，其中 79 个为受管入口；agents 110、codex 105（含 6 个系统入口）、claude 79。确认 16 组 Arc 别名、10 组整包相同副本、4 个示例及 2 个 OfficeCLI 断链。清理确认已发出，未收到前只做工具和文档修改。
