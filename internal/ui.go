package internal

import (
	"time"

	"github.com/getlantern/systray"
)

// 用于定时刷新所有注册的菜单项内容,支持统一刷新所有菜单项
var menuRefreshers []func()

// 通用菜单结构体
// OnClick, OnRefresh 可为 nil
// SubMenus 支持分组和递归

type MenuItemData struct {
	Title     string
	Tooltip   string
	Disable   bool // 是否禁用此项
	Separator bool // 是否在此项下添加分隔符
	SubMenus  []MenuItemData
	OnClick   func(item *systray.MenuItem)
	OnRefresh func(item *systray.MenuItem)
}

// 菜单项递归结构体，绑定配置与菜单项对象
// 用于统一刷新所有 OnRefresh 菜单项
type MenuEntry struct {
	Config *MenuItemData
	Item   *systray.MenuItem
	Subs   []*MenuEntry
}

var RootMenuEntries []*MenuEntry

// 递归生成菜单分组并注册事件，并返回 MenuEntry
func CreateMenuFromConfig(cfg MenuItemData) *MenuEntry {
	item := systray.AddMenuItem(cfg.Title, cfg.Tooltip)
	entry := &MenuEntry{Config: &cfg, Item: item}

	// 禁用支持
	if cfg.Disable {
		item.Disable()
	}

	if cfg.OnClick != nil {
		go func() {
			for {
				<-item.ClickedCh
				cfg.OnClick(item)
			}
		}()
	}

	for i := range cfg.SubMenus {
		subEntry := CreateMenuFromConfig(cfg.SubMenus[i])
		entry.Subs = append(entry.Subs, subEntry)
	}

	// 如果 Separator 为真，则在此项下添加分隔符
	if cfg.Separator {
		systray.AddSeparator()
	}

	return entry
}

// 递归刷新所有菜单项的 OnRefresh
func internalRefreshEntry(entry *MenuEntry) {
	if entry.Config.OnRefresh != nil {
		entry.Config.OnRefresh(entry.Item)
	}
	for _, sub := range entry.Subs {
		internalRefreshEntry(sub)
	}
}

func RefreshAllMenuItems() {
	for _, entry := range RootMenuEntries {
		internalRefreshEntry(entry)
	}
}

func RegisterMenuRefresher(f func()) {
	menuRefreshers = append(menuRefreshers, f)
}

func StartMenuRefresher() {
	go func() {
		for {
			for _, f := range menuRefreshers {
				f()
			}

			RefreshAllMenuItems() // 统一刷新所有菜单项
			time.Sleep(time.Second * 2)
		}
	}()
}

// 标题和 tooltip 刷新复用
func UpdateTrayTitle(current string) {
	systray.SetTitle("EVS: " + current)
	// Windows 下频繁 SetTooltip 可能报错，可选择注释掉或捕获异常
	defer func() { _ = recover() }()
	// systray.SetTooltip("当前应用: " + current) // 避免 Windows 报错
}
