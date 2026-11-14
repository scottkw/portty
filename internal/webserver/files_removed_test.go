package webserver

import (
	"net/http"
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
