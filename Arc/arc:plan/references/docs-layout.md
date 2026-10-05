# Docs Layout

Directory architecture, naming, README indexes and progress-tracker files under `docs/`. Status
transitions: [task-docs.md](task-docs.md) §9. File templates: [task-templates.md](task-templates.md).

## 1. Category roots

| Directory | Holds |
|---|---|
| `docs/00-产品需求/` | PRDs, user stories, business rules, domain glossaries, unresolved raw input |
| `docs/01-系统设计/` | Architecture, RFCs, sequence diagrams, DB and domain models |
| `docs/02-API记录/` | Markdown API contracts ([api-contract.md](../../_shared/api-contract.md)), module indexes, third-party integration notes; the repository contract is the authority, Apifox only an optional downstream sync on explicit request |
| `docs/03-执行任务/` | Task entries, `00-前置约束.md`, leaves, progress trackers |
| `docs/04-性能优化/` | SQL tuning, latency, memory, UI responsiveness plans |
| `docs/05-代码治理/` | Technical debt, standardization, refactoring, deprecated API removal |
| `docs/06-环境运维/` | CI/CD, Docker/K8s, DB migrations, runbooks, release and deploy notes |
| `docs/07-测试质量/` | Test plans, E2E scenarios, execution reports, coverage |
| `docs/08-安全审计/` | SAST/DAST reports, permission models, audit findings, privacy notes |
| `docs/09-线上缺陷/` | Production bugs, incident records, troubleshooting guides |
| `docs/10-复盘决策/` | ADRs, post-mortems, retrospectives |
| `docs/11-参考备忘/` | Reference docs, external links, meeting minutes |

1. New or unformatted project with a docs request → scaffold `00` through `11`, with a `README.md`
   in every active folder.
2. Existing project → keep its structure and any stricter local spec; add only missing indexes and
   required files, while keeping the 前置约束 and progress-tracker gates.
3. No new loose `.md` files at the `docs/` root.
4. A task in `03-执行任务` links back to its requirement in `00-产品需求` and its design in
   `01-系统设计` with relative links; never copy large blocks.
5. Raw input stays in `00-产品需求` until it is executable; only then does it become a task.
6. Requirements, API contracts, task trees and progress are Markdown; BE and FE leaves link the same
   API contract instead of copying it.

## 2. Task category (`03-执行任务`)

1. The root holds only `README.md`, index directories such as `00-任务索引/`, local specs, and
   numbered task directories.
2. Before creating a task, scan `README.md`, `00-任务索引/README.md`, numbered directories and loose
   legacy entries.
3. One top-level directory per business domain; when a matching domain exists, add a task group
   inside it.

Default structure:

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

- Create only the sides in scope. Add `integration/` with `T01-INT-MM` leaves only for a separate
  cross-end acceptance outcome.
- Non-UI/API work uses an accurately named package directory whose leaf IDs carry the same tag, for
  example `tooling/T01-TOOLS-01-发布闭包.md`.
- A group whose constraints differ from the task's may hold its own `00-前置约束.md`.
- Evidence the team tracks in the repository sits in the group's `evidence/` directory; raw logs
  stay under `.arc/artifacts/` per [contract](../../_shared/contract.md) §5.

## 3. Numbering and names

1. Two digits for category `DD`, task `NN`, group `TNN` and leaf `MM`. Leaf IDs are `TNN-BE-MM`,
   `TNN-FE-MM`, `TNN-INT-MM` or `TNN-<TAG>-MM`.
2. Next `NN`: scan task directories, loose legacy entries and indexes; take the highest number + 1.
3. Directory and file names are `NN-中文名称` (`01-技能发布与规则治理`, `02-Skill全量审计修订.md`).
4. No suffixes such as 最终版, 新版, 临时版; no duplicate progress trackers.

## 4. Local compatibility

1. A project using `进度跟踪.md`, `00-总进度看板.md`, or `TNN-任务组名称/` directly without `tasks/`
   keeps that structure; the hierarchy, separate BE/FE cards and difficulty/dependency fields stay.
2. Paired loose entries (`NN-具体任务名.md` plus `NN-具体任务名/`) are both updated while active.
3. Both `进度跟踪.md` and `进度跟踪表.md` present → update the one the task entry links.
4. No local convention → the default structure above.

## 5. README indexes

1. Every category root and numbered task directory has a `README.md`. A category index is an H1
   with the category name, then one line per entry in `NN` order:
   `- [01 技能发布与规则治理](01-技能发布与规则治理/README.md)`.
2. Links are relative and point at the active entry document (a directory `README.md` or the file).
3. Creating or moving a doc updates the category `README.md` and, when present,
   `00-任务索引/README.md` in the same pass.
4. A historical duplicate or loose legacy entry that stays gets a one-line reason in the index.
5. Unrelated historical entries keep their order unless reorganizing docs is the task.
6. Files are moved or renamed only when the user asks to reorganize; then every inbound link found
   by `rg -n '<old-name>' docs` is updated in the same pass.

## 6. Progress tracker files

1. One `进度跟踪表.md` per numbered task directory, covering every leaf of every group; parents are
   not rows.
2. Columns in order: 状态, 编号, 所属端, 优先级, 难度, 子任务, 上级, 依赖/就绪条件, 下一步, 完成说明.
3. Dated follow-up notes may follow the table; there is no separate completed-history table.
4. The task entry `README.md` links the tracker, and the tracker links every leaf card.

## 7. Layout repair procedure

1. Inventory: list `docs/` two levels deep; map each directory to a category root or mark it local.
2. Check: every category root and numbered task directory has `README.md`; run the link check from
   the skill's Verification section.
3. Repair: create missing READMEs listing existing entries in `NN` order; point broken relative
   links at the current target; scaffold missing categories only for a new or unformatted project
   or on request.
4. Report unmapped loose files with a proposed home; move them only after approval (rule §5.6).
