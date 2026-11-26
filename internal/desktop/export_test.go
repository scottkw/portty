package desktop

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/screencap"
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
