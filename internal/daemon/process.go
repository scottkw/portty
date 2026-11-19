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
// as the first argument, then polls until the daemon is ready (waitForDaemon).
func EnsureDaemon(socketPath string) error {
	client := NewDaemonClient(socketPath)
	needsSpawn := true
	if err := client.Health(); err == nil {
		// Daemon is reachable — check version compatibility first.
		if ver := client.GetDaemonVersion(); staleVersion(ver) {
			fmt.Fprintf(os.Stderr, "EnsureDaemon: version mismatch (daemon=%q, binary=%q), restarting\n", ver, BuildVersion)
			_ = client.ShutdownDaemon()
			// No socket cleanup here (review it3 WR-03): only the spawned
			// daemon unlinks it, under the lock, once the old one let go.
			// startDaemon spawns only after that (WR-07).
			// fall through to spawn
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

	return startDaemon(client, daemonReadyWait, filepath.Join(filepath.Dir(socketPath), "daemon.lock"),
		func() (<-chan struct{}, error) { return startDetachedDaemon(exe) })
}

// startDaemon calls spawn once no other process holds the lock at lockPath,
// then waits for the daemon (waitForDaemon), all within wait. 08-REVIEW
// WR-07: a version-mismatched old daemon can take longer to stop than a new
// daemon's 2 s lock wait, so spawning at once lost the lock and then waited
// out the rest of the 15 s. A held lock is never broken: the wait ends early
// when the socket answers (a daemon is up) or at the deadline.
func startDaemon(client *DaemonClient, wait time.Duration, lockPath string, spawn func() (<-chan struct{}, error)) error {
	deadline := time.Now().Add(wait)
	for lockHeld(lockPath) {
		if daemonReady(client) {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("EnsureDaemon: daemon lock still held after %v", wait)
		}
		time.Sleep(50 * time.Millisecond)
	}
	exited, err := spawn()
	if err != nil {
		return fmt.Errorf("EnsureDaemon: start daemon: %w", err)
	}
	return waitForDaemon(client, time.Until(deadline), exited, lockPath)
}

// staleVersion reports whether a daemon reporting ver is from another build.
// Production builds (non-"dev") compare versions; a stale daemon from an older
// release won't have newly added API routes (e.g. start-minimized).
func staleVersion(ver string) bool {
	return BuildVersion != "" && BuildVersion != "dev" && ver != BuildVersion
}

// daemonReady reports whether the daemon at client is fully ready: health,
// this build's version and relay port. 08-REVIEW CR-01: a version-mismatched
// daemon told to stop still answers, and holds the lock, until it exits.
func daemonReady(client *DaemonClient) bool {
	if client.Health() != nil || staleVersion(client.GetDaemonVersion()) {
		return false
	}
	port, err := client.GetRelayPort()
	return err == nil && port > 0
}

// daemonReadyWait bounds how long EnsureDaemon waits for a spawned daemon.
// DEBT-06: at login a daemon took 10 s from spawn to bind (08-VM-CHECKS
// B-07). The old 3 s failed the GUI's startup, which skips the launch grace,
// so the poller respawned a second daemon at once.
const daemonReadyWait = 15 * time.Second

// waitForDaemon polls until the daemon is fully ready (daemonReady) or wait
// passes. WR-04: once exited closes (the spawned daemon is gone) it fails as
// soon as nothing answers and no other process holds the lock at lockPath.
// While one does, the spawn lost the lock to a daemon still starting (a
// concurrent EnsureDaemon's), which gets the rest of the wait; WR-07: the
// lock is checked again on every poll, so a holder that lets go without
// binding fails the wait then, not at the deadline. A nil exited never fires.
func waitForDaemon(client *DaemonClient, wait time.Duration, exited <-chan struct{}, lockPath string) error {
	deadline := time.Now().Add(wait)
	exitedSeen := false
	for time.Now().Before(deadline) {
		if daemonReady(client) {
			return nil
		}
		select {
		case <-exited:
			exited, exitedSeen = nil, true
		case <-time.After(50 * time.Millisecond):
		}
		if exitedSeen && !lockHeld(lockPath) && !daemonReady(client) {
			return fmt.Errorf("EnsureDaemon: daemon exited during startup (see daemon.log)")
		}
	}
	return fmt.Errorf("EnsureDaemon: daemon did not become ready within %v", wait.Round(time.Second))
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
