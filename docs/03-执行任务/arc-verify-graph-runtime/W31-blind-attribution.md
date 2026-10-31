# W31 盲测归因

状态：`[x]`

## T2 简报（W31–W35 共用）

- 目标：未预写完整用例时，探索器能提出有界候选、用独立 Oracle 判定、中断后安全续探；盲测检出必须绑定真实故障与证据。
- 范围：`tools/arc-verify`（探索运行时、盲测评估器、CLI、schema、工作台 DTO/UI）、本目录 W31–W35。
- 非目标：第七个 Arc Skill、常驻服务、重做 v2 图、GitHub Actions、共享 28885/28895/生产写。
- 验收：`go -C tools/arc-verify test -race -count=1 ./...`；`go vet`；Python / DBqual / 技能校验；工作台 check/test；中断后续探；归因不得使用报告字符串匹配。

## 本卡

独立评估器解封故障清单；未激活不计漏检；一条通用 FAIL 不能记多个缺陷。
