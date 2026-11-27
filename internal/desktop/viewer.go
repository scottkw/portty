package desktop

import (
	"time"

	"github.com/pion/webrtc/v4"
)

// ViewerID names one connected viewer for the life of its signalling socket.
type ViewerID string

// Rights is what a viewer may do beyond watching, as a bit set. A viewer
// reached through the same-user /desktop/ route (the own-account viewer) gets
// RightsOwnAccount. Guests of a later phase start at RightsViewOnly and gain
// bits at runtime: rights are daemon runtime state, never token claims.
type Rights uint8

// RightsViewOnly is no right at all: watching only.
const RightsViewOnly Rights = 0

// The rights bits, as announced to a viewer in the hello's rights map.
const (
	RightsControl   Rights = 1 << 0 // pointer, keyboard and text input
	RightsClipboard Rights = 1 << 1 // copy from and paste to the Mac
	RightsSurface   Rights = 1 << 2 // choose which display or window is shared

	// RightsOwnAccount is everything: own-account viewers have full control
	// with no grant step.
	RightsOwnAccount = RightsControl | RightsClipboard | RightsSurface
)

// Viewer is the identity and rights record the Manager keeps per connected
// viewer. Later phases read it through Manager.ViewerList.
type Viewer struct {
	ID       ViewerID
	Rights   Rights
	JoinedAt time.Time
}

// DataChannelHook receives a data channel the viewer created before its
// offer. Contract: never block inside OnMessage. pion calls it inline on the
// SCTP read loop, so a slow handler stalls that viewer's channel (and its
// siblings). Phase 19 registers no OnMessage at all.
type DataChannelHook func(v Viewer, dc *webrtc.DataChannel)

// CloseDataChannel is the Phase 19 default hook: the link is view-only, so a
// channel a viewer opens is closed at once.
func CloseDataChannel(_ Viewer, dc *webrtc.DataChannel) {
	_ = dc.Close()
}
