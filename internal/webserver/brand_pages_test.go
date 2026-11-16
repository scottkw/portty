package webserver

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	webfs "github.com/scottkw/portty/web"
)

// getPage requests a live route, checks 200 and the CSP header, and returns the body.
func getPage(t *testing.T, path string) string {
	t.Helper()
	ws, client := testServer(t)
	resp, err := client.Get(ws.BaseURL() + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", path, resp.StatusCode)
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Errorf("GET %s: Content-Security-Policy header missing", path)
	}
	return string(body)
}

func TestBrandPages_DashboardSaysPortty(t *testing.T) {
	body := getPage(t, "/dashboard")
	for _, want := range []string{
		"<title>Portty</title>",
		"<h1>Portty</h1>",
		"Terminal sessions shared over your tailnet.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/dashboard body missing %q", want)
		}
	}
}

func TestBrandPages_JoinSaysPortty(t *testing.T) {
	body := getPage(t, "/join")
	if !strings.Contains(body, "<title>Join Session — Portty</title>") {
		t.Error("/join body missing em-dash title")
	}
	if n := strings.Count(body, "<h1>Portty</h1>"); n != 5 {
		t.Errorf("/join has %d <h1>Portty</h1>, want 5 (one per state)", n)
	}
	if !strings.Contains(body, "This session is no longer available. The shell may have exited, or the owner stopped sharing it.") {
		t.Error("/join body missing Session Ended copy")
	}
}

func TestBrandPages_TerminalPageSaysPortty(t *testing.T) {
	b, err := webfs.WebFS.ReadFile("terminal.html")
	if err != nil {
		t.Fatalf("read terminal.html: %v", err)
	}
	if !strings.Contains(string(b), "<title>Portty</title>") {
		t.Error("terminal.html missing <title>Portty</title>")
	}
}

func TestBrandPages_NoAgentWording(t *testing.T) {
	agent := regexp.MustCompile(`(?i)agent`)
	for _, name := range []string{"dashboard.html", "join.html", "terminal.html"} {
		b, err := webfs.WebFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if loc := agent.FindIndex(b); loc != nil {
			t.Errorf("%s contains agent wording at byte %d", name, loc[0])
		}
	}
}
