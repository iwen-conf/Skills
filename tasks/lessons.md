# Lessons

## 2026-03-15

- When the user says `arc` or names an `arc:*` capability in this environment, first map it to the local skill set under `/Users/iluwen/Documents/Code/Skills/Arc/` before concluding that a standalone CLI/runtime is required.
- When the user pushes back on Python-heavy orchestration for local long-running services, prefer a one-shot Go or shell controller that exits immediately after updating `tmux` and the project-local registry.
- When a short-lived Go controller will be invoked repeatedly, add a tiny cached launcher so later runs can reuse the compiled binary instead of paying `go run` startup cost every time.
- When migrating Arc engineering helpers off Python, prefer a thin `sh` wrapper that preserves the existing script path while moving parsing, state, and reporting logic into a cached Go binary.

## 2026-03-25

- For this Skills repository, treat "no GitHub Actions" as a hard repository policy, not a soft preference. If the user has already rejected Actions, add a validator/test guard so `.github/workflows/` cannot come back silently.

## 2026-04-10

- When a user says a repository skill "must call" another skill, update the repository source of truth under `/Users/iluwen/Documents/Code/Skills/` first, not only the installed copy under `/Users/iluwen/.cc-switch/skills/`.
- For skill migrations that change output format or tool contract, update all three layers together: `SKILL.md`, bundled references, and scaffold/generator scripts. Do not leave the source skill on the old delivery path.

## 2026-04-15

- When users report draw.io sequence or deployment diagrams with misaligned connectors, do not treat it as a prompt wording problem alone. Harden both the generation contract and the validator so bad geometry is structurally blocked.
- For sequence diagrams in this repository, keep raw draw.io XML arrows off the default path. Prefer `Mermaid` source, and only allow draw.io as a host for embedded Mermaid when the user explicitly requires it.
- For deployment diagrams, semantic rules are not enough. Always specify node alignment, connection binding (`source/target`), and routing style, then add automated checks for floating edges and manual endpoint coordinates.
- If a diagram type repeatedly fails because the model is hand-authoring geometry, move that geometry into a deterministic script. Validation alone catches bad output, but generation scripts prevent it.
- When users still want a `.drawio` deliverable for sequence diagrams, do not fall back to raw XML arrows. Wrap rendered Mermaid output into draw.io and keep the Mermaid source embedded so validation can still inspect the real sequence semantics.
- When a user asks for repeated checking, do not merely rerun the same validator. Add a review loop that compares source files, regenerated artifacts, and workspace outputs, otherwise drift can survive despite green single-file validation.

## 2026-11-04

- 本仓 ARC Verify 语境中的“业务树图 / db 那种树图”指持久化到目标项目 `.arc/verify/data.db` 的 Graph v2 业务图及其静态工作台。解释唤醒方式时，应明确建模入库、复用既有图和 `snapshot` 导出；不要改答成独立 Mermaid 图，也不要加入“不初始化验证账本”来禁止用户要的流程。候选规则仍不得自动批准，图的存在不代表业务验证通过。

## 2026-11-17

- 修复的正式测试放在所属代码包或项目测试目录；`.arc/artifacts/` 保存日志、报告和被动源码快照，不作为持续修改和执行的测试主副本。由 `arc:code` 的 `gate.sh` 检查直接测试入口、工作目录和自动发现，不能仅靠文字约定。
- 用户指出产物目录里的测试变更时，先核对实际文件、执行命令与业务入口。一个 import 的增删不能证明绕过业务，也不能证明已修复；当前不可访问的文件明确保留证据缺口。
