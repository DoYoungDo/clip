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

	translateMu     sync.Mutex
	translator      translator.Translator
	translateToLang translator.TransLang
	menuTitle       string
	translateSeq    uint64

	configHistoryMax         uint
	configKeepLatestOnly     bool
	configSingleDelete       bool
	configAutoRecognizeColor bool
	configSaveLogToLocal     bool

	searchEnable  bool
	searchText    string
	showMenuState ShowMenuState
	clearState    ClearState
}

func NewApp(logger *AppLogger) *App {
	app := &App{
		logger:           logger,
		groups:           make(map[string]*Group),
		groupNames:       []string{},
		echoGuard:        &echoSuppressor{},
		shareClients:     make(map[string]*ShareClient),
		configHistoryMax: const_max_history,
	}
	app.history = NewHistory(app.configHistoryMax)
	return app
}

func (a *App) currentTranslator() translator.Translator {
	a.translateMu.Lock()
	defer a.translateMu.Unlock()
	return a.translator
}

func (a *App) setTranslator(t translator.Translator) {
	a.translateMu.Lock()
	a.translator = t
	a.translateMu.Unlock()
}

func (a *App) currentTranslateToLang() translator.TransLang {
	a.translateMu.Lock()
	defer a.translateMu.Unlock()
	return a.translateToLang
}

func (a *App) setTranslateToLang(lang translator.TransLang) {
	a.translateMu.Lock()
	a.translateToLang = lang
	a.translateMu.Unlock()
}

func (a *App) takeMenuTitle() string {
	a.translateMu.Lock()
	defer a.translateMu.Unlock()
	title := a.menuTitle
	a.menuTitle = ""
	return title
}

func (a *App) translateItem(item *ClipItem) {
	a.translateMu.Lock()
	t := a.translator
	lang := a.translateToLang
	a.translateSeq++
	seq := a.translateSeq
	a.translateMu.Unlock()

	if t == nil {
		return
	}

	go func() {
		translatedText, err := t.Translate(string(item.Content), lang)
		if err != nil {
			return
		}

		a.translateMu.Lock()
		if seq != a.translateSeq {
			a.translateMu.Unlock()
			return
		}
		a.menuTitle = translatedText
		a.translateMu.Unlock()

		systray.SetTitle(formatMenuTitle(fmt.Sprintf("%v: %v", t.Name(), translatedText)))
	}()
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

	a.configHistoryMax = localConfig.HistoryMax
	a.configKeepLatestOnly = localConfig.KeepLatestOnly
	a.configSingleDelete = localConfig.SingleDelete
	a.configAutoRecognizeColor = localConfig.AutoRecognizeColor
	a.configSaveLogToLocal = localConfig.SaveLogToLocal

	a.history.SetMaxSize(a.configHistoryMax)

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
			a.setTranslateToLang(translator.TransLang(localConfig.Translator.Lang))
			currentTranslatorID := ""
			if localConfig.Translator.CurrentTranslatorId != nil {
				currentTranslatorID = *localConfig.Translator.CurrentTranslatorId
			}

			translators := translator.TranslatorFactory()
			translatorsMap := make(map[string]translator.Translator)
			for _, t := range translators {
				translatorsMap[t.Id()] = t
			}
			a.setTranslator(nil)

			for _, inited := range localConfig.Translator.InitedTranslators {
				if t, ok := translatorsMap[inited.Id]; ok {
					secret, err := resolveTranslatorSecret(inited.Id, translatorSecretStore)
					if err != nil {
						global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("加载翻译器 %s 的密钥失败: %v", t.Name(), err)}
						continue
					}
					if t.Enable(secret) {
						if currentTranslatorID == t.Id() {
							a.setTranslator(t)
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
	config.HistoryMax = a.configHistoryMax
	config.KeepLatestOnly = a.configKeepLatestOnly
	config.SingleDelete = a.configSingleDelete
	config.AutoRecognizeColor = a.configAutoRecognizeColor
	config.SaveLogToLocal = a.configSaveLogToLocal
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
			Lang: string(a.currentTranslateToLang()),
		}
		if t := a.currentTranslator(); t != nil {
			id := t.Id()
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

	var succ bool
	if a.configKeepLatestOnly {
		succ = a.history.AddKeepLatest(item)
	} else {
		succ = a.history.Add(item)
	}
	if succ {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("新剪贴板内容: %s", formatMenuItem(item))}

		if item.Type == TypeText {
			a.translateItem(item)
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
		if a.configKeepLatestOnly {
			group.History.AddKeepLatest(item.Clone())
		} else {
			group.History.Add(item.Clone())
		}
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
	a.logger.FlushToFile(a.configSaveLogToLocal)
}
