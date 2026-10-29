# Clip - 跨平台剪貼簿管理工具

[简体中文](README.md) | [English](README.en.md) | **繁體中文**

系統托盤剪貼簿歷史管理工具，支援文字/圖片、分組管理、翻譯和區域網路共享。

## 特性

- 📋 自動記錄剪貼簿歷史（文字/圖片）
- 📁 分組管理，獨立保存不同類別內容
- 🔍 快速搜尋歷史記錄
- 🌈 顏色格式識別與轉換（Hex ↔ RGB）
- 🔢 進制轉換（二進制 / 八進制 / 十進制 / 十六進制互轉）
- 🌍 接入百度翻譯 / 有道翻譯
- 🔐 翻譯器金鑰保存在系統憑證儲存中
- 🌐 區域網路即時共享剪貼簿

## 建置

```bash
# 快速建置
./build.sh

# 指定平台
./script/build-mac-arm64.sh     # macOS Apple Silicon
./script/build-mac-amd64.sh     # macOS Intel
./script/build-linux-amd64.sh   # Linux
./script/build-win-amd64.sh     # Windows
```

> **提示**：macOS 應用簽署需要購買證書，作者沒有證書，因此打包的 macOS 應用均未簽署。如需強行使用，請參考：[mac 安裝未知應用常見問題](https://doyoungdo.github.io/essay/#/Other/mac_install_unknown_app)

## 使用

### 基本操作
- **左鍵** - 查看歷史，點擊複製
- **右鍵** - 完整選單和設定

### 分組管理
```
1. 複製分組名 "工作筆記"
2. 右鍵 → ➕ 建立分組
3. 右鍵點擊分組 → 啟用
4. 之後的剪貼簿內容會自動保存到該分組
```

### 區域網路共享
**場景：兩台電腦互相共享剪貼簿，甚至可以透過複製文字在選單中「聊天」**

**電腦 A（伺服器端）**:
```
1. 右鍵 → 設定 → 區域網路共享 → 啟用
2. 位址自動複製到剪貼簿（如 192.168.1.100:54321）
```

**電腦 B（用戶端）**:
```
1. 貼上位址 192.168.1.100:54321
2. 右鍵 → 設定 → 區域網路共享 → 連接到
3. 連線成功！
```

**「聊天」範例**:
```
電腦 A: 複製 "晚上一起吃飯嗎？"
電腦 B: 在歷史記錄中看到 [R] 標記的訊息
電腦 B: 複製 "好啊，幾點？"
電腦 A: 看到回覆 [R] "好啊，幾點？"
```

### 顏色轉換
```
1. 右鍵 → 設定 → 自動識別顏色 ✓
2. 複製 #FF5733
3. 右鍵點擊該條目 → 顯示 "複製Hex / 複製RGB / 複製R / 複製G / 複製B" 選項
4. 點擊後分別得到 #ff5733 / 255,87,51 / 255 / 87 / 51
```

### 進制轉換
```
1. 右鍵 → 設定 → 進制轉換 ✓
2. 複製 255（或帶 0b/0o/0x 前綴的二/八/十六進制數）
3. 右鍵點擊該條目 → 顯示 "複製二進制: 0b11111111" 等選項
4. 點擊後得到對應進制的值
```

### 翻譯
```
1. 複製翻譯器金鑰到剪貼簿
   - 百度翻譯: "appid 金鑰"
   - 有道翻譯: "appkey 金鑰"
2. 右鍵 → 設定 → 翻譯 → 選擇翻譯器
3. 再選擇 "翻譯為: zh / en"
4. 之後複製文字時，托盤標題會顯示翻譯結果
```

說明：
- 已保存的翻譯器金鑰不會寫入 `config.json`
- 金鑰會寫入目前系統的憑證儲存（如 macOS Keychain、Windows Credential Manager、Linux Secret Service）
- 如果系統憑證儲存不可用，翻譯器將無法自動恢復啟用狀態

### 搜尋
```
1. 複製關鍵詞 "密碼"
2. 右鍵 → 🔎 搜尋
3. 只顯示包含"密碼"的歷史記錄
```

## 設定

設定檔案：系統設定目錄下的 `Clip/config.json`

常見位置：
- macOS: `~/Library/Application Support/Clip/config.json`
- Linux: `~/.config/Clip/config.json`
- Windows: `%AppData%/Clip/config.json`

- `history_max`: 最大歷史條數（1-300）
- `keep_latest_only`: 相同內容只保留最近一條
- `confirm_clear_history`: 清空歷史時不詢問（開關預設關閉，點擊清空需二次確認）
- `single_delete`: 啟用單條刪除
- `search_menu_top`: 置頂【搜尋】選單
- `add_to_group`: 新增至分組
- `auto_recognize_color`: 自動識別顏色
- `base_convert`: 進制轉換
- `save_log_to_local`: 退出時保存日誌
- `highlight_prefix` / `highlight_suffix`: 搜尋高亮標記前後字元，預設 【 和 】
- `time_format`: 條目時間顯示格式，支援 YYYY YY MM DD HH mm ss 任意組合，預設 hh:mm
- `translator_data.lang`: 翻譯目標語言，預設 zh
- `translator_data.current_translator_id`: 目前啟用的翻譯器
- `translator_data.inited_translators`: 已初始化翻譯器列表（僅保存 id）

## 系統需求

macOS 10.13+ / Windows 10+ / Linux (X11/Wayland)
