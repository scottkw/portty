//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// startDetachedDaemon spawns the daemon as a detached child process on Unix.
// Setsid creates a new session so the daemon is not killed when the parent exits.
// The child's stdout and stderr are appended to daemon.log in the config dir
// (the daemon rotates it, see startDaemonLog); a log problem is reported on
// the parent's stderr but never stops the daemon.
func startDetachedDaemon(exe string) error {
	cmd := exec.Command(exe, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logFile, err := openDaemonLog(daemonConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "portty: daemon log: %v\n", err)
	} else {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close() // the child keeps its own descriptor
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// lockDaemon takes the exclusive daemon lock at path, retrying for up to wait
// so a daemon that was just told to stop (EnsureDaemon's version-mismatch
// restart) can exit first. Only the holder may clean up and bind the socket,
// so two daemons spawned at once never unlink each other's socket (WR-05).
// The kernel drops the lock when the process exits; the caller keeps the
// file open for its lifetime.
func lockDaemon(path string, wait time.Duration) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			f.Close()
			return nil, err
		}
		time.Sleep(50 * time.Millisecond)
	}
}
