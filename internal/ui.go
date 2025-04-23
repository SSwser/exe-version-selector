package internal

import (
	"fmt"
	"os"
	"time"

	"github.com/getlantern/systray"
)

// 通用菜单结构体
// OnClick, OnRefresh 可为 nil
// SubMenus 支持分组和递归
// Extra 可用于传递自定义数据

type MenuConfig struct {
	Title     string
	Tooltip   string
	OnClick   func(item *systray.MenuItem)
	OnRefresh func(item *systray.MenuItem)
	SubMenus  []MenuConfig
	Extra     map[string]interface{}
}

// 递归生成菜单分组并注册事件
func CreateMenuFromConfig(cfg MenuConfig) *systray.MenuItem {
	item := systray.AddMenuItem(cfg.Title, cfg.Tooltip)
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
			// 注意：递归生成子菜单
			CreateMenuFromConfigRecursive(subItem, sub)
		}
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
	}
}

// 支持统一刷新所有菜单项
// 用于定时刷新所有注册的菜单项内容
var menuRefreshers []func()

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
	systray.SetTooltip("当前应用: " + current)
}

// 显示错误信息
func ShowError(msg string) {
	fmt.Fprintf(os.Stderr, "[错误] %s\n", msg)
}
