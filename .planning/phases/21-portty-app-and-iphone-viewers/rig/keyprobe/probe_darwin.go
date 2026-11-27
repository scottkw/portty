package main

// THROWAWAY Phase 21-02 key-routing probe. It lives under .planning and is copied into a
// scratch export of the tree by apply.sh; it must never reach the shipped source tree.

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

void probeBlankMenuKeys(int on);
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	probeStart = time.Now()
	probeMu    sync.Mutex
	probeSeq   int
)

func probeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err) // a probe with no home has nowhere to log: fail loudly
	}
	return filepath.Join(home, "p21")
}

// ProbeLog appends one line (sequence number, monotonic ms since start, the page's text).
func (a *App) ProbeLog(line string) {
	if testing.Testing() {
		return
	}
	probeMu.Lock()
	defer probeMu.Unlock()
	probeSeq++
	if err := os.MkdirAll(probeDir(), 0o700); err != nil {
		panic(err)
	}
	f, err := os.OpenFile(filepath.Join(probeDir(), "keyprobe.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "%d %d %s\n", probeSeq, time.Since(probeStart).Milliseconds(), line)
}

// ProbeMode returns the contents of ~/p21/keyprobe.mode: "MODE FOCUS EPOCH" (page|blank|rebuild,
// textarea|div|none, a counter the rig bumps to re-apply the focus target). Missing file: defaults.
func (a *App) ProbeMode() string {
	if testing.Testing() {
		return "page textarea 0"
	}
	b, err := os.ReadFile(filepath.Join(probeDir(), "keyprobe.mode"))
	if err != nil {
		if os.IsNotExist(err) {
			return "page textarea 0"
		}
		panic(err)
	}
	return strings.TrimSpace(string(b))
}

// ProbeBlankMenuKeys clears (on) or restores (off) every key equivalent in NSApp.mainMenu.
func (a *App) ProbeBlankMenuKeys(on bool) {
	if testing.Testing() {
		return
	}
	v := 0
	if on {
		v = 1
	}
	C.probeBlankMenuKeys(C.int(v))
}

// stripAccelerators returns appMenu() with no accelerator anywhere. The Edit and Window role
// menus carry native key equivalents the Go side cannot strip, so they are replaced by plain
// label-only submenus (the probe asks whether a menu without accelerators still leaks).
func stripAccelerators() *menu.Menu {
	m := appMenu()
	for _, it := range m.Items {
		it.Accelerator = nil
		if it.SubMenu == nil {
			continue
		}
		if it.Role == menu.EditMenuRole || it.Role == menu.WindowMenuRole {
			it.Role = 0
			it.SubMenu = menu.NewMenu()
			it.SubMenu.AddText("(probe: "+it.Label+" role removed)", nil, nil)
			continue
		}
		for _, sub := range it.SubMenu.Items {
			sub.Accelerator = nil
		}
	}
	return m
}

// ProbeRebuildMenu swaps in the accelerator-free menu (on) or the real appMenu() (off).
func (a *App) ProbeRebuildMenu(on bool) {
	if testing.Testing() || appCtx == nil {
		return
	}
	if on {
		runtime.MenuSetApplicationMenu(appCtx, stripAccelerators())
	} else {
		runtime.MenuSetApplicationMenu(appCtx, appMenu())
	}
	runtime.MenuUpdateApplicationMenu(appCtx)
}
