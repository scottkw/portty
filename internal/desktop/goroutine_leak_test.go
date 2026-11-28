package desktop_test

import (
	"bytes"
	"context"
	"net"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestFanout_JoinLeaveCyclesLeaveNoGoroutines: viewer joins and leaves (audio,
// input channel, a keyframe each time) bring the goroutine count back to where
// it was after the first one, whether the viewer closes cleanly, drops its
// socket, or its media path goes silent first (a relay that vanished). 23-09
// finding 4 blamed join/leave for the daemon's growth; it was the health poller
// (tailnet.LocalAPI), and this pins that a join/leave leaves nothing.
func TestFanout_JoinLeaveCyclesLeaveNoGoroutines(t *testing.T) {
	gone := func(t *testing.T, env *tracerEnv) {
		eventually(t, 10*time.Second, "viewer gone", func() bool { return env.mgr.Viewers() == 0 })
	}
	modes := []struct {
		name  string
		leave func(t *testing.T, env *tracerEnv, v *fanViewer, h *hole)
	}{
		{"clean", func(t *testing.T, env *tracerEnv, v *fanViewer, _ *hole) {
			_ = v.lc.pc.Close()
			_ = v.conn.Close(websocket.StatusNormalClosure, "")
			gone(t, env)
		}},
		{"socket_dropped", func(t *testing.T, env *tracerEnv, v *fanViewer, _ *hole) {
			_ = v.conn.CloseNow()
			gone(t, env)
			_ = v.lc.pc.Close()
		}},
		{"media_silent_then_socket", func(t *testing.T, env *tracerEnv, v *fanViewer, h *hole) {
			h.drop.Store(true)
			time.Sleep(500 * time.Millisecond)
			_ = v.conn.CloseNow()
			gone(t, env)
			_ = v.lc.pc.Close()
		}},
	}
	for _, md := range modes {
		t.Run(md.name, func(t *testing.T) {
			var (
				mu      sync.Mutex
				sources []*audioFake
			)
			env := newTracerServer(t, withAudioSource(&sources, &mu))
			tok := env.token(t, "g1")
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			cycle := func() {
				h := newHole(t, env.mediaPort)
				defer h.close()
				v := env.joinCaps(ctx, t, tok, map[string]int{"video": 1, "audio": 1, "input": 1}, wantAudio, func(lc *loopbackClient) {
					if _, err := lc.pc.CreateDataChannel("input", nil); err != nil {
						t.Fatal(err)
					}
					lc.answerMunge = h.pointAt
				})
				requireKeyframe(t, "viewer", v.firstAU(t, 10*time.Second))
				md.leave(t, env, v, h)
				<-v.closed
			}
			cycle() // one-time goroutines start here
			base := settledGoroutines(func(prev, n int) bool { return n == prev }, 5*time.Second)
			for range 3 {
				cycle()
			}
			if n := settledGoroutines(func(_, n int) bool { return n <= base }, 15*time.Second); n > base {
				var b bytes.Buffer
				_ = pprof.Lookup("goroutine").WriteTo(&b, 1)
				t.Fatalf("goroutines: %d after the first join/leave, %d after 3 more\n%s", base, n, b.String())
			}
		})
	}
}

// settledGoroutines samples the count every 100 ms until done(previous, now)
// holds or d passes, and returns the last sample.
func settledGoroutines(done func(prev, n int) bool, d time.Duration) int {
	n := runtime.NumGoroutine()
	for deadline := time.Now().Add(d); time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
		prev := n
		n = runtime.NumGoroutine()
		if done(prev, n) {
			break
		}
	}
	return n
}

// hole forwards one viewer's media to the daemon until drop is set, then
// discards both directions without a word.
type hole struct {
	front, back *net.UDPConn
	drop        atomic.Bool
	done        sync.WaitGroup
}

func newHole(t *testing.T, daemonPort int) *hole {
	t.Helper()
	lo := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	front, err := net.ListenUDP("udp4", lo)
	if err != nil {
		t.Fatal(err)
	}
	back, err := net.ListenUDP("udp4", lo)
	if err != nil {
		t.Fatal(err)
	}
	h := &hole{front: front, back: back}
	daemon := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: daemonPort}
	var client atomic.Pointer[net.UDPAddr]
	pump := func(from, to *net.UDPConn, dst func() *net.UDPAddr, src func(*net.UDPAddr)) {
		defer h.done.Done()
		buf := make([]byte, 2048)
		for {
			n, a, err := from.ReadFromUDP(buf)
			if err != nil {
				return
			}
			src(a)
			if d := dst(); d != nil && !h.drop.Load() {
				_, _ = to.WriteToUDP(buf[:n], d)
			}
		}
	}
	h.done.Add(2)
	go pump(front, back, func() *net.UDPAddr { return daemon }, func(a *net.UDPAddr) { client.Store(a) })
	go pump(back, front, client.Load, func(*net.UDPAddr) {})
	return h
}

// pointAt rewrites the daemon's candidate in an answer to the hole's port.
func (h *hole) pointAt(answer string) string {
	return candidateRe.ReplaceAllString(answer, "${1}"+strconv.Itoa(h.front.LocalAddr().(*net.UDPAddr).Port)+"${2}")
}

func (h *hole) close() {
	_ = h.front.Close()
	_ = h.back.Close()
	h.done.Wait()
}
