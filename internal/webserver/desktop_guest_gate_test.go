package webserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/views"

	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// peerGateRun sends one request from remote through a requireTailnetPeer (or
// ...Page) wrapper around a handler that records the identity it was given.
// The wrappers read only ws.whois, so a bare WebServer is enough.
func peerGateRun(t *testing.T, who whoIsClient, page bool, remote string) (*httptest.ResponseRecorder, *PeerIdentity) {
	t.Helper()
	ws := &WebServer{whois: who}
	var got *PeerIdentity
	next := func(w http.ResponseWriter, r *http.Request) {
		if id, ok := peerIdentityFrom(r.Context()); ok {
			got = &id
		}
		w.WriteHeader(http.StatusOK)
	}
	h := ws.requireTailnetPeer(next)
	if page {
		h = ws.requireTailnetPeerPage(next)
	}
	req := httptest.NewRequest(http.MethodGet, "/desktop/guest/", nil)
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec, got
}

func TestTailnetPeer_Admits(t *testing.T) {
	for name, id := range map[string]webservertest.Identity{
		"same":              webservertest.Same,
		"other":             webservertest.Other,
		"colleague":         webservertest.Colleague,
		"other-second-node": webservertest.OtherSecondNode,
		"other-bidi":        webservertest.OtherBidi,
	} {
		for _, page := range []bool{false, true} {
			rec, got := peerGateRun(t, webservertest.New(id), page, "192.0.2.9:41000")
			if rec.Code != http.StatusOK {
				t.Errorf("%s page=%v: status %d, want 200", name, page, rec.Code)
				continue
			}
			if rec.Header().Get("X-Portty") != "" {
				t.Errorf("%s page=%v: an admitted request carries the marker", name, page)
			}
			if page {
				continue // the page wrapper stores no identity
			}
			d := webservertest.DetailsOf(id)
			want := PeerIdentity{
				UserID: int64(d.UserID), LoginName: d.LoginName, DisplayName: d.DisplayName,
				NodeName: d.NodeName, StableID: d.StableID, IP: "192.0.2.9",
			}
			if got == nil || *got != want {
				t.Errorf("%s: identity %+v, want %+v", name, got, want)
			}
		}
	}
}

func TestTailnetPeer_Refuses(t *testing.T) {
	refused := map[string]webservertest.Identity{
		"tagged":            webservertest.Tagged,
		"shared-in":         webservertest.SharedIn,
		"other-nil-profile": webservertest.OtherNilProfile, // the Admit prompt could not name the person
		"not-found":         webservertest.NotFound,
		"self-tagged":       webservertest.SelfTagged,
		"self-shared-in":    webservertest.SelfSharedIn,
	}
	for name, id := range refused {
		for _, page := range []bool{false, true} {
			rec, got := peerGateRun(t, webservertest.New(id), page, "192.0.2.9:41000")
			if rec.Code != http.StatusForbidden || rec.Header().Get("X-Portty") != "1" || got != nil {
				t.Errorf("%s page=%v: status %d marker %q reached=%v, want 403 with the marker and no handler",
					name, page, rec.Code, rec.Header().Get("X-Portty"), got != nil)
			}
			if page && !bytes.Equal(rec.Body.Bytes(), desktopGuestRefusedPage) {
				t.Errorf("%s: page body is not desktop-guest-refused.html", name)
			}
		}
	}
	for name, id := range map[string]webservertest.Identity{
		"status-fault": webservertest.StatusFault,
		"whois-fault":  webservertest.WhoIsFault,
	} {
		for _, page := range []bool{false, true} {
			rec, got := peerGateRun(t, webservertest.New(id), page, "192.0.2.9:41000")
			if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("X-Portty") != "" || got != nil {
				t.Errorf("%s page=%v: status %d marker %q, want 503 with no marker", name, page, rec.Code, rec.Header().Get("X-Portty"))
			}
			if page && !bytes.Equal(rec.Body.Bytes(), serviceRetryPage) {
				t.Errorf("%s: page body is not service-retry.html", name)
			}
		}
	}
}

// TestTailnetPeer_TwoHosts: one server, two guests at once, each with its own
// WhoIs answer; an unset host falls back to the current identity.
func TestTailnetPeer_TwoHosts(t *testing.T) {
	f := webservertest.New(webservertest.Other)
	f.SetFor("192.0.2.2", webservertest.Colleague)
	f.SetFor("192.0.2.3", webservertest.Tagged)

	_, got := peerGateRun(t, f, false, "192.0.2.2:1")
	if got == nil || got.LoginName != webservertest.DetailsOf(webservertest.Colleague).LoginName {
		t.Errorf("host .2 identity %+v, want the colleague", got)
	}
	_, got = peerGateRun(t, f, false, "192.0.2.9:1")
	if got == nil || got.LoginName != webservertest.DetailsOf(webservertest.Other).LoginName {
		t.Errorf("unset host identity %+v, want the current (other) identity", got)
	}
	if rec, got := peerGateRun(t, f, false, "192.0.2.3:1"); rec.Code != http.StatusForbidden || got != nil {
		t.Errorf("tagged host: status %d, want 403", rec.Code)
	}
}

// TestTailnetPeer_Unit pins tailnetPeer's refusals directly, including the
// zero user ID and each missing piece of data.
func TestTailnetPeer_Unit(t *testing.T) {
	tags := views.SliceOf([]string{"tag:server"})
	self := func() *ipnstate.PeerStatus { return &ipnstate.PeerStatus{UserID: testSelfUser} }
	prof := &tailcfg.UserProfile{ID: 8, LoginName: "colleague@example.com", DisplayName: "Pat"}
	who := func(n *tailcfg.Node, p *tailcfg.UserProfile) *apitype.WhoIsResponse {
		return &apitype.WhoIsResponse{Node: n, UserProfile: p}
	}
	cases := []struct {
		name string
		who  *apitype.WhoIsResponse
		self *ipnstate.PeerStatus
		want bool
	}{
		{"other user with profile", who(&tailcfg.Node{User: 8}, prof), self(), true},
		{"same user with profile", who(&tailcfg.Node{User: testSelfUser}, prof), self(), true},
		{"nil who", nil, self(), false},
		{"nil node", who(nil, prof), self(), false},
		{"nil profile", who(&tailcfg.Node{User: 8}, nil), self(), false},
		{"nil self", who(&tailcfg.Node{User: 8}, prof), nil, false},
		{"tagged peer", who(&tailcfg.Node{User: 8, Tags: []string{"tag:ci"}}, prof), self(), false},
		{"shared-in peer", who(&tailcfg.Node{User: 8, Sharer: 9}, prof), self(), false},
		{"zero user", who(&tailcfg.Node{User: 0}, prof), self(), false},
		{"tagged self", who(&tailcfg.Node{User: 8}, prof), &ipnstate.PeerStatus{UserID: testSelfUser, Tags: &tags}, false},
		{"shared-in self", who(&tailcfg.Node{User: 8}, prof), &ipnstate.PeerStatus{UserID: testSelfUser, AltSharerUserID: 9}, false},
		{"zero self user", who(&tailcfg.Node{User: 8}, prof), &ipnstate.PeerStatus{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, got := tailnetPeer(c.who, c.self); got != c.want {
				t.Errorf("tailnetPeer = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTailnetPeer_RefusedPageIsStatic: the refused page carries the UI-SPEC
// wording and nothing from a request: no script, no link but the stylesheet, no
// name.
func TestTailnetPeer_RefusedPageIsStatic(t *testing.T) {
	s := string(desktopGuestRefusedPage)
	if !strings.Contains(s, "This device can't open this link") {
		t.Error("refused page lacks the UI-SPEC heading")
	}
	if strings.Contains(strings.ToLower(s), "<script") || strings.Count(s, "href=") != 1 {
		t.Error("refused page has a script or a link besides the stylesheet")
	}
	if strings.Contains(s, "@example.com") || strings.Contains(s, ".ts.net") {
		t.Error("refused page names a person or a host")
	}
}
