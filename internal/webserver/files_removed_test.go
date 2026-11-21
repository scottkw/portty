package webserver

import (
	"net/http"
	"strings"
	"testing"
)

// TestFilesRemoved_WebServerMux proves the guest-facing webserver no longer
// serves any files route (STRP-02, T-01-10). The request carries a valid cap
// bearing the legacy files perms, so a 404/405 here means the route itself is
// gone, not that the capability gate rejected it.
func TestFilesRemoved_WebServerMux(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("sess-files")
	token := issueCapFor(t, ws, "sess-files", "read,write,files.read,files.write")

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/files/list?session=sess-files&path=."},
		{http.MethodGet, "/api/files/read?session=sess-files&path=x"},
		{http.MethodHead, "/api/files/read?session=sess-files&path=x"},
		{http.MethodPut, "/api/files/write?session=sess-files&path=x"},
	} {
		req, _ := http.NewRequest(r.method, ws.BaseURL()+r.path+"&cap="+token, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, r.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d; want 404 or 405 (files API removed)", r.method, r.path, resp.StatusCode)
		}
	}
}

// TestPreviewRoute_NotOnShareServer proves the tailnet-facing share server
// has no preview route (PREV-03, T-11-11/12): previews enter only through the
// daemon's unix socket. The request carries a valid read,write cap, so a
// refusal here is the missing route, not the capability gate.
func TestPreviewRoute_NotOnShareServer(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("sess-prev")
	token := issueCapFor(t, ws, "sess-prev", "read,write")

	for _, path := range []string{"/sessions/sess-prev/preview", "/app/sessions/sess-prev/preview"} {
		for _, method := range []string{http.MethodPost, http.MethodGet} {
			req, _ := http.NewRequest(method, ws.BaseURL()+path+"?cap="+token, strings.NewReader("x"))
			req.Header.Set("X-Portty-Name", "a.png")
			req.Header.Set("X-Portty-Mime", "image/png")
			req.Header.Set("X-Portty-Key", "0123456789abcdef0123456789abcdef")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			resp.Body.Close()
			if method == http.MethodGet && strings.HasPrefix(path, "/app/") {
				// GET /app/* is the SPA shell fallback (index.html; 503 here, no
				// bundle wired): never preview bytes.
				if resp.StatusCode != http.StatusServiceUnavailable {
					t.Errorf("GET %s = %d; want the SPA fallback's 503 (no bundle in this test)", path, resp.StatusCode)
				}
				continue
			}
			if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d; want 404 or 405 (no preview route on the share server)", method, path, resp.StatusCode)
			}
		}
	}
}
