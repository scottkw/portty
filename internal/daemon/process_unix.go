//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// startDetachedDaemon spawns the daemon as a detached child process on Unix.
// Setsid creates a new session so the daemon is not killed when the parent exits.
// The child's stdout and stderr go to daemon.log in the config dir; a log
// problem is reported on the parent's stderr but never stops the daemon.
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
