package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServer_FilesCORS_ChatRouteShape verifies the relay CORS helpers the way
// internal/daemon/chat_routes.go uses them: a GET route wrapped in FilesCORS
// plus an OPTIONS route on FilesPreflight. The Wails webview calls
// 127.0.0.1:<relayPort> cross-origin; without these headers the browser blocks
// chat history/export. Allowed loopback / Wails origins get CORS headers, a
// disallowed origin does not.
func TestServer_FilesCORS_ChatRouteShape(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/chat/{id}/history", FilesCORS(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	mux.HandleFunc("OPTIONS /api/chat/{id}/history", FilesPreflight)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	url := srv.URL + "/api/chat/s1/history"

	do := func(t *testing.T, method, origin string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, url, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if method == http.MethodOptions {
			req.Header.Set("Access-Control-Request-Method", "GET")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res
	}

	for _, origin := range []string{"wails://wails", "http://wails.localhost"} {
		t.Run("preflight from "+origin+" returns 204 + ACAO", func(t *testing.T) {
			res := do(t, http.MethodOptions, origin)
			if res.StatusCode != http.StatusNoContent {
				t.Fatalf("status: got %d want 204", res.StatusCode)
			}
			if got := res.Header.Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("ACAO: got %q want %q", got, origin)
			}
			if res.Header.Get("Access-Control-Allow-Methods") == "" {
				t.Error("Access-Control-Allow-Methods missing")
			}
		})
	}

	t.Run("preflight from disallowed origin returns 403 without ACAO", func(t *testing.T) {
		res := do(t, http.MethodOptions, "http://evil.example.com")
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("status: got %d want 403", res.StatusCode)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("ACAO unexpected: %q", got)
		}
	})

	t.Run("GET with allowed Origin echoes ACAO", func(t *testing.T) {
		res := do(t, http.MethodGet, "http://wails.localhost")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status: got %d want 200", res.StatusCode)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "http://wails.localhost" {
			t.Errorf("ACAO: got %q want %q", got, "http://wails.localhost")
		}
	})

	t.Run("GET with disallowed Origin gets no ACAO", func(t *testing.T) {
		res := do(t, http.MethodGet, "http://evil.example.com")
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("ACAO unexpected: %q", got)
		}
	})

	t.Run("GET with no Origin (curl-style) works without ACAO", func(t *testing.T) {
		res := do(t, http.MethodGet, "")
		if res.StatusCode != http.StatusOK {
			t.Errorf("status: got %d want 200", res.StatusCode)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("ACAO unexpected: %q", got)
		}
	})
}
