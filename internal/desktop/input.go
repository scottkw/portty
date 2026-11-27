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
	"slices"
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
	PostKey(vk uint16, down bool, flags uint64, autorepeat bool) error
	PostText(units []uint16) error
	PostScroll(dx, dy int, flags uint64) error
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

// viewerInput is one viewer's input state, guarded by Manager.in.mu.
type viewerInput struct {
	fifo         []inputMsg
	held         map[int]struct{}  // browser buttons held
	keys         map[uint16]string // kVK of every key held, with the code it came from
	lastX, lastY float64           // the last mapped global point
	haveLast     bool
	closed       bool              // the viewer's input life is over: nothing more is accepted
	counts       map[string]uint64 // kinds and drop reasons only, for the summary at leave

	lastMsgAt        time.Time // Deps.Now when the last message arrived (the dead-man's clock)
	lastCursorReport time.Time
	cursorPending    *cursorReport // the newest point a throttled report skipped, sent when the window ends
	cursorTimer      bool          // a trailing report is scheduled
	tokens           float64       // the rate bucket
	bucketAt         time.Time

	// Click counting from viewer timestamps: the previous down, and the count
	// each held button was pressed with (its up reuses it).
	lastDownTS, lastDownX, lastDownY float64
	lastButton, clicks               int
	downClicks                       [3]int
}

// release is one queued let-go for the input goroutine (a key when key is set,
// else a button); a non-nil flush is closed when it is reached instead.
type release struct {
	key    bool
	vk     uint16
	flags  uint64
	button int // CG button
	x, y   float64
	flush  chan struct{}
	// A non-nil rv is a request to let go of everything rv holds when the input
	// goroutine reaches it (a right was taken away); why is the log reason.
	rv  *viewer
	why string
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

	// lockUntil: presses are refused until then, set when the Lock Screen chord
	// posts, because the lock poll lags the lock by up to its cache.
	lockUntil time.Time

	ctl controlState // control.go

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

// bump adds one to counts[name], making the map when it is nil.
func bump(counts map[string]uint64, name string) map[string]uint64 {
	if counts == nil {
		counts = make(map[string]uint64)
	}
	counts[name]++
	return counts
}

// countFor counts name for v and for the Manager.
func (m *Manager) countFor(v *viewer, name string) {
	m.in.mu.Lock()
	m.in.counts[name]++
	v.in.counts = bump(v.in.counts, name)
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
	// The viewer learns the control state as soon as the channel is open.
	dc.OnOpen(func() {
		defer recoverLog()
		m.sendControl(dc)
	})
	// pion runs OnMessage inline on the SCTP read loop: parse and hand off only.
	dc.OnMessage(func(dm webrtc.DataChannelMessage) {
		defer recoverLog()
		if !dm.IsString {
			// Binary: only an unlock request (type byte 0x01, then the password
			// bytes), handed over without a string conversion (unlock.go).
			if len(dm.Data) > 0 && dm.Data[0] == unlockMsgType {
				m.handleUnlockBytes(vw, dm.Data)
			} else {
				clear(dm.Data)
				m.countInput("malformed")
			}
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
		// A closed channel must stop counting as open (control sends, cursor
		// reports) and let a new one be accepted; never clear a different one.
		m.vmu.Lock()
		if vw.dc == dc {
			vw.dc = nil
		}
		m.vmu.Unlock()
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
	vi.lastMsgAt = m.d.Now()
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
	tick := time.NewTicker(inputTick)
	defer tick.Stop()
	for {
		for m.runInput() {
		}
		select {
		case <-ctx.Done():
			return
		case <-m.in.wake:
		case <-tick.C:
			m.inputTick()
		}
	}
}

// inputTick is the input goroutine's periodic check: anything a viewer holds is
// let go when the screen locks (a lock has no message from the viewer) and when
// the viewer has sent nothing, heartbeats included, for deadManAfter.
func (m *Manager) inputTick() {
	defer recoverLog()
	now := m.d.Now()
	m.vmu.Lock()
	vs := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		vs = append(vs, v)
	}
	m.vmu.Unlock()
	var locked, polled bool
	for _, v := range vs {
		m.in.mu.Lock()
		holding := len(v.in.held)+len(v.in.keys) > 0
		idle := now.Sub(v.in.lastMsgAt)
		m.in.mu.Unlock()
		switch {
		case !holding:
		case idle >= deadManAfter:
			m.releaseHeld(v, "deadman")
		default:
			if !polled {
				locked, polled = m.inputLocked(), true
			}
			if locked {
				m.releaseHeld(v, "locked")
			}
		}
	}
	m.recoverControl(now)
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
		if rel.rv != nil {
			m.releaseHeld(rel.rv, rel.why)
			return true
		}
		m.postRelease(rel)
		return true
	}
	m.handleInput(v, msg)
	return true
}

func (m *Manager) postMouse(kind screencap.MouseKind, x, y float64, button, clickState, dx, dy int, flags uint64) {
	m.notePointerPost()
	if err := m.d.Input.PostMouse(kind, x, y, button, clickState, dx, dy, flags); err != nil {
		m.countInput("post_failed")
	}
}

func (m *Manager) postRelease(r release) {
	if !r.key {
		x, y := m.clampToWindow(r.x, r.y)
		m.postMouse(screencap.MouseUp, x, y, r.button, 1, 0, 0, r.flags)
		return
	}
	m.postKey(r.vk, false, r.flags, false)
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
// dropped release strands a button on an unattended Mac.
func (m *Manager) handleInput(v *viewer, msg inputMsg) {
	switch msg.T {
	case inMove:
		m.handleMove(v, msg)
	case inRMove:
		m.handleRMove(v, msg)
	case inWheel:
		m.handleWheel(v, msg)
	case inBtn:
		if msg.D {
			m.handlePress(v, msg)
		} else {
			m.handleButtonUp(v, msg)
		}
	case inKey:
		m.handleKey(v, msg)
	case inText:
		m.handleText(v, msg)
	case inRelease:
		m.releaseHeld(v, "release")
	case inHB:
		m.countFor(v, inHB)
	default:
		m.countInput("unhandled")
	}
}

// gate returns the target when a press or move may proceed, else counts the
// drop reason and returns false. A window share has no frame until the window
// gate has read one, so for a window the gate runs first and the target is
// taken after it (the press that reads the frame must not be dropped as no_target).
func (m *Manager) gate(v *viewer, msg inputMsg) (inputTarget, bool) {
	t := m.target()
	win := m.windowShared()
	if !t.ok && !win {
		m.countFor(v, "no_target")
		return t, false
	}
	if !m.allowed(v, msg) {
		return t, false
	}
	if win {
		if t = m.target(); !t.ok {
			m.countFor(v, "no_target")
			return t, false
		}
	}
	return t, true
}

// pointer is what recording a pointer move or press for a viewer yields.
type pointer struct {
	dx, dy int    // movement since the viewer's previous point
	drag   int    // browser button to drag with, -1 for a plain move
	flags  uint64 // the viewer's held modifiers
	clicks int    // a press only: its click state
}

// moveTo records the viewer's new mapped point, and a press of browser button
// press (or -1) with viewer timestamp ts, and returns what to post. ok is false
// when the viewer's input life is over (or the Manager is closing).
func (m *Manager) moveTo(v *viewer, x, y float64, press int, ts float64) (p pointer, ok bool) {
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	vi := v.in
	if vi.closed || m.closed.Load() {
		m.in.counts["closing"]++
		vi.counts = bump(vi.counts, "closing")
		return pointer{drag: -1}, false
	}
	if vi.haveLast {
		p.dx, p.dy = int(math.Round(x-vi.lastX)), int(math.Round(y-vi.lastY))
	}
	vi.lastX, vi.lastY, vi.haveLast = x, y, true
	if press >= 0 {
		if vi.held == nil {
			vi.held = make(map[int]struct{}, 3)
		}
		vi.held[press] = struct{}{}
		p.clicks = nextClickCount(vi.lastDownTS, ts, vi.lastDownX, vi.lastDownY, x, y, vi.lastButton, press, vi.clicks)
		vi.lastDownTS, vi.lastDownX, vi.lastDownY, vi.lastButton, vi.clicks = ts, x, y, press, p.clicks
		vi.downClicks[press] = p.clicks
	}
	p.drag, p.flags = vi.lowestHeld(), modFlags(vi.keys)
	return p, true
}

// postPointer records and posts a move to (x, y) (a drag while a button is
// held) and reports whether it posted.
func (m *Manager) postPointer(v *viewer, kind string, x, y float64) bool {
	p, ok := m.moveTo(v, x, y, -1, 0)
	if !ok {
		return false
	}
	m.countFor(v, kind)
	if p.drag >= 0 {
		m.countFor(v, "drag") // in the leave summary: moves posted with a button held (M-195)
		m.postMouse(screencap.MouseDragged, x, y, cgButton(p.drag), 0, p.dx, p.dy, p.flags)
		return true
	}
	m.postMouse(screencap.MouseMoved, x, y, 0, 0, p.dx, p.dy, p.flags)
	m.scheduleMoveCheck(x, y)
	return true
}

func (m *Manager) handleMove(v *viewer, msg inputMsg) {
	t, ok := m.gate(v, msg)
	if !ok {
		return
	}
	x, y := mapPoint(t, msg.X, msg.Y)
	m.postPointer(v, inMove, x, y)
}

// handleRMove integrates a relative move from the viewer's last point (the
// pointer's own position when that is not on the surface).
func (m *Manager) handleRMove(v *viewer, msg inputMsg) {
	t, ok := m.gate(v, msg)
	if !ok {
		return
	}
	m.in.mu.Lock()
	fx, fy, have := v.in.lastX, v.in.lastY, v.in.haveLast
	m.in.mu.Unlock()
	if !have || fx < t.x || fx > t.x+t.w-1 || fy < t.y || fy > t.y+t.h-1 {
		var err error
		if fx, fy, err = m.d.Input.CursorLocation(); err != nil {
			m.countFor(v, "no_cursor")
			return
		}
	}
	x, y := integrateRelative(t, fx, fy, msg.DX, msg.DY)
	if m.postPointer(v, inRMove, x, y) {
		m.reportCursor(v, t, x, y)
	}
}

func (m *Manager) handlePress(v *viewer, msg inputMsg) {
	t, ok := m.gate(v, msg)
	if !ok {
		return
	}
	x, y := mapPoint(t, msg.X, msg.Y)
	// The button is recorded as held before it is posted, under the same lock a
	// release snapshots it with, so a viewer leaving now still gets its release.
	p, ok := m.moveTo(v, x, y, msg.B, msg.TS)
	if !ok {
		return
	}
	m.countFor(v, inBtn)
	m.postMouse(screencap.MouseDown, x, y, cgButton(msg.B), p.clicks, p.dx, p.dy, p.flags)
}

func (m *Manager) handleButtonUp(v *viewer, msg inputMsg) {
	m.in.mu.Lock()
	vi := v.in
	_, held := vi.held[msg.B]
	x, y, clicks, flags := vi.lastX, vi.lastY, max(vi.downClicks[msg.B], 1), modFlags(vi.keys)
	if held {
		delete(vi.held, msg.B)
	} else {
		m.in.counts["not_held"]++
		vi.counts = bump(vi.counts, "not_held")
	}
	m.in.mu.Unlock()
	if !held {
		return
	}
	// In a window share the up lands at the point clamped to where the window
	// is now (never dropped, so nothing stays held).
	m.refreshWindow()
	x, y = m.clampToWindow(x, y)
	m.countFor(v, inBtn)
	m.postMouse(screencap.MouseUp, x, y, cgButton(msg.B), clicks, 0, 0, flags)
}

func (m *Manager) handleWheel(v *viewer, msg inputMsg) {
	if !m.allowed(v, msg) {
		return
	}
	dx, dy := scrollDelta(msg.DX, msg.DY)
	if dx == 0 && dy == 0 {
		return
	}
	m.in.mu.Lock()
	flags := modFlags(v.in.keys)
	m.in.mu.Unlock()
	m.countFor(v, inWheel)
	if err := m.d.Input.PostScroll(dx, dy, flags); err != nil {
		m.countInput("post_failed")
	}
}

// postKey posts one key event.
func (m *Manager) postKey(vk uint16, down bool, flags uint64, repeat bool) {
	if err := m.d.Input.PostKey(vk, down, flags, repeat); err != nil {
		m.countInput("post_failed")
	}
}

// holdKey records vk as held before it is posted, under the lock a release
// snapshots with, and returns the flags to post (the viewer's held modifiers
// plus alpha-shift when caps). why is "" on success, "closing" when the
// viewer's input life is over and "not_held" for a repeat of a key it does not
// hold (a repeat must never press a key that a release already let go of).
func (m *Manager) holdKey(v *viewer, vk uint16, msg inputMsg) (flags uint64, why string) {
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	vi := v.in
	if vi.closed || m.closed.Load() {
		return 0, "closing"
	}
	if _, held := vi.keys[vk]; !held {
		if msg.R {
			return 0, "not_held"
		}
		if vi.keys == nil {
			vi.keys = make(map[uint16]string, 4)
		}
		vi.keys[vk] = msg.Code
	}
	flags = modFlags(vi.keys)
	if msg.Caps {
		flags |= screencap.FlagAlphaShift
	}
	return flags, ""
}

func (m *Manager) handleKey(v *viewer, msg inputMsg) {
	vk, ok := screencap.KeyCode(msg.Code)
	if !ok {
		m.countFor(v, "unknown_code")
		return
	}
	if !msg.D {
		m.in.mu.Lock()
		_, held := v.in.keys[vk]
		delete(v.in.keys, vk)
		flags := modFlags(v.in.keys)
		m.in.mu.Unlock()
		if !held {
			m.countFor(v, "not_held")
			return
		}
		if msg.Caps {
			flags |= screencap.FlagAlphaShift
		}
		m.countFor(v, inKey)
		m.postKey(vk, false, flags, false)
		return
	}
	m.reconcileModifiers(v, msg.M)
	if !m.allowed(v, msg) {
		return
	}
	flags, why := m.holdKey(v, vk, msg)
	if why != "" {
		m.countFor(v, why)
		return
	}
	if msg.Code == "KeyQ" && flags&screencap.FlagControl != 0 && flags&screencap.FlagCommand != 0 {
		// Ctrl+Cmd+Q locks the screen: refuse presses now, not when the poll notices.
		m.in.mu.Lock()
		m.in.lockUntil = m.d.Now().Add(optimisticLock)
		m.in.mu.Unlock()
	}
	m.countFor(v, inKey)
	m.postKey(vk, true, flags, msg.R)
}

// modifierBits are the wire mask's bits and the flags they stand for.
var modifierBits = [...]struct {
	bit  int
	flag uint64
}{{1, screencap.FlagShift}, {2, screencap.FlagControl}, {4, screencap.FlagOption}, {8, screencap.FlagCommand}}

// reconcileModifiers lets go of every modifier v holds whose bit is clear in
// the key message's mask: the viewer's Mac says it is up, so its key-up was
// lost (a Cmd chord whose Cmd release never arrived must not leave Cmd held).
func (m *Manager) reconcileModifiers(v *viewer, mask int) {
	m.in.mu.Lock()
	var rels []release
	for _, mb := range modifierBits {
		if mask&mb.bit != 0 {
			continue
		}
		for vk, code := range v.in.keys {
			if screencap.ModifierFlag(code) == mb.flag {
				delete(v.in.keys, vk)
				rels = append(rels, release{key: true, vk: vk})
			}
		}
	}
	for i := range rels {
		rels[i].flags = modFlags(v.in.keys)
	}
	m.in.mu.Unlock()
	for _, r := range rels {
		m.countFor(v, "reconciled")
		m.postRelease(r)
	}
}

func (m *Manager) handleText(v *viewer, msg inputMsg) {
	if !m.allowed(v, msg) {
		return
	}
	m.countFor(v, inText)
	for _, chunk := range screencap.UTF16Chunks(msg.S, screencap.MaxTextUnits) {
		if err := m.d.Input.PostText(chunk); err != nil {
			m.countInput("post_failed")
		}
	}
}

// releaseHeld lets go of everything v holds (the input goroutine only).
func (m *Manager) releaseHeld(v *viewer, reason string) {
	m.in.mu.Lock()
	rels := m.takeHeldLocked(v.in)
	m.in.mu.Unlock()
	for _, r := range rels {
		m.postRelease(r)
	}
	if len(rels) > 0 {
		slog.Debug("desktop: input released", "reason", reason, "released", len(rels))
	}
}

// modFlags is the CGEventFlags of the modifier keys in keys.
func modFlags(keys map[uint16]string) (f uint64) {
	for _, code := range keys {
		f |= screencap.ModifierFlag(code)
	}
	return f
}

// takeHeldLocked clears vi's held keys and buttons and returns one release for
// each, in the order they are let go: keys that are not modifiers, then
// modifiers, then buttons.
func (m *Manager) takeHeldLocked(vi *viewerInput) []release {
	var out []release
	vks := make([]uint16, 0, len(vi.keys))
	for vk := range vi.keys {
		vks = append(vks, vk)
	}
	slices.Sort(vks)
	for _, mods := range []bool{false, true} {
		for _, vk := range vks {
			code, held := vi.keys[vk]
			if !held || screencap.IsModifierCode(code) != mods {
				continue
			}
			delete(vi.keys, vk)
			out = append(out, release{key: true, vk: vk, flags: modFlags(vi.keys)})
		}
	}
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
	var summary string
	if reason == "left" && len(v.in.counts) > 0 {
		summary = fmt.Sprint(v.in.counts) // kinds and drop reasons only
	}
	m.in.mu.Unlock()
	if summary != "" {
		slog.Info("desktop: input summary", "viewer", string(v.ID)[:min(8, len(v.ID))], "counts", summary)
	}
	if len(rels) > 0 {
		m.wakeInput()
		slog.Debug("desktop: input released", "reason", reason, "released", len(rels))
	}
}

// releaseAllViewers ends every viewer's input life, then flushes so the input
// goroutine has posted what it queued.
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
	m.flushInput()
}
