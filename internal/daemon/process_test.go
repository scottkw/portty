package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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

	ver, err := client.GetDaemonVersion()
	if err != nil {
		t.Fatal(err)
	}
	if ver != "v2.1.1" {
		t.Fatalf("expected daemon version %q, got %q", "v2.1.1", ver)
	}
	if ver == BuildVersion {
		t.Fatal("version should differ (stale daemon)")
	}
}

func TestGetDaemonVersion_Empty(t *testing.T) {
	// A client pointing at nothing returns an empty string and the error (IN-06).
	client := NewDaemonClient("/tmp/nonexistent-daemon.sock")
	if ver, err := client.GetDaemonVersion(); ver != "" || err == nil {
		t.Errorf("expected empty version and an error from unreachable daemon, got %q, %v", ver, err)
	}
}

func TestRunDaemon_Exports(t *testing.T) {
	// Verify RunDaemon and EnsureDaemon are callable (compile-time check).
	// We cannot call RunDaemon() in tests (it blocks on signal), but we verify
	// the functions exist and are exported.
	var _ func() = RunDaemon
	var _ func(context.Context, string, EnsureOptions) error = EnsureDaemon
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
			_ = EnsureDaemon(context.Background(), path, EnsureOptions{Restart: func(RestartQuestion) bool { return true }}) // a test binary refuses the spawn
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

	if err := waitForDaemon(context.Background(), NewDaemonClient(path), daemonReadyWait, nil, ""); err != nil {
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
	exited, _, err := startDetachedDaemon(exe)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = waitForDaemon(context.Background(), NewDaemonClient(refusingSocket(t, "dead")), daemonReadyWait, exited, filepath.Join(t.TempDir(), "daemon.lock"))
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
	if err := waitForDaemon(context.Background(), NewDaemonClient(path), daemonReadyWait, exited, lockPath); err != nil {
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
	if err := waitForDaemon(context.Background(), NewDaemonClient(refusingSocket(t, "released")), daemonReadyWait, exited, lockPath); err == nil {
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
	spawn := func() (<-chan struct{}, func(), error) {
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
		return exited, nil, nil
	}
	start := time.Now()
	if err := startDaemon(context.Background(), NewDaemonClient(path), daemonReadyWait, lockPath, spawn); err != nil {
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
	spawn := func() (<-chan struct{}, func(), error) {
		serveDaemonAfter(t, path, 500*time.Millisecond)
		return nil, nil, nil
	}
	if err := startDaemon(context.Background(), NewDaemonClient(path), time.Second, lockPath, spawn); err != nil {
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
	spawn := func() (<-chan struct{}, func(), error) {
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
		return exited, nil, nil
	}
	if err := startDaemon(context.Background(), client, daemonReadyWait, lockPath, spawn); err != nil {
		t.Fatalf("startDaemon: %v", err)
	}
	if !spawned.Load() {
		t.Fatal("the old-version daemon counted as ready: nothing was spawned")
	}
	time.Sleep(200 * time.Millisecond) // well past the old daemon's exit
	if err := client.Health(); err != nil {
		t.Fatalf("no daemon left after the restart: %v", err)
	}
	if ver, _ := client.GetDaemonVersion(); ver != BuildVersion {
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
			moveFailed := ""
			err = EnsureDaemon(context.Background(), path, EnsureOptions{Restart: func(q RestartQuestion) bool {
				asked, askedVer, moveFailed = append(asked, q.Sessions), q.DaemonVersion, moveFailed+q.MoveFailed
				return tc.allow(q.Sessions)
			}})
			if moveFailed != "" { // no handoff was tried: the plain DEBT-08 question
				t.Fatalf("policy got MoveFailed %q, want empty", moveFailed)
			}
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

// 08-06-REVIEW IN-06: a version read that fails right after a good /health is
// unknown, not a mismatch. It is read once more; still unknown, the daemon is
// kept with no question and no /shutdown.
func TestEnsureDaemon_UnreadableVersionIsNotAMismatch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		health    []string // successive /health answers after the first (ok) one; "" answers 500
		wantAsked bool
	}{
		{"still unreadable: keep, no question", []string{"", ""}, false},
		{"retry reads this build: no question", []string{"", "v2"}, false},
		{"retry reads another build: the policy is asked", []string{"", "v1"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := BuildVersion
			BuildVersion = "v2"
			t.Cleanup(func() { BuildVersion = old })
			path := shortProcTestSocket(t, "verread")
			ln, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { ln.Close() })
			var healths atomic.Int64
			var shutdown atomic.Bool
			go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/health":
					ver := "v1"
					if i := int(healths.Add(1)) - 2; i >= 0 {
						ver = tc.health[min(i, len(tc.health)-1)]
					}
					if ver == "" {
						http.Error(w, "boom", http.StatusInternalServerError)
						return
					}
					fmt.Fprintf(w, `{"status":"ok","version":%q}`, ver)
				case "/sessions":
					fmt.Fprint(w, `[{"id":"a","state":"running"}]`)
				case "/relay-port":
					fmt.Fprint(w, `{"port":4242}`)
				case "/shutdown":
					shutdown.Store(true)
				}
			}))
			asked := false
			err = EnsureDaemon(context.Background(), path, EnsureOptions{Restart: func(RestartQuestion) bool { asked = true; return false }})
			if err != nil || asked != tc.wantAsked || shutdown.Load() {
				t.Fatalf("err %v, asked %v, shutdown %v; want nil, %v, false", err, asked, shutdown.Load(), tc.wantAsked)
			}
		})
	}
}

// UPGR-05 (08-06 IN-07): Quit All cancels the ensure. Cancelled before a
// spawn, nothing is spawned: EnsureDaemon returns ctx.Err() ahead of the
// test-binary refusal, and startDaemon never calls its spawn seam.
func TestEnsureDaemon_CancelBeforeSpawn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := EnsureDaemon(ctx, refusingSocket(t, "cancel"), EnsureOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureDaemon with a cancelled ctx = %v, want context.Canceled", err)
	}
	spawns := 0
	spawn := func() (<-chan struct{}, func(), error) { spawns++; return nil, nil, nil }
	err := startDaemon(ctx, NewDaemonClient(refusingSocket(t, "cancel2")), time.Second, filepath.Join(t.TempDir(), "daemon.lock"), spawn)
	if !errors.Is(err, context.Canceled) || spawns != 0 {
		t.Fatalf("startDaemon with a cancelled ctx: err %v, spawns %d; want context.Canceled, 0", err, spawns)
	}
}

// IN-07: the wait for a held lock checks ctx on every tick.
func TestStartDaemon_CancelDuringLockWait(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	holder, err := lockDaemon(lockPath, 0) // another file description holds it
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	ctx, cancel := context.WithCancel(context.Background())
	spawns := 0
	spawn := func() (<-chan struct{}, func(), error) { spawns++; return nil, nil, nil }
	done := make(chan error, 1)
	go func() {
		done <- startDaemon(ctx, NewDaemonClient(refusingSocket(t, "lockwait")), daemonReadyWait, lockPath, spawn)
	}()
	time.Sleep(200 * time.Millisecond)
	cancelled := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || spawns != 0 {
			t.Fatalf("err %v, spawns %d; want context.Canceled, 0", err, spawns)
		}
		if d := time.Since(cancelled); d > 100*time.Millisecond {
			t.Fatalf("returned %v after the cancel, want within 100 ms", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("startDaemon kept waiting for the lock after the cancel")
	}
}

// IN-07 (T-09-01): cancelled while a fresh daemon it spawned starts up, the
// ensure kills that child (it owns no sessions yet), once, and returns ctx.Err().
func TestStartDaemon_CancelAfterSpawnKillsChild(t *testing.T) {
	var kills atomic.Int64
	spawn := func() (<-chan struct{}, func(), error) {
		return make(chan struct{}), func() { kills.Add(1) }, nil // never ready, never exits
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	err := startDaemon(ctx, NewDaemonClient(refusingSocket(t, "kill")), daemonReadyWait, filepath.Join(t.TempDir(), "daemon.lock"), spawn)
	if !errors.Is(err, context.Canceled) || kills.Load() != 1 {
		t.Fatalf("err %v, kills %d; want context.Canceled, 1", err, kills.Load())
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("took %v, want soon after the 200 ms cancel", d)
	}
}

// ---- 09-05: the takeover process and EnsureDaemon's handoff branch ----------

// takeoverHelperEnv makes the test binary a fake `daemon --takeover`
// (TestMain): it reports on the real status pipe with takeoverStatus and
// reportTakeover, opens no socket and reaches no network.
const takeoverHelperEnv = "PORTTY_TEST_TAKEOVER_HELPER"

func helperTakeover(mode string) {
	status := takeoverStatus()
	switch mode {
	case "report0-stay": // committed and serving
		reportTakeover(status, 0)
		time.Sleep(30 * time.Second) // the test kills it long before
		os.Exit(0)
	case "report5-exit5":
		reportTakeover(status, 5)
		os.Exit(5)
	case "exit7":
		os.Exit(7)
	case "sigkill":
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		time.Sleep(time.Minute)
	}
	os.Exit(99)
}

// The outcome channel yields the reported byte, at once when the takeover
// keeps running after it (committed), or the exit code when the process
// dies without reporting (1 when a signal killed it).
func TestStartDetachedTakeover_Outcomes(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		want      int
		stillLive bool
	}{
		{"report0-stay", 0, true},
		{"report5-exit5", 5, false},
		{"exit7", 7, false},
		{"sigkill", 1, false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv(takeoverHelperEnv, tc.mode)
			pid, outcome, err := startDetachedTakeover(os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			if tc.stillLive { // only the helper still running: a reaped pid is never signalled
				t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			}
			select {
			case got := <-outcome:
				if got != tc.want {
					t.Fatalf("outcome %d, want %d", got, tc.want)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("no outcome within 10 s")
			}
			if tc.stillLive {
				if err := syscall.Kill(pid, 0); err != nil {
					t.Fatalf("the committed takeover is gone once it reported: %v", err)
				}
			}
		})
	}
}

// T-09-28: the child reads the status fd from the environment, then unsets
// the variable and marks the fd close-on-exec before anything else, so no
// shell the daemon starts inherits either. One byte, then the pipe closes.
func TestTakeoverStatus_FromEnv(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	fd, err := syscall.Dup(int(w.Fd())) // a plain dup is inheritable, like fd 3 after exec
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	if flags, _ := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); flags&unix.FD_CLOEXEC != 0 {
		t.Fatal("the dup is already close-on-exec; the test proves nothing")
	}
	t.Setenv(takeoverStatusEnv, fmt.Sprint(fd))

	status := takeoverStatus()
	if status == nil {
		t.Fatal("takeoverStatus found no status pipe")
	}
	if _, ok := os.LookupEnv(takeoverStatusEnv); ok {
		t.Fatalf("%s is still in the environment", takeoverStatusEnv)
	}
	if flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("status fd is not close-on-exec (flags %#x, err %v)", flags, err)
	}
	reportTakeover(status, 5)
	got, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(got, []byte{5}) {
		t.Fatalf("pipe carried %v (err %v), want exactly [5] then EOF", got, err)
	}

	os.Unsetenv(takeoverStatusEnv) // run by hand: nothing to report to
	if s := takeoverStatus(); s != nil {
		t.Fatal("takeoverStatus without the variable returned a pipe")
	}
	reportTakeover(nil, 0) // must not panic
}

// fakeDaemon is an old daemon on a unix socket: /health reports ver (api 2
// and handoff when not 0), /sessions answers sessions, /relay-port 4242, and
// /shutdown is recorded.
type fakeDaemon struct {
	path     string
	ver      atomic.Pointer[string]
	shutdown atomic.Bool
}

func (f *fakeDaemon) setVersion(v string) { f.ver.Store(&v) }

func startFakeDaemon(t *testing.T, handoff int, sessions string) *fakeDaemon {
	t.Helper()
	f := &fakeDaemon{path: shortProcTestSocket(t, "ho")}
	f.setVersion("v1")
	ln, err := net.Listen("unix", f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Version: *f.ver.Load(), API: 2, Handoff: handoff})
		case "/sessions":
			fmt.Fprint(w, sessions)
		case "/relay-port":
			fmt.Fprint(w, `{"port":4242}`)
		case "/shutdown":
			f.shutdown.Store(true)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	return f
}

const twoLive = `[{"id":"a","state":"running"},{"id":"b","state":"running"}]`

// takeoverAfter is a fake spawnTakeover: after d it switches the fake daemon
// to ver (unless "") and yields code. spawns counts the calls.
func (f *fakeDaemon) takeoverAfter(d time.Duration, code int, ver string, spawns *atomic.Int64) func(string) (int, <-chan int, error) {
	return func(string) (int, <-chan int, error) {
		spawns.Add(1)
		out := make(chan int, 1)
		time.AfterFunc(d, func() {
			if ver != "" {
				f.setVersion(ver)
			}
			out <- code
		})
		return 4242, out, nil
	}
}

func testHooks(spawn func(string) (int, <-chan int, error)) ensureHooks {
	return ensureHooks{spawnTakeover: spawn, readyWait: daemonReadyWait, noteAfter: time.Hour, stderr: io.Discard}
}

// recordPolicy returns a policy answering allow and the questions it got.
func recordPolicy(allow bool) (RestartPolicy, *[]RestartQuestion) {
	var asked []RestartQuestion
	return func(q RestartQuestion) bool { asked = append(asked, q); return allow }, &asked
}

func useBuild(t *testing.T, v string) {
	old := BuildVersion
	BuildVersion = v
	t.Cleanup(func() { BuildVersion = old })
}

// T-D8 (UPGR-01): a daemon from another build with live sessions that speaks
// this handoff version gets a takeover; once it reports committed and this
// build answers, EnsureDaemon returns nil with no question and no /shutdown.
func TestEnsureDaemon_HandsOff(t *testing.T) {
	useBuild(t, "v2")
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	var spawns atomic.Int64
	policy, asked := recordPolicy(false)
	err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(f.takeoverAfter(100*time.Millisecond, 0, "v2", &spawns)))
	if err != nil || spawns.Load() != 1 || len(*asked) != 0 || f.shutdown.Load() {
		t.Fatalf("err %v, spawns %d, asked %v, shutdown %v; want nil, 1, none, false", err, spawns.Load(), *asked, f.shutdown.Load())
	}
}

// UPGR-03: a takeover that fails falls back to the DEBT-08 question with the
// couldn't-move reason for its code, and the old daemon is then kept (no) or
// restarted (yes) exactly as before.
func TestEnsureDaemon_HandoffFailsToDebt08(t *testing.T) {
	const spawnError = -1
	for _, code := range []int{takeoverRefused, takeoverImport, takeoverTimedOut, takeoverFailed, spawnError} {
		for _, allow := range []bool{false, true} {
			t.Run(fmt.Sprintf("code %d, restart %v", code, allow), func(t *testing.T) {
				useBuild(t, "v2")
				f := startFakeDaemon(t, HandoffVersion, twoLive)
				var spawns atomic.Int64
				spawn, want := f.takeoverAfter(50*time.Millisecond, code, "", &spawns), moveFailedReason(code)
				if code == spawnError {
					spawn = func(string) (int, <-chan int, error) { return 0, nil, errors.New("exec failed") }
					want = "the new version exited"
				}
				policy, asked := recordPolicy(allow)
				err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(spawn))
				if len(*asked) != 1 || (*asked)[0] != (RestartQuestion{DaemonVersion: "v1", Sessions: 2, MoveFailed: want}) {
					t.Fatalf("policy asked %+v, want one question with MoveFailed %q", *asked, want)
				}
				if f.shutdown.Load() != allow {
					t.Fatalf("shutdown sent = %v, want %v", f.shutdown.Load(), allow)
				}
				if allow && err == nil { // the test binary refuses the respawn after the shutdown
					t.Fatal("the restart path returned nil")
				}
				if !allow && err != nil {
					t.Fatalf("keeping the old daemon must succeed: %v", err)
				}
			})
		}
	}
}

// Busy (3): another takeover may be winning. This build answering within the
// ready wait means no question; still the old build, the busy reason.
func TestEnsureDaemon_BusyThenReady(t *testing.T) {
	useBuild(t, "v2")
	t.Run("the other takeover wins", func(t *testing.T) {
		f := startFakeDaemon(t, HandoffVersion, twoLive)
		var spawns atomic.Int64
		time.AfterFunc(300*time.Millisecond, func() { f.setVersion("v2") })
		policy, asked := recordPolicy(false)
		err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(f.takeoverAfter(0, takeoverBusy, "", &spawns)))
		if err != nil || len(*asked) != 0 || f.shutdown.Load() {
			t.Fatalf("err %v, asked %+v, shutdown %v; want nil, none, false", err, *asked, f.shutdown.Load())
		}
	})
	t.Run("the old daemon stays", func(t *testing.T) {
		f := startFakeDaemon(t, HandoffVersion, twoLive)
		var spawns atomic.Int64
		h := testHooks(f.takeoverAfter(0, takeoverBusy, "", &spawns))
		h.readyWait = 500 * time.Millisecond
		policy, asked := recordPolicy(false)
		err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, h)
		if err != nil || len(*asked) != 1 || (*asked)[0].MoveFailed != moveFailedReason(takeoverBusy) {
			t.Fatalf("err %v, asked %+v; want nil and the busy reason", err, *asked)
		}
	})
}

// A failure code with this build answering afterwards is a success after all.
func TestEnsureDaemon_HandoffFailedButNowCurrent(t *testing.T) {
	useBuild(t, "v2")
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	var spawns atomic.Int64
	policy, asked := recordPolicy(false)
	err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(f.takeoverAfter(50*time.Millisecond, takeoverFailed, "v2", &spawns)))
	if err != nil || len(*asked) != 0 || f.shutdown.Load() {
		t.Fatalf("err %v, asked %+v, shutdown %v; want nil, none, false", err, *asked, f.shutdown.Load())
	}
}

// slowBuild sets this build for the two 12 s tests before t.Parallel().
// shortcut: never restored (a restore in one parallel test's cleanup would
// race the other's reads); parallel tests run after every sequential test and
// these two are the package's only ones. Upgrade if another parallel test
// needs a different BuildVersion.
func slowBuild(t *testing.T) {
	BuildVersion = "v2"
	t.Parallel()
}

// T-09-29 (plan check B2): a takeover that commits after 12 s gets no
// question and no /shutdown; no clock decides. The diagnosis line is printed
// once, however long the wait.
func TestEnsureDaemon_SlowCommitNoQuestion(t *testing.T) {
	slowBuild(t)
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	var spawns atomic.Int64
	var stderr bytes.Buffer
	h := testHooks(f.takeoverAfter(12*time.Second, takeoverCommitted, "v2", &spawns))
	h.noteAfter, h.stderr = time.Second, &stderr
	policy, asked := recordPolicy(true)
	err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, h)
	if err != nil || len(*asked) != 0 || f.shutdown.Load() {
		t.Fatalf("err %v, asked %+v, shutdown %v; want nil, none, false", err, *asked, f.shutdown.Load())
	}
	if n := strings.Count(stderr.String(), "EnsureDaemon: still waiting for the handoff (pid 4242)\n"); n != 1 {
		t.Fatalf("still-waiting line printed %d times, want 1; stderr:\n%s", n, stderr.String())
	}
}

// T-09-29: a failure that arrives after 12 s is asked about only then.
func TestEnsureDaemon_NoWallClockDecision(t *testing.T) {
	slowBuild(t)
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	var spawns atomic.Int64
	var spawnedAt, askedAt time.Time
	slow := f.takeoverAfter(12*time.Second, takeoverImport, "", &spawns)
	spawn := func(exe string) (int, <-chan int, error) { spawnedAt = time.Now(); return slow(exe) }
	var asked []RestartQuestion
	policy := func(q RestartQuestion) bool { askedAt = time.Now(); asked = append(asked, q); return false }
	if err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(spawn)); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0].MoveFailed != moveFailedReason(takeoverImport) {
		t.Fatalf("asked %+v, want one question with the import reason", asked)
	}
	if d := askedAt.Sub(spawnedAt); d < 12*time.Second {
		t.Fatalf("policy asked %v after the spawn, before the outcome (12 s)", d)
	}
}

// UPGR-03: a v1.1 daemon (no handoff field) or another handoff version gets
// no takeover and the plain DEBT-08 question.
func TestEnsureDaemon_V11GetsPlainDebt08(t *testing.T) {
	for _, handoff := range []int{0, HandoffVersion + 1} {
		t.Run(fmt.Sprintf("handoff %d", handoff), func(t *testing.T) {
			useBuild(t, "v2")
			f := startFakeDaemon(t, handoff, twoLive)
			var spawns atomic.Int64
			policy, asked := recordPolicy(false)
			err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(f.takeoverAfter(0, 0, "v2", &spawns)))
			if err != nil || spawns.Load() != 0 || len(*asked) != 1 || (*asked)[0].MoveFailed != "" {
				t.Fatalf("err %v, spawns %d, asked %+v; want nil, 0, one plain question", err, spawns.Load(), *asked)
			}
		})
	}
}

// Research A9: no live sessions keeps today's plain restart, no takeover.
func TestEnsureDaemon_ZeroSessionsPlainRestart(t *testing.T) {
	useBuild(t, "v2")
	f := startFakeDaemon(t, HandoffVersion, `[]`)
	var spawns atomic.Int64
	policy, asked := recordPolicy(false)
	err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}, testHooks(f.takeoverAfter(0, 0, "v2", &spawns)))
	if spawns.Load() != 0 || len(*asked) != 0 || !f.shutdown.Load() || err == nil {
		t.Fatalf("spawns %d, asked %+v, shutdown %v, err %v; want 0, none, true, the respawn refusal", spawns.Load(), *asked, f.shutdown.Load(), err)
	}
}

// --keep-daemon (NoHandoff) never hands off.
func TestEnsureDaemon_NoHandoffForKeep(t *testing.T) {
	useBuild(t, "v2")
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	var spawns atomic.Int64
	policy, asked := recordPolicy(false)
	err := ensureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy, NoHandoff: true}, testHooks(f.takeoverAfter(0, 0, "v2", &spawns)))
	if err != nil || spawns.Load() != 0 || len(*asked) != 1 || (*asked)[0].MoveFailed != "" {
		t.Fatalf("err %v, spawns %d, asked %+v; want nil, 0, one plain question", err, spawns.Load(), *asked)
	}
}

// T-09-27: Quit All (ctx cancel) while the outcome is pending returns at once
// and leaves the takeover running; there is no kill path. A real process
// stands in for the takeover so "still running" is checked, not assumed.
func TestEnsureDaemon_CancelNeverKillsTakeover(t *testing.T) {
	useBuild(t, "v2")
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	var pid int
	spawn := func(string) (int, <-chan int, error) {
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			return 0, nil, err
		}
		pid = cmd.Process.Pid
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return pid, make(chan int), nil // never reports
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelledAt := make(chan time.Time, 1)
	time.AfterFunc(200*time.Millisecond, func() { cancelledAt <- time.Now(); cancel() })
	policy, asked := recordPolicy(true)
	err := ensureDaemon(ctx, f.path, EnsureOptions{Restart: policy}, testHooks(spawn))
	if d := time.Since(<-cancelledAt); !errors.Is(err, context.Canceled) || d > 100*time.Millisecond {
		t.Fatalf("err %v %v after the cancel; want context.Canceled within 100 ms", err, d)
	}
	if len(*asked) != 0 || f.shutdown.Load() {
		t.Fatalf("asked %+v, shutdown %v after a cancel; want none, false", *asked, f.shutdown.Load())
	}
	if pid == 0 {
		t.Fatal("the takeover seam was never called")
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("the takeover is gone after the cancel: %v", err)
	}
}

// T-09-30: the real spawn refuses from a test binary, so EnsureDaemon falls
// back to the question with the spawn-error reason.
func TestEnsureDaemon_TakeoverRefusedFromTestBinary(t *testing.T) {
	if _, _, err := spawnTakeover(os.Args[0]); err == nil || !strings.Contains(err.Error(), "test binary") {
		t.Fatalf("spawnTakeover from a test binary = %v, want the refusal", err)
	}
	useBuild(t, "v2")
	f := startFakeDaemon(t, HandoffVersion, twoLive)
	policy, asked := recordPolicy(false)
	if err := EnsureDaemon(context.Background(), f.path, EnsureOptions{Restart: policy}); err != nil {
		t.Fatal(err)
	}
	if len(*asked) != 1 || (*asked)[0].MoveFailed != "the new version exited" {
		t.Fatalf("asked %+v, want one question with the spawn-error reason", *asked)
	}
}
