package main

// Self-install, GUI side (UPD-05): where the running bundle is, and the hand-off to the detached
// installer (cmd_install_update.go) that replaces it.

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

// startInstaller writes the job, starts a detached copy of this binary on it and, once the copy
// reports it is running, quits the GUI only (daemon and sessions untouched). Any failure before
// that leaves this process running and returns an error, and the Service falls back to Reveal.
func (a *App) startInstaller(j updater.Job) error {
	work := updateWorkDir()
	if err := os.MkdirAll(work, 0o700); err != nil {
		return err
	}
	run, err := os.MkdirTemp(work, "run-")
	if err != nil {
		return err
	}
	jobPath := filepath.Join(run, "job.json")
	bin := filepath.Join(run, "portty-installer")
	if err := updater.WriteJob(jobPath, j); err != nil {
		_ = os.RemoveAll(run)
		return err
	}
	if err := copyExecutable(bin); err != nil {
		_ = os.RemoveAll(run)
		return err
	}
	proc, err := spawnInstallerFunc(bin, jobPath)
	if err != nil {
		_ = os.RemoveAll(run)
		return err
	}
	marker := filepath.Join(run, updater.StartedMarker)
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if !time.Now().Before(deadline) {
			_ = proc.Kill()
			_, _ = proc.Wait()
			_ = os.RemoveAll(run)
			return errors.New("the installer did not report that it started")
		}
	}
	_ = proc.Release()
	log.Printf("update: installer started")
	a.onTerminate() // GUI-only quit: shutdown never stops the daemon
	return nil
}

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

// copyExecutable copies this binary to dst, a new file (0700) outside the bundle being replaced:
// the process that swaps the bundle must not run from inside it.
func copyExecutable(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	src, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy executable: %w", err)
	}
	return out.Close()
}
