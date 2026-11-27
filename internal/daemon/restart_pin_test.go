package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// swapDaemon is a socket served first by an old-build daemon and then, once a
// late re-home commits, by this build's daemon (the launchd job's), the way a
// real handoff hands the listener over.
type swapDaemon struct {
	path        string
	oldShutdown atomic.Bool // the old daemon's /shutdown was accepted (204)
	newShutdown atomic.Bool // this build's daemon got /shutdown: every moved session would end
	frozen      atomic.Bool // the old daemon answers writes 503 Retry-After (09-04 freeze)
	once        sync.Once
	mu          sync.Mutex
	srv         *http.Server
}

func (s *swapDaemon) serve(t *testing.T, ver string, handler func(w http.ResponseWriter, r *http.Request)) {
	t.Helper()
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Version: ver, API: 2, Handoff: HandoffVersion})
		case "/sessions":
			fmt.Fprint(w, twoLive)
		case "/relay-port":
			fmt.Fprint(w, `{"port":4242}`)
		default:
			handler(w, r)
		}
	})}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()
	go srv.Serve(ln) //nolint:errcheck
}

func (s *swapDaemon) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		_ = s.srv.Close()
	}
}

// commit is the late re-home committing: the old daemon drops every
// connection and exits, and this build's daemon serves the same socket.
func (s *swapDaemon) commit(t *testing.T) {
	s.close()
	_ = os.Remove(s.path)
	s.serve(t, "v2", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shutdown" {
			s.newShutdown.Store(true)
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

// A CLI restart (--restart-daemon) after launchdTakeover gave up at its
// deadline must never stop the daemon a late re-home moved the sessions to.
// The launchd job's daemon froze the old one just after the CLI stopped
// waiting; the restart's /shutdown meets the freeze (503), and the re-home
// commits while it waits. Before the fix the retry, on a fresh connection,
// reached this build's daemon and ended every session.
func TestEnsureDaemon_RestartNeverStopsLateRehome(t *testing.T) {
	useBuild(t, "v2")
	t.Setenv("HOME", t.TempDir())
	s := &swapDaemon{path: shortProcTestSocket(t, "pin")}
	t.Cleanup(s.close)
	s.serve(t, "v1", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shutdown" {
			return
		}
		if s.frozen.Load() {
			s.once.Do(func() { time.AfterFunc(100*time.Millisecond, func() { s.commit(t) }) })
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		s.oldShutdown.Store(true)
		w.WriteHeader(http.StatusNoContent)
	})
	f := newFakeLaunchd(t)
	f.setJob(LaunchdLabel, 0)
	// The kickstart reaches a job whose daemon starts the re-home only after
	// the CLI's wait: the old daemon is frozen from then on.
	f.onKickstart = func() { time.AfterFunc(150*time.Millisecond, func() { s.frozen.Store(true) }) }

	policy, asked := recordPolicy(true) // --restart-daemon
	h := testHooks(nil)
	h.readyWait = 100 * time.Millisecond
	err := ensureDaemon(context.Background(), s.path, EnsureOptions{Restart: policy, Start: StartCLI}, h)
	if s.newShutdown.Load() {
		t.Fatal("the restart stopped this build's daemon, which owns the moved sessions")
	}
	if s.oldShutdown.Load() {
		t.Error("the old daemon was stopped while frozen for a handoff")
	}
	if err != nil {
		t.Errorf("err %v; want nil (this build's daemon answers)", err)
	}
	if len(*asked) != 1 {
		t.Errorf("asked %v; want the one restart question", *asked)
	}
}

// The pinned /shutdown never goes to this build's daemon, even when it is the
// one answering from the start.
func TestShutdownIfStale_RefusesThisBuild(t *testing.T) {
	useBuild(t, "v2")
	s := &swapDaemon{path: shortProcTestSocket(t, "pin2")}
	t.Cleanup(s.close)
	s.serve(t, "v2", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shutdown" {
			s.newShutdown.Store(true)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	err := NewDaemonClient(s.path).shutdownIfStale()
	if err != errNotStale || s.newShutdown.Load() {
		t.Fatalf("err %v, shutdown %v; want errNotStale, false", err, s.newShutdown.Load())
	}
}
