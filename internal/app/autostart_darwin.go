//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// autostartPath 返回 LaunchAgent plist 路径；无法定位主目录时返回空串。
func autostartPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", autostartDarwinLabel+".plist")
}

func autostartEnabled() bool {
	path := autostartPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func enableAutostart(exePath string) error {
	path := autostartPath()
	if path == "" {
		return fmt.Errorf("无法定位用户主目录")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(autostartPlistContent(exePath)), 0644); err != nil {
		return err
	}
	if err := exec.Command("launchctl", "load", path).Run(); err != nil {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("launchctl load 失败(下次登录仍会生效): %v", err)}
	}
	return nil
}

func disableAutostart() error {
	path := autostartPath()
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	if err := exec.Command("launchctl", "unload", path).Run(); err != nil {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("launchctl unload 失败(删除配置后不再自启动): %v", err)}
	}
	return os.Remove(path)
}
