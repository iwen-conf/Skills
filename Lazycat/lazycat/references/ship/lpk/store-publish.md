# Lazycat Micro-service App Store Submission and Publishing Specification

## I. Submission Process

### 0. LPK Package Hard Gate
Every final `.lpk` submitted or installed through this workflow must pass both checks:

1. File size is less than or equal to 12 MB. Use `12,000,000` bytes as the exact limit.
2. No embedded image artifacts exist. The source project must not use `lzc-build.yml.images` or `embed:<alias>`, and the final `.lpk` must not contain `images/` or `images.lock`.

If either check fails, stop publishing and switch to package slimming or remote image bridging before continuing.

### 0.1 Platform and Screenshot Gate
Unless the user explicitly requests another platform and provides real verification evidence, submit the app as desktop-only in the Developer Center.

For LPK v2, platform support metadata belongs in `package.yml`. If a user or older document calls this "manifest platform support", map that requirement to the package/manifest static metadata and make sure the final package records the same unsupported platforms.

Default desktop-only submission metadata:

```yaml
unsupported_platforms:
  - ios
  - tvos
  - android
```

Use 2-5 pairwise-unique 16:9 screenshots for Developer Center submission. By default, screenshots must come from the real desktop runtime, each a different main-flow step. Only prepare mobile or TV screenshots when the user explicitly requested those platforms and the app was verified there. See section 0.4.

### 0.2 Developer Center English Description Gate
If the app runtime UI or primary user-facing copy is English, the Developer Center listing page must include a complete English description before submission.

This is a Developer Center store-listing field requirement, not a manifest/package metadata field. Do not treat `lzc-manifest.yml`, `package.yml.description`, or `package.yml.locales` as sufficient by themselves.

### 0.3 Installation and Business Capability Gate
Before submitting or publishing, install the final `.lpk` on a real Lazycat MicroServer and verify that installation succeeds, the app opens, startup and login work, and the core business capabilities behave normally.

Core business verification must cover the app's main user-facing workflows, such as create/read/update/delete operations, file operations, persistence, or the equivalent domain-specific actions. If installation fails, the app cannot open, startup/login fails, or any core business capability is unverified or abnormal, stop submission and fix the package first.

### 0.4 Reviewer Independent Completion Gate
A store reviewer installs the submitted `.lpk` and uses only Developer Center listing fields. They must complete the main user flow without contacting the developer. This section is the source of truth for listing-rejection classes; `lazycat:ship-app` SKILL.md restates the operational checks.

**Login.** If the app has a login page, OIDC or inject must land on the first authenticated page. Inject must fill the fields and submit; fill-only is a reject. If inject uses a fixed initial account, write that username and password in Developer Center `应用描述` and `版本更新说明` for every listing language. `package.yml.locales.usage` is supplementary. Listing may include in-app initial credentials or a public test account. Listing must omit Lazycat OS / App Store / Developer Center credentials, tokens, internal addresses, and `*.heiyu.space` domains. If ordinary users cannot obtain credentials from the listing or from in-app init, the app cannot be listed.

**Listing language.** 简体中文 fields must be Chinese. 英语 fields must be English. English runtime UI requires complete English `应用简介`, `应用描述`, and `版本更新说明` on the listing page. Manifest/package locales do not replace those fields.

**Screenshots and icon.** Provide 2–5 desktop 16:9 screenshots from the real installed app. File hashes must be pairwise unique; each shot is a different step of the main flow. Desktop layout is required. For ports, the listing icon must match the upstream official icon.

**LPK.** The review record must attach the actual final `.lpk`. A missing package is a reject.

**Deploy params.** `lzc-deploy-params.yml` skips the install wizard when every param is optional. A param without which the main flow cannot complete must be required, or must have a default that actually makes the app usable. Empty optional API keys or secrets that later 500/404 are a reject.

**File flows.** Open, save, import, export, or upload must use the Lazycat file picker and work with 懒猫网盘. Drive context-menu / `file_handler` alone is insufficient when the in-app picker still only sees the local computer.

**Persistence.** App data and settings survive restart. Browser-local SPAs must survive a new browser session.

**Privileged runtime.** `lzc-build.yml.compose_override` must be explained in the listing, and the developer must notify official support or the developer group before submit. See [advanced-compose-override.md](../docs/advanced-compose-override.md).

**Product shape.** Libraries, SDKs, and middleware without a complete in-app user workflow are generally rejected. A protocol, queue, or database with a dashboard is acceptable only when the reviewer can finish a primary in-app workflow from the UI.

**Desktop layout.** Runtime UI and screenshots must be desktop-adapted. A stretched mobile layout is a reject.

### 1. Developer Registration
1. Register an account on the [Lazycat Community](https://lazycat.cloud/login?redirect=https://developer.lazycat.cloud/).
2. Visit the [Developer Center](https://developer.lazycat.cloud/manage) and follow the prompts to submit a developer review application.
3. After submission, it is recommended to contact support or use the official [Contact Us](https://lazycat.cloud/about?navtype=AfterSalesService) channels to expedite review.

### 2. Push Images to the Official Registry
Before submitting to the store, you **must** push all images referenced in the lpk to the official Lazycat registry. Otherwise, reviewers will be unable to install the app, leading to **review failure**.

```bash
lzc-cli appstore copy-image <Publicly_accessible_image_name>
# After completion, the command prints registry.lazycat.cloud/<community-username>/<image-name>:<hash-version>
```

Example:
```bash
lzc-cli appstore copy-image alpine:3.18
# Output: registry.lazycat.cloud/snyh1010/library/alpine:d3b83042301e01a4
```

**Official Image Registry `registry.lazycat.cloud` Usage Restrictions:**
1. For stability, the generated image tag is replaced by the IMAGE_ID. Each `copy-image` execution triggers a mandatory `docker pull` on the server side.
2. Uploaded images **must be publicly accessible**. The `pull` operation occurs on the server; local-only images cannot be processed by `copy-image`.
3. Uploaded images must be referenced by at least one store application; the registry performs periodic garbage collection.
4. `registry.lazycat.cloud` is intended for use within Micro-services only. Using it outside of the Micro-service environment will result in **rate limiting**.

#### Recommended Path for Self-Built Images
If you have modified the upstream image locally, or the upstream only ships a `Dockerfile`, use this bridge workflow:

```bash
docker buildx build --platform linux/amd64 -t your-hub-user/app-name:v1.0 --load .
docker push your-hub-user/app-name:v1.0
lzc-cli appstore copy-image your-hub-user/app-name:v1.0
```

Rules for this path:
1. `copy-image` must receive a registry address that the server can pull. A local-only image tag is invalid.
2. Prefer Docker Hub for the public handoff unless the user explicitly provides another public registry.
3. After `copy-image` succeeds, backwrite `services.<name>.image` in the source `lzc-manifest.yml` with the returned `registry.lazycat.cloud/...` value. If the repo packages from manifest templates or staged manifests, backwrite those source files too.
4. The `lpk` should only retain manifests, runtime scripts, icons, and static assets, and must stay within `12,000,000` bytes. Do **not** embed the application image itself into the package.
5. If the user wants the full release closure, prefer a dedicated release target such as `release-build` / `release-install` that runs build image -> push public image -> `copy-image` -> backwrite source manifest -> build `.lpk` -> install `.lpk`.

After uploading, you **must manually update the image address in the source `lzc-manifest.yml`** to the official `registry.lazycat.cloud/...` address returned before packaging or publishing.

### 3. Submit for Review
Use `lzc-cli` (v1.2.54 or above) to submit:
```bash
lzc-cli project build
lzc-cli lpk info ./your-app.lpk
lzc-cli appstore publish ./your-app.lpk
```

Before publishing, every required Developer Center field must be completed. Do not submit with empty fields, placeholder text, or "to be filled later" notes. The submission record must include LPK information from the actual final `.lpk` package, such as the summary returned by `lzc-cli lpk info ./your-app.lpk`.

Before publishing, confirm the Developer Center platform selection. Default to desktop-only, confirm `package.yml.unsupported_platforms` declares `ios`, `tvos`, and `android`, and attach 2-5 pairwise-unique real desktop 16:9 screenshots unless the user explicitly requested and verified additional platforms.

Before publishing, pass section 0.4. Re-read the Developer Center `应用描述`, `版本更新说明`, English listing fields, screenshots, icon, and attached `.lpk` after filling them, and confirm they match the installed app.

Before publishing an app whose runtime UI or primary user-facing copy is English, confirm the Developer Center listing page has a complete English description. This cannot be satisfied only by manifest/package metadata.

Before publishing, confirm the final `.lpk` has been installed successfully on a real Lazycat MicroServer and that the app opens, login/startup works, and core business workflows behave normally. Do not publish when installation or business capability verification is missing or failed.

For migrated projects, do not select the Developer Center checkbox "应用程序为本人原创开发或本人是源作者". The submission must include the original author's name and the source project or code address, using evidence from the migration license gate or upstream repository. Do not use placeholders or guessed values.

Also record the final `.lpk` byte size and the no-embedded-image check result with the submission evidence.

## II. App Store Review Guidelines (7 Red Line Rules)

Before submitting, ensure all the following conditions are met:

### 1. Completeness of App Information
- `package.yml` must be complete with `package`, `version` (strictly `x.x.x` format), `name`, `description`, `author`, and `license`.
- Unless explicitly requested and verified otherwise, Developer Center platform selection must be desktop-only and `package.yml.unsupported_platforms` must include `ios`, `tvos`, and `android`.
- App Icon and 2-5 pairwise-unique real runtime screenshots must be provided in the Developer Center. By default, use desktop 16:9 screenshots only. Each shot is a different step of the main flow. Ported apps use the upstream official icon.
- All Developer Center app information fields must be completed before submission; the final `.lpk` must be attached to the review. Record `lzc-cli lpk info` from that same package.
- 简体中文 listing fields must be Chinese; 英语 listing fields must be English.
- English apps must include a complete English description on the Developer Center listing page. Manifest/package metadata does not replace this field.
- Migrated projects must leave the originality/source-author checkbox unselected and provide the original author name plus source project or code URL.
- If `package.yml.locales` is provided, language key specifications follow the [BCP 47 standard](https://en.wikipedia.org/wiki/IETF_language_tag). Prefer complete name, description, and usage text for the app's primary audience.
- Runtime Chinese (`zh-CN`) UI is optional. Add or improve runtime i18n when it serves the product and target users, but do not block submission solely because the app UI is English-only.

### 2. Installability and Loadability
- The app must install and load normally.
- Apps that fail to install, fail to load after installation, or become unresponsive will **not pass review**.
- Thoroughly test the installation process and initial loading before submission. **Specifically, verify that dependencies required for installation are accessible.**
- The app's core business capabilities must work after installation. Missing or failed business-flow verification blocks submission.

### 3. Stability and Quality
- Avoid serious crashes or unexpected shutdowns.

### 4. Performance Metrics
- App startup speed and response time **must not exceed 5 minutes**.

### 5. Special Scenario Adaptation
- **Hardware-paired apps**: Must be tested in a real hardware environment; include test notes with hardware model information.
- **Special scenario apps** (e.g., browsers): Must be fully tested in their respective scenarios.
- **Update Prompts**: In-app update prompts should not severely disrupt normal use. If the app cannot complete in-app updates, consider removing the prompt.

### 6. Validity of Use Case
- The submitted app must provide a **genuine and valid use case** for the user.
- **Development libraries and middleware are generally not allowed**, unless the reviewer can finish a primary in-app workflow from the UI. See section 0.4 Product shape.
- Utility apps should be associated with relevant file types in the Lazycat Drive. Open/save/import/export must use the Lazycat file picker with 懒猫网盘.

### 7. Data Persistence
- For apps requiring persistent data, you must verify that data is correctly preserved.
- **Ensure no data loss after app restarts or upgrades**.
- When upgrading an existing store app, avoid changing the instance definition unless necessary (as this changes the storage path). If changes are required, handle data migration and recovery properly.

### 8. Passwordless Auto-Login
- Applications MUST support passwordless auto-login (免密登录). Verification is landing on the first authenticated page, not merely filling the form. See section 0.4 Login.
- **Implementation Methods:**
  1. **OIDC Standard Flow:** Integrate with Lazycat MicroServer's identity authentication system to achieve automatic user identification based on ingress injection.
  2. **Inject Autofill:** Use `builtin://simple-inject-password` or custom inject scripts to automatically fill **and submit** login credentials.
- For implementation details, refer to the "Store Submission Guide" and [Advanced Inject Passwordless Login](https://developer.lazycat.cloud/advanced-inject-passwordless-login.html) in the Developer Documentation.

## III. Prohibited App Types

- Apps involving illegal content (e.g., pornography, gambling, drugs), airdrops, cracked software, or anything violating Chinese laws **are prohibited**.
- For apps requiring a username and password, if ordinary users cannot obtain credentials through the Lazycat Store listing or in-app init, the app **cannot be listed**. Write in-app initial credentials in Developer Center `应用描述` and `版本更新说明`.
