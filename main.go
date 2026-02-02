package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
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

var (
	global_clear_state   = Normal
	global_show_menu_state = Click
)

var (
	config_max           = 50
	config_single_delete = false
)

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

	// fmt.Println("formatMenuItem:", t)
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

func startMonitor() (chan *ClipItem, chan *ClipItem, error) {
	time.Sleep(time.Second)

	if err := clipboard.Init(); err != nil {
		return nil, nil, err
	}

	reader := make(chan *ClipItem, 1)
	writer := make(chan *ClipItem, 1)

	go func() {
		for item := range writer {
			clipboard.Write(Ifel(item.Type == TypeImage, clipboard.FmtImage, clipboard.FmtText), item.Content)
		}
	}()

	go func() {
		var lastText []byte
		var lastImage []byte
		var lastFilePath string

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			// 先检查是否有文件
			filePath := getFilePath()
			if filePath != "" {
				if filePath != lastFilePath {
					reader <- &ClipItem{
						Type:     TypeFile,
						Content:  []byte(filePath),
						Text:     filePath,
						FilePath: filePath,
						Time:     time.Now(),
					}
					lastFilePath = filePath
					lastText = nil
				}
				continue
			} else {
				// 没有文件时清空文件缓存
				lastFilePath = ""
			}

			// 监听文本
			text := clipboard.Read(clipboard.FmtText)
			if len(text) > 0 && !bytes.Equal(text, lastText) {
				textStr := string(text)
				itemType := TypeText
				displayText := textStr
				filePath := ""

				// 检查是否是 file:// 格式（Linux）
				if strings.HasPrefix(textStr, "file://") {
					itemType = TypeFile
					filePath = strings.TrimPrefix(textStr, "file://")
					filePath = strings.TrimSpace(filePath)
					displayText = filePath
				}

				reader <- &ClipItem{
					Type:     itemType,
					Content:  append([]byte(nil), text...),
					Text:     displayText,
					FilePath: filePath,
					Time:     time.Now(),
				}

				lastText = append([]byte(nil), text...)
			}

			// 监听图片
			image := clipboard.Read(clipboard.FmtImage)
			if len(image) > 0 && !bytes.Equal(image, lastImage) {
				hash := fmt.Sprintf("%x", md5.Sum(image))
				reader <- &ClipItem{
					Type:     TypeImage,
					Content:  append([]byte(nil), image...),
					Text:     fmt.Sprintf("图片 [%s]", hash[:8]), // 显示前8位MD5
					FilePath: hash,                             // 完整 MD5 用于去重
					Time:     time.Now(),
				}
				lastImage = append([]byte(nil), image...)
			}
		}
	}()

	return reader, writer, nil
}

func main() {
	history := NewHistory(config_max)
	writer := make(chan *ClipItem)

	groups := make(map[string]*Group)

	reader, writer, err := startMonitor()
	if err != nil {
		return
	}

	go func() {
		for item := range reader {
			history.Add(item)

			for _, group := range groups {
				if group.Active {
					group.History.Add(item)
				}
			}
		}
	}()

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
			all := history.GetAll()
			for i, item := range all {
				menu := systray.AddMenuItem(formatMenuItem(item), item.Text)
				switch global_show_menu_state {
				case Click:
					menu.Click(func() {
						writer <- item
					})
				case RClick:
					if config_single_delete {
						copy := menu.AddSubMenuItem("复制", "")
						del := menu.AddSubMenuItem("删除", "")
						copy.Click(func() {
							writer <- item
						})
						del.Click(func() {
							history.Delete(i)
						})
					}else {
						menu.Click(func() {
							writer <- item
						})
					}
				}
			}
			if len(all) > 0{
				addSeparator()
			}
		}

		addCreateGroupMenuCmd := func() {
			item := systray.AddMenuItem("➕ 创建分组", "使用最新剪贴板内容作为分组名")
			item.Click(func() {
				top := history.GetTop()
				if top == nil {
					return
				}
				groups[top.Text] = NewGroup(top.Text, true, config_max)
			})
			addSeparator()
		}

		addGroupMenuAction := func() {
			for name, group := range groups {
				menu := systray.AddMenuItemCheckbox("📂" + name, "", group.Active)

				if global_show_menu_state == RClick{
					btnActive := menu.AddSubMenuItemCheckbox("激活/取消激活分组", "", group.Active)
					btnRename := menu.AddSubMenuItem("重命名", "")
					btnDelete := menu.AddSubMenuItem("删除分组", "")
					btnActive.Click(func() {
						group.Active = !group.Active
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
				}

				for i, item := range group.History.GetAll() {
					menu := menu.AddSubMenuItem(formatMenuItem(item), item.Text)
					switch global_show_menu_state {
					case Click:
						menu.Click(func() {
							writer <- item
						})
					case RClick:
						if config_single_delete {
							copy := menu.AddSubMenuItem("复制", "")
							del := menu.AddSubMenuItem("删除", "")
							copy.Click(func() {
								writer <- item
							})
							del.Click(func() {
								group.History.Delete(i)
							})
						}else {
							menu.Click(func() {
								writer <- item
							})
						}
					}
				}
			}

			if global_show_menu_state == RClick && len(groups) > 0{
				addSeparator()
			}
		}

		addCleanHistoryMenuCmd := func() {
			if global_clear_state == Normal {
				menu := systray.AddMenuItem("清空历史记录", "")
				menu.Click(func() {
					global_clear_state = ReadyToClear
				})
			} else {
				menu := systray.AddMenuItem("确认/取消清空历史记录？", "")
				menuOk := menu.AddSubMenuItem("确认清空？", "")
				menuOk.Click(func() {
					global_clear_state = Normal
					history.Clear()
				})
				menuCancle := menu.AddSubMenuItem("取消清空?", "")
				menuCancle.Click(func() {
					global_clear_state = Normal
				})
			}
			addSeparator()
		}

		addConfigMenuAction := func() {
			menu := systray.AddMenuItem("配置", "")
			btnSingleDelete := menu.AddSubMenuItemCheckbox("单独删除项", "", config_single_delete)
			btnSingleDelete.Click(func() {
				config_single_delete = !config_single_delete
			})
			addSeparator()
		}

		systray.SetOnClick(func(menu systray.IMenu) {
			global_show_menu_state = Click

			systray.ResetMenu()

			addHistoryMenuAction()
			addGroupMenuAction()

			menu.ShowMenu()
		})
		systray.SetOnRClick(func(menu systray.IMenu) {
			global_show_menu_state = RClick

			systray.ResetMenu()

			addHistoryMenuAction()
			addCleanHistoryMenuCmd()
			addGroupMenuAction()
			addCreateGroupMenuCmd()
			addConfigMenuAction()
			addQuitMenuCmd()

			menu.ShowMenu()
		})
	}, func() {
		close(writer)
	})
}
