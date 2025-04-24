package internal

import (
	"fmt"
	"os"
	"time"

	"github.com/getlantern/systray"
)

// 用于定时刷新所有注册的菜单项内容,支持统一刷新所有菜单项
var menuRefreshers []func()

// 通用菜单结构体
// OnClick, OnRefresh 可为 nil
// SubMenus 支持分组和递归

type MenuConfig struct {
	Title     string
	Tooltip   string
	OnClick   func(item *systray.MenuItem)
	OnRefresh func(item *systray.MenuItem)
	SubMenus  []MenuConfig
	Disable   bool // 是否禁用此项
	Separator bool // 是否在此项下添加分隔符
}

// 递归生成菜单分组并注册事件
func CreateMenuFromConfig(cfg MenuConfig) *systray.MenuItem {
	item := systray.AddMenuItem(cfg.Title, cfg.Tooltip)

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

	for _, sub := range cfg.SubMenus {
		subItem := item.AddSubMenuItem(sub.Title, sub.Tooltip)
		if sub.Disable {
			subItem.Disable()
		}
		if sub.OnClick != nil {
			go func(s *systray.MenuItem, handler func(*systray.MenuItem)) {
				for {
					<-s.ClickedCh
					handler(s)
				}
			}(subItem, sub.OnClick)
		}

		// 递归子菜单
		if len(sub.SubMenus) > 0 {
			CreateMenuFromConfigRecursive(subItem, sub)
		}
	}

	// 如果 Separator 为真，则在此项下添加分隔符
	if cfg.Separator {
		systray.AddSeparator()
	}

	return item
}

// 子菜单递归辅助
func CreateMenuFromConfigRecursive(parent *systray.MenuItem, cfg MenuConfig) {
	for _, sub := range cfg.SubMenus {
		subItem := parent.AddSubMenuItem(sub.Title, sub.Tooltip)
		if sub.OnClick != nil {
			go func(s *systray.MenuItem, handler func(*systray.MenuItem)) {
				for {
					<-s.ClickedCh
					handler(s)
				}
			}(subItem, sub.OnClick)
		}
		if len(sub.SubMenus) > 0 {
			CreateMenuFromConfigRecursive(subItem, sub)
		}
		if sub.Separator {
			systray.AddSeparator()
		}
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
			time.Sleep(time.Second)
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

// 显示错误信息
func ShowError(msg string) {
	fmt.Fprintf(os.Stderr, "[错误] %s\n", msg)
}
