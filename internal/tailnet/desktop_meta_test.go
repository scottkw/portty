package tailnet

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestFetchPeerDesktopCap_Statuses(t *testing.T) {
	const secret = "SECRETTOKEN"
	cases := []struct {
		name    string
		status  int
		body    string
		want    string // token on success
		wantErr error  // matched with errors.Is
		wantMsg string // exact text for the generic error
	}{
		{name: "ok", status: http.StatusOK, body: `{"cap":"T"}`, want: "T"},
		{name: "empty cap", status: http.StatusOK, body: `{"cap":""}`, wantMsg: "peer desktop cap: empty capability"},
		{name: "forbidden", status: http.StatusForbidden, body: secret, wantErr: ErrPeerForbidden},
		{name: "off", status: http.StatusNotFound, body: secret, wantErr: ErrPeerDesktopOff},
		{name: "server error", status: http.StatusInternalServerError, body: secret, wantMsg: "peer desktop cap: unexpected status 500"},
		{name: "oversize body", status: http.StatusOK, body: `{"cap":"` + strings.Repeat("a", 70<<10) + `"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := metaStub(t, c.status, nil, c.body)
			got, err := FetchPeerDesktopCap(context.Background(), srv.URL, srv.Client())
			if c.want != "" {
				if err != nil || got != c.want {
					t.Fatalf("got %q, %v; want %q", got, err, c.want)
				}
				return
			}
			if err == nil || got != "" {
				t.Fatalf("got %q, %v; want an error and no token", got, err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v, want %v", err, c.wantErr)
			}
			if c.wantMsg != "" && err.Error() != c.wantMsg {
				t.Errorf("err = %q, want %q", err, c.wantMsg)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "aaaa") {
				t.Errorf("error %q carries response content", err)
			}
		})
	}
}

func TestFetchPeerDesktopCap_Request(t *testing.T) {
	var method, path, ct, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, ct = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = w.Write([]byte(`{"cap":"T"}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := FetchPeerDesktopCap(context.Background(), srv.URL, srv.Client()); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/api/peer/desktop/cap" || ct != "application/json" || body != "{}" {
		t.Errorf("request = %s %s %q %q; want POST /api/peer/desktop/cap application/json {}", method, path, ct, body)
	}
}

// A transport error wraps its cause and says nothing else.
func TestFetchPeerDesktopCap_TransportError(t *testing.T) {
	srv := metaStub(t, http.StatusOK, nil, `{"cap":"T"}`)
	client := srv.Client()
	srv.Close()
	_, err := FetchPeerDesktopCap(context.Background(), srv.URL, client)
	if err == nil || !strings.HasPrefix(err.Error(), "peer desktop cap: ") {
		t.Fatalf("err = %v, want a wrapped transport error", err)
	}
	if errors.Is(err, ErrPeerForbidden) || errors.Is(err, ErrPeerDesktopOff) {
		t.Errorf("a transport failure was read as a refusal: %v", err)
	}
}
