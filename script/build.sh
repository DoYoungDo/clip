#!/bin/bash

set -euo pipefail

# 全平台构建脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "开始全平台编译..."

# 依次执行各平台构建脚本，便于定位失败步骤。
"$SCRIPT_DIR/build-mac-arm64.sh"
"$SCRIPT_DIR/build-mac-amd64.sh"
"$SCRIPT_DIR/build-linux-amd64.sh"
"$SCRIPT_DIR/build-linux-arm64.sh"
"$SCRIPT_DIR/build-win-amd64.sh"
"$SCRIPT_DIR/build-win-arm64.sh"

echo ""
echo "全平台编译完成！输出目录: $ROOT_DIR/build/"
ls -lh "$ROOT_DIR/build/"
