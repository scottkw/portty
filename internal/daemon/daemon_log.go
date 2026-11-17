package daemon

import (
	"os"
	"path/filepath"
)

// openDaemonLog rotates dir/daemon.log to dir/daemon.log.1 (replacing any
// older copy) and opens a fresh dir/daemon.log, mode 0600, for the daemon's
// stdout and stderr.
//
// Why: the detached daemon's stdio used to go to /dev/null, so nothing it
// logged survived, and UAT gap 1 (04-UAT.md) had no logs to read. One
// rotation per daemon start keeps the previous run for a post-mortem.
func openDaemonLog(dir string) (*os.File, error) {
	path := filepath.Join(dir, "daemon.log")
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
}
