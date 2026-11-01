> 历史执行记录：本报告中的外部模型配置/凭证阻塞已不再是现行工作要求；保留作为历史证据。当前不再开发 Agent/ACP 交接或模型 HTTP 适配。见 [当前状态与需求](当前状态与需求.md)。

# ARC Verify · 第五轮可信执行与边界发现验收

任务：`verify-round5-trusted-exec-boundary`。基线 `3fa8a73` / 上次审计 `0fa6477`。

## FIX-01 恢复不变量

只要已经发出可能有副作用的请求，在无法证明可安全重放之前不得再发。

- OBSERVED 且 RAW 缺失：保留 Attempt ID，标记 `replay_forbidden` / `evidence_incomplete`，写入 Tried，移出 Frontier，Campaign `UnsafeResume`，fixture 锁不释放。
- Oracle 已证明完成但缺 RAW：进入人工复核，不伪造 RAW，不升 PASS。
- 真实进程：`after_observed_before_raw` 窗口 SIGKILL 后，第二进程 resume 被拒，业务副作用次数仍为 1。

## FIX-02 / FIX-03 证据合同

`VerifiedEvidenceResult` 是工作台与 `arc-verify gate`（亦即 arc-policy `business_graph` 调用的同一 argv）的只读结果。

业务判定与证据可信分开。旧记录无 `explore-identity-v1` 合同时为 `LEGACY_UNVERIFIED`，不自动升级，也不把 academic-v1 历史图一律打成 FAIL。

## BD 机制

仍在现有 ExplorePlanner / Campaign / Graph v2 / 独立 Oracle 上：

1. 先读事实再提问。
2. 反事实假设用角色/动作/资源/状态/不变量去重。
3. 同 HTTP 不同业务状态必须有独立 Oracle。
4. 最小实验；无法区分则 INCONCLUSIVE。
5. 假设版本追加；不 APPROVE RULE。
6. 第二次 AI 规划读取第一次观察。审计脱敏，失败则不执行 AI 动作。

## Agent 规划器

**已删除。** 不要再用 `--mode agent`、`discover context` / `discover propose` 或 `context.json`/`proposal.json` 文件握手。现有编程 Agent 阅读 Skill 与 Markdown 进度，自行分析业务；CLI 只执行 heuristic/deterministic（以及脚本夹具 `--mode ai --planner FILE`）并负责 Oracle、RAW 与崩溃恢复。

## LIVE

- 测试主机隔离仍锁 `192.168.0.60`，禁止回退 `.50`。
- IM-mongo 正式 E2E：BLOCKED（60 上无授权隔离写服务）。
- 真实模型 HTTP：非现行要求。CLI 无模型客户端、无 API Key；不得再当作产品缺口。

## 实际命令

| 命令 | 退出码 |
| --- | --- |
| `go -C tools/arc-verify test -race -count=1 ./...` | 0（207s） |
| `go -C tools/arc-verify vet ./...` | 0 |
| `go -C tools/arc-verify build` | 0 |
| `.venv/bin/python -m pytest -q` | 0（477 passed, 1 skipped） |
| `.venv/bin/python -m pytest -q Arc/arc:verify/scripts/dbqual/tests` | 0（160 passed, 1 skipped） |
| `.venv/bin/python scripts/validate_skills.py` | 0（65 skill files） |
| `bunx tsc -b` + `vp test`（workbench） | 0（19 passed） |
| `vp build`（workbench） | 0 |
| `gate.sh check --strict` | 1：`lint(ts)`/`tests(js)` 走 npx 触发已知 `EBADDEVENGINES`；Go/Python/scope PASS。替代：`bunx tsc -b`、`vp test` |

## 未完成

- 独立可信 Runner 与受保护 CI（W16）。
- IM-mongo 生产链 A/B/C（无 `.60` 隔离写授权，BLOCKED）。
- 外部模型多轮探索已从产品范围删除，不再列为未完成项。
