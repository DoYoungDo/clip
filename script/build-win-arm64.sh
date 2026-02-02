#!/bin/bash

# Windows ARM64 构建脚本

APP_NAME="clip"
VERSION="1.0.0"
BUILD_DIR="build"

mkdir -p $BUILD_DIR

echo "编译 Windows ARM64..."

if [ -f "script/icon.ico" ]; then
    echo "为 Windows 版本嵌入图标..."
    # ARM64 版本跳过图标嵌入（rsrc 工具不支持 ARM64）
    echo "警告: Windows ARM64 跳过图标嵌入（rsrc 工具不支持）"
    GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -H=windowsgui" -o $BUILD_DIR/${APP_NAME}-windows-arm64.exe .
else
    GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -H=windowsgui" -o $BUILD_DIR/${APP_NAME}-windows-arm64.exe .
fi

echo "Windows ARM64 编译完成: $BUILD_DIR/${APP_NAME}-windows-arm64.exe"
