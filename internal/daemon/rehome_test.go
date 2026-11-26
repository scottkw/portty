package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// shortHome points HOME at a short /tmp dir so DefaultSocketPath fits the
// sun_path limit and each test has its own socket and lock.
func shortHome(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "pdh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	t.Setenv("HOME", d)
	t.Setenv("XDG_CONFIG_HOME", d)
	t.Setenv("XPC_SERVICE_NAME", "")
	return d
}

// recordExit replaces daemonExit with a recorder of the codes it was given.
func recordExit(t *testing.T) *[]int {
	t.Helper()
	var codes []int
	old := daemonExit
	daemonExit = func(c int) { codes = append(codes, c) }
	t.Cleanup(func() { daemonExit = old })
	return &codes
}

// okSteps makes the four fatal startup steps succeed without doing anything.
func okSteps(t *testing.T) {
	t.Helper()
	c, b, r, a := cleanupStaleSocket, bootstrapCapabilityState, startRelay, startAPI
	cleanupStaleSocket = func(string) error { return nil }
	bootstrapCapabilityState = func(*API) error { return nil }
	startRelay = func(*API) (int, error) { return 0, nil }
	startAPI = func(*API, string) error { return nil }
	t.Cleanup(func() { cleanupStaleSocket, bootstrapCapabilityState, startRelay, startAPI = c, b, r, a })
}

// Research Pitfall 3 (T-19-07): launchd restarts on a non-zero exit only, so
// every real startup failure exits 1, and "already running" exits 0.
func TestRunDaemonCore_ExitCodes(t *testing.T) {
	boom := errors.New("boom")
	fail := map[string]func(t *testing.T){
		"stale socket cleanup": func(t *testing.T) { cleanupStaleSocket = func(string) error { return boom } },
		"capability bootstrap": func(t *testing.T) { bootstrapCapabilityState = func(*API) error { return boom } },
		"relay start":          func(t *testing.T) { startRelay = func(*API) (int, error) { return 0, boom } },
		"api start":            func(t *testing.T) { startAPI = func(*API, string) error { return boom } },
		"lock file unusable": func(t *testing.T) { // a directory where the lock file belongs
			if err := os.MkdirAll(filepath.Join(filepath.Dir(DefaultSocketPath()), "daemon.lock"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakStep := range fail {
		t.Run(name, func(t *testing.T) {
			shortHome(t)
			okSteps(t)
			codes := recordExit(t)
			breakStep(t)
			runDaemonCore(context.Background())
			if len(*codes) != 1 || (*codes)[0] != 1 {
				t.Fatalf("daemonExit calls = %v, want [1]", *codes)
			}
		})
	}
	t.Run("lock held by another daemon", func(t *testing.T) {
		shortHome(t)
		codes := recordExit(t)
		old := lockWait
		lockWait = 20 * time.Millisecond
		t.Cleanup(func() { lockWait = old })
		lock, err := lockDaemon(filepath.Join(filepath.Dir(DefaultSocketPath()), "daemon.lock"), 0)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		runDaemonCore(context.Background())
		if len(*codes) != 0 {
			t.Fatalf("daemonExit calls = %v, want none (exit 0, launchd leaves it down)", *codes)
		}
	})
}
