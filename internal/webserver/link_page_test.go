package webserver

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

const linkPageCopy = "This link was turned off or is incomplete. Copy a fresh link from the Share button on the Mac that runs the session."

func TestLinkPage_FailedOpensShowPage(t *testing.T) {
	const id = "link-page-sess"
	ws, client, _, _ := testServerWithHub(t, id)
	ws.SetSigningKey(capTestKey)
	cleared := issueCapFor(t, ws, id, "read,write")
	ws.ClearGrants(id)

	cases := []struct {
		name, query string
		want        int
	}{
		{"bad signature", "?cap=not-a-token", http.StatusUnauthorized},
		{"cleared grant", "?cap=" + cleared, http.StatusForbidden},
		{"missing cap", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := client.Get(ws.BaseURL() + "/sessions/" + id + c.query)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			body := string(b)
			if resp.StatusCode != c.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, c.want)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", ct)
			}
			for _, want := range []string{"<h1>Portty</h1>", "Link No Longer Works", linkPageCopy} {
				if !strings.Contains(body, want) {
					t.Fatalf("page lacks %q", want)
				}
			}
			if resp.Header.Get("Content-Security-Policy") == "" {
				t.Fatal("page has no Content-Security-Policy")
			}
			if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
				t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
			}
			// testServerWithHub's resolver names the session "Security Review Session" on review-host.
			for _, leak := range []string{id, "Security Review Session", "review-host"} {
				if strings.Contains(body, leak) {
					t.Fatalf("page leaks %q", leak)
				}
			}
		})
	}
}

// API routes keep their plain-text failures.
func TestLinkPage_APIStaysPlainText(t *testing.T) {
	const id = "link-page-api"
	ws, client, _, _ := testServerWithHub(t, id)
	ws.SetSigningKey(capTestKey)
	resp, err := client.Get(ws.BaseURL() + "/api/sessions/" + id + "/info?cap=not-a-token")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUnauthorized || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("info with bad cap = %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if strings.Contains(string(b), "Link No Longer Works") {
		t.Fatal("API route served the HTML page")
	}
}
