package webserver

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestShareRemoved_StopClosesOnlyListener proves Stop owns exactly one
// listener: once it returns, nothing still accepts TLS on the server address.
func TestShareRemoved_StopClosesOnlyListener(t *testing.T) {
	ws, _ := testServer(t)
	addr := ws.Addr()
	if addr == "" {
		t.Fatal("Addr() empty after Start")
	}
	if err := ws.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	d := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", addr, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test dial to a closed listener
	if err == nil {
		conn.Close()
		t.Fatalf("TLS dial to %s succeeded after Stop; want failure", addr)
	}
}

// TestStop_ClosesKeepAliveConns (WR-01): a pooled keep-alive connection stops
// reaching the server once Stop returns.
func TestStop_ClosesKeepAliveConns(t *testing.T) {
	ws, client := testServer(t)
	resp, err := client.Get(ws.BaseURL() + "/dashboard")
	if err != nil {
		t.Fatalf("GET before Stop: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err := ws.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if resp, err := client.Get(ws.BaseURL() + "/dashboard"); err == nil {
		resp.Body.Close()
		t.Fatalf("GET over the pooled connection after Stop = %d; want an error", resp.StatusCode)
	}
}

// TestShareRemoved_WebServerRoutes proves the code-based access page, its
// exchange endpoint and the image endpoint are gone. The request carries a
// valid read,write cap for an enabled session, so a 404/405 means the route
// itself is gone, not that the capability gate rejected it.
func TestShareRemoved_WebServerRoutes(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("sess-share")
	token := issueCapFor(t, ws, "sess-share", "read,write")

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/join"},
		{http.MethodPost, "/join/exchange"},
		{http.MethodGet, "/api/sessions/sess-share/qr"},
	} {
		req, _ := http.NewRequest(r.method, ws.BaseURL()+r.path+"?cap="+token, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, r.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d; want 404 or 405 (route removed)", r.method, r.path, resp.StatusCode)
		}
	}
}
