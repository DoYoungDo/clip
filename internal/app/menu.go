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

func (a *App) addCopyMenuItem(menu *systray.MenuItem, item *ClipItem, logPrefix string) {
	copyItem := menu.AddSubMenuItem("复制", "")
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

// addItemSubMenu 构建右键二级菜单，顺序为：复制（最高优先级）→ 重命名（图片且开启）→ 删除（开启时）。
// 返回是否添加了任何子菜单。
func (a *App) addItemSubMenu(menu *systray.MenuItem, item *ClipItem, h *History, logPrefix string) bool {
	canRename := item.Type == TypeImage && a.configImageRename
	if !canRename && !a.configSingleDelete {
		return false
	}

	a.addCopyMenuItem(menu, item, logPrefix)
	if canRename {
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

		menu := systray.AddMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
		switch a.showMenuState {
		case Click:
			menu.Click(func() { a.writer <- item })
		case RClick:
			if a.addColorRecognizeMenuAction(menu, item) {
				if a.configSingleDelete {
					del := menu.AddSubMenuItem("删除", "")
					del.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除历史记录项: %s", formatMenuItem(item))}
						a.history.Delete(item)
					})
				}
			} else if !a.addItemSubMenu(menu, item, a.history, "历史记录项") {
				menu.Click(func() { a.writer <- item })
			}
		}
	}

	return len(all) > 0
}
