package desktop

import (
	"time"

	"github.com/pion/webrtc/v4"
)

// ViewerID names one connected viewer for the life of its signalling socket.
type ViewerID string

// Rights is what a viewer may do beyond watching. Phase 19 only has view-only;
// Phase 20 adds input rights behind a host-side check, so the field exists
// before any code consults it.
type Rights uint8

// RightsViewOnly is the only right Phase 19 grants.
const RightsViewOnly Rights = 0

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
