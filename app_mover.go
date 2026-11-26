package main

// Mover mode (UPD-08): a Portty opened from a mounted DMG copies itself into the Applications
// folder, or opens the same or newer installed copy, and hands the rest (eject, swap, relaunch,
// DMG to the Trash) to the detached installer. It never starts the daemon, the tray, the pollers,
// the command-line links or the sweep: a daemon run from the volume would pin it.

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scottkw/portty/internal/updater"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/unix"
)

// moverStdoutRunner runs hdiutil for detection: stdout only, refused in a test binary.
var moverStdoutRunner = updater.SafeRunner(updater.StdoutRunner)

// moverRoots are the system Applications folder and the home folder; a var for tests.
var moverRoots = func() (system, home string) {
	home, _ = os.UserHomeDir()
	return "/Applications", home
}

// openAppPathFunc opens an installed Portty (Cancel on the Replace question); a var for tests.
var openAppPathFunc = func(ctx context.Context, app string) error { return openApp(ctx, app) }

// moverStageTimeout bounds the checks and the copy, as Stage's deadline does for an update.
const moverStageTimeout = 5 * time.Minute

// startMover enters mover mode when this GUI runs from a mounted DMG: it closes startupDone so
// nothing waits on a daemon that never starts, runs the mover in the background and reports true.
func (a *App) startMover() bool {
	detect := a.detectMoverFunc
	if detect == nil {
		detect = a.detectMover
	}
	run, ok := detect()
	if !ok {
		return false
	}
	a.moverMode.Store(true)
	if a.startupDone != nil {
		close(a.startupDone)
	}
	go a.runMover(run)
	return true
}

// detectMover reports whether the running bundle is on a read-only disk-image volume. Any error
// is logged and means a normal start.
func (a *App) detectMover() (updater.DMGRun, bool) {
	bundle, err := runningBundleFunc()
	if err != nil {
		log.Printf("mover: off: %v", err)
		return updater.DMGRun{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run, ok, err := updater.DetectDMGRun(ctx, updateRunner, moverStdoutRunner, bundle, statfsOrigin)
	if err != nil {
		log.Printf("mover: detection failed, starting normally: %v", err)
		return updater.DMGRun{}, false
	}
	return run, ok
}

// statfsOrigin is the mount-from name of path's file system: for a Gatekeeper-translocated app,
// where it really is.
func statfsOrigin(path string) (string, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return "", err
	}
	return unix.ByteSliceToString(st.Mntfromname[:]), nil
}

// runMover moves the DMG's Portty into place (P4: a second GUI may be running from an installed
// copy). No copy: stage beside /Applications/Portty.app (or ~/Applications) without asking. An
// older copy: ask first, then stage beside it; Cancel opens it and quits, leaving the DMG alone.
// A same or newer copy: open it (never a downgrade). The staged copy passes the update's checks
// before its quarantine is cleared. Any failure shows why, reveals the app on the volume and quits.
func (a *App) runMover(r updater.DMGRun) {
	version := strings.TrimPrefix(Version, "v")
	system, home := moverRoots()
	ctx, cancel := context.WithTimeout(context.Background(), moverStageTimeout)
	defer cancel()
	staged := ""
	fail := func(err error) {
		if staged != "" {
			_ = os.RemoveAll(filepath.Dir(staged))
		}
		updateLogf("mover: %v", err)
		a.setDockVisible(true) // an agent app's alert is not brought forward otherwise (Pitfall 10)
		a.alert("Portty couldn't move itself to the Applications folder.",
			updater.InstallFailureReason(err)+" Drag Portty to the Applications folder to install it.")
		_ = updater.Reveal(ctx, updateRunner, r.Bundle)
		a.onTerminate()
	}
	start := a.moverStartFunc
	if start == nil {
		start = a.startInstaller
	}
	job := updater.Job{V: updater.JobVersion, Kind: updater.KindMove, OldPID: os.Getpid(), NewVersion: version,
		ImagePath: r.ImagePath, MountPoint: r.MountPoint}

	target := updater.ExistingCopy(system, home)
	if target != "" {
		old, err := updater.InstalledVersion(ctx, updateRunner, target)
		if err == nil {
			if c, cerr := updater.Compare(version, old); cerr == nil && c <= 0 {
				job.Kind, job.Target, job.OldVersion = updater.KindOpenInstalled, target, old
				if err := start(job); err != nil {
					fail(err)
				}
				return
			}
		}
		title := "Replace the Portty in Applications?"
		if err == nil {
			title = "Replace Portty " + old + " in Applications?"
		} else {
			// shortcut: an unreadable installed version is recorded as 0.0.0 once the user agreed
			// to replace it, so the job passes the installer's never-downgrade rule.
			old = "0.0.0"
		}
		a.setDockVisible(true) // Pitfall 10, as above
		choice := a.dialog(runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: title,
			Message: "This copy is version " + version + ".", Buttons: []string{"Replace", "Cancel"},
			DefaultButton: "Replace", CancelButton: "Cancel"})
		if choice != "Replace" {
			if err := openAppPathFunc(ctx, target); err != nil {
				log.Printf("mover: installed copy not opened: %v", err)
			}
			a.onTerminate()
			return
		}
		job.OldVersion = old
	} else {
		var err error
		if target, err = updater.MoveTarget(system, home, func(d string) bool { return unix.Access(d, unix.W_OK) == nil }); err != nil {
			fail(fmt.Errorf("%w: target", updater.ErrNoWrite)) // the error names a path; the log must not
			return
		}
	}
	job.Target = target

	if err := updater.Preflight(target); err != nil {
		fail(err)
		return
	}
	if err := updater.VerifyApp(ctx, updateRunner, r.Bundle); err != nil {
		fail(err)
		return
	}
	size, err := updater.AppSize(r.Bundle)
	if err != nil {
		fail(fmt.Errorf("%w: app-size", updater.ErrCopy))
		return
	}
	if err := updater.CheckSpace(filepath.Dir(target), 2*size); err != nil {
		fail(err)
		return
	}
	if staged, err = updater.CopyApp(ctx, updateRunner, r.Bundle, target); err != nil {
		fail(err)
		return
	}
	for _, check := range []func() error{
		func() error { return updater.ClearQuarantine(ctx, updateRunner, staged) },
		func() error { return updater.VerifyApp(ctx, updateRunner, staged) },
		func() error { return updater.CheckStagedApp(ctx, updateRunner, staged, version) },
	} {
		if err := check(); err != nil {
			fail(err)
			return
		}
	}
	job.Staged = staged
	if err := start(job); err != nil {
		fail(err)
	}
}
