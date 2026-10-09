package main

import (
	"fmt"
	"strings"

	"github.com/energye/systray"
)

func (a *App) addCreateGroupMenuCmd() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "创建`创建分组`菜单"}
	item := systray.AddMenuItem("➕ 创建分组", "使用最新剪贴板内容作为分组名")
	item.Click(func() {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "开始创建分组"}
		top := a.history.GetTop()
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
		a.groupsMu.Lock()
		a.groups[text] = NewGroup(text, false, const_max_history)
		a.groupNames = append(a.groupNames, text)
		a.groupsMu.Unlock()
	})
}

func (a *App) addGroupMenuAction() bool {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加分组项"}

	a.groupsMu.RLock()
	namesSnapshot := append([]string(nil), a.groupNames...)
	a.groupsMu.RUnlock()

	for i, name := range namesSnapshot {
		a.groupsMu.RLock()
		group := a.groups[name]
		a.groupsMu.RUnlock()
		if group == nil {
			continue
		}

		groupName := name
		groupIndex := i
		groupMenu := systray.AddMenuItemCheckbox("📂"+groupName, "", group.Active)

		if a.showMenuState == RClick {
			btnActive := groupMenu.AddSubMenuItemCheckbox("激活/取消激活分组", "", group.Active)
			btnRename := groupMenu.AddSubMenuItem("重命名", "")
			btnDelete := groupMenu.AddSubMenuItem("删除分组", "")

			btnActive.Click(func() {
				a.groupsMu.Lock()
				group.Active = !group.Active
				active := group.Active
				a.groupsMu.Unlock()
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("%s分组%s", Ifel(active, "激活", "取消激活"), group.Name)}
				if active && syncLatestHistoryToGroup(a.history, group) {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("同步最近一条历史记录到分组 %s", group.Name)}
				}
			})

			btnRename.Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("重命名分组: %s", group.Name)}
				top := a.history.GetTop()
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
				a.groupsMu.Lock()
				group.Name = newName
				a.groups[newName] = group
				delete(a.groups, groupName)
				if groupIndex >= 0 && groupIndex < len(a.groupNames) {
					a.groupNames[groupIndex] = newName
				}
				a.groupsMu.Unlock()
			})

			btnDelete.Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组: %s", group.Name)}
				a.groupsMu.Lock()
				delete(a.groups, groupName)
				if groupIndex >= 0 && groupIndex < len(a.groupNames) {
					a.groupNames = append(a.groupNames[:groupIndex], a.groupNames[groupIndex+1:]...)
				}
				a.groupsMu.Unlock()
			})
		}

		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加分组菜单: %s 历史记录", group.Name)}
		for _, item := range group.History.GetAll() {
			if a.searchEnable && !strings.Contains(string(item.Content), a.searchText) {
				continue
			}

			entryMenu := groupMenu.AddSubMenuItem(formatMenuItem(item), formatMenuItemTooltip(item))
			switch a.showMenuState {
			case Click:
				if !a.addColorRecognizeMenuAction(entryMenu, item) {
					entryMenu.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
						a.writer <- item
					})
				}
			case RClick:
				if a.addColorRecognizeMenuAction(entryMenu, item) {
					if a.configSingleDelete {
						del := entryMenu.AddSubMenuItem("删除", "")
						del.Click(func() {
							global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组历史记录项: %s", formatMenuItem(item))}
							group.History.Delete(item)
						})
					}
				} else {
					if a.configSingleDelete {
						copy := entryMenu.AddSubMenuItem("复制", "")
						del := entryMenu.AddSubMenuItem("删除", "")
						copy.Click(func() {
							global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
							a.writer <- item
						})
						del.Click(func() {
							global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组历史记录项: %s", formatMenuItem(item))}
							group.History.Delete(item)
						})
					} else {
						entryMenu.Click(func() {
							global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
							a.writer <- item
						})
					}
				}
			}
		}
	}

	a.groupsMu.RLock()
	count := len(a.groups)
	a.groupsMu.RUnlock()
	return count > 0
}
