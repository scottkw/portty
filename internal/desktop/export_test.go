package desktop

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/screencap"
)

// PacketizeForTest is the fan-out's per-unit RTP step.
func PacketizeForTest(p rtp.Packetizer, au screencap.AccessUnit) []*rtp.Packet {
	return packetize(p, au)
}

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

func init() {
	// Tests never read the host's lock, display or power state, and never take a
	// real power assertion: a Manager built without these Deps gets quiet fakes.
	no := func() (bool, error) { return false, nil }
	defaultLock, defaultDisplayAsleep, defaultOnAC = no, no, no
	hold := func(string) (func() error, error) { return func() error { return nil }, nil }
	defaultHoldDisplay, defaultHoldSystem = hold, hold
}

// PickStateForTest exposes the state precedence for a table test.
func PickStateForTest(paused, perm, asleep, awaiting, locked bool, viewers int) State {
	return pickState(stateFlags{paused: paused, perm: perm, asleep: asleep, awaiting: awaiting, locked: locked}, viewers)
}

// RuntimeState is desktop-state.json's content.
type RuntimeState = runtimeState

// ReadRuntimeStateForTest reads and validates the state file in dir.
func ReadRuntimeStateForTest(dir string) (RuntimeState, error) { return readRuntimeState(dir) }

// WriteRuntimeStateForTest writes the state file the way the Manager does.
func WriteRuntimeStateForTest(dir string, rs RuntimeState) error { return writeRuntimeState(dir, rs) }

// FramesSeenForTest is how many access units the fan-out has handled.
func (m *Manager) FramesSeenForTest() uint64 { return m.frameCount() }

// TickForTest runs one state-loop pass synchronously.
func (m *Manager) TickForTest(ctx context.Context) { m.tick(ctx) }

// DecodeProbeForTest exposes the strict perm-probe decoder.
func DecodeProbeForTest(out []byte) (screencap.Permissions, error) { return decodeProbe(out) }

// NewProberForTest builds a Prober with a fake clock and child runner.
func NewProberForTest(now func() time.Time, run func(ctx context.Context, args ...string) ([]byte, error)) *Prober {
	return &Prober{now: now, run: func(ctx context.Context, _ string, args ...string) ([]byte, error) { return run(ctx, args...) }}
}

// SetTicksForTest shortens the state loop's cadence; call before Start.
func (m *Manager) SetTicksForTest(viewers, idle time.Duration) {
	m.tickViewers, m.tickIdle = viewers, idle
}

// BumpEpochForTest is what a surface change does to the input epoch.
func (m *Manager) BumpEpochForTest() uint32 { return m.bumpEpoch() }

// AudioClock is the audio RTP stamping of audio.go.
type AudioClock struct{ c *audioClock }

// NewAudioClockForTest returns a fresh audio clock.
func NewAudioClockForTest() *AudioClock { return &AudioClock{newAudioClock()} }

// Packetize stamps one Opus packet that arrived at now and returns its RTP timestamp.
func (a *AudioClock) Packetize(now time.Time) uint32 {
	return a.c.packetize(now, screencap.AudioPacket{Data: []byte{0xf8, 0xff, 0xfe}, Samples: screencap.OpusFrames})[0].Timestamp
}

// EnsureOpusStereoForTest is the answer's Opus fmtp completion.
func EnsureOpusStereoForTest(sdp string) string { return ensureOpusStereo(sdp) }

// SetQualityTickForTest shortens the quality controller's cadence; call before Start.
func (m *Manager) SetQualityTickForTest(d time.Duration) { m.q.tickEvery = d }

// QualityTickForTest runs one controller pass synchronously.
func (m *Manager) QualityTickForTest() { m.q.tick() }

// QualityViewersForTest is how many viewers the controller counts.
func (m *Manager) QualityViewersForTest() int {
	m.q.mu.Lock()
	defer m.q.mu.Unlock()
	return len(m.q.viewers)
}

// SetEstimateForTest replaces viewer id's bandwidth estimate with f.
func (m *Manager) SetEstimateForTest(id ViewerID, f func() int) {
	m.q.mu.Lock()
	defer m.q.mu.Unlock()
	if v := m.q.viewers[id]; v != nil {
		v.stub = f
	}
}

// ViewerEstimateForTest is viewer id's own GCC estimate in bits per second (not
// the encoder's target); false when the controller does not count the viewer.
func (m *Manager) ViewerEstimateForTest(id ViewerID) (int, bool) {
	m.q.mu.Lock()
	v := m.q.viewers[id]
	m.q.mu.Unlock()
	if v == nil || v.bwe == nil {
		return 0, false
	}
	return v.bwe.GetTargetBitrate(), true
}

// ViewerBWEForTest says what the controller knows of every viewer's estimator: the
// estimate, whether any transport-cc feedback came back, whether the controller has
// decided there is none, whether the viewer is slow and its last reported loss.
func (m *Manager) ViewerBWEForTest() map[ViewerID]string {
	m.q.mu.Lock()
	defer m.q.mu.Unlock()
	out := map[ViewerID]string{}
	for id, v := range m.q.viewers {
		if v.bwe == nil {
			continue
		}
		out[id] = fmt.Sprintf("est=%d feedback=%v noFB=%v slow=%v loss=%.2f", v.bwe.GetTargetBitrate(), v.bwe.feedback.Load(), v.noFB, v.slow.Slow, v.loss)
	}
	return out
}

// EpochForTest is the input epoch viewers stamp positional input with.
func (m *Manager) EpochForTest() uint32 { return m.currentEpoch() }

// QualityDistinctEstimatorsForTest is how many different estimator objects the
// controller's viewers hold (a binding mix-up would make two share one).
func (m *Manager) QualityDistinctEstimatorsForTest() int {
	m.q.mu.Lock()
	defer m.q.mu.Unlock()
	seen := map[*viewerBWE]bool{}
	for _, v := range m.q.viewers {
		if v.bwe != nil {
			seen[v.bwe] = true
		}
	}
	return len(seen)
}
