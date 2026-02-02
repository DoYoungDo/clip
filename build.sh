#!/bin/bash

# 构建脚本快捷入口

case "$1" in
    "app")
        script/build-app.sh
        ;;
    "all")
        script/build.sh
        ;;
    "mac-arm64")
        script/build-mac-arm64.sh
        ;;
    "mac-amd64")
        script/build-mac-amd64.sh
        ;;
    "linux-amd64")
        script/build-linux-amd64.sh
        ;;
    "linux-arm64")
        script/build-linux-arm64.sh
        ;;
    "win-amd64")
        script/build-win-amd64.sh
        ;;
    "win-arm64")
        script/build-win-arm64.sh
        ;;
    *)
        echo "用法: $0 [app|all|mac-arm64|mac-amd64|linux-amd64|linux-arm64|win-amd64|win-arm64]"
        echo ""
        echo "选项:"
        echo "  app         - 构建 macOS .app 应用包"
        echo "  all         - 构建所有平台"
        echo "  mac-arm64   - 构建 macOS ARM64"
        echo "  mac-amd64   - 构建 macOS AMD64"
        echo "  linux-amd64 - 构建 Linux AMD64"
        echo "  linux-arm64 - 构建 Linux ARM64"
        echo "  win-amd64   - 构建 Windows AMD64"
        echo "  win-arm64   - 构建 Windows ARM64"
        ;;
esac
