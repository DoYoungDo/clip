package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"clip/internal/translator"
	"github.com/energye/systray"
)

func (a *App) addCleanHistoryMenuCmd() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`清空历史记录`菜单"}
	if a.clearState == ReadyToClear {
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
		return
	}

	tooltip := "【清空历史记录】会将历史记录清空，但是不会清空剪贴板中的内容"
	if !a.configConfirmClearHistory.Load() {
		tooltip += "，已关闭询问，点击后直接清空"
	}
	menu := systray.AddMenuItem("清空历史记录", tooltip)
	menu.Click(func() {
		if !a.configConfirmClearHistory.Load() {
			a.history.Clear()
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "已关闭询问，直接清空历史记录"}
			return
		}
		a.clearState = ReadyToClear
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "准备清空历史记录，等待确认..."}
	})
}

func (a *App) addConfigMenuAction() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`配置`菜单"}

	menu := systray.AddMenuItem("配置", "")
	menu.AddSubMenuItem("设置最大历史记录条数"+fmt.Sprintf("(当前: %d/%d)", len(a.history.GetAll()), a.configHistoryMax.Load()), "【设置最大历史记录条数】会设置历史记录的最大条数，超过最大条数会自动删除最早的记录，范围：1-300").Click(func() {
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

		a.configHistoryMax.Store(uint32(digit))
		a.history.SetMaxSize(uint(a.configHistoryMax.Load()))
		a.requestSave()
	})
	menu.AddSubMenuItem("设置搜索高亮标记"+fmt.Sprintf("(当前: %s%s)", a.configHighlightPrefix.Load(), a.configHighlightSuffix.Load()), "【设置搜索高亮标记】复制\"前缀 后缀\"两个标记（空格分隔）后点击，设置搜索命中内容在菜单中的包裹标记").Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "设置搜索高亮标记"}
		top := a.history.GetTop()
		if top == nil || top.Type != TypeText {
			return
		}

		parts := strings.Fields(string(top.Content))
		if len(parts) != 2 {
			global_log_channel <- LogEntry{Kind: KindError, Content: "设置搜索高亮标记失败: 请复制\"前缀 后缀\"格式，两个标记用空格分隔"}
			return
		}

		a.configHighlightPrefix.Store(parts[0])
		a.configHighlightSuffix.Store(parts[1])
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置搜索高亮标记: %s %s", a.configHighlightPrefix.Load(), a.configHighlightSuffix.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItem("设置时间格式"+fmt.Sprintf("(当前: %s)", a.configTimeFormat.Load()), "【设置时间格式】复制时间格式后点击，支持 YYYY YY MM DD HH mm ss 任意组合，如 YYYY-MM-DD HH:mm:ss").Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "设置时间格式"}
		top := a.history.GetTop()
		if top == nil || top.Type != TypeText {
			return
		}

		format := strings.TrimSpace(string(top.Content))
		if format == "" || strings.ContainsAny(format, "\n\r") {
			global_log_channel <- LogEntry{Kind: KindError, Content: "设置时间格式失败: 请复制单行的时间格式，如 YYYY-MM-DD HH:mm:ss"}
			return
		}

		a.configTimeFormat.Store(format)
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置时间格式: %s", a.configTimeFormat.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("清空历史时不询问", "【清空历史时不询问】开启后点击清空历史记录直接清空，关闭时需二次确认", !a.configConfirmClearHistory.Load()).Click(func() {
		a.configConfirmClearHistory.Store(!a.configConfirmClearHistory.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置清空历史时不询问: %v", !a.configConfirmClearHistory.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("单独删除项", "开启后右键历史记录项可单独删除该条记录", a.configSingleDelete.Load()).Click(func() {
		a.configSingleDelete.Store(!a.configSingleDelete.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置单独删除项: %v", a.configSingleDelete.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("自动识别颜色", "开启后自动识别文本中的颜色值，右键条目可复制 Hex/RGB", a.configAutoRecognizeColor.Load()).Click(func() {
		a.configAutoRecognizeColor.Store(!a.configAutoRecognizeColor.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置自动识别颜色: %v", a.configAutoRecognizeColor.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("相同内容只保留最新", "开启后相同内容的剪贴板记录只保留最近一条", a.configKeepLatestOnly.Load()).Click(func() {
		a.configKeepLatestOnly.Store(!a.configKeepLatestOnly.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置相同内容只保留最新: %v", a.configKeepLatestOnly.Load())}
		if a.configKeepLatestOnly.Load() {
			removed := a.history.RemoveDuplicates()
			a.groupsMu.RLock()
			for _, group := range a.groups {
				removed += group.History.RemoveDuplicates()
			}
			a.groupsMu.RUnlock()
			global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("相同内容只保留最新已开启，清理了 %d 条重复记录", removed)}
		}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("图片重命名", "开启后可在右键菜单为图片命名，名称取自最新剪贴板文本", a.configImageRename.Load()).Click(func() {
		a.configImageRename.Store(!a.configImageRename.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置图片重命名: %v", a.configImageRename.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("置顶【搜索】菜单", "开启后【搜索】菜单显示在菜单最顶部，关闭时保持默认位置", a.configSearchMenuTop.Load()).Click(func() {
		a.configSearchMenuTop.Store(!a.configSearchMenuTop.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置置顶【搜索】菜单: %v", a.configSearchMenuTop.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("添加到分组", "开启后右键历史记录可将其添加到指定分组", a.configAddToGroup.Load()).Click(func() {
		a.configAddToGroup.Store(!a.configAddToGroup.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置添加到分组: %v", a.configAddToGroup.Load())}
		a.requestSave()
	})
	menu.AddSubMenuItemCheckbox("进制转换", "开启后自动识别数字，右键条目可转换二/八/十/十六进制（支持 0b/0o/0x 前缀，无前缀数字按十进制）", a.configBaseConvert.Load()).Click(func() {
		a.configBaseConvert.Store(!a.configBaseConvert.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置进制转换: %v", a.configBaseConvert.Load())}
		a.requestSave()
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

		shareMenu.AddSubMenuItemCheckbox("局域网共享"+serverLabel, "开启后本机作为服务端共享剪贴板，关闭后停止共享", server != nil).Click(func() {
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

		shareMenu.AddSubMenuItem("连接到", "复制对方地址（ip:端口）后点击，连接对方的剪贴板共享").Click(func() {
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
					a.addItemToHistory(a.history, item)
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
			shareMenu.AddSubMenuItemCheckbox(addr, "点击断开与该地址的共享连接", true).Click(func() {
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
				langMenu.AddSubMenuItemCheckbox(string(lang), "将剪贴板文本翻译为该语言", currentLang == lang).Click(func() {
					a.setTranslateToLang(lang)
					a.requestSave()
				})
			}
		}
		{
			enabledTranslators := make([]translator.Translator, 0)
			for _, translatorImp := range translator.TranslatorFactory() {
				if translatorImp.IsEnabled() {
					enabledTranslators = append(enabledTranslators, translatorImp)
				}
			}
			if len(enabledTranslators) > 0 {
				deleteSecretMenu := transLateMenu.AddSubMenuItem("删除密钥", "")
				for _, translatorImp := range enabledTranslators {
					deleteSecretMenu.AddSubMenuItem(translatorImp.Name(), fmt.Sprintf("【删除密钥】会删除 %s 保存在系统钥匙串中的密钥，删除后需重新配置", translatorImp.Name())).Click(func() {
						if err := translatorSecretStore.Delete(translatorImp.Id()); err != nil {
							global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("删除翻译器 %s 的密钥失败: %v", translatorImp.Name(), err)}
							return
						}
						if a.currentTranslator() == translatorImp {
							a.setTranslator(nil)
						}
						translatorImp.Disable()
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("已删除翻译器 %s 的密钥", translatorImp.Name())}
						a.requestSave()
					})
				}
			}
		}
		for _, translatorImp := range translator.TranslatorFactory() {
			translatorName := translatorImp.Name()
			if translatorImp.IsEnabled() {
				translatorName = fmt.Sprintf("%v（已配置密钥）", translatorName)
			}
			transLateMenu.AddSubMenuItemCheckbox(translatorName, "点击启用或停用该翻译器；未配置密钥时使用最新剪贴板文本作为密钥", a.currentTranslator() == translatorImp).Click(func() {
				if a.currentTranslator() == translatorImp && translatorImp.IsEnabled() {
					a.setTranslator(nil)
					a.requestSave()
					return
				}

				top := a.history.GetTop()
				if top == nil || top.Type != TypeText {
					return
				}
				if translatorImp.IsEnabled() || translatorImp.Enable(string(top.Content)) {
					a.setTranslator(translatorImp)
					a.requestSave()
				}
			})
		}
	}

	menu.AddSubMenuItem("打开配置文件目录", "打开配置文件和日志所在的系统目录").Click(func() {
		openDir(getAppDataDir())
	})
	menu.AddSubMenuItemCheckbox("退出时保存日志", "开启后退出程序时将运行日志保存到配置目录下的 clip.log", a.configSaveLogToLocal.Load()).Click(func() {
		a.configSaveLogToLocal.Store(!a.configSaveLogToLocal.Load())
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置退出时保存日志: %v", a.configSaveLogToLocal.Load())}
		a.requestSave()
	})
}

func (a *App) toggleSearch() {
	if a.searchEnable {
		a.searchEnable = false
		a.searchText = ""
		a.searchLabel = ""
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "禁用搜索"}
		return
	}

	top := a.history.GetTop()
	if top == nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: "搜索失败: 历史记录为空"}
		return
	}

	switch top.Type {
	case TypeText:
		text := string(top.Content)
		if text == "" {
			global_log_channel <- LogEntry{Kind: KindError, Content: "搜索失败: 最新的历史记录不是有效的文本"}
			return
		}
		a.searchText = text
		a.searchLabel = truncateString(text, 16)
	case TypeImage:
		a.searchText = top.Hash
		prefix := top.Hash
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		a.searchLabel = fmt.Sprintf("图片 [%s]", prefix)
	}

	a.searchEnable = true
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "启用搜索"}
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置搜索关键词: %s", a.searchLabel)}
}

func (a *App) addSearchMenuAction() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`搜索`菜单"}
	systray.AddMenuItemCheckbox("🔎 搜索"+Ifel(a.searchEnable, ":"+a.searchLabel, ""), "【搜索】使用剪贴板内容过滤历史（图片按相同图片匹配），再次点击取消搜索", a.searchEnable).Click(func() {
		a.toggleSearch()
	})
}
