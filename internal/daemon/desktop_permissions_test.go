package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
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
