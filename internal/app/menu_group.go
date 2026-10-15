package app

import (
	"fmt"

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
			global_log_channel <- LogEntry{Kind: KindError, Content: "不支持创建图片分组"}
			return
		}

		text := string(top.Content)
		a.groupsMu.Lock()
		a.groups[text] = NewGroup(text, false, const_max_history)
		a.groupNames = append(a.groupNames, text)
		a.groupsMu.Unlock()
	})
}

func (a *App) renameGroup(oldName string, newName string) bool {
	a.groupsMu.Lock()
	defer a.groupsMu.Unlock()

	group, ok := a.groups[oldName]
	if !ok {
		return false
	}
	group.Name = newName
	a.groups[newName] = group
	delete(a.groups, oldName)
	for i, existing := range a.groupNames {
		if existing == oldName {
			a.groupNames[i] = newName
			break
		}
	}
	return true
}

func (a *App) deleteGroup(name string) bool {
	a.groupsMu.Lock()
	defer a.groupsMu.Unlock()

	if _, ok := a.groups[name]; !ok {
		return false
	}
	delete(a.groups, name)
	filtered := a.groupNames[:0]
	for _, existing := range a.groupNames {
		if existing != name {
			filtered = append(filtered, existing)
		}
	}
	a.groupNames = filtered
	return true
}

func (a *App) addGroupMenuAction() bool {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "添加分组项"}

	a.groupsMu.RLock()
	namesSnapshot := append([]string(nil), a.groupNames...)
	a.groupsMu.RUnlock()

	addedGroups := 0
	for _, name := range namesSnapshot {
		a.groupsMu.RLock()
		group := a.groups[name]
		a.groupsMu.RUnlock()
		if group == nil {
			continue
		}

		matched := make([]*ClipItem, 0)
		for _, item := range group.History.GetAll() {
			if a.matchSearch(item) {
				matched = append(matched, item)
			}
		}
		if a.searchEnable && len(matched) == 0 {
			continue
		}
		addedGroups++

		groupName := name
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
					global_log_channel <- LogEntry{Kind: KindError, Content: "不支持重命名图片分组"}
					return
				}

				newName := string(top.Content)
				a.renameGroup(groupName, newName)
			})

			btnDelete.Click(func() {
				global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("删除分组: %s", group.Name)}
				a.deleteGroup(groupName)
			})
		}

		global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("添加分组菜单: %s 历史记录", group.Name)}
		for _, item := range matched {
			var info itemMenuInfo
			if a.showMenuState == RClick {
				info = a.buildItemMenuInfo(item)
			}
			entryMenu := groupMenu.AddSubMenuItem(a.menuItemTitle(item), itemMenuTooltip(item, info))
			switch a.showMenuState {
			case Click:
				entryMenu.Click(func() {
					global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
					a.writer <- item
				})
			case RClick:
				if !a.addItemSubMenu(entryMenu, item, group.History, "分组历史记录项", info) {
					entryMenu.Click(func() {
						global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("复制分组历史记录项: %s", formatMenuItem(item))}
						a.writer <- item
					})
				}
			}
		}
	}

	return addedGroups > 0
}
