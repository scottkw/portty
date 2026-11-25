package main

// Update checks (UPD-01..UPD-04), the GIFsmith flow on top of internal/updater: a release build
// checks GitHub about 10 s after launch and then once a day by the wall clock, retries a failed
// check on the next hourly tick, stays quiet about every failure on automatic checks, and
// notifies once per new version. Every state change reaches the frontend as update:status.

import (
	"context"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scottkw/portty/internal/updater"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Release-build update endpoint and cadence. Only the VM-only vmpreview build overrides them.
var (
	updateAPI                            = updater.GitHubAPI
	updateAllow      func(*url.URL) bool = updater.GitHubAllow
	updateFirstDelay                     = 10 * time.Second
	updateTick                           = time.Hour
	updateEvery                          = 24 * time.Hour
)

// updateRunner runs every update tool. SafeRunner refuses hdiutil, open and ditto in a test
// binary; only the VM-only vmpreview build (18-08) wraps it further.
var updateRunner updater.Runner = updater.SafeRunner(updater.ExecRunner)

func (a *App) newUpdateService() *updater.Service {
	return &updater.Service{
		Checker:        &updater.Checker{API: updateAPI, Repo: updater.Repo, HTTP: updater.NewCheckClient(updateAPI), Allow: updateAllow},
		Current:        strings.TrimPrefix(Version, "v"),
		Emit:           a.onUpdateStatus,
		Dir:            updateCacheDir(),
		Client:         updater.NewDownloadClient(updateAllow),
		Run:            updateRunner,
		Bundle:         bundleForUpdates(),
		StartInstaller: a.startInstaller,
	}
}

// updateCacheDir is ~/Library/Caches/app.portty.Portty/updates, or "" (Download then fails;
// there is no fallback folder).
func updateCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "app.portty.Portty", "updates")
}

// releaseBuild reports whether Version is a vX.Y.Z release; dev and pre-release builds never check.
func releaseBuild() bool {
	_, err := updater.ParseVersion(Version)
	return err == nil
}

// startUpdatePoller runs the automatic checks for release builds until ctx ends.
func (a *App) startUpdatePoller(ctx context.Context) {
	if !releaseBuild() || a.updates == nil {
		return
	}
	go a.runUpdateLoop(ctx)
}

// runUpdateLoop checks after updateFirstDelay, then wakes every updateTick and checks when
// updateEvery of wall-clock time has passed since the last completed check, or the last check
// failed. Not a plain 24 h ticker: Go timers stop while the Mac sleeps, so a laptop that sleeps
// every night would stretch it to days.
func (a *App) runUpdateLoop(ctx context.Context) {
	after, now := a.updateAfter, a.updateNow
	if after == nil {
		after = time.After
	}
	if now == nil {
		now = time.Now
	}
	var last time.Time // the last completed check; zero until one completes
	wait := updateFirstDelay
	for {
		select {
		case <-ctx.Done():
			return
		case <-after(wait):
		}
		wait = updateTick
		// A negative gap means the wall clock went back past the last check: check now, or no
		// check would run until the clock caught up.
		if d := now().Round(0).Sub(last); !last.IsZero() && d >= 0 && d < updateEvery {
			continue
		}
		if a.autoCheck(ctx) {
			last = now().Round(0) // Round(0) drops the monotonic reading, so the wall clock counts
		}
	}
}

// autoCheck runs one automatic check and reports whether it completed (200, 304 or nothing
// published). Failures only log, in Service.Check.
func (a *App) autoCheck(ctx context.Context) bool {
	st, err := a.updates.Check(ctx) // Check bounds itself once it has its turn
	if err != nil {
		return false
	}
	if st.State == "available" {
		a.notifyUpdateOnce(st.Latest)
	}
	return true
}

// onUpdateStatus is the Service's Emit. It runs under the service mutex, so it must not block:
// emitEvent and the tray setter both return at once. The tray offers the update while a newer
// version is known and is left alone during a check.
func (a *App) onUpdateStatus(st updater.Status) {
	a.emitEvent("update:status", st)
	switch {
	case st.State == "checking":
	case st.Latest != "":
		a.setTrayUpdate("Update Available…")
	default:
		a.setTrayUpdate("")
	}
}

// updateNotificationID identifies the update-available notification; handleNotificationClick
// matches it.
const updateNotificationID = "portty.update-available"

func notifiedVersionPath() string { return filepath.Join(configDir(), "update_notified") }

// notifyUpdateOnce sends one native notification per new version. A read error counts as not
// yet notified: at worst one extra notification.
func (a *App) notifyUpdateOnce(version string) {
	b, _ := os.ReadFile(notifiedVersionPath())
	if strings.TrimSpace(string(b)) == version {
		return
	}
	a.sendNotificationFunc(updateNotificationID, "Portty", "Version "+version+" is available.")
	rememberNotified(version)
	log.Printf("update: notified %s", version)
}

func rememberNotified(version string) {
	if err := os.WriteFile(notifiedVersionPath(), []byte(version+"\n"), 0o600); err != nil {
		log.Printf("update: remember notified version: %v", err)
	}
}

// GetUpdateStatus is the banner's starting point; update:status events may fire before React
// subscribes.
func (a *App) GetUpdateStatus() updater.Status {
	if a.updates == nil {
		return updater.Status{State: "idle", Current: strings.TrimPrefix(Version, "v")}
	}
	return a.updates.Status()
}

// DownloadUpdate downloads, verifies and reveals the available update (UPD-03); the banner's and
// the alert's Download buttons both call it. A failure is in the returned Status.Error.
func (a *App) DownloadUpdate() updater.Status {
	if a.updates == nil {
		return a.GetUpdateStatus()
	}
	st, _ := a.updates.Download()
	return st
}

// checkForUpdatesManually is Help → Check for Updates…: a check at once that always answers with
// a native alert. Unexported so Wails never binds it. The alert blocks, so call it on a goroutine.
func (a *App) checkForUpdatesManually() {
	if !releaseBuild() || a.updates == nil {
		a.alert("This build doesn't check for updates.", "Only release builds of Portty check GitHub for new versions.")
		return
	}
	st, err := a.updates.Check(context.Background()) // its own budget starts after any running check
	switch {
	case err != nil:
		a.alert("Couldn't check for updates.", err.Error())
	case st.State == "downloading" || st.State == "verifying":
		a.alert("Portty is downloading version "+st.Latest+".", "It will be shown in Finder once it's verified.")
	case st.State == "available":
		rememberNotified(st.Latest) // the alert already told the user; no notification for it later
		a.offerDownload(st)
	default:
		a.alert("Portty is up to date.", "Version "+st.Current+" is the newest version.")
	}
}

// offerUpdate is the tray item's action: offer a known update, or show the window with the
// banner while one downloads.
func (a *App) offerUpdate() {
	st := a.GetUpdateStatus()
	switch st.State {
	case "available", "revealed":
		a.showWindow()
		a.offerDownload(st)
	case "downloading", "verifying":
		a.showWindow()
	}
}

// offerDownload asks Download or Later; Download shows the window, where the banner tracks progress.
func (a *App) offerDownload(st updater.Status) {
	choice := a.dialog(runtime.MessageDialogOptions{
		Type: runtime.InfoDialog, Title: "Version " + st.Latest + " is available.", Message: "You have version " + st.Current + ".",
		Buttons: []string{"Download", "Later"}, DefaultButton: "Download", CancelButton: "Later",
	})
	if choice == "Download" {
		a.showWindow()
		a.DownloadUpdate()
	}
}

func (a *App) alert(title, message string) {
	a.dialog(runtime.MessageDialogOptions{Type: runtime.InfoDialog, Title: title, Message: message, Buttons: []string{"OK"}, DefaultButton: "OK"})
}

// dialog shows an app-modal NSAlert and waits for the answer: never call it on the main thread.
func (a *App) dialog(opts runtime.MessageDialogOptions) string {
	if a.messageDialogFunc != nil {
		choice, _ := a.messageDialogFunc(opts)
		return choice
	}
	if a.ctx == nil {
		return ""
	}
	choice, err := runtime.MessageDialog(a.ctx, opts)
	if err != nil {
		log.Printf("update: alert failed: %v", err)
	}
	return choice
}

func (a *App) showWindow() {
	if a.showWindowFunc != nil {
		a.showWindowFunc()
		return
	}
	if a.ctx != nil && a.ctx.Value("frontend") != nil { // no Wails frontend in unit tests, as in emitEvent
		runtime.WindowUnminimise(a.ctx) // WindowShow alone leaves a minimized window in the Dock
		runtime.WindowShow(a.ctx)
		a.setDockVisible(true)
	}
}

// handleNotificationClick runs when the user clicks a notification. For the update notification it
// brings Portty forward and asks the UI to show the update banner even if it was dismissed. It must
// never start the update: the user presses the banner's button for that (CONTEXT decision 2026-11-22).
func (a *App) handleNotificationClick(id string) {
	if id != updateNotificationID {
		return
	}
	a.showWindow()
	a.emitEvent("update:show", nil)
}

// setTrayUpdate sets the tray's update item title ("" hides it).
func (a *App) setTrayUpdate(title string) {
	if a.setTrayUpdateFunc != nil {
		a.setTrayUpdateFunc(title)
		return
	}
	if a.trayInit {
		setTrayUpdateItem(title)
	}
}
