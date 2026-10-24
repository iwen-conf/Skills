# Local skill library

Third-party skill sources bundled into router packages by `scripts/sync_skills.py`. Each source is copied into
its router's `library/<name>/` with `SKILL.md` renamed to `ENTRY.md`, so hosts never discover it as a skill.
The subdirectories are git-ignored: several upstreams publish no license, so their content is not
redistributed from this repository. On a fresh clone, restore them from the backup or upstream before
building `ui-design`, `cloudflare` or `threejs`.

| Directory | Router | Sources (imported 2026-10-08 from local installs) | License |
|---|---|---|---|
| `design/` | `Design/ui-design` | taste-skill (design-taste-frontend), gpt-tasteskill, soft-skill, minimalist-skill, brutalist-skill, redesign-skill, stitch-skill, impeccable, better-ui, brandkit, image-to-code-skill, imagegen-frontend-web, imagegen-frontend-mobile | none published locally; local use only |
| `cloudflare/` | `Cloudflare/cloudflare` | cloudflare (renamed `cloudflare-platform`), cloudflare-one, cloudflare-one-migrations, cloudflare-email-service, durable-objects, wrangler, workers-best-practices, agents-sdk, sandbox-sdk, sandbox-next, sandbox-stable, sandbox-migrate-to-next, turnstile-spin, nextjs-on-cloudflare (newest copy per skill; `~/.agents` 2026-09, sandbox-sdk from `~/.codex`) | per upstream (Cloudflare skills) |
| `threejs/` | `ThreeJS/threejs` | 24 `threejs-*` skills (OpenAEC Foundation) and r3f-best-practices | MIT (frontmatter) |

Local modifications: `cloudflare-platform` frontmatter name only. Router-level overrides live in each router's
`SKILL.md`, never in these sources.
