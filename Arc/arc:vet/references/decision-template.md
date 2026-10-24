# Decision Record Template

Read in `arc:vet` workflow step 8. Use the project's ADR format when it has one; otherwise write
`docs/10-复盘决策/NN-选型-<主题>.md` with the Chinese headings below. Fill only verified facts;
write `未核实` and the reason for anything not checked. Replace every example value.

```markdown
# 选型：<能力名称>

## 需求与约束

- 功能边界与调用方：
- 语言 / 版本 / 运行平台：
- 关键约束（性能、安全、兼容、本项目许可证、是否允许新增依赖）：
- 检查日期（as_of）、GitHub 是否可访问：
- 检索关键词族与来源：
- 本地复用检查（现有依赖、标准库，`file:line`）：

## 候选比较

| 项目（官方 URL） | 功能适配 | 最近实质维护 | 最近稳定版 | 许可证 | 安全 / 依赖 / 集成风险 | 结论 |
|---|---|---|---|---|---|---|
| owner/repo-a | | YYYY-MM-DD（commit 链接）/ 未核实 | vX.Y.Z，YYYY-MM-DD / 无 | SPDX / 未核实 | | 采用 / 借鉴 / 淘汰 |
| owner/repo-b | | | | | | |
| owner/repo-c | | | | | | |

## 证据

- repo-a：核心路径提交、Release、Issue/PR 响应、维护者、LICENSE（仓库与发布包）、安全公告来源与结果，各附链接。
- repo-b：同上。
- repo-c：同上。

## 否决与例外

- 淘汰项与否决规则（归档 / 停更 ≥36 个月 / 许可证不明或不兼容 / 未修复严重漏洞 / 不兼容平台）：
- 停更 24–36 个月项目的处理：
- 过时例外（如有）：维护者响应、兼容性验证、下游使用、补丁渠道的证据；接受的风险；退出方案：
- 未核实项及原因、复核命令：

## 决定

- 方案层级（复用现有 / 引入成熟库 / 借鉴设计独立实现 / 小范围自研）：
- 理由，以及不选其他候选的原因：
- 版本锁定：
- 最小验证（PoC 或测试命令）：
- 安全、性能与回归测试：
- 升级与退出方案：
```
