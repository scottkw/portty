// Package webservertest holds the WhoIs fake that tests, the Playwright
// fixture and VM checks install on a share server. Release code never imports
// it (Plan 14-06 adds the guard). It imports only tailscale.com types, never
// internal/webserver, so internal webserver tests can use it too.
package webservertest

import (
	"context"
	"fmt"
	"net"
	"sync"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/views"
)

// SelfUser is the Tailscale user ID of the fake's own Mac.
const SelfUser tailcfg.UserID = 42

// otherUser is the user ID of the Other identity.
const otherUser tailcfg.UserID = 7

// colleagueUser is the user ID of the Colleague identity, a second person.
const colleagueUser tailcfg.UserID = 8

// Identity is who every request appears to come from.
type Identity int

const (
	Same            Identity = iota // a node of this Mac's user
	Other                           // a node of another user
	Tagged                          // same user, but a tagged node
	SharedIn                        // same user ID, shared in from another tailnet
	NotFound                        // WhoIs finds no such peer
	StatusFault                     // StatusWithoutPeers fails
	WhoIsFault                      // WhoIs fails with a non-not-found error
	Colleague                       // a node of a second other user, with a profile
	OtherNilProfile                 // another user, but WhoIs carries no UserProfile
	OtherSecondNode                 // the Other user on a second device
	OtherBidi                       // the Other user with hostile control characters in every name
	SelfTagged                      // this Mac is tagged; WhoIs answers as Other
	SelfSharedIn                    // this Mac is shared in; WhoIs answers as Other
)

var names = map[string]Identity{
	"same":              Same,
	"other":             Other,
	"tagged":            Tagged,
	"shared-in":         SharedIn,
	"not-found":         NotFound,
	"status-fault":      StatusFault,
	"whois-fault":       WhoIsFault,
	"colleague":         Colleague,
	"other-nil-profile": OtherNilProfile,
	"other-second-node": OtherSecondNode,
	"other-bidi":        OtherBidi,
	"self-tagged":       SelfTagged,
	"self-shared-in":    SelfSharedIn,
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

// Details is what the fake says about the person and device behind an identity
// (placeholders on example.com). Tests compare what a gate hands on with it.
type Details struct {
	UserID      tailcfg.UserID
	LoginName   string
	DisplayName string
	NodeName    string
	StableID    string
}

var (
	selfDetails      = Details{SelfUser, "me@example.com", "Me Myself", "my-other-mac", "nOWN1CNTRL"}
	otherDetails     = Details{otherUser, "other@example.com", "Olive Other", "olive-laptop", "nOLV1CNTRL"}
	secondDetails    = Details{otherUser, "other@example.com", "Olive Other", "olive-desktop", "nOLV2CNTRL"}
	colleagueDetails = Details{colleagueUser, "colleague@example.com", "Pat Colleague", "pat-laptop", "nPAT1CNTRL"}
	bidiDetails      = Details{
		otherUser,
		"ol\x07ive@example.com",
		"Ol‮iv⁦e​ Ot\nher\tx",
		"olive-bidi",
		"nOLV3CNTRL",
	}
)

// DetailsOf returns what WhoIs says for id. The zero Details means id carries
// no answer (NotFound and the two fault identities).
func DetailsOf(id Identity) Details {
	switch id {
	case Same, Tagged, SharedIn:
		return selfDetails
	case Other, OtherNilProfile, SelfTagged, SelfSharedIn:
		return otherDetails
	case OtherSecondNode:
		return secondDetails
	case Colleague:
		return colleagueDetails
	case OtherBidi:
		return bidiDetails
	}
	return Details{}
}

// WhoIs answers each caller with one identity, chosen by the caller's remote
// host when SetFor named it, else the current identity. The mutex lets the
// fixture switch identity while serving.
type WhoIs struct {
	mu     sync.Mutex
	id     Identity
	byHost map[string]Identity
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

// SetFor makes requests from host (the host part of the remote address, such as
// 127.0.0.1) answer as id, so one server can model two people at once. The
// current identity answers every other host.
func (f *WhoIs) SetFor(host string, id Identity) {
	f.mu.Lock()
	if f.byHost == nil {
		f.byHost = map[string]Identity{}
	}
	f.byHost[host] = id
	f.mu.Unlock()
}

func (f *WhoIs) current() Identity {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.id
}

func (f *WhoIs) forRemote(remoteAddr string) Identity {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.byHost[host]; ok {
		return id
	}
	return f.id
}

// StatusWithoutPeers returns a status whose Self is this Mac's user, tagged or
// shared in for SelfTagged and SelfSharedIn.
func (f *WhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	self := &ipnstate.PeerStatus{UserID: SelfUser}
	switch f.current() {
	case StatusFault:
		return nil, fmt.Errorf("webservertest: status fault")
	case SelfTagged:
		tags := views.SliceOf([]string{"tag:server"})
		self.Tags = &tags
	case SelfSharedIn:
		self.AltSharerUserID = 9
	}
	return &ipnstate.Status{Self: self}, nil
}

// WhoIs answers for the caller according to its identity.
func (f *WhoIs) WhoIs(_ context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
	id := f.forRemote(remoteAddr)
	d := DetailsOf(id)
	node := &tailcfg.Node{
		ID:           tailcfg.NodeID(1000 + d.UserID),
		StableID:     tailcfg.StableNodeID(d.StableID),
		ComputedName: d.NodeName,
		User:         d.UserID,
	}
	profile := &tailcfg.UserProfile{ID: d.UserID, LoginName: d.LoginName, DisplayName: d.DisplayName}
	switch id {
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
		node.User = SelfUser
	case OtherNilProfile:
		profile = nil
	}
	return &apitype.WhoIsResponse{Node: node, UserProfile: profile}, nil
}
