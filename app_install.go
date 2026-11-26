package main

// Self-install, GUI side (UPD-05): where the running bundle is, and the hand-off to the detached
// installer (cmd_install_update.go) that replaces it.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/updater"
)

// bundleFromExecutable is the pure path rule: the bundle is three folders above the executable,
// must be named Portty.app, and the executable must be Contents/MacOS/Portty in it.
func bundleFromExecutable(exe string) (string, error) {
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if filepath.Base(bundle) != updater.BundleName {
		return "", errors.New("the executable is not inside a Portty.app")
	}
	if exe != filepath.Join(bundle, updater.BundleExecutable) {
		return "", errors.New("the executable is not Contents/MacOS/Portty")
	}
	return bundle, nil
}

// runningBundle is the Portty.app this process runs from, symlinks resolved.
func runningBundle() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	return bundleFromExecutable(exe)
}

// runningBundleFunc is a var for tests.
var runningBundleFunc = runningBundle

// bundleForUpdates is the bundle self-install replaces, or "" for a dev build or test binary,
// which keeps the reveal flow. The log line says why, so a release build cannot lose
// self-install silently.
func bundleForUpdates() string {
	bundle, err := runningBundle()
	if err != nil {
		log.Printf("update: self-install off: %v", err)
		return ""
	}
	return bundle
}

// installerStartWait bounds spawn to started marker only: the download, verify, staging and the
// installer copy all happen before the clock starts. 18-10 Task 2 measured spawn-to-marker at
// 417 ms on a Developer ID build in the VM (row P2d-bundle), so 3 s keeps over 2x headroom.
const installerStartWait = 3 * time.Second

// installerCopyTimeout bounds copyBundle (ditto, the codesign check, the quarantine strip).
const installerCopyTimeout = 2 * time.Minute

// installerStartWaitFor is installerStartWait; tests lower it.
var installerStartWaitFor = installerStartWait

// startInstaller starts install-update from a whole copy of the running Portty.app in a fresh
// run folder and, once the copy reports it is running, quits the GUI only (daemon and sessions
// untouched). Any failure before that removes the run folder, leaves this process running, names
// the failed step in update.log and returns an error, and the Service falls back to Reveal.
func (a *App) startInstaller(j updater.Job) error {
	run := ""
	fail := func(step string, err error) error {
		if run != "" {
			_ = os.RemoveAll(run)
		}
		updateLogf("update: installer not started: %s", step)
		return fmt.Errorf("installer not started: %s: %w", step, err)
	}
	src, err := runningBundleFunc()
	if err != nil {
		return fail("bundle", err)
	}
	work := updateWorkDir()
	if err := os.MkdirAll(work, 0o700); err != nil {
		return fail("workdir", err)
	}
	if run, err = os.MkdirTemp(work, "run-"); err != nil {
		return fail("workdir", err)
	}
	jobPath := filepath.Join(run, "job.json")
	if err := updater.WriteJob(jobPath, j); err != nil {
		return fail("job", err)
	}
	helper := filepath.Join(run, updater.BundleName)
	copyStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), installerCopyTimeout)
	step, err := copyBundleFunc(ctx, src, helper)
	cancel()
	if err != nil {
		return fail(step, err)
	}
	copyTook := time.Since(copyStart)
	spawnStart := time.Now()
	proc, err := spawnInstallerFunc(filepath.Join(helper, updater.BundleExecutable), jobPath)
	if err != nil {
		return fail("spawn", err)
	}
	marker := filepath.Join(run, updater.StartedMarker)
	for deadline := spawnStart.Add(installerStartWaitFor); ; time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if !time.Now().Before(deadline) {
			_ = proc.Kill()
			_, _ = proc.Wait()
			return fail("no-marker", errors.New("the installer did not report that it started"))
		}
	}
	startTook := time.Since(spawnStart)
	_ = proc.Release()
	updateLogf("update: installer started (copy %dms, start %dms)", copyTook.Milliseconds(), startTook.Milliseconds())
	if vmFaultHook != nil {
		vmFaultHook("before-quit")
	}
	a.onTerminate() // GUI-only quit: shutdown never stops the daemon
	return nil
}

// copyBundle copies the whole running bundle src to dst with ditto (signature, sealed resources
// and xattrs kept), checks the copy's signature and Team, then removes the copy's quarantine
// flag. It returns the failed step (copy, copy-check, quarantine) with the error. The whole
// bundle, because a Developer ID signature binds Info.plist and a lone executable outside its
// bundle dies at exec (P2d). The copy is checked before its flag is removed (UPD-09). src is
// only ditto's source and is never modified.
func copyBundle(ctx context.Context, src, dst string) (string, error) {
	if _, err := updateRunner(ctx, "/usr/bin/ditto", src, dst); err != nil {
		return "copy", err
	}
	if err := updater.VerifyApp(ctx, updateRunner, dst); err != nil {
		return "copy-check", err
	}
	if err := updater.ClearQuarantine(ctx, updateRunner, dst); err != nil {
		return "quarantine", err
	}
	return "", nil
}

// copyBundleFunc is a var for tests.
var copyBundleFunc = copyBundle

// spawnInstallerFunc is a var for tests.
var spawnInstallerFunc = spawnInstaller

// spawnInstaller starts `bin install-update jobPath` detached (own session, cwd /), with output
// appended to update.log. Like spawnTakeover, a test binary never starts one.
func spawnInstaller(bin, jobPath string) (*os.Process, error) {
	if testing.Testing() {
		return nil, errors.New("refusing to spawn an installer from a test binary")
	}
	cmd := exec.Command(bin, installUpdateCmd, jobPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Dir = "/"
	logFile, err := openUpdateLog()
	if err != nil {
		log.Printf("update: installer log not opened") // the installer still runs, output to /dev/null
	} else {
		cmd.Stdout, cmd.Stderr = logFile, logFile
		defer func() { _ = logFile.Close() }() // the child keeps its own copy
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}

// openUpdateLog opens update.log (0600) for append, rotating it to update.log.1 above 1 MiB.
func openUpdateLog() (*os.File, error) {
	p := filepath.Join(configDir(), "update.log")
	if fi, err := os.Stat(p); err == nil && fi.Size() > 1<<20 {
		_ = os.Rename(p, p+".1")
	}
	return os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

// updateLogf logs the line and appends it to update.log without rotating (only spawnInstaller
// rotates, before the child opens the file): a LaunchServices-launched GUI's stderr goes
// nowhere. Callers pass step names and numbers only, never a path.
func updateLogf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	log.Print(line)
	f, err := os.OpenFile(filepath.Join(configDir(), "update.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(f, line)
	_ = f.Close()
}

// sweepAge is how old a staging or run dir must be before the startup sweep touches it. An
// installer lives at most QuitWait + HealthTimeout + 2 minutes, so an hour never catches a live one.
const sweepAge = time.Hour

// trashItemFunc is a var for tests.
var trashItemFunc = trashItem

// sweepParents is every folder a Portty.app and its staging dirs can sit in: the running
// bundle's parent, /Applications and ~/Applications, deduped.
func sweepParents() []string {
	var dirs []string
	if b, err := runningBundleFunc(); err == nil {
		dirs = append(dirs, filepath.Dir(b))
	}
	dirs = append(dirs, "/Applications")
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	var out []string
	for _, d := range dirs {
		if d = filepath.Clean(d); !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

// sweepUpdateLeftovers cleans what an interrupted install left behind (UPD-06). It runs once per
// normal GUI start, in the background after the daemon is up.
func sweepUpdateLeftovers(ctx context.Context, now time.Time) {
	sweepDirs(ctx, now, sweepParents(), updateWorkDir(), updateCacheDir())
}

// sweepDirs: in parents, a .Portty-update-* dir older than sweepAge has its Portty.app moved to
// the Trash (never deleted; a refused Trash leaves it for the next start) and is then removed if
// empty. In work, run-* dirs older than sweepAge are removed: they
// hold only Portty's own job file, marker and installer copy, never the user's app. In cache,
// mnt-* dirs are detached (no age floor: no stage runs in a process that is just starting).
// Symlinks are never followed or touched.
func sweepDirs(ctx context.Context, now time.Time, parents []string, work, cache string) {
	cutoff := now.Add(-sweepAge)
	// entries lists the real dirs in dir named prefix*; with stale, only those older than cutoff.
	entries := func(dir, prefix string, stale bool) []string {
		des, _ := os.ReadDir(dir) // a missing folder has nothing to sweep
		var out []string
		for _, de := range des {
			p := filepath.Join(dir, de.Name())
			fi, err := os.Lstat(p)
			if strings.HasPrefix(de.Name(), prefix) && err == nil && fi.IsDir() && (!stale || fi.ModTime().Before(cutoff)) {
				out = append(out, p)
			}
		}
		return out
	}
	var trashed, kept, runs, detached int
	for _, parent := range parents {
		for _, dir := range entries(parent, updater.StagingPrefix, true) {
			if app := filepath.Join(dir, updater.BundleName); isRealDir(app) {
				if err := trashItemFunc(app); err != nil {
					kept++
					updateLogf("update: sweep: old copy kept aside: %v", err)
				} else {
					trashed++
				}
			}
			_ = os.Remove(dir) // fails harmlessly while a copy is still inside
		}
	}
	if work != "" {
		for _, dir := range entries(work, "run-", true) {
			if os.RemoveAll(dir) == nil {
				runs++
			}
		}
	}
	if cache != "" {
		for _, dir := range entries(cache, "mnt-", false) {
			if !isRealDir(filepath.Join(dir, updater.BundleName)) {
				_ = os.Remove(dir)
				continue
			}
			dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if updater.Detach(dctx, updateRunner, dir) == nil {
				detached++
			}
			cancel()
		}
	}
	if trashed+kept+runs+detached > 0 {
		updateLogf("update: sweep trashed %d, kept aside %d, run dirs removed %d, mounts detached %d", trashed, kept, runs, detached)
	}
}

// isRealDir reports whether path is a directory, not a symlink to one.
func isRealDir(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.IsDir()
}

// updateResultNotificationID identifies the notification about a rolled-back or failed install.
const updateResultNotificationID = "portty.update-result"

// vmFaultHook is nil in release builds; only the VM-only vmpreview build sets it, to make a rig
// build fail at a named stage (before-quit, before-daemon, after-daemon) for the rollback rows.
var vmFaultHook func(stage string)

// consumeInstallResult takes the installer's result once, at startup before the daemon is
// ensured. A rollback's keepDaemon makes the next ensure keep whichever daemon holds the sessions
// and ask nothing (a newer release's daemon cannot hand them back, handoff.go). A rolled-back or
// failed install is told to the user: the banner with the kept DMG revealed, and a notification
// (UPD-07). A missing or refused result file changes nothing.
func (a *App) consumeInstallResult() {
	r, ok, err := updater.TakeResult(installResultPath())
	if err != nil {
		updateLogf("update: result ignored: %v", err)
		return
	}
	if !ok {
		return
	}
	updateLogf("update: result %s %s to %s %s", r.Outcome, r.OldVersion, r.NewVersion, r.Reason)
	if r.KeepDaemon {
		a.keepDaemonOnce.Store(true)
	}
	var body string
	switch r.Outcome {
	case updater.OutcomeRolledBack:
		body = "The update to Portty " + r.NewVersion + " didn't take. Portty " + r.OldVersion + " is still installed."
	case updater.OutcomeFailed:
		body = "Portty couldn't install version " + r.NewVersion + "."
	default:
		return
	}
	_, nerr := updater.ParseVersion(r.NewVersion)
	if _, oerr := updater.ParseVersion(r.OldVersion); nerr != nil || oerr != nil {
		updateLogf("update: result versions malformed")
		return
	}
	if a.updates != nil {
		a.updates.ShowResult(context.Background(), r)
	}
	a.sendNotificationFunc(updateResultNotificationID, "Portty", body)
}
