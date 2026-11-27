package webserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// forgeGuest signs an arbitrary payload under the real guest key, so a test can
// present claims SignDesktopGuest would never produce (another kind, v 2, a bad
// share ID).
func forgeGuest(t *testing.T, payload string) string {
	t.Helper()
	key, err := capability.DesktopGuestKey(capTestKey)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// TestGuestCrossToken_TokenLayer is DSHR-03/08 at the token layer: the guest
// key is its own derived key, so no other family's token verifies as a guest
// token and a guest token verifies nowhere else.
func TestGuestCrossToken_TokenLayer(t *testing.T) {
	guest := guestToken(t, guestShareID)
	foreign := map[string]string{
		"session capability": sessionCapToken(t, "s1"),
		"port token":         portToken(t, ppSID, 3000, writeGrantID, capTestKey),
		"always desktop":     desktopToken(t, "g1"),
	}
	for name, tok := range foreign {
		if _, err := capability.VerifyDesktopGuest(tok, capTestKey); err == nil {
			t.Errorf("VerifyDesktopGuest accepted a %s", name)
		}
	}
	if _, err := capability.VerifyDesktopGuest(guest, capTestKey); err != nil {
		t.Fatalf("VerifyDesktopGuest refused its own token: %v", err)
	}
	if _, err := capability.Verify(guest, capTestKey); err == nil {
		t.Error("Verify accepted a guest token")
	}
	if _, err := capability.VerifyPort(guest, capTestKey); err == nil {
		t.Error("VerifyPort accepted a guest token")
	}
	if _, err := capability.VerifyDesktop(guest, capTestKey); err == nil {
		t.Error("VerifyDesktop accepted a guest token")
	}

	// Claims marshal in the fixed order kind, share_id, iat, v, with no expiry.
	payload, err := base64.RawURLEncoding.DecodeString(strings.SplitN(guest, ".", 2)[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(payload), `{"kind":"desktop-guest","share_id":"`+guestShareID+`","iat":`) || !strings.HasSuffix(string(payload), `,"v":1}`) {
		t.Errorf("claims %s, want the order kind, share_id, iat, v", payload)
	}
	if strings.Contains(string(payload), `"exp"`) {
		t.Errorf("claims %s carry an expiry", payload)
	}

	// SignDesktopGuest sets Kind and V itself.
	odd, err := capability.SignDesktopGuest(capability.DesktopGuestClaims{Kind: "desktop", ShareID: guestShareID, IAT: 1, V: 9}, capTestKey)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := capability.VerifyDesktopGuest(odd, capTestKey); err != nil || c.Kind != capability.DesktopGuestTokenKind || c.V != 1 {
		t.Errorf("SignDesktopGuest left caller Kind/V: %+v, %v", c, err)
	}
	if _, err := capability.SignDesktopGuest(capability.DesktopGuestClaims{ShareID: guestShareID}, nil); err == nil {
		t.Error("SignDesktopGuest signed under an empty key")
	}

	// Forged claims under the real key.
	sid := `"share_id":"` + guestShareID + `"`
	for name, tc := range map[string]struct {
		payload string
		want    error
	}{
		"wrong kind":      {`{"kind":"desktop",` + sid + `,"iat":1,"v":1}`, capability.ErrNotDesktopGuestToken},
		"v 2":             {`{"kind":"desktop-guest",` + sid + `,"iat":1,"v":2}`, capability.ErrNotDesktopGuestToken},
		"short share id":  {`{"kind":"desktop-guest","share_id":"abc","iat":1,"v":1}`, capability.ErrMalformedClaims},
		"upper share id":  {`{"kind":"desktop-guest","share_id":"0123456789ABCDEF0123456789abcdef","iat":1,"v":1}`, capability.ErrMalformedClaims},
		"non-hex id":      {`{"kind":"desktop-guest","share_id":"0123456789abcdef0123456789abcdeg","iat":1,"v":1}`, capability.ErrMalformedClaims},
		"missing id":      {`{"kind":"desktop-guest","iat":1,"v":1}`, capability.ErrMalformedClaims},
		"not json claims": {`[1,2]`, capability.ErrMalformedClaims},
	} {
		if _, err := capability.VerifyDesktopGuest(forgeGuest(t, tc.payload), capTestKey); !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", name, err, tc.want)
		}
	}
	for name, tok := range map[string]string{"empty": "", "one segment": "abc", "bad base64": "!!.!!"} {
		if _, err := capability.VerifyDesktopGuest(tok, capTestKey); !errors.Is(err, capability.ErrMalformedToken) {
			t.Errorf("%s: err %v, want ErrMalformedToken", name, err)
		}
	}
}

// TestGuestCrossToken_Matrix is the research matrix by test, with every
// accepting cell asserted so a dead route cannot pass as "refused". The account
// is this Mac's own (admitted on the guest family too), so only the token
// decides each cell. A cell is refused when it answers 400 or above.
func TestGuestCrossToken_Matrix(t *testing.T) {
	e := newGuestEnv(t)
	e.setIdentity(webservertest.Same)

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
		"guest":   guestToken(t, guestShareID),
		"always":  desktopToken(t, "g1"),
		"session": e.pp.capTok,
		"port":    portToken(t, ppSID, port, writeGrantID, capTestKey),
	}
	query := func(path string) func(string) int {
		return func(tok string) int {
			resp, _ := e.get(t, path+"?cap="+tok)
			return resp.StatusCode
		}
	}
	socket := func(path string) func(string) int {
		return func(tok string) int { return e.upgrade(t, path+"?cap="+tok).StatusCode }
	}
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

	sess := "/sessions/" + ppSID
	routes := []struct {
		name  string
		kind  string // the one token kind that opens it
		route func(tok string) int
	}{
		{"GET /desktop/guest/", "guest", query("/desktop/guest/")},
		{"GET /desktop/guest/ws", "guest", socket("/desktop/guest/ws")},
		{"GET /desktop/guest/check", "guest", query("/desktop/guest/check")},
		{"GET /desktop/", "always", query("/desktop/")},
		{"GET /desktop/ws", "always", socket("/desktop/ws")},
		{"GET /desktop/check", "always", query("/desktop/check")},
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

	// The guest token's other lives, on every family: a stopped share, an older
	// share and a token from before Regenerate Signing Key are refused.
	live := tokens["guest"]
	older := guestToken(t, "ffffffffffffffffffffffffffffffff")
	for _, r := range routes {
		if r.kind != "guest" {
			continue
		}
		e.gb.inactive.Store(true)
		if status := r.route(live); status < 400 {
			t.Errorf("%s: stopped share's token accepted: %d", r.name, status)
		}
		e.gb.inactive.Store(false)
		if status := r.route(older); status < 400 {
			t.Errorf("%s: older share's token accepted: %d", r.name, status)
		}
		e.ws.SetSigningKey([]byte("regenerated-signing-key-32-bytes!"))
		if status := r.route(live); status < 400 {
			t.Errorf("%s: pre-regenerate token accepted: %d", r.name, status)
		}
		e.ws.SetSigningKey(capTestKey)
		if status := r.route(live); status >= 400 {
			t.Errorf("%s: live token refused after the key was restored: %d", r.name, status)
		}
	}

	// No token and another account: the guest page and check answer the token
	// refusal (the identity gate passed), every other family the marker 403.
	e.setIdentity(webservertest.Other)
	if status := query("/desktop/guest/check")(""); status != http.StatusUnauthorized {
		t.Errorf("guest check with no token, other account: %d, want 401", status)
	}
	for _, r := range routes {
		if r.kind == "guest" {
			continue
		}
		if status := r.route(""); status != http.StatusForbidden {
			t.Errorf("%s with no token, other account: %d, want 403", r.name, status)
		}
	}
}
