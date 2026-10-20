package app

import (
	"fmt"
	"sync"
	"sync/atomic"

	"clip/internal/translator"
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

	configHistoryMax          atomic.Uint32
	configKeepLatestOnly      atomic.Bool
	configConfirmClearHistory atomic.Bool
	configSingleDelete        atomic.Bool
	configSearchMenuTop       atomic.Bool
	configImageRename         atomic.Bool
	configMoveToGroup         atomic.Bool
	configAutoRecognizeColor  atomic.Bool
	configSaveLogToLocal      atomic.Bool

	configHighlightPrefix atomicString
	configHighlightSuffix atomicString
	configTimeFormat      atomicString

	saveTrigger chan struct{}
	saveStop    chan struct{}
	saveDone    chan struct{}
	saveMu      sync.Mutex

	searchEnable  bool
	searchText    string
	searchLabel   string
	showMenuState ShowMenuState
	clearState    ClearState
}

func NewApp(logger *AppLogger) *App {
	app := &App{
		logger:       logger,
		groups:       make(map[string]*Group),
		groupNames:   []string{},
		echoGuard:    &echoSuppressor{},
		shareClients: make(map[string]*ShareClient),
		saveTrigger:  make(chan struct{}, 1),
	}
	app.configHistoryMax.Store(uint32(const_max_history))
	app.configHighlightPrefix.Store(defaultHighlightPrefix)
	app.configHighlightSuffix.Store(defaultHighlightSuffix)
	app.configTimeFormat.Store(defaultTimeFormat)
	app.history = NewHistory(uint(app.configHistoryMax.Load()))
	app.history.SetOnChange(app.requestSave)
	return app
}

// addItemToHistory 按当前配置写入历史：开启"相同内容只保留最新"时先移除旧的同内容项；
// 图片无名称时按哈希继承历史中已保存的名称（覆盖点击复制、局域网接收等路径）。
func (a *App) addItemToHistory(h *History, item *ClipItem) bool {
	if item != nil && item.Type == TypeImage && item.Name == "" {
		if name := a.history.NameByHash(item.Hash); name != "" {
			item.Name = name
		}
	}
	if a.configKeepLatestOnly.Load() {
		return h.AddKeepLatest(item)
	}
	return h.Add(item)
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

	a.configHistoryMax.Store(uint32(localConfig.HistoryMax))
	a.configKeepLatestOnly.Store(localConfig.KeepLatestOnly)
	a.configConfirmClearHistory.Store(localConfig.ConfirmClearHistory)
	a.configSingleDelete.Store(localConfig.SingleDelete)
	a.configSearchMenuTop.Store(localConfig.SearchMenuTop)
	a.configImageRename.Store(localConfig.ImageRename)
	a.configMoveToGroup.Store(localConfig.MoveToGroup)
	a.configAutoRecognizeColor.Store(localConfig.AutoRecognizeColor)
	a.configSaveLogToLocal.Store(localConfig.SaveLogToLocal)
	if localConfig.HighlightPrefix != "" {
		a.configHighlightPrefix.Store(localConfig.HighlightPrefix)
	}
	if localConfig.HighlightSuffix != "" {
		a.configHighlightSuffix.Store(localConfig.HighlightSuffix)
	}
	if localConfig.TimeFormat != "" {
		a.configTimeFormat.Store(localConfig.TimeFormat)
	}

	a.history.SetMaxSize(uint(a.configHistoryMax.Load()))

	if localConfig.Data.History != nil {
		a.history.items = localConfig.Data.History
	}
	if localConfig.Data.Groups != nil {
		for name, groupData := range localConfig.Data.Groups {
			group := NewGroup(name, groupData.Active, const_max_history)
			group.History.SetOnChange(a.requestSave)
			if groupData.History != nil {
				group.History.items = groupData.History
			}
			a.groups[name] = group
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
	a.saveStateToDisk()
}

// saveLocalStateSilent 静默保存，供实时自动落盘使用，避免日志刷屏；保存失败仍会记录错误日志。
func (a *App) saveLocalStateSilent() {
	a.saveStateToDisk()
}

func (a *App) saveStateToDisk() {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()

	config := NewDefaultConfig()
	config.HistoryMax = uint(a.configHistoryMax.Load())
	config.KeepLatestOnly = a.configKeepLatestOnly.Load()
	config.ConfirmClearHistory = a.configConfirmClearHistory.Load()
	config.SingleDelete = a.configSingleDelete.Load()
	config.SearchMenuTop = a.configSearchMenuTop.Load()
	config.ImageRename = a.configImageRename.Load()
	config.MoveToGroup = a.configMoveToGroup.Load()
	config.AutoRecognizeColor = a.configAutoRecognizeColor.Load()
	config.SaveLogToLocal = a.configSaveLogToLocal.Load()
	config.HighlightPrefix = a.configHighlightPrefix.Load()
	config.HighlightSuffix = a.configHighlightSuffix.Load()
	config.TimeFormat = a.configTimeFormat.Load()
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

	succ := a.addItemToHistory(a.history, item)
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
		a.addItemToHistory(group.History, item.Clone())
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

	a.stopStateSaver()

	a.monitor.Close()
	a.saveLocalState()
	a.logger.FlushToFile(a.configSaveLogToLocal.Load())
}
