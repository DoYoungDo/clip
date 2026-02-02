#!/bin/bash

# Windows AMD64 构建脚本

APP_NAME="clip"
VERSION="1.0.0"
BUILD_DIR="build"

mkdir -p $BUILD_DIR

echo "编译 Windows AMD64..."

if [ -f "script/icon.ico" ]; then
    echo "为 Windows 版本嵌入图标..."
    # 检查是否安装了 rsrc 工具
    if ! command -v rsrc &> /dev/null; then
        echo "安装 rsrc 工具..."
        go install github.com/akavel/rsrc@latest
        # 确保 GOPATH/bin 在 PATH 中
        export PATH=$PATH:$(go env GOPATH)/bin
    fi
    
    # 检查 rsrc 是否可用
    if command -v rsrc &> /dev/null; then
        # 生成 Windows 资源文件
        rsrc -ico script/icon.ico -o rsrc.syso
        GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o $BUILD_DIR/${APP_NAME}-windows-amd64.exe .
        rm -f rsrc.syso
    else
        echo "警告: rsrc 工具安装失败，跳过图标嵌入"
        GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o $BUILD_DIR/${APP_NAME}-windows-amd64.exe .
    fi
else
    GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o $BUILD_DIR/${APP_NAME}-windows-amd64.exe .
fi

echo "Windows AMD64 编译完成: $BUILD_DIR/${APP_NAME}-windows-amd64.exe"
