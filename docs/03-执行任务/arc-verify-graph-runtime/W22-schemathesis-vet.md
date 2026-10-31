# W22 Schemathesis 选型

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 选型（执行技能 arc-vet） |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D2 |
| 难度依据 | 新依赖必须先有 arc-vet 记录；需要比较 ≥3 个候选，核对维护、许可证、传递依赖和接入方式（外部 CLI 适配器还是嵌入）。 |
| 目标面 | 选型记录（`docs/10-复盘决策/`）；不改源码与依赖清单 |
| 前置依赖 | 无 |
| 下游影响 | W09 探索适配器、W03 runner 适配器清单 |

## 目标与范围

需求：[BG31](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」；外部依据见 PRD「外部研究核对与借鉴」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

候选至少包括：Schemathesis（MIT，2026-10-27 发布 v4.32.7）、AutoRestTest（研究原型，无发布）、RESTler 或其他同类。

要比较的方面：
- 作为现有 `AdapterCall` argv 适配器接入的方式；
- 能否输出逐步请求/响应，作为 RAW；
- 状态化链路与自定义业务断言能力；
- 许可证与停更情况。

结论只有四种：复用、采用、借鉴设计、自建。

不做：不安装依赖；不改 `go.mod`、`package.json`；不把选型结论当成已接入。

## 2026-10-31 结论

选型记录：[01 选型：API 状态化探索适配器](../../10-复盘决策/01-选型-API状态化探索.md)（新建 `docs/10-复盘决策/` 与其 README）；原始证据在 `.arc/artifacts/verify-schemathesis-vet/reports/`。

- 结论：**借鉴设计**——W09 的 DISCOVERY 走模型驱动自建（runner 现有 HTTP 适配器加独立 Oracle）；**Schemathesis 4.32.7（MIT）作为可选外部命令适配器**，只在目标有 OpenAPI 时启用，不进 `go.mod`。
- 比较了 7 个候选：Schemathesis（条件采用）、CATS（备选）、RESTler 与 EvoMaster（借鉴设计）、WuppieFuzz（不适配）、AutoRestTest（无稳定版，需要第三方 LLM）、openapi-fuzzer（停更 36 个月以上且为 AGPL，否决）。
- 关键事实：首个试点 IM-mongo 没有 OpenAPI 文档，所有候选都以它为输入；要用 Schemathesis 得先由用户确认维护一份 OpenAPI。
- 未核实：CATS 发布包的安全公告；各候选核心路径 diff 的人工审阅（记录里附了复核命令）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (AdapterCall|performCall|validateAdapter)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 按 arc-vet 流程收集候选证据（带日期和链接）。
- [x] 写选型记录；结论回写 W09、W03 卡。
- [x] 同步 tracker。

## 验收

```bash
test -s docs/10-复盘决策/01-选型-API状态化探索.md && grep -c "Schemathesis" docs/10-复盘决策/01-选型-API状态化探索.md   # 记录主题是 API 状态化探索，Schemathesis 是其中候选
```

选型完成不代表已接入；接入另起卡。
