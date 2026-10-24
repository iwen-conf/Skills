# T05 Waza 精选整合与安装

状态：`[x]`。日期：2026-10-06。所属端：技能/工具；优先级 P1；难度 D3。

需求：[Waza 精选与整合](../../../../00-产品需求/03-Waza精选与整合.md)。逐项比较：[Waza 清单](../../../../../Waza/README.md)。

## 范围与输入

上游固定提交 `d8fa34ff50a37043d40bbd3b9e622029a12dd37b`。本机同名 8 技能均已存在且与上游新版本不同；上游 plugin 目录仅为镜像，不重复安装。

1. 保留并适配 read/write/learn/check/health；合并 think/hunt/ui 到已有 Arc 所属能力。
2. Waza 成为本仓的受管家族；修改注册、完整包测试、README 和来源许可。
3. 本机安装采用 shared 根，Codex/Claude 引用同份；被替换/合并的旧包移到可恢复备份。
4. 保留本轮开始时 45 个已修改或未跟踪文件；不提交推送，不执行真实发布或公开消息，不改无关技能和全局配置。

## 上下文检索

<Context-Queries>

```bash
arc-idx files '*SKILL.md' --format json
arc-idx files 'scripts/sync_skills.py' --format json
arc-idx files 'src/arc_core/domain/skill.py' --format json
arc-idx files 'tests/test_sync_skills.py' --format json
arc-idx search 'f:^scripts/sync_skills.py "family"' --format ai-md --context 3 --max 5
```

</Context-Queries>

以上已执行。上游 clone 独立建 arc-idx 文本索引；现有默认工具配置未产生符号表，比较采用 files/search 与逐文件阅读，未调用其他仓内引擎。本机全局技能目录在仓外，使用精确 rg 路径发现；未发现 skills 锁文件或已安装 Waza 插件副本。

## 决策与辅助证据

| 技能 | 取舍 | TypeSafe noul | 依据 |
|---|---|---:|---|
| think | 合并到 Arc plan | 0.75 | 需求、任务、交接已覆盖，吸收零配置和脆弱前提检查 |
| hunt | 合并到 Arc code repair | 0.70 | 根因、反证、兄弟路径已覆盖，补历史/截图/安装 oracle |
| ui | 合并到 Arc frontend | 0.84 | 通用设计重叠，保留截图矩阵和减少动效的语义反馈 |
| read | 升级适配 | 0.71 | 独立获取能力，改进默认隐私边界及平台路由 |
| write | 升级适配 | 0.57 | 实际对比确认技术说明、口吻保护、批量清单与方言能力；保留原默认行为 |
| learn | 升级适配 | 0.59 | 实际对比确认多源组织增益与取消强制模式确认，保留不确定材料 |
| health | 收窄后保留 | 0.92 | Agent 配置/权限/工具/技能体检独立；删源码扫描和默认会话采集 |
| check | 收窄后保留 | 0.90 | Arc 仅给发布决策输入，本技能保留明确授权的交付执行及回读 |

这些是本回合 `jev ask` 返回的语义辅助，不是质量分、许可或删除门禁。每项最终选择由上游/本机/Arc 的内容对比与可重复验证共同支持；原包全部可恢复。

## 验收与进度

| 项目 | 状态 | 证据 |
|---|---|---|
| 8 技能比较、5 保留与3合并 | `[x]` | 逐项源文件读取、Waza 清单和已落地参考 |
| 脚本兼容与完整包回归 | `[x]` | 75 入口校验，122 passed / 1 skipped；11 项新 CLI 用例 |
| 本机安装与旧入口消冲突 | `[x]` | 11 包真实安装与回读通过；291 文件内容/权限一致；13 项安装入口检查通过；旧入口与链接已备份 |

```bash
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
.venv/bin/python scripts/build_skills_index.py
.venv/bin/python scripts/sync_skills.py --family Waza --family Arc --naming dash --dry-run
.venv/bin/python scripts/sync_skills.py --family Waza --family Arc --naming dash
.venv/bin/python scripts/sync_skills.py --family Waza --family Arc --naming dash --check
```

## 安装记录与限制

安装到 `/Users/iluwen/.agents/skills`，Codex 和 Claude 各 11 个入口均解析到同份共享包。`arc-plan`、`arc-code`、`arc-verify`、`arc-audit` 为 2.2.0；`arc-explore`、`arc-closure` 保持 2.1.0。5 个 Waza 入口为本仓适配版本，来源提交固定如上。

- 替换前的 11 个包：`/Users/iluwen/.agents/skills-backup-hkx0mx4g`。
- 已合并的 think/hunt/ui 原包和 Claude 链接：`/Users/iluwen/.agents/waza-retired-khhc_fbh`。
- 恢复清单：`/Users/iluwen/.agents/waza-retired-khhc_fbh/restore-manifest.json`，记录原路径、备份路径、链接目标及新增 Codex 链接。恢复旧入口时先移回 shared 包，再按清单恢复宿主链接。
- 实际安装证据：`.arc/artifacts/waza-curation/reports/installed-proof.json`。重新打包后比较 291 个文件的 SHA-256 和权限，全部一致；两宿主链接与三个已合并入口的缺席均通过验证。
- 实际脚本证据：对安装路径执行 13 项检查，覆盖 Gollum/Obsidian 标签修复与目标保护、方言检测、shell 包装器、默认直连、显式网络代理、403 失败时不启用第三方提取。HTTP 使用受控临时本地 fixture，执行后已停止。
- 最终门禁：`.arc/artifacts/waza-curation/reports/gate-20261006T140429Z.txt`，`0 FAIL / 2 NOT_RUN / 0 WARN`；严格测试为 `122 passed / 1 skipped`。最初门禁发现上游两处类型屏蔽注释，移除后重跑通过。

`NOT_RUN`：Ruff 与 mypy/pyright 未安装，格式/类型检查未执行；已有虚拟环境的 Python 版本不在项目声明的 3.12 范围内，本轮未调整环境。1 项已有 R8 protobuf 测试因缺少满足版本要求的 `R8_PROTO_PYTHON` 跳过。框架及本地 CLI 验收不代表真实发布、线上 MCP、第三方网页提取、GUI 或长期研究质量已验证；本轮未执行这些外部动作。新技能在下一轮可加载。
