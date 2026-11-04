# arch-diagram 业务图示例

这两份页面由 Arc plugin 0.4.0 的 `diagram` 命令调用官方 arch-diagram 1.2.3 构建。
它们展示仓库已有的 `course-v2` 工具回归快照，不是实际产品业务验收。

- [业务流程图](flows.html) · [对应 spec](flows.spec.json)：6 个节点、4 条关系。
- [业务能力树](capabilities.html) · [对应 spec](capabilities.spec.json)：4 个节点、3 条关系。
- [中文展示映射](presentation.json) · [原始构建与检查结果](build-report.json)。

输入来自 [sample-snapshot.json](../../../tools/arc-verify/workbench/dev/sample-snapshot.json)。
原始导出时间为 `2026-10-31T08:17:41Z`；源图及确认状态的指纹为
`45abde3ca7c55187d906d278cd54be36982701004bfb47864acb06062d2e0fdb`。
源快照含测试夹具当时的临时路径，这些路径只是历史出处，不是当前产品环境。

## 图中保留的事实

源快照记录了一次 FAIL 和随后一次 PASS；同轮次结果波动仍是 FLAKY，总体为 BLOCKED。
图页不会用后一次成功抹掉首次偏离，也不会将 RAW 完整性 PASS 改称业务 PASS。
当前没有真实业务包的证据映射，因此节点保留 `业务包待核实`，没有把业务 ID 或 API 名伪装成包名。
RULE、原始节点／边 ID、导出时的环境和轮次、证据错误及省略范围都保留在 spec 中。

## 本次实际验证

两个页面的官方 HTML build 都成功（退出码 0）。默认布局 check 均退出 2：

```text
no Chrome/Chromium found — set CHROME=/path/to/chrome
```

因此 HTML/spec 已生成；布局检查 BLOCKED，截图和目视检查 NOT_RUN。不能把这份示例称作视觉验收通过。
浏览器安装尝试收到上游 CDN 的 `Site Unavailable` HTML，未取得浏览器二进制。
这些页面是自包含只读文件，可直接用已有浏览器打开，无需启动服务。

重新生成时使用新的输出目录，命令不会覆盖已有图：

```bash
arc-policy diagram \
  --snapshot tools/arc-verify/workbench/dev/sample-snapshot.json \
  --presentation docs/examples/arc-business-diagram/presentation.json \
  --view flows --out .arc/verify/diagrams/course-v2/new-review
```

官方渲染器固定于
[`879ca7fa503a64ab46b5bc77d2062ded32fadc84`](https://github.com/bassnova42/arch-diagram-skill/tree/879ca7fa503a64ab46b5bc77d2062ded32fadc84)。
上游 MIT 许可证全文、版权及固定来源已嵌入每份 spec/HTML 的 `arc.renderer` 元数据。
