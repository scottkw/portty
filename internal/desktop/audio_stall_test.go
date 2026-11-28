package desktop_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// stallAudio is a rig source with an audio half the test drives by hand: a
// packet only when the test sends one, so it can go silent with no error the
// way an SCK stream does after coreaudiod restarts (A-03). RestartAudio counts.
type stallAudio struct {
	*fakeSource
	pkts     chan screencap.AudioPacket
	errs     chan error
	restarts atomic.Int32
}

func (a *stallAudio) AudioPackets() <-chan screencap.AudioPacket { return a.pkts }
func (a *stallAudio) AudioErrors() <-chan error                  { return a.errs }
func (a *stallAudio) RestartAudio() error                        { a.restarts.Add(1); return nil }

// packet sends one Opus silence packet.
func (a *stallAudio) packet() {
	select {
	case a.pkts <- screencap.AudioPacket{Data: opusSilence, Samples: screencap.OpusFrames}:
	default:
	}
}

// withStallAudio wraps the rig's sources (src.opt() runs first) in stallAudio
// and records each one.
func withStallAudio(out *[]*stallAudio, mu *sync.Mutex) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) {
		inner := d.NewSource
		d.NewSource = func() screencap.Source {
			a := &stallAudio{fakeSource: inner().(*fakeSource), pkts: make(chan screencap.AudioPacket, 64), errs: make(chan error, 4)}
			mu.Lock()
			*out = append(*out, a)
			mu.Unlock()
			return a
		}
	}
}

func newStallRig(t *testing.T) (*rig, *fanViewer, *stallAudio) {
	t.Helper()
	var (
		mu  sync.Mutex
		got []*stallAudio
	)
	r := newRig(t, withStallAudio(&got, &mu))
	// A nudge yields a frame: the video watchdog reads a still screen and never
	// restarts the stream however far the clock moves.
	r.w.set(func() { r.w.nudgeFn = func() { r.src.last().emit(false) } })
	v := r.join()
	mu.Lock()
	a := got[0]
	mu.Unlock()
	a.packet()
	r.settle(0) // the watchdog sees the packet flow
	return r, v, a
}

// TestAudio_SilentStallIsFailed (A-03): the audio packets stop with no error
// while the picture keeps coming. Within the stall bound the stats route and
// the viewers' state read audio failed, the audio capture alone is restarted
// (same source, no stream restart, the viewer stays), and the first packet
// after it reads ok again.
func TestAudio_SilentStallIsFailed(t *testing.T) {
	r, v, a := newStallRig(t)
	if got := r.env.mgr.Stats().Audio; got != "ok" {
		t.Fatalf("stats audio %q before the stall, want ok", got)
	}

	r.settle(2500 * time.Millisecond) // no packet and no error, still inside the 3 s bound
	if got, n := r.env.mgr.Stats().Audio, a.restarts.Load(); got != "ok" || n != 0 {
		t.Fatalf("stats audio %q, restarts %d at 2.5 s, want ok and 0", got, n)
	}
	r.settle(time.Second) // past the bound
	eventually(t, 2*time.Second, "stats audio failed", func() bool { return r.env.mgr.Stats().Audio == "failed" })
	eventually(t, 2*time.Second, "state audio failed", func() bool { return v.lastAudio() == "failed" })
	eventually(t, 2*time.Second, "audio restart", func() bool { return a.restarts.Load() >= 1 })

	if n := r.src.count(); n != 1 {
		t.Errorf("source started %d times, want 1 (an audio stall never restarts the stream)", n)
	}
	if n := r.env.mgr.Viewers(); n != 1 {
		t.Errorf("viewers %d after the audio stall, want 1", n)
	}
	before := r.env.mgr.FramesSeenForTest()
	r.src.last().emit(false)
	eventually(t, 2*time.Second, "picture still handled", func() bool { return r.env.mgr.FramesSeenForTest() > before })

	a.packet() // the restart brought the buffers back
	eventually(t, 2*time.Second, "stats audio ok again", func() bool { return r.env.mgr.Stats().Audio == "ok" })
	eventually(t, 2*time.Second, "state audio cleared", func() bool { return v.lastAudio() == "" })
}

// TestAudio_StallRestartBacksOff: while the stall lasts the audio restart is
// retried after 5, 10, 20 and 40 s and then every 60 s at most; packets coming
// back end the episode, and the next stall starts again at 5 s.
func TestAudio_StallRestartBacksOff(t *testing.T) {
	r, _, a := newStallRig(t)
	want := int32(0)
	expect := func(what string) {
		t.Helper()
		eventually(t, 2*time.Second, what, func() bool { return a.restarts.Load() == want })
	}

	r.settle(3500 * time.Millisecond)
	want = 1
	expect("first restart at the stall")
	for _, gap := range []time.Duration{5, 10, 20, 40, 60, 60} {
		r.settle(gap*time.Second - 300*time.Millisecond)
		if got := a.restarts.Load(); got != want {
			t.Fatalf("restarts %d before the %ds gap ended, want %d", got, gap, want)
		}
		r.settle(400 * time.Millisecond)
		want++
		expect("restart after the gap")
	}

	a.packet()
	eventually(t, 2*time.Second, "stats audio ok", func() bool { return r.env.mgr.Stats().Audio == "ok" })
	r.settle(0)
	r.settle(3500 * time.Millisecond)
	want++
	expect("restart at the next stall")
	r.settle(5100 * time.Millisecond)
	want++
	expect("second restart of the next stall after 5 s again")
}

// TestAudio_WindowShareNotWatched: a window share's audio is per app and
// whether SCK sends a silent app's buffers is unmeasured, so a window share
// never reads as an audio stall.
func TestAudio_WindowShareNotWatched(t *testing.T) {
	r, _, a := newStallRig(t)
	r.env.mgr.SetSurfaceForTest(screencap.Surface{Kind: screencap.SurfaceWindow, ID: windowID}, sharedWindow())
	r.settle(0)
	r.settle(10 * time.Second)
	if got := r.env.mgr.Stats().Audio; got != "ok" {
		t.Errorf("stats audio %q on a quiet window share, want ok", got)
	}
	if n := a.restarts.Load(); n != 0 {
		t.Errorf("audio restarted %d times on a window share, want 0", n)
	}
	if n := r.env.mgr.Viewers(); n != 1 {
		t.Fatalf("viewers %d, want 1 (the check ran on a live share)", n)
	}
}

// TestAudio_SilenceIsNotAStall: SCK sends a buffer every 20 ms even when the Mac
// is silent (R21 control run), so packets that keep flowing never read as
// failed, however long the share runs.
func TestAudio_SilenceIsNotAStall(t *testing.T) {
	r, _, a := newStallRig(t)
	for range 80 { // 20 s of fake time in steps far below the bound
		a.packet()
		r.w.clock.Advance(250 * time.Millisecond)
		time.Sleep(30 * time.Millisecond)
	}
	if got := r.env.mgr.Stats().Audio; got != "ok" {
		t.Errorf("stats audio %q while silence packets flow, want ok", got)
	}
	if n := a.restarts.Load(); n != 0 {
		t.Errorf("audio restarted %d times while packets flow, want 0", n)
	}
}
