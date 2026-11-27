package tailnet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// metaStub answers every GET with status, the given headers and body.
func metaStub(t *testing.T, status int, headers map[string]string, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Only the exact value "always" (spaces trimmed) sets Desktop; a different case, a
// list or a later value is ignored, never misread.
func TestDesktopMeta_HeaderValues(t *testing.T) {
	cases := []struct {
		name   string
		header map[string]string
		want   bool
	}{
		{"always", map[string]string{DesktopHeader: "always"}, true},
		{"always with spaces", map[string]string{DesktopHeader: " always "}, true},
		{"absent", nil, false},
		{"upper case", map[string]string{DesktopHeader: "ALWAYS"}, false},
		{"list", map[string]string{DesktopHeader: "always,admit"}, false},
		{"later value", map[string]string{DesktopHeader: "admit"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := metaStub(t, http.StatusOK, c.header, `[{"id":"s1","name":"n","status":"running","url":"u"}]`)
			m, err := doFetchSessionsMeta(context.Background(), srv.URL, srv.Client(), "")
			if err != nil || m.Status != MetaOK || len(m.Sessions) != 1 {
				t.Fatalf("doFetchSessionsMeta = %+v, %v; want OK with one session", m, err)
			}
			if m.Desktop != c.want {
				t.Errorf("Desktop = %v, want %v", m.Desktop, c.want)
			}
		})
	}
}

// The forbidden branch is read first: a 403 with the marker never yields Desktop,
// whatever header rides along.
func TestDesktopMeta_ForbiddenIgnoresHeader(t *testing.T) {
	srv := metaStub(t, http.StatusForbidden, map[string]string{MarkerHeader: MarkerValue, DesktopHeader: DesktopAlways}, "")
	m, err := doFetchSessionsMeta(context.Background(), srv.URL, srv.Client(), "")
	if err != nil || m.Status != MetaForbidden || m.Desktop {
		t.Fatalf("got %+v, %v; want Forbidden with Desktop false", m, err)
	}
}

// An empty array with the header: Desktop true and zero sessions (non-nil).
func TestDesktopMeta_EmptyListWithHeader(t *testing.T) {
	srv := metaStub(t, http.StatusOK, map[string]string{DesktopHeader: DesktopAlways}, `[]`)
	m, err := FetchPeerMetaWithClient(context.Background(), srv.URL, srv.Client())
	if err != nil || m.Status != MetaOK || !m.Desktop || m.Sessions == nil || len(m.Sessions) != 0 {
		t.Fatalf("got %+v, %v; want OK, Desktop, empty non-nil list", m, err)
	}
	// The old wrapper keeps its shape and ignores the flag.
	sessions, status, err := FetchPeerSessionsMetaWithClient(context.Background(), srv.URL, srv.Client())
	if err != nil || status != MetaOK || sessions == nil || len(sessions) != 0 {
		t.Fatalf("wrapper got %v, %v, %v", sessions, status, err)
	}
}

// The fan-out carries Desktop into the group and still drops a forbidden peer.
func TestDesktopMeta_FanOutCarriesDesktop(t *testing.T) {
	fetch := func(_ context.Context, p Peer) PeerMeta {
		switch p.Hostname {
		case "office":
			return PeerMeta{Status: MetaOK, Desktop: true}
		case "plain":
			return PeerMeta{Status: MetaOK, Sessions: []ShareableSessionMeta{{ID: "s1"}}}
		case "other-account":
			return PeerMeta{Status: MetaForbidden, Desktop: true}
		}
		return PeerMeta{Status: MetaUnreachable}
	}
	groups := fetchAllPeerMeta(context.Background(), []Peer{{Hostname: "office"}, {Hostname: "plain"}, {Hostname: "other-account"}, {Hostname: "asleep"}}, fetch)
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3 (forbidden dropped): %+v", len(groups), groups)
	}
	want := map[string]bool{"asleep": false, "office": true, "plain": false}
	for _, g := range groups {
		if g.Desktop != want[g.Hostname] {
			t.Errorf("%s: Desktop = %v, want %v", g.Hostname, g.Desktop, want[g.Hostname])
		}
		if g.Sessions == nil {
			t.Errorf("%s: Sessions is nil, want a non-nil list", g.Hostname)
		}
	}
}
