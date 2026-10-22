# Clip - Cross-platform Clipboard Manager

[简体中文](README.md) | **English** | [繁體中文](README.zh-Hant.md)

A system tray clipboard history manager supporting text/images, groups, translation, and LAN sharing.

## Features

- 📋 Automatically record clipboard history (text/images)
- 📁 Group management for storing content by category
- 🔍 Quick search through history
- 🌈 Color format recognition and conversion (Hex ↔ RGB)
- 🌍 Baidu Translate / Youdao Translate integration
- 🔐 Translator keys stored in the system credential store
- 🌐 Real-time clipboard sharing over the LAN

## Build

```bash
# Quick build
./build.sh

# Target a specific platform
./script/build-mac-arm64.sh     # macOS Apple Silicon
./script/build-mac-amd64.sh     # macOS Intel
./script/build-linux-amd64.sh   # Linux
./script/build-win-amd64.sh     # Windows
```

> **Note**: Signing macOS apps requires a purchased certificate, which the author does not have, so the packaged macOS apps are unsigned. If you still want to use them, see: [Common issues installing unknown apps on macOS](https://doyoungdo.github.io/essay/#/Other/mac_install_unknown_app)

## Usage

### Basics
- **Left click** - View history, click an entry to copy it
- **Right click** - Full menu and settings

### Groups
```
1. Copy a group name such as "Work Notes"
2. Right click → ➕ Create Group
3. Right click the group → Activate
4. Clipboard content will then be saved to that group automatically
```

### LAN Sharing
**Scenario: two computers sharing clipboards, you can even "chat" by copying text in the menu**

**Computer A (server)**:
```
1. Right click → Settings → LAN Sharing → Enable
2. The address is copied to the clipboard automatically (e.g. 192.168.1.100:54321)
```

**Computer B (client)**:
```
1. Paste the address 192.168.1.100:54321
2. Right click → Settings → LAN Sharing → Connect To
3. Connected!
```

**"Chat" example**:
```
Computer A: copy "Dinner together tonight?"
Computer B: sees the message marked with [R] in the history
Computer B: copy "Sure, what time?"
Computer A: sees the reply [R] "Sure, what time?"
```

### Color Conversion
```
1. Right click → Settings → Auto Recognize Colors ✓
2. Copy #FF5733
3. Right click the entry → shows "Copy Hex / Copy RGB" options
4. Click to get 255,87,51
```

### Translation
```
1. Copy a translator key to the clipboard
   - Baidu Translate: "appid secret"
   - Youdao Translate: "appkey secret"
2. Right click → Settings → Translation → choose a translator
3. Then choose "Translate to: zh / en"
4. From then on, copying text shows the translation in the tray title
```

Notes:
- Saved translator keys are never written to `config.json`
- Keys are stored in the system credential store (e.g. macOS Keychain, Windows Credential Manager, Linux Secret Service)
- If the system credential store is unavailable, translators cannot be restored automatically

### Search
```
1. Copy a keyword such as "password"
2. Right click → 🔎 Search
3. Only history entries containing "password" are shown
```

## Configuration

Config file: `Clip/config.json` in the system config directory

Common locations:
- macOS: `~/Library/Application Support/Clip/config.json`
- Linux: `~/.config/Clip/config.json`
- Windows: `%AppData%/Clip/config.json`

- `history_max`: Maximum number of history entries (1-300)
- `keep_latest_only`: Keep only the latest entry for identical content
- `confirm_clear_history`: Clear history without confirmation (off by default; clearing requires a second confirmation)
- `single_delete`: Enable single-entry deletion
- `search_menu_top`: Pin the Search menu to the top
- `move_to_group`: Move entries to groups
- `auto_recognize_color`: Auto recognize colors
- `save_log_to_local`: Save logs on exit
- `highlight_prefix` / `highlight_suffix`: Characters wrapping search matches, defaults to 【 and 】
- `time_format`: Entry time display format, any combination of YYYY YY MM DD HH mm ss, default hh:mm
- `translator_data.lang`: Translation target language, defaults to zh
- `translator_data.current_translator_id`: Currently enabled translator
- `translator_data.inited_translators`: Initialized translator list (ids only)

## System Requirements

macOS 10.13+ / Windows 10+ / Linux (X11/Wayland)
