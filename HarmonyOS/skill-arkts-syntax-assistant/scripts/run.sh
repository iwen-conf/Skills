#!/bin/bash
# 从技能目录调用，工作目录固定为显式项目根，保留项目 registry。
set -euo pipefail
: "${1:?usage: run.sh PROJECT_ROOT}"
cd "$1"
command -v ohpm >/dev/null
if [ ! -f ./hvigorw ]; then
  echo 'Missing project hvigorw wrapper' >&2
  exit 2
fi
ohpm install --all
./hvigorw assembleApp
