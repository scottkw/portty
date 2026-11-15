package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

// findMenuItem returns the item labelled label inside the submenu labelled sub.
func findMenuItem(t *testing.T, m *menu.Menu, sub, label string) *menu.MenuItem {
	t.Helper()
	for _, top := range m.Items {
		if top.Label != sub || top.SubMenu == nil {
			continue
		}
		for _, item := range top.SubMenu.Items {
			if item.Label == label {
				return item
			}
		}
	}
	t.Fatalf("menu item %q > %q not found", sub, label)
	return nil
}

func TestMacWindowOptions(t *testing.T) {
	o := macWindowOptions()
	if o.TitleBar == nil {
		t.Fatal("TitleBar is nil")
	}
	// UseToolbar is not asserted: the spike may fall back to mac.TitleBarHidden().
	if !o.TitleBar.TitlebarAppearsTransparent || !o.TitleBar.HideTitle || !o.TitleBar.FullSizeContent {
		t.Errorf("TitleBar = %+v, want transparent, hidden title, full-size content", *o.TitleBar)
	}
	if !o.WebviewIsTransparent || !o.WindowIsTranslucent {
		t.Errorf("WebviewIsTransparent=%v WindowIsTranslucent=%v, want both true", o.WebviewIsTransparent, o.WindowIsTranslucent)
	}
	if o.Appearance != "" {
		t.Errorf("Appearance = %q, want empty (follow system)", o.Appearance)
	}
}

func TestAppMenu_NewSessionEmitsEvent(t *testing.T) {
	item := findMenuItem(t, appMenu(), "File", "New Session")
	acc := item.Accelerator
	if acc == nil || acc.Key != "t" || len(acc.Modifiers) != 1 || acc.Modifiers[0] != keys.CmdOrCtrlKey {
		t.Fatalf("New Session accelerator = %+v, want Cmd+T", acc)
	}

	var got []string
	orig := menuEmit
	menuEmit = func(event string) { got = append(got, event) }
	t.Cleanup(func() { menuEmit = orig })

	item.Click(&menu.CallbackData{MenuItem: item})
	if len(got) != 1 || got[0] != "menu:new-session" {
		t.Fatalf("emitted %v, want [menu:new-session]", got)
	}
}
