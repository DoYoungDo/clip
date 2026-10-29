//go:build windows

package app

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const autostartRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func autostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	return err == nil
}

func enableAutostart(exePath string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autostartRegistryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %w", err)
	}
	defer key.Close()

	return key.SetStringValue(appName, fmt.Sprintf(`"%s"`, exePath))
}

func disableAutostart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRegistryKey, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
