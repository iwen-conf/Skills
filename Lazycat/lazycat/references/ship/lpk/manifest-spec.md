# lzc-manifest.yml Specification Document

## I. Overview
`lzc-manifest.yml` is the configuration file used to define application deployment settings. This document details its structure and the meaning of each field.

**Note (LPK v2):** Static metadata such as `package`, `version` (strictly `x.x.x`), `name`, `description`, `locales`, `author`, `license`, `homepage`, `min_os_version`, and `unsupported_platforms` are now moved to `package.yml`. `lzc-manifest.yml` focuses solely on runtime execution and service configuration.

When a task says "write desktop platform support into the manifest", treat it as a static package/manifest metadata requirement. For default desktop-only store submission, write this in `package.yml`:

```yaml
unsupported_platforms:
  - ios
  - tvos
  - android
```

## II. Top-Level Data Structure `ManifestConfig`

### 2.1 Basic Execution Info {#basic-config}

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `application` | `ApplicationConfig` | lzcapp core service configuration. |
| `services` | `map[string]ServiceConfig` | Traditional Docker container service configurations. |
| `ext_config` | `ExtConfig` | Experimental attributes and system features. |
| `handlers` | `HandlersConfig` | Request handlers for OIDC and custom pages. |

---

*(Rest of the file remains the same...)*

## III. `IngressConfig` Configuration
### 3.1 Network Configuration
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `protocol` | `string` | Protocol type, supports `tcp` or `udp`. |
| `port` | `int` | Target port number. If empty, the actual inbound port is used. |
| `service` | `string` | Name of the service container. If empty, defaults to the special `app` service. |
| `description` | `string` | Service description, used by system components for administrator review. |
| `publish_port` | `string` | Allowed inbound port(s). Can be a specific port or a range like `1000~50000`. |
| `send_port_info` | `bool` | Sends the actual inbound port as a little-endian uint16 to the target port before data forwarding. |
| `yes_i_want_80_443`| `bool` | If true, allows forwarding 80/443 traffic directly to the app. Traffic bypasses the system, so authentication and wake-up won't take effect. |


## IV. `ApplicationConfig` Configuration
### 4.1 Base Configuration
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `image` | `string` | Application image. Leave empty to use the system default (alpine3.21) unless specific requirements exist. |
| `background_task` | `bool` | If `true`, the app starts automatically and won't be hibernated. Defaults to `true`. |
| `subdomain` | `string` | Inbound subdomain for the app. Used by default when opening the app. |
| `multi_instance` | `bool` | Whether to deploy in multi-instance mode. |
| `usb_accel` | `bool` | Mounts USB devices to `/dev/bus/usb` in all service containers. |
| `gpu_accel` | `bool` | Mounts DRI devices to `/dev/dri` in all service containers. |
| `kvm_accel` | `bool` | Mounts KVM and vhost-net devices to `/dev/kvm` and `/dev/vhost-net`. |
| `depends_on` | `[]string` | Dependencies on other container services within the same app. Mandatory health check type: `healthy`. Optional. |

### 4.2 Functional Configuration
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `file_handler` | `FileHandlerConfig` | Declares supported file extensions so other apps can invoke this app for specific files. |
| `entries` | `[]EntryConfig` | Entry point declarations for multiple entry names and addresses. See 4.3. |
| `routes` | `[]string` | Simplified HTTP routing rules. |
| `upstreams` | `[]UpstreamConfig` | Advanced HTTP routing rules, coexists with `routes`. |
| `public_path` | `[]string` | List of HTTP paths with independent authentication. |
| `injects` | `[]InjectConfig` | Script injection configuration for specified paths (lzcinit). |
| `workdir` | `string` | Working directory for the `app` container on startup. |
| `ingress` | `[]IngressConfig` | TCP/UDP service-related settings. |
| `environment` | `map[string]string \| []string` | Environment variables for the `app` container (map or list). |
| `health_check` | `AppHealthCheckExt` | Health check for the `app` container. Setting `disable` is only recommended during development; do not replace it otherwise, or auto-dependency logic may be lost. |
| `oidc_redirect_path` | `string` | Valid OIDC redirect path. The full domain is automatically concatenated based on the subdomain. |

Note: `routes` strips the path prefix by default during forwarding. To retain the prefix, use `upstreams` and set `disable_trim_location: true` (lzcos v1.3.9+). For internal HTTP routing, prefer concrete service targets such as `http://backend:80`. `lzc-manifest.yml` is not shell-interpolated, so `${lzcapp_appid}` and other `${...}` placeholders must not appear literally in `application.routes`.

### 4.3 Multiple Entries Configuration {#entries}

`entries` declares multiple entry points that the system can display in the launcher.

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `id` | `string` | Unique ID for the entry. |
| `title` | `string` | Entry title. |
| `path` | `string` | Entry path, usually starting with `/`. Supports query parameters. |
| `prefix_domain` | `string` | Entry domain prefix. Final domain: `<prefix>-<subdomain>.<rootdomain>`. |

Entry titles support multi-language localization via `locales` using the key `entries.<entry_id>.title`.

### 4.4 Script Injection Configuration {#injects}

`injects` is used to inject scripts into HTML pages at specific paths, ideal for minimal-intrusion adaptation of third-party apps.

Matching Rules:
- Uses `when` (whitelist) and `exclude` (blacklist).
- `when` is required; any match enters candidacy.
- `exclude` is optional; any match excludes the request.
- Final Result: `matched = whenMatched && !excludeMatched`.
- If `prefix_domain` is not empty, it only matches requests with the `<prefix>-` domain prefix.
- `mode` supports `exact` or `prefix` (default: `exact`), applied to `path/hash`.
- For `on: request` or `on: response` phases, hash routing is NOT matched.
- `auth_required` dictates if the execution requires the request to be authenticated.

Multiple `injects` entries are processed in order. The `do` actions within each entry are also executed sequentially.

#### InjectConfig
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `id` | `string` | Unique ID for the injection config. |
| `on` | `string` | Phase: `request`, `response`, or `browser` (default). |
| `auth_required` | `bool` | Default true. Set false if intercepting unauthenticated API calls (like login). |
| `prefix_domain` | `string` | Domain prefix (optional). |
| `mode` | `string` | Matching mode: `exact` or `prefix`. |
| `when` | `[]string` | Whitelist rules (at least one). |
| `exclude` | `[]string` | Blacklist rules (optional). |
| `do` | `[]any` \| `string` | List of actions or inline JS code to execute. |

#### InjectScriptConfig
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `src` | `string` | Script source: `builtin://name`, `file:///lzcapp/...`, or `http(s)://...`. |
| `params` | `map[string]any` | Parameters passed to the script. |

Rule Syntax:

Single rule format: `<path>[?<query>][#<hash>]`

Semantic Description:

- `path` is required; `query/hash` are optional.
- `query` tokens support `key` or `key=value`.
- Multiple query tokens within a single rule use AND logic.
- Query matching uses "contains" semantics (additional parameters allowed).
- `hash` is a client-side soft-match condition; the server decides injection based on `path/query`.

Example:
```yml
application:
  injects:
    - id: login-autofill
      mode: exact
      when:
        - /login
        - /signin?channel=stable
        - /#login
      exclude:
        - /api
        - /#debug
      do:
        - src: builtin://hello
          params:
            message: "hello world"
        - src: file:///lzcapp/pkg/content/custom_inject.js
          params:
            usernameField: "#user"
            passwordField: "#pass"
        - src: https://dev.example.com/inject.js
          params:
            mode: "debug"
```

Tip: `params` are injected via closure; scripts can access them directly via `__LZC_INJECT_PARAMS__` to avoid global namespace conflicts.

For runtime behavior, hashchange, built-in script parameters, and best practices, see: [Script Injection (injects)](../docs/advanced-injects.md).

## V. `HealthCheckConfig` Configuration
### 5.1 AppHealthCheckExt
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `test_url` | `string` | Only valid under the `application` field. Provides an HTTP URL for checking, avoiding dependency on `curl`/`wget` inside the container. |
| `disable` | `bool` | Disables health checks for this container. |
| `start_period` | `string` | Startup grace period. If not `healthy` after this time, the container becomes `unhealthy`. |
| `timeout` | `string` | Time after which a single check is considered failed. |


### 5.2 HealthCheckConfig

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `test` | `[]string` | Command to execute for checking, e.g., `["CMD", "curl", "-f", "http://localhost"]`. |
| `timeout` | `string` | Time after which a single check is considered failed. |
| `interval` | `string` | Interval between checks. |
| `retries` | `int` | Number of consecutive failures before the container is marked `unhealthy`. Default: 1. |
| `start_period` | `string` | Startup grace period. If not `healthy` after this time, the container becomes `unhealthy`. |
| `start_interval` | `string` | Interval between checks during the `start_period`. |
| `disable` | `bool` | Disables health checks for this container. |


## VI. `ExtConfig` Configuration {#ext_config}

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `enable_document_access` | `bool` | If true, mounts the document directory to `/lzcapp/document`. |
| `enable_media_access` | `bool` | If true, mounts the media directory to `/lzcapp/media`. |
| `enable_clientfs_access` | `bool` | If true, mounts the clientfs directory to `/lzcapp/clientfs`. |
| `disable_grpc_web_on_root` | `bool` | If true, stops hijacking grpc-web traffic. Requires a compatible lzc-sdk for proper system traffic forwarding. |
| `default_prefix_domain` | string | Adjusts the [final domain](https://developer.lazycat.cloud/advanced-secondary-domains) opened from the launcher. Any string without `.` is allowed. |
| `enable_bind_mime_globs` | `bool` | If true, binds system mime globs to `/usr/share/mime/globs2` inside the container. |



## VII. `ServiceConfig` Configuration

### 7.1 Container Configuration {#container-config}

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `image` | `string` | Docker image for the container. |
| `environment` | `map[string]string \| []string` | Environment variables (map or list). |
| `entrypoint` | `*string` | Container entrypoint (optional). |
| `command` | `*string` | Container command (optional). |
| `tmpfs` | `[]string` | Mount tmpfs volumes (optional). |
| `depends_on` | `[]string` | Dependencies on other services (excluding `app`). Mandatory health check type: `healthy`. |
| `healthcheck` | `*HealthCheckConfig` | Health check policy. Old `health_check` is deprecated. |
| `user` | `*string` | UID or username for the container (optional). |
| `cpu_shares` | `int64` | CPU shares. |
| `cpus` | `float32` | Number of CPU cores. |
| `mem_limit`| `string\|int` | Memory limit for the container. |
| `shm_size`| `string\|int` | Size of `/dev/shm/`. |
| `network_mode` | `string` | Network mode: `host` or empty. If `host`, the container uses the host network namespace. Ensure proper authentication when listening on `0.0.0.0`. |
| `netadmin` | `bool` | If true, grants `NET_ADMIN` privileges. Use with caution; do not disrupt iptables rules. |
| `setup_script` | `*string` | Configuration script executed as root before the original entrypoint. Conflicts with `entrypoint`/`command`. |
| `binds` | `[]string` | Persistent directory bindings. Only `/lzcapp/var` and `/lzcapp/cache` are preserved across restarts. Must start with `/lzcapp`. |
| `runtime` | `string` | OCI runtime: `runc` or `sysbox-runc`. `sysbox-runc` offers higher isolation for running `dockerd`/`systemd` but lacks `network_mode=host` support. |


## VIII. `FileHandlerConfig` Configuration
### 8.1 File Handling Configuration
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `mime` | `[]string` | List of supported MIME types. |
| `actions` | `map[string]string` | Action mappings. |

## IX. `HandlersConfig` Configuration

### 9.1 Handler Configuration
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `acl_handler` | `string` | ACL handler. |
| `error_page_templates` | `map[string]string` | Error page templates (optional). |


## X. `UpstreamConfig` Configuration
| Field Name | Type | Description |
| ---- | ---- | ---- |
| `location` | `string` | Matching path for the entry. |
| `disable_trim_location` | `bool` | Do not automatically strip the `location` prefix when forwarding to the backend (lzcos v1.3.9+). |
| `domain_prefix` | `string` | Matching domain prefix for the entry. |
| `backend` | `string` | Upstream address (valid URL, supports http, https, file). |
| `use_backend_host` | `bool` | If true, uses the backend host for the HTTP Host header instead of the browser's request host. |
| `backend_launch_command` | `string` | Automatically starts the program specified in this field. |
| `trim_url_suffix` | `string` | Automatically removes specified characters from the end of the URL when requesting the backend. |
| `disable_backend_ssl_verify` | `bool` | Disables SSL verification when requesting the backend. |
| `disable_auto_health_checking` | `bool` | Disables system-generated auto health checks for this entry. |
| `disable_url_raw_path` | `bool` | If true, removes the raw URL from HTTP headers. |
| `remove_this_request_headers` | `[]string` | List of HTTP request headers to remove (e.g., "Origin", "Referer"). |
| `fix_websocket_header` | `bool` | Automatically replaces `Sec-Websocket-xxx` with `Sec-WebSocket-xxx`. |
| `dump_http_headers_when_5xx` | `bool` | Dumps the request if the upstream returns a 5xx error. |
| `dump_http_headers_when_paths` | `[]string` | Dumps requests matching these paths. |



## XI. Localization Configuration {#i18n}

**Note (LPK v2):** Localization for `name`, `description`, and `usage` is now handled in `package.yml`.
Specific runtime localization (like entry titles) still remains here but follows the same BCP 47 keys.

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `entries.<entry_id>.title` | `string` | Localized entry title; `entry_id` must match `application.entries`. |
