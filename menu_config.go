package main

import (
	"fmt"
	"strconv"
	"time"

	"clip/translator"
	"github.com/energye/systray"
)

func (a *App) addCleanHistoryMenuCmd() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`清空历史记录`菜单"}
	if a.clearState == Normal {
		menu := systray.AddMenuItem("清空历史记录", "【清空历史记录】会将历史记录清空，但是不会清空剪贴板中的内容")
		menu.Click(func() {
			a.clearState = ReadyToClear
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "准备清空历史记录，等待确认..."}
		})
	} else {
		menu := systray.AddMenuItem("确认/取消清空历史记录？", "")
		menuOk := menu.AddSubMenuItem("确认清空？", "")
		menuOk.Click(func() {
			a.clearState = Normal
			a.history.Clear()
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "历史记录已清空"}
		})
		menuCancel := menu.AddSubMenuItem("取消清空?", "")
		menuCancel.Click(func() {
			a.clearState = Normal
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "取消清空历史记录"}
		})
	}
}

func (a *App) addConfigMenuAction() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`配置`菜单"}

	menu := systray.AddMenuItem("配置", "")
	menu.AddSubMenuItemCheckbox("单独删除项", "", a.configSingleDelete).Click(func() {
		a.configSingleDelete = !a.configSingleDelete
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置单独删除项: %v", a.configSingleDelete)}
	})
	menu.AddSubMenuItemCheckbox("自动识别颜色", "", a.configAutoRecognizeColor).Click(func() {
		a.configAutoRecognizeColor = !a.configAutoRecognizeColor
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置自动识别颜色: %v", a.configAutoRecognizeColor)}
	})
	menu.AddSubMenuItem("设置最大历史记录条数"+fmt.Sprintf("(当前: %d)", a.configHistoryMax), "【设置最大历史记录条数】会设置历史记录的最大条数，超过最大条数会自动删除最早的记录，范围：1-300").Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "设置最大历史记录条数"}
		top := a.history.GetTop()
		if top == nil || top.Type != TypeText {
			return
		}

		text := string(top.Content)
		digit, err := strconv.ParseUint(text, 10, 0)
		if err != nil {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("设置最大历史记录条数失败: 无法解析数字: %s", text)}
			return
		}

		if digit > 300 || digit == 0 {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("设置最大历史记录条数失败: 数字超出范围: %d", digit)}
			return
		}

		a.configHistoryMax = uint(digit)
		a.history.SetMaxSize(a.configHistoryMax)
	})

	shareMenu := menu.AddSubMenuItem("局域网共享", "")
	{
		a.shareServerMu.RLock()
		server := a.shareServer
		serverLabel := ""
		if server != nil {
			serverLabel = fmt.Sprintf("(%v)", server.AddrString())
		}
		a.shareServerMu.RUnlock()

		shareMenu.AddSubMenuItemCheckbox("局域网共享"+serverLabel, "", server != nil).Click(func() {
			a.shareServerMu.RLock()
			currentServer := a.shareServer
			a.shareServerMu.RUnlock()

			global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(currentServer == nil, "启动局域网共享", "关闭局域网共享")}
			if currentServer == nil {
				newServer, err := NewShareServer()
				if err != nil {
					global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("启动局域网共享失败: %v", err)}
					return
				}
				a.shareServerMu.Lock()
				a.shareServer = newServer
				a.shareServerMu.Unlock()
				a.writer <- NewClipItem(TypeText, []byte(newServer.AddrString()))
				newServer.Start()
			} else {
				currentServer.Stop()
				a.shareServerMu.Lock()
				a.shareServer = nil
				a.shareServerMu.Unlock()
			}
		})

		shareMenu.AddSubMenuItem("连接到", "").Click(func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "连接到局域网共享"}
			top := a.history.GetTop()
			if top == nil || top.Type != TypeText {
				global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 历史记录为空，无法获取地址"}
				return
			}

			addr := string(top.Content)
			if addr == "" {
				global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 地址为空"}
				return
			}

			a.shareClientsMu.RLock()
			_, exists := a.shareClients[addr]
			a.shareClientsMu.RUnlock()
			if exists {
				global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 已经连接过了"}
				return
			}

			shareClient := NewShareClient(addr)
			if shareClient.ConnectTo() {
				a.shareClientsMu.Lock()
				a.shareClients[addr] = shareClient
				a.shareClientsMu.Unlock()
				shareClient.OnShared(func(item *ClipItem) {
					a.echoGuard.Mark(item, 3*time.Second)
					a.history.Add(item)
					a.writer <- item
				})
				shareClient.OnClose(func() {
					a.shareClientsMu.Lock()
					delete(a.shareClients, addr)
					a.shareClientsMu.Unlock()
				})
			}
		})

		a.shareClientsMu.RLock()
		clientSnapshot := make(map[string]*ShareClient, len(a.shareClients))
		for addr, client := range a.shareClients {
			clientSnapshot[addr] = client
		}
		a.shareClientsMu.RUnlock()

		for addr, client := range clientSnapshot {
			shareMenu.AddSubMenuItemCheckbox(addr, "", true).Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("断开与局域网共享%s的连接", addr)}
				client.Close()
				a.shareClientsMu.Lock()
				delete(a.shareClients, addr)
				a.shareClientsMu.Unlock()
			})
		}
	}

	transLateMenu := menu.AddSubMenuItemCheckbox("翻译", "", a.currentTranslator() != nil)
	{
		langMenu := transLateMenu.AddSubMenuItem("翻译为："+string(a.currentTranslateToLang()), "")
		{
			currentLang := a.currentTranslateToLang()
			for _, lang := range translator.TransLangFactory() {
				langMenu.AddSubMenuItemCheckbox(string(lang), "", currentLang == lang).Click(func() {
					a.setTranslateToLang(lang)
				})
			}
		}
		for _, translatorImp := range translator.TranslatorFactory() {
			transLateMenu.AddSubMenuItemCheckbox(translatorImp.Name(), "", a.currentTranslator() == translatorImp).Click(func() {
				if a.currentTranslator() == translatorImp && translatorImp.IsEnabled() {
					a.setTranslator(nil)
					return
				}

				top := a.history.GetTop()
				if top == nil || top.Type != TypeText {
					return
				}
				if translatorImp.IsEnabled() || translatorImp.Enable(string(top.Content)) {
					a.setTranslator(translatorImp)
				}
			})
		}
	}

	menu.AddSubMenuItem("打开配置文件目录", "").Click(func() {
		openDir(getAppDataDir())
	})
	menu.AddSubMenuItemCheckbox("退出时保存日志", "", a.configSaveLogToLocal).Click(func() {
		a.configSaveLogToLocal = !a.configSaveLogToLocal
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置退出时保存日志: %v", a.configSaveLogToLocal)}
	})
}

func (a *App) addSearchMenuAction() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`搜索`菜单"}
	systray.AddMenuItemCheckbox("🔎 搜索"+Ifel(a.searchEnable, ":"+a.searchText, ""), "【搜索】会使用剪贴板内的内容进行过滤，再次点击取消搜索", a.searchEnable).Click(func() {
		a.searchEnable = !a.searchEnable
		global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(a.searchEnable, "启用搜索", "禁用搜索")}
		if !a.searchEnable {
			a.searchText = ""
			return
		}

		top := a.history.GetTop()
		if top == nil {
			return
		}

		text := string(top.Content)
		if text == "" {
			return
		}

		a.searchText = text
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置搜索关键词: %s", a.searchText)}
	})
}
