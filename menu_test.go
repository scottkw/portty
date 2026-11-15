package main

import (
	"reflect"
	"strings"
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

// recordMenuEmit swaps menuEmit for a recorder for the duration of the test.
func recordMenuEmit(t *testing.T) *[]string {
	t.Helper()
	var got []string
	orig := menuEmit
	menuEmit = func(event string) { got = append(got, event) }
	t.Cleanup(func() { menuEmit = orig })
	return &got
}

func TestAppMenu_Structure(t *testing.T) {
	items := appMenu().Items
	if len(items) != 6 {
		t.Fatalf("top-level items = %d, want 6", len(items))
	}
	// macOS shows the first submenu as the application menu.
	if items[0].SubMenu == nil || len(items[0].SubMenu.Items) == 0 || items[0].SubMenu.Items[0].Label != "Settings…" {
		t.Errorf("first submenu does not start with Settings…: %+v", items[0])
	}
	if items[1].Label != "File" {
		t.Errorf("items[1] = %q, want File", items[1].Label)
	}
	if items[2].Role != menu.EditMenuRole {
		t.Errorf("items[2] role = %v, want Edit", items[2].Role)
	}
	if items[3].Label != "View" {
		t.Errorf("items[3] = %q, want View", items[3].Label)
	}
	if items[4].Role != menu.WindowMenuRole {
		t.Errorf("items[4] role = %v, want Window", items[4].Role)
	}
	if items[5].Label != "Help" || items[5].SubMenu == nil {
		t.Fatalf("items[5] = %q, want Help submenu", items[5].Label)
	}
	var help []string
	for _, it := range items[5].SubMenu.Items {
		help = append(help, it.Label)
	}
	want := []string{"Portty Help", "Check for Updates", "AgentHub on GitHub"}
	if strings.Join(help, "|") != strings.Join(want, "|") {
		t.Errorf("Help items = %v, want %v", help, want)
	}
}

func TestAppMenu_Accelerators(t *testing.T) {
	m := appMenu()
	cases := []struct {
		sub, label, key string
		mods            []keys.Modifier
	}{
		{"AgentHub", "Settings…", ",", []keys.Modifier{keys.CmdOrCtrlKey}},
		{"AgentHub", "Hide AgentHub", "h", []keys.Modifier{keys.CmdOrCtrlKey}},
		{"AgentHub", "Quit AgentHub", "q", []keys.Modifier{keys.CmdOrCtrlKey}},
		{"File", "New Session", "t", []keys.Modifier{keys.CmdOrCtrlKey}},
		{"File", "Close Tab", "w", []keys.Modifier{keys.CmdOrCtrlKey}},
		{"View", "Toggle Sidebar", "s", []keys.Modifier{keys.CmdOrCtrlKey, keys.ControlKey}},
	}
	for _, c := range cases {
		acc := findMenuItem(t, m, c.sub, c.label).Accelerator
		if acc == nil || acc.Key != c.key || !reflect.DeepEqual(acc.Modifiers, c.mods) {
			t.Errorf("%s > %s accelerator = %+v, want key %q mods %v", c.sub, c.label, acc, c.key, c.mods)
		}
	}
}

func TestAppMenu_EventsEmitted(t *testing.T) {
	got := recordMenuEmit(t)
	m := appMenu()
	for _, p := range [][2]string{
		{"AgentHub", "Settings…"},
		{"File", "New Session"},
		{"File", "Close Tab"},
		{"View", "Toggle Sidebar"},
	} {
		item := findMenuItem(t, m, p[0], p[1])
		item.Click(&menu.CallbackData{MenuItem: item})
	}
	want := []string{"menu:settings", "menu:new-session", "menu:close-tab", "menu:toggle-sidebar"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("emitted %v, want %v", *got, want)
	}
}

func TestAppMenu_HelpOpensPorttyURL(t *testing.T) {
	var got []string
	orig := menuOpenURL
	menuOpenURL = func(url string) { got = append(got, url) }
	t.Cleanup(func() { menuOpenURL = orig })

	item := findMenuItem(t, appMenu(), "Help", "Portty Help")
	item.Click(&menu.CallbackData{MenuItem: item})
	if len(got) != 1 || got[0] != "https://portty.app" {
		t.Fatalf("opened %v, want [https://portty.app]", got)
	}
}
