// Package webservertest holds the WhoIs fake that tests, the Playwright
// fixture and VM checks install on a share server. Release code never imports
// it (Plan 14-06 adds the guard). It imports only tailscale.com types, never
// internal/webserver, so internal webserver tests can use it too.
package webservertest

import (
	"context"
	"fmt"
	"sync"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

// SelfUser is the Tailscale user ID of the fake's own Mac.
const SelfUser tailcfg.UserID = 42

// otherUser is the user ID of the Other identity.
const otherUser tailcfg.UserID = 7

// Identity is who every request appears to come from.
type Identity int

const (
	Same        Identity = iota // a node of this Mac's user
	Other                       // a node of another user
	Tagged                      // same user, but a tagged node
	SharedIn                    // same user ID, shared in from another tailnet
	NotFound                    // WhoIs finds no such peer
	StatusFault                 // StatusWithoutPeers fails
	WhoIsFault                  // WhoIs fails with a non-not-found error
)

var names = map[string]Identity{
	"same":         Same,
	"other":        Other,
	"tagged":       Tagged,
	"shared-in":    SharedIn,
	"not-found":    NotFound,
	"status-fault": StatusFault,
	"whois-fault":  WhoIsFault,
}

// ParseIdentity maps an identity name to its Identity. An unknown name is an
// error, never a default.
func ParseIdentity(s string) (Identity, error) {
	id, ok := names[s]
	if !ok {
		return 0, fmt.Errorf("webservertest: unknown identity %q", s)
	}
	return id, nil
}

// WhoIs answers every caller with one identity. The mutex lets the fixture
// switch identity while serving.
type WhoIs struct {
	mu sync.Mutex
	id Identity
}

// New returns a fake answering as id.
func New(id Identity) *WhoIs { return &WhoIs{id: id} }

// SameUser returns a fake under which every request comes from this Mac's own user.
func SameUser() *WhoIs { return New(Same) }

// Set switches the identity for later requests.
func (f *WhoIs) Set(id Identity) {
	f.mu.Lock()
	f.id = id
	f.mu.Unlock()
}

func (f *WhoIs) current() Identity {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.id
}

// StatusWithoutPeers returns a status whose Self is this Mac's user.
func (f *WhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	if f.current() == StatusFault {
		return nil, fmt.Errorf("webservertest: status fault")
	}
	return &ipnstate.Status{Self: &ipnstate.PeerStatus{UserID: SelfUser}}, nil
}

// WhoIs answers for the caller according to the current identity.
func (f *WhoIs) WhoIs(context.Context, string) (*apitype.WhoIsResponse, error) {
	node := &tailcfg.Node{User: SelfUser}
	switch f.current() {
	case Same:
	case Other:
		node.User = otherUser
	case Tagged:
		node.Tags = []string{"tag:ci"}
	case SharedIn:
		node.Sharer = 9
	case NotFound:
		return nil, local.ErrPeerNotFound
	case WhoIsFault:
		return nil, fmt.Errorf("webservertest: whois fault")
	case StatusFault:
		// Status fails first in the gate; WhoIs is never reached in practice.
	}
	return &apitype.WhoIsResponse{Node: node}, nil
}
