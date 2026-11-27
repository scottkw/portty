package daemon

import (
	"net/url"
	"testing"

	"github.com/scottkw/portty/internal/capability"
)

func peerToken(t *testing.T, e *desktopEnv) (string, bool) {
	t.Helper()
	return desktopAdapter{e.api}.DesktopToken()
}

// The standing token is the cap query value of the Copy Link link: one signing
// site, so the peer token is always the Copy Link token.
func TestDesktopPeerToken_EqualsLinkCap(t *testing.T) {
	e := newDesktopEnv(t)
	on := e.mustOn(t)
	u, err := url.Parse(on.URL)
	if err != nil {
		t.Fatal(err)
	}
	want := u.Query().Get("cap")
	if want == "" {
		t.Fatalf("link %q has no cap", on.URL)
	}
	got, ok := peerToken(t, e)
	if !ok || got != want {
		t.Fatalf("DesktopToken = %q, %v; want the link's cap", got, ok)
	}
}

func TestDesktopPeerToken_OffIsFalse(t *testing.T) {
	e := newDesktopEnv(t)
	if tok, ok := peerToken(t, e); ok || tok != "" {
		t.Fatalf("before on: %q, %v; want none", tok, ok)
	}
	e.mustOn(t)
	if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
		t.Fatal(err)
	}
	if tok, ok := peerToken(t, e); ok || tok != "" {
		t.Fatalf("after off: %q, %v; want none", tok, ok)
	}
}

func TestDesktopPeerToken_RegenerateChangesIt(t *testing.T) {
	e := newDesktopEnv(t)
	e.mustOn(t)
	old, ok := peerToken(t, e)
	if !ok {
		t.Fatal("no token while on")
	}
	if err := e.client.RegenerateSigningKey(); err != nil {
		t.Fatalf("RegenerateSigningKey: %v", err)
	}
	fresh, ok := peerToken(t, e)
	if !ok || fresh == old {
		t.Fatalf("after regeneration: %q, %v; want a different token", fresh, ok)
	}
	key := e.api.CurrentSigningKey()
	if _, err := capability.VerifyDesktop(old, key); err == nil {
		t.Error("the old token still verifies under the new key")
	}
	if _, err := capability.VerifyDesktop(fresh, key); err != nil {
		t.Errorf("the new token does not verify: %v", err)
	}
}
