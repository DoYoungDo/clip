package main

import (
	"encoding/json"
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

func TestLoadMergedConfigMigratesLegacyConfig(t *testing.T) {
	tempDir := t.TempDir()
	legacyDir := filepath.Join(tempDir, "legacy")
	newDir := filepath.Join(tempDir, "xdg")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("创建 legacy 目录失败: %v", err)
	}

	oldHome := os.Getenv("HOME")
	oldXdg := os.Getenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", newDir)
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldXdg)
	}()
	legacyConfigPathOverride = filepath.Join(legacyDir, "config.json")
	defer func() { legacyConfigPathOverride = "" }()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(legacyDir); err != nil {
		t.Fatalf("切换工作目录失败: %v", err)
	}

	legacy := NewDefaultConfig()
	legacy.HistoryMax = 123
	legacy.Data.History = []*ClipItem{NewClipItem(TypeText, []byte("legacy-item"))}
	legacyBytes, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("序列化 legacy 配置失败: %v", err)
	}
	if err := os.WriteFile("config.json", legacyBytes, 0644); err != nil {
		t.Fatalf("写入 legacy 配置失败: %v", err)
	}

	merged, source, err := loadMergedConfig()
	if err != nil {
		t.Fatalf("加载迁移配置失败: %v", err)
	}
	if source != "migrated" {
		t.Fatalf("期望来源为 migrated，实际 %s", source)
	}
	if merged.HistoryMax != 123 {
		t.Fatalf("迁移后配置值错误，got %d", merged.HistoryMax)
	}

	if _, err := os.Stat(getConfigPath()); err != nil {
		t.Fatalf("新路径配置应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "config.json.migrated.bak")); err != nil {
		t.Fatalf("旧配置备份应存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("旧配置原文件应被迁移移除，err=%v", err)
	}
}

func TestLoadMergedConfigMergesCurrentAndLegacy(t *testing.T) {
	tempDir := t.TempDir()
	legacyDir := filepath.Join(tempDir, "legacy")
	newDir := filepath.Join(tempDir, "xdg")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("创建 legacy 目录失败: %v", err)
	}

	oldHome := os.Getenv("HOME")
	oldXdg := os.Getenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", newDir)
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldXdg)
	}()
	legacyConfigPathOverride = filepath.Join(legacyDir, "config.json")
	defer func() { legacyConfigPathOverride = "" }()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(legacyDir); err != nil {
		t.Fatalf("切换工作目录失败: %v", err)
	}

	legacy := NewDefaultConfig()
	legacy.HistoryMax = 100
	legacy.Data.History = []*ClipItem{
		NewClipItem(TypeText, []byte("legacy-only")),
		NewClipItem(TypeText, []byte("shared")),
	}
	legacy.Data.Groups["legacy-group"] = HistoryGroupData{
		Active:  false,
		History: []*ClipItem{NewClipItem(TypeText, []byte("legacy-group-item"))},
	}
	legacy.Data.GroupNames = []string{"legacy-group"}

	current := NewDefaultConfig()
	current.HistoryMax = 200
	current.Data.History = []*ClipItem{
		NewClipItem(TypeText, []byte("current-only")),
		NewClipItem(TypeText, []byte("shared")),
	}
	current.Data.Groups["current-group"] = HistoryGroupData{
		Active:  true,
		History: []*ClipItem{NewClipItem(TypeText, []byte("current-group-item"))},
	}
	current.Data.GroupNames = []string{"current-group"}

	legacyBytes, _ := json.Marshal(legacy)
	currentBytes, _ := json.Marshal(current)
	if err := os.WriteFile("config.json", legacyBytes, 0644); err != nil {
		t.Fatalf("写入 legacy 配置失败: %v", err)
	}
	if err := saveConfigToPath(getConfigPath(), current); err != nil {
		t.Fatalf("写入 current 配置失败: %v", err)
	}
	_ = currentBytes

	merged, source, err := loadMergedConfig()
	if err != nil {
		t.Fatalf("加载合并配置失败: %v", err)
	}
	if source != "merged" {
		t.Fatalf("期望来源为 merged，实际 %s", source)
	}
	if merged.HistoryMax != 200 {
		t.Fatalf("应优先保留新配置值，got %d", merged.HistoryMax)
	}
	if len(merged.Data.History) != 3 {
		t.Fatalf("历史应去重合并为 3 条，实际 %d", len(merged.Data.History))
	}
	if _, ok := merged.Data.Groups["legacy-group"]; !ok {
		t.Fatalf("合并后应保留 legacy-group")
	}
	if _, ok := merged.Data.Groups["current-group"]; !ok {
		t.Fatalf("合并后应保留 current-group")
	}
	if len(merged.Data.GroupNames) != 2 {
		t.Fatalf("分组名应合并为 2 个，实际 %d", len(merged.Data.GroupNames))
	}
}
