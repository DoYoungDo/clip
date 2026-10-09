package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/energye/systray"
)

func (a *App) onTrayReady() {
	systray.SetIcon(logo)
	systray.SetTooltip("Clip")

	systray.SetOnClick(a.onTrayClick)
	systray.SetOnRClick(a.onTrayRClick)
}

func (a *App) onTrayClick(menu systray.IMenu) {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "点击托盘图标"}
	a.showMenuState = Click
	systray.ResetMenu()
	a.flushMenuTitle()

	if a.addHistoryMenuAction() {
		addSeparator()
	}
	a.addGroupMenuAction()

	global_log_channel <- LogEntry{Kind: KindInfo, Content: "显示菜单"}
	menu.ShowMenu()
}

func (a *App) onTrayRClick(menu systray.IMenu) {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "右键点击托盘图标"}
	a.showMenuState = RClick
	systray.ResetMenu()
	a.flushMenuTitle()

	if a.addHistoryMenuAction() {
		addSeparator()
	}
	a.addCleanHistoryMenuCmd()
	addSeparator()
	if a.addGroupMenuAction() {
		addSeparator()
	}
	a.addCreateGroupMenuCmd()
	addSeparator()
	a.addSearchMenuAction()
	addSeparator()
	a.addConfigMenuAction()
	addSeparator()
	addQuitMenuCmd()

	global_log_channel <- LogEntry{Kind: KindInfo, Content: "显示菜单"}
	menu.ShowMenu()
}

func (a *App) flushMenuTitle() {
	title := a.takeMenuTitle()
	if title == "" {
		return
	}

	translatedItem := NewClipItem(TypeText, []byte(title))
	a.echoGuard.Mark(translatedItem, 3*time.Second)

	a.history.Add(translatedItem)
	a.writer <- translatedItem

	systray.SetTitle("")
}

func addSeparator() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加分隔线"}
	systray.AddSeparator()
}

func addQuitMenuCmd() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加退出菜单"}
	mQuit := systray.AddMenuItem("退出", "退出程序")
	mQuit.Click(func() {
		systray.Quit()
	})
}

func (a *App) addColorRecognizeMenuAction(menu *systray.MenuItem, item *ClipItem) bool {
	if !a.configAutoRecognizeColor || item.Type != TypeText {
		return false
	}

	r, g, b, base, ok := getColor(string(item.Content))
	if !ok {
		return false
	}

	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("识别颜色成功: %s,并添加菜单", string(item.Content))}
	rt, _ := strconv.ParseInt(r, base, 0)
	gt, _ := strconv.ParseInt(g, base, 0)
	bt, _ := strconv.ParseInt(b, base, 0)
	hexT := fmt.Sprintf("#%x%x%x", rt, gt, bt)
	rgbT := fmt.Sprintf("%d,%d,%d", rt, gt, bt)

	copyH := menu.AddSubMenuItem("复制Hex", "")
	copyRGB := menu.AddSubMenuItem("复制RGB", "")
	copyH.Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制Hex颜色: %s", hexT)}
		a.writer <- NewClipItem(TypeText, []byte(hexT))
	})
	copyRGB.Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制RGB颜色: %s", rgbT)}
		a.writer <- NewClipItem(TypeText, []byte(rgbT))
	})
	return true
}

func (a *App) addHistoryMenuAction() bool {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加历史记录项"}
	all := a.history.GetAll()
	for _, item := range all {
		if a.searchEnable && !strings.Contains(string(item.Content), a.searchText) {
			continue
		}

		menu := systray.AddMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
		switch a.showMenuState {
		case Click:
			if !a.addColorRecognizeMenuAction(menu, item) {
				menu.Click(func() { a.writer <- item })
			}
		case RClick:
			if a.addColorRecognizeMenuAction(menu, item) {
				if a.configSingleDelete {
					del := menu.AddSubMenuItem("删除", "")
					del.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除历史记录项: %s", formatMenuItem(item))}
						a.history.Delete(item)
					})
				}
			} else {
				if a.configSingleDelete {
					copy := menu.AddSubMenuItem("复制", "")
					del := menu.AddSubMenuItem("删除", "")
					copy.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制历史记录项: %s", formatMenuItem(item))}
						a.writer <- item
					})
					del.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除历史记录项: %s", formatMenuItem(item))}
						a.history.Delete(item)
					})
				} else {
					menu.Click(func() { a.writer <- item })
				}
			}
		}
	}

	return len(all) > 0
}
