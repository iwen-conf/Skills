#!/usr/bin/env bash
set -euo pipefail

TOOLS="semgrep,trivy,gitleaks,gosec,govulncheck,nuclei,schemathesis"
DRY_RUN=0
WITH_ZAP=0
UPDATE_NUCLEI_TEMPLATES=0
export HOMEBREW_NO_AUTO_UPDATE="${HOMEBREW_NO_AUTO_UPDATE:-1}"
export HOMEBREW_NO_INSTALL_CLEANUP="${HOMEBREW_NO_INSTALL_CLEANUP:-1}"
export HOMEBREW_NO_INTERACTIVE="${HOMEBREW_NO_INTERACTIVE:-1}"

usage() {
  cat <<'EOF'
Usage:
  install-security-tools.sh [--core|--all] [--tools comma,list] [--with-zap] [--update-nuclei-templates] [--dry-run]

Installs local security CLI tools used by arc:security:
  semgrep, trivy, gitleaks, gosec, govulncheck, nuclei, schemathesis

ZAP is delivered as a Docker image and is pulled only with --with-zap or --all.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --core)
      TOOLS="semgrep,trivy,gitleaks,gosec,govulncheck,nuclei,schemathesis"
      ;;
    --all)
      TOOLS="semgrep,trivy,gitleaks,gosec,govulncheck,nuclei,schemathesis"
      WITH_ZAP=1
      ;;
    --tools)
      shift
      TOOLS="${1:-}"
      ;;
    --with-zap)
      WITH_ZAP=1
      ;;
    --update-nuclei-templates)
      UPDATE_NUCLEI_TEMPLATES=1
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

have() {
  command -v "$1" >/dev/null 2>&1
}

run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf '[dry-run] '
    printf '%q ' "$@"
    printf '\n'
  else
    "$@"
  fi
}

ensure_go_path() {
  if have go; then
    local gobin
    gobin="$(go env GOBIN 2>/dev/null || true)"
    if [[ -z "$gobin" ]]; then
      gobin="$(go env GOPATH 2>/dev/null)/bin"
    fi
    if [[ -n "$gobin" ]]; then
      export PATH="$gobin:$PATH"
    fi
  fi
  export PATH="$HOME/.local/bin:$PATH"
}

install_with_brew() {
  local package="$1"
  if ! have brew; then
    return 1
  fi
  run brew list "$package" >/dev/null 2>&1 || run brew install "$package"
}

install_semgrep() {
  if have semgrep; then return 0; fi
  if have brew; then
    run brew install semgrep && return 0
  fi
  if have uv; then
    run uv tool install semgrep && return 0
  fi
  echo "semgrep install failed: need brew or uv" >&2
  return 1
}

install_trivy() {
  if have trivy; then return 0; fi
  if have brew; then
    run brew install trivy || run brew install aquasecurity/trivy/trivy
    return 0
  fi
  echo "trivy install failed: need brew" >&2
  return 1
}

install_gitleaks() {
  if have gitleaks; then return 0; fi
  if have brew; then
    run brew install gitleaks && return 0
  fi
  if have go; then
    run go install github.com/zricethezav/gitleaks/v8@latest && return 0
  fi
  echo "gitleaks install failed: need brew or go" >&2
  return 1
}

install_gosec() {
  if have gosec; then return 0; fi
  if have go; then
    run go install github.com/securego/gosec/v2/cmd/gosec@latest && return 0
  fi
  echo "gosec install failed: need go" >&2
  return 1
}

install_govulncheck() {
  if have govulncheck; then return 0; fi
  if have go; then
    run go install golang.org/x/vuln/cmd/govulncheck@latest && return 0
  fi
  echo "govulncheck install failed: need go" >&2
  return 1
}

install_nuclei() {
  if have nuclei; then return 0; fi
  if have brew; then
    run brew install nuclei && return 0
  fi
  if have go; then
    run go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest && return 0
  fi
  echo "nuclei install failed: need brew or go" >&2
  return 1
}

install_schemathesis() {
  if have schemathesis; then return 0; fi
  if have uv; then
    run uv tool install schemathesis && return 0
  fi
  if have brew; then
    run brew install schemathesis && return 0
  fi
  echo "schemathesis install failed: need uv or brew" >&2
  return 1
}

pull_zap() {
  if ! have docker; then
    echo "Skipping ZAP image: docker CLI is not installed"
    return 0
  fi
  if ! docker info >/dev/null 2>&1; then
    echo "Skipping ZAP image: docker daemon is not running"
    return 0
  fi
  run docker pull ghcr.io/zaproxy/zaproxy:stable
}

ensure_go_path

IFS=',' read -r -a requested_tools <<< "$TOOLS"
for tool in "${requested_tools[@]}"; do
  tool="${tool//[[:space:]]/}"
  [[ -z "$tool" ]] && continue
  echo "==> Installing/checking $tool"
  case "$tool" in
    semgrep) install_semgrep ;;
    trivy) install_trivy ;;
    gitleaks) install_gitleaks ;;
    gosec) install_gosec ;;
    govulncheck) install_govulncheck ;;
    nuclei) install_nuclei ;;
    schemathesis) install_schemathesis ;;
    zap) WITH_ZAP=1 ;;
    *)
      echo "Unknown tool: $tool" >&2
      exit 2
      ;;
  esac
done

if [[ "$WITH_ZAP" == "1" ]]; then
  echo "==> Installing/checking zap docker image"
  pull_zap
fi

if [[ "$UPDATE_NUCLEI_TEMPLATES" == "1" ]] && have nuclei; then
  echo "==> Updating nuclei templates"
  run nuclei -update-templates
fi

echo
echo "Installed tool availability:"
for bin in semgrep trivy gitleaks gosec govulncheck nuclei schemathesis docker; do
  if have "$bin"; then
    printf '  [ok] %s -> %s\n' "$bin" "$(command -v "$bin")"
  else
    printf '  [missing] %s\n' "$bin"
  fi
done
