package desktop

// input.go: the viewer-to-daemon input path. A viewer opens a reliable ordered
// data channel labelled "input" before its offer (it must have declared caps
// input 1 in its hello); JSON messages on it are parsed, handed without
// blocking to the one input goroutine per Manager, and handleInput, the single
// entry point every message passes with its sending viewer, gates and posts
// them through the InputBridge.
//
// Wire protocol 1, viewer to daemon (one JSON object per message, t picks the
// kind, unknown fields are ignored, at most MaxInputMessage bytes): move, rmove,
// btn, wheel, key, text, hb, release. The frozen examples are
// testdata/input-v1.jsonl.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/screencap"
)

// MaxInputMessage is the largest input message the daemon decodes.
const MaxInputMessage = 4096

const (
	inputLabel   = "input" // the one data channel label the daemon accepts
	maxFifo      = 1024    // queued non-release messages per viewer
	maxCodeBytes = 32      // key code and typed-character fields
	maxTextBytes = 1024
	flushWait    = time.Second // how long Close waits for queued releases to post
)

// Message kinds.
const (
	inMove    = "move"
	inRMove   = "rmove"
	inBtn     = "btn"
	inWheel   = "wheel"
	inKey     = "key"
	inText    = "text"
	inHB      = "hb"
	inRelease = "release"
)

// InputBridge is what the input goroutine posts through; screencap.InputBridge
// is the real one and tests inject a fake. Only that goroutine calls it.
type InputBridge interface {
	PostMouse(kind screencap.MouseKind, x, y float64, button, clickState, dx, dy int, flags uint64) error
	CursorLocation() (x, y float64, err error)
}

var (
	errUnknownKind = errors.New("desktop: unknown input kind")
	errBadInput    = errors.New("desktop: invalid input message")
)

// inputMsg is one decoded input message: the union of every kind's fields.
type inputMsg struct {
	T      string
	X, Y   float64 // move, btn: normalized 0..1 of the shared surface
	DX, DY float64 // rmove: normalized units; wheel: pixels, browser sign
	E      uint32  // surface epoch, at least 1
	B      int     // btn: browser button index, 0 left, 1 middle, 2 right
	D      bool    // btn, key: down
	TS     float64 // viewer performance.now() in ms
	Code   string  // key: W3C KeyboardEvent.code
	R      bool    // key: auto-repeat
	M      int     // key: modifier mask, 1 shift, 2 ctrl, 4 alt, 8 meta
	Caps   bool    // key: Caps Lock state
	K      string  // key: the typed character (validated, never typed from)
	S      string  // text: composed text
}

// isRelease reports whether msg lets go of something the viewer may hold. A
// release is never dropped by a gate, a rate cap or a full queue.
func (msg inputMsg) isRelease() bool {
	return (msg.T == inBtn && !msg.D) || (msg.T == inKey && !msg.D) || msg.T == inRelease
}

// wireInput decodes with pointers so a missing required field is told from zero.
type wireInput struct {
	T    string   `json:"t"`
	X    *float64 `json:"x"`
	Y    *float64 `json:"y"`
	DX   *float64 `json:"dx"`
	DY   *float64 `json:"dy"`
	E    *uint32  `json:"e"`
	B    *int     `json:"b"`
	D    *bool    `json:"d"`
	TS   float64  `json:"ts"`
	Code string   `json:"code"`
	R    bool     `json:"r"`
	M    int      `json:"m"`
	Caps bool     `json:"caps"`
	K    string   `json:"k"`
	S    string   `json:"s"`
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func invalid(why string) error { return fmt.Errorf("%w: %s", errBadInput, why) }

// parseInput decodes and validates one message. Anything it cannot vouch for is
// an error (counted by the caller, never guessed at).
func parseInput(b []byte) (inputMsg, error) {
	if len(b) > MaxInputMessage {
		return inputMsg{}, invalid("too large")
	}
	// Decoding would replace invalid bytes with U+FFFD and hide them.
	if !utf8.Valid(b) {
		return inputMsg{}, invalid("not utf-8")
	}
	var w wireInput
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&w); err != nil {
		return inputMsg{}, invalid("not json")
	}
	msg := inputMsg{T: w.T, TS: w.TS, Code: w.Code, R: w.R, M: w.M, Caps: w.Caps, K: w.K, S: w.S}
	if w.E != nil {
		msg.E = *w.E
	}
	if w.B != nil {
		msg.B = *w.B
	}
	if w.D != nil {
		msg.D = *w.D
	}
	switch w.T {
	case inMove, inBtn:
		if w.X == nil || w.Y == nil || w.E == nil || msg.E == 0 {
			return inputMsg{}, invalid("missing x, y or e")
		}
		msg.X, msg.Y = clamp01(*w.X), clamp01(*w.Y)
		if w.T == inBtn && (w.B == nil || w.D == nil || msg.B < 0 || msg.B > 2) {
			return inputMsg{}, invalid("btn needs b 0..2 and d")
		}
	case inRMove, inWheel:
		if w.DX == nil || w.DY == nil || w.E == nil || msg.E == 0 {
			return inputMsg{}, invalid("missing dx, dy or e")
		}
		msg.DX, msg.DY = *w.DX, *w.DY
	case inKey:
		if w.D == nil || w.Code == "" || len(w.Code) > maxCodeBytes || len(w.K) > maxCodeBytes || w.M < 0 || w.M > 15 {
			return inputMsg{}, invalid("bad key")
		}
	case inText:
		if len(w.S) < 1 || len(w.S) > maxTextBytes {
			return inputMsg{}, invalid("text length")
		}
	case inHB, inRelease:
	default:
		return inputMsg{}, errUnknownKind
	}
	return msg, nil
}

// inputTarget is the surface input is mapped onto, in global points.
type inputTarget struct {
	epoch      uint32
	x, y, w, h float64
	ok         bool // the geometry is known; without it presses are dropped
}

// point maps normalized (nx, ny) to the nearest global point, at most one point
// inside the far edge.
func (t inputTarget) point(nx, ny float64) (x, y float64) {
	return t.x + min(math.Round(nx*t.w), t.w-1), t.y + min(math.Round(ny*t.h), t.h-1)
}

// viewerInput is one viewer's input state, guarded by Manager.in.mu.
type viewerInput struct {
	fifo         []inputMsg
	held         map[int]struct{} // browser buttons held
	lastX, lastY float64          // the last mapped global point
	haveLast     bool
	closed       bool // the viewer's input life is over: nothing more is accepted
}

// release is one queued let-go for the input goroutine; a non-nil flush is
// closed when it is reached instead.
type release struct {
	button int // CG button
	x, y   float64
	flush  chan struct{}
}

// inputState is the Manager's input-wide state, kept here (not in manager.go)
// so the input features add to it without touching the Manager struct.
type inputState struct {
	epoch atomic.Uint32 // the surface epoch; input sent with another one is dropped

	mu     sync.Mutex
	active []*viewer // viewers with queued messages, in round-robin order
	rels   []release
	counts map[string]uint64
	wake   chan struct{}

	cancel func()
	done   chan struct{}
}

func (m *Manager) currentEpoch() uint32 { return m.in.epoch.Load() }

// bumpEpoch invalidates every viewer's in-flight coordinates (a surface change).
func (m *Manager) bumpEpoch() uint32 { return m.in.epoch.Add(1) }

func (m *Manager) wakeInput() {
	select {
	case m.in.wake <- struct{}{}:
	default:
	}
}

// countInput adds one to a counter. Counters name event kinds and drop reasons
// only, never a key, text or coordinate.
func (m *Manager) countInput(name string) {
	m.in.mu.Lock()
	m.in.counts[name]++
	m.in.mu.Unlock()
}

// InputCounters returns a copy of the input counters.
func (m *Manager) InputCounters() map[string]uint64 {
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	out := make(map[string]uint64, len(m.in.counts))
	for k, v := range m.in.counts {
		out[k] = v
	}
	return out
}

// acceptDataChannel is the DataChannelHook of a Manager without another: only a
// reliable ordered channel labelled "input" from a viewer that declared caps
// input 1 is kept (one per viewer), anything else is closed.
func (m *Manager) acceptDataChannel(v Viewer, dc *webrtc.DataChannel) {
	m.vmu.Lock()
	vw := m.viewers[v.ID]
	ok := vw != nil && vw.inputCap == 1 && vw.dc == nil && dc.Label() == inputLabel &&
		dc.Ordered() && dc.MaxRetransmits() == nil && dc.MaxPacketLifeTime() == nil
	if ok {
		vw.dc = dc
	}
	m.vmu.Unlock()
	if !ok {
		_ = dc.Close()
		return
	}
	// pion runs OnMessage inline on the SCTP read loop: parse and hand off only.
	dc.OnMessage(func(dm webrtc.DataChannelMessage) {
		defer recoverLog()
		if !dm.IsString {
			m.countInput("malformed")
			return
		}
		msg, err := parseInput(dm.Data)
		if err != nil {
			if errors.Is(err, errUnknownKind) {
				m.countInput("unknown_kind")
			} else {
				m.countInput("malformed")
			}
			return
		}
		m.enqueueInput(vw, msg)
	})
	dc.OnClose(func() {
		defer recoverLog()
		m.ReleaseViewer(v.ID, "channel_closed")
	})
}

// enqueueInput hands msg to the input goroutine without blocking. A later move
// replaces a pending move only while nothing is queued behind it, so
// move-before-click order stays right. A queue that is full drops everything
// but releases.
func (m *Manager) enqueueInput(v *viewer, msg inputMsg) {
	m.in.mu.Lock()
	vi := v.in
	if vi.closed {
		m.in.mu.Unlock()
		return
	}
	if !msg.isRelease() && len(vi.fifo) >= maxFifo {
		m.in.counts["queue_full"]++
		m.in.mu.Unlock()
		return
	}
	wasEmpty := len(vi.fifo) == 0
	if n := len(vi.fifo); n > 0 && msg.T == inMove && vi.fifo[n-1].T == inMove {
		vi.fifo[n-1] = msg
	} else {
		vi.fifo = append(vi.fifo, msg)
	}
	if wasEmpty {
		m.in.active = append(m.in.active, v)
	}
	m.in.mu.Unlock()
	m.wakeInput()
}

// nextInput pops the next work item: queued releases first, then one message
// from the viewer at the head of the round-robin.
func (m *Manager) nextInput() (rel release, v *viewer, msg inputMsg, kind int) {
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	if len(m.in.rels) > 0 {
		rel, m.in.rels = m.in.rels[0], m.in.rels[1:]
		return rel, nil, inputMsg{}, 1
	}
	if len(m.in.active) == 0 {
		return release{}, nil, inputMsg{}, 0
	}
	v, m.in.active = m.in.active[0], m.in.active[1:]
	msg, v.in.fifo = v.in.fifo[0], v.in.fifo[1:]
	if len(v.in.fifo) > 0 {
		m.in.active = append(m.in.active, v)
	}
	return release{}, v, msg, 2
}

// inputLoop is the only caller of the bridge.
func (m *Manager) inputLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	for {
		for m.runInput() {
		}
		select {
		case <-ctx.Done():
			return
		case <-m.in.wake:
		}
	}
}

// runInput does one work item and reports whether there was one. A panic in
// the bridge or a handler costs that item, not the daemon.
func (m *Manager) runInput() bool {
	rel, v, msg, kind := m.nextInput()
	if kind == 0 {
		return false
	}
	defer recoverLog()
	if kind == 1 {
		if rel.flush != nil {
			close(rel.flush)
			return true
		}
		m.postMouse(screencap.MouseUp, rel.x, rel.y, rel.button, 1, 0, 0)
		return true
	}
	m.handleInput(v, msg)
	return true
}

func (m *Manager) postMouse(kind screencap.MouseKind, x, y float64, button, clickState, dx, dy int) {
	if err := m.d.Input.PostMouse(kind, x, y, button, clickState, dx, dy, 0); err != nil {
		m.countInput("post_failed")
	}
}

// cgButton maps a browser button index to the bridge's button (0 left, 1 right, 2 other).
func cgButton(b int) int { return [3]int{0, 2, 1}[b] }

// lowestHeld returns the lowest held browser button, or -1.
func (vi *viewerInput) lowestHeld() int {
	for b := range 3 {
		if _, ok := vi.held[b]; ok {
			return b
		}
	}
	return -1
}

// handleInput is the single path from a viewer's channel to the bridge; v is
// the viewer who sent msg. Presses and moves are gated (stale epoch, locked
// screen); releases of what the viewer holds are never gated, because a
// dropped release strands a button on an unattended Mac. Kinds this plan does
// not post yet are counted.
func (m *Manager) handleInput(v *viewer, msg inputMsg) {
	switch msg.T {
	case inMove:
		m.handleMove(v, msg)
	case inBtn:
		if msg.D {
			m.handlePress(v, msg)
		} else {
			m.handleButtonUp(v, msg)
		}
	case inRelease:
		m.releaseHeld(v, "release")
	case inHB:
	default:
		m.countInput("unhandled")
	}
}

// gate returns the target when a press or move may proceed, else counts the
// drop reason and returns false.
func (m *Manager) gate(msg inputMsg) (inputTarget, bool) {
	t := m.target()
	switch {
	case !t.ok:
		m.countInput("no_target")
	case msg.E != t.epoch:
		m.countInput("stale_epoch")
	case m.Locked():
		m.countInput("locked")
	default:
		return t, true
	}
	return t, false
}

// moveTo records the viewer's new mapped point and returns the movement from
// the last one, with the button to drag with (-1 for a plain move). ok is false
// when the viewer's input life is over (or the Manager is closing).
func (m *Manager) moveTo(v *viewer, x, y float64, press int) (dx, dy, drag int, ok bool) {
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	vi := v.in
	if vi.closed || m.closed.Load() {
		m.in.counts["closing"]++
		return 0, 0, -1, false
	}
	if vi.haveLast {
		dx, dy = int(math.Round(x-vi.lastX)), int(math.Round(y-vi.lastY))
	}
	vi.lastX, vi.lastY, vi.haveLast = x, y, true
	if press >= 0 {
		if vi.held == nil {
			vi.held = make(map[int]struct{}, 3)
		}
		vi.held[press] = struct{}{}
	}
	return dx, dy, vi.lowestHeld(), true
}

func (m *Manager) handleMove(v *viewer, msg inputMsg) {
	t, ok := m.gate(msg)
	if !ok {
		return
	}
	x, y := t.point(msg.X, msg.Y)
	dx, dy, drag, ok := m.moveTo(v, x, y, -1)
	if !ok {
		return
	}
	m.countInput(inMove)
	if drag >= 0 {
		m.postMouse(screencap.MouseDragged, x, y, cgButton(drag), 0, dx, dy)
		return
	}
	m.postMouse(screencap.MouseMoved, x, y, 0, 0, dx, dy)
}

func (m *Manager) handlePress(v *viewer, msg inputMsg) {
	t, ok := m.gate(msg)
	if !ok {
		return
	}
	x, y := t.point(msg.X, msg.Y)
	// The button is recorded as held before it is posted, under the same lock a
	// release snapshots it with, so a viewer leaving now still gets its release.
	dx, dy, _, ok := m.moveTo(v, x, y, msg.B)
	if !ok {
		return
	}
	m.countInput(inBtn)
	m.postMouse(screencap.MouseDown, x, y, cgButton(msg.B), 1, dx, dy)
}

func (m *Manager) handleButtonUp(v *viewer, msg inputMsg) {
	m.in.mu.Lock()
	vi := v.in
	_, held := vi.held[msg.B]
	x, y := vi.lastX, vi.lastY
	if held {
		delete(vi.held, msg.B)
	} else {
		m.in.counts["not_held"]++
	}
	m.in.mu.Unlock()
	if !held {
		return
	}
	m.countInput(inBtn)
	m.postMouse(screencap.MouseUp, x, y, cgButton(msg.B), 1, 0, 0)
}

// releaseHeld lets go of everything v holds (the input goroutine only).
func (m *Manager) releaseHeld(v *viewer, reason string) {
	m.in.mu.Lock()
	rels := m.takeHeldLocked(v.in)
	m.in.mu.Unlock()
	for _, r := range rels {
		m.postMouse(screencap.MouseUp, r.x, r.y, r.button, 1, 0, 0)
	}
	if len(rels) > 0 {
		slog.Debug("desktop: input released", "reason", reason, "buttons", len(rels))
	}
}

// takeHeldLocked clears vi's held set and returns one release per button.
func (m *Manager) takeHeldLocked(vi *viewerInput) []release {
	var out []release
	for b := range 3 {
		if _, ok := vi.held[b]; ok {
			out = append(out, release{button: cgButton(b), x: vi.lastX, y: vi.lastY})
		}
	}
	clear(vi.held)
	return out
}

// ReleaseViewer ends a viewer's input life (it left, its channel closed, the
// Manager is closing): queued messages are discarded, nothing more is accepted,
// and one release per held button is queued for the input goroutine, which
// posts them even after the viewer record is gone. Safe to call twice.
func (m *Manager) ReleaseViewer(id ViewerID, reason string) {
	m.vmu.Lock()
	v := m.viewers[id]
	m.vmu.Unlock()
	if v != nil {
		m.endViewerInput(v, reason)
	}
}

func (m *Manager) endViewerInput(v *viewer, reason string) {
	m.in.mu.Lock()
	v.in.closed = true
	v.in.fifo = nil
	for i, a := range m.in.active {
		if a == v {
			m.in.active = append(m.in.active[:i], m.in.active[i+1:]...)
			break
		}
	}
	rels := m.takeHeldLocked(v.in)
	m.in.rels = append(m.in.rels, rels...)
	m.in.mu.Unlock()
	if len(rels) > 0 {
		m.wakeInput()
		slog.Debug("desktop: input released", "reason", reason, "buttons", len(rels))
	}
}

// releaseAllViewers ends every viewer's input life, then waits up to flushWait
// for the input goroutine to post what it queued.
func (m *Manager) releaseAllViewers(reason string) {
	m.vmu.Lock()
	vs := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		vs = append(vs, v)
	}
	m.vmu.Unlock()
	for _, v := range vs {
		m.endViewerInput(v, reason)
	}
	flush := make(chan struct{})
	m.in.mu.Lock()
	running := m.in.done != nil
	if running {
		m.in.rels = append(m.in.rels, release{flush: flush})
	}
	m.in.mu.Unlock()
	if !running {
		return
	}
	m.wakeInput()
	select {
	case <-flush:
	case <-time.After(flushWait):
		slog.Warn("desktop: input releases not posted before close")
	}
}
