//go:build linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// autostartPath 返回 XDG 自启动 desktop 文件路径；无法定位配置目录时返回空串。
func autostartPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "autostart", autostartLinuxFileName)
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
		return fmt.Errorf("无法定位用户配置目录")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(autostartDesktopContent(exePath)), 0644)
}

func disableAutostart() error {
	path := autostartPath()
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
