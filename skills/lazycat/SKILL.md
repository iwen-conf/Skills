---
name: lazycat
description: Lazycat Microserver (懒猫微服) app intake, non-invasive migration, original app baseline, `copy-image`, LPK build (`<=12MB`), store release, and LPK Inspector updates. Triggers on 懒猫微服, Lazycat, LPK打包, lzc-manifest, copy-image, 上架发布.
---

# lazycat

Follow fixed stage gates (`intake -> boundary -> ship` for ports; `original -> ship` for new apps; `update` for LPK Inspector upgrades) and load `references/cheatsheet.md` before editing packaging files.

## Guardrails

1. **Zero Unauthorized Business Edits**: Port third-party apps strictly via `package.yml`, `lzc-manifest.yml`, `lzc-build.yml`, and wrapper entrypoints after license & Store dedup pass.
2. **LPK & Image Hard Gate**: Final `.lpk <= 12,000,000` bytes; ban `embed:<alias>` and bundled `images/`; sync pullable images via `lzc-cli appstore copy-image` into `registry.lazycat.cloud/...`.
3. **Store & Runtime Proof**: Declare `unsupported_platforms: [ios, tvos, android]` on desktop-default builds, land login flows authenticated, wire Lazycat Drive picker for file I/O, and never write `*.heiyu.space` domains or credentials into repo files.
