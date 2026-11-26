package desktop_test

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
)

// TestPeer_AnswerHasOneHostCandidate: the SDP answer offers exactly one ICE
// candidate, a host candidate on the media socket, so candidate order cannot
// matter and nothing else is advertised.
func TestPeer_AnswerHasOneHostCandidate(t *testing.T) {
	env := newTracerServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, _, err := env.dial(ctx, env.token(t, "g1"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	writeJSON(ctx, t, conn, map[string]any{"type": "hello", "v": 1})
	readType(ctx, t, conn, "hello")
	lc := newLoopbackClient(t)
	writeJSON(ctx, t, conn, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	sdp := readType(ctx, t, conn, "answer")["sdp"].(string)

	// pion writes every candidate twice, as component 1 (RTP) and 2 (RTCP);
	// with rtcp-mux the second is ignored. So: one candidate, whatever the
	// component, and each of the two lines is a udp host candidate on the
	// media socket.
	want := "udp host 127.0.0.1 " + strconv.Itoa(env.mediaPort)
	seen := map[string]bool{}
	for _, l := range strings.Split(sdp, "\r\n") {
		if !strings.HasPrefix(l, "a=candidate:") {
			continue
		}
		f := strings.Fields(l) // a=candidate:<found> <comp> udp <prio> <ip> <port> typ host ...
		if len(f) < 8 || f[6] != "typ" {
			t.Fatalf("unparseable candidate line %q", l)
		}
		seen[strings.ToLower(f[2])+" "+f[7]+" "+f[4]+" "+f[5]] = true
	}
	if len(seen) != 1 || !seen[want] {
		t.Errorf("answer candidates %v, want exactly one: %q", seen, want)
	}
	if !strings.Contains(sdp, "a=ice-lite") {
		t.Error("answer lacks a=ice-lite")
	}
}

// recConn records every destination written through it.
type recConn struct {
	net.PacketConn
	mu    sync.Mutex
	dests []string
}

func (r *recConn) WriteTo(p []byte, a net.Addr) (int, error) {
	r.mu.Lock()
	r.dests = append(r.dests, a.String())
	r.mu.Unlock()
	return r.PacketConn.WriteTo(p, a)
}

func (r *recConn) destinations() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.dests...)
}

// TestPeer_NoStunTurnMdnsTraffic: through a whole session (join, media, leave)
// the media socket writes only to the viewer, never to a STUN or TURN server
// (3478, 19302) or the mDNS group (5353).
func TestPeer_NoStunTurnMdnsTraffic(t *testing.T) {
	rec := &recConn{}
	env := newTracerServer(t, func(e *tracerEnv, _ *desktop.Deps) {
		e.preStart = func(m *desktop.Manager) {
			m.WrapConnForTest(func(c net.PacketConn) net.PacketConn { rec.PacketConn = c; return rec })
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v := env.join(ctx, t, env.token(t, "g1"))
	requireKeyframe(t, "viewer", v.firstAU(t, 5*time.Second))
	time.Sleep(2 * time.Second) // 2 s of media
	_ = v.conn.Close(websocket.StatusNormalClosure, "")
	_ = v.lc.pc.Close()
	eventually(t, 2*time.Second, "viewers to reach 0", func() bool { return env.mgr.Viewers() == 0 })

	dests := rec.destinations()
	if len(dests) == 0 {
		t.Fatal("recorded no writes: the recording socket is not on the media path")
	}
	for _, d := range dests {
		host, port, err := net.SplitHostPort(d)
		if err != nil {
			t.Fatal(err)
		}
		switch port {
		case "3478", "19302", "5353":
			t.Fatalf("media socket wrote to port %s (%s)", port, d)
		}
		if host != "127.0.0.1" || d != dests[0] {
			t.Fatalf("media socket wrote to %s, want only the viewer's one address %s", d, dests[0])
		}
	}
}

// TestPeer_APIHasNoICEServersAndNoMDNS: the PeerConnection configuration has
// no ICE servers, and the engine is ICE-lite on UDP4 with mDNS off.
func TestPeer_APIHasNoICEServersAndNoMDNS(t *testing.T) {
	if n := len(desktop.PeerConfigForTest().ICEServers); n != 0 {
		t.Errorf("PeerConnection configuration has %d ICE servers, want 0", n)
	}
	s := desktop.EngineSummaryForTest()
	if s.MDNS != ice.MulticastDNSModeDisabled {
		t.Errorf("mDNS mode %v, want disabled", s.MDNS)
	}
	if len(s.NetworkTypes) != 1 || s.NetworkTypes[0] != webrtc.NetworkTypeUDP4 {
		t.Errorf("network types %v, want only udp4", s.NetworkTypes)
	}
	if !s.Lite {
		t.Error("engine is not ICE-lite")
	}
}
