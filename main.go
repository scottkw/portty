// Portty — unified binary: desktop GUI, CLI commands, and daemon mode.
//
// Dispatch strategy (locked in STATE.md):
//   - No args, or first arg starts with "-" (except --help/-h) → GUI
//   - --help or -h → print usage and exit
//   - Otherwise → CLI command dispatch
//
// Build for production:  wails build
// Run in dev mode:       wails dev
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is injected at build time via:
//
//	wails build -ldflags "-X main.Version=v1.9.0"
//
// Falls back to "dev" in local dev builds.
var Version = "dev"

// appCtx holds the Wails runtime context for use in menu callbacks.
// Set in app.startup() after Wails initialises.
var appCtx context.Context

// appInstance holds the App pointer for use in menu callbacks that need
// App methods (not just the Wails runtime context).
var appInstance *App

func main() {
	daemon.BuildVersion = Version
	// Phase 120-04 — hand the embedded React bundle FS to the daemon package so
	// the daemon-mode invocation (`portty daemon`) can mount it on the
	// webserver's /app/ route. In dev builds (no wailsassets tag) this
	// returns nil; the webserver answers /app/ with 503. In prod builds
	// (wailsassets) this returns the same fs.FS used by Wails for the
	// desktop bundle, mounted under /app/ for remote/web-share viewers.
	daemon.SetStaticAppFS(staticAppForDaemon())

	// GUI mode: no args, or first arg is a flag (except help).
	if len(os.Args) == 1 || strings.HasPrefix(os.Args[1], "-") {
		if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
			usage()
			return
		}
		runGUI()
		return
	}
	// CLI mode: dispatch to command handler.
	runCLI(os.Args[1:])
}

func runGUI() {
	daemon.AugmentServicePath() // Put the Tailscale CLI on PATH when launched from Finder or the Dock (internal/daemon/path.go)
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "Portty",
		Width:             1200,
		Height:            800,
		MinWidth:          800,
		MinHeight:         600,
		HideWindowOnClose: true,
		StartHidden:       true,
		// Alpha 0 so the native vibrancy shows through; nil would default to white.
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Mac:              macWindowOptions(),
		Menu:             appMenu(),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		panic(err)
	}
}

// macWindowOptions: hidden-inset title bar (traffic lights over the sidebar
// strip), translucent window behind a transparent webview for sidebar vibrancy.
// Appearance is left unset so the window follows the system.
func macWindowOptions() *mac.Options {
	return &mac.Options{
		TitleBar:             mac.TitleBarHiddenInset(),
		WebviewIsTransparent: true,
		WindowIsTranslucent:  true,
	}
}

// menuEmit, menuOpenURL and menuShowAbout are seams so tests can record menu
// actions without a Wails runtime.
var menuEmit = func(event string) {
	if appCtx != nil {
		runtime.EventsEmit(appCtx, event)
	}
}

var menuOpenURL = func(url string) {
	if appCtx != nil {
		runtime.BrowserOpenURL(appCtx, url)
	}
}

var menuShowAbout = func() { showNativeAboutPanel() }

// emitMenuEvent returns a menu callback that emits event to the frontend.
func emitMenuEvent(event string) menu.Callback {
	return func(_ *menu.CallbackData) { menuEmit(event) }
}

// helpURL is where Help > Portty Help points (docs land in Phase 3).
const helpURL = "https://portty.app"

// appMenu constructs the macOS application menu bar.
// Ordering still matters on macOS: the FIRST submenu is shown as the
// application menu (STATE.md ordering pitfall). It is a custom submenu, not
// the built-in AppMenu role, because Go cannot insert Settings… into the role.
func appMenu() *menu.Menu {
	m := menu.NewMenu()
	// 1. Application menu — must stay first.
	appSub := m.AddSubmenu("Portty")
	appSub.AddText("About Portty", nil, func(_ *menu.CallbackData) { menuShowAbout() })
	appSub.AddSeparator()
	appSub.AddText("Settings…", keys.CmdOrCtrl(","), emitMenuEvent("menu:settings"))
	appSub.AddSeparator()
	appSub.AddText("Hide Portty", keys.CmdOrCtrl("h"), func(_ *menu.CallbackData) {
		if appCtx != nil {
			runtime.Hide(appCtx)
		}
	})
	appSub.AddSeparator()
	// runtime.Quit runs OnBeforeClose first, so the quit-confirm modal still owns quitting.
	appSub.AddText("Quit Portty", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		if appCtx != nil {
			runtime.Quit(appCtx)
		}
	})
	// 2. File menu (custom — FileMenuRole is commented out in v2.10.2)
	fileMenu := m.AddSubmenu("File")
	fileMenu.AddText("New Session", keys.CmdOrCtrl("t"), emitMenuEvent("menu:new-session"))
	fileMenu.AddSeparator()
	fileMenu.AddText("Close Tab", keys.CmdOrCtrl("w"), emitMenuEvent("menu:close-tab"))
	// 3. EditMenu — enables Cmd+C/V/X/Z via native NSMenu (MENU-02)
	m.Append(menu.EditMenu())
	// 4. View menu — static label; Wails v2 labels only change on a menu rebuild.
	viewMenu := m.AddSubmenu("View")
	viewMenu.AddText("Toggle Sidebar", keys.Combo("s", keys.CmdOrCtrlKey, keys.ControlKey), emitMenuEvent("menu:toggle-sidebar"))
	// 5. Window menu
	m.Append(menu.WindowMenu())
	// 6. Help menu (custom — HelpSubMenuRole is commented out in v2.10.2)
	helpMenu := m.AddSubmenu("Help")
	helpMenu.AddText("Portty Help", nil, func(_ *menu.CallbackData) { menuOpenURL(helpURL) })
	helpMenu.AddText("Check for Updates", nil, checkForUpdatesCallback)
	return m
}

// checkForUpdatesCallback triggers an immediate update check from the Help menu.
// Runs in a goroutine to avoid blocking the UI thread.
func checkForUpdatesCallback(_ *menu.CallbackData) {
	if appInstance != nil {
		go appInstance.CheckForUpdates()
	}
}

func runCLI(args []string) {
	if len(args) == 0 {
		usage()
		return
	}
	cmd := args[0]

	// Daemon sub-command: run daemon mode without EnsureDaemon, or manage service lifecycle.
	// daemon status needs a running daemon — fall through to EnsureDaemon path.
	if cmd == "daemon" && len(args) > 1 && args[1] == "status" {
		// handled below in switch after EnsureDaemon
	} else if cmd == "daemon" {
		if err := cmdDaemon(args[1:], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	// All other commands: auto-start daemon, create client, dispatch.
	socketPath := daemon.DefaultSocketPath()
	if err := daemon.EnsureDaemon(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "portty: %v\n", err)
		os.Exit(1)
	}
	client := daemon.NewDaemonClient(socketPath)

	cmdArgs := args[1:]
	var err error

	switch cmd {
	case "new":
		err = cmdNew(client, cmdArgs, os.Stdout)
	case "list":
		err = cmdList(client, cmdArgs, os.Stdout)
	case "kill":
		err = cmdKill(client, cmdArgs)
	case "rename":
		err = cmdRename(client, cmdArgs)
	case "attach":
		err = cmdAttach(client, cmdArgs)
	case "web":
		err = cmdWeb(client, cmdArgs, os.Stdout)
	case "serve":
		err = cmdServe(client, cmdArgs)
	case "unserve":
		err = cmdUnserve(client, cmdArgs)
	case "health":
		err = cmdHealth(cmdArgs, os.Stdout)
	case "qr":
		err = cmdQR(client, cmdArgs, os.Stdout)
	case "settings":
		err = cmdSettings(client, os.Stdout)
	case "daemon":
		// Only "daemon status" reaches here (others handled above).
		err = cmdDaemonStatus(client, cmdArgs[1:], os.Stdout)
	default:
		fmt.Fprint(os.Stderr, unknownCommandMessage(cmd))
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
