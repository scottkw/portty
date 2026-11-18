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
)

// Release-build update endpoint and cadence. Only the VM-only vmpreview build overrides them.
var (
	updateAPI                            = updater.GitHubAPI
	updateAllow      func(*url.URL) bool = updater.GitHubAllow
	updateFirstDelay                     = 10 * time.Second
	updateTick                           = time.Hour
	updateEvery                          = 24 * time.Hour
)

func (a *App) newUpdateService() *updater.Service {
	return &updater.Service{
		Checker: &updater.Checker{API: updateAPI, Repo: updater.Repo, HTTP: updater.NewCheckClient(updateAPI), Allow: updateAllow},
		Current: strings.TrimPrefix(Version, "v"),
		Emit:    a.onUpdateStatus,
		Dir:     updateCacheDir(),
		Client:  updater.NewDownloadClient(updateAllow),
		Run:     updater.ExecRunner,
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
		if !last.IsZero() && now().Round(0).Sub(last) < updateEvery {
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
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	st, err := a.updates.Check(ctx)
	if err != nil {
		return false
	}
	if st.State == "available" {
		a.notifyUpdateOnce(st.Latest)
	}
	return true
}

// onUpdateStatus is the Service's Emit. It runs under the service mutex, so it must not block.
func (a *App) onUpdateStatus(st updater.Status) {
	a.emitEvent("update:status", st)
}

func notifiedVersionPath() string { return filepath.Join(configDir(), "update_notified") }

// notifyUpdateOnce sends one native notification per new version. A read error counts as not
// yet notified: at worst one extra notification.
func (a *App) notifyUpdateOnce(version string) {
	b, _ := os.ReadFile(notifiedVersionPath())
	if strings.TrimSpace(string(b)) == version {
		return
	}
	a.sendNotificationFunc("portty.update-available", "Portty", "Version "+version+" is available.")
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

// CheckForUpdates runs a check at once for the Help menu. Task 3 replaces it with native alerts.
func (a *App) CheckForUpdates() updater.Status {
	if !releaseBuild() || a.updates == nil {
		return a.GetUpdateStatus()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, _ := a.updates.Check(ctx)
	return st
}
