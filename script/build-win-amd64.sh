#!/bin/bash

set -euo pipefail

# Windows AMD64 构建脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

APP_NAME="clip"
VERSION="${VERSION:-1.0.0}"
BUILD_DIR="$ROOT_DIR/build"

mkdir -p "$BUILD_DIR"

echo "编译 Windows AMD64..."

if [ -f "$SCRIPT_DIR/icon.ico" ]; then
    echo "为 Windows 版本嵌入图标..."
    # 检查是否安装了 rsrc 工具
    if ! command -v rsrc &> /dev/null; then
        echo "安装 rsrc 工具..."
        go install github.com/akavel/rsrc@latest
        # 确保 GOPATH/bin 在 PATH 中
        export PATH="$PATH:$(go env GOPATH)/bin"
    fi
    
    # 检查 rsrc 是否可用
    if command -v rsrc &> /dev/null; then
        # 生成 Windows 资源文件
        trap 'rm -f "$ROOT_DIR/rsrc.syso"' EXIT
        (cd "$ROOT_DIR" && rsrc -ico "$SCRIPT_DIR/icon.ico" -o rsrc.syso)
        (cd "$ROOT_DIR" && GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o "$BUILD_DIR/${APP_NAME}-windows-amd64.exe" .)
    else
        echo "警告: rsrc 工具安装失败，跳过图标嵌入"
        (cd "$ROOT_DIR" && GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o "$BUILD_DIR/${APP_NAME}-windows-amd64.exe" .)
    fi
else
    (cd "$ROOT_DIR" && GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o "$BUILD_DIR/${APP_NAME}-windows-amd64.exe" .)
fi

echo "Windows AMD64 编译完成: $BUILD_DIR/${APP_NAME}-windows-amd64.exe"
