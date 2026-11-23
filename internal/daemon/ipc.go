package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// daemonSocketMode is the mode of the daemon control socket: it is private to
// its user (write access to every shell).
const daemonSocketMode os.FileMode = 0o600

// listenDaemonSocket binds the socket and then chmods it to 0600. The brief
// bind-to-chmod window is fenced by the 0700 parent directory that
// DefaultSocketPath enforces (no other user can traverse to the inode), and
// the daemon also checks the peer UID. It deliberately does not touch the
// process-wide umask: the daemon forks shells and creates files on other
// goroutines, and a changed umask would leak into them.
func listenDaemonSocket(path string) (net.Listener, error) {
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	// Never serve on a wider mode; a failed chmod fails the start.
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
