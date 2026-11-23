package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

// daemonSocketMode is the mode of the daemon control socket: it is private to
// its user (write access to every shell). The 0700 parent directory is the
// second fence (DefaultSocketPath tightens it).
const daemonSocketMode os.FileMode = 0o600

// umaskMu serialises the process-wide umask change around the bind.
var umaskMu sync.Mutex

func listenDaemonSocket(path string) (net.Listener, error) {
	// Bind under a 0177 umask so the socket inode is created 0600, not
	// 0777&^umask: there is no window in which it is wider. The umask is
	// restored before anything else runs, on the error path too.
	umaskMu.Lock()
	old := syscall.Umask(0o177)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	umaskMu.Unlock()
	if err != nil {
		return nil, err
	}
	// Belt and braces: never serve on a wider mode; a failed chmod fails the start.
	if err := os.Chmod(path, daemonSocketMode); err != nil {
		ln.Close()
		return nil, fmt.Errorf("daemon socket: set mode %o on %s: %w", daemonSocketMode, path, err)
	}
	return ln, nil
}

// tightenSocketMode sets a received socket to daemonSocketMode: one inherited
// from a pre-fix daemon keeps its old mode. It runs after COMMIT, so a failure
// is only logged and never costs the moved sessions.
func tightenSocketMode(path string) {
	if err := os.Chmod(path, daemonSocketMode); err != nil {
		log.Printf("handoff: could not set the daemon socket to %o: %v", daemonSocketMode, err)
	}
}

func dialDaemonSocket(ctx context.Context, path string) (net.Conn, error) {
	return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", path)
}

func removeDaemonSocket(path string) error {
	return os.Remove(path)
}
