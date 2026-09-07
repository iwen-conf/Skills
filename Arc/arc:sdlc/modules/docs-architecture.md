# Docs Architecture

Load this file when scaffolding `docs/`, choosing category roots, or creating numbered task directories.

## SDLC Directory Roots

For new projects, or when upgrading a project's documentation architecture, enforce this partitioning under `docs/`. Every document belongs to a logical phase.

- `docs/00-产品需求/`: PRDs, user stories, business rules, and domain glossaries.
- `docs/01-系统设计/`: Architecture, RFCs, sequence diagrams, and DB/domain models.
- `docs/02-API记录/`: Markdown API contracts, module indexes, and third-party integration notes. Repository documents are the authority for implementation and acceptance; follow [`../../arc:build/references/api-contract.md`](../../arc:build/references/api-contract.md). Apifox is an optional downstream sync only when explicitly requested, never a required source or workflow step.
- `docs/03-执行任务/`: Feature/bugfix task plans, execution subtasks, progress tables, and `00-前置约束.md`.
- `docs/04-性能优化/`: SQL tuning, latency, memory, UI responsiveness plans.
- `docs/05-代码治理/`: Technical debt, standardization, refactoring, deprecated API removal.
- `docs/06-环境运维/`: CI/CD, Docker/K8s, DB migrations, runbooks. Release/deploy notes live here; Arc has no separate release skill.
- `docs/07-测试质量/`: Test plans, E2E scenarios, execution reports, coverage metrics.
- `docs/08-安全审计/`: SAST/DAST reports, permission models, audit findings, privacy notes.
- `docs/09-线上缺陷/`: Production bugs, incident records, troubleshooting guides.
- `docs/10-复盘决策/`: ADRs, post-mortems, retrospectives.
- `docs/11-参考备忘/`: Reference docs, external links, meeting minutes.

Rules:

1. When asked to create docs for a new or unformatted project, scaffold `00` through `11` (including `README.md` for active folders).
2. Do not dump loose markdown files at the root of `docs/`.
3. A task in `03-执行任务` MUST link back to the source requirement in `00-产品需求` and the architecture in `01-系统设计`. Use relative links; do not copy-paste large blocks.
4. Use `README.md` at the root of each category as the index.
5. Author requirements, API contracts, task decomposition, and progress as Markdown, not JSON documents. Use relative links from BE/FE tasks to the same API contract rather than copying a specification into each task.

## Task Category Rules (`03-执行任务`)

1. Keep the task category root for `README.md`, index directories such as `00-任务索引/`, local specs, and numbered task directories.
2. For new work, do not add loose top-level topic `.md` files. Create or reuse a numbered task directory with a `README.md` entry.
3. Scan `README.md`, `00-任务索引/README.md`, and existing numbered task directories before creating a new top-level task.
4. Keep one top-level task directory per business domain. If a matching domain exists, add a task group inside it.

## Requirement Category Rules (`00-产品需求`)

1. Store unresolved raw input and product/data notes in `docs/00-产品需求`.
2. Promote requirements into `docs/03-执行任务` only after they are executable.

## Default Task Structure

```text
docs/03-执行任务/
  README.md
  00-任务索引/
    README.md
  NN-具体任务名/
    README.md
    00-前置约束.md
    进度跟踪表.md
    tasks/
      T01-任务组名称/
        README.md
        backend/
          README.md
          T01-BE-01-后端行为.md
        frontend/
          README.md
          T01-FE-01-前端行为.md
```

The hierarchy is business group (`T01`) → side work package (`T01-BE` / `T01-FE`) → executable leaf (`T01-BE-01` / `T01-FE-01`). Create only the sides in scope. Add `integration/` with `T01-INT-MM` tasks only when a separate cross-end acceptance outcome is needed. Difficulty determines how leaves are split; see [`authoring.md`](authoring.md).

Local compatibility:

1. If the project spec uses `进度跟踪.md`, `00-总进度看板.md`, or direct `TNN-任务组名称/` directories without `tasks/`, follow that existing structure while preserving the hierarchy, separate frontend/backend task files, and difficulty/dependency fields.
2. If the project already uses paired loose entries such as `NN-具体任务名.md` plus `NN-具体任务名/`, update both when they are active.
3. If both `进度跟踪.md` and `进度跟踪表.md` exist, update the one referenced by the task entry.
4. If no local convention exists, use the default structure above.

Rules:

- Use two digits for `DD`, task `NN`, group `TNN`, and subtask `MM`; new leaf IDs are `TNN-BE-MM` / `TNN-FE-MM` (or `TNN-INT-MM` for integration verification).
- Pick the next available `NN` by scanning existing task directories, loose legacy entries, and indexes.
- Do not use suffixes such as "最终版", "新版", "临时版", or duplicate progress tables.
- If the repository already has a stricter local task-document spec, obey the local spec while keeping pre-constraint and progress-tracking gates.
