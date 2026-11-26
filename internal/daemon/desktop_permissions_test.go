package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// 19-11: GET /desktop/permissions and POST /desktop/test-capture. The probe is
// the 19-09 Prober over a fake child runner and the capture is a fake still, so
// nothing here reads TCC or captures the host's screen.

// fakeJPEG is a few bytes with a JPEG start-of-image marker; the endpoint passes
// the bytes through and never decodes them.
var fakeJPEG = []byte{0xff, 0xd8, 0xff, 0xe0, 'p', 't', 0xff, 0xd9}

// probeRunnerAnswering returns a runner that serves each answer once, in order,
// then the last one again, and counts its calls.
func probeRunnerAnswering(calls *atomic.Int32, answers ...string) func(context.Context, string, ...string) ([]byte, error) {
	return func(context.Context, string, ...string) ([]byte, error) {
		i := int(calls.Add(1)) - 1
		if i >= len(answers) {
			i = len(answers) - 1
		}
		return []byte(answers[i]), nil
	}
}

// doDesktop calls the daemon handler in-process so a test sees the raw status,
// headers and body (the typed client hides them).
func doDesktop(api *API, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

// TestDesktopPermissions_EndToEnd is Task 1's tracer: the endpoint reads the
// grants through the Prober's child, two asks 1.1 s apart see a grant flip, and
// a Test Capture returns a JPEG and records the approval in desktop-state.json.
func TestDesktopPermissions_EndToEnd(t *testing.T) {
	api, client := shareTestDaemon(t)
	var spawns atomic.Int32
	t.Cleanup(SetDesktopPermsForTest(desktop.NewProberWithRunner(probeRunnerAnswering(&spawns,
		`{"sr":false,"ax":true,"pe":true}`, `{"sr":true,"ax":true,"pe":true}`))))
	t.Cleanup(SetDesktopCaptureForTest(func(_ context.Context, maxLongEdge int) ([]byte, error) {
		if maxLongEdge != testCaptureMaxLongEdge {
			t.Errorf("capture asked for a %d px edge, want %d", maxLongEdge, testCaptureMaxLongEdge)
		}
		return fakeJPEG, nil
	}, 0))

	sr, ax, err := client.DesktopPermissions()
	if err != nil || sr || !ax {
		t.Fatalf("first read = sr %v ax %v err %v; want sr false, ax true", sr, ax, err)
	}
	time.Sleep(1100 * time.Millisecond) // past the Prober's 1 s cache
	sr, ax, err = client.DesktopPermissions()
	if err != nil || !sr || !ax {
		t.Fatalf("second read = sr %v ax %v err %v; want both granted", sr, ax, err)
	}
	if n := spawns.Load(); n != 2 {
		t.Fatalf("child spawns = %d, want 2 (one per read past the cache)", n)
	}

	jpeg, code, err := client.DesktopTestCapture()
	if err != nil || code != "" || !bytes.Equal(jpeg, fakeJPEG) {
		t.Fatalf("test capture = %d bytes, code %q, err %v; want the JPEG", len(jpeg), code, err)
	}
	raw, err := os.ReadFile(filepath.Join(api.engine.configDir, "desktop-state.json"))
	if err != nil {
		t.Fatalf("a successful test capture left no desktop-state.json: %v", err)
	}
	var st struct {
		ApprovalSeenAt int64 `json:"approvalSeenAt"`
	}
	if err := json.Unmarshal(raw, &st); err != nil || st.ApprovalSeenAt == 0 {
		t.Fatalf("desktop-state.json = %s (%v); want approvalSeenAt set", raw, err)
	}
	if rec := doDesktop(api, http.MethodPost, "/desktop/test-capture"); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("POST test-capture = %d %q; want 200 image/jpeg", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// DPRM-01 / T-19-41: overlapping asks (the sheet polling, the state loop, a
// second window) share one child spawn.
func TestDesktopPermissions_SharedSpawn(t *testing.T) {
	api, _ := shareTestDaemon(t)
	var spawns atomic.Int32
	slow := func(context.Context, string, ...string) ([]byte, error) {
		spawns.Add(1)
		time.Sleep(100 * time.Millisecond)
		return []byte(`{"sr":true,"ax":false,"pe":false}`), nil
	}
	t.Cleanup(SetDesktopPermsForTest(desktop.NewProberWithRunner(slow)))

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := doDesktop(api, http.MethodGet, "/desktop/permissions")
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sr":true`) {
				t.Errorf("GET /desktop/permissions = %d %s; want 200 with sr true", rec.Code, rec.Body)
			}
		}()
	}
	wg.Wait()
	if n := spawns.Load(); n != 1 {
		t.Fatalf("five overlapping asks ran %d child spawns, want 1", n)
	}
}

// Research C1: "not granted" must mean the OS said so. A probe that fails, or
// prints something other than the three-key line, is probe_failed, never
// {sr:false}.
func TestDesktopPermissions_ProbeFailure(t *testing.T) {
	api, client := shareTestDaemon(t)
	prints := func(s string) func(context.Context, string, ...string) ([]byte, error) {
		return func(context.Context, string, ...string) ([]byte, error) { return []byte(s), nil }
	}
	for name, run := range map[string]func(context.Context, string, ...string) ([]byte, error){
		"child fails":    func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("exit status 1") },
		"malformed":      prints(`{"sr":tru`),
		"missing key":    prints(`{"sr":true,"ax":true}`),
		"unknown key":    prints(`{"sr":true,"ax":true,"pe":true,"x":1}`),
		"empty":          prints(``),
		"trailing lines": prints("{\"sr\":true,\"ax\":true,\"pe\":true}\n{}"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(SetDesktopPermsForTest(desktop.NewProberWithRunner(run)))
			rec := doDesktop(api, http.MethodGet, "/desktop/permissions")
			if rec.Code != http.StatusServiceUnavailable || strings.TrimSpace(rec.Body.String()) != `{"code":"probe_failed"}` {
				t.Fatalf("GET = %d %q; want 503 {\"code\":\"probe_failed\"}", rec.Code, rec.Body)
			}
			if sr, ax, err := client.DesktopPermissions(); err == nil || sr || ax {
				t.Fatalf("client.DesktopPermissions = %v %v %v; want an error", sr, ax, err)
			}
		})
	}
}

// WR-03: the setup sheet's permission poll refreshes the desktop state at
// once, so Settings and the tray stop saying Needs permission the moment
// Screen Recording is granted, not at the next 5-minute idle probe.
func TestDesktopPermissions_PollRefreshesState(t *testing.T) {
	var granted atomic.Bool
	answer := func(context.Context, string, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"sr":%v,"ax":true,"pe":true}`, granted.Load()), nil
	}
	old := desktopTestDeps
	desktopTestDeps = func(d *desktop.Deps) {
		old(d)
		d.Probe = func(context.Context) (screencap.Permissions, error) {
			return screencap.Permissions{SR: granted.Load(), AX: true, PE: true}, nil
		}
	}
	t.Cleanup(func() { desktopTestDeps = old })
	e := newDesktopEnv(t)
	e.mustOn(t)

	t.Cleanup(SetDesktopPermsForTest(desktop.NewProberWithRunner(answer)))
	if rec := doDesktop(e.api, http.MethodGet, "/desktop/permissions"); rec.Code != http.StatusOK {
		t.Fatalf("GET /desktop/permissions = %d", rec.Code)
	}
	if st := e.status(t); st.State != string(desktop.StateNeedsPermission) {
		t.Fatalf("state after a not-granted poll = %q, want needs_permission", st.State)
	}
	granted.Store(true)
	t.Cleanup(SetDesktopPermsForTest(desktop.NewProberWithRunner(answer))) // a fresh cache
	if rec := doDesktop(e.api, http.MethodGet, "/desktop/permissions"); rec.Code != http.StatusOK {
		t.Fatalf("GET /desktop/permissions = %d", rec.Code)
	}
	if st := e.status(t); st.State == string(desktop.StateNeedsPermission) {
		t.Fatalf("state right after a granted poll = %q, want it cleared", st.State)
	}
}

// The Test Capture error codes, and what a failure must not do: record an
// approval (NoteTestCapture writes the state file; a failure leaves none).
func TestDesktopTestCapture_Codes(t *testing.T) {
	api, client := shareTestDaemon(t)
	stateFile := filepath.Join(api.engine.configDir, "desktop-state.json")
	for _, tc := range []struct {
		name     string
		capture  func(ctx context.Context, _ int) ([]byte, error)
		wantCode string
		wantHTTP int
	}{
		{"permission", func(context.Context, int) ([]byte, error) {
			return nil, fmt.Errorf("start: %w", screencap.ErrPermissionDenied)
		}, "needs_permission", http.StatusConflict},
		{"ceiling", func(ctx context.Context, _ int) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}, "timeout", http.StatusGatewayTimeout},
		{"other", func(context.Context, int) ([]byte, error) { return nil, errors.New("encoder exploded") }, "capture_failed", http.StatusInternalServerError},
		{"no display", func(context.Context, int) ([]byte, error) { return nil, screencap.ErrNoDisplay }, "capture_failed", http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(SetDesktopCaptureForTest(tc.capture, 100*time.Millisecond))
			rec := doDesktop(api, http.MethodPost, "/desktop/test-capture")
			if rec.Code != tc.wantHTTP || strings.TrimSpace(rec.Body.String()) != `{"code":"`+tc.wantCode+`"}` {
				t.Fatalf("POST = %d %q; want %d code %s", rec.Code, rec.Body, tc.wantHTTP, tc.wantCode)
			}
			if jpeg, code, err := client.DesktopTestCapture(); jpeg != nil || code != tc.wantCode || err != nil {
				t.Fatalf("client = %d bytes, code %q, err %v; want code %s", len(jpeg), code, err, tc.wantCode)
			}
			if _, err := os.Stat(stateFile); err == nil {
				t.Fatal("a failed test capture wrote desktop-state.json (recorded an approval)")
			}
		})
	}
}

// A good Test Capture clears a tripped crash-loop breaker on a running Manager
// (NoteTestCapture alone only clears the file).
func TestDesktopTestCapture_ClearsBreaker(t *testing.T) {
	e := newDesktopEnv(t)
	now := time.Now().Unix()
	state := fmt.Sprintf(`{"schemaVersion":1,"approvalSeenAt":0,"captureActive":false,"abnormalExits":[%d,%d,%d]}`, now, now, now)
	if err := os.WriteFile(filepath.Join(e.api.engine.configDir, "desktop-state.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	e.mustOn(t)
	if st := e.status(t); st.State != string(desktop.StateCapturePaused) {
		t.Fatalf("with three recent abnormal exits the state is %q, want capture_paused", st.State)
	}
	t.Cleanup(SetDesktopCaptureForTest(func(context.Context, int) ([]byte, error) { return fakeJPEG, nil }, 0))
	if _, code, err := e.client.DesktopTestCapture(); err != nil || code != "" {
		t.Fatalf("test capture: code %q, err %v", code, err)
	}
	eventually(t, 5*time.Second, "capture_paused to clear after a good test capture", func() bool {
		return e.status(t).State != string(desktop.StateCapturePaused)
	})
}
