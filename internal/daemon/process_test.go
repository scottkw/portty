package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

var procTestSeq atomic.Int64

func shortProcTestSocket(t *testing.T, name string) string {
	t.Helper()
	seq := procTestSeq.Add(1)
	path := fmt.Sprintf("/tmp/dptest%d_%s.sock", seq, name)
	t.Cleanup(func() { os.Remove(path) })
	return path
}

func TestEnsureDaemon_AlreadyRunning(t *testing.T) {
	// Start an in-process daemon API to simulate a running daemon.
	socketPath := shortProcTestSocket(t, "already")
	engine := NewSessionEngine()
	engine.configDir = t.TempDir()
	api := NewAPI(engine)
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	defer api.Stop()
	time.Sleep(10 * time.Millisecond)

	// EnsureDaemon should return nil immediately (daemon reachable).
	client := NewDaemonClient(socketPath)
	if err := client.Health(); err != nil {
		t.Fatalf("Health check before EnsureDaemon: %v", err)
	}
}

func TestEnsureDaemon_Timeout(t *testing.T) {
	// Use a socket path where nothing is listening.
	socketPath := shortProcTestSocket(t, "timeout")
	client := NewDaemonClient(socketPath)
	// Health should fail since nothing is listening.
	if err := client.Health(); err == nil {
		t.Fatal("expected Health to fail on empty socket path")
	}
}

func TestEnsureDaemon_VersionMismatchDetected(t *testing.T) {
	// Simulate an old daemon by using an httptest server that returns
	// a stale version in the health response. This can't be tested with
	// an in-process daemon because handler and caller share BuildVersion.
	old := BuildVersion
	BuildVersion = "v2.1.2"
	t.Cleanup(func() { BuildVersion = old })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			// Simulate old daemon reporting v2.1.1
			fmt.Fprint(w, `{"status":"ok","version":"v2.1.1"}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	client := &DaemonClient{
		http: srv.Client(),
		base: srv.URL,
	}

	ver := client.GetDaemonVersion()
	if ver != "v2.1.1" {
		t.Fatalf("expected daemon version %q, got %q", "v2.1.1", ver)
	}
	if ver == BuildVersion {
		t.Fatal("version should differ (stale daemon)")
	}
}

func TestGetDaemonVersion_Empty(t *testing.T) {
	// A client pointing at nothing should return empty string.
	client := NewDaemonClient("/tmp/nonexistent-daemon.sock")
	if ver := client.GetDaemonVersion(); ver != "" {
		t.Errorf("expected empty version from unreachable daemon, got %q", ver)
	}
}

func TestRunDaemon_Exports(t *testing.T) {
	// Verify RunDaemon and EnsureDaemon are callable (compile-time check).
	// We cannot call RunDaemon() in tests (it blocks on signal), but we verify
	// the functions exist and are exported.
	var _ func() = RunDaemon
	var _ func(string, RestartPolicy) error = EnsureDaemon
}

// WR-05: one daemon at a time. A second daemon cannot take the lock while the
// first holds it, and gets it once the first lets go within the wait (the
// version-mismatch restart: the old daemon exits just after its shutdown reply).
func TestLockDaemon_ExclusiveAndWaits(t *testing.T) {
	path := t.TempDir() + "/daemon.lock"
	first, err := lockDaemon(path, 0)
	if err != nil {
		t.Fatalf("first lockDaemon: %v", err)
	}
	if second, err := lockDaemon(path, 100*time.Millisecond); err == nil {
		second.Close()
		t.Fatal("second lockDaemon succeeded while the first holds the lock")
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		first.Close() // what the kernel does when the old daemon exits
	}()
	third, err := lockDaemon(path, 2*time.Second)
	if err != nil {
		t.Fatalf("lockDaemon after the holder let go: %v", err)
	}
	third.Close()
}

// Review it3 WR-03: EnsureDaemon (GUI, CLI) never unlinks the socket path. Its
// dial-then-remove could take a daemon's live socket that was bound between
// the two steps; the spawned daemon cleans up under the lock instead. The fake
// daemon here stops listening (leaving its file) just before EnsureDaemon
// would have cleaned up, so the file looks stale.
func TestEnsureDaemon_NeverUnlinksSocket(t *testing.T) {
	for _, tc := range []struct {
		name, build, closeOn string
	}{
		{"version mismatch", "v2", "/shutdown"},
		{"relay not ready", "dev", "/relay-port"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := BuildVersion
			BuildVersion = tc.build
			t.Cleanup(func() { BuildVersion = old })
			path := shortProcTestSocket(t, "nounlink")
			ln, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			ln.(*net.UnixListener).SetUnlinkOnClose(false)
			t.Cleanup(func() { ln.Close() })
			go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
				if r.URL.Path == tc.closeOn {
					ln.Close()
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/health":
					fmt.Fprint(w, `{"status":"ok","version":"v1"}`)
				case "/relay-port":
					fmt.Fprint(w, `{"port":0}`)
				default:
					fmt.Fprint(w, `{}`)
				}
			}))
			_ = EnsureDaemon(path, func(string, int) bool { return true }) // a test binary refuses the spawn
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("EnsureDaemon removed the socket path: %v", err)
			}
		})
	}
}

// DEBT-06: at login a spawned daemon took up to 10 s to bind (08-VM-CHECKS
// B-07), so EnsureDaemon's 3 s wait failed startup, the launch grace did not
// apply, and the poller spawned a second daemon. Until the daemon binds, the
// previous run's socket file answers "connection refused", as captured.
func TestWaitForDaemon_SlowStart(t *testing.T) {
	path := shortProcTestSocket(t, "slow")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	stale.Close() // the file stays: dials now get "connection refused"

	served := make(chan net.Listener, 1)
	go func() {
		time.Sleep(4 * time.Second) // slower than the old 3 s wait
		os.Remove(path)
		ln, err := net.Listen("unix", path)
		if err != nil {
			close(served)
			return
		}
		served <- ln
		http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/relay-port" {
				fmt.Fprint(w, `{"port":4242}`)
				return
			}
			fmt.Fprint(w, `{"status":"ok"}`)
		}))
	}()
	t.Cleanup(func() {
		if ln, ok := <-served; ok {
			ln.Close()
		}
	})

	if err := waitForDaemon(NewDaemonClient(path), daemonReadyWait, nil, ""); err != nil {
		t.Fatalf("a daemon that binds after 4 s must count as started: %v", err)
	}
}

// refusingSocket returns a socket path whose file exists but refuses
// connections, like the previous run's socket before a daemon binds.
func refusingSocket(t *testing.T, name string) string {
	t.Helper()
	path := shortProcTestSocket(t, name)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	return path
}

// serveDaemonAfter binds a fake ready daemon of this build's version at path
// after d.
func serveDaemonAfter(t *testing.T, path string, d time.Duration) {
	t.Helper()
	ver := BuildVersion // read here: the handler goroutine may outlive a test's restore
	served := make(chan net.Listener, 1)
	go func() {
		time.Sleep(d)
		os.Remove(path)
		ln, err := net.Listen("unix", path)
		if err != nil {
			close(served)
			return
		}
		served <- ln
		http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/relay-port" {
				fmt.Fprint(w, `{"port":4242}`)
				return
			}
			fmt.Fprintf(w, `{"status":"ok","version":%q}`, ver)
		}))
	}()
	t.Cleanup(func() {
		if ln, ok := <-served; ok {
			ln.Close()
		}
	})
}

// 08-REVIEW WR-04: a spawned daemon that dies at once (a crash at init, a bad
// config) must fail EnsureDaemon in well under daemonReadyWait, since the wait
// gates startup's error UI, Retry, Quit All and every CLI command. The real
// spawn path runs `false`, which also checks the child is reaped (exited closes).
func TestWaitForDaemon_DiedAtSpawnFailsFast(t *testing.T) {
	exe, err := exec.LookPath("false")
	if err != nil {
		t.Skip("no false binary")
	}
	exited, err := startDetachedDaemon(exe)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = waitForDaemon(NewDaemonClient(refusingSocket(t, "dead")), daemonReadyWait, exited, filepath.Join(t.TempDir(), "daemon.lock"))
	if err == nil {
		t.Fatal("a daemon that exited at spawn counted as started")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v to report a dead daemon, want well under %v", d, daemonReadyWait)
	}
}

// WR-04: a spawned daemon that exits because another daemon holds the lock
// (a concurrent EnsureDaemon's, still starting) is no failure: wait for the
// lock holder, which binds later.
func TestWaitForDaemon_LostLockWaitsForHolder(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	holder, err := lockDaemon(lockPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	path := refusingSocket(t, "holder")
	serveDaemonAfter(t, path, time.Second)
	exited := make(chan struct{})
	close(exited)
	if err := waitForDaemon(NewDaemonClient(path), daemonReadyWait, exited, lockPath); err != nil {
		t.Fatalf("the lock holder bound after 1 s, but: %v", err)
	}
}

// 08-REVIEW WR-07: a lock holder that lets go without binding (a crash before
// bind) fails the wait then, not at the deadline: the lock is checked on every
// poll, not once.
func TestWaitForDaemon_LockReleasedWithoutBindFailsFast(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	holder, err := lockDaemon(lockPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(200*time.Millisecond, func() { holder.Close() })
	exited := make(chan struct{})
	close(exited)
	start := time.Now()
	if err := waitForDaemon(NewDaemonClient(refusingSocket(t, "released")), daemonReadyWait, exited, lockPath); err == nil {
		t.Fatal("nothing bound, but the wait succeeded")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("took %v after the lock was released at 200 ms, want well under 1 s", d)
	}
}

// 08-REVIEW WR-07: the version-mismatch restart. The old daemon holds the
// lock past the new daemon's 2 s lock wait (killing PTYs is slow), then lets
// go. startDaemon spawns only once the lock is free, so the new daemon gets
// it and starts, and the call returns soon after the release, not at 15 s.
// The fake spawn does what runDaemonCore does: a 2 s lockDaemon, then bind.
func TestStartDaemon_WaitsForOldDaemonLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	old, err := lockDaemon(lockPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(2500*time.Millisecond, func() { old.Close() })
	path := refusingSocket(t, "restart")
	spawn := func() (<-chan struct{}, error) {
		exited := make(chan struct{})
		go func() {
			lock, err := lockDaemon(lockPath, 2*time.Second)
			if err != nil {
				close(exited) // "daemon: already running"
				return
			}
			t.Cleanup(func() { lock.Close() })
			serveDaemonAfter(t, path, 0)
		}()
		return exited, nil
	}
	start := time.Now()
	if err := startDaemon(NewDaemonClient(path), daemonReadyWait, lockPath, spawn); err != nil {
		t.Fatalf("the old daemon let go at 2.5 s, but: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("took %v, want soon after the 2.5 s release", d)
	}
}

// 08-REVIEW IN-15: a lock freed near the deadline still gives the spawned
// daemon its own readiness window. Here the lock frees at 0.8 s of a 1 s
// wait and the daemon binds 0.5 s after the spawn; with only the remaining
// time it timed out.
func TestStartDaemon_LateLockReleaseKeepsSpawnWindow(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	old, err := lockDaemon(lockPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(800*time.Millisecond, func() { old.Close() })
	path := refusingSocket(t, "late")
	spawn := func() (<-chan struct{}, error) {
		serveDaemonAfter(t, path, 500*time.Millisecond)
		return nil, nil
	}
	if err := startDaemon(NewDaemonClient(path), time.Second, lockPath, spawn); err != nil {
		t.Fatalf("the daemon bound 0.5 s after the spawn, but: %v", err)
	}
}

// 08-REVIEW CR-01: after ShutdownDaemon's 204 the old daemon keeps answering,
// and holds the lock, for 50 ms before its os.Exit (handleShutdown). The lock
// wait must not count that old-version daemon as ready, or nothing is spawned
// and no daemon is left once it exits.
func TestStartDaemon_MismatchedDaemonAnsweringIsNotReady(t *testing.T) {
	old := BuildVersion
	BuildVersion = "v2"
	t.Cleanup(func() { BuildVersion = old })
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	oldLock, err := lockDaemon(lockPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	path := shortProcTestSocket(t, "mismatch")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/shutdown": // like handleShutdown: reply, then exit 50 ms later
			w.WriteHeader(http.StatusNoContent)
			w.(http.Flusher).Flush()
			time.AfterFunc(50*time.Millisecond, func() { srv.Close(); oldLock.Close() })
		case "/relay-port":
			fmt.Fprint(w, `{"port":4242}`)
		default:
			fmt.Fprint(w, `{"status":"ok","version":"v1"}`)
		}
	})
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Close() })

	client := NewDaemonClient(path)
	if err := client.ShutdownDaemon(); err != nil {
		t.Fatal(err)
	}
	var spawned atomic.Bool
	spawn := func() (<-chan struct{}, error) {
		spawned.Store(true)
		exited := make(chan struct{})
		go func() {
			lock, err := lockDaemon(lockPath, 2*time.Second)
			if err != nil {
				close(exited)
				return
			}
			t.Cleanup(func() { lock.Close() })
			serveDaemonAfter(t, path, 0)
		}()
		return exited, nil
	}
	if err := startDaemon(client, daemonReadyWait, lockPath, spawn); err != nil {
		t.Fatalf("startDaemon: %v", err)
	}
	if !spawned.Load() {
		t.Fatal("the old-version daemon counted as ready: nothing was spawned")
	}
	time.Sleep(200 * time.Millisecond) // well past the old daemon's exit
	if err := client.Health(); err != nil {
		t.Fatalf("no daemon left after the restart: %v", err)
	}
	if ver := client.GetDaemonVersion(); ver != BuildVersion {
		t.Fatalf("daemon version %q, want %q", ver, BuildVersion)
	}
}

// Review it3 WR-03: a daemon whose socket path is removed or rebound can no
// longer be reached, yet it holds the lock, so no replacement could start.
// socketLost spots that so the daemon exits and frees the lock.
func TestSocketLost(t *testing.T) {
	bind := func(t *testing.T) (string, os.FileInfo) {
		path := shortProcTestSocket(t, "lost")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		ln.(*net.UnixListener).SetUnlinkOnClose(false)
		t.Cleanup(func() { ln.Close() })
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return path, fi
	}
	watch := func(path string, fi os.FileInfo) chan bool {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		t.Cleanup(cancel)
		lost := make(chan bool, 1)
		go func() { lost <- socketLost(ctx, path, fi, 10*time.Millisecond) }()
		return lost
	}
	t.Run("kept", func(t *testing.T) {
		path, fi := bind(t)
		if <-watch(path, fi) {
			t.Fatal("socketLost reported an untouched socket as lost")
		}
	})
	t.Run("removed", func(t *testing.T) {
		path, fi := bind(t)
		lost := watch(path, fi)
		os.Remove(path)
		if !<-lost {
			t.Fatal("socketLost missed a removed socket")
		}
	})
	t.Run("replaced", func(t *testing.T) {
		path, fi := bind(t)
		lost := watch(path, fi)
		os.Remove(path)
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		if !<-lost {
			t.Fatal("socketLost missed a socket rebound by someone else")
		}
	})
}

// DEBT-08: a version-mismatch restart ends every session, so with live
// sessions EnsureDaemon asks the caller's policy first, and a "no" keeps the
// old daemon serving. With none it restarts at once (the CR-01 path). A count
// it cannot get is unknown (-1) and is asked about, never killed blindly.
// 08-06-REVIEW WR-02: the count is read again right before /shutdown, and one
// that grew (or turned unknown) goes back to the policy.
func TestEnsureDaemon_StaleRestartConsent(t *testing.T) {
	running := `[{"id":"a","state":"running"},{"id":"b","state":"running"},{"id":"c","state":"stopped"}]`
	one := `[{"id":"a","state":"running"}]`
	three := `[{"id":"a","state":"running"},{"id":"b","state":"running"},{"id":"d","state":"running"}]`
	yes := func(int) bool { return true }
	no := func(int) bool { return false }
	upTo2 := func(n int) bool { return n > 0 && n <= 2 }
	for _, tc := range []struct {
		name         string
		sessions     []string // successive /sessions bodies (the last repeats); "" answers 500
		allow        func(int) bool
		wantAsked    []int // the session counts passed to the policy
		wantShutdown bool
	}{
		{"live sessions, keep", []string{running}, no, []int{2}, false},
		{"live sessions, restart", []string{running}, yes, []int{2}, true},
		{"only stopped sessions restart without asking", []string{`[{"id":"c","state":"stopped"}]`}, no, nil, true},
		{"no sessions restart without asking", []string{`[]`}, no, nil, true},
		{"unknown count asks", []string{""}, no, []int{-1}, false},
		{"a session created before shutdown is asked about", []string{`[]`, one}, no, []int{1}, false},
		{"a count that grew after a yes is asked again", []string{running, three}, upTo2, []int{2, 3}, false},
		{"a count that grew is restarted on a second yes", []string{running, three}, yes, []int{2, 3}, true},
		{"a count that turned unknown is asked again", []string{running, ""}, upTo2, []int{2, -1}, false},
		{"fewer sessions on the recount need no new answer", []string{running, one}, upTo2, []int{2}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := BuildVersion
			BuildVersion = "v2"
			t.Cleanup(func() { BuildVersion = old })
			path := shortProcTestSocket(t, "consent")
			ln, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { ln.Close() })
			var shutdown atomic.Bool
			var lists atomic.Int64
			go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/health":
					fmt.Fprint(w, `{"status":"ok","version":"v1"}`)
				case "/sessions":
					body := tc.sessions[min(int(lists.Add(1))-1, len(tc.sessions)-1)]
					if body == "" {
						http.Error(w, "boom", http.StatusInternalServerError)
						return
					}
					fmt.Fprint(w, body)
				case "/relay-port":
					fmt.Fprint(w, `{"port":4242}`)
				case "/shutdown":
					shutdown.Store(true)
					w.WriteHeader(http.StatusNoContent)
				}
			}))

			var asked []int
			askedVer := ""
			err = EnsureDaemon(path, func(ver string, n int) bool {
				asked, askedVer = append(asked, n), ver
				return tc.allow(n)
			})
			if !slices.Equal(asked, tc.wantAsked) {
				t.Fatalf("policy asked with %v sessions, want %v", asked, tc.wantAsked)
			}
			if len(asked) != 0 && askedVer != "v1" {
				t.Fatalf("policy got daemon version %q, want v1", askedVer)
			}
			if shutdown.Load() != tc.wantShutdown {
				t.Fatalf("shutdown sent = %v, want %v", shutdown.Load(), tc.wantShutdown)
			}
			if tc.wantShutdown {
				if err == nil { // the test binary refuses the respawn that follows the shutdown
					t.Fatal("the restart path returned nil")
				}
			} else if err != nil {
				t.Fatalf("keeping the old daemon must succeed: %v", err)
			}
		})
	}
}
