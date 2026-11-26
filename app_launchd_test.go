package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

// fakeLaunchctl is a fake launchd behind daemon's launchctl seam: it records
// every call and never touches the real launchd. loaded and disabled describe
// Portty's job; a kickstart runs onKickstart (which brings a test daemon up).
type fakeLaunchctl struct {
	mu          sync.Mutex
	calls       []string // subcommands, in order
	loaded      bool
	disabled    bool
	onKickstart func()
}

func newFakeLaunchctl(t *testing.T) *fakeLaunchctl {
	t.Helper()
	f := &fakeLaunchctl{}
	t.Cleanup(daemon.SetLaunchctlForTest(f.run))
	return f
}

func (f *fakeLaunchctl) run(_ context.Context, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, args[0])
	loaded, disabled, onKick := f.loaded, f.disabled, f.onKickstart
	f.mu.Unlock()
	switch args[0] {
	case "print":
		if !loaded {
			return []byte(`Could not find service "` + daemon.LaunchdLabel + `" in domain`), errors.New("exit status 113")
		}
		return []byte("job = {\n\tstate = not running\n}\n"), nil
	case "print-disabled":
		out := "disabled services = {\n"
		if disabled {
			out += "\t\"" + daemon.LaunchdLabel + "\" => disabled\n"
		}
		return []byte(out + "}\n"), nil
	case "kickstart":
		if onKick != nil {
			onKick()
		}
	}
	return nil, nil
}

func (f *fakeLaunchctl) count(sub string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == sub {
			n++
		}
	}
	return n
}

func (f *fakeLaunchctl) set(loaded, disabled bool) {
	f.mu.Lock()
	f.loaded, f.disabled = loaded, disabled
	f.mu.Unlock()
}

// installedApp makes the running executable look like ~/Applications/Portty.app
// under the test HOME, with the agent seam recording its calls.
func installedApp(t *testing.T) (agentCalls *[]string) {
	t.Helper()
	exe := filepath.Join(os.Getenv("HOME"), "Applications/Portty.app/Contents/MacOS/Portty")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldExe, oldAgent := executablePathFunc, ensureAgentFunc
	t.Cleanup(func() { executablePathFunc, ensureAgentFunc = oldExe, oldAgent })
	executablePathFunc = func() (string, error) { return exe, nil }
	agentCalls = new([]string)
	ensureAgentFunc = func(_ context.Context, e string) (daemon.AgentState, error) {
		*agentCalls = append(*agentCalls, e)
		return daemon.AgentReady, nil
	}
	return agentCalls
}

// fallbackRecorder replaces the detached spawn: it counts calls and brings a
// test daemon up at sock, as a real fallback daemon would be.
func fallbackRecorder(t *testing.T, sock string) *int {
	t.Helper()
	n := new(int)
	t.Cleanup(daemon.SetFallbackSpawnForTest(func(string) (<-chan struct{}, func(), error) {
		*n++
		startTestDaemon(t, sock)
		return nil, nil, nil
	}))
	return n
}

func quietApp() *App {
	return &App{
		startupDone:         make(chan struct{}),
		emitEventFunc:       func(string, any) {},
		applyAppearanceFunc: func(string) {},
	}
}

// A GUI start with Login Items turned off still starts the daemon, through the
// fallback, and marks only the desktop for repair: no daemon error appears.
func TestStartDaemon_DisabledAgentFallsBackAndMarksRepair(t *testing.T) {
	sock := respawnHome(t)
	installedApp(t)
	ensureAgentFunc = func(context.Context, string) (daemon.AgentState, error) {
		return daemon.AgentReady, daemon.ErrAgentDisabled
	}
	f := newFakeLaunchctl(t)
	f.set(false, true)
	fallbacks := fallbackRecorder(t, sock)

	a := quietApp()
	a.startDaemon(sock)

	if got := a.GetDaemonError(); got != "" {
		t.Errorf("GetDaemonError = %q, want none while the fallback daemon runs", got)
	}
	if *fallbacks != 1 {
		t.Errorf("fallback spawns = %d, want 1", *fallbacks)
	}
	if got := a.GetDaemonRepair(); got != "login_items" {
		t.Errorf("GetDaemonRepair = %q, want login_items", got)
	}
	if n := f.count("kickstart"); n != 0 {
		t.Errorf("kickstart ran %d times though the agent is disabled", n)
	}
}

func TestOpenLoginItems_FixedURL(t *testing.T) {
	old := openSystemURL
	t.Cleanup(func() { openSystemURL = old })
	var got []string
	openSystemURL = func(url string) error { got = append(got, url); return nil }
	if err := (&App{}).OpenLoginItems(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{openURLLoginItems}) {
		t.Errorf("opened %v, want exactly %q", got, openURLLoginItems)
	}
	if !strings.HasPrefix(openURLLoginItems, "x-apple.systempreferences:com.apple.LoginItems") {
		t.Errorf("unexpected Login Items URL %q", openURLLoginItems)
	}
	openSystemURL = old
	if err := (&App{}).OpenLoginItems(); err == nil {
		t.Error("the real open ran under the test binary")
	}
}
