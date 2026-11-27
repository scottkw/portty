package webserver

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"

	"github.com/scottkw/portty/internal/capability"
)

const guestShareID = "0123456789abcdef0123456789abcdef"

// fakeGuestBackend is a fakeDesktopBackend that also hosts a guest share: it
// records the identity each guest socket arrives with, then closes normally.
type fakeGuestBackend struct {
	fakeDesktopBackend
	share    string
	inactive atomic.Bool
	whoCh    chan PeerIdentity
}

func newFakeGuestBackend() *fakeGuestBackend {
	return &fakeGuestBackend{
		fakeDesktopBackend: fakeDesktopBackend{grant: "g1"},
		share:              guestShareID,
		whoCh:              make(chan PeerIdentity, 8),
	}
}

func (b *fakeGuestBackend) GuestShareID() (string, bool) { return b.share, !b.inactive.Load() }

func (b *fakeGuestBackend) ServeGuestViewer(_ context.Context, c *websocket.Conn, _ func() bool, who PeerIdentity) {
	b.whoCh <- who
	_ = c.Close(websocket.StatusNormalClosure, "")
}

// colleagueWhoIs answers every loopback caller as a second person on the same
// tailnet (user 8, with a profile), on a Mac whose own user is 42.
func colleagueWhoIs() *fakeWhoIs {
	return &fakeWhoIs{
		self: &ipnstate.Status{Self: &ipnstate.PeerStatus{UserID: testSelfUser}},
		byAddr: map[string]*apitype.WhoIsResponse{"127.0.0.1": {
			Node: &tailcfg.Node{User: 8, ComputedName: "pat-laptop", StableID: "nPAT1CNTRL"},
			UserProfile: &tailcfg.UserProfile{
				ID: 8, LoginName: "colleague@example.com", DisplayName: "Pat Colleague",
			},
		}},
	}
}

// guestEnv is a desktopEnv whose backend also hosts a guest share, answering
// every request as the colleague.
type guestEnv struct {
	*desktopEnv
	gb *fakeGuestBackend
}

func newGuestEnv(t *testing.T) *guestEnv {
	t.Helper()
	e := newDesktopEnv(t)
	gb := newFakeGuestBackend()
	e.ws.SetDesktopBackend(gb)
	e.ws.SetWhoIsClientForTest(colleagueWhoIs())
	return &guestEnv{desktopEnv: e, gb: gb}
}

func guestToken(t *testing.T, shareID string) string {
	t.Helper()
	tok, err := capability.SignDesktopGuest(capability.DesktopGuestClaims{ShareID: shareID, IAT: time.Now().Unix()}, capTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// TestGuestRoutes_TracerWS: a person of another account reaches the guest
// socket with their WhoIs identity bound to the connection, is refused on the
// always-available socket, and a share that is not active kills the link
// before any upgrade.
func TestGuestRoutes_TracerWS(t *testing.T) {
	e := newGuestEnv(t)
	tok := guestToken(t, guestShareID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "wss" + e.ws.BaseURL()[len("https"):] + "/desktop/guest/ws?cap=" + tok
	conn, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient: e.client,
		HTTPHeader: http.Header{"Origin": {e.ws.BaseURL()}},
	})
	if err != nil {
		t.Fatalf("guest dial: %v (resp %v)", err, resp)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("guest dial status %d, want 101", resp.StatusCode)
	}
	select {
	case who := <-e.gb.whoCh:
		want := PeerIdentity{
			UserID: 8, LoginName: "colleague@example.com", DisplayName: "Pat Colleague",
			NodeName: "pat-laptop", StableID: "nPAT1CNTRL", IP: "127.0.0.1",
		}
		if who != want {
			t.Errorf("backend identity %+v, want %+v", who, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backend never saw the guest")
	}

	// The same person is another account on every other family.
	r := e.upgrade(t, "/desktop/ws?cap="+desktopToken(t, "g1"))
	if r.StatusCode != http.StatusForbidden || r.Header.Get("X-Portty") != "1" || r.Header.Get("Upgrade") != "" {
		t.Errorf("colleague on /desktop/ws: %d marker %q, want 403 with the marker", r.StatusCode, r.Header.Get("X-Portty"))
	}

	// No active share: 410 before any upgrade, and the backend sees nobody new.
	e.gb.inactive.Store(true)
	r = e.upgrade(t, "/desktop/guest/ws?cap="+tok)
	if r.StatusCode != http.StatusGone || r.Header.Get("Upgrade") != "" {
		t.Errorf("guest ws with no share: %d, want 410 and no upgrade", r.StatusCode)
	}
	select {
	case who := <-e.gb.whoCh:
		t.Errorf("backend saw a second guest %+v from a refused request", who)
	default:
	}
}
