---
name: lazycat
description: >-
  Lazycat (懒猫微服) app lifecycle: find migration candidates and gate them on Developer Center and App Store
  duplicates and commercial licenses (迁移候选, 商店查重, GitHub许可证, 可商用); judge non-invasive migration
  feasibility, write scope and workload (迁移可行性, 不改业务代码, 工作量评估); create or align an original app
  baseline (原创应用, 从零开发, 登录模型, 文件能力); ship a ready app with copy-image, LPK build, install test,
  Developer Center materials, submission and post-release checks (上架发布, 打包, copy-image, 提审); and update
  installed apps from an LPK Inspector source (已安装应用更新, LPK Inspector, manifest回写, 重新打包).
---

# lazycat

One skill for every Lazycat app stage. Each stage is a mode with its own allowed and forbidden actions; a mode
never performs another mode's actions without passing that mode's gate.

## Modes

| Condition | Mode | Read |
|---|---|---|
| Find or check third-party candidates: discovery, unlisted inventory, store duplicates, read-only clone, license | intake | [intake.md](references/intake.md) |
| A licensed candidate: runtime mapping, LPK/image boundary, allowed write scope, workload, proceed / POC / stop | boundary | [boundary.md](references/boundary.md) |
| An original or user-owned project needs a Lazycat baseline | original | [original.md](references/original.md) |
| A ready app goes to the store: metadata, copy-image, LPK, install test, materials, submit, verify release | ship | [ship.md](references/ship.md) |
| Installed apps need new versions from an LPK Inspector source | update | [update.md](references/update.md) |

Gate order is fixed: third-party apps go intake → boundary (→ POC when boundary says so) → ship; original apps
go original → ship; update is independent. Never ship a third-party app whose intake license decision or
boundary decision is missing; report which gate is missing instead.

## Credentials

Read credentials only from the current process environment; report missing names, never values.

| Target | Variables |
|---|---|
| Lazycat device | `LAZYCAT_USERNAME` / `LAZYCAT_PASSWORD` |
| App Store | `LAZYCAT_APPSTORE_USERNAME` / `LAZYCAT_APPSTORE_PASSWORD` |
| Developer Center | `LAZYCAT_DEVELOPMENT_USERNAME` / `LAZYCAT_DEVELOPMENT_PASSWORD` |

1. Never write values into repository files, reports, screenshots, commit messages or memory.
2. Check presence with `test -n "${LAZYCAT_PASSWORD:-}"`; never `echo` or `printenv` a secret.
3. Browser logins type the value directly; logs and replies name only the variable, site and result.
4. Reuse an existing CLI login for `copy-image`; never put passwords on a command line or in shell history.
5. Credentials unlock authentication only; submit, publish, install-over and remote writes still need this
   task's authorization.

## Completion

- [ ] Mode chosen; its preconditions, allowed and forbidden lists followed.
- [ ] The mode's output format delivered with real identifiers (repo URL, license, LPK path and size, image
      address, submission or release state) or `blocked` with the missing input.
- [ ] Next mode named, or the stop decision recorded with its reason.
