package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/updater"
	"github.com/scottkw/portty/internal/wakewatch"
	"github.com/scottkw/portty/internal/webserver"
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
	// Phase 131 — Hub card fields (CARD-04, CARD-05, CARD-06, GRID-02).
	// Omitting any of these silently drops them to zero in the Wails RPC
	// response (silent false-drop is a UAT-class bug).
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
	DNSName   string          `json:"dnsName"` // what ConnectPeerSession should be given (WR-05)
	Reachable bool            `json:"reachable"`
	Sessions  []RemoteSession `json:"sessions"`
}

// App holds all application state and exposes the Wails-bound methods.
// App is a thin Wails-binding shell — all session state lives in the daemon process.
// All session operations are delegated through DaemonClient over the Unix socket.
type App struct {
	ctx              context.Context
	client           *daemon.DaemonClient  // only daemon communication field; set in NewApp, before Wails starts any goroutine (nil only in tests)
	trayInit         bool                  // true once initTray has been called
	lastTrayQuartile atomic.Int32          // Phase 98 PRG-03 — last applied tray progress quartile [0..4]; -1 = unset. Atomic because written by Wails RPC goroutine (SetTrayProgress) and read by startTrayPoller goroutine via trayIconBytesForState (CR-01).
	daemonErr        atomic.Pointer[error] // non-nil when EnsureDaemon failed; atomic: Retry writes it while init reads it (IN-08)
	quitting         atomic.Bool           // set first by QuitAll and shutdown: beforeClose allows exit and the poller never respawns (WR-04)
	// updates is the update state machine (app_update.go); set in NewApp.
	updates *updater.Service
	// updateAfter and updateNow are the update loop's timer and clock seams; nil means time.After and time.Now.
	updateAfter func(time.Duration) <-chan time.Time
	updateNow   func() time.Time
	// messageDialogFunc is the native alert seam; nil means runtime.MessageDialog on a.ctx.
	messageDialogFunc func(runtime.MessageDialogOptions) (string, error)
	// setTrayUpdateFunc is the tray "Update Available…" item seam; nil means the cgo setter once the tray exists.
	setTrayUpdateFunc func(title string)

	// saveFileDialogFunc allows unit tests to mock runtime.SaveFileDialog.
	// Defaults to runtime.SaveFileDialog. Phase 97 SER-01 / PROJECT.md
	// "Function injection" pattern, parallel to serviceControlFunc and
	// statusFunc.
	saveFileDialogFunc func(ctx context.Context, opts runtime.SaveDialogOptions) (string, error)

	// refreshTrayStateFunc allows unit tests to mock refreshTrayState().
	// Defaults to nil (production path calls a.refreshTrayState() directly).
	// Phase 98 PRG-03 / PROJECT.md "Function injection" pattern.
	refreshTrayStateFunc func()

	// sendNotificationFunc is the one native notification path (the quit-to-tray
	// notice and the update-available notice). Defaults to sendNotification;
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

	// wakeWatchFunc is the wake/network detector seam; nil means wakewatch.Watch.
	wakeWatchFunc func(context.Context) <-chan wakewatch.Reason

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

	// daemonMu serializes every EnsureDaemon after startup (RetryDaemon and
	// the poller's respawn) and guards the respawn streak. The poller respawns
	// because a daemon that dies on the viewing Mac otherwise stays dead and
	// its remote tabs retry it forever (R6). a.client is never replaced: the
	// new daemon binds the same socket and the client dials per request (WR-05).
	daemonMu        sync.Mutex
	respawnAttempts int       // attempts in the current streak
	lastRespawn     time.Time // when the last attempt ran

	// HARD-04: the first poller tick after launch saw a healthy daemon as
	// unreachable and spent an attempt from the streak. Until the poller has
	// seen the daemon healthy, a failed check within respawnLaunchGrace of a
	// successful startup is ignored. The grace is bounded so R6 still
	// recovers a daemon that dies early; a failed startup leaves launchedAt
	// zero and gets none. seenHealthy and launchWaitLogged are guarded by
	// daemonMu; launchedAt is written before startupDone closes and read after.
	seenHealthy      bool
	launchWaitLogged bool
	launchedAt       time.Time

	// seenSessions is every id the exit watcher has listed and not yet
	// reported via session:exit. Owned by the exit watcher goroutine.
	seenSessions map[string]bool

	// DEBT-08: a daemon from another build with live sessions is kept until
	// the user agrees to restart it. restartPrompt is the session count the
	// dialog shows (0: no prompt; -1: unknown); restartDeclined ("Not Now")
	// stops further prompts this launch; restartApproved lets one "Restart
	// Now" EnsureDaemon through. Guarded by restartMu (taken after daemonMu).
	restartMu       sync.Mutex
	restartPrompt   int
	restartDeclined bool
	restartApproved bool
}

// defaultTailscaleCheck seeds App.tailscaleCheckFunc; nil means
// webserver.CheckHealth. Only the VM-only vmpreview build sets it.
var defaultTailscaleCheck func(context.Context) webserver.TailscaleHealth

// NewApp creates a new App without starting any subsystems.
func NewApp() *App {
	a := &App{
		tailscaleCheckFunc:   defaultTailscaleCheck,
		saveFileDialogFunc:   runtime.SaveFileDialog,
		sendNotificationFunc: sendNotification,
		startupDone:          make(chan struct{}),
		// The client only dials on use. Creating it here, before wails.Run,
		// gives every Wails goroutine a happens-before edge on it.
		client: daemon.NewDaemonClient(daemon.DefaultSocketPath()),
	}
	a.lastTrayQuartile.Store(-1) // Phase 98 PRG-03 — ensure first SetTrayProgress call always updates
	a.updates = a.newUpdateService()
	return a
}

// startupWait bounds domReady's wait for startup. domReady runs on the Wails
// message loop, so a wedged daemon must not block it forever (review WR-05).
var startupWait = 5 * time.Second

// startupBindingWait bounds how long GetDaemonError and RetryDaemon wait for
// startup. It covers the common EnsureDaemon worst case, not every one: a daemon
// that hangs on several 30 s client calls can overrun it, and then Retry
// recovers. Bindings run on their own goroutines, so this wait never stalls
// the message loop (review WR-07, IN-15).
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

	// Frontend.Run creates the window before starting OnStartup, so it exists here.
	// It is still hidden (StartHidden), and the serial main queue runs this before the later WindowShow.
	applySidebarMaterial()

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

	// Tell guest views when this Mac wakes or changes networks.
	a.startWakeWatcher(ctx)
}

// startDaemon brings the daemon up for startup and closes startupDone on every
// return path, so domReady and the bindings never wait out their bounds.
func (a *App) startDaemon(socketPath string) {
	if a.startupDone != nil {
		defer close(a.startupDone)
	}
	if err := a.ensure()(socketPath); err != nil {
		a.daemonErr.Store(&err)
		// Notify frontend — window is already rendered when OnStartup runs (goroutine).
		a.emitEvent("daemon:error", err.Error())
	} else {
		a.launchedAt = time.Now()
		a.applyStoredAppearance()
	}
}

// ensure returns the EnsureDaemon seam, or daemon.EnsureDaemon with the GUI's
// restart consent when unset.
func (a *App) ensure() func(socketPath string) error {
	if a.ensureDaemonFunc != nil {
		return a.ensureDaemonFunc
	}
	return func(socketPath string) error { return daemon.EnsureDaemon(socketPath, a.restartConsent) }
}

// restartConsent is the GUI's daemon.RestartPolicy (DEBT-08). It never blocks
// startup: unless "Restart Now" approved this call, it keeps the old daemon and
// asks the frontend, once per launch at most once the user said "Not Now".
func (a *App) restartConsent(_ string, sessions int) bool {
	a.restartMu.Lock()
	defer a.restartMu.Unlock()
	if a.restartApproved {
		return true
	}
	if !a.restartDeclined {
		a.restartPrompt = sessions
		a.emitEvent("daemon:restart-needed", sessions)
	}
	return false
}

// GetDaemonRestartPrompt returns the session count to ask about, or 0 when
// there is no question. The frontend calls it after GetDaemonError, because
// startup's daemon:restart-needed can fire before React subscribes.
func (a *App) GetDaemonRestartPrompt() int {
	a.restartMu.Lock()
	defer a.restartMu.Unlock()
	return a.restartPrompt
}

// AnswerDaemonRestart is the dialog's answer. "Not Now" (false) keeps the old
// daemon for the rest of this launch. "Restart Now" (true) restarts it, which
// ends its sessions, and tells the frontend to reload; a failure keeps the prompt.
func (a *App) AnswerDaemonRestart(restart bool) error {
	if !restart {
		a.restartMu.Lock()
		a.restartPrompt, a.restartDeclined = 0, true
		a.restartMu.Unlock()
		return nil
	}
	a.daemonMu.Lock()
	defer a.daemonMu.Unlock()
	if a.quitting.Load() { // Quit All may have just stopped the daemon
		return errors.New("quitting")
	}
	a.restartMu.Lock()
	a.restartApproved = true
	a.restartMu.Unlock()
	err := a.ensure()(daemon.DefaultSocketPath())
	a.restartMu.Lock()
	a.restartApproved = false
	if err == nil {
		a.restartPrompt = 0
	}
	a.restartMu.Unlock()
	if err != nil {
		return err
	}
	a.emitEvent("daemon:recovered", nil) // new relay port; the exit watcher closes the ended tabs
	return nil
}

// Respawn pacing: attempts wait 10 s, 20 s, 40 s and so on up to 2 min after
// the previous one; a minute of healthy daemon ends the streak; the third
// attempt raises daemon:error once.
const (
	respawnBaseDelay    = 10 * time.Second
	respawnMaxDelay     = 2 * time.Minute
	respawnHealthyReset = time.Minute
	respawnErrorAfter   = 3
	respawnLaunchGrace  = 10 * time.Second // the first two 5 s ticks (HARD-04)
)

const respawnKeepsStoppingMsg = "Portty restarted the daemon 3 times and it keeps stopping."

// respawnDaemonIfDown starts the daemon again when the poller finds it
// unreachable after startup (R6), so remote tabs, the sidebar and new sessions
// come back without a click. It skips while another EnsureDaemon runs.
func (a *App) respawnDaemonIfDown(now time.Time) {
	if a.client == nil || a.quitting.Load() {
		return
	}
	if a.startupDone != nil {
		select {
		case <-a.startupDone:
		default:
			return // startup's own EnsureDaemon is still running
		}
	}
	healthErr := a.client.Health()
	if !a.daemonMu.TryLock() {
		return
	}
	defer a.daemonMu.Unlock()
	if a.quitting.Load() { // QuitAll ran while Health was in flight
		return
	}
	if healthErr == nil {
		a.seenHealthy = true
		if a.respawnAttempts > 0 && now.Sub(a.lastRespawn) >= respawnHealthyReset {
			a.respawnAttempts = 0
		}
		// IN-01: the daemon is back (and stayed up through any streak), so drop
		// the error and let the frontend reload as Retry would.
		if a.respawnAttempts == 0 && a.daemonErr.Load() != nil {
			a.daemonErr.Store(nil)
			a.applyStoredAppearance() // skipped while startup failed
			a.emitEvent("daemon:recovered", nil)
		}
		return
	}
	if !a.seenHealthy && !a.launchedAt.IsZero() && now.Sub(a.launchedAt) < respawnLaunchGrace {
		if !a.launchWaitLogged {
			a.launchWaitLogged = true
			fmt.Fprintf(os.Stderr, "portty: daemon not answering right after launch (%v); waiting up to 10s before any respawn\n", healthErr)
		}
		return
	}
	if a.respawnAttempts > 0 {
		wait := min(respawnBaseDelay<<min(a.respawnAttempts-1, 4), respawnMaxDelay)
		if now.Sub(a.lastRespawn) < wait {
			return
		}
	}
	a.respawnAttempts++
	a.lastRespawn = now
	err := a.ensure()(daemon.DefaultSocketPath())
	result := "ok"
	if err != nil {
		result = err.Error()
	}
	fmt.Fprintf(os.Stderr, "portty: daemon unreachable (%v), respawn attempt %d: %s\n", healthErr, a.respawnAttempts, result)
	if a.respawnAttempts == respawnErrorAfter {
		if err == nil {
			err = errors.New(respawnKeepsStoppingMsg)
		}
		a.daemonErr.Store(&err) // GetDaemonError agrees with the banner (IN-01)
		a.emitEvent("daemon:error", err.Error())
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
	if err := a.daemonErr.Load(); err != nil {
		return (*err).Error()
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
	a.daemonMu.Lock()
	if a.quitting.Load() { // after the lock: Quit All may have just stopped the daemon (review it3 IN-01)
		a.daemonMu.Unlock()
		return errors.New("quitting")
	}
	err := a.ensure()(daemon.DefaultSocketPath())
	if err == nil {
		a.respawnAttempts = 0
	}
	a.daemonMu.Unlock()
	if err != nil {
		a.daemonErr.Store(&err)
		return err
	}
	a.daemonErr.Store(nil)
	a.applyStoredAppearance() // startup skipped it while the daemon was down
	return nil
}

// shutdown is called when the Wails app is about to exit.
func (a *App) shutdown(_ context.Context) {
	a.quitting.Store(true) // the app is exiting: no respawn from here on (WR-04)
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
	if a.quitting.Load() {
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

// QuitGUIOnly hides the window to the menu bar without stopping the daemon
// or any active sessions, and sends a notification saying Portty is still
// running (D-10, D-11).
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
	a.sendNotificationFunc(quitNotificationID, quitNotificationTitle, quitToMenuBarBody(count))
}

// QuitAll shuts down the daemon (terminating all sessions) and quits the
// application completely (D-09, APP-02).
func (a *App) QuitAll() {
	if a.ctx == nil {
		return
	}
	a.quitting.Store(true) // first: the poller must not respawn the daemon we stop (WR-04)
	if a.client != nil {
		a.daemonMu.Lock() // waits out a respawn already running
		_ = a.client.ShutdownDaemon()
		a.daemonMu.Unlock()
	}
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
			// Phase 131 / CARD-04..06, GRID-02: propagate Hub card fields from
			// daemon source of truth. Omitting these silently drops them to zero
			// (silent corruption).
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

// GetRelayPort returns the TCP port the daemon's relay HTTP server is listening on.
func (a *App) GetRelayPort() int {
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

// GetTerminalFont returns the persisted Terminal Font, or the default
// (empty family, size 13) when the daemon is not connected or the RPC fails.
func (a *App) GetTerminalFont() daemon.TerminalFont {
	def := daemon.TerminalFont{Size: daemon.DefaultTerminalFontSize}
	if a.client == nil {
		return def
	}
	f, err := a.client.GetTerminalFont()
	if err != nil {
		return def
	}
	return f
}

// SetTerminalFont persists the Terminal Font. The daemon is the single
// validation point (quick 261116-ku9).
func (a *App) SetTerminalFont(family string, size int) error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.SetTerminalFont(daemon.TerminalFont{Family: family, Size: size})
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

// configDir returns os.UserConfigDir()/portty (~/Library/Application Support/portty
// on macOS), creating it 0700 if it does not exist.
func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "portty")
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// ShareResult is what the share bindings return. They never throw: a failure
// fills ErrorCode (the daemon share error codes, or start_failed / stop_failed
// when the daemon is unreachable) and ErrorDetail. URL is the full-control link.
type ShareResult struct {
	Shared      bool   `json:"shared"`
	URL         string `json:"url"`
	FirstShare  bool   `json:"firstShare"`
	ErrorCode   string `json:"errorCode"`
	ErrorDetail string `json:"errorDetail"`
}

func shareResult(resp daemon.ShareResponse, err error, fallbackCode string) ShareResult {
	if err == nil {
		return ShareResult{Shared: resp.Shared, URL: resp.URL, FirstShare: resp.FirstShare}
	}
	var se *daemon.ShareError
	if errors.As(err, &se) {
		return ShareResult{ErrorCode: se.Code, ErrorDetail: se.Detail}
	}
	return ShareResult{ErrorCode: fallbackCode, ErrorDetail: err.Error()}
}

// GetSessionShare reports whether a session is shared and its link.
func (a *App) GetSessionShare(id string) ShareResult {
	if a.client == nil {
		return ShareResult{ErrorCode: "unavailable", ErrorDetail: "daemon not connected"}
	}
	resp, err := a.client.GetSessionShare(id)
	return shareResult(resp, err, "unavailable")
}

// SetSessionShared flips the session's one share switch in the daemon, the
// same state `portty share` / `portty unshare` and the sidebar menu flip.
func (a *App) SetSessionShared(id string, shared bool) ShareResult {
	code := "start_failed"
	if !shared {
		code = "stop_failed"
	}
	if a.client == nil {
		return ShareResult{ErrorCode: code, ErrorDetail: "daemon not connected"}
	}
	resp, err := a.client.SetSessionShared(id, shared)
	return shareResult(resp, err, code)
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

// RegenerateSigningKey rotates the HMAC signing key (D-16 panic button in
// Settings > Security). Every old link stops verifying; shares get new links.
func (a *App) RegenerateSigningKey() error {
	if a.client == nil {
		return fmt.Errorf("daemon not connected")
	}
	return a.client.RegenerateSigningKey()
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

// GetRemoteSessionsWithMeta discovers tailnet peers and fetches their shareable-session
// metadata via the open /api/sessions/meta endpoint (no cap required). Returns ALL
// probed peers including unreachable ones (Reachable=false) and peers with zero
// shareable sessions (Reachable=true, len(Sessions)==0) — enabling honest per-peer
// states in the Remote Sessions panel (RB-01, RB-04).
// Peers signed in to a different Tailscale account are dropped (SHAR-01).
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
			DNSName:   g.DNSName,
			Reachable: g.Reachable,
			Sessions:  sessions,
		})
	}
	return results
}

// PeerConnectResult is what ConnectPeerSession returns. It never throws: a
// failure fills ErrorCode with a POST /peers/connect code, or "unreachable" when
// the daemon can't be reached. URL is the session's link and carries the cap.
type PeerConnectResult struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	URL       string `json:"url"`
	ErrorCode string `json:"errorCode"`
}

// ConnectPeerSession connects to a session shared from another Mac on the same
// Tailscale account through the daemon (the same path `portty attach` uses).
// The daemon keeps the capability, so the remote terminal attaches through
// /api/relay/remote/{sessionId}/ws.
func (a *App) ConnectPeerSession(host, sessionID string) PeerConnectResult {
	if a.client == nil {
		return PeerConnectResult{ErrorCode: daemon.PeerErrUnreachable}
	}
	res, err := a.client.ConnectPeer(host, sessionID)
	if err != nil {
		var pe *daemon.PeerConnectError
		if errors.As(err, &pe) {
			return PeerConnectResult{ErrorCode: pe.Code}
		}
		return PeerConnectResult{ErrorCode: daemon.PeerErrUnreachable}
	}
	return PeerConnectResult{SessionID: res.SessionID, Name: res.Name, Host: res.Host, URL: res.URL}
}

// startTrayPoller starts a background goroutine that refreshes tray state
// (icon, tooltip, session list) immediately and then every 5 seconds.
// The goroutine exits when ctx is cancelled (Wails shutdown).
func (a *App) startTrayPoller(ctx context.Context) {
	go func() {
		// Do an immediate refresh before the first tick. Each refresh first
		// brings a dead daemon back (R6), so the tray reads the new one.
		a.respawnDaemonIfDown(time.Now())
		a.refreshTrayState()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.respawnDaemonIfDown(time.Now())
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

// startWakeWatcher forwards the 05-10 wake/network detector to the frontend as
// system:wake ("wake" or "network"), so guest views redial right after sleep or
// a network switch instead of waiting out the watchdog (user requirement, SHAR-07).
// The detector closes its channel when ctx ends, which ends the goroutine.
func (a *App) startWakeWatcher(ctx context.Context) {
	watch := a.wakeWatchFunc
	if watch == nil {
		watch = wakewatch.Watch
	}
	events := watch(ctx)
	go func() {
		for r := range events {
			a.emitEvent("system:wake", string(r))
		}
	}()
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
