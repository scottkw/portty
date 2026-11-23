//go:build vmwhois

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

// fakeInner is the real client's stand-in: this Mac is node "self", user 42;
// every other address is node "peer", also user 42.
type fakeInner struct{}

func (fakeInner) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	return &ipnstate.Status{Self: &ipnstate.PeerStatus{ID: "self", UserID: 42}}, nil
}

func (fakeInner) WhoIs(_ context.Context, addr string) (*apitype.WhoIsResponse, error) {
	id := tailcfg.StableNodeID("peer")
	if addr == "self" {
		id = "self"
	}
	return &apitype.WhoIsResponse{Node: &tailcfg.Node{StableID: id, User: 42}}, nil
}

func TestVMWhoIs_Modes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	w := vmWhoIs{inner: fakeInner{}}
	ctx := context.Background()

	setMode := func(content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(home, "p14"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "p14", "whois-mode"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	user := func(addr string) tailcfg.UserID {
		t.Helper()
		who, err := w.WhoIs(ctx, addr)
		if err != nil {
			t.Fatalf("WhoIs(%s): %v", addr, err)
		}
		return who.Node.User
	}
	bothFault := func() {
		t.Helper()
		if _, err := w.WhoIs(ctx, "peer"); err == nil {
			t.Fatal("WhoIs: want error")
		}
		if _, err := w.WhoIs(ctx, "self"); err == nil {
			t.Fatal("WhoIs self: want error")
		}
		if _, err := w.StatusWithoutPeers(ctx); err == nil {
			t.Fatal("StatusWithoutPeers: want error")
		}
	}

	// Missing file means other: the peer is another user, this Mac's node passes.
	if got := user("peer"); got != 1042 {
		t.Fatalf("missing file: peer user = %d, want 1042", got)
	}
	if got := user("self"); got != 42 {
		t.Fatalf("missing file: self user = %d, want 42", got)
	}
	if st, err := w.StatusWithoutPeers(ctx); err != nil || st.Self.UserID != 42 {
		t.Fatalf("other: status = %v, %v", st, err)
	}

	setMode("") // an empty file also means other
	if got := user("peer"); got != 1042 {
		t.Fatalf("empty file: peer user = %d, want 1042", got)
	}

	setMode("pass\n")
	if got := user("peer"); got != 42 {
		t.Fatalf("pass: peer user = %d, want 42", got)
	}

	setMode("fault")
	bothFault()

	setMode("bogus") // unknown content fails closed
	bothFault()

	// The wrapper copies; the inner client's answers are never mutated.
	in := &apitype.WhoIsResponse{Node: &tailcfg.Node{StableID: "peer", User: 42}}
	setMode("other")
	out, err := vmWhoIs{inner: fixedInner{in}}.WhoIs(ctx, "peer")
	if err != nil || out.Node.User != 1042 {
		t.Fatalf("copy case: %v, %v", out, err)
	}
	if in.Node.User != 42 || out == in || out.Node == in.Node {
		t.Fatal("the inner response was mutated or aliased")
	}
}

// fixedInner returns one fixed WhoIs response, to check it is not mutated.
type fixedInner struct{ who *apitype.WhoIsResponse }

func (f fixedInner) StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error) {
	return fakeInner{}.StatusWithoutPeers(ctx)
}

func (f fixedInner) WhoIs(context.Context, string) (*apitype.WhoIsResponse, error) {
	return f.who, nil
}
