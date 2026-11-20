---
name: arc-plan
description: Explore code & impact (`arc idx`), vet dependencies (`github_health.py`), freeze API/SQL contracts, check state machines (`check_closure.py`), and plan execution waves (`arc-wave`). Triggers on 代码在哪, 搜代码, 谁调用, 调用链, 影响面, 架构地图, 开源选型, 技术选型, 引入依赖, 写PRD, 需求澄清, 接口契约, 拆任务, 业务闭环, 状态机检查.
---

# arc-plan

Understand code, vet external libraries, freeze contracts, and register wave tasks. Mutate zero application or test source files.

## Protocol

1. **Explore & Impact (`arc idx`)**: Run `arc idx doctor --format json` (`arc idx index --quiet` if stale). Stop at the shallowest rung: symbol lookup -> 3-hop caller/callee trace -> 8-surface impact list, citing exact `file:line`.
2. **Dependency Vet (When Adding Libraries)**: Check stdlib and repo helpers first. If external code is needed, run `python3 scripts/github_health.py <owner/repo>...` on `>= 3` candidates; reject archived or `>= 36`-month stale packages and record license, CVEs, version pin, and exit cost.
3. **Clarify & Freeze Contracts (Wave 0)**: Ask at most 3 questions only when ambiguity alters schema, public API, or authz. Freeze OpenAPI / Protobuf / SQL contracts (`../_shared/api-contract.md`) before coding tasks.
4. **Derive Waves & Closure Gate**: Compute disjoint `write_scope` waves from code anchors via `arc-wave plan` + `arc task create` (`../_shared/handoff.md`). For financial or complex state lifecycles, validate models with `python3 scripts/check_closure.py gate <spec>`.
