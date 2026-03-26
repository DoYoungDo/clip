package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/energye/systray"
	"golang.design/x/clipboard"
)

type ClearState int

const (
	Normal ClearState = iota
	ReadyToClear
)

type ShowMenuState int

const (
	Click ShowMenuState = iota
	RClick
)

type LogKind string

const (
	KindInfo  LogKind = "INFO"
	KindError LogKind = "ERR"
)

type LogEntry struct {
	Kind    LogKind
	Content string
}

type AppLogger struct {
	entries chan LogEntry
	mu      sync.Mutex
	buffer  bytes.Buffer
}

func NewAppLogger() *AppLogger {
	logger := &AppLogger{entries: make(chan LogEntry, 128)}

	go func() {
		for entry := range logger.entries {
			logger.mu.Lock()
			fmt.Fprintf(&logger.buffer, "%v [%v] %v", time.Now().Format("2006-01-02 15:04:05"), entry.Kind, fmt.Sprintln(entry.Content))
			logger.mu.Unlock()
		}
	}()

	return logger
}

func (l *AppLogger) FlushToFile(enabled bool) {
	if !enabled {
		return
	}

	for i := 0; i < 10 && len(l.entries) > 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}

	path := getLogPath()
	if err := ensureParentDir(path); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
		return
	}

	l.mu.Lock()
	data := append([]byte(nil), l.buffer.Bytes()...)
	l.mu.Unlock()

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志失败: %v\n", err)
	}
}

type clipboardMonitor struct {
	reader chan *ClipItem
	writer chan *ClipItem
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

func (m *clipboardMonitor) Close() {
	m.once.Do(func() {
		close(m.done)
		m.wg.Wait()
	})
}

func (m *clipboardMonitor) Done() <-chan struct{} {
	return m.done
}

func selectClipboardChange(lastTextHash string, lastImageHash string, text []byte, image []byte) (*ClipItem, string, string) {
	textHash := ""
	imageHash := ""
	if len(text) > 0 {
		textHash = calcClipItemHash(TypeText, text)
	}
	if len(image) > 0 {
		imageHash = calcClipItemHash(TypeImage, image)
	}

	textChanged := textHash != "" && textHash != lastTextHash
	imageChanged := imageHash != "" && imageHash != lastImageHash

	switch {
	case imageChanged:
		return NewClipItem(TypeImage, image), textHash, imageHash
	case textChanged:
		return NewClipItem(TypeText, text), textHash, imageHash
	default:
		return nil, textHash, imageHash
	}
}

type echoSuppressor struct {
	mu        sync.Mutex
	itemType  ItemType
	hash      string
	expiresAt time.Time
}

func (e *echoSuppressor) Mark(item *ClipItem, ttl time.Duration) {
	if item == nil {
		return
	}

	e.mu.Lock()
	e.itemType = item.Type
	e.hash = item.Hash
	e.expiresAt = time.Now().Add(ttl)
	e.mu.Unlock()
}

func (e *echoSuppressor) ShouldSuppress(item *ClipItem) bool {
	if item == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Now().After(e.expiresAt) {
		return false
	}
	if e.itemType == item.Type && e.hash != "" && e.hash == item.Hash {
		e.hash = ""
		e.expiresAt = time.Time{}
		return true
	}
	return false
}

// 全局状态
var (
	global_clear_state                                   = Normal
	global_show_menu_state                               = Click
	global_search_enable                                 = false
	global_search_text                                   = ""
	global_history_share_server  *ShareServer            = nil
	global_history_share_clients map[string]*ShareClient = make(map[string]*ShareClient)
	global_log_channel                                   = make(chan LogEntry, 128)
)

// 全局常量
const (
	const_max_history uint = 300
)

// 全局配置
var (
	config_history_max          uint = const_max_history
	config_single_delete             = false
	config_auto_recognize_color      = false
	config_save_log_to_local         = false
)

func formatMenuItem(item *ClipItem) string {
	text := string(item.Content)
	var prefix string

	switch item.Type {
	case TypeText:
		prefix = "📝"
		text = truncateString(text, 40)
	case TypeImage:
		prefix = "🖼️"
		text = fmt.Sprintf("图片 [%s]", fmt.Sprintf("%x", md5.Sum(item.Content))[:8])
	}

	t := fmt.Sprintf("%s [%s]%s%s", prefix, item.Time.Format("15:04"), Ifel(item.From == FromRemote, " [R] ", ""), text)
	if t == "" {
		t = prefix + " [empty]"
	}

	return t
}

func formatMenuItemTooltip(item *ClipItem) string {
	switch item.Type {
	case TypeText:
		return string(item.Content)
	case TypeImage:
		return "图片"
	default:
		return ""
	}
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func startMonitor() (*clipboardMonitor, error) {
	time.Sleep(time.Second)

	if err := clipboard.Init(); err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("初始化剪贴板失败: %v", err)}
		return nil, err
	}

	monitor := &clipboardMonitor{
		reader: make(chan *ClipItem, 1),
		writer: make(chan *ClipItem, 1),
		done:   make(chan struct{}),
	}

	monitor.wg.Add(2)

	go func() {
		defer monitor.wg.Done()
		for {
			select {
			case <-monitor.done:
				return
			case item := <-monitor.writer:
				if item == nil {
					continue
				}
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("写入剪贴板: %s", formatMenuItem(item))}
				clipboard.Write(Ifel(item.Type == TypeImage, clipboard.FmtImage, clipboard.FmtText), item.Content)
			}
		}
	}()

	go func() {
		defer monitor.wg.Done()
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "开始监听剪贴板, 每200毫秒检查一次..."}
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		lastTextHash := ""
		lastImageHash := ""

		sendItem := func(item *ClipItem) bool {
			select {
			case <-monitor.done:
				return false
			case monitor.reader <- item:
				return true
			}
		}

		for {
			select {
			case <-monitor.done:
				return
			case <-ticker.C:
				text := clipboard.Read(clipboard.FmtText)
				image := clipboard.Read(clipboard.FmtImage)
				item, nextTextHash, nextImageHash := selectClipboardChange(lastTextHash, lastImageHash, text, image)
				lastTextHash = nextTextHash
				lastImageHash = nextImageHash
				if item != nil && !sendItem(item) {
					return
				}
			}
		}
	}()

	return monitor, nil
}

func main() {
	logger := NewAppLogger()
	global_log_channel = logger.entries
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "程序启动"}

	history := NewHistory(config_history_max)
	groups := make(map[string]*Group)
	groupNames := []string{}
	var groupsMu sync.RWMutex
	var shareClientsMu sync.RWMutex
	var shareServerMu sync.RWMutex
	echoGuard := &echoSuppressor{}

	loadLocalState := func() {
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

		history.SetMaxSize(config_history_max)

		if localConfig.Data.History != nil {
			history.items = localConfig.Data.History
		}
		if localConfig.Data.Groups != nil {
			for name, groupData := range localConfig.Data.Groups {
				groups[name] = NewGroup(name, groupData.Active, const_max_history)
				if groupData.History != nil {
					groups[name].History.items = groupData.History
				}
			}
		}

		groupNames = localConfig.Data.GroupNames
	}

	saveLocalState := func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在保存配置和历史记录..."}

		config := NewDefaultConfig()
		config.HistoryMax = config_history_max
		config.SingleDelete = config_single_delete
		config.AutoRecognizeColor = config_auto_recognize_color
		config.SaveLogToLocal = config_save_log_to_local
		config.Data.History = history.GetAll()

		groupsMu.RLock()
		for name, group := range groups {
			config.Data.Groups[name] = HistoryGroupData{
				Active:  group.Active,
				History: group.History.GetAll(),
			}
		}
		config.Data.GroupNames = append([]string(nil), groupNames...)
		groupsMu.RUnlock()

		if err := saveConfigToPath(getConfigPath(), config); err != nil {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("保存配置失败: %v", err)}
		}
	}

	loadLocalState()

	monitor, err := startMonitor()
	if err != nil {
		saveLocalState()
		logger.FlushToFile(config_save_log_to_local)
		return
	}
	writer := monitor.writer

	go func() {
		for {
			select {
			case <-monitor.Done():
				return
			case item := <-monitor.reader:
				if item == nil {
					continue
				}
				if item.From == FromLocal && echoGuard.ShouldSuppress(item) {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("忽略回声剪贴板内容: %s", formatMenuItem(item))}
					continue
				}

				succ := history.Add(item)
				if succ {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("新剪贴板内容: %s", formatMenuItem(item))}
				}

				groupsMu.RLock()
				activeGroups := make([]*Group, 0, len(groups))
				for _, group := range groups {
					if group.Active {
						activeGroups = append(activeGroups, group)
					}
				}
				groupsMu.RUnlock()

				for _, group := range activeGroups {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加到分组 %s", group.Name)}
					group.History.Add(item.Clone())
				}

				shareServerMu.RLock()
				server := global_history_share_server
				shareServerMu.RUnlock()
				if succ && server != nil {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: "共享到局域网"}
					server.Share(item.CloneToRemote())
				}
			}
		}
	}()

	systray.Run(func() {
		systray.SetIcon(logo)
		systray.SetTooltip("Clip")

		addColorRecognizeMenuAction := func(menu *systray.MenuItem, item *ClipItem) bool {
			if !config_auto_recognize_color || item.Type != TypeText {
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
				writer <- NewClipItem(TypeText, []byte(hexT))
			})
			copyRGB.Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制RGB颜色: %s", rgbT)}
				writer <- NewClipItem(TypeText, []byte(rgbT))
			})
			return true
		}

		addSeparator := func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加分隔线"}
			systray.AddSeparator()
		}

		addQuitMenuCmd := func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加退出菜单"}
			mQuit := systray.AddMenuItem("退出", "退出程序")
			mQuit.Click(func() {
				systray.Quit()
			})
		}

		addHistoryMenuAction := func() bool {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加历史记录项"}
			all := history.GetAll()
			for i, item := range all {
				if global_search_enable && !strings.Contains(string(item.Content), global_search_text) {
					continue
				}

				menu := systray.AddMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
				switch global_show_menu_state {
				case Click:
					if !addColorRecognizeMenuAction(menu, item) {
						menu.Click(func() { writer <- item })
					}
				case RClick:
					if addColorRecognizeMenuAction(menu, item) {
						if config_single_delete {
							del := menu.AddSubMenuItem("删除", "")
							del.Click(func() {
								global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除历史记录项: %s", formatMenuItem(item))}
								history.Delete(i)
							})
						}
					} else {
						if config_single_delete {
							copy := menu.AddSubMenuItem("复制", "")
							del := menu.AddSubMenuItem("删除", "")
							copy.Click(func() {
								global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制历史记录项: %s", formatMenuItem(item))}
								writer <- item
							})
							del.Click(func() {
								global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除历史记录项: %s", formatMenuItem(item))}
								history.Delete(i)
							})
						} else {
							menu.Click(func() { writer <- item })
						}
					}
				}
			}

			return len(all) > 0
		}

		addCreateGroupMenuCmd := func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "创建`创建分组`菜单"}
			item := systray.AddMenuItem("➕ 创建分组", "使用最新剪贴板内容作为分组名")
			item.Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: "开始创建分组"}
				top := history.GetTop()
				if top == nil {
					global_log_channel <- LogEntry{Kind: KindError, Content: "创建分组失败: 历史记录为空，无法获取分组名"}
					return
				}
				if top.Type != TypeText {
					global_log_channel <- LogEntry{Kind: KindError, Content: "创建分组失败: 最新的历史记录不是文本，无法作为分组名"}
					fmt.Println("不支持创建图片分组")
					return
				}

				text := string(top.Content)
				groupsMu.Lock()
				groups[text] = NewGroup(text, false, const_max_history)
				groupNames = append(groupNames, text)
				groupsMu.Unlock()
			})
		}

		addGroupMenuAction := func() bool {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加分组项"}

			groupsMu.RLock()
			namesSnapshot := append([]string(nil), groupNames...)
			groupsMu.RUnlock()

			for i, name := range namesSnapshot {
				groupsMu.RLock()
				group := groups[name]
				groupsMu.RUnlock()
				if group == nil {
					continue
				}

				groupName := name
				groupIndex := i
				groupMenu := systray.AddMenuItemCheckbox("📂"+groupName, "", group.Active)

				if global_show_menu_state == RClick {
					btnActive := groupMenu.AddSubMenuItemCheckbox("激活/取消激活分组", "", group.Active)
					btnRename := groupMenu.AddSubMenuItem("重命名", "")
					btnDelete := groupMenu.AddSubMenuItem("删除分组", "")

					btnActive.Click(func() {
						groupsMu.Lock()
						group.Active = !group.Active
						active := group.Active
						groupsMu.Unlock()
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("%s分组%s", Ifel(active, "激活", "取消激活"), group.Name)}
					})

					btnRename.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("重命名分组: %s", group.Name)}
						top := history.GetTop()
						if top == nil {
							global_log_channel <- LogEntry{Kind: KindError, Content: "重命名分组失败: 历史记录为空，无法获取新分组名"}
							return
						}
						if top.Type != TypeText {
							global_log_channel <- LogEntry{Kind: KindError, Content: "重命名分组失败: 最新的历史记录不是文本，无法作为新分组名"}
							fmt.Println("不支持重命名图片分组")
							return
						}

						newName := string(top.Content)
						groupsMu.Lock()
						group.Name = newName
						groups[newName] = group
						delete(groups, groupName)
						if groupIndex >= 0 && groupIndex < len(groupNames) {
							groupNames[groupIndex] = newName
						}
						groupsMu.Unlock()
					})

					btnDelete.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组: %s", group.Name)}
						groupsMu.Lock()
						delete(groups, groupName)
						if groupIndex >= 0 && groupIndex < len(groupNames) {
							groupNames = append(groupNames[:groupIndex], groupNames[groupIndex+1:]...)
						}
						groupsMu.Unlock()
					})
				}

				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加分组菜单: %s 历史记录", group.Name)}
				for itemIndex, item := range group.History.GetAll() {
					if global_search_enable && !strings.Contains(string(item.Content), global_search_text) {
						continue
					}

					entryMenu := groupMenu.AddSubMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
					switch global_show_menu_state {
					case Click:
						if !addColorRecognizeMenuAction(entryMenu, item) {
							entryMenu.Click(func() {
								global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
								writer <- item
							})
						}
					case RClick:
						if addColorRecognizeMenuAction(entryMenu, item) {
							if config_single_delete {
								del := entryMenu.AddSubMenuItem("删除", "")
								del.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组历史记录项: %s", formatMenuItem(item))}
									group.History.Delete(itemIndex)
								})
							}
						} else {
							if config_single_delete {
								copy := entryMenu.AddSubMenuItem("复制", "")
								del := entryMenu.AddSubMenuItem("删除", "")
								copy.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
									writer <- item
								})
								del.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组历史记录项: %s", formatMenuItem(item))}
									group.History.Delete(itemIndex)
								})
							} else {
								entryMenu.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
									writer <- item
								})
							}
						}
					}
				}
			}

			groupsMu.RLock()
			count := len(groups)
			groupsMu.RUnlock()
			return count > 0
		}

		addCleanHistoryMenuCmd := func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`清空历史记录`菜单"}
			if global_clear_state == Normal {
				menu := systray.AddMenuItem("清空历史记录", "【清空历史记录】会将历史记录清空，但是不会清空剪贴板中的内容")
				menu.Click(func() {
					global_clear_state = ReadyToClear
					global_log_channel <- LogEntry{Kind: KindInfo, Content: "准备清空历史记录，等待确认..."}
				})
			} else {
				menu := systray.AddMenuItem("确认/取消清空历史记录？", "")
				menuOk := menu.AddSubMenuItem("确认清空？", "")
				menuOk.Click(func() {
					global_clear_state = Normal
					history.Clear()
					global_log_channel <- LogEntry{Kind: KindInfo, Content: "历史记录已清空"}
				})
				menuCancel := menu.AddSubMenuItem("取消清空?", "")
				menuCancel.Click(func() {
					global_clear_state = Normal
					global_log_channel <- LogEntry{Kind: KindInfo, Content: "取消清空历史记录"}
				})
			}
		}

		addConfigMenuAction := func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`配置`菜单"}

			menu := systray.AddMenuItem("配置", "")
			menu.AddSubMenuItemCheckbox("单独删除项", "", config_single_delete).Click(func() {
				config_single_delete = !config_single_delete
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置单独删除项: %v", config_single_delete)}
			})
			menu.AddSubMenuItemCheckbox("自动识别颜色", "", config_auto_recognize_color).Click(func() {
				config_auto_recognize_color = !config_auto_recognize_color
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置自动识别颜色: %v", config_auto_recognize_color)}
			})
			menu.AddSubMenuItem("设置最大历史记录条数"+fmt.Sprintf("(当前: %d)", config_history_max), "【设置最大历史记录条数】会设置历史记录的最大条数，超过最大条数会自动删除最早的记录，范围：1-300").Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: "设置最大历史记录条数"}
				top := history.GetTop()
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

				config_history_max = uint(digit)
				history.SetMaxSize(config_history_max)
			})

			shareMenu := menu.AddSubMenuItem("局域网共享", "")
			shareServerMu.RLock()
			server := global_history_share_server
			serverLabel := ""
			if server != nil {
				serverLabel = fmt.Sprintf("(%v)", server.AddrString())
			}
			shareServerMu.RUnlock()

			shareMenu.AddSubMenuItemCheckbox("局域网共享"+serverLabel, "", server != nil).Click(func() {
				shareServerMu.RLock()
				currentServer := global_history_share_server
				shareServerMu.RUnlock()

				global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(currentServer == nil, "启动局域网共享", "关闭局域网共享")}
				if currentServer == nil {
					newServer, err := NewShareServer()
					if err != nil {
						global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("启动局域网共享失败: %v", err)}
						return
					}
					shareServerMu.Lock()
					global_history_share_server = newServer
					shareServerMu.Unlock()
					writer <- NewClipItem(TypeText, []byte(newServer.AddrString()))
					newServer.Start()
				} else {
					currentServer.Stop()
					shareServerMu.Lock()
					global_history_share_server = nil
					shareServerMu.Unlock()
				}
			})

			shareMenu.AddSubMenuItem("连接到", "").Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: "连接到局域网共享"}
				top := history.GetTop()
				if top == nil || top.Type != TypeText {
					global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 历史记录为空，无法获取地址"}
					return
				}

				addr := string(top.Content)
				if addr == "" {
					global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 地址为空"}
					return
				}

				shareClientsMu.RLock()
				_, exists := global_history_share_clients[addr]
				shareClientsMu.RUnlock()
				if exists {
					global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 已经连接过了"}
					return
				}

				shareClient := NewShareClient(addr)
				if shareClient.ConnectTo() {
					shareClientsMu.Lock()
					global_history_share_clients[addr] = shareClient
					shareClientsMu.Unlock()
					shareClient.OnShared(func(item *ClipItem) {
						echoGuard.Mark(item, 3*time.Second)
						history.Add(item)
						writer <- item
					})
					shareClient.OnClose(func() {
						shareClientsMu.Lock()
						delete(global_history_share_clients, addr)
						shareClientsMu.Unlock()
					})
				}
			})

			menu.AddSubMenuItemCheckbox("退出时保存日志", "", config_save_log_to_local).Click(func() {
				config_save_log_to_local = !config_save_log_to_local
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置退出时保存日志: %v", config_save_log_to_local)}
			})

			shareClientsMu.RLock()
			clientSnapshot := make(map[string]*ShareClient, len(global_history_share_clients))
			for addr, client := range global_history_share_clients {
				clientSnapshot[addr] = client
			}
			shareClientsMu.RUnlock()

			for addr, client := range clientSnapshot {
				shareMenu.AddSubMenuItemCheckbox(addr, "", true).Click(func() {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("断开与局域网共享%s的连接", addr)}
					client.Close()
					shareClientsMu.Lock()
					delete(global_history_share_clients, addr)
					shareClientsMu.Unlock()
				})
			}
		}

		addSearchMenuAction := func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`搜索`菜单"}
			systray.AddMenuItemCheckbox("🔎 搜索"+Ifel(global_search_enable, ":"+global_search_text, ""), "【搜索】会使用剪贴板内的内容进行过滤，再次点击取消搜索", global_search_enable).Click(func() {
				global_search_enable = !global_search_enable
				global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(global_search_enable, "启用搜索", "禁用搜索")}
				if !global_search_enable {
					global_search_text = ""
					return
				}

				top := history.GetTop()
				if top == nil {
					return
				}

				text := string(top.Content)
				if text == "" {
					return
				}

				global_search_text = text
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置搜索关键词: %s", global_search_text)}
			})
		}

		systray.SetOnClick(func(menu systray.IMenu) {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "点击托盘图标"}
			global_show_menu_state = Click
			systray.ResetMenu()

			if addHistoryMenuAction() {
				addSeparator()
			}
			addGroupMenuAction()

			global_log_channel <- LogEntry{Kind: KindInfo, Content: "显示菜单"}
			menu.ShowMenu()
		})

		systray.SetOnRClick(func(menu systray.IMenu) {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "右键点击托盘图标"}
			global_show_menu_state = RClick
			systray.ResetMenu()

			if addHistoryMenuAction() {
				addSeparator()
			}
			addCleanHistoryMenuCmd()
			addSeparator()
			if addGroupMenuAction() {
				addSeparator()
			}
			addCreateGroupMenuCmd()
			addSeparator()
			addSearchMenuAction()
			addSeparator()
			addConfigMenuAction()
			addSeparator()
			addQuitMenuCmd()

			global_log_channel <- LogEntry{Kind: KindInfo, Content: "显示菜单"}
			menu.ShowMenu()
		})
	}, func() {
		shareClientsMu.RLock()
		clients := make([]*ShareClient, 0, len(global_history_share_clients))
		for _, client := range global_history_share_clients {
			clients = append(clients, client)
		}
		shareClientsMu.RUnlock()

		for _, client := range clients {
			client.Close()
		}

		shareServerMu.RLock()
		server := global_history_share_server
		shareServerMu.RUnlock()
		if server != nil {
			server.Stop()
			shareServerMu.Lock()
			global_history_share_server = nil
			shareServerMu.Unlock()
		}

		monitor.Close()
		saveLocalState()
		logger.FlushToFile(config_save_log_to_local)
	})
}
