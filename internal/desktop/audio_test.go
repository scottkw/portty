package desktop_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// opusSilence is the 3-byte packet AudioConverter makes for a silent 20 ms
// block (R21 control run).
var opusSilence = []byte{0xf8, 0xff, 0xfe}

// audioFake is a synthetic source that also implements screencap.AudioSource:
// the usual fixture picture plus one Opus silence packet every 20 ms. Stop
// closes both audio channels, as the real source does.
type audioFake struct {
	*screencap.SyntheticSource
	pkts  chan screencap.AudioPacket
	errs  chan error
	quiet atomic.Bool // no packets while set

	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func newAudioFake(w, h int) *audioFake {
	return &audioFake{
		SyntheticSource: screencap.NewSyntheticSource(w, h),
		pkts:            make(chan screencap.AudioPacket, 8),
		errs:            make(chan error, 4),
	}
}

func (a *audioFake) Start(ctx context.Context, cfg screencap.Config) error {
	if err := a.SyntheticSource.Start(ctx, cfg); err != nil {
		return err
	}
	a.mu.Lock()
	a.stop, a.done = make(chan struct{}), make(chan struct{})
	stop, done := a.stop, a.done
	a.mu.Unlock()
	go func() {
		defer close(done)
		defer close(a.pkts)
		defer close(a.errs)
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if a.quiet.Load() {
					continue
				}
				select {
				case a.pkts <- screencap.AudioPacket{Data: opusSilence, Samples: screencap.OpusFrames}:
				default:
				}
			case <-stop:
				return
			}
		}
	}()
	return nil
}

func (a *audioFake) Stop() error {
	a.mu.Lock()
	stop, done := a.stop, a.done
	a.stop = nil
	a.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
	return a.SyntheticSource.Stop()
}

func (a *audioFake) AudioPackets() <-chan screencap.AudioPacket { return a.pkts }
func (a *audioFake) AudioErrors() <-chan error                  { return a.errs }

// withAudioSource makes every capture run use an audioFake and records it.
func withAudioSource(out *[]*audioFake, mu *sync.Mutex) tracerOpt {
	return func(env *tracerEnv, d *desktop.Deps) {
		d.NewSource = func() screencap.Source {
			a := newAudioFake(env.srcW, env.srcH)
			mu.Lock()
			*out = append(*out, a)
			mu.Unlock()
			return a
		}
	}
}

// wantAudio adds an audio recvonly transceiver and munges stereo=1 into the
// offer's Opus fmtp, as the browser viewer does.
func wantAudio(lc *loopbackClient) {
	if _, err := lc.pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		panic(err)
	}
	lc.munge = func(sdp string) string {
		return strings.ReplaceAll(sdp, "useinbandfec=1", "useinbandfec=1;stereo=1")
	}
}

// TestAudio_TracerLoopback: a viewer whose hello caps carry audio 1 and whose
// offer has an audio m-line gets an answer with stereo Opus and receives an Opus
// packet within 2 s, in the same stream as the picture.
func TestAudio_TracerLoopback(t *testing.T) {
	var (
		mu      sync.Mutex
		sources []*audioFake
	)
	env := newTracerServer(t, withAudioSource(&sources, &mu))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v := env.joinCaps(ctx, t, env.token(t, "g1"), map[string]int{"video": 1, "audio": 1}, wantAudio)
	for _, want := range []string{"opus/48000/2", "stereo=1", "sprop-stereo=1"} {
		if !strings.Contains(v.answerSDP, want) {
			t.Errorf("answer SDP lacks %q:\n%s", want, v.answerSDP)
		}
	}
	select {
	case p := <-v.lc.audio:
		if string(p.Payload) != string(opusSilence) {
			t.Errorf("audio payload %x, want the source's packet %x", p.Payload, opusSilence)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no Opus packet within 2s")
	}
	requireKeyframe(t, "viewer", v.firstAU(t, 5*time.Second))
	vs, _ := v.lc.videoStream.Load().(string)
	as, _ := v.lc.audioStream.Load().(string)
	if vs == "" || vs != as {
		t.Errorf("video stream %q and audio stream %q, want one shared stream", vs, as)
	}
}

// TestAudio_NoAudioOffer: a viewer with no audio m-line (an older viewer), or one
// that says audio 1 but offered none, gets the Phase 19/20 answer: video only,
// and video still flows.
func TestAudio_NoAudioOffer(t *testing.T) {
	var (
		mu      sync.Mutex
		sources []*audioFake
	)
	env := newTracerServer(t, withAudioSource(&sources, &mu))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, caps := range []map[string]int{{"video": 1}, {"video": 1, "audio": 1}} {
		v := env.joinCaps(ctx, t, env.token(t, "g1"), caps)
		if strings.Contains(v.answerSDP, "m=audio") {
			t.Errorf("caps %v: answer has an audio m-line:\n%s", caps, v.answerSDP)
		}
		requireKeyframe(t, "viewer", v.firstAU(t, 5*time.Second))
		select {
		case p := <-v.lc.audio:
			t.Errorf("caps %v: received audio packet %x", caps, p.Payload)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// TestAudio_ClockStepsAndResync: packets 20 ms apart advance the RTP timestamp
// by 960 each; after a 500 ms gap the next one jumps by 960 plus the gap, so the
// timestamp follows the host clock the way video's does.
func TestAudio_ClockStepsAndResync(t *testing.T) {
	clk := desktop.NewAudioClockForTest()
	t0 := time.Unix(1_700_000_000, 0)
	prev := clk.Packetize(t0)
	for i := 1; i < 50; i++ {
		ts := clk.Packetize(t0.Add(time.Duration(i) * 20 * time.Millisecond))
		if d := ts - prev; d != 960 {
			t.Fatalf("packet %d advanced the timestamp by %d, want 960", i, d)
		}
		prev = ts
	}
	ts := clk.Packetize(t0.Add(49*20*time.Millisecond + 500*time.Millisecond))
	if d := ts - prev; d != 24000 {
		t.Errorf("after a 500 ms gap the timestamp advanced by %d, want 24000 (resynced)", d)
	}
}

// TestAudio_ClockDriftLimit: a packet 50 ms late is not corrected (under the
// 60 ms limit); one 70 ms late is, on that packet.
func TestAudio_ClockDriftLimit(t *testing.T) {
	steps := func(arrivalsMs ...int) []uint32 {
		clk := desktop.NewAudioClockForTest()
		t0 := time.Unix(1_700_000_000, 0)
		var out []uint32
		prev := clk.Packetize(t0.Add(time.Duration(arrivalsMs[0]) * time.Millisecond))
		for _, ms := range arrivalsMs[1:] {
			ts := clk.Packetize(t0.Add(time.Duration(ms) * time.Millisecond))
			out = append(out, ts-prev)
			prev = ts
		}
		return out
	}
	if got := steps(0, 20, 40, 60, 80, 150); got[len(got)-1] != 960 {
		t.Errorf("a packet 50 ms late stepped by %d, want 960 (not corrected)", got[len(got)-1])
	}
	if got := steps(0, 20, 40, 60, 80, 170); got[len(got)-1] != 960+3360 {
		t.Errorf("a packet 70 ms late stepped by %d, want %d (corrected)", got[len(got)-1], 960+3360)
	}
}

// TestAudio_FailureIsStateNotOutage: an audio error becomes the state field
// audio=failed on a viewer's state message while video access units keep
// arriving and the source is not restarted; the next good packet clears it.
func TestAudio_FailureIsStateNotOutage(t *testing.T) {
	var (
		mu      sync.Mutex
		sources []*audioFake
	)
	env := newTracerServer(t, withAudioSource(&sources, &mu))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	v := env.joinCaps(ctx, t, env.token(t, "g1"), map[string]int{"video": 1, "audio": 1}, wantAudio)
	requireKeyframe(t, "viewer", v.firstAU(t, 5*time.Second))
	select {
	case <-v.lc.audio:
	case <-time.After(2 * time.Second):
		t.Fatal("no Opus packet within 2s")
	}
	mu.Lock()
	src := sources[0]
	mu.Unlock()

	src.quiet.Store(true) // the failing source stops sending; the error reaches the Manager
	src.errs <- errors.New("audio converter broke")
	eventually(t, 2*time.Second, "state audio=failed", func() bool { return v.lastAudio() == "failed" })
	v.drain()
	v.firstAU(t, 2*time.Second) // the picture keeps coming
	mu.Lock()
	n := len(sources)
	mu.Unlock()
	if n != 1 {
		t.Errorf("source started %d times, want 1 (no restart on an audio failure)", n)
	}

	src.quiet.Store(false)
	eventually(t, 2*time.Second, "state audio field cleared", func() bool { return v.lastAudio() == "" })
}

// TestAudio_GuestHearsAudio: a click-to-share guest gets the same audio as an
// own-account viewer (mute is viewer-side only), and the guest hello says audio 1.
func TestAudio_GuestHearsAudio(t *testing.T) {
	e := newGuestEnv(t, func(d *desktop.Deps) {
		d.NewSource = func() screencap.Source { return newAudioFake(1280, 800) }
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := e.joinGuestCaps(ctx, t, 1001, "nodeA", true, map[string]int{"video": 1, "input": 1, "audio": 1}, wantAudio)
	if caps, _ := g.hello["caps"].(map[string]any); caps["audio"] != float64(1) {
		t.Errorf("guest hello caps %v, want audio 1", caps)
	}
	select {
	case p := <-g.lc.audio:
		if string(p.Payload) != string(opusSilence) {
			t.Errorf("guest audio payload %x, want %x", p.Payload, opusSilence)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the guest got no Opus packet within 5s")
	}
}

// TestAudio_AnswerFmtp: the answer's Opus line carries stereo=1 and sprop-stereo=1
// whatever the offer's fmtp said, and an SDP with no Opus is untouched.
func TestAudio_AnswerFmtp(t *testing.T) {
	const head = "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=rtpmap:111 opus/48000/2\r\n"
	const noOpus = "v=0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\na=rtpmap:102 H264/90000\r\n"
	for name, c := range map[string]struct{ in, want string }{
		"offer fmtp": {head + "a=fmtp:111 minptime=10;useinbandfec=1\r\na=sendonly\r\n", head + "a=fmtp:111 minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1\r\na=sendonly\r\n"},
		"mono offer": {head + "a=fmtp:111 stereo=0;useinbandfec=1\r\n", head + "a=fmtp:111 useinbandfec=1;stereo=1;sprop-stereo=1\r\n"},
		"no fmtp":    {head + "a=sendonly\r\n", head + "a=fmtp:111 stereo=1;sprop-stereo=1\r\na=sendonly\r\n"},
		"no opus":    {noOpus, noOpus},
	} {
		if got := desktop.EnsureOpusStereoForTest(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}
