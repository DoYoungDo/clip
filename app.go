package main

import (
	"fmt"
	"sync"

	"clip/translator"
	"github.com/energye/systray"
)

type App struct {
	logger *AppLogger

	history    *History
	groups     map[string]*Group
	groupNames []string
	groupsMu   sync.RWMutex

	echoGuard *echoSuppressor
	monitor   *clipboardMonitor
	writer    chan *ClipItem

	shareServer    *ShareServer
	shareClients   map[string]*ShareClient
	shareServerMu  sync.RWMutex
	shareClientsMu sync.RWMutex
}

func NewApp(logger *AppLogger) *App {
	return &App{
		logger:       logger,
		history:      NewHistory(config_history_max),
		groups:       make(map[string]*Group),
		groupNames:   []string{},
		echoGuard:    &echoSuppressor{},
		shareClients: make(map[string]*ShareClient),
	}
}

func (a *App) loadLocalState() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在加载配置和历史记录..."}

	localConfig, source, err := loadMergedConfig()
	if err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("加载配置失败: %v", err)}
		localConfig = NewDefaultConfig()
	} else {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("配置加载来源: %s", source)}
	}

	config_history_max = localConfig.HistoryMax
	config_single_delete = localConfig.SingleDelete
	config_auto_recognize_color = localConfig.AutoRecognizeColor
	config_save_log_to_local = localConfig.SaveLogToLocal

	a.history.SetMaxSize(config_history_max)

	if localConfig.Data.History != nil {
		a.history.items = localConfig.Data.History
	}
	if localConfig.Data.Groups != nil {
		for name, groupData := range localConfig.Data.Groups {
			a.groups[name] = NewGroup(name, groupData.Active, const_max_history)
			if groupData.History != nil {
				a.groups[name].History.items = groupData.History
			}
		}
	}

	a.groupNames = localConfig.Data.GroupNames

	{
		if localConfig.Translator != nil {
			global_translate_to_lang = translator.TransLang(localConfig.Translator.Lang)
			currentTranslatorID := ""
			if localConfig.Translator.CurrentTranslatorId != nil {
				currentTranslatorID = *localConfig.Translator.CurrentTranslatorId
			}

			translators := translator.TranslatorFactory()
			translatorsMap := make(map[string]translator.Translator)
			for _, t := range translators {
				translatorsMap[t.Id()] = t
			}
			global_translator = nil

			for _, inited := range localConfig.Translator.InitedTranslators {
				if t, ok := translatorsMap[inited.Id]; ok {
					secret, err := resolveTranslatorSecret(inited.Id, translatorSecretStore)
					if err != nil {
						global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("加载翻译器 %s 的密钥失败: %v", t.Name(), err)}
						continue
					}
					if t.Enable(secret) {
						if currentTranslatorID == t.Id() {
							global_translator = t
						}
						continue
					}
					global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("启用翻译器 %s 失败", t.Name())}
				}
			}
		}
	}
}

func (a *App) saveLocalState() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在保存配置和历史记录..."}

	config := NewDefaultConfig()
	config.HistoryMax = config_history_max
	config.SingleDelete = config_single_delete
	config.AutoRecognizeColor = config_auto_recognize_color
	config.SaveLogToLocal = config_save_log_to_local
	config.Data.History = a.history.GetAll()

	a.groupsMu.RLock()
	for name, group := range a.groups {
		config.Data.Groups[name] = HistoryGroupData{
			Active:  group.Active,
			History: group.History.GetAll(),
		}
	}
	config.Data.GroupNames = append([]string(nil), a.groupNames...)
	a.groupsMu.RUnlock()

	{
		config.Translator = &TranslatorData{
			Lang: string(global_translate_to_lang),
		}
		if global_translator != nil {
			id := global_translator.Id()
			config.Translator.CurrentTranslatorId = &id
		}

		translators := translator.TranslatorFactory()
		inited, err := buildTranslatorInitData(translators, translatorSecretStore)
		if err != nil {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("保存翻译器密钥失败: %v", err)}
		}
		config.Translator.InitedTranslators = inited
	}

	if err := saveConfigToPath(getConfigPath(), config); err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("保存配置失败: %v", err)}
	}
}

func (a *App) consumeClipboardItems() {
	for {
		select {
		case <-a.monitor.Done():
			return
		case item := <-a.monitor.reader:
			a.handleClipboardItem(item)
		}
	}
}

func (a *App) handleClipboardItem(item *ClipItem) {
	if item == nil {
		return
	}
	if item.From == FromLocal && a.echoGuard.ShouldSuppress(item) {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("忽略回声剪贴板内容: %s", formatMenuItem(item))}
		return
	}

	succ := a.history.Add(item)
	if succ {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("新剪贴板内容: %s", formatMenuItem(item))}

		if item.Type == TypeText && global_translator != nil {
			translatedText, err := global_translator.Translate(string(item.Content), global_translate_to_lang)
			if err == nil {
				global_menu_title = translatedText
				systray.SetTitle(formatMenuTitle(fmt.Sprintf("%v: %v", global_translator.Name(), global_menu_title)))
			}
		}
	}

	a.groupsMu.RLock()
	activeGroups := make([]*Group, 0, len(a.groups))
	for _, group := range a.groups {
		if group.Active {
			activeGroups = append(activeGroups, group)
		}
	}
	a.groupsMu.RUnlock()

	for _, group := range activeGroups {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加到分组 %s", group.Name)}
		group.History.Add(item.Clone())
	}

	a.shareServerMu.RLock()
	server := a.shareServer
	a.shareServerMu.RUnlock()
	if succ && server != nil {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "共享到局域网"}
		server.Share(item.CloneToRemote())
	}
}

func (a *App) shutdown() {
	a.shareClientsMu.RLock()
	clients := make([]*ShareClient, 0, len(a.shareClients))
	for _, client := range a.shareClients {
		clients = append(clients, client)
	}
	a.shareClientsMu.RUnlock()

	for _, client := range clients {
		client.Close()
	}

	a.shareServerMu.RLock()
	server := a.shareServer
	a.shareServerMu.RUnlock()
	if server != nil {
		server.Stop()
		a.shareServerMu.Lock()
		a.shareServer = nil
		a.shareServerMu.Unlock()
	}

	a.monitor.Close()
	a.saveLocalState()
	a.logger.FlushToFile(config_save_log_to_local)
}
