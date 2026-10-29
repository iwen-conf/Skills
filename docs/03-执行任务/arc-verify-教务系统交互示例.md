# 教务系统交互的业务链验证示例

目标：“测试教务系统的交互”。

## 1. 用户先确认业务契约，而不是让实现定义正确性

AI 首先盘点学生、教师、管理员的页面、菜单、按钮、API 和源代码，
给出应有业务目录与实际发现入口目录。用户确认缺失/多余/废弃的功能范围。
来自行业惯例或模型推理的内容一律归入 `RISK`，不能被算作 RULE 或业务缺陷。

例如，时间冲突校验如果没有被用户明确确认为规则，仅为待审风险。
相反，用户明确要求“已登录且开放的课程可报名，名额不足必须拒绝”
才可以建立可通过/失败的验收规则。规则必须带身份、入口、全部前置条件、
成功与失败预期、独立读回判据及来源。

源码里找不到功能，最多是 `NOT_FOUND` 候选缺口；还要检查其他角色、
隐藏入口、API 和运行态，才可交由用户确认 MISSING。

## 2. 构建图并生成任务

- FEATURE：选课、退课、课表、审批等能力。
- CONDITION：学生已登录、窗口开放、名额充足。
- ACTION：登录、提交报名、审批、退课。
- STATE：未选、待审、已选、已取消。
- REQUIRES：条件到操作；TRANSITIONS：操作到状态。
- RECOVERS：恢复操作到目标条件；NEXT：业务状态到后续操作。

使用 `tools/arc-verify/examples/academic-v1.json` 演示单一业务规则。
只有用户在交互终端确认后的规则才能进入必测任务。
`next` 输出短任务卡，不让 AI 把整张图放入上下文。
前置条件不满足时，运行 `recover` 获取已建模的满足路径；
任何未经确认的恢复动作只能报告 unresolved。

## 3. 实际执行（不是模拟的 PASS）

```bash
# 使用 Go 1.24+ 编译带官方 tursogo 驱动的 arc-verify 后，在业务项目根目录：
arc-verify init
arc-verify import --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
arc-verify approve --graph academic-v1
arc-verify approve --graph academic-v1 --rule R-ENROLL
arc-verify plan --graph academic-v1
arc-verify next --graph academic-v1
arc-verify recover --graph academic-v1 --condition logged_in
arc-verify start --task academic-v1/R-ENROLL/POSITIVE --env test60 --commit YOUR_TESTED_SHA
arc-verify step --run RUN_ID --expected ENROLLED --observed ENROLLED --evidence .arc/verify/raw/RUN_ID-1.json
arc-verify finish --run RUN_ID
arc-verify trace --run RUN_ID
arc-verify gate --graph academic-v1
```

`config.jsonc` 中必须明确列出授权环境 test60，CLI 不得自动转移
到生产或其他测试服务器。账号密码仅通过 `env://` / 密钥引用交给受信任
测试执行器；不得提交仓库或 RAW。

RAW 至少包括 run、step、expected、observed、preconditions、
oracle、runner、exit_code；拒绝行为还需要 before、after、denied。
CLI 对比冻结预期与实际观测：条件不满足记 BLOCKED，不变量偏离记 FAIL。
在已知且安全的实际状态下可新建诊断分支，但不修改原路径失败结论。

## 4. 重试与证据

中途失败使用 `abort --run --reason`，恢复测试环境后用
`retry --task --reason`。必须生成新 Run ID；旧 RAW 不可覆盖。
`arc-policy task verify` 对启用业务图的项目调用 `arc-verify gate`，
并在收据复用时重新验证本地数据库与 RAW 的完整性。

## 5. 明确未完成的自动化范围

这套 CLI 是图与执行证据的核心账本，**不是**浏览器自动驾驶系统。
实际运行浏览器/小程序、从实际项目源码全自动提取业务能力、
跨角色并发交错、自动将失败状态重建为已知 fixture、扩展成完整
SEQUENCE 路径及多端 Oracle，都仍需要后续实现和真实测试。
见 `docs/03-执行任务/arc-verify-graph-runtime.md`。
