#!/bin/bash

# 全平台构建脚本

echo "开始全平台编译..."

# 执行各平台构建脚本
script/build-mac-arm64.sh
script/build-mac-amd64.sh
script/build-linux-amd64.sh
script/build-linux-arm64.sh
script/build-win-amd64.sh
script/build-win-arm64.sh

echo ""
echo "全平台编译完成！输出目录: build/"
ls -lh build/
