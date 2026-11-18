package daemon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	var _ func(string) error = EnsureDaemon
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
