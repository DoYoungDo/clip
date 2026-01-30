package main

import (
	"fmt"
	"sync"

	"github.com/energye/systray"
	"golang.design/x/clipboard"
)

var (
	history        *History
	menuItems      []*systray.MenuItem
	groupMenus     []*systray.MenuItem
	groupSubItems  [][]*systray.MenuItem // 每个分组的子菜单项
	createGroupBtn *systray.MenuItem
	maxMenuSize    = 20
	maxGroupSize   = 10
	updating       bool
	updateMutex    sync.Mutex
)

func main() {
	history = NewHistory(50)
	fmt.Println("程序启动中...")
	systray.Run(onReady, onExit)
}

func onReady() {
	fmt.Println("托盘初始化...")
	
	// 设置图标（跨平台）
	systray.SetIcon(iconData)
	
	// 预创建历史菜单项
	for i := 0; i < maxMenuSize; i++ {
		menuItem := systray.AddMenuItem("", "点击复制")
		menuItem.Hide()
		menuItems = append(menuItems, menuItem)
		
		idx := i
		menuItem.Click(func() {
			items := history.GetAll()
			if idx < len(items) {
				copyItem(items[idx])
			}
		})
	}
	
	systray.AddSeparator()
	createGroupBtn = systray.AddMenuItem("➕ 创建分组", "使用最新剪贴板内容作为分组名")
	
	// 预创建分组菜单项
	for i := 0; i < maxGroupSize; i++ {
		groupMenu := systray.AddMenuItem("", "分组")
		groupMenu.Hide()
		groupMenus = append(groupMenus, groupMenu)
		
		// 为每个分组预创建子菜单项
		var subItems []*systray.MenuItem
		for j := 0; j < maxMenuSize; j++ {
			subItem := groupMenu.AddSubMenuItem("", "点击复制")
			subItem.Hide()
			subItems = append(subItems, subItem)
			
			groupIdx := i
			itemIdx := j
			subItem.Click(func() {
				groups := history.GetGroups()
				if groupIdx < len(groups) && itemIdx < len(groups[groupIdx].Items) {
					copyItem(groups[groupIdx].Items[itemIdx])
				}
			})
		}
		groupSubItems = append(groupSubItems, subItems)
		
		// 为每个分组添加操作按钮
		btnActive := groupMenu.AddSubMenuItem("激活/取消激活分组", "")
		btnRename := groupMenu.AddSubMenuItem("重命名", "")
		btnDelete := groupMenu.AddSubMenuItem("删除分组", "")
		
		idx := i
		btnActive.Click(func() {
			history.ToggleGroupActive(idx)
			updateMenu()
		})
		
		btnRename.Click(func() {
			name := history.GetLatestText()
			if len(name) > 20 {
				name = name[:20] + "..."
			}
			history.RenameGroup(idx, name)
			updateMenu()
		})
		
		btnDelete.Click(func() {
			history.DeleteGroup(idx)
			updateMenu()
		})
	}
	
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")
	
	fmt.Println("菜单已创建")
	
	go startMonitor(history, updateMenu)
	
	createGroupBtn.Click(func() {
		name := history.GetLatestText()
		if len(name) > 20 {
			name = name[:20] + "..."
		}
		history.CreateGroup(name)
		updateMenu()
	})
	
	mQuit.Click(func() {
		fmt.Println("退出程序")
		systray.Quit()
	})
	
	fmt.Println("托盘初始化完成")
}

func onExit() {
}

func updateMenu() {
	updateMutex.Lock()
	if updating {
		updateMutex.Unlock()
		return // 正在更新中，跳过
	}
	updating = true
	updateMutex.Unlock()
	
	defer func() {
		updateMutex.Lock()
		updating = false
		updateMutex.Unlock()
	}()
	
	// 只更新已存在的菜单项，不重新创建
	items := history.GetAll()
	
	// 更新历史菜单项
	for i := 0; i < maxMenuSize; i++ {
		if i < len(menuItems) {
			if i < len(items) {
				menuItems[i].SetTitle(formatMenuItem(items[i]))
				menuItems[i].Show()
			} else {
				menuItems[i].Hide()
			}
		}
	}
	
	// 更新分组
	groups := history.GetGroups()
	for i := 0; i < len(groupMenus); i++ {
		if i < len(groups) {
			activeIcon := ""
			if groups[i].Active {
				activeIcon = "✓ "
			}
			groupMenus[i].SetTitle(activeIcon + "📁 " + groups[i].Name)
			groupMenus[i].Show()
			
			// 更新分组内的子菜单项
			groupItems := groups[i].Items
			for j := 0; j < maxMenuSize; j++ {
				if j < len(groupSubItems[i]) {
					if j < len(groupItems) {
						groupSubItems[i][j].SetTitle(formatMenuItem(groupItems[j]))
						groupSubItems[i][j].Show()
					} else {
						groupSubItems[i][j].Hide()
					}
				}
			}
		} else {
			groupMenus[i].Hide()
		}
	}
}

func formatMenuItem(item *ClipItem) string {
	text := item.Text
	var prefix string
	
	switch item.Type {
	case TypeText:
		prefix = "📝"
		if len(text) > 40 {
			text = text[:40] + "..."
		}
	case TypeImage:
		prefix = "🖼️"
	case TypeFile:
		prefix = "📁"
		if len(text) > 50 {
			text = "..." + text[len(text)-47:]
		}
	}
	
	return fmt.Sprintf("%s [%s] %s", prefix, item.Time.Format("15:04"), text)
}

func copyItem(clipItem *ClipItem) {
	// 不触发历史记录，因为这是用户主动从历史中复制的
	switch clipItem.Type {
	case TypeText, TypeFile:
		clipboard.Write(clipboard.FmtText, clipItem.Content)
	case TypeImage:
		clipboard.Write(clipboard.FmtImage, clipItem.Content)
	}
}
