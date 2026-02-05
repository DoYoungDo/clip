#!/bin/bash

# macOS ARM64 构建脚本

APP_NAME="clip"
VERSION="1.0.0"
BUILD_DIR="build"

mkdir -p $BUILD_DIR

echo "编译 macOS ARM64..."
#GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $BUILD_DIR/${APP_NAME}-darwin-arm64 .

CGO_ENABLED=1 \
GOOS=darwin \
GOARCH=arm64 \
CGO_CFLAGS="-mmacosx-version-min=10.13 -O2" \
CGO_LDFLAGS="-mmacosx-version-min=10.13" \
go build \
  -ldflags="-s -w -X main.buildVersion=1.0.0" \
  -a \
  -o $BUILD_DIR/${APP_NAME}-darwin-arm64 .

echo "macOS ARM64 编译完成: $BUILD_DIR/${APP_NAME}-darwin-arm64"
echo "提示: 使用 ./build-app.sh 创建完整的 .app 应用包"
