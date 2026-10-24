---
name: cloudflare
description: >-
  Cloudflare platform work: choose products for apps, APIs, AI agents, storage, networking and security; build,
  debug or review Workers, Durable Objects, Agents SDK, Sandbox SDK (stable, next preview, migration), Email
  Sending and Routing, Turnstile, Next.js on Workers (vinext); run Wrangler; and design, troubleshoot or migrate
  Cloudflare One Zero Trust / SASE. Not for generic web-page performance audits (use arc:verify) or non-Cloudflare
  hosting.
metadata:
  library:
    - cloudflare-platform
    - workers-best-practices
    - wrangler
    - durable-objects
    - agents-sdk
    - sandbox-sdk
    - sandbox-stable
    - sandbox-next
    - sandbox-migrate-to-next
    - cloudflare-email-service
    - turnstile-spin
    - nextjs-on-cloudflare
    - cloudflare-one
    - cloudflare-one-migrations
---

# cloudflare

One entry for Cloudflare. Pick exactly one library entry per step and follow it. Cloudflare limits, prices, type
signatures and config fields change often: before citing a number or field, fetch the current docs the entry
names (retrieval over memory) and record the source.

## Route

| Request | Entry |
|---|---|
| Which product fits a need; architecture across Workers, storage, AI, networking, security; any product without its own row | [cloudflare-platform](../../Library/cloudflare/cloudflare-platform/SKILL.md) |
| Write, review or configure production Workers | [workers-best-practices](../../Library/cloudflare/workers-best-practices/SKILL.md) |
| Wrangler commands, local dev, previews, deploys, resource management | [wrangler](../../Library/cloudflare/wrangler/SKILL.md) |
| Durable Objects state and coordination | [durable-objects](../../Library/cloudflare/durable-objects/SKILL.md) |
| Agents SDK (`agents` package) | [agents-sdk](../../Library/cloudflare/agents-sdk/SKILL.md) |
| Sandbox: overview and secure code execution | [sandbox-sdk](../../Library/cloudflare/sandbox-sdk/SKILL.md) |
| Sandbox on stable `@cloudflare/sandbox` / on `@next` / migrating stable → next | [sandbox-stable](../../Library/cloudflare/sandbox-stable/SKILL.md) / [sandbox-next](../../Library/cloudflare/sandbox-next/SKILL.md) / [sandbox-migrate-to-next](../../Library/cloudflare/sandbox-migrate-to-next/SKILL.md) |
| Email Sending or Email Routing | [cloudflare-email-service](../../Library/cloudflare/cloudflare-email-service/SKILL.md) |
| Turnstile bot verification with server-side Siteverify | [turnstile-spin](../../Library/cloudflare/turnstile-spin/SKILL.md) |
| Next.js on Workers (vinext or OpenNext) | [nextjs-on-cloudflare](../../Library/cloudflare/nextjs-on-cloudflare/SKILL.md) |
| Cloudflare One Zero Trust / SASE design or troubleshooting | [cloudflare-one](../../Library/cloudflare/cloudflare-one/SKILL.md) |
| Migrating from another VPN, SWG or SASE vendor | [cloudflare-one-migrations](../../Library/cloudflare/cloudflare-one-migrations/SKILL.md) |

Code changes still follow `arc:code` (impact, tests, gate); deploys and DNS or Zero Trust policy changes need
this task's explicit authorization and a read-back of the resulting state.

## Completion

- [ ] One entry per step; every cited limit, price or field has a current source.
- [ ] Commands and deploys report their output, or `BLOCKED` with the missing credential or permission.
