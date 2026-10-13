#!/bin/bash

set -euo pipefail

# Windows ARM64 构建脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

APP_NAME="clip"
VERSION="${VERSION:-1.0.0}"
BUILD_DIR="$ROOT_DIR/build"
APP_VERSION="$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)"

mkdir -p "$BUILD_DIR"

echo "编译 Windows ARM64..."

if [ -f "$SCRIPT_DIR/icon.ico" ]; then
    echo "为 Windows 版本嵌入图标..."
    if ! command -v rsrc &> /dev/null; then
        echo "安装 rsrc 工具..."
        go install github.com/akavel/rsrc@latest
        export PATH="$PATH:$(go env GOPATH)/bin"
    fi

    if command -v rsrc &> /dev/null; then
        # 生成 Windows 资源文件（syso 必须与入口包同目录才会被链接）
        trap 'rm -f "$ROOT_DIR/cmd/clip/rsrc.syso"' EXIT
        (cd "$ROOT_DIR" && rsrc -arch arm64 -ico "$SCRIPT_DIR/icon.ico" -o cmd/clip/rsrc.syso)
        (cd "$ROOT_DIR" && GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -X clip/internal/app.version=$APP_VERSION -H=windowsgui" -o "$BUILD_DIR/${APP_NAME}-windows-arm64.exe" ./cmd/clip)
    else
        echo "警告: rsrc 工具安装失败，跳过图标嵌入"
        (cd "$ROOT_DIR" && GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -X clip/internal/app.version=$APP_VERSION -H=windowsgui" -o "$BUILD_DIR/${APP_NAME}-windows-arm64.exe" ./cmd/clip)
    fi
else
    (cd "$ROOT_DIR" && GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -X clip/internal/app.version=$APP_VERSION -H=windowsgui" -o "$BUILD_DIR/${APP_NAME}-windows-arm64.exe" ./cmd/clip)
fi

echo "Windows ARM64 编译完成: $BUILD_DIR/${APP_NAME}-windows-arm64.exe"
