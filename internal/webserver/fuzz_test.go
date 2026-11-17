package webserver

// Security fuzz harness (STRP-04) for the webserver's untrusted HTTP surface:
// the capability middleware and route/path handling. Requests go straight to
// handlers via httptest (no TLS listener). Run one target at a time, e.g.
//
//	go test -run='^$' -fuzz='^FuzzRequireCapability$' -fuzztime=10s ./internal/webserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
)

// fuzzWebServer builds a WebServer the way testServer does, minus Start(), with
// session "s1" web-enabled and the test signing key set. A bare (never served)
// loopback listener gives BaseURL a value: cspHeaders fails closed with a 500
// when the listener is unset, which would mask real 500s.
func fuzzWebServer(f *testing.F) *WebServer {
	f.Helper()
	ws, err := NewWebServer(Config{BindIP: "127.0.0.1", FQDN: "127.0.0.1"}, relay.NewHubManager())
	if err != nil {
		f.Fatalf("NewWebServer: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		f.Fatalf("net.Listen: %v", err)
	}
	f.Cleanup(func() { _ = ln.Close() })
	ws.listener = ln
	ws.SetWhoIsClientForTest(&fakeWhoIs{err: errors.New("no tailscaled in tests")})
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("s1")
	return ws
}

// fuzzCap signs a cap and registers its grant (issueCapFor needs a *testing.T).
func fuzzCap(f *testing.F, ws *WebServer, sid, perms string) string {
	f.Helper()
	claims := capability.Claims{SID: sid, Perms: perms, IAT: time.Now().Unix(), GrantID: "grant-" + sid + "-" + perms, V: 1}
	token, err := capability.Sign(claims, capTestKey)
	if err != nil {
		f.Fatalf("capability.Sign: %v", err)
	}
	ws.AddGrant(sid, claims.GrantID)
	return token
}

func FuzzRequireCapability(f *testing.F) {
	ws := fuzzWebServer(f)
	readS1 := fuzzCap(f, ws, "s1", "read")
	writeS2 := fuzzCap(f, ws, "s2", "read,write") // granted but session s2 is not enabled

	f.Add(readS1, "s1")
	f.Add(readS1, "s2")
	f.Add(writeS2, "s2")
	f.Add("", "s1")
	f.Add("not.a.token", "s1")
	f.Add(readS1, "s1/../s2")

	f.Fuzz(func(t *testing.T, token, sessionID string) {
		nextRan := false
		var gotSID string
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/sessions/{id}/info", ws.requireCapability(func(w http.ResponseWriter, r *http.Request) {
			nextRan = true
			gotSID = r.PathValue("id")
			w.WriteHeader(http.StatusOK)
		}))

		req, err := http.NewRequest(http.MethodGet,
			"/api/sessions/"+url.PathEscape(sessionID)+"/info?cap="+url.QueryEscape(token), nil)
		if err != nil {
			t.Skip() // not a request a client could send
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code == http.StatusInternalServerError {
			t.Fatalf("500 for token=%q sid=%q", token, sessionID)
		}
		if !nextRan {
			return
		}
		claims, err := capability.Verify(token, capTestKey)
		if err != nil {
			t.Fatalf("next ran for a cap that does not verify: token=%q sid=%q", token, sessionID)
		}
		if claims.SID != gotSID {
			t.Fatalf("next ran with cap SID %q for path session %q", claims.SID, gotSID)
		}
	})
}

func FuzzWebServerRoutes(f *testing.F) {
	ws := fuzzWebServer(f)
	readS1 := fuzzCap(f, ws, "s1", "read")

	f.Add("", "")
	f.Add("dashboard", "")
	f.Add("api/sessions/meta", "")
	f.Add("api/sessions/s1/info", "cap="+url.QueryEscape(readS1))
	f.Add("api/chat/s1/history", "cap=bad") // removed chat route (CHAT-01): must 404, not 500
	f.Add("sessions/s1", "cap=x")
	f.Add("assets/../../etc/passwd", "")
	f.Add("app/%2e%2e/secret", "a=%zz")

	f.Fuzz(func(t *testing.T, path, rawQuery string) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = "/" + path
		req.URL.RawPath = ""
		req.URL.RawQuery = rawQuery
		// Guard against any streaming handler holding the request open.
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		rec := httptest.NewRecorder()
		ws.mux.ServeHTTP(rec, req.WithContext(ctx))
		if rec.Code == http.StatusInternalServerError {
			t.Fatalf("500 for path=%q query=%q body=%q", req.URL.Path, rawQuery, rec.Body.String())
		}
	})
}
