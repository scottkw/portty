package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const msec = time.Millisecond

// socketCounter provides unique short socket names across parallel tests.
var socketCounter atomic.Int64

// shortSocketPath creates a short socket path in /tmp to avoid the 103-char
// macOS sun_path limit that t.TempDir() frequently exceeds.
func shortSocketPath(t *testing.T, name string) string {
	t.Helper()
	n := socketCounter.Add(1)
	path := fmt.Sprintf("/tmp/dtest%d_%s", n, name)
	t.Cleanup(func() { os.Remove(path) })
	return path
}

func TestSocketPathDefault(t *testing.T) {
	path := DefaultSocketPath()
	if path == "" {
		t.Fatal("DefaultSocketPath returned empty string")
	}
	if !strings.HasSuffix(path, filepath.Join("portty", "daemon.sock")) {
		t.Errorf("DefaultSocketPath = %q, want suffix %q", path, filepath.Join("portty", "daemon.sock"))
	}
}

func TestSocketPathLength(t *testing.T) {
	// Path <= 103 chars: no error.
	short := "/" + strings.Repeat("a", 10) + "/d.sock"
	if err := ValidateSocketPath(short); err != nil {
		t.Errorf("ValidateSocketPath(%d chars): unexpected error: %v", len(short), err)
	}

	// Path > 103 chars: expect error.
	long := "/" + strings.Repeat("a", 104)
	if err := ValidateSocketPath(long); err == nil {
		t.Errorf("ValidateSocketPath(%d chars): expected error, got nil", len(long))
	}
}

func TestCleanupStaleSocket_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.sock")
	if err := CleanupStaleSocket(path); err != nil {
		t.Errorf("CleanupStaleSocket on nonexistent path: unexpected error: %v", err)
	}
}

func TestCleanupStaleSocket_StaleFile(t *testing.T) {
	path := shortSocketPath(t, "stale.sock")

	// Create a socket file with nothing listening.
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("net.Listen to create socket file: %v", err)
	}
	ln.Close() // close immediately — socket file remains but no one is listening

	if err := CleanupStaleSocket(path); err != nil {
		t.Errorf("CleanupStaleSocket on stale socket: unexpected error: %v", err)
	}

	// Verify the file was removed.
	conn, err := net.DialTimeout("unix", path, 100*msec)
	if err == nil {
		conn.Close()
		t.Error("stale socket file still exists (dial succeeded)")
	}
}

func TestCleanupStaleSocket_ActiveSocket(t *testing.T) {
	path := shortSocketPath(t, "active.sock")

	// Start a real listener — daemon is "running".
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	// Accept connections in background so the dial in CleanupStaleSocket succeeds.
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	err = CleanupStaleSocket(path)
	if err == nil {
		t.Error("CleanupStaleSocket on active socket: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error should mention 'already running', got: %v", err)
	}
}

// A daemon's control socket is private to its user from the moment it listens
// (Phase 9 audit O-2), and the owner can still use it.
func TestListenDaemonSocket_Mode0600(t *testing.T) {
	_, client, path := testDaemon(t)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("socket mode = %o, want 600", got)
	}
	if err := client.Health(); err != nil {
		t.Errorf("Health on the 0600 socket: %v", err)
	}
}
