# W14 技能文档同步（SKILL.md、business-graph.md）

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 技能文档（执行技能 arc-code，改技能源码） |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D2 |
| 难度依据 | 改动集中在 2–3 个 Markdown 和生成的注册表；风险在 token 预算和标题结构门禁（`validate_skills.py`），以及只写已验收的能力。 |
| 目标面 | `Arc/arc:verify/` 技能源码与生成的 `skills.index.json` |
| 前置依赖 | [W07](W07-obligation-kinds.md)、[W12](W12-risk-report.md)、[W13](W13-snapshot.md)、[W11](W11-run-chain.md)、[W08](W08-derived-obligations.md)、[W09](W09-discovery.md) 各自达到 `[x]` 或 `[?]`；只写已实现的部分 |
| 下游影响 | 六技能使用者、W06 |

## 目标与范围

需求：[BG17、BG19–BG25](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`Arc/arc:verify/SKILL.md`、`Arc/arc:verify/references/business-graph.md`、`tools/arc-verify/README.md`；注册表用 `.venv/bin/python scripts/build_skills_index.py` 重新生成，不手改。

- 写清 v2 模型（节点与关系类型）、新增义务种类、风险报告、`snapshot`、`run-chain`、`discover`。
- 删掉已作废的 `workbench` 服务说法；保持 SKILL 标题结构与 token 预算。
- 已落地待写入文档（2026-10-31）：W07 的 `tests[].kind` 新值 `EXISTENCE`/`SEQUENCE`/`CONSISTENCY` 及字段 `flow`、`steps`、`layers`；`scan` 输入的可选 `commit`；W12 的 `report --kind`、`accept-risk`、`promote-risk`、`risk`、`gate --risk-review` 与 RISK `origin`/`reference`。

不做：不描述未实现或未验收的能力；不手改 `skills.index.json`；不改其他五个技能，除非交接口径变化（变化时另起卡）。

## 2026-10-31 实施记录

- `Arc/arc:verify/references/business-graph.md`：义务表补 EXISTENCE、SEQUENCE、CONSISTENCY、DERIVED 四行；`scan` 说明补 `"commit"`；新增"v2 model and runtime commands"一节，只放指向新文件的一行链接。
- 新 reference `Arc/arc:verify/references/business-graph-v2.md`：v2 字段（`history`、`equivalence.ignore`、`reference_bindings`）、状态等价规则、命令表（`explore`、`run-chain` 及退出码、`derive`、`discover`、`report --kind`、`accept/reject/promote-risk`、`risk refute`、`reference-import/match`、`mutation-report`、`snapshot`）和三种来源分离原则。拆出来是因为原文件超过 3500 token 的 reference 预算（校验器报 4014）。
- `tools/arc-verify/README.md`：新增"v2 runtime commands"命令速查（W13 已加"Static snapshot"一节）。
- `SKILL.md` 不改：它本来就把业务链工作指向 business-graph；全文没有提到已作废的 `workbench` 服务。
- 注册表：`.venv/bin/python scripts/build_skills_index.py` 重新生成（`skills.index.json` 4 行变化）。

验收：`.venv/bin/python scripts/validate_skills.py` 通过（65 个技能文件）；`.venv/bin/python -m pytest -q` 470 passed、1 skipped（protobuf 解释器，既有）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:Arc/arc:verify/ (business-graph|feature/action/condition/state)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 对照上游卡的实际命令和测试结果，逐段改文档。
- [x] 重新生成注册表；跑结构与安装校验。
- [x] 回写并同步 tracker。

## 验收

```bash
.venv/bin/python scripts/build_skills_index.py
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest -q
grep -n 'workbench --address' Arc/arc:verify/references/business-graph.md Arc/arc:verify/SKILL.md   # 期望无输出
```

文档 PASS 只证明技能说明与实现一致，不代表业务 E2E 通过。
