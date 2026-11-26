package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stalledDesktopViewer starts the old daemon's desktop, attaches a viewer whose
// close handshake never completes and waits until captureActive is recorded.
func stalledDesktopViewer(t *testing.T) (*desktopHandoff, *desktopViewer) {
	t.Helper()
	d, link := newDesktopHandoff(t, true, tickScript(false))
	v := connectViewerMode(t, link, true)
	v.waitIDR(t, 10*time.Second)
	eventually(t, 5*time.Second, "captureActive to be recorded", func() bool {
		a, _ := readCaptureActive(t, d.cfg)
		return a
	})
	return d, v
}

// The handoff's tail gives desktop viewers restartDrain (1 s) and then exits, so
// a viewer whose close handshake is slow (DERP) must not keep captureActive true
// while it is pending: the next process would count a clean upgrade as a crash.
func TestHandoffDesktop_CaptureActiveClearedBeforeViewerHandshake(t *testing.T) {
	d, v := stalledDesktopViewer(t)
	done := make(chan struct{})
	go func() { defer close(done); d.old.closeDesktopForHandoff() }()
	t.Cleanup(func() { _ = v.conn.CloseNow(); <-done })

	eventually(t, 2*time.Second, "captureActive cleared while the handshake is pending", func() bool {
		a, _ := readCaptureActive(t, d.cfg)
		return !a
	})
	select {
	case <-done:
		t.Fatal("closeDesktopForHandoff returned before the stalled handshake: the test no longer proves the ordering")
	default:
	}
	if _, exits := readCaptureActive(t, d.cfg); exits != 0 {
		t.Fatalf("abnormalExits %d, want 0", exits)
	}
}

// Quit All posts /shutdown and the daemon exits a moment later without going
// through api.Stop: the exit must not leave captureActive true.
func TestShutdown_CaptureActiveClearedBeforeExit(t *testing.T) {
	d, v := stalledDesktopViewer(t)
	exited := make(chan bool, 1)
	prev := shutdownExit
	shutdownExit = func(int) {
		a, _ := readCaptureActive(t, d.cfg)
		exited <- a
	}
	t.Cleanup(func() { shutdownExit = prev; _ = v.conn.CloseNow() })

	d.old.handleShutdown(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/shutdown", nil))
	select {
	case active := <-exited:
		if active {
			t.Fatal("the daemon exited on /shutdown with captureActive still true")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon did not exit after /shutdown")
	}
}
