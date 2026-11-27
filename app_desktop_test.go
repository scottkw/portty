package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// 19-11: the three Set Up Desktop Sharing bindings. The daemon is real (a test
// daemon on a temp socket); its probe runner and still capture are fakes, so no
// TCC read, no capture and no System Settings pane happens on the host.

func probePrints(s string, err error) func(context.Context, string, ...string) ([]byte, error) {
	return func(context.Context, string, ...string) ([]byte, error) { return []byte(s), err }
}

func TestDesktopBindings_NeverThrow(t *testing.T) {
	check := func(t *testing.T, a *App, perm DesktopPermissionsResult, capt TestCaptureResult, wantPermCode, wantCapCode string) {
		t.Helper()
		if perm.ErrorCode != wantPermCode || perm.SR || perm.AX {
			t.Errorf("GetDesktopPermissions = %+v; want code %q and nothing granted (never a guessed grant)", perm, wantPermCode)
		}
		if capt.ErrorCode != wantCapCode || capt.ImageDataURL != "" {
			t.Errorf("RunDesktopTestCapture = %+v; want code %q and no image", capt, wantCapCode)
		}
	}

	t.Run("nil client", func(t *testing.T) {
		a := &App{}
		check(t, a, a.GetDesktopPermissions(), a.RunDesktopTestCapture(), "unavailable", "unavailable")
	})
	t.Run("daemon not running", func(t *testing.T) {
		a := &App{client: daemon.NewDaemonClient(respawnHome(t))}
		check(t, a, a.GetDesktopPermissions(), a.RunDesktopTestCapture(), "unavailable", "unavailable")
	})
	t.Run("daemon answering with failures", func(t *testing.T) {
		sock := respawnHome(t)
		startTestDaemon(t, sock)
		a := &App{client: daemon.NewDaemonClient(sock)}
		t.Cleanup(daemon.SetDesktopPermsForTest(desktop.NewProberWithRunner(probePrints("", errors.New("exit status 1")))))
		t.Cleanup(daemon.SetDesktopCaptureForTest(func(context.Context, int) ([]byte, error) {
			return nil, errors.New("encoder exploded")
		}, 0))
		check(t, a, a.GetDesktopPermissions(), a.RunDesktopTestCapture(), "unavailable", "capture_failed")
	})
	t.Run("permission refused and ceiling", func(t *testing.T) {
		sock := respawnHome(t)
		startTestDaemon(t, sock)
		a := &App{client: daemon.NewDaemonClient(sock)}
		var err error
		t.Cleanup(daemon.SetDesktopCaptureForTest(func(context.Context, int) ([]byte, error) { return nil, err }, 0))
		err = fmt.Errorf("start: %w", screencap.ErrPermissionDenied)
		if got := a.RunDesktopTestCapture(); got.ErrorCode != "needs_permission" || got.ImageDataURL != "" {
			t.Errorf("a permission refusal = %+v, want needs_permission", got)
		}
		err = context.DeadlineExceeded
		if got := a.RunDesktopTestCapture(); got.ErrorCode != "timeout" || got.ImageDataURL != "" {
			t.Errorf("a ceiling hit = %+v, want timeout", got)
		}
	})
	t.Run("success", func(t *testing.T) {
		sock := respawnHome(t)
		startTestDaemon(t, sock)
		a := &App{client: daemon.NewDaemonClient(sock)}
		jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
		t.Cleanup(daemon.SetDesktopPermsForTest(desktop.NewProberWithRunner(probePrints(`{"sr":true,"ax":false,"pe":false}`, nil))))
		t.Cleanup(daemon.SetDesktopCaptureForTest(func(context.Context, int) ([]byte, error) { return jpeg, nil }, 0))
		if got := a.GetDesktopPermissions(); got != (DesktopPermissionsResult{SR: true}) {
			t.Errorf("GetDesktopPermissions = %+v, want sr only", got)
		}
		got := a.RunDesktopTestCapture()
		want := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg)
		if got.ErrorCode != "" || got.ImageDataURL != want {
			t.Errorf("RunDesktopTestCapture = %+v, want the data URL %q", got, want)
		}
	})
}

// T-19-38 / DPRM-01: only the two permission panes open, chosen by a two-value
// kind. Full Disk Access, an empty kind and a raw URL are refused, nothing
// opens, and the real opener refuses under a test binary.
func TestOpenDesktopPermissionPane(t *testing.T) {
	var opened []string
	old := openSystemURL
	openSystemURL = func(u string) error { opened = append(opened, u); return nil }
	t.Cleanup(func() { openSystemURL = old })
	a := &App{}

	for kind, want := range map[string]string{
		"screen":        "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture",
		"accessibility": "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility",
	} {
		opened = nil
		if err := a.OpenDesktopPermissionPane(kind); err != nil || len(opened) != 1 || opened[0] != want {
			t.Errorf("OpenDesktopPermissionPane(%q) = %v, opened %v; want %q", kind, err, opened, want)
		}
	}
	for _, kind := range []string{"files", "full_disk", "", "Screen", "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles", "https://example.com"} {
		opened = nil
		if err := a.OpenDesktopPermissionPane(kind); err == nil || len(opened) != 0 {
			t.Errorf("OpenDesktopPermissionPane(%q) = %v, opened %v; want an error and nothing opened", kind, err, opened)
		}
	}

	openSystemURL = old
	if err := a.OpenDesktopPermissionPane("screen"); err == nil || !strings.Contains(err.Error(), "test binary") {
		t.Errorf("the real opener under go test = %v, want a refusal", err)
	}
}

// fakeDesktopDaemon answers the three /desktop routes the Settings bindings
// call on the daemon socket, so the bindings are tested against real HTTP
// without a daemon, a tailnet or any capture.
func fakeDesktopDaemon(t *testing.T, putStatus int, putBody string) *App {
	t.Helper()
	sock := respawnHome(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /desktop", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"alwaysAvailable":true,"state":"streaming","viewers":2,"locked":true,"approachingLapse":true,"lapseSeq":3,"tailscale":"ok","launchd":true,"reason":"r"}`))
	})
	mux.HandleFunc("PUT /desktop", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(putStatus)
		_, _ = w.Write([]byte(putBody))
	})
	mux.HandleFunc("GET /desktop/link", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"url":"https://example.invalid/desktop/tok"}`))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return &App{client: daemon.NewDaemonClient(sock)}
}

// DSHR-01 / DPRM-02: the Desktop group's three bindings carry the daemon's
// answer, keep a refusal's code and detail, and never throw.
func TestDesktopSettingsBindings(t *testing.T) {
	t.Run("status, on and link", func(t *testing.T) {
		a := fakeDesktopDaemon(t, 200, `{"alwaysAvailable":true,"firstShare":true}`)
		want := DesktopStatusResult{AlwaysAvailable: true, State: "streaming", Viewers: 2, Locked: true, ApproachingLapse: true, Tailscale: "ok", Launchd: true, Reason: "r"}
		if got := a.GetDesktopStatus(); got != want {
			t.Errorf("GetDesktopStatus = %+v, want %+v", got, want)
		}
		if got := a.SetDesktopAlwaysAvailable(true); got != (DesktopResult{AlwaysAvailable: true, FirstShare: true}) {
			t.Errorf("SetDesktopAlwaysAvailable(true) = %+v", got)
		}
		if got := a.GetDesktopLink(); got != (DesktopLinkResult{URL: "https://example.invalid/desktop/tok"}) {
			t.Errorf("GetDesktopLink = %+v", got)
		}
	})
	t.Run("a refusal keeps its code and detail", func(t *testing.T) {
		a := fakeDesktopDaemon(t, 400, `{"error":"https_off","detail":"certs are off"}`)
		if got := a.SetDesktopAlwaysAvailable(true); got != (DesktopResult{ErrorCode: "https_off", ErrorDetail: "certs are off"}) {
			t.Errorf("SetDesktopAlwaysAvailable = %+v, want https_off with its detail", got)
		}
	})
	t.Run("nil client and dead daemon are unavailable", func(t *testing.T) {
		for name, a := range map[string]*App{"nil": {}, "dead": {client: daemon.NewDaemonClient(respawnHome(t))}} {
			if got := a.GetDesktopStatus(); got != (DesktopStatusResult{ErrorCode: "unavailable"}) {
				t.Errorf("%s: GetDesktopStatus = %+v", name, got)
			}
			if got := a.SetDesktopAlwaysAvailable(false); got.ErrorCode != "unavailable" || got.ErrorDetail == "" {
				t.Errorf("%s: SetDesktopAlwaysAvailable = %+v", name, got)
			}
			if got := a.GetDesktopLink(); got != (DesktopLinkResult{ErrorCode: "unavailable"}) {
				t.Errorf("%s: GetDesktopLink = %+v", name, got)
			}
		}
	})
}
