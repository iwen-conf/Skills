# 80 项 SKILL 审计修订结果

日期：2026-10-05。输入审计基线：`03b611e`。范围：本仓技能、直接依赖的参考/脚本、覆盖校验与本地任务文档；未安装到 Agent 目录、未发布、未推送，未调用飞书/CNB 写 API。

以上为修订阶段边界。用户随后授权本机安装，79 个运行技能已安装并验证；详见 [后续安装记录](installation.md)。

80/80 项完成逐项处置：**72 个 SKILL.md 修订，8 个按审计建议保留**。没有整体删除任何技能；专业长文按需拆分，共享合同保留单一维护源。详见 [80 项处置清单](remediation.csv) 与 [原始审计](audit-source.md)。本次完成的是本地技能修订验收，不是所有技能的真实 Agent/平台行为认证。

| 家族 | 入口数 | 修订 | 保留 |
| --- | ---: | ---: | ---: |
| Arc | 16 | 15 | 1 |
| Android | 19 | 18 | 1 |
| HarmonyOS | 2 | 2 | 0 |
| CNB | 14 | 14 | 0 |
| Lark | 28 | 22 | 6 |
| 独立可靠性来源包 | 1 | 1 | 0 |
| 合计 | 80 | 72 | 8 |

## 主要修订

- CNB：base 贯穿分支创建与 PR，保留原有工作/暂存；HTTP、SHA、共同祖先失败显式退出；评审覆盖删除与锁文件，未审范围报告 partial；路径转换只承诺实现支持的 Markdown/HTML 链接。
- Android/HarmonyOS：R8 独立可执行脚本、无数据失败与量化/启发式区分；Styles 的 clickable 传 enabled；verified-email 仅服务端验证且返回会话才认证成功，不输出 claims；真实解析依赖与设备/视觉验证边界；ArkTS 保留项目 registry、固定项目 cwd 和 wrapper。
- Arc/来源包：默认栈、workspace、任务分级与 prewalk 合同归一；安全扫描进程成功不等于安全通过；trace 采用项目结构化 logger 或 slog JSONHandler；来源包只作独立分发来源，日常默认入口为 arc:reliability。
- Lark：共享身份/授权/路径分层，既有同范围授权持续有效；日程活动区间扫描修复嵌套冲突漏报；会议内容总结必须读取正文；事件退出0独立于业务目标；简单文档快速路径、表格只读与结构编辑边界、有限公式修复、Slides 按需读取和原生素材 fallback。
- 打包：修正 ArkTS 官方资料、妙搭创意入口、邮件、事件与 Slides 的可达参考断链；Perfetto 分析显式声明 SQL owner 依赖。

## 已执行证据

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 全入口 frontmatter / 规则 / 预算 / 入口链接 | PASS，80 = 79 runtime + 1 source | [validation.txt](evidence/validation.txt) |
| 仓库测试 | PASS，82 tests，含新增26项本地回归/打包测试，无 skip | [tests.txt](evidence/tests.txt) |
| 79 个运行包完整 family dry-run | PASS，未修改安装目标 | [packaging.txt](evidence/packaging.txt) |
| 从80入口递归核对可达资源 | PASS，730 个资源，0 本地断链 | [source-links.json](evidence/source-links.json) |
| Python AST / Node / bash 语法 | PASS；PowerShell NOT_RUN | [syntax.json](evidence/syntax.json) |
| git diff --check | PASS | 本地执行 |

新增回归包含：独立计数核对 R8 分数、空/缺失/重复/越界/悬空引用/未知约束失败、protobuf 转换与未知 wire 字段拒绝、旧输出不覆盖；100组区间扫描与独立两两比较/离散空闲 oracle；转换支持与代码区保留、缺环境；临时 Git 非默认 base 与原暂存保留；整行 JSON 日志；mock ArkTS cwd/参数/失败传播；source 校验不安装；完整包及独立 Perfetto 依赖。

复现主要命令：

```bash
.venv/bin/python scripts/validate_skills.py
# R8_PROTO_PYTHON 指向安装 protobuf >=6.33.4,<7 的测试解释器；本次在系统临时目录建环境。
R8_PROTO_PYTHON=/tmp/skills-audit-proto-venv/bin/python .venv/bin/python -m pytest tests -q
.venv/bin/python scripts/sync_skills.py --dry-run --family Arc --family Android --family HarmonyOS --family CNB --family Lark
```

未给 R8_PROTO_PYTHON 且默认解释器无 protobuf 时，该单项会明确 skip，不得把这种运行记为本次的无跳过证据。使用 Node 26.3.1、Go 1.26.5 和 protobuf 6.33.6 完成对应本地检查。已更新 Arc 版本、CHANGELOG 与生成的路由索引；token/账单收益未测量。

## 保留项

`android-intent-security`, `arc:fix`, `lark-contact`, `lark-meeting`, `lark-minutes`, `lark-note`, `lark-vc-agent`, `lark-vc`。

这些入口在审计中无实质缺陷，仍参与入口/链接/打包校验；不为制造改动而修改。

## 未执行与外部不确定性

- Android/HarmonyOS 原生构建、设备/GMS、实际 R8 生产构建、飞书/CNB API 和真实 Agent 行为：NOT_RUN。脚本 fixture、静态语法和包闭包证据不替代这些行为。
- PowerShell 不可用，run.ps1 未执行；bash 分支通过 mock 项目测试。
- verified-email 官方实现页仍有 digital 外层文字与 requests 示例冲突。已读取官方页面确认并写入技能，目标依赖/provider 请求合同为 NEEDS_TARGET_VALIDATION；没有声称任一封装已经实测。服务端认证成功边界和日志修订已完成。
- ArkTS 官方链接的远端路径依据完整仓库树核对，master 不是固定 API 版本证据。

详见 [外部资料与限制](evidence/api-provenance.md)。
