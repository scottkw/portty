package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// BuildVersion is set by the main package at startup. The daemon embeds it in
// the health response so that EnsureDaemon can detect stale daemons from older
// builds and restart them.
var BuildVersion string

// RunDaemon is the daemon's main entry point. It creates a signal context and
// delegates to runDaemonCore. Called from main.go when os.Args[1] == "daemon".
func RunDaemon() {
	// Microseconds so viewer-diag lines within a few seconds can be ordered (daemon.log).
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	runDaemonCore(ctx)
}

// runDaemonCore is the blocking core of the daemon. It creates a SessionEngine,
// starts the API (with relay server inside), and blocks until ctx is cancelled.
// Using a context parameter allows it to be driven by either signal handling
// (RunDaemon) or a service manager (daemonSvc).
func runDaemonCore(ctx context.Context) {
	AugmentServicePath() // Must be before NewSessionEngine / any exec.LookPath
	socketPath := DefaultSocketPath()
	// WR-05: the lock comes before any socket cleanup or bind. A daemon that
	// cannot get it within the wait leaves the running one alone.
	lock, err := lockDaemon(filepath.Join(filepath.Dir(socketPath), "daemon.lock"), 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: already running (lock: %v)\n", err)
		return
	}
	defer lock.Close() // held for the process lifetime
	if err := CleanupStaleSocket(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: %v\n", err)
		return
	}

	engine := NewSessionEngine()
	api := NewAPI(engine)

	// Phase 87 (D-04): bootstrap the capability signing key BEFORE any share
	// server can start. A missing key here is fatal
	// — without it, requireCapability would 401 every request and the daemon
	// is effectively a brick for web access.
	if err := api.BootstrapCapabilityState(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: bootstrap capability state: %v\n", err)
		return
	}

	// Start the relay TCP server inside the daemon.
	relayPort, err := api.StartRelay()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: start relay: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "daemon: relay listening on port %d\n", relayPort)

	if err := api.Start(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: start api: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "daemon: listening on %s\n", socketPath)
	startDaemonLog(daemonConfigDir()) // after the bind: a failed duplicate spawn never rotates

	// Review it3 WR-03: a daemon whose socket path was removed or rebound is
	// unreachable but still holds the lock, so no replacement could ever start.
	// It exits instead. No watch where the socket is no file (Windows pipe).
	lost := make(chan struct{})
	if bound, err := os.Stat(socketPath); err == nil {
		go func() {
			if socketLost(ctx, socketPath, bound, 5*time.Second) {
				close(lost)
			}
		}()
	}

	// No web server starts at boot: the share server starts on the first
	// share and stops when nothing is shared (share.go).

	select {
	case <-ctx.Done():
	case <-lost:
		fmt.Fprintf(os.Stderr, "daemon: socket %s was removed or replaced; exiting to free the lock\n", socketPath)
		// The path may now be another daemon's socket: close without unlinking it.
		if ul, ok := api.ln.(*net.UnixListener); ok {
			ul.SetUnlinkOnClose(false)
			_ = ul.Close()
			api.ln = nil
		}
	}

	fmt.Fprintf(os.Stderr, "daemon: shutting down\n")
	_ = api.Stop()
	engine.Manager().Shutdown()
}

// EnsureDaemon checks if the daemon is reachable on socketPath. If not, it
// spawns a detached daemon subprocess using the current binary with "daemon"
// as the first argument, then polls until the daemon is ready (max 3 seconds).
func EnsureDaemon(socketPath string) error {
	client := NewDaemonClient(socketPath)
	needsSpawn := true
	if err := client.Health(); err == nil {
		// Daemon is reachable — check version compatibility first.
		// Production builds (non-"dev") compare versions; a stale daemon from
		// an older release won't have newly added API routes (e.g. start-minimized).
		if BuildVersion != "" && BuildVersion != "dev" {
			if ver := client.GetDaemonVersion(); ver != BuildVersion {
				fmt.Fprintf(os.Stderr, "EnsureDaemon: version mismatch (daemon=%q, binary=%q), restarting\n", ver, BuildVersion)
				_ = client.ShutdownDaemon()
				time.Sleep(200 * time.Millisecond)
				// No socket cleanup here (review it3 WR-03): only the spawned
				// daemon unlinks it, under the lock, once the old one let go.
				// fall through to spawn
			} else {
				needsSpawn = false
			}
		} else {
			needsSpawn = false
		}
		if !needsSpawn {
			// Same version (or dev mode) — verify relay is also ready.
			if port, relayErr := client.GetRelayPort(); relayErr == nil && port > 0 {
				return nil // fully operational
			}
			// Stale daemon without relay support. Log and continue to spawn a new one.
			// The socket is left alone: the new daemon cleans up under the lock.
			fmt.Fprintf(os.Stderr, "EnsureDaemon: stale daemon detected (relay not ready), respawning\n")
		}
	}

	// A test binary must never spawn a detached daemon: the child re-runs the
	// test binary, outlives the test, and holds the user's real socket.
	if testing.Testing() {
		return fmt.Errorf("EnsureDaemon: refusing to spawn a daemon from a test binary")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("EnsureDaemon: locate binary: %w", err)
	}

	if err := startDetachedDaemon(exe); err != nil {
		return fmt.Errorf("EnsureDaemon: start daemon: %w", err)
	}

	// Poll until daemon is fully ready — health + relay port (max 3 seconds).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Health(); err == nil {
			if port, relayErr := client.GetRelayPort(); relayErr == nil && port > 0 {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("EnsureDaemon: daemon did not become ready within 3s")
}

// socketLost checks every interval whether path still names the socket this
// daemon bound (bound). It returns true once the path is gone or names another
// file, and false when ctx ends first. Other stat errors are not counted: a
// false alarm would end every session.
func socketLost(ctx context.Context, path string, bound os.FileInfo, every time.Duration) bool {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
			cur, err := os.Stat(path)
			if errors.Is(err, fs.ErrNotExist) || (err == nil && !os.SameFile(bound, cur)) {
				return true
			}
		}
	}
}
