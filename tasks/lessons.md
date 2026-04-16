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
