package webserver

import (
	"crypto/tls"
	"net"
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
