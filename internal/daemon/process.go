package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// BuildVersion is set by the main package at startup. The daemon embeds it in
// the health response so that EnsureDaemon can detect stale daemons from older
// builds and restart them.
var BuildVersion string

// launchdManaged is whether launchd started this daemon (GET /health
// "launchd"). Set once at daemon start, before the API serves; a takeover
// daemon (RunTakeover) leaves it false.
var launchdManaged bool

// RunDaemon is the daemon's main entry point. It creates a signal context and
// delegates to runDaemonCore. Called from main.go when os.Args[1] == "daemon".
func RunDaemon() {
	// Microseconds so viewer-diag lines within a few seconds can be ordered (daemon.log).
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	runDaemonCore(ctx)
}

// takeoverStatusEnv names the status pipe's fd in a takeover that
// startDetachedTakeover started.
const takeoverStatusEnv = "PORTTY_TAKEOVER_STATUS_FD"

// RunTakeover is `portty daemon --takeover`: it takes the sessions of the
// running daemon (runTakeover) and, on success, serves them as the daemon.
// Only EnsureDaemon spawns it, with the status pipe; its outcome byte (0
// once committed) is what EnsureDaemon decides from. On failure it exits
// with the outcome code and the old daemon keeps serving.
func RunTakeover() {
	status := takeoverStatus() // first: no child may inherit the pipe or the variable
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	socketPath := DefaultSocketPath()
	code := takeoverFailed
	// No daemon answering: nothing to take over, so no cert warm-up either.
	if err := NewDaemonClient(socketPath).Health(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: no running daemon to take over (%v)\n", err)
	} else {
		AugmentServicePath() // before NewSessionEngine, as runDaemonCore
		engine := NewSessionEngine()
		api := NewAPI(engine)
		deps := takeoverDeps{socketPath: socketPath, lockPath: filepath.Join(filepath.Dir(socketPath), "daemon.lock"), api: api, build: BuildVersion}
		if code = runTakeover(ctx, deps); code == takeoverCommitted {
			reportTakeover(status, code)
			serveDaemon(ctx, api, engine, socketPath)
			return
		}
	}
	reportTakeover(status, code)
	fmt.Fprintf(os.Stderr, "daemon: takeover failed: %s\n", moveFailedReason(code))
	os.Exit(code)
}

// takeoverStatus returns the status pipe startDetachedTakeover gave this
// process, or nil when run by hand. It unsets the variable and marks the fd
// close-on-exec at once (T-09-28), so shells the daemon starts get neither.
func takeoverStatus() *os.File {
	v, ok := os.LookupEnv(takeoverStatusEnv)
	if !ok {
		return nil
	}
	os.Unsetenv(takeoverStatusEnv)
	fd, err := strconv.Atoi(v)
	if err != nil || fd < 3 { // never stdin, stdout or stderr
		return nil
	}
	syscall.CloseOnExec(fd)
	return os.NewFile(uintptr(fd), "takeover-status")
}

// reportTakeover writes the outcome byte and closes the pipe. A write error
// (EPIPE once the spawner has gone) is ignored; on this fd it is an error,
// not a signal.
func reportTakeover(status *os.File, code int) {
	if status == nil {
		return
	}
	_, _ = status.Write([]byte{byte(code)})
	_ = status.Close()
}

// Seams of runDaemonCore: the exit and the four steps whose failure is fatal,
// so a test can fail one without a real daemon. daemonExit is how a startup
// failure reaches launchd: KeepAlive SuccessfulExit false restarts only a
// non-zero exit.
var (
	daemonExit               = os.Exit
	lockWait                 = 2 * time.Second
	cleanupStaleSocket       = CleanupStaleSocket
	bootstrapCapabilityState = (*API).BootstrapCapabilityState
	startRelay               = (*API).StartRelay
	startAPI                 = (*API).Start
)

// runDaemonCore is the blocking core of the daemon. It creates a SessionEngine,
// starts the API (with relay server inside), and blocks until ctx is cancelled.
// Using a context parameter allows it to be driven by either signal handling
// (RunDaemon) or a service manager (daemonSvc).
func runDaemonCore(ctx context.Context) {
	AugmentServicePath() // Must be before NewSessionEngine / any exec.LookPath
	socketPath := DefaultSocketPath()
	// WR-05: the lock comes before any socket cleanup or bind. A daemon that
	// cannot get it within the wait leaves the running one alone.
	lock, err := lockDaemon(filepath.Join(filepath.Dir(socketPath), "daemon.lock"), lockWait)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: already running (lock: %v)\n", err)
		return
	}
	defer lock.Close() // held for the process lifetime
	launchdManaged = ManagedByLaunchd(ctx)
	if err := cleanupStaleSocket(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: %v\n", err)
		return
	}

	engine := NewSessionEngine()
	api := NewAPI(engine)

	// Phase 87 (D-04): bootstrap the capability signing key BEFORE any share
	// server can start. A missing key here is fatal
	// — without it, requireCapability would 401 every request and the daemon
	// is effectively a brick for web access.
	if err := bootstrapCapabilityState(api); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: bootstrap capability state: %v\n", err)
		return
	}

	// Start the relay TCP server inside the daemon.
	relayPort, err := startRelay(api)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: start relay: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "daemon: relay listening on port %d\n", relayPort)

	if err := startAPI(api, socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: start api: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "daemon: listening on %s\n", socketPath)
	api.lock = lock // a handoff sends it to the takeover
	serveDaemon(ctx, api, engine, socketPath)
}

// serveDaemon is the daemon's serve tail once it holds the lock and serves
// its socket and relay: shared by runDaemonCore and the takeover daemon
// (09-05), which receives them in a handoff. It blocks until ctx ends or the
// socket is lost.
//
// SIGTERM guard (Research Pitfall 4): when ctx ends during a handoff it waits
// for the outcome. Handed off (or committed and still unresolved) means exit
// with no api.Stop, no unlink and no session teardown: the takeover serves
// that socket and owns the sessions. Rolled back means today's shutdown.
func serveDaemon(ctx context.Context, api *API, engine *SessionEngine, socketPath string) {
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
		if api.waitHandoff(maxFrozen()) {
			fmt.Fprintf(os.Stderr, "daemon: sessions were handed to the new daemon; exiting without cleanup\n")
			return
		}
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

// RestartQuestion is what a RestartPolicy is asked about a daemon from another
// build that has live sessions. Sessions is -1 when the daemon could not list
// them. MoveFailed is empty unless a session handoff was tried and failed; it
// then holds the short reason (UPGR-03).
type RestartQuestion struct {
	DaemonVersion string
	Sessions      int
	MoveFailed    string
}

// RestartPolicy decides whether EnsureDaemon may restart a daemon from another
// build that has live sessions; the restart ends them (DEBT-08). false keeps
// the old daemon serving.
type RestartPolicy func(RestartQuestion) bool

// EnsureOptions are EnsureDaemon's caller choices. NoHandoff ("use it as it
// is", --keep-daemon) keeps a daemon from another build without moving its
// sessions to this build; the handoff branch (09-05) reads it.
type EnsureOptions struct {
	Restart   RestartPolicy
	NoHandoff bool
	// Start says who is asking; the zero value is the GUI. Every start tries
	// the launchd job first (launchctl kickstart); the CLI also prints a
	// repair sentence before it falls back.
	Start StartMode
	// OnFallback is called once, with the typed agent error, just before a
	// start falls back to the detached spawn: the daemon it starts is not
	// launchd-managed, which callers surface as needs-repair.
	OnFallback func(reason error)
}

// StartMode is the caller of EnsureDaemon.
type StartMode int

const (
	StartGUI StartMode = iota
	StartCLI
)

// executablePath is os.Executable; a var so tests choose the running binary.
var executablePath = os.Executable

// fallbackSpawn starts the daemon detached, outside launchd. Only a typed
// agent error leads here. A test binary never starts one: the child re-runs
// the test binary, outlives the test and holds the user's real socket.
var fallbackSpawn = func(exe string) (<-chan struct{}, func(), error) {
	if testing.Testing() {
		return nil, nil, errors.New("refusing to spawn a daemon from a test binary")
	}
	return startDetachedDaemon(exe)
}

// SetFallbackSpawnForTest replaces the fallback spawn; restore puts it back.
func SetFallbackSpawnForTest(fn func(exe string) (<-chan struct{}, func(), error)) (restore func()) {
	old := fallbackSpawn
	fallbackSpawn = fn
	return func() { fallbackSpawn = old }
}

// EnsureDaemon checks if the daemon is reachable on socketPath. If not, it
// spawns a detached daemon subprocess using the current binary with "daemon"
// as the first argument, then polls until the daemon is ready (waitForDaemon).
// A daemon from another build is restarted, after opts.Restart agrees when it
// has live sessions. When the policy says no, EnsureDaemon returns nil with
// the old daemon still running: the policy's own "no" is the caller's signal.
// A cancelled ctx (Quit All, IN-07) spawns nothing, or kills the fresh daemon
// it spawned, and returns ctx.Err().
//
// UPGR-01: a daemon from another build with live sessions that speaks this
// HandoffVersion first gets a takeover (`portty daemon --takeover`), unless
// opts.NoHandoff. The decision comes only from the takeover's outcome code,
// never from a clock: committed means no question; a failure falls back to
// the DEBT-08 question with the couldn't-move reason (UPGR-03).
func EnsureDaemon(ctx context.Context, socketPath string, opts EnsureOptions) error {
	return ensureDaemon(ctx, socketPath, opts, ensureHooks{
		spawnTakeover: spawnTakeover,
		readyWait:     daemonReadyWait,
		noteAfter:     readyTimeout + committedWatchdog + 5*time.Second,
		stderr:        os.Stderr,
	})
}

// ensureHooks are EnsureDaemon's process seams, passed in (not package vars)
// so the slow-outcome tests can run in parallel.
type ensureHooks struct {
	spawnTakeover func(exe string) (pid int, outcome <-chan int, err error)
	readyWait     time.Duration // for the daemon of this build after a takeover
	noteAfter     time.Duration // when "still waiting for the handoff" is printed (diagnosis only)
	stderr        io.Writer
}

// spawnTakeover starts the takeover process. Like the fresh spawn, a test
// binary never starts one: the child would take the user's real daemon.
func spawnTakeover(exe string) (int, <-chan int, error) {
	if testing.Testing() {
		return 0, nil, errors.New("refusing to spawn a takeover from a test binary")
	}
	return startDetachedTakeover(exe)
}

func ensureDaemon(ctx context.Context, socketPath string, opts EnsureOptions, h ensureHooks) error {
	client := NewDaemonClient(socketPath).WithContext(ctx) // a cancel ends a call to a hung daemon (V-05b)
	needsSpawn := true
	if health, err := client.HealthInfo(); err == nil {
		// Daemon is reachable — check version compatibility first.
		ver, verErr := client.GetDaemonVersion()
		if verErr != nil { // IN-06: a blip is not a mismatch; read once more
			ver, verErr = client.GetDaemonVersion()
		}
		if verErr != nil {
			fmt.Fprintf(h.stderr, "EnsureDaemon: daemon answered but its version could not be read (%v); keeping it\n", verErr)
			return nil
		}
		if StaleVersion(ver) {
			n := liveSessions(client)
			moveFailed := ""
			if n != 0 && !opts.NoHandoff && health.Handoff == HandoffVersion {
				done, reason, err := handOff(ctx, client, h)
				if done || err != nil {
					return err
				}
				moveFailed = reason
			}
			allowRestart := func(ver string, n int) bool {
				return opts.Restart(RestartQuestion{DaemonVersion: ver, Sessions: n, MoveFailed: moveFailed})
			}
			// A count read under a cancelled ctx is -1, not an answer: never ask during Quit All.
			if err := ctx.Err(); err != nil {
				return err
			}
			if n != 0 && !allowRestart(ver, n) {
				fmt.Fprintf(h.stderr, "EnsureDaemon: version mismatch (daemon=%q, binary=%q) with %d live sessions; keeping it\n", ver, BuildVersion, n)
				return nil
			}
			// 08-06-REVIEW WR-02: a session created since the count was not
			// part of the answer, so count again right before /shutdown.
			// shortcut: a session created between this recount and /shutdown
			// still ends unasked (a few ms); an old daemon's /shutdown takes no
			// condition. Upgrade in v1.2's session handoff across an upgrade
			// (REQUIREMENTS.md DEBT-08), e.g. a /shutdown?ifSessions=<n>.
			n2 := liveSessions(client)
			if err := ctx.Err(); err != nil {
				return err
			}
			if countGrew(n, n2) && !allowRestart(ver, n2) {
				fmt.Fprintf(h.stderr, "EnsureDaemon: version mismatch (daemon=%q, binary=%q), live sessions went from %d to %d; keeping it\n", ver, BuildVersion, n, n2)
				return nil
			}
			fmt.Fprintf(h.stderr, "EnsureDaemon: version mismatch (daemon=%q, binary=%q), restarting\n", ver, BuildVersion)
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
			fmt.Fprintf(h.stderr, "EnsureDaemon: stale daemon detected (relay not ready), respawning\n")
		}
	}

	if err := ctx.Err(); err != nil {
		return err // cancelled before a spawn: never spawn
	}
	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("EnsureDaemon: locate binary: %w", err)
	}

	// launchd first: the detached spawn (fallbackSpawn) only when the agent
	// cannot be used. A test binary reaches the kickstart through an injected
	// launchctl recorder; its default refuses.
	return startDaemon(ctx, client, h.readyWait, filepath.Join(filepath.Dir(socketPath), "daemon.lock"),
		func() (<-chan struct{}, func(), error) {
			if agentStartable(ctx, exe) == nil {
				return nil, nil, Kickstart(ctx)
			}
			return fallbackSpawn(exe)
		})
}

// handOff starts a takeover and waits for its outcome code; no clock decides
// (the takeover bounds itself, 09-04). done: the sessions are on this build's
// daemon, ask nothing. Otherwise reason is the couldn't-move reason for the
// DEBT-08 question. err: ctx was cancelled (the takeover is never killed: it
// may own the sessions), or the takeover committed and its daemon did not
// become ready (never the question: the sessions already moved). The lock
// wait of startDaemon is not used: the old daemon holds the lock by design.
func handOff(ctx context.Context, client *DaemonClient, h ensureHooks) (done bool, reason string, err error) {
	code := takeoverFailed
	var pid int
	var outcome <-chan int
	exe, err := os.Executable()
	if err == nil {
		pid, outcome, err = h.spawnTakeover(exe)
	}
	if err != nil {
		fmt.Fprintf(h.stderr, "EnsureDaemon: cannot start the handoff: %v\n", err)
	} else {
		// Only a takeover stopped by a signal never reports (09-04 threat I2);
		// the line is for diagnosis and decides nothing.
		note := time.NewTimer(h.noteAfter)
		defer note.Stop()
	wait:
		for {
			select {
			case code = <-outcome:
				break wait
			case <-note.C:
				fmt.Fprintf(h.stderr, "EnsureDaemon: still waiting for the handoff (pid %d)\n", pid)
			case <-ctx.Done():
				return false, "", ctx.Err()
			}
		}
	}
	switch code {
	case takeoverCommitted:
		if err := waitForDaemon(ctx, client, h.readyWait, nil, ""); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return false, "", ctxErr
			}
			return false, "", fmt.Errorf("EnsureDaemon: the sessions moved to the new daemon, but it is not ready: %w", err)
		}
		return true, "", nil
	case takeoverBusy: // another takeover may be winning
		if err := waitForDaemon(ctx, client, h.readyWait, nil, ""); err == nil {
			return true, "", nil
		} else if ctxErr := ctx.Err(); ctxErr != nil {
			return false, "", ctxErr
		}
	}
	// This build answering means the sessions moved after all; asking would
	// offer to restart the daemon that now owns them.
	if v, err := client.GetDaemonVersion(); err == nil && !StaleVersion(v) {
		return true, "", nil
	}
	fmt.Fprintf(h.stderr, "EnsureDaemon: the handoff failed (outcome %d: %s)\n", code, moveFailedReason(code))
	return false, moveFailedReason(code), nil
}

// startDaemon calls spawn once no other process holds the lock at lockPath,
// then waits for the daemon (waitForDaemon). The lock wait and the spawned
// daemon's readiness wait each get up to wait (08-REVIEW IN-15: a lock freed
// at the deadline never spawns with no time left). 08-REVIEW WR-07: a
// version-mismatched old daemon can take longer to stop than a new daemon's
// 2 s lock wait, so spawning at once lost the lock and then waited out the
// rest of the 15 s. A held lock is never broken: the wait ends early when this
// build's daemon answers or at the deadline. IN-07: both waits end when ctx
// is cancelled; nothing is spawned after that, and a child already spawned
// is killed with spawn's kill func (a fresh daemon owns no sessions yet).
func startDaemon(ctx context.Context, client *DaemonClient, wait time.Duration, lockPath string, spawn func() (exited <-chan struct{}, kill func(), err error)) error {
	start := time.Now()
	deadline := start.Add(wait)
	for lockHeld(lockPath) {
		if daemonReady(client) {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("EnsureDaemon: daemon lock still held after %v", wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	exited, kill, err := spawn()
	if err != nil {
		return fmt.Errorf("EnsureDaemon: start daemon: %w", err)
	}
	if err := waitForDaemon(ctx, client, wait, exited, lockPath); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if kill != nil {
				kill()
			}
			return ctxErr
		}
		return fmt.Errorf("%w (%v waited in total)", err, time.Since(start).Round(time.Second))
	}
	return nil
}

// liveSessions counts the daemon's sessions that have not stopped, or returns
// -1 when it cannot list them (T-08-31: an unknown count is asked about).
func liveSessions(client *DaemonClient) int {
	sessions, err := client.ListSessions()
	if err != nil {
		return -1
	}
	n := 0
	for _, s := range sessions {
		if s.State != "stopped" {
			n++
		}
	}
	return n
}

// countGrew reports whether a live-session count read again (after) needs a
// new answer: it has sessions and is higher, or is unknown (-1) where before
// was known, or known where before was unknown.
func countGrew(before, after int) bool {
	return after != 0 && after != before && (before < 0 || after < 0 || after > before)
}

// StaleVersion reports whether a daemon reporting ver is from another build.
// Production builds (non-"dev") compare versions; a stale daemon from an older
// release won't have newly added API routes (e.g. start-minimized).
func StaleVersion(ver string) bool {
	return BuildVersion != "" && BuildVersion != "dev" && ver != BuildVersion
}

// daemonReady reports whether the daemon at client is fully ready: health,
// this build's version and relay port. 08-REVIEW CR-01: a version-mismatched
// daemon told to stop still answers, and holds the lock, until it exits.
func daemonReady(client *DaemonClient) bool {
	if client.Health() != nil {
		return false
	}
	if ver, err := client.GetDaemonVersion(); err != nil || StaleVersion(ver) {
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
// A cancelled ctx ends the wait with ctx.Err() (IN-07).
func waitForDaemon(ctx context.Context, client *DaemonClient, wait time.Duration, exited <-chan struct{}, lockPath string) error {
	deadline := time.Now().Add(wait)
	exitedSeen := false
	for time.Now().Before(deadline) {
		if daemonReady(client) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
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
