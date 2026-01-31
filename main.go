package main

import (
	"fmt"

	"github.com/energye/systray"
)

type ClearState int

const (
	Normal ClearState = iota
	ReadyToClear
)

var (
	config_max         = 50
	global_clear_state = Normal
)

// func formatMenuItem(item *ClipItem) string {
// 	text := item.Text
// 	var prefix string

// 	switch item.Type {
// 	case TypeText:
// 		prefix = "📝"
// 		if len(text) > 40 {
// 			text = text[:40] + "..."
// 		}
// 	case TypeImage:
// 		prefix = "🖼️"
// 	case TypeFile:
// 		prefix = "📁"
// 		if len(text) > 50 {
// 			text = "..." + text[len(text)-47:]
// 		}
// 	}

// 	t := fmt.Sprintf("%s [%s] %s", prefix, item.Time.Format("15:04"), text)
// 	fmt.Println("formatMenuItem:", t)
// 	return t
// }

func formatMenuItem(item *ClipItem) string {
	text := item.Text
	var prefix string

	switch item.Type {
	case TypeText:
		prefix = "📝"
		text = truncateString(text, 40)

	case TypeImage:
		prefix = "🖼️"

	case TypeFile:
		prefix = "📁"
		text = truncateStringFromEnd(text, 50)
	}

	t := fmt.Sprintf("%s [%s] %s", prefix, item.Time.Format("15:04"), text)

	// 安全检查：确保返回值不为空
	if t == "" {
		t = prefix + " [empty]"
	}

	fmt.Println("formatMenuItem:", t)
	return t
}

// 从开头截断（保留前面部分）
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// 从末尾截断（保留后面部分，适合文件路径）
func truncateStringFromEnd(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	// 保留后面 maxLen-3 个字符
	return "..." + string(runes[len(runes)-(maxLen-3):])
}

func main() {
	history := NewHistory(config_max)
	writer := make(chan *ClipItem)

	groups := make(map[string]struct {
		active  *bool
		history *History
	})

	go startMonitor(func(item *ClipItem) {
		history.Add(item)
		for _, group := range groups {
			if *group.active {
				group.history.Add(item)
			}
		}
	}, writer)

	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTooltip("Clip")

		addSeparator := func() {
			systray.AddSeparator()
		}

		addQuitMenuCmd := func() {
			mQuit := systray.AddMenuItem("退出", "退出程序")
			mQuit.Click(func() {
				systray.Quit()
			})
		}

		addHistoryMenuAction := func() {
			for _, item := range history.GetAll() {
				menu := systray.AddMenuItem(formatMenuItem(item), item.Text)
				menu.Click(func() {
					writer <- item
				})
			}
		}

		addCreateGroupMenuCmd := func() {
			item := systray.AddMenuItem("➕ 创建分组", "使用最新剪贴板内容作为分组名")
			item.Click(func() {
				top := history.GetTop()
				if top == nil {
					return
				}
				groups[top.Text] = struct {
					active  *bool
					history *History
				}{
					active:  BoolPtr(false),
					history: NewHistory(config_max),
				}
			})
		}

		addGroupMenuAction := func() {
			for name, group := range groups {
				menu := systray.AddMenuItemCheckbox(name, "", *group.active)

				btnActive := menu.AddSubMenuItem("激活/取消激活分组", "")
				btnRename := menu.AddSubMenuItem("重命名", "")
				btnDelete := menu.AddSubMenuItem("删除分组", "")
				btnActive.Click(func() {
					*group.active = !*group.active
				})
				btnRename.Click(func() {
					top := history.GetTop()
					if top == nil {
						return
					}
					groups[top.Text] = group
					delete(groups, name)
				})
				btnDelete.Click(func() {
					delete(groups, name)
				})

				for _, item := range group.history.GetAll() {
					menu := menu.AddSubMenuItem(formatMenuItem(item), item.Text)
					menu.Click(func() {
						writer <- item
					})
				}
			}
		}

		addCleanHistoryMenuCmd := func() {
			if global_clear_state == Normal {
				menu := systray.AddMenuItem("清空历史记录", "")
				menu.Click(func() {
					global_clear_state = ReadyToClear
				})
			} else {
				menuOk := systray.AddMenuItem("确认清空历史记录？", "")
				menuOk.Click(func() {
					global_clear_state = Normal
					history.Clear()
				})
				menuCancle := systray.AddMenuItem("取消清空历史记录", "")
				menuCancle.Click(func() {
					global_clear_state = Normal
				})
			}
		}

		readyAndShow := func(menu systray.IMenu) {
			systray.ResetMenu()

			addHistoryMenuAction()
			addSeparator()
			addCleanHistoryMenuCmd()
			addSeparator()
			addGroupMenuAction()
			addSeparator()
			addCreateGroupMenuCmd()
			addSeparator()
			addQuitMenuCmd()

			menu.ShowMenu()
		}
		systray.SetOnClick(readyAndShow)
		systray.SetOnRClick(readyAndShow)
	}, func() {
		close(writer)
	})
}
