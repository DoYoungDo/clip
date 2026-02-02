#!/bin/bash

# macOS AMD64 构建脚本

APP_NAME="clip"
VERSION="1.0.0"
BUILD_DIR="build"

mkdir -p $BUILD_DIR

echo "编译 macOS AMD64..."
echo "跳过 macOS AMD64 - systray 库不支持交叉编译"
