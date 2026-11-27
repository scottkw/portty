package desktop_test

import (
	"context"
	"strings"
	"sync"
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
	pkts chan screencap.AudioPacket
	errs chan error

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
