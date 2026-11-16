package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottkw/agenthub/internal/daemon"
	"github.com/scottkw/agenthub/internal/tailnet"
	"github.com/scottkw/agenthub/internal/updater"
	"github.com/scottkw/agenthub/internal/webserver"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SessionInfo is the JSON-serialisable representation of a session returned by ListSessions.
type SessionInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	State      string `json:"state"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	Hostname   string `json:"hostname"`
	WebEnabled bool   `json:"webEnabled"`
	// Phase 165 / FNL-01: true when Tailscale Funnel is active for this session.
	// NOT omitempty: false must serialize so the frontend poll detects expiry
	// (silent false-drop is a UAT-class bug).
	FunnelActive bool `json:"funnelActive"`
	// Phase 171 / FNL-09: true when the gate-minted public write cap (consent-gated
	// Full Access write-share) is active for this session. NOT omitempty: false must
	// serialize so the frontend poll detects a write-share teardown flip (same rule
	// as FunnelActive — silent false-drop is a UAT-class bug).
	FunnelWriteActive bool `json:"funnelWriteActive"`
	// Phase 131 — Hub card fields (CARD-04, CARD-05, CARD-06, GRID-02).
	// Omitting any of these silently drops them to zero in the Wails RPC
	// response — same class of UAT bug documented on FunnelActive above.
	ViewerCount int    `json:"viewerCount"`
	ExitCode    *int   `json:"exitCode,omitempty"`
	Duration    *int   `json:"duration,omitempty"`
	WorkDir     string `json:"workDir"`
}

// RemoteSession is a session on a remote tailnet peer.
type RemoteSession struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// RemotePeerSessions groups sessions by peer hostname.
// Reachable discriminates unreachable peers (Reachable=false) from reachable
// peers with zero shareable sessions (Reachable=true, len(Sessions)==0), enabling
// honest per-peer states in the Remote Sessions panel (RB-04).
type RemotePeerSessions struct {
	Hostname  string          `json:"hostname"`
	Reachable bool            `json:"reachable"`
	Sessions  []RemoteSession `json:"sessions"`
}

// App holds all application state and exposes the Wails-bound methods.
// App is a thin Wails-binding shell — all session state lives in the daemon process.
// All session operations are delegated through DaemonClient over the Unix socket.
type App struct {
	ctx              context.Context
	client           *daemon.DaemonClient // only daemon communication field; set in NewApp, before Wails starts any goroutine (nil only in tests)
	trayInit         bool                 // true once initTray has been called
	lastTrayQuartile atomic.Int32         // Phase 98 PRG-03 — last applied tray progress quartile [0..4]; -1 = unset. Atomic because written by Wails RPC goroutine (SetTrayProgress) and read by startTrayPoller goroutine via trayIconBytesForState (CR-01).
	daemonErr        error                // non-nil when EnsureDaemon failed; read only after startupDone (awaitStartup)
	quitting         bool                 // true when tray Quit was clicked; lets beforeClose allow exit
	// Update checker state
	lastUpdate   *updater.UpdateInfo
	lastUpdateMu sync.Mutex

	// saveFileDialogFunc allows unit tests to mock runtime.SaveFileDialog.
	// Defaults to runtime.SaveFileDialog. Phase 97 SER-01 / PROJECT.md
	// "Function injection" pattern, parallel to serviceControlFunc and
	// statusFunc.
	saveFileDialogFunc func(ctx context.Context, opts runtime.SaveDialogOptions) (string, error)

	// refreshTrayStateFunc allows unit tests to mock refreshTrayState().
	// Defaults to nil (production path calls a.refreshTrayState() directly).
	// Phase 98 PRG-03 / PROJECT.md "Function injection" pattern.
	refreshTrayStateFunc func()

	// sendNotificationFunc is the one native notification path (quit-to-tray
	// notice now, Phase 6 update notices later). Defaults to sendNotification;
	// tests swap it so no real OS notification fires.
	sendNotificationFunc func(identifier, title, body string)

	// emitEventFunc allows unit tests to capture Wails events. nil in
	// production (emitEvent calls runtime.EventsEmit). Mirrors the
	// sendNotificationFunc injection pattern.
	emitEventFunc func(name string, data any)

	// tailscaleCheckFunc is the Tailscale health check seam; nil means
	// webserver.CheckHealth. lastHealth is the last emitted result (nil until
	// the first check); the poller and GetTailscaleStatus share it under healthMu.
	tailscaleCheckFunc func(context.Context) webserver.TailscaleHealth
	healthMu           sync.Mutex
	lastHealth         *webserver.TailscaleHealth

	// openAppFunc and openURLFunc are the OpenTailscale seams; nil means
	// `/usr/bin/open -a <name>` (openAppCmd) and runtime.BrowserOpenURL.
	openAppFunc func(name string) error
	openURLFunc func(url string)

	// applyAppearanceFunc is the native Appearance seam; nil means
	// setNativeAppearance (NSApp.appearance).
	applyAppearanceFunc func(mode string)
	// appearanceMu makes each persist-and-apply (or read-and-apply) atomic, so
	// concurrent binding calls apply natively in the order they persisted.
	appearanceMu sync.Mutex

	// startupDone is closed once startup has the daemon up (or failed) and has
	// applied the stored Appearance. Wails runs OnStartup on a goroutine that
	// races domReady and the bindings, so they wait on it (WR-07).
	startupDone chan struct{}

	// ensureDaemonFunc and showWindowFunc are startup seams; nil means
	// daemon.EnsureDaemon and runtime.WindowShow plus the Dock icon.
	ensureDaemonFunc func(socketPath string) error
	showWindowFunc   func()

	// seenSessions is every id the exit watcher has listed and not yet
	// reported via session:exit. Owned by the exit watcher goroutine.
	seenSessions map[string]bool
}

// NewApp creates a new App without starting any subsystems.
func NewApp() *App {
	a := &App{
		saveFileDialogFunc:   runtime.SaveFileDialog,
		sendNotificationFunc: sendNotification,
		startupDone:          make(chan struct{}),
		// The client only dials on use. Creating it here, before wails.Run,
		// gives every Wails goroutine a happens-before edge on it.
		client: daemon.NewDaemonClient(daemon.DefaultSocketPath()),
	}
	a.lastTrayQuartile.Store(-1) // Phase 98 PRG-03 — ensure first SetTrayProgress call always updates
	return a
}

// startupWait bounds domReady's wait for startup. domReady runs on the Wails
// message loop, so a wedged daemon must not block it forever (review WR-05).
var startupWait = 5 * time.Second

// startupBindingWait bounds how long bindings wait for startup. It covers
// EnsureDaemon's worst case with 30 s client timeouts. Bindings run on their
// own goroutines, so this wait never stalls the message loop (review WR-07).
var startupBindingWait = 90 * time.Second

var errStartupTimeout = errors.New("daemon startup timed out")

// awaitStartup blocks until startup closes startupDone, giving the caller a
// happens-before edge on daemonErr. It returns false if startupBindingWait
// passes first. Call it only from binding goroutines, never from domReady.
func (a *App) awaitStartup() bool {
	if a.startupDone == nil {
		return true
	}
	select {
	case <-a.startupDone:
		return true
	case <-time.After(startupBindingWait):
		return false
	}
}

// domReady is called by Wails after the WebView DOM is ready.
// Shows the window unless the persisted start-minimized preference is set.
// Falls back to showing the window when the daemon is unreachable (safe default).
func (a *App) domReady(ctx context.Context) {
	startMinimized := false
	ready := true
	if a.startupDone != nil {
		// The stored Appearance should land before the first visible paint.
		select {
		case <-a.startupDone:
		case <-time.After(startupWait):
			ready = false // daemon slow or wedged: show the window without another RPC
		}
	}
	if ready && a.client != nil {
		if val, err := a.client.GetStartMinimized(); err == nil {
			startMinimized = val
		}
	}
	if !startMinimized {
		if a.showWindowFunc != nil {
			a.showWindowFunc()
			return
		}
		runtime.WindowShow(ctx)
		a.setDockVisible(true)
	}
}

// startup is called when Wails initialises the app.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	appCtx = ctx    // expose to menu callbacks (openGitHubCallback)
	appInstance = a // expose to menu callbacks (checkForUpdatesCallback)

	// Start system tray icon immediately — it must be visible regardless of
	// daemon state. The poller will set the error icon if daemon is unreachable.
	a.initTray()
	a.trayInit = true

	// a.client was set in NewApp and is never reassigned, so the pollers read
	// it lock-free and RetryDaemon only has to bring the daemon up. While it is
	// down, calls fail and pollers retry.
	a.startDaemon(daemon.DefaultSocketPath())

	// One app-wide exit watcher closes tabs for every exited session (TERM-05).
	a.startExitWatcher(ctx)

	// Start tray state poller (updates icon, tooltip, session list every 5s).
	a.startTrayPoller(ctx)

	// Start Tailscale health check background poller.
	a.startHealthPoller(ctx)

	// Start update checker background poller.
	a.startUpdatePoller(ctx)
}

// startDaemon brings the daemon up for startup and closes startupDone on every
// return path, so domReady and the bindings never wait out their bounds.
func (a *App) startDaemon(socketPath string) {
	if a.startupDone != nil {
		defer close(a.startupDone)
	}
	ensure := a.ensureDaemonFunc
	if ensure == nil {
		ensure = daemon.EnsureDaemon
	}
	if err := ensure(socketPath); err != nil {
		a.daemonErr = err
		// Notify frontend — window is already rendered when OnStartup runs (goroutine).
		a.emitEvent("daemon:error", err.Error())
	} else {
		a.applyStoredAppearance()
	}
}

// GetDaemonError returns the startup error message, or "" if startup succeeded.
// Called by the frontend on mount to detect a failed startup (the daemon:error
// event may fire before React subscribes, so this is the reliable path). It
// waits for startup, so init never sees a half-started daemon (WR-07).
func (a *App) GetDaemonError() string {
	if !a.awaitStartup() {
		return errStartupTimeout.Error()
	}
	if a.daemonErr != nil {
		return a.daemonErr.Error()
	}
	return ""
}

// GetVersion returns the application version injected at build time.
// Returns "dev" in local development builds.
func (a *App) GetVersion() string {
	return Version
}

// RetryDaemon re-attempts daemon spawn after a startup failure.
// Called by the frontend "Retry Connection" button. It neither replaces
// a.client nor starts pollers: startup did both once (WR-05).
func (a *App) RetryDaemon() error {
	if !a.awaitStartup() {
		return errStartupTimeout // never run a second EnsureDaemon beside startup's
	}
	if err := daemon.EnsureDaemon(daemon.DefaultSocketPath()); err != nil {
		a.daemonErr = err
		return err
	}
	a.daemonErr = nil
	a.applyStoredAppearance() // startup skipped it while the daemon was down
	return nil
}

// shutdown is called when the Wails app is about to exit.
func (a *App) shutdown(_ context.Context) {
	// Remove the system tray icon before cleaning up other resources.
	if a.trayInit {
		a.cleanupTray()
	}
	// Daemon is an independent process — GUI does NOT stop it.
	// Sessions persist after GUI exits (DAEMON-03).
}

// beforeClose intercepts the window close button and emits a Wails event so
// the frontend can show a quit confirmation modal (D-12). The window is shown
// first if hidden (D-08) so the modal is always visible to the user.
// When called outside a Wails context (e.g., in unit tests), the event emit
// is skipped safely — sessions are unaffected in both cases.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitting {
		return false // QuitAll already set flag — allow quit
	}
	// Wails stores the frontend under the "frontend" key; skip the call when
	// running outside the Wails event loop (tests, CLI helpers).
	if ctx.Value("frontend") != nil {
		runtime.WindowShow(ctx) // D-08: ensure window visible for modal
		a.setDockVisible(true)
		runtime.EventsEmit(ctx, "app:quit-requested", nil)
	}
	return true // always prevent default quit — modal owns the decision
}

// QuitGUIOnly hides the GUI window to the system tray without stopping
// the daemon or any active sessions. Sends a macOS notification confirming
// the app is still running in the background (D-10, D-11).
func (a *App) QuitGUIOnly() {
	if a.ctx == nil {
		return
	}
	a.setDockVisible(false)
	runtime.WindowHide(a.ctx)
	// Send macOS notification with session count (D-11)
	sessions := a.ListSessions()
	count := 0
	for _, s := range sessions {
		if s.Status != "" && s.Status != "stopped" {
			count++
		}
	}
	var body string
	if count == 1 {
		body = "AgentHub is still running in the background. 1 session active."
	} else {
		body = fmt.Sprintf("AgentHub is still running in the background. %d sessions active.", count)
	}
	a.sendNotificationFunc("agenthub.quit-gui-only", "AgentHub", body)
}

// QuitAll shuts down the daemon (terminating all sessions) and quits the
// application completely (D-09, APP-02).
func (a *App) QuitAll() {
	if a.ctx == nil {
		return
	}
	if a.client != nil {
		_ = a.client.ShutdownDaemon()
	}
	a.quitting = true
	runtime.Quit(a.ctx)
}

// --- Wails-bound methods ---

// CreateSession spawns a new login-shell session and returns its ID.
// Empty workDir starts in the home directory.
// cols and rows specify the initial PTY dimensions; pass 0 for defaults (80x24).
// Exit detection is the app-wide exit watcher (startExitWatcher), which covers
// every session regardless of how or when it was created.
func (a *App) CreateSession(name, workDir string, cols, rows int) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("daemon not connected")
	}
	return a.client.CreateSession(name, workDir, cols, rows)
}

// emitEvent sends a Wails event to the frontend. Tests inject emitEventFunc;
// otherwise the emit is skipped when there is no Wails frontend (unit tests,
// CLI helpers) because runtime.EventsEmit needs one.
func (a *App) emitEvent(name string, data any) {
	if a.emitEventFunc != nil {
		a.emitEventFunc(name, data)
		return
	}
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}

// startExitWatcher starts the single app-wide exit watcher: checkExits runs
// immediately and then every 500 ms until ctx is done. There is no deadline,
// so a shell that exits hours after it started is still reported (TERM-05).
// startup is the only caller.
func (a *App) startExitWatcher(ctx context.Context) {
	go func() {
		a.checkExits()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.checkExits()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// checkExits removes every session the daemon reports as stopped (a failed
// kill leaves it listed and the next tick retries) and emits session:exit once
// for every previously listed session that is now gone: one it just removed,
// or one removed elsewhere (CLI kill, daemon restart). Only the watcher
// goroutine (or a test) calls this, so seenSessions needs no lock.
func (a *App) checkExits() {
	if a.client == nil {
		return
	}
	sessions, err := a.client.ListSessions()
	if err != nil {
		return // daemon unreachable; the next tick retries
	}
	if a.seenSessions == nil {
		a.seenSessions = make(map[string]bool)
	}
	listed := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		a.seenSessions[s.ID] = true
		if s.State == "stopped" && a.client.KillSession(s.ID) == nil {
			continue // removed; reported below
		}
		listed[s.ID] = true
	}
	for id := range a.seenSessions {
		if !listed[id] {
			delete(a.seenSessions, id)
			a.emitEvent("session:exit", map[string]any{"sessionId": id})
		}
	}
}

// ListSessions returns a snapshot of all registered sessions.
func (a *App) ListSessions() []SessionInfo {
	a.awaitStartup() // the daemon may still be starting; past the bound, try anyway
	if a.client == nil {
		return []SessionInfo{}
	}
	sessions, err := a.client.ListSessions()
	if err != nil {
		return []SessionInfo{}
	}
	result := make([]SessionInfo, len(sessions))
	for i, s := range sessions {
		result[i] = SessionInfo{
			ID:         s.ID,
			Name:       s.Name,
			State:      s.State,
			Status:     s.Status,
			CreatedAt:  s.CreatedAt,
			Hostname:   s.Hostname,
			WebEnabled: s.WebEnabled,
			// Phase 165 / FNL-01: propagate Funnel state so the frontend poll
			// can detect expiry. Omitting silently drops to false (T-165-15).
			FunnelActive: s.FunnelActive,
			// Phase 171 / FNL-09: propagate the public write-share cap state so the
			// frontend poll can detect a write-share teardown flip. Omitting silently
			// drops to false (T-177-01 — this is the load-bearing runtime fix).
			FunnelWriteActive: s.FunnelWriteActive,
			// Phase 131 / CARD-04..06, GRID-02: propagate Hub card fields from
			// daemon source of truth. Omitting these silently drops them to zero
			// — the same class of silent-corruption bug documented on FunnelActive above.
			ViewerCount: s.ViewerCount,
			ExitCode:    s.ExitCode,
			Duration:    s.Duration,
			WorkDir:     s.WorkDir,
		}
	}
	return result
}

// RenameSession updates the display name of a session.
func (a *App) RenameSession(id, name string) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.RenameSession(id, name)
}

// KillSession terminates the session and removes it from all registries.
func (a *App) KillSession(id string) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.KillSession(id)
}

// GetSessionTailLines returns the last n plain-text lines from the session's
// scrollback buffer, with ANSI escape sequences and relay framing bytes stripped.
// Returns an empty slice if the session has no scrollback (e.g. remote sessions)
// or if the daemon is unreachable. n is clamped to [1..20] to bound response size.
// Used by the Hub mini-preview poller (CARD-07). Phase 132.
func (a *App) GetSessionTailLines(id string, n int) []string {
	if a.client == nil {
		return []string{}
	}
	if n < 1 {
		n = 1
	}
	if n > 20 {
		n = 20
	}
	lines, err := a.client.GetSessionTailLines(id, n)
	if err != nil || lines == nil {
		return []string{}
	}
	return lines
}

// GetShellWebShareWarned returns the persisted "user has been warned about shell
// web-share security implications" flag. Used by ShellWebShareBanner (plan 101-03)
// to suppress the one-time confirmation banner on subsequent toggles.
func (a *App) GetShellWebShareWarned() bool {
	if a.client == nil {
		return false
	}
	warned, err := a.client.GetShellWebShareWarned()
	if err != nil {
		return false
	}
	return warned
}

// SetShellWebShareWarned persists the "user has been warned" flag. Called by
// ShellWebShareBanner (plan 101-03) when the user dismisses the one-time
// confirmation banner.
func (a *App) SetShellWebShareWarned(v bool) error {
	if a.client == nil {
		return nil
	}
	return a.client.SetShellWebShareWarned(v)
}

// GetShellWebShareWarningEnabled returns the warning-enabled master switch.
// Returns true (enabled) when the daemon is not connected — safe degradation per D-08.
// Phase 150 SET-01.
func (a *App) GetShellWebShareWarningEnabled() bool {
	if a.client == nil {
		return true // default ON (not false like GetShellWebShareWarned)
	}
	val, err := a.client.GetShellWebShareWarningEnabled()
	if err != nil {
		return true // default: enabled (safe degradation per D-08)
	}
	return val
}

// SetShellWebShareWarningEnabled persists the warning-enabled master switch.
// When enabling, the daemon atomically resets shellWebShareWarned (D-03 re-arm).
// Phase 150 SET-01.
func (a *App) SetShellWebShareWarningEnabled(v bool) error {
	if a.client == nil {
		return nil
	}
	return a.client.SetShellWebShareWarningEnabled(v)
}

// GetRelayPort returns the TCP port the daemon's relay HTTP server is listening on.
func (a *App) GetRelayPort() int {
	a.awaitStartup() // the daemon may still be starting; past the bound, try anyway
	if a.client == nil {
		return 0
	}
	port, err := a.client.GetRelayPort()
	if err != nil {
		return 0
	}
	return port
}

// GetStartMinimized returns the persisted start-minimized preference.
// Returns false (show window) when daemon is not connected.
func (a *App) GetStartMinimized() bool {
	if a.client == nil {
		return false
	}
	val, err := a.client.GetStartMinimized()
	if err != nil {
		return false
	}
	return val
}

// SetStartMinimized persists the start-minimized preference.
func (a *App) SetStartMinimized(val bool) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.SetStartMinimized(val)
}

// GetAppearance returns the persisted Appearance override (system|light|dark),
// or "system" when the daemon is not connected or the RPC fails.
func (a *App) GetAppearance() string {
	if a.client == nil {
		return "system"
	}
	val, err := a.client.GetAppearance()
	if err != nil {
		return "system"
	}
	return val
}

// SetAppearance validates, persists and applies the Appearance override live.
func (a *App) SetAppearance(mode string) error {
	switch mode {
	case "system", "light", "dark":
	default:
		return fmt.Errorf("invalid appearance %q: want system, light or dark", mode)
	}
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	a.appearanceMu.Lock()
	defer a.appearanceMu.Unlock()
	if err := a.client.SetAppearance(mode); err != nil {
		return err
	}
	a.applyAppearance(mode)
	return nil
}

// applyStoredAppearance applies the persisted Appearance natively. startup calls
// it while the window is still hidden, so a stored Dark never flashes Light.
func (a *App) applyStoredAppearance() {
	a.appearanceMu.Lock()
	defer a.appearanceMu.Unlock()
	a.applyAppearance(a.GetAppearance())
}

func (a *App) applyAppearance(mode string) {
	if a.applyAppearanceFunc != nil {
		a.applyAppearanceFunc(mode)
		return
	}
	setNativeAppearance(mode)
}

// GetPluginSettings returns the persisted plugin enable/disable preferences.
// Returns the zero-value PluginSettings when the daemon is not connected
// or when the RPC fails — callers MUST also gate on a toggleLoaded-style
// guard (the React Settings UI does this via `pluginsLoaded`).
//
// PLUG-03 / Phase 92.
func (a *App) GetPluginSettings() daemon.PluginSettings {
	if a.client == nil {
		return daemon.PluginSettings{}
	}
	s, err := a.client.GetPluginSettings()
	if err != nil {
		return daemon.PluginSettings{}
	}
	return s
}

// SetPluginSettings persists plugin preferences via the daemon AND
// broadcasts the change to all open desktop terminals via the
// "settings:plugins" Wails runtime event (PLUG-03).
//
// EventsEmit lives in app.go ONLY — internal/daemon has no Wails runtime
// context (Pitfall #2). The event fires AFTER the daemon RPC succeeds:
// a failed save MUST NOT emit the event, otherwise consumers would see
// a state the daemon never persisted.
func (a *App) SetPluginSettings(s daemon.PluginSettings) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	if err := a.client.SetPluginSettings(s); err != nil {
		return err
	}
	// WR-05: guard against nil a.ctx (test harness or early Wails-bound
	// RPC fired before startup) — runtime.EventsEmit panics on nil ctx.
	// Mirrors the existing pattern at app.go:266 / app.go:355 / app.go:1006.
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.EventsEmit(a.ctx, "settings:plugins", s)
	}
	return nil
}

// SetSearchConfig persists ONLY the find-bar SearchConfig sub-key of
// PluginSettings via the daemon AND broadcasts the resulting full
// PluginSettings to all open desktop terminals via the "settings:plugins"
// Wails runtime event.
//
// Phase 94-07 WR-03 (gap closure) — used by TerminalPanel.handleSearchOptionsChange
// instead of SetPluginSettings(full snapshot from prop), which raced
// PluginsSection's stale local edit buffer.
//
// The event payload is the full PluginSettings (re-fetched via the daemon)
// because App.tsx's EventsOn('settings:plugins') subscription expects a
// PluginSettings shape — same listener consumes both Set methods. The
// re-fetch happens on the App side so the event reflects the post-write
// truth (including the unchanged non-search fields).
func (a *App) SetSearchConfig(cfg daemon.SearchConfig) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	if err := a.client.SetSearchConfig(cfg); err != nil {
		return err
	}
	// Re-fetch the full PluginSettings so the event payload matches the
	// SetPluginSettings event shape (App.tsx listener expects PluginSettings).
	full, err := a.client.GetPluginSettings()
	if err != nil {
		// Persistence succeeded but readback failed — synthesize a payload
		// from defaults + new SearchConfig so listeners still receive a frame.
		// The next GetPluginSettings call will reconcile.
		full = daemon.PluginSettings{SearchConfig: cfg}
	}
	// WR-05: guard against nil a.ctx (test harness or pre-startup RPC).
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.EventsEmit(a.ctx, "settings:plugins", full)
	}
	return nil
}

// SetWebLinksConfig persists ONLY the WebLinksConfig sub-key of
// PluginSettings via the daemon AND broadcasts the resulting full
// PluginSettings to all open desktop terminals via the "settings:plugins"
// Wails runtime event.
//
// Phase 95 LNK-05 / LNK-06 — mirror of Phase 94-07 SetSearchConfig. The
// sub-key writer preserves PluginsSection's edit buffer semantics: a
// future Settings advanced-disclosure write (Phase 99 / PUI-03) cannot
// stomp an in-flight Plugins-tab boolean edit.
//
// The event payload is the full PluginSettings (re-fetched via the daemon)
// because App.tsx's EventsOn('settings:plugins') subscription expects a
// PluginSettings shape — the same listener consumes SetPluginSettings,
// SetSearchConfig, and SetWebLinksConfig events. The re-fetch happens on
// the App side so the event reflects the post-write truth (including the
// unchanged non-web-links fields).
func (a *App) SetWebLinksConfig(cfg daemon.WebLinksConfig) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	if err := a.client.SetWebLinksConfig(cfg); err != nil {
		return err
	}
	// Re-fetch the full PluginSettings so the event payload matches the
	// SetPluginSettings event shape (App.tsx listener expects PluginSettings).
	full, err := a.client.GetPluginSettings()
	if err != nil {
		// Persistence succeeded but readback failed — synthesize a payload
		// from defaults + new WebLinksConfig so listeners still receive a
		// frame. The next GetPluginSettings call will reconcile.
		full = daemon.PluginSettings{WebLinksConfig: cfg}
	}
	// WR-05: guard against nil a.ctx (test harness or pre-startup RPC).
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.EventsEmit(a.ctx, "settings:plugins", full)
	}
	return nil
}

// SetImageConfig persists ONLY the ImageConfig sub-key of PluginSettings
// via the daemon, then re-fetches the full PluginSettings and emits
// 'settings:plugins' so the React listener consumes it like a
// SetPluginSettings frame (same shape, same handler).
//
// Phase 96 IMG-02 — mirror of Phase 95 SetWebLinksConfig and Phase 94-07
// SetSearchConfig. Concurrency contract is delegated to the daemon's
// engine.SetImageConfig sub-key writer (mutate under e.mu.Lock(); save;
// capture listener; release lock; invoke listener after release).
//
// Note on next-session-only semantics: the event fires (so future
// <details> UIs and the web SSE consumer reflect the new persisted
// value), but the desktop TerminalPanel mount useEffect intentionally
// does NOT include `imageConfig` in any hot-swap dep array — newly-
// mounted sessions pick up the new StorageLimit; already-open sessions
// do not. The PluginsSection italic caption is the user-facing
// affordance for this constraint.
func (a *App) SetImageConfig(cfg daemon.ImageConfig) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	if err := a.client.SetImageConfig(cfg); err != nil {
		return err
	}
	// Re-fetch the full PluginSettings so the event payload matches the
	// SetPluginSettings event shape (App.tsx listener expects PluginSettings).
	full, err := a.client.GetPluginSettings()
	if err != nil {
		// Persistence succeeded but readback failed — synthesize a payload
		// from defaults + new ImageConfig so listeners still receive a
		// frame. The next GetPluginSettings call will reconcile.
		full = daemon.PluginSettings{ImageConfig: cfg}
	}
	// WR-05: guard against nil a.ctx (test harness or pre-startup RPC).
	// Phase 95 code-review fix carried forward.
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.EventsEmit(a.ctx, "settings:plugins", full)
	}
	return nil
}

// configDir returns the path to the agenthub config directory (~/.config/agenthub).
// Creates the directory if it does not exist.
func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "agenthub")
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// StartWebServer tells the daemon to start the web server.
// Refused while Tailscale is not connected (NET-01, Amendment 3); uses
// Tailscale mode when connected with certs, local mode when connected without.
func (a *App) StartWebServer(port int) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	h := a.GetTailscaleStatus()
	if !h.Connected {
		// Same sentence as frontend/src/lib/tailscale.ts TAILSCALE_OFF_REASON.
		return errors.New("Tailscale isn't connected. Remote access and sharing are off until it is.")
	}
	if h.Connected && h.IP != "" && h.HasCerts {
		_, err := a.client.StartWebServer(h.IP, port, h.Domain, "tailscale", "")
		return err
	}
	// Local mode fallback — daemon already holds the generated password.
	pwd, _ := a.client.GetLocalNetworkPassword()
	_, err := a.client.StartWebServer("", port, "", "local", pwd)
	return err
}

// GetLocalNetworkPassword returns the generated LAN access password from the daemon,
// or empty string if not in local mode or daemon is unreachable.
func (a *App) GetLocalNetworkPassword() string {
	if a.client == nil {
		return ""
	}
	pwd, err := a.client.GetLocalNetworkPassword()
	if err != nil {
		return ""
	}
	return pwd
}

// GetWebServerMode returns the web server mode ("tailscale", "local", or "").
// Returns "" when the web server is not running or daemon is unreachable.
func (a *App) GetWebServerMode() string {
	if a.client == nil {
		return ""
	}
	resp, err := a.client.GetWebServerStatus()
	if err != nil || !resp.Running {
		return ""
	}
	return resp.Mode
}

// StopWebServer tells the daemon to stop the web server.
func (a *App) StopWebServer() error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.StopWebServer()
}

// ToggleWebServing enables or disables web serving for a specific session.
// Returns an error if the web server is not running.
func (a *App) ToggleWebServing(sessionID string, enabled bool) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.ToggleWebServing(sessionID, enabled)
}

// DisconnectViewers force-closes every remote (web-origin) viewer currently
// connected to the session, without revoking the share capability. Phase 168
// / FIX-02, #117.
func (a *App) DisconnectViewers(sessionID string) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.DisconnectViewers(sessionID)
}

// SetSessionFunnel enables or disables Tailscale Funnel for a session.
// Phase 165 / FNL-01. Thin bridge — all Funnel logic lives in the daemon
// (165-02). expiresIn is the auto-expiry in seconds (0 = no expiry, FNL-07)
// so Phase 166's expiry picker can drive the full FNL-07 path.
func (a *App) SetSessionFunnel(sessionID string, enabled bool, expiresIn int) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.SetSessionFunnel(sessionID, enabled, expiresIn)
}

// SetSessionFunnelWrite mints the gate-minted, terminal-only public write
// capability for a session (FNL-09). Phase 171-02. Thin bridge — all
// RW-gate lifecycle logic lives in the daemon; expiresIn is the requested
// TTL in seconds (server-clamped unconditionally to (0, 3600]).
func (a *App) SetSessionFunnelWrite(sessionID string, expiresIn int) (daemon.SetSessionFunnelWriteResponse, error) {
	if a.client == nil {
		return daemon.SetSessionFunnelWriteResponse{}, fmt.Errorf("daemon not connected")
	}
	writeURL, writeCode, expiresAt, err := a.client.SetSessionFunnelWrite(sessionID, expiresIn)
	if err != nil {
		return daemon.SetSessionFunnelWriteResponse{}, err
	}
	return daemon.SetSessionFunnelWriteResponse{WriteURL: writeURL, WriteCode: writeCode, ExpiresAt: expiresAt}, nil
}

// DisableSessionFunnelWrite revokes the gate-minted write grant/code/gate
// for a session (FNL-09) without disturbing the reusable public read share.
// Phase 171-02. Thin bridge — mirrors SetSessionFunnelWrite.
func (a *App) DisableSessionFunnelWrite(sessionID string) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.DisableSessionFunnelWrite(sessionID)
}

// GetWebServerURL returns the base HTTPS URL of the running web server,
// or an empty string if the server is not running.
func (a *App) GetWebServerURL() string {
	if a.client == nil {
		return ""
	}
	resp, err := a.client.GetWebServerStatus()
	if err != nil || !resp.Running {
		return ""
	}
	return resp.URL
}

// ctDisclosurePath returns the path to the CT disclosure acknowledgement file.
func ctDisclosurePath() string {
	return filepath.Join(configDir(), "ct_disclosed")
}

// HasCTDisclosure returns true if the user has acknowledged the CT log disclosure.
func (a *App) HasCTDisclosure() bool {
	_, err := os.Stat(ctDisclosurePath())
	return err == nil
}

// AcknowledgeCTDisclosure persists the CT disclosure acknowledgement.
func (a *App) AcknowledgeCTDisclosure() error {
	return os.WriteFile(ctDisclosurePath(), []byte("1"), 0600)
}

// IsWebServerRunning returns true if the daemon's web server is active.
func (a *App) IsWebServerRunning() bool {
	if a.client == nil {
		return false
	}
	resp, err := a.client.GetWebServerStatus()
	if err != nil {
		return false
	}
	return resp.Running
}

// GetWebServerQRCode returns a base64-encoded PNG QR code for the web server dashboard URL.
// Returns an error if the daemon is disconnected or the web server is not running.
func (a *App) GetWebServerQRCode() (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("daemon not connected")
	}
	resp, err := a.client.GetWebServerStatus()
	if err != nil || !resp.Running {
		return "", fmt.Errorf("web server not running")
	}
	png, err := qrcode.Encode(resp.URL, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("GetWebServerQRCode: encode: %w", err)
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

// --- Phase 87 capability Wails bindings (D-06, D-07, D-09, D-16) ---------

// IssueCapabilities asks the daemon to mint two capabilities (read + read,write)
// for the given session (D-07). The session must be web-enabled (caller must
// have already called ToggleWebServing). Returns the read/write URLs and the
// matching single-use join codes.
func (a *App) IssueCapabilities(sessionID string) (daemon.IssueCapabilitiesResponse, error) {
	if a.client == nil {
		return daemon.IssueCapabilitiesResponse{}, fmt.Errorf("daemon not connected")
	}
	return a.client.IssueCapabilities(sessionID)
}

// ExchangeJoinCode consumes a single-use join code and returns the
// capability-bearing URL the client should follow. Called by the /join page
// after the user taps "Join Session".
func (a *App) ExchangeJoinCode(code string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("daemon not connected")
	}
	return a.client.ExchangeJoinCode(code)
}

// RegenerateSigningKey rotates the HMAC signing key (D-16 panic button in
// Settings > Security). All previously-issued capabilities become invalid.
func (a *App) RegenerateSigningKey() error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.RegenerateSigningKey()
}

// GetCapabilityQRCode encodes a join-code exchange URL as a base64 PNG QR
// code (D-09). The caller must pass the join-code URL (e.g.
// "https://host/join?code=A7K-4P2N"), NOT the raw capability token URL.
// Mirrors GetWebServerQRCode; the encoder call is identical.
func (a *App) GetCapabilityQRCode(joinURL string) (string, error) {
	if joinURL == "" {
		return "", fmt.Errorf("joinURL required")
	}
	png, err := qrcode.Encode(joinURL, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("GetCapabilityQRCode: encode: %w", err)
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

// OpenDirectoryDialog opens a native OS folder picker and returns the selected path.
// Returns "" if the user cancels. Falls back to the user's home directory when
// defaultDir is empty.
func (a *App) OpenDirectoryDialog(defaultDir string) (string, error) {
	if defaultDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultDir = home
		}
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Select Working Directory",
		DefaultDirectory: defaultDir,
	})
}

// SaveTerminalSession opens a native Save File dialog and writes the supplied
// terminal scrollback content to the user-chosen path. Cancellation is silent
// success. Returns wrapped errors for dialog setup or write failures.
//
// Phase 97 SER-01. Follows the Wails dialog cancel="" pattern. Uses saveFileDialogFunc indirection (function-injection per
// PROJECT.md "Key Decisions") so unit tests can mock the Wails runtime.
func (a *App) SaveTerminalSession(defaultDir, defaultName, content string) error {
	if defaultDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultDir = home
		}
	}
	path, err := a.saveFileDialogFunc(a.ctx, runtime.SaveDialogOptions{
		Title:                "Save Terminal As… (file will include any printed secrets)",
		DefaultDirectory:     defaultDir,
		DefaultFilename:      defaultName,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: "Text File (*.txt)", Pattern: "*.txt"},
		},
	})
	if err != nil {
		return fmt.Errorf("SaveTerminalSession: dialog: %w", err)
	}
	if path == "" {
		return nil // user cancelled — silent success, like OpenDirectoryDialog
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("SaveTerminalSession: write: %w", err)
	}
	return nil
}

// SetTrayProgress sets the cross-session aggregate progress quartile and
// refreshes the system tray icon if the quartile changed.
//
// Quartile semantics: 0 = no active progress (revert to base icon);
// 1..4 = 25/50/75/100% quartile glyphs. Frontend caller is App.tsx,
// which debounces at 200ms before invoking this RPC (Pitfall #5);
// the Go-side idempotency check (Pitfall #3 transition guard) ensures
// identical quartile values do not churn the platform tray API even
// if the debounce is bypassed by a slow flap.
//
// Returns an error if quartile is out of range [0,4]; silent no-op if
// the tray subsystem hasn't initialized (a.trayInit == false).
func (a *App) SetTrayProgress(quartile int) error {
	if !a.trayInit {
		return nil
	}
	if quartile < 0 || quartile > 4 {
		return fmt.Errorf("SetTrayProgress: quartile out of range [0,4]: %d", quartile)
	}
	if a.lastTrayQuartile.Load() == int32(quartile) {
		return nil
	}
	a.lastTrayQuartile.Store(int32(quartile))
	if a.refreshTrayStateFunc != nil {
		a.refreshTrayStateFunc()
	} else {
		a.refreshTrayState()
	}
	return nil
}

// GetTailscaleStatus runs a fresh Tailscale health check and returns it.
// It goes through checkTailscaleHealth so the poller knows every value the
// frontend has seen and emits the recovery from a transient failure.
func (a *App) GetTailscaleStatus() webserver.TailscaleHealth {
	return a.checkTailscaleHealth(context.Background())
}

// openAppCmd uses an absolute path so a modified PATH cannot swap in another
// `open` (review IN-05).
func openAppCmd(name string) *exec.Cmd {
	return exec.Command("/usr/bin/open", "-a", name)
}

// OpenTailscale launches the installed Tailscale app, or opens its download
// page if that fails (NET-01: it never installs or configures anything). The
// argv and URL are fixed; no frontend value reaches exec (T-02-30).
func (a *App) OpenTailscale() error {
	openApp := a.openAppFunc
	if openApp == nil {
		openApp = func(name string) error { return openAppCmd(name).Run() }
	}
	if err := openApp("Tailscale"); err == nil {
		return nil
	}
	openURL := a.openURLFunc
	if openURL == nil {
		openURL = func(url string) {
			if a.ctx != nil {
				runtime.BrowserOpenURL(a.ctx, url)
			}
		}
	}
	openURL("https://tailscale.com/download/mac")
	return nil
}

func (a *App) tailscaleCheck(ctx context.Context) webserver.TailscaleHealth {
	if a.tailscaleCheckFunc != nil {
		return a.tailscaleCheckFunc(ctx)
	}
	return webserver.CheckHealth(ctx)
}

// GetRemoteSessions discovers tailnet peers and fetches their session lists.
// Returns an empty slice if the daemon is unreachable or no peers are found.
// Individual peer fetch failures are silently omitted.
// Delegates to tailnet.FetchAllPeerSessions for concurrent fetch with IP fallback.
func (a *App) GetRemoteSessions() []RemotePeerSessions {
	if a.client == nil {
		return []RemotePeerSessions{}
	}
	peers, err := a.client.ListTailnetPeers()
	if err != nil || len(peers) == 0 {
		return []RemotePeerSessions{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	groups := tailnet.FetchAllPeerSessions(ctx, peers)

	results := make([]RemotePeerSessions, 0, len(groups))
	for _, g := range groups {
		sessions := make([]RemoteSession, 0, len(g.Sessions))
		for _, s := range g.Sessions {
			sessions = append(sessions, RemoteSession{
				ID:     s.ID,
				Name:   s.Name,
				Status: s.Status,
				URL:    s.URL,
			})
		}
		// Every group returned by FetchAllPeerSessions answered the cap-gated
		// probe, so it is reachable by definition. Set Reachable=true explicitly
		// to match the RemotePeerSessions semantics of GetRemoteSessionsWithMeta
		// (WR-04: leaving it defaulted to false mislabels reachable peers as
		// "Unreachable" in RemoteSessionsPanel for any future caller).
		results = append(results, RemotePeerSessions{Hostname: g.Hostname, Reachable: true, Sessions: sessions})
	}
	return results
}

// GetRemoteSessionsWithMeta discovers tailnet peers and fetches their shareable-session
// metadata via the open /api/sessions/meta endpoint (no cap required). Returns ALL
// probed peers including unreachable ones (Reachable=false) and peers with zero
// shareable sessions (Reachable=true, len(Sessions)==0) — enabling honest per-peer
// states in the Remote Sessions panel (RB-01, RB-04).
//
// This RPC uses the new tailnet metadata path (FetchAllPeerSessionsMeta) that never
// silently drops peers, unlike the cap-gated GetRemoteSessions path. GetRemoteSessions
// is retained for backward compatibility during the plan-04 frontend rewire.
func (a *App) GetRemoteSessionsWithMeta() []RemotePeerSessions {
	if a.client == nil {
		return []RemotePeerSessions{}
	}
	peers, err := a.client.ListTailnetPeers()
	if err != nil || len(peers) == 0 {
		return []RemotePeerSessions{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	groups := tailnet.FetchAllPeerSessionsMeta(ctx, peers)

	results := make([]RemotePeerSessions, 0, len(groups))
	for _, g := range groups {
		sessions := make([]RemoteSession, 0, len(g.Sessions))
		for _, s := range g.Sessions {
			sessions = append(sessions, RemoteSession{
				ID:     s.ID,
				Name:   s.Name,
				Status: s.Status,
				URL:    s.URL,
			})
		}
		results = append(results, RemotePeerSessions{
			Hostname:  g.Hostname,
			Reachable: g.Reachable,
			Sessions:  sessions,
		})
	}
	return results
}

// ExchangeJoinCodeAtURL exchanges a 5-character join code against a REMOTE
// peer's `/join/exchange` endpoint (NOT the local daemon) and returns the cap
// token extracted from the response.
//
// Phase 122-03 / REMOTE-01 — the desktop GUI's paste-join-code modal calls
// this to obtain a cap for browsing files on a remote tailnet session. The
// returned cap is subsequently deposited via RegisterRemoteCap; it must NEVER
// be passed to the React frontend (it lives in the daemon's RemoteCapStore).
func (a *App) ExchangeJoinCodeAtURL(remoteBaseURL, code string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("daemon not connected")
	}
	return a.client.ExchangeJoinCodeAtURL(remoteBaseURL, code)
}

// RegisterRemoteCap deposits a (sessionID, remote baseURL, cap token) tuple
// into the local daemon's RemoteCapStore. OpenRemoteSessionURL and the relay
// remote terminal WS proxy (/api/relay/remote/{sid}/ws) use this cap.
// Phase 122-03 / REMOTE-01.
func (a *App) RegisterRemoteCap(sessionID, baseURL, capToken string) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.RegisterRemoteCap(sessionID, baseURL, capToken)
}

// OpenRemoteSessionURL returns the cap-bearing open URL for a remote session
// by reading the already-stored cap from the daemon's RemoteCapStore. The
// daemon composes baseURL+/sessions/{id}?cap=TOKEN from its own store entry
// (keyed by sessionID) and returns the full URL as a string.
//
// Convention (matches ExchangeJoinCodeAtURL): the frontend receives the
// composed URL string and opens it via BrowserOpenURL; the raw cap token never
// enters React state — it lives only inside the returned URL string.
//
// Returns an error when the daemon is not connected or when no cap is stored
// for sessionID (e.g. "status 404"). The caller treats a 404-flavoured error
// as "no cap held; fall back to the join-code modal" so a stale/evicted entry
// self-heals gracefully (T-146-05-02 accept).
//
// Do NOT call BrowserOpenURL here — the frontend opens the returned URL,
// matching the existing exchange→open convention. Phase 146-05 / GAP-146-A.
func (a *App) OpenRemoteSessionURL(sessionID string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("daemon not connected")
	}
	return a.client.RemoteSessionOpenURL(sessionID)
}

// startTrayPoller starts a background goroutine that refreshes tray state
// (icon, tooltip, session list) immediately and then every 5 seconds.
// The goroutine exits when ctx is cancelled (Wails shutdown).
func (a *App) startTrayPoller(ctx context.Context) {
	go func() {
		// Do an immediate refresh before the first tick.
		a.refreshTrayState()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.refreshTrayState()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// refreshTrayState reads daemon connectivity and session list, then updates
// the tray icon, tooltip, and session menu entries.
func (a *App) refreshTrayState() {
	if !a.trayInit {
		return // tray not yet initialised
	}
	if a.client == nil {
		// Startup failed — tray is visible but daemon is unreachable.
		// Show error icon and appropriate tooltip.
		a.updateTray(nil, false)
		return
	}
	connected := a.client.Health() == nil
	var sessions []SessionInfo
	if connected {
		sessions = a.ListSessions()
	}
	a.updateTray(sessions, connected)
}

// startUpdatePoller checks for updates on startup and every hour.
// Follows the startHealthPoller/startTrayPoller goroutine+ticker pattern.
func (a *App) startUpdatePoller(ctx context.Context) {
	go func() {
		// Initial check after 5-second delay to avoid startup race
		// (frontend needs time to mount and subscribe to events).
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
		a.runUpdateCheck(ctx, false)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.runUpdateCheck(ctx, false)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// runUpdateCheck performs a single update check and emits an event if a newer version is found.
func (a *App) runUpdateCheck(ctx context.Context, force bool) {
	info, err := updater.Check(ctx, configDir(), "scottkw/agenthub", Version, updater.DefaultDetect, force)
	if err != nil || info == nil {
		return
	}
	a.lastUpdateMu.Lock()
	a.lastUpdate = info
	a.lastUpdateMu.Unlock()
	runtime.EventsEmit(ctx, "update:available", info)
}

// GetLastUpdateInfo returns the latest update info, or nil if no update is available.
// Called by frontend on mount to handle the startup race condition where
// update:available events may fire before React subscribes.
func (a *App) GetLastUpdateInfo() *updater.UpdateInfo {
	a.lastUpdateMu.Lock()
	defer a.lastUpdateMu.Unlock()
	return a.lastUpdate
}

// CheckForUpdates performs an immediate update check, bypassing the rate limit.
// Called from Help > Check for Updates menu item.
func (a *App) CheckForUpdates() *updater.UpdateInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, _ := updater.Check(ctx, configDir(), "scottkw/agenthub", Version, updater.DefaultDetect, true)
	if info != nil {
		a.lastUpdateMu.Lock()
		a.lastUpdate = info
		a.lastUpdateMu.Unlock()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "update:available", info)
		}
	}
	return info
}

// checkTailscaleHealth runs one health check and emits "tailscale:health" when
// it is the first result or differs from the last one. The first result is
// always emitted: "not connected" is the zero value, so comparing against a
// zero starting point would never report a launch-time disconnect. The emit
// happens under healthMu so events reach the frontend in lastHealth order.
func (a *App) checkTailscaleHealth(ctx context.Context) webserver.TailscaleHealth {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	h := a.tailscaleCheck(checkCtx)
	cancel()
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.lastHealth == nil || *a.lastHealth != h {
		a.lastHealth = &h
		a.emitEvent("tailscale:health", h)
	}
	return h
}

// startHealthPoller checks Tailscale health once immediately and then every
// 10 seconds. The goroutine exits when ctx is cancelled (Wails shutdown).
func (a *App) startHealthPoller(ctx context.Context) {
	go func() {
		a.checkTailscaleHealth(ctx)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.checkTailscaleHealth(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}
