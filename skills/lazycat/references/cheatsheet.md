# Lazycat Microserver (懒猫微服) Hard-Spec Cheatsheet

## 1. Stage Gate Order & Privacy

- **Third-Party Port**: `intake` (Store & Dev Center dedup + commercial license `Pass`) -> `boundary` (non-invasive wrapper feasibility & workload tier) -> `ship`.
- **Original App**: `original` -> `ship`. **Installed App Upgrade**: `update` (read-only LPK Inspector via `agent-browser` + user confirmation before overwriting install).
- Read `LAZYCAT_*` credentials strictly from process env; never write real `*.heiyu.space` domains, passwords, tokens, or cookies into repository files or commit logs.

## 2. LPK & Image Hard Limits

- **Archive Ceiling**: Final `.lpk <= 12,000,000` bytes; **strictly ban** `lzc-build.yml.images`, `embed:<alias>`, bundled `images/`, or `images.lock`.
- **Image Sync**: Run `lzc-cli appstore copy-image <pullable-image>` and write the returned `registry.lazycat.cloud/...` URI/digest back to `lzc-manifest.yml` (never rely on bare mutable `latest`). Verify package metadata via `lzc-cli lpk info <file.lpk>`.

## 3. `package.yml` & Store Acceptance Rules

- Required fields: `package`, `version` (strict `x.x.x`), `name`, `description`, `author` (must match GitHub URL owner char-for-char when `homepage` is on GitHub), `license`, `locales`.
- **Desktop Default**: Unless mobile/TV is explicitly requested and verified on device, declare:
  `unsupported_platforms: [ios, tvos, android]`
- **Auth & File Integration**: Apps with login pages must enter an authenticated session via OIDC or auto-filled+submitted `inject` (record fixed initial test credentials in `locales.usage` / Dev Center). Apps with open/save/import/export must integrate Lazycat Drive file picker.
