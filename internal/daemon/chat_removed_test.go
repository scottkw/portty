package daemon_test

// CHAT-01: the chat history/export routes are gone from the relay loopback
// surface the desktop webview reaches (they were mounted in an outer wrap layer
// with chat-only CORS). Reuses the files_removed_test.go helpers.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

func TestChatRemoved_RelayLoopbackMux(t *testing.T) {
	engine := daemon.NewSessionEngine()
	engine.ConfigDirForTest(t.TempDir())
	engine.SetSpawnCommandForTest("/bin/cat", nil)
	// A live session, so a surviving route would answer 200 rather than 404 for an unknown id.
	id, err := engine.CreateSession(context.Background(), "chat-removed", "", 80, 24, nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _ = engine.KillSession(id) })
	h := daemon.NewAPI(engine).RelayHandler()

	for _, path := range []string{"/api/chat/" + id + "/history", "/api/chat/" + id + "/export"} {
		assertRouteGone(t, h, http.MethodGet, path)
		assertRouteGone(t, h, http.MethodOptions, path)
	}

	// The kept relay routes still answer once the chat wrap layer is gone.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /sessions on relay = %d; want 200", rec.Code)
	}
}
