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

func TestHasURLPort_Table(t *testing.T) {
	for _, c := range []struct {
		name, line string
		port       int
		want       bool
	}{
		{"python banner", "(http://[::]:8000/)", 8000, true},
		{"bare", "http://localhost:8000", 8000, true},
		{"sentence full stop", "http://localhost:8000.", 8000, true},
		{"path and query", "see http://127.0.0.1:8000/x?y=1", 8000, true},
		{"quoted ipv6", `"http://[::1]:3000"`, 3000, true},
		{"assignment and semicolon", "url=http://0.0.0.0:8000;", 8000, true},
		{"shorter port in longer", "http://localhost:8000/", 80, false},
		{"glued prefix", "xhttp://localhost:8000", 8000, false},
		{"suffix domain", "http://localhost:8000.evil.com", 8000, false},
		{"userinfo evil.com", "http://localhost:8000@evil.com/", 8000, false},
		{"second colon", "http://localhost:8000:12", 8000, false},
		{"https", "https://localhost:8000", 8000, false},
		{"upper case host", "http://LOCALHOST:8000", 8000, false},
		{"leading zero", "http://localhost:08000", 8000, false},
		{"short ipv4", "http://127.1:8000", 8000, false},
		{"dash suffix", "http://localhost:8000-x", 8000, false},
		{"refused then accepted", "http://localhost:8000@evil.com/ http://localhost:8000/", 8000, true},
		{"refused only twice", "xhttp://localhost:8000 http://localhost:8000.evil.com", 8000, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := hasURLPort(c.line, c.port); got != c.want {
				t.Errorf("hasURLPort(%q, %d) = %v, want %v", c.line, c.port, got, c.want)
			}
		})
	}
}

func TestHasURLPort_Banners(t *testing.T) {
	for _, c := range []struct {
		line string
		port int
	}{
		{"Serving HTTP on :: port 8000 (http://[::]:8000/) ...", 8000},
		{"  ➜  Local:   http://localhost:5173/", 5173},
		{"   - Local:        http://localhost:3000", 3000},
		{"Starting development server at http://127.0.0.1:8000/", 8000},
		{"* Listening on http://127.0.0.1:3000", 3000},
		{"* Listening on http://[::1]:3000", 3000},
		{"INFO:     Uvicorn running on http://127.0.0.1:8000 (Press CTRL+C to quit)", 8000},
		{"Loopback: http://localhost:8080/, http://[::1]:8080/", 8080},
	} {
		if !hasURLPort(c.line, c.port) {
			t.Errorf("banner %q: port %d not found", c.line, c.port)
		}
	}
}

func TestPortPrinted_History(t *testing.T) {
	hub := ppHub(t, 100, 24, "Local: http://localhost:5173/\r\n"+fillerLines(200))
	hub.emuMu.Lock()
	n := hub.history.Len()
	hub.emuMu.Unlock()
	if n == 0 {
		t.Fatal("fixture: nothing scrolled into history")
	}
	if !hub.PortPrinted(5173) {
		t.Error("URL in history not found")
	}
}

func TestPortPrinted_SoftWrap(t *testing.T) {
	url := "http://localhost:8000/abcdefghijkl" // 34 characters
	hub := ppHub(t, 20, 24, url+"\r\n")
	if !hub.PortPrinted(8000) {
		t.Error("URL soft-wrapped over two rows not found")
	}
	hub = ppHub(t, 20, 24, "http://localhost:80\r\n00/\r\n")
	if hub.PortPrinted(8000) {
		t.Error("URL split by a hard break found")
	}
}

func TestPortPrinted_ANSIPort(t *testing.T) {
	hub := ppHub(t, 100, 24, "  Local:   http://localhost:\x1b[1m5173\x1b[22m/\r\n")
	if !hub.PortPrinted(5173) {
		t.Error("ANSI-styled port not found")
	}
}

func TestPortPrinted_AltScreen(t *testing.T) {
	hub := ppHub(t, 100, 24, "\x1b[?1049h\x1b[HServing on http://127.0.0.1:9000/\r\n")
	if !hub.PortPrinted(9000) {
		t.Error("URL on a full-screen program's screen not found")
	}
}

func TestPortPrinted_Busy(t *testing.T) {
	hub := ppHub(t, 100, 24, "http://localhost:8000\r\n")
	hub.portPrintedBusy.Store(true)
	if hub.PortPrinted(8000) {
		t.Error("a busy hub answered true")
	}
	hub.portPrintedBusy.Store(false)
	if !hub.PortPrinted(8000) {
		t.Error("an idle hub answered false")
	}
}
