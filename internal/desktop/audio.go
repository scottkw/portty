package desktop

// audio.go: the shared Mac's system audio, from the capture source to every
// viewer's PeerConnection as a stereo Opus track beside the picture (DCTL-07).
// Mute exists only on the viewer, so audio goes to every ready viewer, muted
// ones and click-to-share guests included. An audio fault is the state field
// audio=failed and never touches the picture.

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/screencap"
)

// opusFmtp asks the browser to decode stereo: libwebrtc's receive side defaults
// to mono when stereo is absent from the fmtp (RESEARCH Pitfall 8).
const opusFmtp = "minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1"

// opusCapability is the one audio codec every viewer track offers.
var opusCapability = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: screencap.OpusRate, Channels: 2, SDPFmtpLine: opusFmtp}

// audioResync is how far the wall clock may drift from the sample count before
// the RTP timestamp is corrected (CONTEXT [Audio] sync).
const audioResync = 60 * time.Millisecond

// audioClock stamps Opus packets on the host clock, like video: a 48 kHz sample
// counter that advances 960 per packet and follows wall-clock time when it
// drifts more than audioResync. The sender reports pion writes then let the
// browser line the two tracks up.
// Recorded deviation: the CONTEXT decision stamps from SCK presentation
// timestamps; this stamps from packet arrival, the clock video already uses
// (packetize, WR-02). Arrival trails the PTS by the capture and encode delay.
type audioClock struct {
	pkt  rtp.Packetizer
	base time.Time // wall time of sample 0
	n    uint64    // samples sent since base
}

func newAudioClock() *audioClock {
	return &audioClock{pkt: rtp.NewPacketizer(outboundMTU, 0, 0, &codecs.OpusPayloader{}, rtp.NewRandomSequencer(), screencap.OpusRate)}
}

// packetize stamps ap, which arrived at now.
func (c *audioClock) packetize(now time.Time, ap screencap.AudioPacket) []*rtp.Packet {
	if c.base.IsZero() {
		c.base = now
	}
	wall := int64(now.Sub(c.base) * screencap.OpusRate / time.Second)
	drift := wall - int64(c.n)
	limit := int64(audioResync * screencap.OpusRate / time.Second)
	switch {
	case drift > limit:
		// A gap (the capture paused or the Mac slept): jump the timestamp forward.
		c.pkt.SkipSamples(uint32(drift))
		c.n += uint64(drift)
	case drift < -limit:
		// A burst arrived faster than real time: timestamps never go back, so the
		// base moves instead.
		c.base = now.Add(-time.Duration(c.n) * time.Second / screencap.OpusRate)
	}
	c.n += uint64(ap.Samples)
	return c.pkt.Packetize(ap.Data, uint32(ap.Samples))
}

// offersAudio reports whether an offer SDP carries an audio m-line that is not
// rejected (port 0). A viewer that does not gets video only.
func offersAudio(sdp string) bool {
	for _, line := range strings.Split(sdp, "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "m=audio" && f[1] != "0" {
			return true
		}
	}
	return false
}

// ensureOpusStereo makes the answer's Opus fmtp carry stereo=1 and
// sprop-stereo=1. pion answers with the offer's own fmtp for the codec it
// matched, which never has sprop-stereo, so the text sent to the viewer is
// completed here; pion's own state is not touched.
func ensureOpusStereo(sdp string) string {
	lines := strings.Split(sdp, "\r\n")
	pt, at := "", -1
	for i, l := range lines {
		if rest, ok := strings.CutPrefix(l, "a=rtpmap:"); ok {
			if p, enc, ok := strings.Cut(rest, " "); ok && strings.EqualFold(enc, "opus/48000/2") {
				pt, at = p, i
				break
			}
		}
	}
	if at < 0 {
		return sdp
	}
	prefix := "a=fmtp:" + pt + " "
	params := []string{}
	fmtpAt := -1
	for i, l := range lines {
		if rest, ok := strings.CutPrefix(l, prefix); ok {
			fmtpAt = i
			for _, p := range strings.Split(rest, ";") {
				if k, _, _ := strings.Cut(p, "="); k != "stereo" && k != "sprop-stereo" && p != "" {
					params = append(params, p)
				}
			}
		}
	}
	line := prefix + strings.Join(append(params, "stereo=1", "sprop-stereo=1"), ";")
	if fmtpAt >= 0 {
		lines[fmtpAt] = line
	} else {
		lines = append(lines[:at+1], append([]string{line}, lines[at+1:]...)...)
	}
	return strings.Join(lines, "\r\n")
}

// audioOut packetizes every Opus packet of src once and writes it to each ready
// viewer's audio track until the packets end or ctx is cancelled. It shares no
// lock with the video fan-out while writing, so a slow audio write cannot delay
// the picture.
func (m *Manager) audioOut(ctx context.Context, src screencap.AudioSource, done chan<- struct{}) {
	defer close(done)
	defer recoverLog()
	clk := m.aclk // continues across capture runs, like video's m.vpkt
	pkts, errs := src.AudioPackets(), src.AudioErrors()
	for {
		select {
		case <-ctx.Done():
			return
		case ap, ok := <-pkts:
			if !ok {
				return
			}
			m.audioPkts.Add(1)
			m.noteAudioOK()
			m.writeAudio(clk.packetize(m.d.Now().Round(0), ap))
		case err, ok := <-errs:
			if !ok {
				errs = nil // a nil channel never fires; the packets case ends the loop
				continue
			}
			m.noteAudioFailed(err)
		}
	}
}

// writeAudio sends one packet's RTP packets to every viewer whose media path is
// up and that took an audio track. Unlike video there is no keyframe to wait for.
func (m *Manager) writeAudio(pkts []*rtp.Packet) {
	m.vmu.Lock()
	targets := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		if v.ready && v.audio != nil {
			targets = append(targets, v)
		}
	}
	m.vmu.Unlock()
	for _, v := range targets {
		for _, p := range pkts {
			// A write error means that viewer's PeerConnection is closing.
			_ = v.audio.WriteRTP(p)
		}
	}
}

// noteAudioFailed records that audio capture or encoding failed and tells the
// viewers once. The picture keeps streaming.
func (m *Manager) noteAudioFailed(err error) {
	if m.audioFail.Swap(true) {
		return
	}
	slog.Warn("desktop: audio failed", "err", err)
	m.pushIfChanged(context.Background())
}

// noteAudioOK clears a failure on the first good packet after it.
func (m *Manager) noteAudioOK() {
	if !m.audioFail.Load() || !m.audioFail.CompareAndSwap(true, false) {
		return
	}
	slog.Info("desktop: audio recovered")
	m.pushIfChanged(context.Background())
}

// The audio stall watchdog (A-03). ScreenCaptureKit sends an audio buffer every
// 20 ms even when the Mac is silent (R21 control run: 3-byte Opus packets), so a
// display capture with no audio packet for audioStallAfter has lost its audio
// without an error, as after a coreaudiod restart.
const (
	audioStallAfter = 3 * time.Second
	audioRetryFirst = 5 * time.Second // the audio restart's backoff doubles from here
	audioRetryMax   = time.Minute
)

var errAudioStalled = errors.New("no audio from the capture for " + audioStallAfter.String())

// audioWatch is the audio stall watchdog's memory, guarded by Manager.tickMu.
type audioWatch struct {
	run   chan struct{} // the watched capture run's audioDone
	seen  uint64        // Manager.audioPkts when it last moved
	at    time.Time     // when it last moved, or the watch began
	next  time.Time     // no audio restart before this
	every time.Duration // the backoff after the last audio restart
}

// audioWatchStep is one state-loop step of the audio stall watchdog: no packet
// for audioStallAfter makes audio failed and restarts the audio capture alone,
// with backoff; the stream itself never restarts for audio (CONTEXT [Audio]).
// The next packet clears the failure (audioOut). Progress is a packet count per
// tick, so a wall-clock step while packets flow is no stall.
func (m *Manager) audioWatchStep(now time.Time) {
	m.smu.Lock()
	blocked := m.paused || m.permDenied || m.asleep || m.noDisplay || !m.wakeAt.IsZero()
	m.smu.Unlock()
	m.mu.Lock()
	src, run := m.src, m.audioDone
	m.mu.Unlock()
	w, n := &m.aw, m.audioPkts.Load()
	// shortcut: window shares are not watched (whether SCK sends a silent app's audio buffers is unmeasured); watch them once a VM row shows it does.
	if blocked || run == nil || run != w.run || n != w.seen || m.surfaceSel().Kind != screencap.SurfaceDisplay {
		*w = audioWatch{run: run, seen: n, at: now}
		return
	}
	if now.Sub(w.at) < audioStallAfter || now.Before(w.next) {
		return
	}
	m.noteAudioFailed(errAudioStalled)
	w.every = min(max(2*w.every, audioRetryFirst), audioRetryMax)
	w.next = now.Add(w.every)
	if r, ok := src.(screencap.AudioRestarter); ok {
		slog.Info("desktop: restarting audio capture", "retry_in", w.every)
		if err := r.RestartAudio(); err != nil {
			slog.Warn("desktop: audio capture restart failed", "err", err)
		}
	}
}
