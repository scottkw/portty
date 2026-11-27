package desktop

// control.go: whether remote control works right now, as a state the viewers and
// the host GUI can show. Control needs Accessibility is measured by effect (do
// posted moves move the pointer?), never read from a permission preflight: the
// preflights are cached and stay stale after a grant or a revoke.

import (
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

// Control input values and window values of the control message.
const (
	ControlOK                 = "ok"
	ControlNeedsAccessibility = "needs_accessibility"

	WindowRaiseFailed = "raise_failed"
	WindowMinimized   = "minimized"
)

const (
	effectDelay  = 60 * time.Millisecond // how long after a post the pointer is read back
	effectMisses = 3                     // consecutive missed moves that mean no Accessibility
	recoverEvery = 5 * time.Second       // nudge cadence while control needs Accessibility
)

// msgControl and msgCursor are the daemon-to-viewer messages of wire protocol 1
// on the input channel (frozen in testdata/input-v1.jsonl).
type msgControl struct {
	T      string `json:"t"`
	Input  string `json:"input"`
	Window string `json:"window"`
}

type msgCursor struct {
	T string  `json:"t"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	E uint32  `json:"e"`
}

// controlState is the control attribute, behind its own leaf mutex.
type controlState struct {
	mu        sync.Mutex
	input     string // "" reads as ControlOK
	window    string
	gen       uint64 // pointer posts so far: an effect check only counts for the latest
	misses    int
	nextNudge time.Time
}

// ControlState returns the input and window control values the host GUI shows.
func (m *Manager) ControlState() (input, window string) {
	c := &m.in.ctl
	c.mu.Lock()
	defer c.mu.Unlock()
	return orOK(c.input), c.window
}

func orOK(s string) string {
	if s == "" {
		return ControlOK
	}
	return s
}

func (m *Manager) setControlInput(s string) {
	c := &m.in.ctl
	c.mu.Lock()
	changed := orOK(c.input) != s
	c.input, c.misses, c.nextNudge = s, 0, time.Time{}
	c.mu.Unlock()
	if changed {
		m.pushControl()
	}
}

// setControlWindow records the window control value (the surface plan sets it).
func (m *Manager) setControlWindow(s string) {
	c := &m.in.ctl
	c.mu.Lock()
	changed := c.window != s
	c.window = s
	c.mu.Unlock()
	if changed {
		m.pushControl()
	}
}

// sendControl writes the current control snapshot to one input channel. A send
// error is ignored: the channel is closing.
func (m *Manager) sendControl(dc *webrtc.DataChannel) {
	in, win := m.ControlState()
	b, err := json.Marshal(msgControl{T: "control", Input: in, Window: win})
	if err != nil {
		return
	}
	_ = dc.SendText(string(b))
}

// inputChannels returns every open input channel.
func (m *Manager) inputChannels() []*webrtc.DataChannel {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	var out []*webrtc.DataChannel
	for _, v := range m.viewers {
		if v.dc != nil {
			out = append(out, v.dc)
		}
	}
	return out
}

func (m *Manager) pushControl() {
	for _, dc := range m.inputChannels() {
		m.sendControl(dc)
	}
}

// notePointerPost marks that a pointer event was posted, so an effect check
// scheduled for an earlier one is skipped.
func (m *Manager) notePointerPost() {
	c := &m.in.ctl
	c.mu.Lock()
	c.gen++
	c.mu.Unlock()
}

// scheduleMoveCheck reads the pointer back effectDelay after a plain move (never
// on the post path) and compares it with where the move was sent.
func (m *Manager) scheduleMoveCheck(x, y float64) {
	c := &m.in.ctl
	c.mu.Lock()
	gen := c.gen
	c.mu.Unlock()
	time.AfterFunc(effectDelay, func() { m.checkMoveEffect(gen, x, y) })
}

// checkMoveEffect counts a hit or a miss for the move posted at generation gen
// unless a newer pointer event was posted since (then the pointer is elsewhere
// for a good reason). effectMisses misses in a row mean posted events are not
// taking effect: control needs Accessibility.
func (m *Manager) checkMoveEffect(gen uint64, x, y float64) {
	defer recoverLog()
	if m.closed.Load() {
		return
	}
	c := &m.in.ctl
	c.mu.Lock()
	stale := gen != c.gen
	c.mu.Unlock()
	if stale {
		return
	}
	if on, err := m.d.PointOnDisplay(x, y); err == nil && !on {
		// Past every display edge (a window share of a window that extends off
		// screen): the WindowServer clamps the pointer, so a differing read-back
		// says nothing about Accessibility (M-04). An unreadable answer checks.
		m.countInput("effect_offscreen")
		return
	}
	cx, cy, err := m.d.Input.CursorLocation()
	if err != nil {
		return
	}
	if math.Abs(cx-x) <= 1 && math.Abs(cy-y) <= 1 {
		m.countInput("effect_hit")
		c.mu.Lock()
		c.misses = 0
		recovered := orOK(c.input) != ControlOK
		c.mu.Unlock()
		if recovered {
			m.setControlInput(ControlOK)
		}
		return
	}
	m.countInput("effect_miss")
	c.mu.Lock()
	c.misses++
	trip := c.misses >= effectMisses && orOK(c.input) == ControlOK
	c.mu.Unlock()
	if trip {
		m.setControlInput(ControlNeedsAccessibility)
	}
}

// controlNeedsAccessibility reports the state a press is gated on.
func (m *Manager) controlNeedsAccessibility() bool {
	in, _ := m.ControlState()
	return in == ControlNeedsAccessibility
}

// recoverControl, run by the input tick, nudges the pointer every recoverEvery
// while control needs Accessibility and some viewer has an input channel open;
// a nudge that returns means events take effect again.
func (m *Manager) recoverControl(now time.Time) {
	if !m.controlNeedsAccessibility() || len(m.inputChannels()) == 0 {
		return
	}
	c := &m.in.ctl
	c.mu.Lock()
	due := !now.Before(c.nextNudge)
	if due {
		c.nextNudge = now.Add(recoverEvery)
	}
	c.mu.Unlock()
	if !due {
		return
	}
	if ok, err := m.d.Nudge(); err == nil && ok {
		m.setControlInput(ControlOK)
	}
}

// reportCursor tells the viewer where its relative move put the pointer, as
// normalized surface coordinates, at most every cursorReportEvery.
// shortcut: no trailing report, so the last position of a burst is reported
// with the next move; upgrade if the viewer needs it exact at rest.
func (m *Manager) reportCursor(v *viewer, t inputTarget, x, y float64) {
	now := m.d.Now()
	m.in.mu.Lock()
	due := v.in.lastCursorReport.IsZero() || now.Sub(v.in.lastCursorReport) >= cursorReportEvery
	if due {
		v.in.lastCursorReport = now
	}
	m.in.mu.Unlock()
	if !due {
		return
	}
	m.vmu.Lock()
	dc := v.dc
	m.vmu.Unlock()
	if dc == nil {
		return
	}
	b, err := json.Marshal(msgCursor{T: "cursor", X: (x - t.x) / t.w, Y: (y - t.y) / t.h, E: t.epoch})
	if err != nil {
		return
	}
	_ = dc.SendText(string(b))
}
