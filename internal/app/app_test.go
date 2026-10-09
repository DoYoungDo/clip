package app

import (
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
	app.configKeepLatestOnly = true
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
