package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryChangeCallbackFiresOnMutations(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(2)
	changes := 0
	history.SetOnChange(func() { changes++ })

	first := NewClipItem(TypeText, []byte("one"))
	history.Add(first)
	if changes != 1 {
		t.Fatalf("添加新记录应触发变更回调，实际 %d 次", changes)
	}

	history.Add(NewClipItem(TypeText, []byte("one")))
	if changes != 1 {
		t.Fatalf("顶部重复记录不应触发变更回调，实际 %d 次", changes)
	}

	history.Add(NewClipItem(TypeText, []byte("two")))
	history.Add(NewClipItem(TypeText, []byte("three")))
	if changes != 3 {
		t.Fatalf("添加及超限截断应触发变更回调，实际 %d 次", changes)
	}

	if !history.Rename(history.GetTop(), "renamed") {
		t.Fatalf("重命名应成功")
	}
	if changes != 4 {
		t.Fatalf("重命名应触发变更回调，实际 %d 次", changes)
	}
	if history.Rename(first, "missing") {
		t.Fatalf("已被替换的旧项不应重命名成功")
	}
	if changes != 4 {
		t.Fatalf("失败的重命名不应触发变更回调，实际 %d 次", changes)
	}

	if !history.Delete(history.GetTop()) {
		t.Fatalf("删除应成功")
	}
	if changes != 5 {
		t.Fatalf("删除应触发变更回调，实际 %d 次", changes)
	}
	if history.Delete(nil) {
		t.Fatalf("删除 nil 应失败")
	}
	if changes != 5 {
		t.Fatalf("失败的删除不应触发变更回调，实际 %d 次", changes)
	}

	history.Clear()
	if changes != 6 {
		t.Fatalf("清空应触发变更回调，实际 %d 次", changes)
	}
	history.Clear()
	if changes != 6 {
		t.Fatalf("清空空历史不应触发变更回调，实际 %d 次", changes)
	}
}

func TestHistoryChangeCallbackFiresOnRemoveDuplicates(t *testing.T) {
	resetTestLogChannel()
	history := NewHistory(10)
	history.Add(NewClipItem(TypeText, []byte("dup")))
	history.Add(NewClipItem(TypeText, []byte("other")))
	history.Add(NewClipItem(TypeText, []byte("dup")))

	changes := 0
	history.SetOnChange(func() { changes++ })

	if removed := history.RemoveDuplicates(); removed != 1 {
		t.Fatalf("应移除 1 条重复记录，实际 %d", removed)
	}
	if changes != 1 {
		t.Fatalf("去重应触发变更回调，实际 %d 次", changes)
	}

	if removed := history.RemoveDuplicates(); removed != 0 {
		t.Fatalf("再次去重不应移除记录，实际 %d", removed)
	}
	if changes != 1 {
		t.Fatalf("无变化的去重不应触发变更回调，实际 %d 次", changes)
	}
}

func TestRequestSaveNeverBlocksWhenQueueFull(t *testing.T) {
	app := NewApp(NewAppLogger())

	for i := 0; i < 3; i++ {
		app.requestSave()
	}

	done := make(chan struct{})
	go func() {
		app.requestSave()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("保存请求不应阻塞菜单事件")
	}

	if got := len(app.saveTrigger); got != 1 {
		t.Fatalf("保存请求应合并为 1 个待处理信号，实际 %d", got)
	}
}

func TestStateSaverPersistsChangesInRealTime(t *testing.T) {
	resetTestLogChannel()
	tempDir := t.TempDir()
	configPathOverride = filepath.Join(tempDir, "config.json")
	legacyConfigPathOverride = filepath.Join(tempDir, "legacy-config.json")
	defer func() {
		configPathOverride = ""
		legacyConfigPathOverride = ""
	}()

	app := NewApp(NewAppLogger())
	app.startStateSaver()
	defer app.stopStateSaver()

	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("realtime")))
	app.configHistoryMax.Store(7)
	app.requestSave()

	waitForAppCondition(t, 3*time.Second, func() bool {
		config, err := loadConfigFromPath(configPathOverride)
		if err != nil {
			return false
		}
		if config.HistoryMax != 7 || len(config.Data.History) != 1 {
			return false
		}
		return string(config.Data.History[0].Content) == "realtime"
	})
}

func TestStateSaverSavesSilently(t *testing.T) {
	resetTestLogChannel()
	tempDir := t.TempDir()
	configPathOverride = filepath.Join(tempDir, "config.json")
	legacyConfigPathOverride = filepath.Join(tempDir, "legacy-config.json")
	defer func() {
		configPathOverride = ""
		legacyConfigPathOverride = ""
	}()

	app := NewApp(NewAppLogger())
	app.startStateSaver()
	defer app.stopStateSaver()

	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("silent")))
	waitForAppCondition(t, 3*time.Second, func() bool {
		_, err := os.Stat(configPathOverride)
		return err == nil
	})

	for {
		select {
		case entry := <-global_log_channel:
			if strings.Contains(entry.Content, "正在保存配置和历史记录") {
				t.Fatalf("自动保存不应产生保存日志: %s", entry.Content)
			}
		default:
			return
		}
	}
}

func TestStateSaverCanRestart(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())

	app.startStateSaver()
	app.stopStateSaver()
	app.startStateSaver()
	app.stopStateSaver()
}
