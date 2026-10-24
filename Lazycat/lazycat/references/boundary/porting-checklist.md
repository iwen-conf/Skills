# Lazycat Porting Checklist

## 1. Before Selection
- [ ] GitHub search completed.
- [ ] License allows porting and commercial use (non-commercial projects prohibited).
- [ ] Upstream URL recorded.

## 2. Project Execution
- [ ] Before editing, declared the allowed write scope for this port and limited changes to packaging/runtime wrapper files.
- [ ] **Cloned upstream onto a dedicated porting branch (e.g., `lazycat/port`), not upstream's default branch**. All porting changes live on this branch so the upstream default branch stays clean and future `git merge upstream/main` remains possible. Recorded the branch name and base commit.
- [ ] Created `docs/requirements`.
- [ ] Created `docs/api-design`.
- [ ] Created `docs/architecture`.
- [ ] Created `docs/release-prep`.
- [ ] Created `build.sh`.
- [ ] Created or completed `Makefile` in the repo after migration, not just as a proposed template.
- [ ] `make build` implemented.
- [ ] `make install` implemented.
- [ ] No upstream business source files were modified for the port. Only Lazycat packaging/runtime wrapper files were changed, unless the user explicitly approved business-code changes in the current task and named the feature/file scope.
- [ ] If startup, login, healthcheck, schema, routing, or review requirements seemed to require a business-code change, the agent stopped and reported the blocker instead of silently editing upstream source.
- [ ] Final diff was checked against the boundary: allowed files are `package.yml`, `lzc-build.yml`, `lzc-manifest.yml`, `lzc-deploy-params.yml`, `Makefile`, `build.sh`, Docker wrapper files, `runtime/`, setup/seed scripts, config templates, docs, icons, and store assets.
- [ ] Runtime UI language reviewed. If Chinese UI is important for the target users, document whether the upstream app already supports it or whether i18n work is intentionally deferred. Do not modify upstream business source just to add Chinese UI during ordinary porting.

## 3. Lazycat Adaptation
- [ ] Runtime model preflight completed before writing manifest: delivery form, entry path, persistence, dependency layers, initialization, and login path.
- [ ] Runability decision recorded as one of: `Can Run`, `Can Run After Packaging Fixes`, `Cannot Determine Yet`, `Not Suitable For Standard Lazycat Port`.
- [ ] Image/architecture evidence recorded: pullable image, build path, `linux/amd64` compatibility, or explicit advanced-runtime requirement.
- [ ] Long-running process model verified: entrypoint/command starts a foreground service rather than only a one-shot CLI or manual shell task.
- [ ] Actual listener protocol and port verified from upstream docs, Compose, Dockerfile, config, or runtime logs before writing `routes`/`upstreams`/`ingress`.
- [ ] Delivery path is justified: official image, Compose conversion, Dockerfile-built image, wrapper image, or source-only fallback.
- [ ] HTTP/TCP entry decision is explicit: `routes`, `upstreams`, `secondary_domains`, or `ingress`.
- [ ] Persistent paths are mapped only to Lazycat-approved writable locations such as `/lzcapp/var` or `/lzcapp/cache`; runtime-writable configs are not left under `/lzcapp/pkg/content`.
- [ ] Service dependency graph is documented as infra -> middleware -> seed/migration -> business.
- [ ] External requirements classified: none, user-configurable, advanced capability, or blocking dependency.
- [ ] Prepared `lzc-build.yml`.
- [ ] Prepared `lzc-manifest.yml`.
- [ ] Prepared `package.yml` with `unsupported_platforms` declaring `ios`, `tvos`, and `android` unless those platforms were explicitly requested and verified.
- [ ] Planned Developer Center platform selection as desktop-only unless the user explicitly requested mobile or TV support and verification evidence exists.
- [ ] Prepared `package.yml.locales` with BCP 47 keys when localized metadata is needed, including usage text when login credentials need explanation.
- [ ] If the app uses bridged images, the final pullable image refs have already been written back to `lzc-manifest.yml` before `make install`.
- [ ] Evaluated OIDC requirement.
- [ ] Evaluated `file_handler` requirement.
- [ ] Clarified credential acquisition path for apps requiring login.
- [ ] Distinguished credential scopes: `LAZYCAT_USERNAME` / `LAZYCAT_PASSWORD` for Lazycat OS, `LAZYCAT_APPSTORE_USERNAME` / `LAZYCAT_APPSTORE_PASSWORD` for the App Store, and `LAZYCAT_DEVELOPMENT_USERNAME` / `LAZYCAT_DEVELOPMENT_PASSWORD` for the Developer Center. In-app login uses app-level variables.
- [ ] Application MUST support passwordless auto-login (免密登录) via OIDC or Inject (`builtin://simple-inject-password`). Verification is landing on the first authenticated page, including auto-submit.
- [ ] For apps with an internal login page, a fixed initial account is created non-invasively at startup when supported by upstream CLI/CMD/env/admin API.
- [ ] Initial passwordless-login credentials are drafted for Developer Center `应用描述` and `版本更新说明` (`账号`, `密码`, `昵称`), not only `package.yml` usage.
- [ ] Modifiable passwordless login follows the official three-phase inject pattern: request -> response -> browser, with `ctx.flow`, `ctx.persist`, and `builtin://simple-inject-password`.
- [ ] Login/init/change-password API paths, payload keys, and CSS selectors were verified from docs/runtime traffic or explicitly provided by the user; none were guessed.
- [ ] Startup order gates seed/login initialization behind app and infra healthchecks, and gates dependent business services on `condition: service_healthy` where applicable.
- [ ] Ingress/application healthchecks were tested as end-to-end symptoms, not treated as the root cause. If ingress reports a service is not ready, target service logs and dependency health were inspected.
- [ ] Database/cache first-boot readiness was validated with real probes and realistic `start_period`, especially PostgreSQL/MySQL cases that temporarily reject connections.
- [ ] One-shot migration/seed services use bounded retry for transient dependency failures, write a terminal success marker only after success, and are idempotent on reinstall/restart.
- [ ] Required deploy parameters were installed through the setup wizard and rendered config/env values were inspected inside the container. Missing config or `need setup deploy params` is resolved before route/healthcheck tuning.
- [ ] Skipping the deploy-param wizard still leaves the main flow usable, or params without which the app 500/404 are required.
- [ ] Open/save/import/export uses the Lazycat file picker with 懒猫网盘; Drive context-menu alone is not the file-flow proof.
- [ ] `compose_override` is explained for listing and flagged for official/dev-group notice.
## 4. Workload and Listing Handoff
- [ ] Original/Porting path clarified.
- [ ] Prepared upstream attribution (for ports). If `package.yml.homepage` is a GitHub repository URL, set `author` to the URL owner segment exactly, preserving case and symbols (for example, `https://github.com/Sliverkiss/mimocode2api` requires `author: Sliverkiss`); mismatches are review-blocking.
- [ ] Listing icon matches the upstream official icon.
- [ ] Planned 2-5 pairwise-unique desktop 16:9 screenshots of distinct main-flow steps; desktop layout, not a stretched mobile UI.
- [ ] Planned Chinese listing copy in Chinese and English listing copy in English when the runtime UI is English.
- [ ] Planned OIDC or `file_handler` if applicable.
- [ ] Planned to verify core capabilities by installing and opening the app within Lazycat OS, including persistence after restart.
- [ ] Middleware/SDK-only shape without an in-app main flow is recorded as `Listing risk: middleware`.
