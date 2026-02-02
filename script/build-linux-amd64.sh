#!/bin/bash

# Linux AMD64 构建脚本

APP_NAME="clip"
VERSION="1.0.0"
BUILD_DIR="build"

mkdir -p $BUILD_DIR

echo "编译 Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $BUILD_DIR/${APP_NAME}-linux-amd64 .

# 创建 desktop 文件和图标
if [ -f "script/icon.png" ]; then
    echo "为 Linux 创建 desktop 文件..."
    cp script/icon.png $BUILD_DIR/clip.png
    
    cat > $BUILD_DIR/clip.desktop << EOF
[Desktop Entry]
Name=Clip
Comment=Clipboard History Tool
Exec=/path/to/clip-linux-amd64
Icon=/path/to/clip.png
Type=Application
Categories=Utility;
StartupNotify=false
NoDisplay=false
EOF
    echo "Linux desktop 文件已创建: $BUILD_DIR/clip.desktop"
    echo "安装方法："
    echo "  1. 复制可执行文件到目标位置"
    echo "  2. 编辑 clip.desktop 中的 Exec 和 Icon 路径"
    echo "  3. cp clip.desktop ~/.local/share/applications/"
fi

echo "Linux AMD64 编译完成: $BUILD_DIR/${APP_NAME}-linux-amd64"
