package webserver

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

// TestAppStatic_CacheHeaders pins the /app/ cache policy per file class: the
// entry point is no-store, Vite's content-hashed assets/ files keep caching (no
// Cache-Control), and every other file (manifest, icons: not hashed) is
// no-cache so a changed one is never served stale (MOB-02).
func TestAppStatic_CacheHeaders(t *testing.T) {
	ws, client := testServer(t)
	ws.SetStaticAppFS(fstest.MapFS{
		"index.html":             {Data: []byte(fakeIndexHTML)},
		"assets/index-abc123.js": {Data: []byte("// js")},
		"manifest.json":          {Data: []byte(`{"name":"Portty"}`)},
		"icon-192.png":           {Data: []byte("png")},
		"apple-touch-icon.png":   {Data: []byte("png")},
	})
	for _, c := range []struct{ path, cacheControl string }{
		{"/app/", "no-store"},
		{"/app/?session=x", "no-store"},
		{"/app/assets/index-abc123.js", ""},
		{"/app/manifest.json", "no-cache"},
		{"/app/icon-192.png", "no-cache"},
		{"/app/apple-touch-icon.png", "no-cache"},
	} {
		t.Run(c.path, func(t *testing.T) {
			resp, err := client.Get(ws.BaseURL() + c.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if got := resp.Header.Get("Cache-Control"); got != c.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, c.cacheControl)
			}
			if c.path == "/app/manifest.json" {
				if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}
			}
		})
	}
}
