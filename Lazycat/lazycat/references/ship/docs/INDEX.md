# Lazycat Developer Docs Index

When a user asks about any of the following topics or URLs, read the corresponding local file to get the full context.

Repository override:

1. These files include upstream documentation snapshots. If they mention LPK v2 embedded images, `lzc-build.yml.images`, `embed:<alias>`, `images/`, or `images.lock`, treat those as upstream capability descriptions only.
2. This skill repository forbids embedded images. All generated `.lpk` files must be less than or equal to `12,000,000` bytes and must not contain embedded image artifacts.
3. For image-based apps, use a pullable remote image and `lzc-cli appstore copy-image`, then write the returned `registry.lazycat.cloud/...` image into the source manifest before packaging.

| URL | Local File |
|---|---|
| `https://developer.lazycat.cloud/getting-started/hello-world-fast.md` | [getting-started_hello-world-fast.md](../docs/getting-started_hello-world-fast.md) |
| `https://developer.lazycat.cloud/advanced-api-auth-token.md` | [advanced-api-auth-token.md](../docs/advanced-api-auth-token.md) |
| `https://developer.lazycat.cloud/advanced-compose-override.md` | [advanced-compose-override.md](../docs/advanced-compose-override.md) |
| `https://developer.lazycat.cloud/deploy-params.md` | [deploy-params.md](../docs/deploy-params.md) |
| `https://developer.lazycat.cloud/spec/deploy-params.md` | [spec_deploy-params.md](../docs/spec_deploy-params.md) |
| `https://developer.lazycat.cloud/network-diagnostic/dnsbypass.md` | [network-diagnostic_dnsbypass.md](../docs/network-diagnostic_dnsbypass.md) |
| `https://developer.lazycat.cloud/network-diagnostic/dns.md` | [network-diagnostic_dns.md](../docs/network-diagnostic_dns.md) |
| `https://developer.lazycat.cloud/dockerd-support.md` | [dockerd-support.md](../docs/dockerd-support.md) |
| `https://developer.lazycat.cloud/advanced-gpu.md` | [advanced-gpu.md](../docs/advanced-gpu.md) |
| `https://developer.lazycat.cloud/hello-world.md` | [hello-world.md](../docs/hello-world.md) |
| `https://developer.lazycat.cloud/http-request-headers.md` | [http-request-headers.md](../docs/http-request-headers.md) |
| `https://developer.lazycat.cloud/advanced-inject-request-dev-cookbook.md` | [advanced-inject-request-dev-cookbook.md](../docs/advanced-inject-request-dev-cookbook.md) |
| `https://developer.lazycat.cloud/spec/inject-ctx.md` | [spec_inject-ctx.md](../docs/spec_inject-ctx.md) |
| `https://developer.lazycat.cloud/network-diagnostic/ipv4routingbypass.md` | [network-diagnostic_ipv4routingbypass.md](../docs/network-diagnostic_ipv4routingbypass.md) |
| `https://developer.lazycat.cloud/network-diagnostic/ipv4-routing.md` | [network-diagnostic_ipv4-routing.md](../docs/network-diagnostic_ipv4-routing.md) |
| `https://developer.lazycat.cloud/network-diagnostic/ipv6-connectivity.md` | [network-diagnostic_ipv6-connectivity.md](../docs/network-diagnostic_ipv6-connectivity.md) |
| `https://developer.lazycat.cloud/network-diagnostic/ipv6routingbypass.md` | [network-diagnostic_ipv6routingbypass.md](../docs/network-diagnostic_ipv6routingbypass.md) |
| `https://developer.lazycat.cloud/network-diagnostic/ipv6-routing.md` | [network-diagnostic_ipv6-routing.md](../docs/network-diagnostic_ipv6-routing.md) |
| `https://developer.lazycat.cloud/kvm.md` | [kvm.md](../docs/kvm.md) |
| `https://developer.lazycat.cloud/advanced-lightos.md` | [advanced-lightos.md](../docs/advanced-lightos.md) |
| `https://developer.lazycat.cloud/lpk-format.md` | [lpk-format.md](../docs/lpk-format.md) |
| `https://developer.lazycat.cloud/getting-started/lpk-how-it-works.md` | [getting-started_lpk-how-it-works.md](../docs/getting-started_lpk-how-it-works.md) |
| `https://developer.lazycat.cloud/spec/lpk-format.md` | [spec_lpk-format.md](../docs/spec_lpk-format.md) |
| `https://developer.lazycat.cloud/build.md` | [build.md](../docs/build.md) |
| `https://developer.lazycat.cloud/spec/build.md` | [spec_build.md](../docs/spec_build.md) |
| `https://developer.lazycat.cloud/spec/manifest.md` | [spec_manifest.md](../docs/spec_manifest.md) |
| `https://developer.lazycat.cloud/introduction.md` | [introduction.md](../docs/introduction.md) |
| `https://developer.lazycat.cloud/advanced-oidc.md` | [advanced-oidc.md](../docs/advanced-oidc.md) |
| `https://developer.lazycat.cloud/advanced-manifest-render.md` | [advanced-manifest-render.md](../docs/advanced-manifest-render.md) |
| `https://developer.lazycat.cloud/network-diagnostic/nat-type.md` | [network-diagnostic_nat-type.md](../docs/network-diagnostic_nat-type.md) |
| `https://developer.lazycat.cloud/spec/package.md` | [spec_package.md](../docs/spec_package.md) |
| `https://developer.lazycat.cloud/network-diagnostic/lzcrelaysbypass.md` | [network-diagnostic_lzcrelaysbypass.md](../docs/network-diagnostic_lzcrelaysbypass.md) |
| `https://developer.lazycat.cloud/advanced-setupscript.md` | [advanced-setupscript.md](../docs/advanced-setupscript.md) |
| `https://developer.lazycat.cloud/network-diagnostic/stuntcpbypass.md` | [network-diagnostic_stuntcpbypass.md](../docs/network-diagnostic_stuntcpbypass.md) |
| `https://developer.lazycat.cloud/advanced-l4forward.md` | [advanced-l4forward.md](../docs/advanced-l4forward.md) |
| `https://developer.lazycat.cloud/api/golang.md` | [api_golang.md](../docs/api_golang.md) |
| `https://developer.lazycat.cloud/api/javascript.md` | [api_javascript.md](../docs/api_javascript.md) |
| `https://developer.lazycat.cloud/network-diagnostic/windows-registry.md` | [network-diagnostic_windows-registry.md](../docs/network-diagnostic_windows-registry.md) |
| `https://developer.lazycat.cloud/advanced-secondary-domains.md` | [advanced-secondary-domains.md](../docs/advanced-secondary-domains.md) |
| `https://developer.lazycat.cloud/network-diagnostic/originbypass.md` | [network-diagnostic_originbypass.md](../docs/network-diagnostic_originbypass.md) |
| `https://developer.lazycat.cloud/advanced-inject-passwordless-login.md` | [advanced-inject-passwordless-login.md](../docs/advanced-inject-passwordless-login.md) |
| `https://developer.lazycat.cloud/getting-started.md` | [getting-started.md](../docs/getting-started.md) |
| `https://developer.lazycat.cloud/network-diagnostic/public-servers.md` | [network-diagnostic_public-servers.md](../docs/network-diagnostic_public-servers.md) |
| `https://developer.lazycat.cloud/publish-app.md` | [publish-app.md](../docs/publish-app.md) |
| `https://developer.lazycat.cloud/advanced-entries.md` | [advanced-entries.md](../docs/advanced-entries.md) |
| `https://developer.lazycat.cloud/advanced-multi-instance.md` | [advanced-multi-instance.md](../docs/advanced-multi-instance.md) |
| `https://developer.lazycat.cloud/lzc-cli-wsl.md` | [lzc-cli-wsl.md](../docs/lzc-cli-wsl.md) |
| `https://developer.lazycat.cloud/faq-startup_script.md` | [faq-startup_script.md](../docs/faq-startup_script.md) |
| `https://developer.lazycat.cloud/network-config.md` | [network-config.md](../docs/network-config.md) |
| `https://developer.lazycat.cloud/extensions.md` | [extensions.md](../docs/extensions.md) |
| `https://developer.lazycat.cloud/advanced-frontend-app-dev.md` | [advanced-frontend-app-dev.md](../docs/advanced-frontend-app-dev.md) |
| `https://developer.lazycat.cloud/advanced-platform.md` | [advanced-platform.md](../docs/advanced-platform.md) |
| `https://developer.lazycat.cloud/store-submission-guide.md` | [store-submission-guide.md](../docs/store-submission-guide.md) |
| `https://developer.lazycat.cloud/advanced-mime.md` | [advanced-mime.md](../docs/advanced-mime.md) |
| `https://developer.lazycat.cloud/app-block.md` | [app-block.md](../docs/app-block.md) |
| `https://developer.lazycat.cloud/advanced-app-interconnect.md` | [advanced-app-interconnect.md](../docs/advanced-app-interconnect.md) |
| `https://developer.lazycat.cloud/faq-dev.md` | [faq-dev.md](../docs/faq-dev.md) |
| `https://developer.lazycat.cloud/getting-started/dev-workflow.md` | [getting-started_dev-workflow.md](../docs/getting-started_dev-workflow.md) |
| `https://developer.lazycat.cloud/lzc-cli.md` | [lzc-cli.md](../docs/lzc-cli.md) |
| `https://developer.lazycat.cloud/third-apps.md` | [third-apps.md](../docs/third-apps.md) |
| `https://developer.lazycat.cloud/advanced-envs.md` | [advanced-envs.md](../docs/advanced-envs.md) |
| `https://developer.lazycat.cloud/getting-started/env-setup.md` | [getting-started_env-setup.md](../docs/getting-started_env-setup.md) |
| `https://developer.lazycat.cloud/developer-cyber-discount.md` | [developer-cyber-discount.md](../docs/developer-cyber-discount.md) |
| `https://developer.lazycat.cloud/ssh.md` | [ssh.md](../docs/ssh.md) |
| `https://developer.lazycat.cloud/wangjishanren-lazycat-developer-startup.md` | [wangjishanren-lazycat-developer-startup.md](../docs/wangjishanren-lazycat-developer-startup.md) |
| `https://developer.lazycat.cloud/framework.md` | [framework.md](../docs/framework.md) |
| `https://developer.lazycat.cloud/advanced-db.md` | [advanced-db.md](../docs/advanced-db.md) |
| `https://developer.lazycat.cloud/advanced-file.md` | [advanced-file.md](../docs/advanced-file.md) |
| `https://developer.lazycat.cloud/getting-started/http-route-backend.md` | [getting-started_http-route-backend.md](../docs/getting-started_http-route-backend.md) |
| `https://developer.lazycat.cloud/advanced-depends.md` | [advanced-depends.md](../docs/advanced-depends.md) |
| `https://developer.lazycat.cloud/advanced-browser-extension.md` | [advanced-browser-extension.md](../docs/advanced-browser-extension.md) |
| `https://developer.lazycat.cloud/advanced-public-api.md` | [advanced-public-api.md](../docs/advanced-public-api.md) |
| `https://developer.lazycat.cloud/app-example-porting.md` | [app-example-porting.md](../docs/app-example-porting.md) |
| `https://developer.lazycat.cloud/app-example-go.md` | [app-example-go.md](../docs/app-example-go.md) |
| `https://developer.lazycat.cloud/app-vnc.md` | [app-vnc.md](../docs/app-vnc.md) |
| `https://developer.lazycat.cloud/changelog.md` | [changelog.md](../docs/changelog.md) |
| `https://developer.lazycat.cloud/network.md` | [network.md](../docs/network.md) |
| `https://developer.lazycat.cloud/advanced-injects.md` | [advanced-injects.md](../docs/advanced-injects.md) |
| `https://developer.lazycat.cloud/network-pass-through.md` | [network-pass-through.md](../docs/network-pass-through.md) |
| `https://developer.lazycat.cloud/network-diagnostic/relay-issues.md` | [network-diagnostic_relay-issues.md](../docs/network-diagnostic_relay-issues.md) |
| `https://developer.lazycat.cloud/network-diagnostic/origin-issues.md` | [network-diagnostic_origin-issues.md](../docs/network-diagnostic_origin-issues.md) |
| `https://developer.lazycat.cloud/contributors.md` | [contributors.md](../docs/contributors.md) |
| `https://developer.lazycat.cloud/advanced-route.md` | [advanced-route.md](../docs/advanced-route.md) |
| `https://developer.lazycat.cloud/advanced-user-deploy-guide.md` | [advanced-user-deploy-guide.md](../docs/advanced-user-deploy-guide.md) |
| `https://developer.lazycat.cloud/advanced-error-template.md` | [advanced-error-template.md](../docs/advanced-error-template.md) |
| `https://developer.lazycat.cloud/getting-started/advanced-vnc-embed-image.md` | [getting-started_advanced-vnc-embed-image.md](../docs/getting-started_advanced-vnc-embed-image.md) |
