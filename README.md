# Clipboard History Tool

跨平台剪贴板历史工具，支持文本、图片、文件的历史记录和快速复制。

## 功能

- 实时监听剪贴板变化
- 缓存最近 50 条历史记录
- 支持文本、图片和文件类型
- 系统托盘显示，无窗口干扰
- 支持分组管理
- 一键复制历史内容

## 安装依赖

```bash
go mod tidy
```

## 运行

```bash
go run .
```

## 构建

### macOS
```bash
# 构建 .app 应用包（推荐）
./build-app.sh

# 运行
open build/Clip.app

# 或安装到应用程序文件夹
cp -r build/Clip.app /Applications/
```

### Windows
```bash
# 编译（不显示控制台窗口）
./build.sh

# 运行
build/clip-windows-amd64.exe
```

### Linux
```bash
# 编译
./build.sh

# 运行
./build/clip-linux-amd64 &

# 或安装 desktop 文件
cp clip.desktop ~/.local/share/applications/
# 编辑文件中的 Exec 路径为实际路径
```

## 使用

1. 启动程序后会在系统托盘显示 📋 图标
2. 点击托盘图标查看历史记录
3. 点击任意历史条目可重新复制
4. 支持创建分组、激活分组等高级功能

## 分组功能

- **创建分组**：点击"➕ 创建分组"，使用最新剪贴板内容作为分组名
- **激活分组**：激活后，新的剪贴板内容会自动同步到该分组
- **重命名分组**：使用最新剪贴板内容重命名分组
- **删除分组**：删除整个分组及其内容
