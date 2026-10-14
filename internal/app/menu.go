package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/energye/systray"
)

func (a *App) onTrayReady() {
	systray.SetTemplateIcon(logoTemplate, logo)
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
	if a.addGroupMenuAction() {
		addSeparator()
	}
	a.addSearchMenuAction()

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
	addAboutMenuCmd()
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

	a.addItemToHistory(a.history, translatedItem)
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

func addAboutMenuCmd() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`关于`菜单"}
	menu := systray.AddMenuItem("关于", "")
	menu.AddSubMenuItem("名称: "+appName, "")
	menu.AddSubMenuItem("版本: "+currentVersion(), "")
	menu.AddSubMenuItem("协议: "+appLicense, "")
	menu.AddSubMenuItem("作者: "+appAuthor, "").Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "打开作者主页: " + appAuthorURL}
		if err := openURL(appAuthorURL); err != nil {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("打开作者主页失败: %v", err)}
		}
	})
	menu.AddSubMenuItem("源码仓库", "").Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "打开源码仓库: " + appRepoURL}
		if err := openURL(appRepoURL); err != nil {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("打开源码仓库失败: %v", err)}
		}
	})
}

// isColorRecognizable 判断文本项是否命中颜色识别。
func (a *App) isColorRecognizable(item *ClipItem) bool {
	if !a.configAutoRecognizeColor || item.Type != TypeText {
		return false
	}
	_, _, _, _, ok := getColor(string(item.Content))
	return ok
}

// addColorRecognizeMenuItems 为可识别颜色的文本项添加"复制Hex / 复制RGB"子菜单。
func (a *App) addColorRecognizeMenuItems(menu *systray.MenuItem, item *ClipItem) {
	r, g, b, base, ok := getColor(string(item.Content))
	if !ok {
		return
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
}

func (a *App) addCopyMenuItem(menu *systray.MenuItem, item *ClipItem, logPrefix string, tooltip string) {
	copyItem := menu.AddSubMenuItem("复制", tooltip)
	copyItem.Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制%s: %s", logPrefix, formatMenuItem(item))}
		a.writer <- item
	})
}

func (a *App) addDeleteMenuItem(menu *systray.MenuItem, item *ClipItem, logPrefix string, deleteItem func()) {
	del := menu.AddSubMenuItem("删除", "")
	del.Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除%s: %s", logPrefix, formatMenuItem(item))}
		deleteItem()
	})
}

// hasItemSubMenu 判断右键时该条目是否会构建二级菜单（颜色识别/重命名/删除任一开启）。
func (a *App) hasItemSubMenu(item *ClipItem) bool {
	if a.configSingleDelete {
		return true
	}
	if item.Type == TypeImage && a.configImageRename {
		return true
	}
	return a.isColorRecognizable(item)
}

// addItemSubMenu 构建右键二级菜单，顺序为：复制（最高优先级，承载悬浮提示）→ 颜色识别 → 重命名 → 删除。
// 返回是否添加了任何子菜单。
func (a *App) addItemSubMenu(menu *systray.MenuItem, item *ClipItem, h *History, logPrefix string) bool {
	if !a.hasItemSubMenu(item) {
		return false
	}

	a.addCopyMenuItem(menu, item, logPrefix, formatMenuItemTooltip(item))
	if a.isColorRecognizable(item) {
		a.addColorRecognizeMenuItems(menu, item)
	}
	if item.Type == TypeImage && a.configImageRename {
		rename := menu.AddSubMenuItem("重命名", "使用最新剪贴板内容作为新名称")
		rename.Click(func() {
			a.renameImageItem(h, item)
		})
	}
	if a.configSingleDelete {
		a.addDeleteMenuItem(menu, item, logPrefix, func() { h.Delete(item) })
	}
	return true
}

// maxImageNameRunes 限制图片名称的最大字符数。
const maxImageNameRunes = 40

// limitRunes 将字符串截断到最多 maxLen 个字符（按 rune 计）。
func limitRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// renameImageItem 用最新一条文本历史作为新名称重命名图片；最新记录非文本时跳过。
func (a *App) renameImageItem(h *History, item *ClipItem) {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("重命名图片: %s", formatMenuItem(item))}

	top := a.history.GetTop()
	if top == nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: "重命名失败: 历史记录为空，无法获取新名称"}
		return
	}
	if top.Type != TypeText {
		global_log_channel <- LogEntry{Kind: KindError, Content: "重命名失败: 最新的历史记录不是文本，无法作为新名称"}
		return
	}

	newName := limitRunes(strings.TrimSpace(string(top.Content)), maxImageNameRunes)
	if newName == "" {
		global_log_channel <- LogEntry{Kind: KindError, Content: "重命名失败: 新名称为空"}
		return
	}

	if !h.Rename(item, newName) {
		global_log_channel <- LogEntry{Kind: KindError, Content: "重命名失败: 目标项已不在历史记录中"}
		return
	}
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("图片重命名成功: %s", newName)}
}

func (a *App) matchSearch(item *ClipItem) bool {
	if !a.searchEnable {
		return true
	}
	if item.Type == TypeImage {
		return strings.Contains(item.Hash, a.searchText)
	}
	return strings.Contains(string(item.Content), a.searchText)
}

func (a *App) addHistoryMenuAction() bool {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加历史记录项"}
	all := a.history.GetAll()
	for _, item := range all {
		if !a.matchSearch(item) {
			continue
		}

		tooltip := formatMenuItemTooltip(item)
		if a.showMenuState == RClick && a.hasItemSubMenu(item) {
			tooltip = ""
		}
		menu := systray.AddMenuItem(formatMenuItem(item), tooltip)
		switch a.showMenuState {
		case Click:
			menu.Click(func() { a.writer <- item })
		case RClick:
			if !a.addItemSubMenu(menu, item, a.history, "历史记录项") {
				menu.Click(func() { a.writer <- item })
			}
		}
	}

	return len(all) > 0
}
