package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigAndLogPathUseUserConfigDir(t *testing.T) {
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldXdg := os.Getenv("XDG_CONFIG_HOME")

	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "xdg"))
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldXdg)
	}()

	configPath := getConfigPath()
	logPath := getLogPath()

	if !strings.Contains(configPath, appDataDirName) {
		t.Fatalf("配置路径应包含应用目录名，got %s", configPath)
	}
	if filepath.Base(configPath) != "config.json" {
		t.Fatalf("配置文件名错误: %s", filepath.Base(configPath))
	}
	if filepath.Base(logPath) != "clip.log" {
		t.Fatalf("日志文件名错误: %s", filepath.Base(logPath))
	}

	target := filepath.Join(configPath, "..", "nested", "file.txt")
	if err := ensureParentDir(target); err != nil {
		t.Fatalf("ensureParentDir 返回错误: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(target)); err != nil {
		t.Fatalf("父目录应已创建: %v", err)
	}
}

func TestGetColor(t *testing.T) {
	r, g, b, base, ok := getColor("#aabbcc")
	if !ok || r != "aa" || g != "bb" || b != "cc" || base != 16 {
		t.Fatalf("HEX 解析错误: r=%s g=%s b=%s base=%d ok=%v", r, g, b, base, ok)
	}

	r, g, b, base, ok = getColor("12, 34, 56")
	if !ok || r != "12" || g != "34" || b != "56" || base != 10 {
		t.Fatalf("RGB 解析错误: r=%s g=%s b=%s base=%d ok=%v", r, g, b, base, ok)
	}

	if _, _, _, _, ok = getColor("not-a-color"); ok {
		t.Fatalf("非法颜色字符串不应解析成功")
	}
}
