package relay

import (
	"io"
	"strings"
	"testing"
)

// Phase 13 (URL-02): Hub.PortPrinted, the anti-scan check for the port grant.

// ppHub is a hub at cols x rows that has printed s.
func ppHub(t *testing.T, cols, rows int, s string) *Hub {
	t.Helper()
	hub := cwdHub(t, cols, rows)
	feedHubs(s, 4096, hub)
	return hub
}

func TestPortPrinted_PythonBanner(t *testing.T) {
	hub := ppHub(t, 100, 30, "Serving HTTP on :: port 8000 (http://[::]:8000/) ...\r\n")
	for port, want := range map[int]bool{8000: true, 8001: false, 80: false} {
		if got := hub.PortPrinted(port); got != want {
			t.Errorf("PortPrinted(%d) = %v, want %v", port, got, want)
		}
	}
}

func TestPortPrinted_RangeRefused(t *testing.T) {
	hub := ppHub(t, 100, 30, "http://localhost:0 http://localhost:-1 http://localhost:65536\r\n")
	for _, port := range []int{0, -1, 65536} {
		if hub.PortPrinted(port) {
			t.Errorf("PortPrinted(%d) = true", port)
		}
	}
}

func TestPortPrinted_NoOutput(t *testing.T) {
	hub := NewHub("pp", strings.NewReader(""), io.Discard, DefaultScrollbackBytes, nil)
	if _, ok := hub.pathSnapshot(); ok {
		t.Fatal("fixture: a fresh hub has a snapshot")
	}
	if hub.PortPrinted(8000) {
		t.Error("PortPrinted(8000) = true with no output")
	}
}
