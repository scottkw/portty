package webserver

import (
	"net/http"
	"testing"

	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// gateCase is what a gated route answers for one identity: other accounts get
// 403 with the X-Portty marker; a tailscaled fault gets 503 with no marker
// (MOB-05, WR-08). The same user gets the route's own answer, tested per route.
type gateCase struct {
	name   string
	id     webservertest.Identity
	status int
	marker bool
}

var gateRefusals = []gateCase{
	{"other", webservertest.Other, http.StatusForbidden, true},
	{"tagged", webservertest.Tagged, http.StatusForbidden, true},
	{"shared-in", webservertest.SharedIn, http.StatusForbidden, true},
	{"not-found", webservertest.NotFound, http.StatusForbidden, true},
	{"status-fault", webservertest.StatusFault, http.StatusServiceUnavailable, false},
	{"whois-fault", webservertest.WhoIsFault, http.StatusServiceUnavailable, false},
}

func checkGateAnswer(t *testing.T, resp *http.Response, c gateCase) {
	t.Helper()
	if resp.StatusCode != c.status {
		t.Errorf("%s: status = %d, want %d", c.name, resp.StatusCode, c.status)
	}
	if got := resp.Header.Get("X-Portty") == "1"; got != c.marker {
		t.Errorf("%s: X-Portty marker = %v, want %v", c.name, got, c.marker)
	}
}

func TestSameUserGate_SessionInfo(t *testing.T) {
	const sid = "sess-gate-info"
	ws, client, _, _ := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	tok := issueCapFor(t, ws, sid, "read,write")
	url := ws.BaseURL() + "/api/sessions/" + sid + "/info?cap="

	get := func(t *testing.T, id webservertest.Identity, cap string) *http.Response {
		t.Helper()
		ws.SetWhoIsClientForTest(webservertest.New(id))
		resp, err := client.Get(url + cap)
		if err != nil {
			t.Fatalf("GET info: %v", err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	t.Run("same", func(t *testing.T) {
		if resp := get(t, webservertest.Same, tok); resp.StatusCode != http.StatusOK {
			t.Errorf("same user: status = %d, want 200", resp.StatusCode)
		}
	})
	for _, c := range gateRefusals {
		t.Run(c.name, func(t *testing.T) { checkGateAnswer(t, get(t, c.id, tok), c) })
	}
	// The gate runs before the capability: another account with no cap at all
	// still gets the marker 403, not the capability 401.
	t.Run("other-no-cap", func(t *testing.T) {
		checkGateAnswer(t, get(t, webservertest.Other, ""), gateCase{"other-no-cap", 0, http.StatusForbidden, true})
	})
}
