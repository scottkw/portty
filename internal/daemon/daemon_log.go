package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// daemonLogMax caps daemon.log during a run; past it the file rolls over to
// daemon.log.1 so a long-lived daemon cannot fill the disk (WR-01).
const daemonLogMax = 10 << 20

// openDaemonLog opens dir/daemon.log, mode 0600, in append mode for the
// detached daemon's stdout and stderr.
//
// Why: the detached daemon's stdio used to go to /dev/null, so nothing it
// logged survived, and UAT gap 1 (04-UAT.md) had no logs to read. The
// spawning parent never rotates: a duplicate or retried spawn must not move
// away the log of the run that just died. The daemon rotates once it holds
// the socket (startDaemonLog).
func openDaemonLog(dir string) (*os.File, error) {
	return os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
}

// startDaemonLog runs in the daemon after it has bound its socket, so only a
// daemon that is really starting rotates. If stderr is dir/daemon.log, the
// file so far (the previous run plus this run's startup lines) becomes
// daemon.log.1 and the log package writes through a size-capped writer.
// A stderr of /dev/null (launchd's default for the LaunchAgent, whose plist
// has no log paths) is first pointed at dir/daemon.log, so a launchd-managed
// daemon logs like the detached one (19-07 finding: under launchd every
// relay, handoff and re-home line was lost). Any other stderr (a terminal) is
// left alone.
func startDaemonLog(dir string) {
	path := filepath.Join(dir, "daemon.log")
	adoptNullStdio(path)
	cur, err1 := os.Stderr.Stat()
	onDisk, err2 := os.Stat(path)
	if err1 != nil || err2 != nil || !os.SameFile(cur, onDisk) {
		return
	}
	d := &daemonLog{path: path, max: daemonLogMax, out: os.Stderr, fds: []int{1, 2}}
	d.mu.Lock()
	d.rotate()
	d.mu.Unlock()
	log.SetOutput(d)
}

// adoptNullStdio points stdout and stderr at path when stderr is /dev/null.
// Best effort: on any error the daemon keeps running with its stdio as is.
func adoptNullStdio(path string) {
	cur, err1 := os.Stderr.Stat()
	null, err2 := os.Stat(os.DevNull)
	if err1 != nil || err2 != nil || !os.SameFile(cur, null) {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	for _, fd := range []int{1, 2} {
		_ = syscall.Dup2(int(f.Fd()), fd)
	}
}

// daemonLog writes to out and, when the next write would pass max bytes,
// renames path to path.1 and points fds (stdout and stderr in the daemon) at a
// fresh path, so direct stderr writes and Go panics follow the rotation.
type daemonLog struct {
	mu   sync.Mutex
	path string
	max  int64
	size int64
	out  *os.File
	fds  []int
}

func (d *daemonLog) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.size+int64(len(p)) > d.max {
		d.rotate()
	}
	n, err := d.out.Write(p)
	d.size += int64(n)
	return n, err
}

// rotate must be called with mu held. On failure it keeps writing to the
// current file and retries after another max bytes.
func (d *daemonLog) rotate() {
	d.size = 0
	if err := os.Rename(d.path, d.path+".1"); err != nil {
		fmt.Fprintf(d.out, "daemon: rotate log: %v\n", err)
		return
	}
	f, err := os.OpenFile(d.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(d.out, "daemon: rotate log: %v\n", err)
		return
	}
	defer f.Close()
	for _, fd := range d.fds {
		if err := syscall.Dup2(int(f.Fd()), fd); err != nil {
			fmt.Fprintf(d.out, "daemon: rotate log: %v\n", err)
		}
	}
}
