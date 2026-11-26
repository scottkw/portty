package desktop

import (
	"net"
	"net/netip"

	"github.com/pion/webrtc/v4"
)

// KeyframeCoalesce lets the external tests step a fake clock past the window.
const KeyframeCoalesce = keyframeCoalesce

// PeerConfigForTest returns the configuration every PeerConnection is built with.
func PeerConfigForTest() webrtc.Configuration { return peerConfig() }

// EngineSummary is what the SettingEngine was told, for assertions: pion's
// SettingEngine has no getters.
type EngineSummary = engineSummary

// EngineSummaryForTest returns the settings newSettingEngine applies.
func EngineSummaryForTest() EngineSummary {
	_, s := newSettingEngine(nil, netip.Addr{})
	return s
}

// WrapConnForTest puts f around the media socket before the mux uses it.
func (m *Manager) WrapConnForTest(f func(net.PacketConn) net.PacketConn) { m.wrapConn = f }
