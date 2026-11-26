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
	"github.com/pion/webrtc/v4"
)

// h264Fmtp is the one negotiated H.264 configuration: constrained-baseline
// level 3.1, packetization mode 1. The encoder (19-05) and the fixture SPS
// both carry profile-level-id 42e01f.
const h264Fmtp = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"

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

// newAPI builds the pion API every viewer's PeerConnection shares: ICE-lite on
// the one UDP mux, UDP4 only, no mDNS, only the bind address as a candidate,
// and exactly one video codec.
func newAPI(mux ice.UDPMux, bind netip.Addr) (*webrtc.API, error) {
	se := webrtc.SettingEngine{}
	se.SetICEUDPMux(mux)
	se.SetLite(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetIPFilter(func(ip net.IP) bool { return ip.Equal(bind.AsSlice()) })
	if allowLoopbackForTest.Load() {
		se.SetIncludeLoopbackCandidate(true)
	}

	me := &webrtc.MediaEngine{}
	if err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: h264Fmtp},
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

// answer builds one viewer's PeerConnection with the shared track and returns
// it with the complete (non-trickle) answer SDP. Every error closes the
// PeerConnection. The caller owns the returned one.
func (m *Manager) answer(ctx context.Context, offerSDP string) (*webrtc.PeerConnection, string, error) {
	m.mu.Lock()
	api := m.api
	m.mu.Unlock()
	if api == nil {
		return nil, "", errors.New("desktop: manager not started")
	}
	// ICEServers stays empty: no STUN, no TURN, media never leaves the tailnet.
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, "", fmt.Errorf("desktop: new peer connection: %w", err)
	}
	fail := func(err error) (*webrtc.PeerConnection, string, error) {
		_ = pc.Close()
		return nil, "", err
	}
	sender, err := pc.AddTrack(m.track)
	if err != nil {
		return fail(fmt.Errorf("desktop: add track: %w", err))
	}
	// 19-03 reads PLI and FIR from here; until then RTCP is only drained so
	// the interceptors keep running.
	go func() {
		defer recoverLog()
		buf := make([]byte, 1500)
		for {
			if _, _, err := sender.Read(buf); err != nil {
				return
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
