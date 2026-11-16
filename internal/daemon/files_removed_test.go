package daemon_test

// STRP-02: the file browser API is gone. These negative route tests prove no
// files route survives on the daemon socket mux or the relay loopback mux
// (T-01-10: a stale cap carrying legacy files perms has nothing to reach).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

func newRemovedRoutesAPI(t *testing.T) *daemon.API {
	t.Helper()
	engine := daemon.NewSessionEngine()
	engine.ConfigDirForTest(t.TempDir())
	return daemon.NewAPI(engine)
}

func assertRouteGone(t *testing.T, h http.Handler, method, path string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("%s %s = %d; want 404 or 405 (route removed)", method, path, rec.Code)
	}
}

var removedFilesRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/files/list?session=s&path=."},
	{http.MethodGet, "/api/files/read?session=s&path=x"},
	{http.MethodPut, "/api/files/write?session=s&path=x"},
	{http.MethodGet, "/api/files/remote/s/list?path=."},
	{http.MethodPut, "/api/files/remote/s/write?path=x"},
}

func TestFilesRemoved_DaemonSocketMux(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	for _, r := range removedFilesRoutes {
		assertRouteGone(t, h, r.method, r.path)
	}
}

func TestFilesRemoved_RelayLoopbackMux(t *testing.T) {
	h := newRemovedRoutesAPI(t).RelayHandler()
	for _, r := range removedFilesRoutes {
		assertRouteGone(t, h, r.method, r.path)
	}

	// The parent mux must still fall through to the relay server for paths it
	// does not own (sessions list), so the removal did not break the wrapper.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /sessions on relay = %d; want 200 (fallthrough to relay server)", rec.Code)
	}
}

// WR-06: GET /sessions/{id}/status answered "running" for unknown ids and had
// no production caller; status comes from ListSessions only.
func TestSessionStatusRouteRemoved(t *testing.T) {
	assertRouteGone(t, newRemovedRoutesAPI(t).Handler(), http.MethodGet, "/sessions/nonexistent/status")
}
