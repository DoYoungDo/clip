package app

import (
	"fmt"
	"os"
	"strings"
)

// autostartDarwinLabel 是 macOS LaunchAgent 的 Label，同时作为 plist 文件名。
const autostartDarwinLabel = "com.clip.history"

// autostartLinuxFileName 是 Linux XDG 自启动文件名。
const autostartLinuxFileName = "clip.desktop"

var plistEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

// autostartPlistContent 生成 macOS LaunchAgent plist 内容。
func autostartPlistContent(exePath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, autostartDarwinLabel, plistEscaper.Replace(exePath))
}

// autostartDesktopContent 生成 Linux XDG 自启动 desktop 文件内容。
func autostartDesktopContent(exePath string) string {
	return fmt.Sprintf(`[Desktop Entry]
Name=%s
Comment=Clipboard History Tool
Exec=%s
Type=Application
X-GNOME-Autostart-enabled=true
`, appName, desktopExecEscape(exePath))
}

// desktopExecEscape 在路径含空白或引号时按 desktop 规范包裹引号并转义。
func desktopExecEscape(exePath string) string {
	if !strings.ContainsAny(exePath, " \t\"'\\") {
		return exePath
	}
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(exePath)
	return `"` + escaped + `"`
}

// toggleAutostart 切换开机自启动开关，状态以系统实际配置为准。
func (a *App) toggleAutostart() {
	enabled := !autostartEnabled()
	if err := a.setAutostart(enabled); err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("设置开机自启动失败: %v", err)}
		return
	}
	global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(enabled, "已开启开机自启动", "已关闭开机自启动")}
}

// setAutostart 显式设置开机自启动状态；与系统当前状态一致时不做任何操作。
func (a *App) setAutostart(enabled bool) error {
	if enabled == autostartEnabled() {
		return nil
	}
	if !enabled {
		return disableAutostart()
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取程序路径: %v", err)
	}
	return enableAutostart(exePath)
}
