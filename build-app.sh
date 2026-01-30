#!/bin/bash

# macOS App 打包脚本

APP_NAME="Clip"
VERSION="1.0.0"
BUILD_DIR="build"
APP_DIR="$BUILD_DIR/$APP_NAME.app"

echo "开始打包 macOS 应用..."

# 编译
echo "编译..."
go build -ldflags="-s -w" -o $BUILD_DIR/clip .

# 创建 .app 目录结构
echo "创建 .app 结构..."
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# 移动可执行文件
mv $BUILD_DIR/clip "$APP_DIR/Contents/MacOS/$APP_NAME"

# 创建 Info.plist
cat > "$APP_DIR/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$APP_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>com.clip.history</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

echo ""
echo "打包完成！"
echo "应用位置: $APP_DIR"
echo ""
echo "运行方式："
echo "  open $APP_DIR"
echo ""
echo "或者拖动到 /Applications 文件夹"
