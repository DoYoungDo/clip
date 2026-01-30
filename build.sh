#!/bin/bash

# 编译脚本 - 支持多平台编译

APP_NAME="clip"
VERSION="1.0.0"
BUILD_DIR="build"

# 创建构建目录
mkdir -p $BUILD_DIR

echo "开始编译 $APP_NAME v$VERSION..."

# 当前平台编译（macOS ARM64）
echo "编译 macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $BUILD_DIR/${APP_NAME}-darwin-arm64 .

# macOS Intel
echo "编译 macOS Intel..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $BUILD_DIR/${APP_NAME}-darwin-amd64 .

# Linux
echo "编译 Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -o $BUILD_DIR/${APP_NAME}-linux-amd64 .

echo "编译 Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -o $BUILD_DIR/${APP_NAME}-linux-arm64 .

# Windows
echo "编译 Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o $BUILD_DIR/${APP_NAME}-windows-amd64.exe .

echo ""
echo "编译完成！输出目录: $BUILD_DIR/"
ls -lh $BUILD_DIR/
