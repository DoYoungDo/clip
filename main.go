package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	KindInfo LogKind = "INFO"
	KindError LogKind = "ERR"
)
type LogEntry struct {
	Kind    LogKind
	Content string
}
// 全局状态
var (
	global_clear_state   = Normal
	global_show_menu_state = Click
	global_search_enable = false
	global_search_text string = ""
	global_history_share_server *ShareServer = nil
	global_history_share_clients map[string]*ShareClient = make(map[string]*ShareClient)
	global_log_channel = make(chan LogEntry, 5)
)

// 全局常量
const (
	const_max_history uint = 300
)

// 全局配置
var (
	config_history_max uint = const_max_history
	config_single_delete = false
	config_auto_recognize_color = false
	config_save_log_to_local = false
)


func formatMenuItem(item *ClipItem) string {
	text := (string(item.Content))
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

	// 安全检查：确保返回值不为空
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

// 从开头截断（保留前面部分）
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func startMonitor() (chan *ClipItem, chan *ClipItem, error) {
	time.Sleep(time.Second)

	if err := clipboard.Init(); err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("初始化剪贴板失败: %v", err)}
		return nil, nil, err
	}

	reader := make(chan *ClipItem, 1)
	writer := make(chan *ClipItem, 1)

	go func() {
		for item := range writer {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("写入剪贴板: %s", formatMenuItem(item))}
			clipboard.Write(Ifel(item.Type == TypeImage, clipboard.FmtImage, clipboard.FmtText), item.Content)
		}
	}()

	go func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "开始监听剪贴板, 每200毫秒检查一次..."}
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			// 监听文本
			text := clipboard.Read(clipboard.FmtText)
			if len(text) > 0 {
				reader <- NewClipItem(TypeText, text)
			}

			// 监听图片
			image := clipboard.Read(clipboard.FmtImage)
			if len(image) > 0 {
				reader <- NewClipItem(TypeImage, image)
			}
		}
	}()

	return reader, writer, nil
}

func main() {
	logToLocal := func () func()  {
		buffer := &bytes.Buffer{}

		go func ()  {
			for entry := range global_log_channel {
				fmt.Fprintf(buffer, "%v [%v] %v", time.Now().Format("2006-01-02 15:04:05"), entry.Kind, fmt.Sprintln(entry.Content))
			}
		}()

		global_log_channel <- LogEntry{Kind: KindInfo, Content: "程序启动"}
		return func ()  {
			// 将日志写入文件
			if config_save_log_to_local {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在保存日志到本地..."}
				os.WriteFile(getLogPath(), buffer.Bytes(), 0644)
			}
			close(global_log_channel)
		}
	}()
	defer logToLocal()

	history := NewHistory(config_history_max)
	groups := make(map[string]*Group)
	groupNames := []string{}

	cacheToLocal := func() func()  {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在加载配置和历史记录..."}

		localConfig := NewDefaultConfig()
		data, err := os.ReadFile(getConfigPath())
		if err == nil{
			json.Unmarshal(data, &localConfig)
		}

		config_history_max = localConfig.HistoryMax
		config_single_delete = localConfig.SingleDelete
		config_auto_recognize_color = localConfig.AutoRecognizeColor
		config_save_log_to_local = localConfig.SaveLogToLocal

		history.SetMaxSize(config_history_max)

		// 加载本地历史记录
		if localConfig.Data.History != nil{
			history.items = localConfig.Data.History
		}
		if localConfig.Data.Groups != nil{
			for name, groupData := range localConfig.Data.Groups{
				groups[name] = NewGroup(name, groupData.Active, const_max_history)
				if groupData.History != nil{
					groups[name].History.items = groupData.History
				}
			}
		}

		groupNames = localConfig.Data.GroupNames

		return func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在保存配置和历史记录..."}
			// 保存配置
			config := NewDefaultConfig()
			config.HistoryMax = config_history_max
			config.SingleDelete = config_single_delete
			config.AutoRecognizeColor = config_auto_recognize_color
			config.SaveLogToLocal = config_save_log_to_local
			config.Data.History = history.GetAll()
			for name, group := range groups {
				config.Data.Groups[name] = HistoryGroupData{
					Active: group.Active,
					History: group.History.GetAll(),
				}
			}
			config.Data.GroupNames = groupNames

			data, _ := json.Marshal(config)
			os.WriteFile(getConfigPath(), data, 0644)
		}
	}()
	defer logToLocal()

	// 启动监听
	writer, err, def := func () (w chan *ClipItem, e error, def func())  {
		reader, writer, err := startMonitor()
		if err != nil {
			return writer, err, func() {}
		}

		// 更新监听通道
		go func() {
			for item := range reader {
				succ := history.Add(item)
				if succ{
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("新剪贴板内容: %s", formatMenuItem(item))}
				}

				for _, group := range groups {
					if group.Active {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加到分组 %s", group.Name)}
						group.History.Add(item.Clone())
					}
				}

				if succ && global_history_share_server != nil{
					global_log_channel <- LogEntry{Kind: KindInfo, Content: "共享到局域网"}
					global_history_share_server.Share(item.CloneToRemote())
				}
			}
		}()
		return writer, nil, func() {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "关闭所有监听..."}
			if global_history_share_server != nil {
				global_history_share_server.Stop()
			}
			close(reader)
			close(writer)
		}
	}()
	defer def()
	if err != nil {
		return
	}
	
	// 初始化系统托盘
	systray.Run(func() {
		// Windows 系统托盘图标设置
		systray.SetIcon(logo)
		systray.SetTooltip("Clip")

		addColorRecognizeMenuAction := func (menu *systray.MenuItem, item *ClipItem) bool  {
			if !config_auto_recognize_color || item.Type != TypeText{
				return false
			}

			r,g,b,base,ok := getColor(string(item.Content))
			if ok {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("识别颜色成功: %s,并添加菜单", string(item.Content))}
				rt,_ := strconv.ParseInt(r,base,0)
				gt,_ := strconv.ParseInt(g,base,0)
				bt,_ := strconv.ParseInt(b,base,0)
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
			return false
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
				if global_search_enable {
					if !strings.Contains(string(item.Content), global_search_text){
						continue
					}
				}
				menu := systray.AddMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
				switch global_show_menu_state {
				case Click:
					if !addColorRecognizeMenuAction(menu, item){
						menu.Click(func() {
							writer <- item
						})
					} 
				case RClick:
					if addColorRecognizeMenuAction(menu, item){
						if config_single_delete {
							del := menu.AddSubMenuItem("删除", "")
							del.Click(func() {
								global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除历史记录项: %s", formatMenuItem(item))}
								history.Delete(i)
							})
						}
					}else{
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
						}else {
							menu.Click(func() {
								writer <- item
							})
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
				if top.Type == TypeText {
					text := string(top.Content)
					groups[text] = NewGroup(text, false, const_max_history)
					groupNames = append(groupNames, text)
				}else{
					global_log_channel <- LogEntry{Kind: KindError, Content: "创建分组失败: 最新的历史记录不是文本，无法作为分组名"}
					fmt.Println("不支持创建图片分组")
				}
			})
		}

		addGroupMenuAction := func() bool {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加分组项"}
			for i, name := range groupNames {
				group := groups[name]
				menu := systray.AddMenuItemCheckbox("📂" + name, "", group.Active)

				if global_show_menu_state == RClick{
					btnActive := menu.AddSubMenuItemCheckbox("激活/取消激活分组", "", group.Active)
					btnRename := menu.AddSubMenuItem("重命名", "")
					btnDelete := menu.AddSubMenuItem("删除分组", "")
					btnActive.Click(func() {
						group.Active = !group.Active
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("%s分组%s", Ifel(group.Active, "激活", "取消激活"), group.Name)}
					})
					btnRename.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("重命名分组: %s", group.Name)}
						top := history.GetTop()
						if top == nil {
							global_log_channel <- LogEntry{Kind: KindError, Content: "重命名分组失败: 历史记录为空，无法获取新分组名"}
							return
						}
						if top.Type == TypeText {
							groups[string(top.Content)] = group
							delete(groups, name)
							groupNames[i] = string(top.Content)
						}else{
							global_log_channel <- LogEntry{Kind: KindError, Content: "重命名分组失败: 最新的历史记录不是文本，无法作为新分组名"}
							fmt.Println("不支持重命名图片分组")
						}
					})
					btnDelete.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组: %s", group.Name)}
						delete(groups, name)
						groupNames = append(groupNames[:i], groupNames[i+1:]...)
					})
				}

				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加分组菜单: %s 历史记录", group.Name)}
				for i, item := range group.History.GetAll() {
					if global_search_enable {
						if !strings.Contains(string(item.Content), global_search_text){
							continue
						}
					}
					menu := menu.AddSubMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
					switch global_show_menu_state {
					case Click:
						if !addColorRecognizeMenuAction(menu, item){
							menu.Click(func() {
								global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制历史记录项: %s", formatMenuItem(item))}
								writer <- item
							})
						} 
					case RClick:
						if addColorRecognizeMenuAction(menu, item){
							if config_single_delete {
								del := menu.AddSubMenuItem("删除", "")
								del.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组历史记录项: %s", formatMenuItem(item))}
									history.Delete(i)
								})
							}
						}else{
							if config_single_delete {
								copy := menu.AddSubMenuItem("复制", "")
								del := menu.AddSubMenuItem("删除", "")
								copy.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
									writer <- item
								})
								del.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组历史记录项: %s", formatMenuItem(item))}
									group.History.Delete(i)
								})
							}else {
								menu.Click(func() {
									global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
									writer <- item
								})
							}	
						}
					}
				}
			}

			return len(groups) > 0
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
				menuCancle := menu.AddSubMenuItem("取消清空?", "")
				menuCancle.Click(func() {
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
			menu.AddSubMenuItem("设置最大历史记录条数" + fmt.Sprintf("(当前: %d)", config_history_max), "【设置最大历史记录条数】会设置历史记录的最大条数，超过最大条数会自动删除最早的记录，范围：1-300").Click(func() {
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

				if digit > 300 || digit <= 0 {
					global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("设置最大历史记录条数失败: 数字超出范围: %d", digit)}
					return
				}

				config_history_max = uint(digit)
				history.SetMaxSize(config_history_max)
			})
			shareMenu := menu.AddSubMenuItem("局域网共享","")
			shareMenu.AddSubMenuItemCheckbox("局域网共享" + IfelFunc(global_history_share_server != nil, func() string { return fmt.Sprintf("(%v)", global_history_share_server.AddrString()) }, func() string { return "" }), "", global_history_share_server != nil).Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(global_history_share_server == nil, "启动局域网共享", "关闭局域网共享")}
				if global_history_share_server == nil {
					// 创建tcp server
					global_history_share_server = NewShareServer()
					// 将tcp server地址写入剪贴板
					writer <- NewClipItem(TypeText, []byte(global_history_share_server.AddrString()))
					// 启动tcp server 监听
					global_history_share_server.Start()
				}else{
					// 关闭tcp server 监听
					global_history_share_server.Stop()
					global_history_share_server = nil
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
				if addr == ""{
					global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 地址为空"}
					return
				}

				if _, ok := global_history_share_clients[addr]; ok{
					global_log_channel <- LogEntry{Kind: KindError, Content: "连接到局域网共享失败: 已经连接过了"}
					return
				}

				shareClient := NewShareClient(addr)
				if shareClient.ConnectTo(){
					global_history_share_clients[addr] = shareClient
					shareClient.OnShared(func(item *ClipItem) {
						history.Add(item)
						writer <- item
					})
					shareClient.OnClose(func ()  {
						delete(global_history_share_clients, addr)
					})
				}
			})
			menu.AddSubMenuItemCheckbox("退出时保存日志", "", config_save_log_to_local).Click(func() {
				config_save_log_to_local = !config_save_log_to_local
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("设置退出时保存日志: %v", config_save_log_to_local)}
			})

			for addr, client := range global_history_share_clients{
				shareMenu.AddSubMenuItemCheckbox(addr, "", true).Click(func() {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("断开与局域网共享%s的连接", addr)}
					client.Close()
					delete(global_history_share_clients, addr)
				})
			}
		}

		addSearchMenuAction := func ()  {
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加`搜索`菜单"}
			systray.AddMenuItemCheckbox("🔎 搜索" + Ifel(global_search_enable, ":" + global_search_text, ""), "【搜索】会使用剪贴板内的内容进行过滤，再次点击取消搜索", global_search_enable).Click(func() {
				global_search_enable = !global_search_enable
				global_log_channel <- LogEntry{Kind: KindInfo, Content: Ifel(global_search_enable, "启用搜索", "禁用搜索")}
				if !global_search_enable{
					global_search_text = ""
					return
				}

				top := history.GetTop()
				if top == nil{
					return
				}
				text := string(top.Content)
				if text == ""{
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
			if (addGroupMenuAction()) {
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
		def()
		cacheToLocal()
		logToLocal()
	})
}
