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

// TestBrandPages_DashboardPointsToLinks: the dashboard has no entry form; it
// tells the visitor to open a session from its link (UI-SPEC Web pages).
func TestBrandPages_DashboardPointsToLinks(t *testing.T) {
	body := getPage(t, "/dashboard")
	if !strings.Contains(body, "To open a session here, use its link. Copy it from the Share button on the Mac that runs the session.") {
		t.Error("/dashboard body missing the link paragraph")
	}
	for _, gone := range []string{"<form", "<input", "<script"} {
		if strings.Contains(body, gone) {
			t.Errorf("/dashboard body still contains %q", gone)
		}
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
	for _, name := range []string{"dashboard.html", "terminal.html"} {
		b, err := webfs.WebFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if loc := agent.FindIndex(b); loc != nil {
			t.Errorf("%s contains agent wording at byte %d", name, loc[0])
		}
	}
}
