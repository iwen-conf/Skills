# lzc-build.yml Specification Document

## I. Overview

`lzc-build.yml` is the configuration file used to define application build settings. This document describes its structure and the meaning of each field.

Repository hard constraints:

1. Every generated `.lpk` must be less than or equal to 12 MB. In this repository, the enforceable limit is `12,000,000` bytes.
2. Embedded images are forbidden. Do not use a top-level `images` build configuration, do not reference `embed:<alias>` in the manifest, and do not ship `images/` or `images.lock` inside the final `.lpk`.
3. If an app needs a container image, use a pullable image reference and, before store submission, sync it with `lzc-cli appstore copy-image` and backwrite the returned `registry.lazycat.cloud/...` image into the source manifest.

## II. Top-Level Data Structure `BuildConfig`

### 2.1 Basic Information {#basic-config}

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `buildscript` | `string` | Path to a build script or a shell command. |
| `manifest` | `string` | Path to the `manifest.yml` file for the lpk package. |
| `contentdir` | `string` | Directory containing content to be packaged into the lpk. |
| `pkgout` | `string` | Output path for the generated lpk package. |
| `icon` | `string` | Path to the lpk package icon. A warning is issued if not specified. Only `.png` files are allowed. |
| `devshell` | `DevshellConfig` | Development dependency configuration. |
| `compose_override` | `ComposeOverrideConfig` | Advanced Compose override configuration. **Requires lzc-os >= v1.3.0**. |

## III. Development Dependency `DevshellConfig` {#devshell}

| Field Name | Type | Description |
| ---- | ---- | ---- |
| `routes` | `[]string` | Development routing rule configuration. |
| `dependencies` | `[]string` | Development dependencies to be installed automatically. |
| `setupscript` | `string` | Script for manual installation of development dependencies. |
| `image` | `string` | Optional. Specifies a Docker image to use. |
| `pull_policy` | `string` | Optional. If set to `build`, uses the specified Dockerfile to build the image. `image` can then be set to `${package}-devshell:${version}`. |
| `build` | `string` | Optional. Path to the Dockerfile used for building the container. |

For details, see [Development Dependency Installation and Usage](https://developer.lazycat.cloud/devshell-install-and-use.md).

::: warning ⚠️ Note

If both `dependencies` and `build` are present, `dependencies` takes precedence.

:::

## IV. Advanced Compose Override `ComposeOverrideConfig` {#compose-override}

1. `compose_override` is supported by `lzc-cli@1.2.61` and above. It allows specifying Compose override configurations at build time.
2. Introduced in `lzcos v1.3.0+`, it handles runtime permission requirements not covered by the standard lpk specification.

For details, see [Advanced Compose Override](../docs/advanced-compose-override.md).

::: details Configuration Example
```yml
# Throughout the file, you can use ${var} to reference values defined in the file specified by the 'manifest' field.

# buildscript:
# - Can be a path to a script.
# - If the build command is simple, it can be a shell command.
# - ⚠️ Edge Case Warning: Never execute 'lzc project build' or similar commands within the buildscript, as it will cause an infinite loop.
buildscript: sh build.sh

# manifest: Path to the lzc-manifest.yml file.
manifest: ./lzc-manifest.yml

# contentdir: Directory containing content to be packaged into the lpk.
contentdir: ./dist

# pkgout: Output path for the lpk package.
pkgout: ./

# icon: Path to the lpk package icon. Only .png files are allowed.
icon: ./lzc-icon.png

compose_override:
  services:
    # Specify service name
    some_container:
      # Specify capabilities to drop
      cap_drop:
        - SETCAP
        - MKNOD
      # Specify volumes to mount
      volumes:
        - /data/playground:/lzcapp/run/playground:ro

# devshell: Development dependency configuration.
# In this case, alpine:latest is used as the base image. Add dependencies to the list.
# If 'dependencies' and 'build' both exist, 'dependencies' takes precedence.
devshell:
  routes:
    - /=http://127.0.0.1:5173
  dependencies:
    - nodejs
    - npm
    - python3
    - py3-pip
  setupscript: |
    export npm_config_registry=https://registry.npmmirror.com
    export PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple   
```
:::
