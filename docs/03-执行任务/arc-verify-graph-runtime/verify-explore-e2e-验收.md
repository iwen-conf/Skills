# verify-explore-e2e 最终验收

日期：2026-10-31  
基线：`78666eaad8df5e838ff6e5006dfc20746c11679d`（origin/main）  
任务：`verify-explore-e2e` · skill=`arc-code` · mode=`implement+repair`  
CLI：`tools/arc-verify`（Go，短生命周期）  
模型：course-v2 / seat-v2（工具图）；探索规划器是有界启发式，不是会话内 LLM。  
环境：本机 Darwin，独立进程 `127.0.0.1:0`；IM-mongo `lan50`/`tx` 均为 `approved: false`。

运行证据（gitignored）：`.arc/artifacts/verify-explore-e2e/`。

## 门禁

| 命令 | 结果 |
|---|---|
| `go -C tools/arc-verify test -race -count=1 .` | PASS（145s；292 tests） |
| `go -C tools/arc-verify vet .` | PASS |
| `go -C tools/arc-verify build` | PASS |
| `.venv/bin/python -m pytest -q` | 476 passed, 1 skipped |
| DBqual pytest | 160 passed, 1 skipped, 139 subtests |
| `scripts/validate_skills.py` | 65 skill files |
| workbench `vp check` / `vp test` / `vp build` | 19 tests PASS |
| `file://` 浏览器 | 六视图可点；能力节点显示探索账本；网络仅本地 file 资源 |
| `arc-policy task verify` | PASS |

`gate.sh check --require-red`：Go 测试/格式/lint PASS；前端 tsc/vitest 因 gate 走 npm 与 bun `devEngines` 冲突 FAIL（已知）。同一工作台已用 `vp` 复验通过。

未跑：GitHub Actions（本仓禁令）。IM-mongo 真实目标：BLOCKED。

## A. 自主探索

起点不是人工预写完整路径。`discover run` 从已批准图、适配器入口和推断构造候选，按风险/未覆盖/成本选择，经 HTTP Runner 执行，独立 Oracle 读回，写 RAW，再走既有 DISCOVERY。

盲测（探索器源码不含密封缺陷 ID；解封后比对）：

| ID | 类别 | 结果 | 依据 |
|---|---|---|---|
| B-STATE | 非法转移 | 检出 | `R-CLAIM` FAIL（期望 pending，观察到 idle / skip-review） |
| B-AUTH | 未授权 | 检出 | `R-FORCE` / `R-NO-FORCE` FAIL |
| B-IDEM | 幂等 | 检出 | 重复 `request_id` 与 records 观察 |
| B-RACE | 超额占用 | 检出 | `invariant.capacity` FAIL |
| B-VALID | 关课后仍接受 | 漏检 | 适配器没有关课动作，该边界不可达 |

UNSPECIFIED：`UNDECLARED_TRANSITION`（force/claim）。  
RISK：推断候选 `repeat`，未升 RULE。  
消耗：12 动作，停止原因 `budget_actions`。

这不是「已知五种缺陷 → 预写五种测试 → 全部检出」。

## B. 真实 E2E

独立于 `isolated_seat_test.go` httptest 的进程：`tools/arc-verify/testdata/enroll-sut`（文件持久化、角色、容量、独立 `/oracle`）。复用 seat-v2 合同，不另建平行模型。

| 场景 | 判定 |
|---|---|
| 正常 claim | PASS，Oracle 读回 pending |
| 学生 force | 服务端拒绝，义务 PASS |
| 学生 claim → 管理员 approve | PASS |
| 重复 request_id | PASS |
| 末席并发 | PASS（进程内屏障，可复现） |
| hang 故障 | BLOCKED，不是产品 PASS |
| reset 失败 | BLOCKED，锁保留 |
| skip-review 后修复再测 | 新 PASS；历史 FAIL 仍可 handoff |

IM-mongo：`approved: false`。未改共享系统、未部署、未重启。**不标记 IM-mongo E2E 完成。**

## 七问

1. **现在能不能独立探索没有预写完整用例的系统？**  
   能，但有界。Agent 能在已授权适配器入口上自己选动作、真执行、按观察更新 Frontier、登记 UNSPECIFIED。它不是对任意未知系统的开放式 LLM 巡航；候选来自确认 RULE、实现入口和启发式推断。

2. **实际自主发现了哪些未知行为？哪些是缺陷，哪些只是 RISK？**  
   盲测里四条是已确认规则被违背（FAIL）。`UNDECLARED_TRANSITION` 保持 UNSPECIFIED。推断 `repeat` 是 RISK。没有把实现结果反写进 RULE。

3. **哪些缺陷没被发现，为什么？**  
   B-VALID：探索器没有关课动作，关课后仍接受报名的路径不可达。未声明、适配器未列出的入口也不会被发明出来。

4. **独立真实业务系统的 E2E 是否通过？**  
   `enroll-sut` 独立进程链通过。IM-mongo 未授权，BLOCKED。seat-v2 httptest 只作工具回归，不是本项唯一依据。

5. **跨角色、并发、故障恢复是否有可复核证据？**  
   有。enroll 与 seat 每步有 RUN/STEP/RAW/Oracle。并发在可注入屏障的夹具/进程上可确定复现；对不可控内部同步点的外部系统，不得宣称确定性复现，只能做有界压力并区分业务竞争与调度失败。

6. **哪些能力只在内置夹具里成立？**  
   五类缺陷开关、occupancy barrier、关课/未授权 force 等注入。盲测的被测面是带开关的隔离服务，不是第三方生产系统。`enroll-sut` 独立但合同与 seat-v2 相同。

7. **是否已经满足最初的业务边界自动探索目标？**  
   部分满足：已确认规则可被探索违背并留下独立证据；契约外行为可登记且不升 RULE；未知空间会写停止原因和剩余候选。未满足：对未授权外部产品做完整 E2E；在没有适配器入口时发现全新业务面；独立可信 Runner / 受保护 CI。

## 仍只在工具层成立的边界

- 探索规划器是启发式，不是会话 LLM。
- 状态等价仍要求影响未来行为的历史属性；未证明则不合并。
- 工作台只读快照，不执行、不批准。
- 节点不因探索次数或信心分数变绿。
