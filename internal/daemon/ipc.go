package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

// daemonSocketMode is the mode of the daemon control socket: it is private to
// its user (write access to every shell). The 0700 parent directory is the
// second fence.
const daemonSocketMode os.FileMode = 0o600

func listenDaemonSocket(path string) (net.Listener, error) {
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	// Never serve on a wider mode: a failed chmod fails the start.
	if err := os.Chmod(path, daemonSocketMode); err != nil {
		ln.Close()
		return nil, fmt.Errorf("daemon socket: set mode %o on %s: %w", daemonSocketMode, path, err)
	}
	return ln, nil
}

func dialDaemonSocket(ctx context.Context, path string) (net.Conn, error) {
	return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", path)
}

func removeDaemonSocket(path string) error {
	return os.Remove(path)
}
