package webserver_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
	"golang.org/x/sys/unix"
)

// dupListener duplicates ln's descriptor the way a handoff receiver gets it.
func dupListener(t *testing.T, ln net.Listener) net.Listener {
	t.Helper()
	sc, err := ln.(syscall.Conn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	d, derr := -1, error(nil)
	if err := sc.Control(func(fd uintptr) { d, derr = unix.Dup(int(fd)) }); err != nil || derr != nil {
		t.Fatalf("dup listener: %v %v", err, derr)
	}
	f := os.NewFile(uintptr(d), "share-ln")
	defer f.Close()
	moved, err := net.FileListener(f)
	if err != nil {
		t.Fatal(err)
	}
	return moved
}

// Phase 9 (Pattern 7): the share listener moves to another WebServer, which
// keeps serving TLS on the same port after the first one stops.
func TestWebServer_StartOnPassedListener(t *testing.T) {
	tlsCfg, client := selfSignedTLSForTest(t)
	cfg := webserver.Config{BindIP: "127.0.0.1", FQDN: "127.0.0.1", TLSConfig: tlsCfg}
	ws1, err := webserver.NewWebServer(cfg, relay.NewHubManager())
	if err != nil {
		t.Fatal(err)
	}
	if err := ws1.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws1.Stop() })
	addr := ws1.Addr()
	if ws1.Listener() == nil {
		t.Fatal("Listener() is nil on a started server")
	}

	ws2, err := webserver.NewWebServer(cfg, relay.NewHubManager())
	if err != nil {
		t.Fatal(err)
	}
	if err := ws2.StartOn(dupListener(t, ws1.Listener())); err != nil {
		t.Fatalf("StartOn: %v", err)
	}
	t.Cleanup(func() { _ = ws2.Stop() })

	if err := ws1.Stop(); err != nil {
		t.Fatal(err)
	}
	if ws1.Listener() != nil {
		t.Error("Listener() is not nil after Stop")
	}
	if ws2.Addr() != addr {
		t.Fatalf("ws2 addr %s, want ws1's %s", ws2.Addr(), addr)
	}
	resp, err := client.Get("https://" + addr + "/dashboard")
	if err != nil {
		t.Fatalf("TLS GET after handover: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d after handover, want 200", resp.StatusCode)
	}
}

// Phase 9 (UPGR-04): /info reports the share server's API version; /meta
// keeps its array shape.
func TestSessionInfo_ReportsAPIVersion(t *testing.T) {
	for _, api := range []int{2, 0} {
		tlsCfg, client := selfSignedTLSForTest(t)
		ws, err := webserver.NewWebServer(webserver.Config{BindIP: "127.0.0.1", FQDN: "127.0.0.1", TLSConfig: tlsCfg, APIVersion: api}, relay.NewHubManager())
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = ws.Stop() })
		ws.SetSigningKey(ssExtTestKey)
		ws.SetWhoIsClientForTest(webserver.SameUserWhoIsForTest())
		ws.SetSessionResolver(func(id string) (string, string, string, bool) {
			return "S", "running", "h", id == "sess1"
		})
		ws.EnableSession("sess1")
		token := capForSession(t, ws, "sess1")

		resp, err := client.Get(ws.BaseURL() + "/api/sessions/sess1/info?cap=" + token)
		if err != nil {
			t.Fatal(err)
		}
		var info map[string]any
		err = json.NewDecoder(resp.Body).Decode(&info)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("api=%d: /info status %d, decode %v", api, resp.StatusCode, err)
		}
		got, has := info["api"]
		if api == 0 && has {
			t.Errorf("api=0: /info has \"api\": %v, want it absent", got)
		}
		if api != 0 && got != float64(api) {
			t.Errorf("api=%d: /info \"api\" = %v", api, got)
		}

		resp, err = client.Get(ws.BaseURL() + "/api/sessions/meta")
		if err != nil {
			t.Fatal(err)
		}
		var meta []map[string]any
		err = json.NewDecoder(resp.Body).Decode(&meta)
		resp.Body.Close()
		if err != nil || len(meta) != 1 {
			t.Fatalf("api=%d: /meta is not a one-item array: %v %v", api, meta, err)
		}
	}
}
