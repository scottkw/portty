//go:build vmwhois

// VM-only fake for the MOB-05 same-account refusal check in the Tart VMs,
// which sign in to one Tailscale account. Never in a release build: the
// vmwhois tag must not appear in .github, build.sh or scripts
// (tests/check-brand.sh enforces this in CI). The real tailnet share server
// is kept; only the WhoIs answer changes, chosen per request by the file
// $HOME/p14/whois-mode: "other" (also a missing or empty file), "pass" or
// "fault". Any other content means "fault" (fail closed). This Mac's own node
// always passes unchanged, so the host can still open its own link.

package daemon

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
)

// vmWhoIsInner is the method set the share server asks of its WhoIs client.
type vmWhoIsInner interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
	StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error)
}

// vmWhoIs answers like inner, except as the mode file says.
type vmWhoIs struct{ inner vmWhoIsInner }

// vmWhoIsMode reads the mode file on every call.
func vmWhoIsMode() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "fault"
	}
	b, err := os.ReadFile(filepath.Join(home, "p14", "whois-mode"))
	if err != nil && !os.IsNotExist(err) {
		return "fault"
	}
	switch m := strings.TrimSpace(string(b)); m {
	case "", "other":
		return "other"
	case "pass", "fault":
		return m
	}
	return "fault"
}

// Any non-nil error reads to the share server as a tailscaled fault.
var errVMWhoIsFault = errors.New("vmwhois: simulated tailscaled fault")

func (w vmWhoIs) StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error) {
	if vmWhoIsMode() == "fault" {
		slog.Debug("vmwhois: status fault")
		return nil, errVMWhoIsFault
	}
	return w.inner.StatusWithoutPeers(ctx)
}

func (w vmWhoIs) WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
	mode := vmWhoIsMode()
	if mode == "fault" {
		slog.Debug("vmwhois: whois fault")
		return nil, errVMWhoIsFault
	}
	who, err := w.inner.WhoIs(ctx, remoteAddr)
	if mode == "pass" || err != nil || who == nil || who.Node == nil {
		return who, err
	}
	st, err := w.inner.StatusWithoutPeers(ctx)
	if err != nil || st == nil || st.Self == nil {
		return nil, errVMWhoIsFault
	}
	if who.Node.StableID == st.Self.ID {
		slog.Debug("vmwhois: own node passes", "user", int64(who.Node.User))
		return who, nil
	}
	// Copies: never mutate what the client returned.
	node := *who.Node
	node.User = st.Self.UserID + 1000
	out := *who
	out.Node = &node
	slog.Debug("vmwhois: peer reported as another user", "user", int64(node.User), "self_user", int64(st.Self.UserID))
	return &out, nil
}

func init() {
	prev := defaultNewShareServer
	defaultNewShareServer = func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		ws, err := prev(h, m)
		if err != nil {
			return nil, err
		}
		ws.SetWhoIsClientForTest(vmWhoIs{inner: &local.Client{}})
		return ws, nil
	}
}
