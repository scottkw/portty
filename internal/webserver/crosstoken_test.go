package webserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// TestCrossToken_Matrix is DSHR-08 by test: three token kinds, all signed with
// the same signing key, against every route family. Only the matching kind
// opens a route; every other cell is refused (status 400 or above, so never a
// 2xx, a redirect to a page or a 101). The matching cells are asserted too, so a
// route that stopped working would not pass as "refused".
func TestCrossToken_Matrix(t *testing.T) {
	e := newDesktopEnv(t)

	// A real upstream on this Mac's loopback for the port route's matching cell.
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(up.Close)
	u, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	e.pp.print(t, port)

	tokens := map[string]string{
		"session": e.pp.capTok,
		"port":    portToken(t, ppSID, port, writeGrantID, capTestKey),
		"desktop": desktopToken(t, "g1"),
	}

	sess := "/sessions/" + ppSID
	query := func(path string) func(string) int {
		return func(tok string) int {
			resp, _ := e.get(t, path+"?cap="+tok)
			return resp.StatusCode
		}
	}
	socket := func(path string) func(string) int {
		return func(tok string) int { return e.upgrade(t, path+"?cap="+tok).StatusCode }
	}
	// The port proxy takes a session capability in X-Portty-Cap and a port token
	// in X-Portty-Port-Token; the cell under test is the port-token slot, with a
	// good write capability beside it.
	portProxy := func(tok string) int {
		req, err := http.NewRequest(http.MethodGet, e.ws.BaseURL()+"/api/peer/sessions/"+ppSID+"/ports/"+strconv.Itoa(port), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Portty-Cap", e.pp.capTok)
		req.Header.Set("X-Portty-Port-Token", tok)
		req.Header.Set("X-Portty-Target", "/x")
		resp, err := e.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	routes := []struct {
		name  string
		kind  string // the one token kind that opens it
		route func(tok string) int
	}{
		{"GET /desktop/", "desktop", query("/desktop/")},
		{"GET /desktop/ws", "desktop", socket("/desktop/ws")},
		{"GET /desktop/check", "desktop", query("/desktop/check")},
		{"GET /sessions/{id}", "session", query(sess)},
		{"GET /sessions/{id}/ws", "session", socket(sess + "/ws")},
		{"GET /api/sessions", "session", query("/api/sessions")},
		{"GET /api/sessions/{id}/info", "session", query("/api/sessions/" + ppSID + "/info")},
		{"GET /api/plugin-config", "session", query("/api/plugin-config")},
		{"GET /api/peer/sessions/{id}/ports/{port}", "port", portProxy},
	}
	for _, r := range routes {
		for kind, tok := range tokens {
			t.Run(r.name+" with a "+kind+" token", func(t *testing.T) {
				status := r.route(tok)
				if kind == r.kind {
					if status >= 400 {
						t.Errorf("matching %s token refused: status %d", kind, status)
					}
					return
				}
				if status < 400 {
					t.Errorf("%s token accepted on a %s route: status %d", kind, r.kind, status)
				}
			})
		}
	}

	// The port route's other slot: neither a port token nor a desktop token is a
	// write capability, whatever sits in the port-token slot.
	for _, kind := range []string{"port", "desktop"} {
		t.Run("port route with a "+kind+" token as the capability", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, e.ws.BaseURL()+"/api/peer/sessions/"+ppSID+"/ports/"+strconv.Itoa(port), nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-Portty-Cap", tokens[kind])
			req.Header.Set("X-Portty-Port-Token", tokens["port"])
			req.Header.Set("X-Portty-Target", "/x")
			resp, err := e.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode < 400 {
				t.Errorf("%s token accepted as the capability: status %d", kind, resp.StatusCode)
			}
		})
	}
}
