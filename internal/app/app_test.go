package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"clip/internal/translator"
)

type stubTranslator struct {
	name      string
	translate func(text string) (string, error)
}

func (s *stubTranslator) Id() string   { return "StubTranslator" }
func (s *stubTranslator) Name() string { return s.name }

func (s *stubTranslator) Translate(text string, lang translator.TransLang) (string, error) {
	return s.translate(text)
}

func (s *stubTranslator) Enable(secret string) bool { return true }
func (s *stubTranslator) IsEnabled() bool           { return true }
func (s *stubTranslator) Secret() string            { return "" }

func menuTitleOf(app *App) string {
	app.translateMu.Lock()
	defer app.translateMu.Unlock()
	return app.menuTitle
}

func waitForAppCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待条件超时")
}

func newStubApp(translate func(text string) (string, error)) *App {
	app := NewApp(NewAppLogger())
	app.setTranslator(&stubTranslator{name: "Stub", translate: translate})
	return app
}

func TestTranslateItemWritesMenuTitle(t *testing.T) {
	app := newStubApp(func(text string) (string, error) {
		return "T:" + text, nil
	})

	app.translateItem(NewClipItem(TypeText, []byte("hello")))
	waitForAppCondition(t, time.Second, func() bool { return menuTitleOf(app) == "T:hello" })

	if got := app.takeMenuTitle(); got != "T:hello" {
		t.Fatalf("takeMenuTitle 应返回翻译结果，got %q", got)
	}
	if got := app.takeMenuTitle(); got != "" {
		t.Fatalf("取走后标题应清空，got %q", got)
	}
}

func TestTranslateItemKeepsLatestResultOnOutOfOrder(t *testing.T) {
	releaseSlow := make(chan struct{})
	app := newStubApp(func(text string) (string, error) {
		if text == "slow" {
			<-releaseSlow
		}
		return "T:" + text, nil
	})

	app.translateItem(NewClipItem(TypeText, []byte("slow")))
	app.translateItem(NewClipItem(TypeText, []byte("fast")))

	waitForAppCondition(t, time.Second, func() bool { return menuTitleOf(app) == "T:fast" })

	close(releaseSlow)
	time.Sleep(50 * time.Millisecond)
	if got := menuTitleOf(app); got != "T:fast" {
		t.Fatalf("过期翻译不应覆盖最新结果，got %q", got)
	}
}

func TestTranslateItemWithoutTranslator(t *testing.T) {
	app := NewApp(NewAppLogger())
	app.translateItem(NewClipItem(TypeText, []byte("hello")))
	time.Sleep(20 * time.Millisecond)

	if got := app.takeMenuTitle(); got != "" {
		t.Fatalf("未启用翻译时不应产生标题，got %q", got)
	}
}

func TestTranslateItemInFlightKeptAfterTake(t *testing.T) {
	release := make(chan struct{})
	app := newStubApp(func(text string) (string, error) {
		<-release
		return "T:" + text, nil
	})

	app.translateItem(NewClipItem(TypeText, []byte("hello")))
	if got := app.takeMenuTitle(); got != "" {
		t.Fatalf("翻译未完成时不应取到标题，got %q", got)
	}

	close(release)
	waitForAppCondition(t, time.Second, func() bool { return menuTitleOf(app) == "T:hello" })
	if got := app.takeMenuTitle(); got != "T:hello" {
		t.Fatalf("进行中的翻译完成后应可被取走，got %q", got)
	}
}

func TestAppTranslatorStateConcurrentAccess(t *testing.T) {
	app := newStubApp(func(text string) (string, error) {
		return text, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				app.setTranslator(app.currentTranslator())
				app.setTranslateToLang(translator.EN)
				_ = app.currentTranslateToLang()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				app.translateItem(NewClipItem(TypeText, []byte("x")))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = app.takeMenuTitle()
			}
		}()
	}
	wg.Wait()
}

func TestHandleClipboardItemKeepLatestOnly(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	app.configKeepLatestOnly.Store(true)
	group := NewGroup("g", true, 10)
	app.groups["g"] = group

	app.handleClipboardItem(NewClipItem(TypeText, []byte("same")))
	app.handleClipboardItem(NewClipItem(TypeText, []byte("other")))
	app.handleClipboardItem(NewClipItem(TypeText, []byte("same")))

	all := app.history.GetAll()
	if len(all) != 2 {
		t.Fatalf("开启去重后主历史期望 2 条，实际 %d", len(all))
	}
	if string(all[0].Content) != "same" || string(all[1].Content) != "other" {
		t.Fatalf("相同内容应只保留最近一条: got [%s, %s]", string(all[0].Content), string(all[1].Content))
	}

	groupAll := group.History.GetAll()
	if len(groupAll) != 2 {
		t.Fatalf("开启去重后分组历史期望 2 条，实际 %d", len(groupAll))
	}
}

func TestHandleClipboardItemKeepsDuplicatesByDefault(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())

	app.handleClipboardItem(NewClipItem(TypeText, []byte("same")))
	app.handleClipboardItem(NewClipItem(TypeText, []byte("other")))
	app.handleClipboardItem(NewClipItem(TypeText, []byte("same")))

	if got := len(app.history.GetAll()); got != 3 {
		t.Fatalf("默认不应去重，期望 3 条，实际 %d", got)
	}
}

func TestToggleSearchRejectsEmptyHistory(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())

	app.toggleSearch()
	if app.searchEnable || app.searchText != "" || app.searchLabel != "" {
		t.Fatalf("空历史时点击搜索不应启用")
	}
}

func TestToggleSearchUsesImageHash(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	image := NewClipItem(TypeImage, []byte("image-bytes"))
	app.history.Add(image)

	app.toggleSearch()
	if !app.searchEnable {
		t.Fatalf("顶部是图片时应启用以图搜图")
	}
	if app.searchText != image.Hash {
		t.Fatalf("图片搜索词应为像素哈希: got %q want %q", app.searchText, image.Hash)
	}
	wantLabel := "图片 [" + image.Hash[:8] + "]"
	if app.searchLabel != wantLabel {
		t.Fatalf("图片搜索标签错误: got %q want %q", app.searchLabel, wantLabel)
	}

	app.toggleSearch()
	if app.searchEnable || app.searchText != "" || app.searchLabel != "" {
		t.Fatalf("再次点击应关闭搜索并清空关键词")
	}
}

func TestToggleSearchUsesTextTop(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	app.history.Add(NewClipItem(TypeText, []byte("keyword")))

	app.toggleSearch()
	if !app.searchEnable || app.searchText != "keyword" || app.searchLabel != "keyword" {
		t.Fatalf("应以顶部文本为搜索词: enable=%v text=%q label=%q", app.searchEnable, app.searchText, app.searchLabel)
	}

	app.toggleSearch()
	if app.searchEnable || app.searchText != "" || app.searchLabel != "" {
		t.Fatalf("再次点击应关闭搜索并清空关键词: enable=%v text=%q label=%q", app.searchEnable, app.searchText, app.searchLabel)
	}
}

func TestMatchSearch(t *testing.T) {
	app := NewApp(NewAppLogger())
	imageA := NewClipItem(TypeImage, []byte("image-a"))
	imageB := NewClipItem(TypeImage, []byte("image-b"))
	text := NewClipItem(TypeText, []byte("hello world"))

	if !app.matchSearch(imageA) || !app.matchSearch(text) {
		t.Fatalf("未启用搜索时应全部匹配")
	}

	app.searchEnable = true
	app.searchText = imageA.Hash
	if !app.matchSearch(imageA) {
		t.Fatalf("相同图片应匹配")
	}
	if app.matchSearch(imageB) {
		t.Fatalf("不同图片不应匹配")
	}
	if app.matchSearch(text) {
		t.Fatalf("图片哈希不应匹配普通文本")
	}

	app.searchText = imageA.Hash[:8]
	if !app.matchSearch(imageA) {
		t.Fatalf("哈希前缀应匹配相同图片")
	}

	app.searchText = "hello"
	if !app.matchSearch(text) {
		t.Fatalf("文本关键词应匹配文本")
	}
	if app.matchSearch(imageA) {
		t.Fatalf("文本关键词不应匹配图片")
	}
}

func TestCurrentVersionFallback(t *testing.T) {
	original := version
	defer func() { version = original }()

	version = ""
	if got := currentVersion(); got != "dev" {
		t.Fatalf("空版本应回退为 dev，got %q", got)
	}

	version = "v1.2.3"
	if got := currentVersion(); got != "v1.2.3" {
		t.Fatalf("应返回注入的版本，got %q", got)
	}
}

func TestAddItemToHistoryKeepLatestOnly(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	app.configKeepLatestOnly.Store(true)

	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("dup")))
	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("other")))
	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("dup")))

	all := app.history.GetAll()
	if len(all) != 2 {
		t.Fatalf("相同内容应只保留一条，实际 %d", len(all))
	}
	if string(all[0].Content) != "dup" || string(all[1].Content) != "other" {
		t.Fatalf("最新重复项应置顶: [%s, %s]", string(all[0].Content), string(all[1].Content))
	}
}

func TestFlushMenuTitleUsesKeepLatestOnly(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	app.configKeepLatestOnly.Store(true)
	app.writer = make(chan *ClipItem, 1)

	app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("dup")))

	app.translateMu.Lock()
	app.menuTitle = "dup"
	app.translateMu.Unlock()
	app.flushMenuTitle()

	all := app.history.GetAll()
	if len(all) != 1 {
		t.Fatalf("翻译回写应走去重，期望 1 条，实际 %d", len(all))
	}
	if string(all[0].Content) != "dup" {
		t.Fatalf("翻译回写内容错误: %s", string(all[0].Content))
	}

	select {
	case item := <-app.writer:
		if string(item.Content) != "dup" {
			t.Fatalf("写回剪贴板内容错误: %s", string(item.Content))
		}
	default:
		t.Fatalf("翻译回写应写入剪贴板")
	}
}

func TestConfigKeepLatestOnlyConcurrentAccess(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				app.configKeepLatestOnly.Store(j%2 == 0)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				app.addItemToHistory(app.history, NewClipItem(TypeText, []byte("x")))
			}
		}()
	}
	wg.Wait()
}

func TestRenameGroupKeepsOrderAndHistory(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	group := NewGroup("old", true, 10)
	group.History.Add(NewClipItem(TypeText, []byte("item")))
	app.groups["old"] = group
	app.groupNames = []string{"a", "old", "b"}

	if !app.renameGroup("old", "new") {
		t.Fatalf("重命名应成功")
	}
	if _, ok := app.groups["old"]; ok {
		t.Fatalf("旧分组名应被移除")
	}
	renamed, ok := app.groups["new"]
	if !ok || renamed.Name != "new" || renamed != group {
		t.Fatalf("新分组应指向同一对象且 Name 更新")
	}
	if got := renamed.History.GetTop(); got == nil || string(got.Content) != "item" {
		t.Fatalf("重命名后历史应保留")
	}

	want := []string{"a", "new", "b"}
	if len(app.groupNames) != len(want) {
		t.Fatalf("分组名数量错误: %v", app.groupNames)
	}
	for i := range want {
		if app.groupNames[i] != want[i] {
			t.Fatalf("分组名顺序错误: %v", app.groupNames)
		}
	}
}

func TestDeleteGroupRemovesOnlyTarget(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	app.groups["a"] = NewGroup("a", false, 10)
	app.groups["b"] = NewGroup("b", false, 10)
	app.groupNames = []string{"a", "b"}

	if !app.deleteGroup("a") {
		t.Fatalf("删除已存在分组应成功")
	}
	if _, ok := app.groups["a"]; ok {
		t.Fatalf("目标分组应被删除")
	}
	if _, ok := app.groups["b"]; !ok {
		t.Fatalf("其它分组不应受影响")
	}
	if len(app.groupNames) != 1 || app.groupNames[0] != "b" {
		t.Fatalf("分组名列表错误: %v", app.groupNames)
	}
	if app.deleteGroup("missing") {
		t.Fatalf("删除不存在分组应返回 false")
	}
}

func TestRenameImageItemUsesTopText(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	image := NewClipItem(TypeImage, []byte("image"))
	app.history.Add(image)
	app.history.Add(NewClipItem(TypeText, []byte("我的截图")))

	app.renameImageItem(app.history, image)

	var renamed *ClipItem
	for _, item := range app.history.GetAll() {
		if item.Type == TypeImage {
			renamed = item
		}
	}
	if renamed == nil || renamed.Name != "我的截图" {
		t.Fatalf("图片应以顶部文本重命名: %#v", renamed)
	}
	if renamed.Hash != image.Hash {
		t.Fatalf("重命名不应改变内容哈希")
	}
}

func TestRenameImageItemSkipsImageTop(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	target := NewClipItem(TypeImage, []byte("target"))
	app.history.Add(target)
	app.history.Add(NewClipItem(TypeImage, []byte("top-image")))

	app.renameImageItem(app.history, target)
	for _, item := range app.history.GetAll() {
		if item.Name != "" {
			t.Fatalf("顶部为图片时不应重命名: %#v", item)
		}
	}
}

func TestRenameImageItemRejectsEmptyHistory(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	app.renameImageItem(app.history, NewClipItem(TypeImage, []byte("target")))
}

func TestFormatMenuItemShowsImageName(t *testing.T) {
	item := NewClipItem(TypeImage, []byte("image"))
	plain := formatMenuItem(item)
	if !strings.Contains(plain, "图片 [") {
		t.Fatalf("未命名图片应显示默认标签: %s", plain)
	}

	item.Name = "截图"
	named := formatMenuItem(item)
	if !strings.Contains(named, "截图") {
		t.Fatalf("命名图片应显示名称: %s", named)
	}
	if got := formatMenuItemTooltip(item); got != "截图" {
		t.Fatalf("tooltip 应显示名称，got %q", got)
	}
}

func TestRenameImageItemLimitsNameLength(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	image := NewClipItem(TypeImage, []byte("image"))
	app.history.Add(image)

	longText := strings.Repeat("名", maxImageNameRunes+20)
	app.history.Add(NewClipItem(TypeText, []byte(longText)))

	app.renameImageItem(app.history, image)

	var renamed *ClipItem
	for _, item := range app.history.GetAll() {
		if item.Type == TypeImage {
			renamed = item
		}
	}
	if renamed == nil {
		t.Fatalf("图片项应存在")
	}
	want := strings.Repeat("名", maxImageNameRunes)
	if renamed.Name != want {
		t.Fatalf("名称应截断为 %d 个字符: got %q", maxImageNameRunes, renamed.Name)
	}
}

func TestAddItemToHistoryInheritsImageName(t *testing.T) {
	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	older := NewClipItem(TypeImage, []byte("same-image"))
	app.history.Add(older)
	app.history.Rename(older, "截图")

	app.addItemToHistory(app.history, NewClipItem(TypeImage, []byte("same-image")))
	for _, item := range app.history.GetAll() {
		if item.Name != "截图" {
			t.Fatalf("图片新项应继承名称: %#v", item)
		}
	}

	app.configKeepLatestOnly.Store(true)
	app.addItemToHistory(app.history, NewClipItem(TypeImage, []byte("same-image")))
	all := app.history.GetAll()
	if len(all) != 1 || all[0].Name != "截图" {
		t.Fatalf("去重后应保留继承的名称: %#v", all)
	}
}

func TestSaveLoadLocalStatePersistsImageName(t *testing.T) {
	tempDir := t.TempDir()
	legacyDir := filepath.Join(tempDir, "legacy")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "xdg"))
	legacyConfigPathOverride = filepath.Join(legacyDir, "config.json")
	defer func() { legacyConfigPathOverride = "" }()

	resetTestLogChannel()
	app := NewApp(NewAppLogger())
	image := NewClipItem(TypeImage, []byte("image"))
	app.history.Add(image)
	app.history.Add(NewClipItem(TypeText, []byte("我的截图")))
	app.renameImageItem(app.history, image)
	app.saveLocalState()

	reloaded := NewApp(NewAppLogger())
	reloaded.loadLocalState()

	var found *ClipItem
	for _, item := range reloaded.history.GetAll() {
		if item.Type == TypeImage {
			found = item
		}
	}
	if found == nil || found.Name != "我的截图" {
		t.Fatalf("重启后图片名称应持久化: %#v", found)
	}
}

func TestBuildItemMenuInfo(t *testing.T) {
	app := NewApp(NewAppLogger())
	plainText := NewClipItem(TypeText, []byte("hello"))
	colorText := NewClipItem(TypeText, []byte("#aabbcc"))
	image := NewClipItem(TypeImage, []byte("img"))

	if info := app.buildItemMenuInfo(plainText); info.hasSub {
		t.Fatalf("默认配置下不应有子菜单")
	}

	app.configSingleDelete = true
	if info := app.buildItemMenuInfo(plainText); !info.hasSub || !info.hasDelete {
		t.Fatalf("开启单独删除后所有条目应有子菜单")
	}
	app.configSingleDelete = false

	app.configAutoRecognizeColor = true
	if info := app.buildItemMenuInfo(colorText); !info.hasColor || !info.hasSub || info.colorHex != "#aabbcc" || info.colorRGB != "170,187,204" {
		t.Fatalf("颜色文本应命中颜色识别: %+v", info)
	}
	if info := app.buildItemMenuInfo(plainText); info.hasColor || info.hasSub {
		t.Fatalf("非颜色文本不应命中颜色识别")
	}
	if info := app.buildItemMenuInfo(image); info.hasColor {
		t.Fatalf("图片不应命中颜色识别")
	}
	app.configAutoRecognizeColor = false

	app.configImageRename = true
	if info := app.buildItemMenuInfo(image); !info.canRename || !info.hasSub {
		t.Fatalf("开启图片重命名后图片应有子菜单")
	}
	if info := app.buildItemMenuInfo(plainText); info.canRename {
		t.Fatalf("文本不应有重命名子菜单")
	}
}
