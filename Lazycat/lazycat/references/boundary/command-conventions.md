# Lazycat 命令入口约定

所有 Lazycat 项目都必须在仓库根目录提供统一入口，确保构建、验证、镜像同步和提审流程可重复执行。

## 1. 核心文件

- `build.sh`：整理会进入 `lpk` 的运行时脚本、静态资源和辅助产物。
- `Makefile`：编排镜像构建、推送、`copy-image`、打包、安装和提审前检查。

## 1.1 远程镜像桥接项目的职责边界

当项目采用“本地构建镜像 -> Docker Hub -> `lzc-cli appstore copy-image` -> 回填 manifest”这条路线时，职责必须固定：

- `build.sh`：不要构建应用主镜像，只处理 `runtime/`、静态资源和 `lpk` 交付内容。
- `Makefile`：负责 `docker-build`、`docker-push`、`copy-image`、`update`、`build`、`install`，但职责必须分层：`copy-image` / manifest 回填属于迁移或更新阶段；`build` / `install` 只面向已经准备好的镜像引用和 `lpk` 交付物，不负责源码级构建。
- `lzc-manifest.yml`：正式发布时只保留 `registry.lazycat.cloud/...` 镜像地址。
- `lpk`：只保留 manifest、runtime、图标、静态资源；包体必须 `<= 12,000,000` bytes，且不得内嵌应用主镜像或任何 `images/`、`images.lock` 产物。

## 2. 标准 Makefile 模板

```makefile
.RECIPEPREFIX := >

LPK_FILE ?= app.lpk
LPK_MAX_BYTES ?= 12000000
IMAGE_REPO ?= your-hub-user/app-name
IMAGE_TAG ?= v1.0.0
PUBLIC_IMAGE := $(IMAGE_REPO):$(IMAGE_TAG)
COPIED_IMAGE_FILE := .lazycat-image-ref

.PHONY: all doctor docker-login-check docker-build docker-push copy-image check-lpk build install update backend-test ui-test ui-build ui-e2e capture-screenshots verify release-prep

all: install

doctor:
> @echo "Checking development environment..."
> @lzc-cli --version >/dev/null || (echo "Error: lzc-cli not installed" && exit 1)
> @docker --version >/dev/null || (echo "Error: Docker not installed" && exit 1)
> @echo "Environment ready"

docker-login-check:
> @test -f "$$HOME/.docker/config.json" || (echo "Error: ~/.docker/config.json not found" && exit 1)
> @grep -q 'index.docker.io' "$$HOME/.docker/config.json" || (echo "Error: Docker Hub login entry not found" && exit 1)
> @echo "Docker Hub credential entry detected"

docker-build:
> docker buildx build --platform linux/amd64 -t $(PUBLIC_IMAGE) --load .

docker-push: docker-login-check docker-build
> docker push $(PUBLIC_IMAGE)

copy-image: docker-push
> @TMP_FILE=$$(mktemp); \
> lzc-cli appstore copy-image $(PUBLIC_IMAGE) | tee $$TMP_FILE; \
> NEW_IMAGE=$$(grep -Eo 'registry\.lazycat\.cloud[^[:space:]]+' $$TMP_FILE | tail -n1); \
> test -n "$$NEW_IMAGE" || (echo "Error: failed to parse Lazycat registry image" && rm -f $$TMP_FILE && exit 1); \
> printf '%s\n' "$$NEW_IMAGE" > $(COPIED_IMAGE_FILE); \
> rm -f $$TMP_FILE; \
> echo "Lazycat image saved to $(COPIED_IMAGE_FILE)"

backend-test:
> @echo "Running backend tests..."
> # Example: cd backend && go test ./...

ui-test:
> @echo "Running frontend unit tests..."
> # Example: cd ui && npm test

ui-build:
> @echo "Building frontend..."
> # Example: cd ui && npm run build

ui-e2e:
> @echo "Running frontend E2E tests..."
> # Example: cd ui && ./node_modules/.bin/playwright install chromium && npm run test:e2e

capture-screenshots:
> @echo "Generating submission screenshots..."
> # Example: cd ui && ./node_modules/.bin/playwright install chromium && npm run capture:screenshots

verify: backend-test ui-test ui-build ui-e2e
> @echo "All verifications passed"

release-prep: verify capture-screenshots
> @echo "Submission assets generated"

check-lpk:
> @test -f "$(LPK_FILE)" || (echo "Error: LPK not found: $(LPK_FILE)" && exit 1)
> @SIZE=$$(wc -c < "$(LPK_FILE)" | tr -d ' '); \
> test "$$SIZE" -le "$(LPK_MAX_BYTES)" || (echo "Error: $(LPK_FILE) is $$SIZE bytes, exceeds $(LPK_MAX_BYTES)" && exit 1); \
> echo "LPK size OK: $$SIZE bytes"
> @TMP_FILE=$$(mktemp); \
> if tar -tf "$(LPK_FILE)" > $$TMP_FILE 2>/dev/null; then :; \
> elif unzip -Z1 "$(LPK_FILE)" > $$TMP_FILE 2>/dev/null; then :; \
> else echo "Error: cannot list LPK archive contents" && rm -f $$TMP_FILE && exit 1; fi; \
> ! grep -Eq '(^|/)images/|(^|/)images.lock$$' $$TMP_FILE || (echo "Error: embedded image artifacts found in $(LPK_FILE)" && rm -f $$TMP_FILE && exit 1); \
> rm -f $$TMP_FILE; \
> echo "LPK embed-image check OK"

build:
> @echo "Packaging current image-level release inputs into LPK..."
> lzc-cli project build -o $(LPK_FILE)
> @$(MAKE) check-lpk

install: build
> @echo "Installing current LPK into Lazycat MicroServer..."
> lzc-cli app install $(LPK_FILE)
> @echo "LPK build and install complete"

update: copy-image
> @echo "Executing update workflow..."
> @NEW_IMAGE=$$(cat $(COPIED_IMAGE_FILE)); \
> perl -0pi -e 's#(^\s*image:\s*).*$#$$1'"$$NEW_IMAGE"'#m' lzc-manifest.yml
> lzc-cli project build -o $(LPK_FILE)
> @$(MAKE) check-lpk
```

## 2.1 远程镜像桥接的 `build.sh` 模板

```sh
#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
DIST_DIR="$ROOT_DIR/dist"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# 只复制会随 LPK 一起交付的内容；应用主镜像通过 Docker Hub + copy-image 管理
if [ -d "$ROOT_DIR/runtime" ]; then
  mkdir -p "$DIST_DIR/runtime"
  cp -R "$ROOT_DIR/runtime/." "$DIST_DIR/runtime/"
fi

if [ -d "$ROOT_DIR/static" ]; then
  mkdir -p "$DIST_DIR/static"
  cp -R "$ROOT_DIR/static/." "$DIST_DIR/static/"
fi
```

## 3. 核心目标说明

- `make build`：只负责把当前已经准备好的 `package.yml`、`lzc-build.yml`、`lzc-manifest.yml`、runtime/静态资源等交付物打成 `lpk`；不要在这里做源码编译、业务镜像构建、`docker push` 或 `copy-image`。构建后必须检查包体 `<= 12,000,000` bytes，且没有 `images/` 或 `images.lock`。
- `make install`：通过依赖 `make build` 先生成一次当前 `lpk`，再执行 `lzc-cli app install $(LPK_FILE)` 安装到当前 Lazycat 环境；提审前必须真实执行，且不要在这个目标里偷偷做 `docker push`、`copy-image` 或 `lzc-manifest.yml` 回填。
- `make update`：适用于镜像升级或同步上游，负责 `copy-image`、回填 `lzc-manifest.yml`、重新打包。
- `make release-prep`：提审前的最终步骤，包含测试、截图和交付证据整理。
- `make verify`：无副作用校验入口，适合 CI/CD。
- `make docker-build` / `make docker-push` / `make copy-image`：自定义镜像的标准桥接入口。

## 4. 镜像移植约定

对 `lzc-manifest.yml` 中引用外部镜像的项目：

1. 如果镜像由本地构建，先为 `linux/amd64` 构建并推送到 Docker Hub。
2. 使用 `lzc-cli appstore copy-image` 同步到 Lazycat 官方镜像仓。
3. 将返回的 `registry.lazycat.cloud/...` 地址写回 `lzc-manifest.yml`。
4. `make update` 应自动化或半自动完成上述流程。
5. 在进入 `make build` / `make install` 之前，manifest 中的镜像地址必须已经是最终可拉取地址；不要把这一步延期到安装阶段。
6. 不得使用 `lzc-build.yml.images` 或 `embed:<alias>` 绕过远程镜像桥接；最终 `.lpk` 必须保持在 12 MB 上限内。

如果需要一次同步多个镜像，优先使用原生 CLI 包装器，不要依赖 `npx` 常驻进程。仓库内提供了一个 Go 参考实现：

- `references/lzc-copy-image-go/`
- 支持 JSON 数组输入
- 支持固定并发
- 每个任务单独超时
- 输出稳定 JSON，便于后续脚本回填 `lzc-manifest.yml`
