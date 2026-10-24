# Waza 精选与本机适配

来源：[tw93/Waza](https://github.com/tw93/Waza/tree/d8fa34ff50a37043d40bbd3b9e622029a12dd37b)，固定提交 `d8fa34ff50a37043d40bbd3b9e622029a12dd37b`，2026-10-06 核对。上游 `skills/` 为源，`plugins/waza/skills/` 是生成镜像；本次不重复安装插件。保留 [MIT 许可](LICENSE)，打包时随引用复制。

本机原已安装这 8 个同名技能的旧版本。2026-10-06 按实际差异保留 5 个独立入口，将 3 个重叠入口的有效增量合入 Arc；2026-10-08（T08）再把 read、learn、write 与 AnySearch 合并为一个 `research-write`（fetch / search / research / edit 四个模式），现为 3 个入口：check、health、research-write。

| 上游技能 | 处理 | 当前入口 / 选择依据 |
|---|---|---|
| read | 升级并适配，T08 并入 research-write | [fetch 模式](research-write/references/fetch.md)：URL/PDF 获取；默认直连，第三方提取需授权 |
| write | 升级并适配，T08 并入 research-write | [edit 模式](research-write/references/edit.md)：原作者口吻、技术说明、双语及本地化；标点门禁 |
| learn | 升级并适配，T08 并入 research-write | [research 模式](research-write/references/research.md)：多源研究、溯源与矛盾记录 |
| health | 保留独特能力、收窄职责 | [health](health/SKILL.md)：Agent 指令/权限/MCP/安装体检；不带源码扫描器，不默认采集会话；代码审计归 Arc |
| check | 保留独特能力、收窄职责 | [check](check/SKILL.md)：已授权提交/推送/发布及线程跟进；一般 review、实现、运行时证据分别交 Arc |
| think | 合并，不保留并行入口 | [Arc brief](../Arc/arc:plan/references/brief.md)：吸收零配置成本、脆弱前提与简化检查；Arc 的任务/来源/授权规则更完整 |
| hunt | 合并，不保留并行入口 | [Arc repair](../Arc/arc:code/references/repair.md)：补已知好版本、隔离 bisect、截图/实际安装 oracle；保留 Arc 的根因与兄弟路径门禁 |
| ui | 合并，不保留并行入口 | [Arc visual iteration](../Arc/_shared/conventions/visual-iteration.md)：吸收截图目标/保留边界、状态矩阵、减少动效后的语义反馈；通用设计已有专项技能 |

未导入上游的强制表情话术、人工概率阈值、源码树扫描器、默认会话采集、自建飞书凭据流程、Safari AppleScript 浏览器流程和通用 UI 强制动效规则。状态栏及全局 optional rules 属独立配置面，本次没有安装。

## 本地修订

- `read` 的 wrapper 原有第二位置网络代理参数未到达 Python 本地抓取层；现在显式传递。默认保持直连，`--use-proxy` 仍专指用户许可的第三方提取。
- `write` 保留默认 Gollum `[[label|target]]`；通过 `--wikilink-style obsidian` 选择 `[[target|label]]`。两种方言都保护链接目标，只检查/修复显示文字。
- `learn` 声明 read/write 为包依赖；语义筛选按本机 TypeSafe 规则执行，确定性标点检查留在本地。
- check/health 不携带上游扫描或执行脚本。外部动作须有本任务授权；已有授权跨阶段有效。

## 分发

本仓为这 5 个适配包的维护源。使用本仓同步器，避免上游整包安装重新引入已合并的入口或覆盖适配：

```bash
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
.venv/bin/python scripts/sync_skills.py --family Waza --naming dash --dry-run
.venv/bin/python scripts/sync_skills.py --family Waza --family Arc --naming dash
.venv/bin/python scripts/sync_skills.py --family Waza --naming dash --check
```

此次具体比较、概率辅助、安装备份和验证限制见 [执行记录](../docs/03-执行任务/01-技能发布与规则治理/tasks/T05-Waza精选整合/README.md)。
