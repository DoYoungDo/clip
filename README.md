# Clip - 跨平台剪贴板管理工具

系统托盘剪贴板历史管理工具，支持文本/图片、分组管理、翻译和局域网共享。

## 特性

- 📋 自动记录剪贴板历史（文本/图片）
- 📁 分组管理，独立保存不同类别内容
- 🔍 快速搜索历史记录
- 🌈 颜色格式识别与转换（Hex ↔ RGB）
- 🌍 接入百度翻译 / 有道翻译
- 🔐 翻译器密钥保存在系统凭据存储中
- 🌐 局域网实时共享剪贴板

## 构建

```bash
# 快速构建
./build.sh

# 指定平台
./script/build-mac-arm64.sh     # macOS Apple Silicon
./script/build-mac-amd64.sh     # macOS Intel
./script/build-linux-amd64.sh   # Linux
./script/build-win-amd64.sh     # Windows
```

## 使用

### 基本操作
- **左键** - 查看历史，点击复制
- **右键** - 完整菜单和配置

### 分组管理
```
1. 复制分组名 "工作笔记"
2. 右键 → ➕ 创建分组
3. 右键点击分组 → 激活
4. 之后的剪贴板内容会自动保存到该分组
```

### 局域网共享
**场景：两台电脑互相共享剪贴板，甚至可以通过复制文本在菜单中"聊天"**

**电脑 A（服务端）**:
```
1. 右键 → 配置 → 局域网共享 → 启用
2. 地址自动复制到剪贴板（如 192.168.1.100:54321）
```

**电脑 B（客户端）**:
```
1. 粘贴地址 192.168.1.100:54321
2. 右键 → 配置 → 局域网共享 → 连接到
3. 连接成功！
```

**"聊天"示例**:
```
电脑 A: 复制 "晚上一起吃饭吗？"
电脑 B: 在历史记录中看到 [R] 标记的消息
电脑 B: 复制 "好啊，几点？"
电脑 A: 看到回复 [R] "好啊，几点？"
```

### 颜色转换
```
1. 右键 → 配置 → 自动识别颜色 ✓
2. 复制 #FF5733
3. 左键点击该条目 → 显示 "复制RGB" 选项
4. 点击后得到 255,87,51
```

### 翻译
```
1. 复制翻译器密钥到剪贴板
   - 百度翻译: "appid 密钥"
   - 有道翻译: "appkey 密钥"
2. 右键 → 配置 → 翻译 → 选择翻译器
3. 再选择 "翻译为: zh / en"
4. 之后复制文本时，托盘标题会显示翻译结果
```

说明：
- 已保存的翻译器密钥不会写入 `config.json`
- 密钥会写入当前系统的凭据存储（如 macOS Keychain、Windows Credential Manager、Linux Secret Service）
- 如果系统凭据存储不可用，翻译器将无法自动恢复启用状态

### 搜索
```
1. 复制关键词 "密码"
2. 右键 → 🔎 搜索
3. 只显示包含"密码"的历史记录
```

## 配置

配置文件：系统配置目录下的 `Clip/config.json`

常见位置：
- macOS: `~/Library/Application Support/Clip/config.json`
- Linux: `~/.config/Clip/config.json`
- Windows: `%AppData%/Clip/config.json`

- `history_max`: 最大历史条数（1-300）
- `single_delete`: 启用单条删除
- `auto_recognize_color`: 自动识别颜色
- `save_log_to_local`: 退出时保存日志
- `translator_data.lang`: 翻译目标语言
- `translator_data.current_translator_id`: 当前启用的翻译器
- `translator_data.inited_translators`: 已初始化翻译器列表（仅保存 id）

## 系统要求

macOS 10.12+ / Windows 10+ / Linux (X11/Wayland)
