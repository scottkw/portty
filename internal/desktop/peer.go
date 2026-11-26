package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

// h264Fmtp is the one negotiated H.264 configuration: constrained-baseline
// level 3.1, packetization mode 1. The encoder (19-05) and the fixture SPS
// both carry profile-level-id 42e01f.
const h264Fmtp = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"

// h264Capability is the one codec every viewer track offers.
var h264Capability = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: h264Fmtp}

// peerConfig is the one PeerConnection configuration. ICEServers stays empty:
// no STUN, no TURN, media never leaves the tailnet.
func peerConfig() webrtc.Configuration { return webrtc.Configuration{} }

// answerGatherTimeout bounds ICE gathering for the non-trickle answer. An
// ICE-lite answer carries one host candidate, so this is a backstop.
const answerGatherTimeout = 5 * time.Second

// allowLoopbackForTest lets pion offer loopback candidates. Production never
// sets it, so a loopback bind yields no usable candidate.
var allowLoopbackForTest atomic.Bool

// AllowLoopbackForTest lets the in-process loopback tests exchange media over
// 127.0.0.1. It panics outside a test binary.
func AllowLoopbackForTest() {
	if !testing.Testing() {
		panic("desktop: AllowLoopbackForTest outside a test binary")
	}
	allowLoopbackForTest.Store(true)
}

// engineSummary records what newSettingEngine applied, because pion's
// SettingEngine has no getters and the tests assert on these.
type engineSummary struct {
	Lite         bool
	NetworkTypes []webrtc.NetworkType
	MDNS         ice.MulticastDNSMode
}

// newSettingEngine is the one place the media path is hardened (T-19-10):
// ICE-lite on the one UDP mux, UDP4 only, no mDNS, and the bind address as the
// only candidate.
func newSettingEngine(mux ice.UDPMux, bind netip.Addr) (webrtc.SettingEngine, engineSummary) {
	sum := engineSummary{Lite: true, NetworkTypes: []webrtc.NetworkType{webrtc.NetworkTypeUDP4}, MDNS: ice.MulticastDNSModeDisabled}
	se := webrtc.SettingEngine{}
	se.SetICEUDPMux(mux)
	se.SetLite(sum.Lite)
	se.SetNetworkTypes(sum.NetworkTypes)
	se.SetICEMulticastDNSMode(sum.MDNS)
	se.SetIPFilter(func(ip net.IP) bool { return ip.Equal(bind.AsSlice()) })
	if allowLoopbackForTest.Load() {
		se.SetIncludeLoopbackCandidate(true)
	}
	return se, sum
}

// newAPI builds the pion API every viewer's PeerConnection shares, with exactly
// one video codec.
func newAPI(mux ice.UDPMux, bind netip.Addr) (*webrtc.API, error) {
	se, _ := newSettingEngine(mux, bind)

	me := &webrtc.MediaEngine{}
	if err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: h264Capability,
		PayloadType:        102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("desktop: register h264: %w", err)
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return nil, fmt.Errorf("desktop: interceptors: %w", err)
	}
	return webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir)), nil
}

// answer builds viewer v's PeerConnection and its own outbound track, and
// returns the PeerConnection with the complete (non-trickle) answer SDP. Every
// error closes the PeerConnection. The caller owns the returned one.
func (m *Manager) answer(ctx context.Context, v *viewer, offerSDP string) (*webrtc.PeerConnection, string, error) {
	m.mu.Lock()
	api := m.api
	m.mu.Unlock()
	if api == nil {
		return nil, "", errors.New("desktop: manager not started")
	}
	pc, err := api.NewPeerConnection(peerConfig())
	if err != nil {
		return nil, "", fmt.Errorf("desktop: new peer connection: %w", err)
	}
	fail := func(err error) (*webrtc.PeerConnection, string, error) {
		_ = pc.Close()
		return nil, "", err
	}
	// One track per viewer, fed from the one shared packetizer (fanOut), so a
	// joining viewer receives nothing until its own first keyframe.
	track, err := webrtc.NewTrackLocalStaticRTP(h264Capability, "video", "portty-desktop")
	if err != nil {
		return fail(fmt.Errorf("desktop: viewer track: %w", err))
	}
	sender, err := pc.AddTrack(track)
	if err != nil {
		return fail(fmt.Errorf("desktop: add track: %w", err))
	}
	v.track = track
	// PLI and FIR from the viewer ask for a keyframe; the loop also keeps the
	// interceptors reading. It ends when the sender is closed.
	go func() {
		defer recoverLog()
		for {
			pkts, _, err := sender.ReadRTCP()
			if err != nil {
				return
			}
			for _, p := range pkts {
				switch p.(type) {
				case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
					m.requestKeyframe("pli")
				}
			}
		}
	}()
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}); err != nil {
		return fail(fmt.Errorf("desktop: set remote description: %w", err))
	}
	ans, err := pc.CreateAnswer(nil)
	if err != nil {
		return fail(fmt.Errorf("desktop: create answer: %w", err))
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(ans); err != nil {
		return fail(fmt.Errorf("desktop: set local description: %w", err))
	}
	select {
	case <-gathered:
	case <-time.After(answerGatherTimeout):
		return fail(errors.New("desktop: ice gathering timed out"))
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	return pc, pc.LocalDescription().SDP, nil
}
