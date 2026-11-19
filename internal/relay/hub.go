package relay

import (
	"fmt"
	"io"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	xvt "github.com/charmbracelet/x/vt"
)

// Subscriber represents a single WebSocket connection subscribed to a session's output.
type Subscriber struct {
	// Msgs is the outbound channel for framed messages. Buffered to 256 frames.
	Msgs chan []byte
	// CloseSlow is called in its own goroutine when the Msgs channel is full.
	// Implementations should close the WebSocket and call Unsubscribe.
	CloseSlow func()

	// ReadOnly: if true, input frames from this client are discarded by the server read pump. (MC-03)
	ReadOnly bool

	// Name: optional client identity from ?client= query param. (MC-05)
	Name string

	// Cols, Rows: last reported terminal dimensions from this client.
	// Read/written under hub.mu. (MC-06)
	Cols int
	Rows int

	// Origin: "local" (relay loopback) or "web" (webserver Tailscale). Set once
	// at subscribe time; the host-authority resize arbiter and RemoteViewerCount read it.
	Origin string

	// Kick closes the connection with the given reason (RevokeWebViewers).
	// Called in its own goroutine; nil falls back to CloseSlow.
	Kick func(reason string)

	// ID: per-hub connection number for viewer-diag log lines, assigned by
	// Subscribe before it returns. Diagnostics only.
	ID uint64
	// Remote: the viewer's IP for viewer-diag lines, set by the webserver
	// before Subscribe; empty for local viewers. Diagnostics only.
	Remote string

	// slowLogged: the first slow kick has been logged. Read/written under hub.mu.
	slowLogged bool

	// replaying: Join set it and the caller is still writing the replay, so
	// frames go to pending (capped at ReplayPendingMax bytes) instead of Msgs,
	// which nothing drains yet (WR-01). Read/written under hub.mu.
	replaying    bool
	pending      [][]byte
	pendingBytes int
}

// Hub manages a single PTY session's output fan-out.
// One goroutine (Run) drains the reader; N subscribers receive every frame.
type Hub struct {
	sessionID  string
	reader     io.Reader
	writer     io.Writer
	scrollback *Scrollback
	resizeFn   func(cols, rows int) error

	mu          sync.Mutex
	subscribers map[*Subscriber]struct{}
	nextConnID  uint64 // last Subscriber.ID handed out; under mu
	done        chan struct{}
	closed      bool
	closeOnce   sync.Once

	// ptyCols, ptyRows: current PTY dimensions as set by the host-authority arbiter.
	// Only local-origin subscribers drive these fields; the last host grid is frozen
	// when no local host is connected (D-01). (VIEW-01, VIEW-02)
	ptyCols int
	ptyRows int

	// Phase 175-04 (BUG-04, #119): live per-hub VT emulator + its own guard.
	// liveEmu is nil until EnsureLiveEmulator's first call (lazy — T-175-04-01:
	// never constructed for a session that never gets a shared viewer). Once
	// constructed it lives for the rest of the hub's life, continuously fed by
	// Run()'s drain loop via recordFrameLocked, so its state survives any later wrap
	// of the raw scrollback ring — RenderSnapshot() never re-derives from the
	// (possibly truncated) ring (RESEARCH "Pitfall 3").
	//
	// emuMu is a SEPARATE lock from mu (T-175-04-02). recordFrameLocked runs
	// INLINE in the drain goroutine, so emu.Write must never block: the
	// anti-hang guarantee is NewQuietEmulator's closed response pipe (CR-01),
	// not the query strip. emuMu only keeps the emulator's own state consistent
	// (append+feed vs bootstrap vs resize) without contending on the fan-out mu.
	emuMu   sync.Mutex
	liveEmu *xvt.Emulator

	// history holds the rendered lines that scrolled off the main screen
	// (SHAR-07, Phase 5). It lives here, outside liveEmu, so an emulator
	// rebuild on resize keeps it. Only touched under emuMu.
	history *History
	// modes is the DEC private mode state the app last set (mode number to
	// set/reset), replayed after the screen. Hub-owned so it survives emulator
	// rebuilds, which start from default modes. Only touched under emuMu.
	modes map[int]bool
	// geomEnd is the scrollback end offset at the last genuine host geometry
	// change (0: none). Raw ring bytes are laid out for the width they were
	// written at; replayed at another width, Claude Code's relative redraws
	// land in the wrong rows (05-08, the reattach jumble). Join sends no delta
	// or raw replay that reaches back before it. Only touched under emuMu.
	geomEnd uint64
	// altExited is set by the vt AltScreen callback when the app leaves the
	// alternate screen; recordFrameLocked rebuilds the emulator after the
	// write (rebuildAfterAltExitLocked). Only touched under emuMu.
	altExited bool
	// mainRegion (DECSTBM, 1-based top and bottom; zero: the whole screen) and
	// savedCursor (DECSC; nil: none) are the main screen's, which vt does not
	// expose; the alt-exit rebuild re-applies them (WR-02). Only under emuMu.
	mainRegion  [2]int
	savedCursor *uv.Position
}

// altScreenEnterSeq is the DEC private-mode sequence a full-screen TUI emits
// exactly once on entering the alternate screen buffer (ESC[?1049h).
// RenderSnapshot prefixes it onto the reconnect preamble whenever the live
// emulator reports IsAltScreen(), so a reconnecting/late-joining client's
// xterm.js re-enters the correct buffer before painting replayed content —
// reconstructing the mode marker the raw 256 KiB scrollback ring may have
// wrapped past (BUG-04, #119).
const altScreenEnterSeq = "\x1b[?1049h"

// liveEmuQueryStripPattern matches the reply-eliciting terminal queries and
// the in-band resize mode. Its job is the replay: a joining terminal answers
// every historic query in it, and the answers are typed into the shared shell
// (UAT gap 1). It is NOT what keeps the hub's and the engine's vt emulators
// from hanging: vt honours encodings no pattern covers (8-bit CSI, a query
// split across reads, 2048 inside a mode list), so NewQuietEmulator makes
// every reply fail fast instead (CR-01). The live feed still strips, which
// keeps ?2048 out of h.modes.
//
// Covered: DA1, DA2 and DA3 (CSI c, CSI > c, CSI = c); DSR and DECXCPR
// (CSI n, CSI ? n); DECRQM ANSI and DEC (CSI $ p); in-band resize set/reset
// (CSI ? 2048 h/l, the set makes the terminal report its size); XTVERSION
// (CSI > q); the kitty keyboard query (CSI ? u); XTWINOPS reports
// (CSI 13 t position, 14 t, 14;2 t, 16 t, 18 t, 19 t sizes, 21 t the title,
// the classic echo-back vector); XTGETTCAP (DCS + q); DECRQSS (DCS $ q);
// OSC 10/11/12 colour queries, including the multi-query OSC 10;?;? form;
// OSC 4 palette queries; OSC 52 clipboard reads.
// Window-op sets (CSI 8;r;c t, 22;0 t), OSC 4 sets, OSC 52 writes, sixel DCS
// images, SGR and every other mode set pass through untouched.
//
// It is shared through StripTerminalQueries: internal/daemon imports relay
// (never the reverse), so the hub and the engine strip the same set.
//
// Known limits: the ring stores 0x01-framed PTY reads, so a query split across
// two reads is not matched; an OSC 4 that mixes sets and queries is kept whole;
// a query not listed above is not stripped.
var liveEmuQueryStripPattern = regexp.MustCompile(
	`\x1b\[[0-9;]*c` + // DA1 (and any params before c)
		`|\x1b\[>[0-9;]*c` + // DA2
		`|\x1b\[=[0-9;]*c` + // DA3
		`|\x1b\[[0-9;]*n` + // DSR (5n/6n and any CSI n)
		`|\x1b\[\?[0-9;]*n` + // DECXCPR (ESC[?...n)
		`|\x1b\[[0-9;]*\$p` + // DECRQM ANSI
		`|\x1b\[\?[0-9;]*\$p` + // DECRQM DEC
		`|\x1b\[\?2048[hl]` + // in-band resize set/reset (set triggers a pw write)
		`|\x1b\[>[0-9;]*q` + // XTVERSION
		`|\x1b\[\?u` + // kitty keyboard flags query
		`|\x1b\[(?:1[34689]|21)(?:;[0-9]*)?t` + // XTWINOPS 13/14/16/18/19/21 reports (not the 8/22/23 sets)
		`|\x1bP\+q[0-9A-Fa-f;]*(?:\x1b\\|\x07)` + // XTGETTCAP (hex-encoded names)
		`|\x1bP\$q[\x20-\x7e]*(?:\x1b\\|\x07)` + // DECRQSS (sixel DCS has no $ after P)
		`|\x1b\]1[012](?:;\?)+(?:\x07|\x1b\\)` + // OSC 10/11/12 color query (one or more), BEL- or ST-terminated
		`|\x1b\]4(?:;[0-9]+;\?)+(?:\x07|\x1b\\)` + // OSC 4 palette query (index;? pairs only)
		`|\x1b\]52;[a-z0-9]*;\?(?:\x07|\x1b\\)`, // OSC 52 clipboard read (writes stay)
)

// StripTerminalQueries returns b without the sequences liveEmuQueryStripPattern
// matches. It keeps historic queries in a replay from eliciting replies that
// reach the shared shell; NewQuietEmulator, not this strip, keeps emulator
// writes from hanging. The hub's live emulator feed, both raw replay paths in
// Join and the daemon engine's styled tail all use it, so the hub and the
// engine strip the same set.
func StripTerminalQueries(b []byte) []byte {
	return liveEmuQueryStripPattern.ReplaceAll(b, nil)
}

// NewQuietEmulator returns a vt emulator whose query replies are discarded.
// vt writes each reply (DA, DSR, DECRQM, OSC 10-12, the ?2048 report) into an
// unbuffered io.Pipe and blocks Write until someone reads it; with the pipe
// writer closed every reply returns ErrClosedPipe at once, which vt ignores,
// so Write never blocks whatever bytes it is fed (CR-01). Closing the pipe,
// not emu.Close, leaves Write working and needs no reader goroutine, so the
// #100 Read/Close race on e.closed cannot come back. Use it for every
// emulator whose replies nobody wants.
func NewQuietEmulator(cols, rows int) *xvt.Emulator {
	emu := xvt.NewEmulator(cols, rows)
	_ = emu.InputPipe().(io.Closer).Close()
	return emu
}

// NewHub constructs a Hub for the given session.
// scrollbackBytes controls the scrollback buffer capacity.
// resizeFn is called when a resize event is received; may be nil.
func NewHub(sessionID string, reader io.Reader, writer io.Writer, scrollbackBytes int, resizeFn func(cols, rows int) error) *Hub {
	return &Hub{
		sessionID:   sessionID,
		reader:      reader,
		writer:      writer,
		scrollback:  NewScrollback(scrollbackBytes),
		resizeFn:    resizeFn,
		subscribers: make(map[*Subscriber]struct{}),
		done:        make(chan struct{}),
		history:     NewHistory(HistoryMaxLines, HistoryMaxBytes),
		modes:       make(map[int]bool),
	}
}

// DiagID is a session ID shortened for viewer-diag log lines.
func DiagID(sessionID string) string {
	if len(sessionID) > 8 {
		return sessionID[:8]
	}
	return sessionID
}

// Resize calls the resize callback registered at construction time.
// If no callback was provided it is a no-op.
func (h *Hub) Resize(cols, rows int) error {
	if h.resizeFn != nil {
		return h.resizeFn(cols, rows)
	}
	return nil
}

// Subscribe adds a subscriber to receive future frames.
// Must be called before Hub.Run or while Run is active under the mu lock.
func (h *Hub) Subscribe(sub *Subscriber) {
	h.mu.Lock()
	h.subscribers[sub] = struct{}{}
	h.nextConnID++
	sub.ID = h.nextConnID
	total, web := len(h.subscribers), h.webCountLocked()
	h.mu.Unlock() // unlock-before-IO: no log write under mu
	log.Printf("viewer-diag: join session=%s conn=%d origin=%s from=%s client=%q viewers=%d web=%d",
		DiagID(h.sessionID), sub.ID, sub.Origin, sub.Remote, sub.Name, total, web)
}

// Unsubscribe removes a subscriber.
func (h *Hub) Unsubscribe(sub *Subscriber) {
	h.mu.Lock()
	_, present := h.subscribers[sub]
	delete(h.subscribers, sub)
	total := len(h.subscribers)
	h.mu.Unlock()
	if present {
		log.Printf("viewer-diag: leave session=%s conn=%d origin=%s client=%q viewers=%d",
			DiagID(h.sessionID), sub.ID, sub.Origin, sub.Name, total)
	}
}

// webCountLocked counts Origin=="web" subscribers. Caller holds h.mu.
func (h *Hub) webCountLocked() int {
	count := 0
	for s := range h.subscribers {
		if s.Origin == "web" {
			count++
		}
	}
	return count
}

// kickSlow closes a subscriber whose channel is full, in its own goroutine as
// before, and logs the first kick per subscriber. Caller holds h.mu; the log
// write happens in the goroutine so no I/O runs under mu (T-157-04).
func (h *Hub) kickSlow(sub *Subscriber) {
	first := !sub.slowLogged
	sub.slowLogged = true
	queued := len(sub.Msgs)
	go func() {
		if first {
			log.Printf("viewer-diag: slow session=%s conn=%d origin=%s client=%q queued=%d",
				DiagID(h.sessionID), sub.ID, sub.Origin, sub.Name, queued)
		}
		sub.CloseSlow()
	}()
}

// sendLocked queues frame for sub without blocking; a full queue kicks it.
// Caller holds h.mu.
func (h *Hub) sendLocked(sub *Subscriber, frame []byte) {
	if sub.replaying {
		if sub.pendingBytes+len(frame) > ReplayPendingMax {
			h.kickSlow(sub)
			return
		}
		sub.pending = append(sub.pending, frame)
		sub.pendingBytes += len(frame)
		return
	}
	select {
	case sub.Msgs <- frame:
	default:
		h.kickSlow(sub)
	}
}

// TakePending returns the frames held for sub since Join (or the last call),
// in order, to be written after the replay. Once nothing is held it ends the
// replay and returns nil: later frames go to Msgs. Callers loop until nil.
func (h *Hub) TakePending(sub *Subscriber) [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	frames := sub.pending
	sub.pending, sub.pendingBytes = nil, 0
	if len(frames) == 0 {
		sub.replaying = false
	}
	return frames
}

// SubscriberCount returns the number of currently subscribed clients. (MC-04)
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

// RemoteViewerCount returns the number of currently subscribed clients whose
// Origin is "web" — i.e. real remote/shared viewers, excluding the app's own
// internal Origin=="local" WebSocket subscribers (TerminalPanel, ChatPanel,
// status watcher, Hub-card preview). This is a raw per-connection count
// (D-01, D-02, D-03 — Phase 168 / FIX-04, #121). SubscriberCount itself is left unchanged; it is still
// consumed by relay/server.go's NotifyViewerCount.
func (h *Hub) RemoteViewerCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.webCountLocked()
}

// RevokeWebViewers closes every currently subscribed Origin=="web"
// subscriber with reason (via Kick, else CloseSlow), leaving Origin=="local"
// subscribers (the app's own terminal connections) untouched (D-05, FIX-02).
// The caller revokes the capability first (ClearGrants), so a kicked viewer
// cannot reconnect with the same link.
//
// Matching subscribers are collected under h.mu, the lock is released, and
// ONLY THEN is each close called. This unlock-before-IO ordering is
// mandatory: closing calls back into Unsubscribe, which re-enters h.mu —
// calling it while still holding h.mu self-deadlocks (T-157-04).
func (h *Hub) RevokeWebViewers(reason string) {
	h.mu.Lock()
	targets := make([]*Subscriber, 0, len(h.subscribers))
	for s := range h.subscribers {
		if s.Origin == "web" {
			targets = append(targets, s)
		}
	}
	h.mu.Unlock()
	log.Printf("viewer-diag: revoke session=%s reason=%q web=%d", DiagID(h.sessionID), reason, len(targets))

	for _, sub := range targets {
		if sub.Kick != nil {
			go sub.Kick(reason)
		} else {
			go sub.CloseSlow()
		}
	}
}

// ResizeClient stores the subscriber's reported dimensions and applies the
// host-authority PTY-size policy (VIEW-01, VIEW-02):
//
//   - VIEW-02 origin gate: non-local (web/remote) subscribers return immediately;
//     their reported size never enters the arbiter and cannot drive or DoS the PTY
//     grid (T-157-01 mitigation).
//   - D-02 min-among-local: the PTY grid tracks the smallest terminal among all
//     connected local-origin subscribers so every host viewer sees the full output.
//   - D-01 freeze: when no local subscriber currently reports a positive size the
//     last host grid is preserved — the PTY is never reset to zero or to a guest size.
//   - VIEW-01 broadcast: each grid change is broadcast to all subscribers via a
//     MsgResize (0x02) frame before resizeFn is called, so guests learn the new
//     authoritative grid immediately.
//
// broadcastResize and resizeFn are called AFTER releasing hub.mu (unlock-before-IO
// discipline) to avoid self-deadlock and PTY-syscall contention.
func (h *Hub) ResizeClient(sub *Subscriber, cols, rows int) error {
	// VIEW-02: non-local origin is a read-only guest — it may not drive the PTY grid.
	// Return immediately; the guest's reported size is not recorded anywhere.
	if sub.Origin != "local" {
		return nil
	}

	h.mu.Lock()
	sub.Cols = cols
	sub.Rows = rows

	// D-02 min-among-local: iterate only local-origin subscribers with a positive size.
	minCols, minRows := 0, 0
	for s := range h.subscribers {
		if s.Origin != "local" || s.Cols <= 0 || s.Rows <= 0 {
			continue
		}
		if minCols == 0 || s.Cols < minCols {
			minCols = s.Cols
		}
		if minRows == 0 || s.Rows < minRows {
			minRows = s.Rows
		}
	}

	// D-01 freeze: if no local subscriber reports a positive size, leave ptyCols/ptyRows
	// unchanged (last host grid persists). Only update when we have a valid min.
	needResize := (minCols > 0 && minRows > 0) && (minCols != h.ptyCols || minRows != h.ptyRows)
	oldCols, oldRows := h.ptyCols, h.ptyRows
	if needResize {
		h.ptyCols = minCols
		h.ptyRows = minRows
	}
	pc, pr := h.ptyCols, h.ptyRows
	h.mu.Unlock() // unlock-before-IO: broadcastResize and resizeFn must not run under mu

	if needResize {
		// Diagnostic for UAT gap 1 (04-07 H-grid): who changed the host grid, and to what.
		log.Printf("viewer-diag: grid session=%s %dx%d -> %dx%d by=%s client=%q", DiagID(h.sessionID), oldCols, oldRows, pc, pr, sub.Origin, sub.Name)
		// VIEW-01: broadcast the new authoritative grid to all subscribers.
		h.broadcastResize(uint16(pc), uint16(pr))
		// CR-01: keep the live emulator's geometry in lockstep with the PTY so a
		// later reconnect preamble (RenderSnapshot) is not mis-dimensioned. Safe
		// here — hub.mu is already released (resizeLiveEmulator takes only emuMu).
		// Genuine only when the hub already had a real grid: the first resize from
		// the unknown grid matches the size the PTY was spawned at.
		h.resizeLiveEmulator(pc, pr, oldCols > 0 && oldRows > 0)
		if h.resizeFn != nil {
			return h.resizeFn(minCols, minRows)
		}
	}
	return nil
}

// broadcastResize sends a MsgResize (0x02) frame encoding cols and rows to all
// subscribers using a non-blocking send. Slow subscribers (full channel) have
// CloseSlow called in a new goroutine. Mirrors BroadcastMeta (hub.go:306).
//
// MUST be called after hub.mu is released (self-acquires mu internally; calling
// while holding mu causes a self-deadlock — T-157-04 mitigation).
func (h *Hub) broadcastResize(cols, rows uint16) {
	frame := MakeResizeFrame(cols, rows)
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subscribers {
		h.sendLocked(sub, frame)
	}
}

// Run is the drain goroutine. It reads from the PTY reader in 32 KiB chunks,
// wraps each chunk in a MakeOutputFrame, appends to scrollback, and broadcasts
// to all subscribers. Slow subscribers (full channel) have CloseSlow called in a
// separate goroutine so they cannot block the drain loop.
//
// Run returns when the reader returns any error (typically io.EOF on PTY close).
// On return, Run calls Shutdown() to signal completion.
func (h *Hub) Run() {
	defer h.Shutdown()

	buf := make([]byte, 32*1024)
	for {
		n, err := h.reader.Read(buf)
		if n > 0 {
			// Copy the live slice before broadcasting — each frame allocation
			// is independent so slow subscribers cannot corrupt fast ones.
			frame := MakeOutputFrame(buf[:n])
			// CR-03 (code review): append to scrollback AND feed the live
			// emulator as ONE atomic step under emuMu — see recordFrameLocked.
			// This prevents a boundary frame being counted twice (once by
			// EnsureLiveEmulator's bootstrap ScrollbackSnapshot, once by the live
			// feed), the TestHub_TwoClientsFanOut "hello worldhello world" race.
			//
			// SHAR-07 exactly-once: the broadcast happens under the same emuMu
			// hold, the lock Join subscribes and captures under, so a frame lands
			// either in a joining viewer's replay or in its queue, never both and
			// never neither (recording outside the broadcast let one frame be
			// duplicated). broadcast never blocks (slow viewers are kicked in their
			// own goroutine), so holding emuMu never waits on a viewer. Lock order
			// stays emuMu then hub.mu.
			h.emuMu.Lock()
			h.recordFrameLocked(frame, buf[:n])
			h.broadcast(frame)
			h.emuMu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// recordFrameLocked appends the framed bytes to the scrollback ring AND feeds the
// raw (pre-framing) PTY bytes to the live emulator (if one has been built) as ONE
// atomic step. The caller (Run's drain loop) holds emuMu, NOT hub.mu (emuMu is a
// separate lock, T-175-04-02).
//
// Atomicity vs EnsureLiveEmulator's bootstrap is REQUIRED (CR-03, code review):
// EnsureLiveEmulator builds the emulator and bootstraps it once from
// ScrollbackSnapshot() while holding emuMu. If the scrollback append and the
// live feed were separate operations, a frame could be appended to scrollback,
// picked up by a concurrent bootstrap, and THEN fed again by the live feed —
// double-counting it (the TestHub_TwoClientsFanOut "hello worldhello world"
// race). Holding emuMu across both here guarantees each frame is captured
// exactly once: either it is already in the scrollback the bootstrap reads (and
// the nil-guarded feed below no-ops because liveEmu is not yet set) OR it
// arrives after liveEmu is set (and only the live feed applies it).
//
// emu.Write runs inline in the drain loop, so it must never block: the live
// emulator comes from NewQuietEmulator, whose replies fail fast (CR-01).
func (h *Hub) recordFrameLocked(frame, raw []byte) {
	h.scrollback.Append(frame)
	if h.liveEmu == nil {
		// EAGER BUILD (M-51 fix): construct the emulator on the FIRST frame and
		// feed it from byte 1, rather than lazily at the first guest connect.
		// Being fed since the first PTY byte is the ONLY way the emulator can
		// hold a COMPLETE current screen after the raw ring wraps: a full-screen
		// app (top/htop) writes structural header rows ONCE via absolute cursor
		// positioning and thereafter only repaints them in place, so a lazy
		// bootstrap from an already-truncated ring never sees them and renders a
		// garbled header. No bootstrap-from-scrollback is needed — this feed IS
		// the complete history. Later PTY resizes are tracked by resizeLiveEmulator
		// (CR-01). Lock order matches EnsureLiveEmulator (emuMu → hub.mu via
		// Cols()/Rows()); no inversion.
		h.liveEmu = h.newLiveEmulator(h.Cols(), h.Rows())
	}
	clean := StripTerminalQueries(raw)
	h.liveEmu.Write(clean) //nolint:errcheck // emulator Write never returns a meaningful error
	h.drainScrollbackLocked()
	if h.altExited {
		h.altExited = false
		// Re-entered in the same read: the next exit rebuilds.
		if !h.liveEmu.IsAltScreen() {
			h.rebuildAfterAltExitLocked(incompleteTail(clean))
		}
	}
}

// incompleteTail returns the bytes after b's last ground-state point: an
// escape sequence (ESC, CSI, OSC, DCS, SOS, PM, APC) or UTF-8 character the
// read cut off, or nil. It scans as vt's parser does, bytewise with C1
// controls, assuming ground at b's start. That can be wrong only before an
// ESC or C1 introducer, which resets the state, and the alt-exit sequence the
// rebuild follows is one, so the tail after it is exact (WR-02).
func incompleteTail(b []byte) []byte {
	const (
		ground = iota
		esc
		csi
		osc
		str // DCS, SOS, PM, APC: ST-terminated only
		strEsc
	)
	state, need, last := ground, 0, 0
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c == 0x18 || c == 0x1a: // CAN, SUB abort any sequence
			state, need = ground, 0
		case need > 0 && c&0xc0 == 0x80:
			need--
		case state == ground:
			need = 0
			switch {
			case c == 0x1b:
				state = esc
			case c == 0x9b:
				state = csi
			case c == 0x9d:
				state = osc
			case c == 0x90 || c == 0x98 || c == 0x9e || c == 0x9f:
				state = str
			case c&0xe0 == 0xc0:
				need = 1
			case c&0xf0 == 0xe0:
				need = 2
			case c&0xf8 == 0xf0:
				need = 3
			}
		case state == esc:
			switch {
			case c == '[':
				state = csi
			case c == ']':
				state = osc
			case c == 'P' || c == 'X' || c == '^' || c == '_':
				state = str
			case c >= 0x30 && c <= 0x7e:
				state = ground
			} // intermediates, C0 controls and ESC itself stay in esc
		case state == csi:
			if c == 0x1b {
				state = esc
			} else if c >= 0x40 && c <= 0x7e {
				state = ground
			}
		case state == osc || state == str:
			if c == 0x9c || (state == osc && c == 0x07) {
				state = ground
			} else if c == 0x1b {
				state = strEsc
			}
		case state == strEsc:
			if c == '\\' {
				state = ground
			} else {
				state = esc // the ESC ended the string and starts a sequence
				i--
				continue
			}
		}
		if state == ground && need == 0 {
			last = i + 1
		}
	}
	if last == len(b) {
		return nil
	}
	return b[last:]
}

// rebuildAfterAltExitLocked replaces the live emulator once the app has left
// the alternate screen, so the alt screen's scrollback is freed (HARD-01).
// vt gives each screen its own 10,000-line scrollback, and SetScrollbackSize,
// Scrollback and ClearScrollback reach only the main screen, so the alt
// screen's lines go only with the emulator. Caller holds emuMu.
//
// Unlike resizeLiveEmulator it re-seeds the new emulator with the old main
// screen rather than pushing the rows into history: no SIGWINCH redraw follows
// an alt-screen exit, so the main screen must stay where the host shows it.
// h.history, h.modes and geomEnd are Hub-owned and stay (not a geometry change).
//
// WR-02: the rendered seed carries text, cursor and DEC modes only, so the
// rebuild also re-applies the main screen's scroll region and saved cursor
// (tracked by newLiveEmulator's handlers), the current pen and the DEC line
// drawing G0 charset (read by printing one probe cell into the old emulator,
// which is discarded anyway), and then replays tail, the sequence or UTF-8
// character the read cut off, so the next read completes it.
// shortcut: tab stops, G1-G3/SO, DECSC's saved pen and a pending wrap after a
// region or saved-cursor restore are lost; track them if a shell needs them.
func (h *Hub) rebuildAfterAltExitLocked(tail []byte) {
	old := h.liveEmu
	seed := h.renderScreenLocked()
	pos := old.CursorPosition()
	// CAN first: the probe must start from ground whatever tail the read left
	// in the old parser (a bare DCS introducer swallowed the CUP, WR-05).
	old.Write([]byte("\x18\x1b[1;1H")) //nolint:errcheck // emulator Write never returns a meaningful error
	probe := old.CursorPosition()  // region-relative under origin mode
	old.Write([]byte("q"))         //nolint:errcheck // 'q' is '─' in DEC line drawing
	cell := old.CellAt(probe.X, probe.Y)

	h.liveEmu = h.newLiveEmulator(old.Width(), old.Height())
	var b strings.Builder
	b.Write(seed)
	if r, s := h.mainRegion, h.savedCursor; r != [2]int{} || s != nil {
		if r != [2]int{} {
			fmt.Fprintf(&b, "\x1b[%d;%dr", r[0], r[1]) // homes the cursor
		}
		if s != nil {
			fmt.Fprintf(&b, "\x1b[%d;%dH\x1b7", s.Y+1, s.X+1)
		}
		fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
	}
	if cell != nil {
		if cell.Content == "─" {
			b.WriteString("\x1b(0")
		}
		b.WriteString(cell.Style.String())
	}
	h.liveEmu.Write([]byte(b.String())) //nolint:errcheck // emulator Write never returns a meaningful error
	h.drainScrollbackLocked()           // the seed fills exactly Height rows, so nothing scrolls
	h.liveEmu.Write(tail)               //nolint:errcheck // emulator Write never returns a meaningful error
}

// newLiveEmulator builds the live emulator. It is the only constructor, so a
// rebuild (resizeLiveEmulator) can never miss a setting (RESEARCH Pitfall 12).
//
// Its scrollback max equals HistoryMaxLines, though the real history lives in
// h.history: drainScrollbackLocked empties the emulator scrollback after every
// write, so it stays near empty, and the large max only means one 32 KiB read
// that scrolls many lines is not truncated before the drain. Keeping 10,000
// lines in the emulator itself would cost about 126 MiB per session (uv.Cell
// is 112 bytes); rendered strings cost about 1.1 MiB (RESEARCH Pitfall 1).
//
// The hooks below run inside emu.Write, which only runs under emuMu, so they
// touch h.history and h.modes safely. Each handler returns false so the
// emulator's default handling still runs.
func (h *Hub) newLiveEmulator(cols, rows int) *xvt.Emulator {
	emu := NewQuietEmulator(cols, rows)
	// shortcut: while an app stays in the alt screen its scrollback is capped
	// only by vt's DefaultScrollbackSize, because vt does not expose it;
	// rebuildAfterAltExitLocked frees it on exit. The cap is per session and
	// scales with width: 10,000 lines x cols x about 112-120 B (the uv.Cell),
	// so about 128 MiB per session at 100 columns and about 270 MiB at the
	// 220-column fallback Cols() uses before a local viewer sizes the PTY;
	// three sessions in `less +F` overnight can hold 0.4-0.8 GiB. Bound it
	// (vendor a vt change for scrs[1], or rebuild every N alt lines) if a
	// long-lived alt-screen session shows up on the 8 GB Macs (WR-04).
	emu.SetScrollbackSize(HistoryMaxLines)
	// Track DEC private modes for the replay's restores; ANSI modes are not
	// replayed (the callbacks report both).
	// WR-02: the main screen's saved cursor, for the alt-exit rebuild.
	saveCursor := func() {
		if !emu.IsAltScreen() {
			p := emu.CursorPosition()
			h.savedCursor = &p
		}
	}
	track := func(on bool) func(ansi.Mode) {
		return func(m ansi.Mode) {
			if d, ok := m.(ansi.DECMode); ok {
				h.modes[int(d)] = on
				if on && d == ansi.ModeSaveCursor {
					saveCursor()
				}
			}
		}
	}
	emu.SetCallbacks(xvt.Callbacks{
		EnableMode:  track(true),
		DisableMode: track(false),
		// Runs inside emu.Write: only flag the exit, recordFrameLocked rebuilds.
		AltScreen: func(on bool) {
			if !on {
				h.altExited = true
				// ?1049h saved the main cursor, which stood still in the alt
				// screen; the mode is still set while this callback runs.
				if h.modes[int(ansi.ModeAltScreenSaveCursor)] {
					saveCursor()
				}
			}
		},
	})
	emu.RegisterEscHandler('7', func() bool { saveCursor(); return false })
	emu.RegisterCsiHandler('s', func(ansi.Params) bool {
		if !h.modes[69] { // with DECLRMM, CSI s sets margins instead
			saveCursor()
		}
		return false
	})
	emu.RegisterCsiHandler('r', func(params ansi.Params) bool {
		if emu.IsAltScreen() {
			return false // the alt screen's region goes with it
		}
		top, _, _ := params.Param(0, 1)
		bottom, _, _ := params.Param(1, emu.Height())
		top, bottom = max(top, 1), min(bottom, emu.Height())
		if bottom < 1 {
			bottom = emu.Height()
		}
		if top < bottom { // vt ignores the rest
			h.mainRegion = [2]int{top, bottom}
			if top == 1 && bottom == emu.Height() {
				h.mainRegion = [2]int{}
			}
		}
		return false
	})
	// ESC[3J (zsh `clear`, Claude Code's /clear) erases scrollback: empty the
	// history too, so a cleared transcript is never replayed (T-05-10).
	emu.RegisterCsiHandler('J', func(params ansi.Params) bool {
		if n, _, _ := params.Param(0, 0); n == 3 {
			h.history.Clear()
		}
		return false
	})
	// RIS (ESC c) resets the terminal: empty the history and the mode state.
	// The default reset does not touch the emulator scrollback, so clear lines
	// scrolled earlier in this same write as well. The default handler then
	// reports every mode at its default, which refills h.modes.
	emu.RegisterEscHandler('c', func() bool {
		h.history.Clear()
		clear(h.modes)
		h.mainRegion, h.savedCursor = [2]int{}, nil
		emu.ClearScrollback()
		return false
	})
	return emu
}

// drainScrollbackLocked moves the lines that scrolled off the emulator's main
// screen into h.history as rendered ANSI strings. Caller holds emuMu.
//
// SHAR-07 / Phase 5: this is what makes "restores scrollback" true after the
// raw ring wraps. Lines() returns the scrollback's internal slice, and Clear()
// only reslices it to zero length, so every old uv.Line stays reachable through
// the backing array: zero the full capacity first (RESEARCH Pitfall 2, a
// 9,000-line burst kept about 110 MiB alive without it).
func (h *Hub) drainScrollbackLocked() {
	sb := h.liveEmu.Scrollback()
	if sb.Len() == 0 {
		return
	}
	ls := sb.Lines()
	for _, l := range ls {
		h.history.Push(l.Render())
	}
	clear(ls[:cap(ls)])
	sb.Clear()
}

// resizeLiveEmulator propagates an authoritative PTY resize to the live per-hub
// VT emulator (CR-01) by DISCARDING the old emulator and rebuilding an EMPTY one
// at the new geometry — NOT by calling xvt.Emulator.Resize() in place.
//
// Why rebuild instead of resize (M-51, debug session m51-top-header-garble):
// xvt.Emulator.Resize() is a DESTRUCTIVE, non-reflow grid truncate/pad (shrink
// drops columns/rows via slice truncation; grow pads with blanks). After a
// mid-session geometry change, content laid out for the PRE-resize geometry
// survives on the wrong rows. A full-screen app (top/htop/vim) FULLY redraws on
// SIGWINCH — the real PTY resize (resizeFn, called right after this in
// ResizeClient) delivers that SIGWINCH — so the correct post-resize screen is
// derivable ENTIRELY from the redraw bytes that follow. Any surviving pre-resize
// content is stale, wrong-geometry garbage that never self-heals (top only
// re-lays-out its header on resize, then patches numbers in place forever after),
// which was the residual M-51 header garble. Starting empty lets the imminent
// SIGWINCH redraw repopulate a clean screen.
//
// The common case (no resize for the session's life) is unaffected: the eager
// build in recordFrameLocked feeds a single emulator continuously, so completeness
// after a raw-ring wrap (the original BUG-04 fix) is preserved. recordFrameLocked's
// nil-guard means if output arrives before this rebuild's first frame, the feed
// simply lands in the freshly-built emulator.
//
// Takes ONLY emuMu (never hub.mu): callers acquire emuMu→hub.mu via Cols()/Rows()
// elsewhere, so this MUST be called after hub.mu is released, or the reverse lock
// order would deadlock. No-op until the emulator exists (recordFrameLocked will build
// at the current geometry on the first frame).
func (h *Hub) resizeLiveEmulator(cols, rows int, genuine bool) {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	if h.liveEmu == nil {
		return
	}
	if genuine {
		h.geomEnd = h.scrollback.End()
	}
	// SHAR-07 / Phase 5: the old emulator's screen is about to be discarded, so
	// keep a main screen's rows in history first: drain what already scrolled,
	// then push the visible rows down to the cursor row (below it is blank or
	// stale). The shell redraws its prompt after SIGWINCH, so a few prompt
	// lines can appear twice in history; accepted (CONTEXT open question).
	// An alt-screen TUI's rows are not history and are simply redrawn, but the
	// main screen behind it is kept the same way, and the new emulator enters
	// the alt screen, so the redraw stays out of history and the app's exit
	// still fires the HARD-01 rebuild (WR-03).
	old := h.liveEmu
	inAlt := old.IsAltScreen()
	if inAlt {
		old.Write([]byte("\x1b[?1049l")) //nolint:errcheck // emulator Write never returns a meaningful error
		h.altExited = false              // our exit, not the app's
	}
	h.drainScrollbackLocked()
	if old.String() != "" {
		lines := strings.Split(old.Render(), "\n")
		for _, row := range lines[:min(old.CursorPosition().Y+1, len(lines))] {
			h.history.Push(row)
		}
	}
	h.mainRegion, h.savedCursor = [2]int{}, nil // the new emulator starts without them
	h.liveEmu = h.newLiveEmulator(cols, rows)
	if inAlt {
		h.liveEmu.Write([]byte(altScreenEnterSeq)) //nolint:errcheck // emulator Write never returns a meaningful error
	}
}

// broadcast sends frame to all current subscribers using a non-blocking send.
// Subscribers with a full channel have their CloseSlow invoked in a new goroutine.
func (h *Hub) broadcast(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		h.sendLocked(sub, frame)
	}
}

// BroadcastMeta sends a metadata frame to all subscribers using a non-blocking
// send. Slow subscribers have CloseSlow called — MsgMeta must never block the
// PTY drain loop. Used by relay.Server and webserver to push viewer count updates.
func (h *Hub) BroadcastMeta(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		h.sendLocked(sub, frame)
	}
}

// Shutdown signals that the hub has stopped. Safe to call multiple times.
func (h *Hub) Shutdown() {
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		close(h.done)
	})
}

// WriteInput writes raw input bytes to the underlying PTY writer (stdin).
func (h *Hub) WriteInput(data []byte) error {
	_, err := h.writer.Write(data)
	return err
}

// ScrollbackSnapshot returns a copy of the current scrollback buffer contents.
// Callers should Subscribe before calling ScrollbackSnapshot to avoid missing
// frames written between the snapshot and the first message in Msgs.
func (h *Hub) ScrollbackSnapshot() []byte {
	return h.scrollback.Snapshot()
}

// EnsureLiveEmulator is a safety no-op now that the emulator is built EAGERLY
// from the first PTY frame in recordFrameLocked (M-51 fix). Any session that has
// produced ANY output already has a live emulator that was fed since byte 1, so
// this returns immediately. It survives only for the degenerate case of a
// connection accepted BEFORE any PTY output has flowed: there it constructs an
// EMPTY emulator so RenderSnapshot has something to render. It deliberately does
// NOT bootstrap from ScrollbackSnapshot — that path was the M-51 bug (bootstrap
// from an already-wrapped ring misses the one-time header writes), and after the
// eager build it is dead code: liveEmu is non-nil the moment any frame arrives.
//
// Join calls its locked variant before rendering; it remains idempotent and cheap.
func (h *Hub) EnsureLiveEmulator() {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	h.ensureLiveEmulatorLocked()
}

// ensureLiveEmulatorLocked is EnsureLiveEmulator's body. Caller holds emuMu.
func (h *Hub) ensureLiveEmulatorLocked() {
	if h.liveEmu != nil {
		return
	}

	// No output has flowed yet (recordFrameLocked never ran) — build an empty emulator
	// at the current fallback geometry so RenderSnapshot is well-defined. The
	// drain loop's recordFrameLocked will feed this same instance once bytes arrive.
	h.liveEmu = h.newLiveEmulator(h.Cols(), h.Rows())
}

// RenderSnapshot returns the live emulator's current screen as a
// MsgOutput-framed reconnect preamble (BUG-04, #119): when the emulator
// reports IsAltScreen(), altScreenEnterSeq is prefixed so a reconnecting or
// late-joining client's xterm.js re-enters alternate-screen mode BEFORE
// painting the replayed content — reconstructing the mode marker the raw
// scrollback ring may have wrapped past.
//
// Callers MUST call EnsureLiveEmulator before RenderSnapshot — this method
// does not construct the emulator itself. Returns nil if no emulator has
// been constructed yet, OR if there is genuinely nothing to show (fresh
// session, no PTY output at all, main screen) — matching the pre-existing
// scrollback-replay contract's `len(snapshot) > 0` guard at both call sites,
// so connecting to a brand-new session does not emit a spurious empty
// preamble frame ahead of the first real PTY output.
func (h *Hub) RenderSnapshot() []byte {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	content := h.renderScreenLocked()
	if content == nil {
		return nil
	}
	return MakeOutputFrame(content)
}

// renderScreenLocked returns the live emulator's current screen, unframed, or
// nil when there is no emulator or nothing to show. Caller holds emuMu.
func (h *Hub) renderScreenLocked() []byte {
	if h.liveEmu == nil {
		return nil
	}

	altScreen := h.liveEmu.IsAltScreen()
	if h.liveEmu.String() == "" && !altScreen {
		// Nothing to show yet, and no mode marker to reconstruct — sending an
		// empty frame would be pure overhead and would break the "no data
		// received before real output arrives" contract callers rely on.
		return nil
	}

	// SHAR-07 / Phase 5: lay the screen out exactly like the host in xterm.js,
	// which runs with convertEol off (LF only moves down). Render() joins rows
	// with a bare LF, which staircased every wrapped replay, so rows are joined
	// with CRLF instead. All Height() rows are sent, trailing blank rows
	// included, so the viewport fills like the host's, and a CUP puts the cursor
	// where the host's is (Render() trims trailing spaces, so the text alone
	// leaves it short, e.g. after "% "). RESEARCH Pattern 5, Pitfall 4.
	var b strings.Builder
	if altScreen {
		b.WriteString(altScreenEnterSeq)
	}
	b.WriteString(strings.ReplaceAll(h.liveEmu.Render(), "\n", "\r\n"))
	pos := h.liveEmu.CursorPosition()
	fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
	// A host cursor at the last column after printing there is in the pending
	// (deferred) wrap state, which a CUP clears: re-print that cell so the
	// viewer's next character wraps as the host's does. vt does not expose the
	// state, so this infers it.
	// shortcut: a cursor moved onto a filled last column (not printed there) is
	// replayed as pending wrap too; track the state per write if that shows up.
	if aw, ok := h.modes[7]; (aw || !ok) && pos.X == h.liveEmu.Width()-1 {
		if c := h.liveEmu.CellAt(pos.X, pos.Y); c != nil && c.Width == 1 && c.Content != "" && c.Content != " " {
			b.WriteString(uv.Line{*c}.Render())
		}
	}
	// Restore the app's DEC private modes (bracketed paste, cursor keys, cursor
	// visibility, mouse) in ascending order, so the viewer's xterm behaves like
	// the host even though the ring wrapped past the sets. Never the alt-screen
	// modes (handled by the prefix above) or 2048 (in-band resize, stripped on
	// purpose by liveEmuQueryStripPattern).
	for _, m := range slices.Sorted(maps.Keys(h.modes)) {
		switch m {
		case 47, 1047, 1049, 2048, 2026:
			// 2026 (synchronized output): the replay is a complete screen, so a
			// join mid-update must not leave the viewer holding its rendering.
			continue
		}
		set := byte('l')
		if h.modes[m] {
			set = 'h'
		}
		fmt.Fprintf(&b, "\x1b[?%d%c", m, set)
	}
	return []byte(b.String())
}

// Replay is what Join captured for a joining viewer: Mode is ReplayFull or
// ReplayDelta, Chunks are MsgOutput frames of at most ReplayChunk payload each
// (none when there is nothing to replay), End is the ring end offset the
// client resumes from.
type Replay struct {
	Mode   string
	Chunks [][]byte
	End    uint64
}

// Join subscribes sub and captures its catch-up replay as one atomic step, and
// returns the replay the caller writes (after the grid frame) bracketed by
// MakeMeta{Replay: Mode} and MakeMeta{Offset: End}.
//
// For a full replay the discriminator is whether the raw 256 KiB scrollback
// ring still holds the FULL session history:
//
//   - Ring INTACT (not truncated) — replay the RAW byte history. This is
//     byte-faithful: the guest's xterm ends up identical to the host TUI, so
//     live differential repaints apply cleanly. This is Phase 157's fix for
//     #109 ("Video issues on guest" — layout garble across different host/guest
//     screen sizes) and is the common case.
//
//   - Ring WRAPPED (truncated — oldest bytes dropped) — raw replay is now
//     INCOMPLETE: any content a full-screen app positioned before the cutoff and
//     never rewrote (top/htop's static columns, vim's screen, a long Claude
//     scrollback) is missing, so a raw tail replays scrambled/partial (the M-51
//     top garble). The live per-hub emulator — continuously fed since the first
//     PTY byte — holds the COMPLETE current screen, so its rendered screen
//     (which re-establishes alt-screen mode when applicable) is the correct
//     source. This subsumes BUG-04 / #119 Problem 2 (the alt-screen ESC[?1049h
//     wrap) as one instance of the general "history lost" condition.
//
// Both raw paths drop reply-eliciting terminal queries (StripTerminalQueries;
// liveEmuQueryStripPattern's doc comment lists the set and its limits). A
// joining terminal answers every historic query in the replay, and those
// answers would be typed into the shared shell for every viewer (UAT gap 1,
// H-replies). Queries change no terminal state, so the replay still renders the
// same. Colours and mode sets survive the strip, except the in-band resize mode
// ?2048, which is stripped because setting it makes the terminal report its
// size. The ring stores 0x01-framed chunks, so a query split across two PTY
// reads is not matched: accepted, it needs a read boundary inside a few-byte
// sequence.
//
// SHAR-07 / Phase 5 (overnight survival): offsets are frame bytes of the ring
// (Scrollback.End). A client that sends hasResume with an offset the ring still
// covers gets ReplayDelta: only the ring bytes after it, query-stripped like the
// full path, so a rejoining terminal never answers hours-old queries into the
// PTY. Any other resume (absent, older than the ring start, past the end) is
// ReplayFull, never an error or a silent partial. Replays are chunked to
// ReplayChunk so no reader needs a raised limit for the catch-up. Subscribe and
// capture both happen under emuMu, the lock Run records and broadcasts under, so each frame
// reaches the viewer exactly once: in the replay, in TakePending or in sub.Msgs.
// Frames broadcast while the caller writes the replay are held for TakePending
// rather than queued in Msgs, so a long replay cannot fill the 256-frame queue
// and get the viewer kicked as slow (WR-01).
// The ring bytes ride along with their 0x01 frame headers as SOH bytes; the
// client takes its position from the offset meta, never by counting them.
//
// Geometry (05-08): the ring stores raw bytes but not resizes. Bytes written
// before a genuine host resize, replayed into a viewer at the new width, put
// relative redraws (Claude Code's Ink frames) in the wrong rows: the reattach
// jumble. So a delta is sent only from an offset past the last genuine resize
// (geomEnd), and the raw full replay only while no genuine resize was
// recorded; otherwise the rendered history + screen goes out. The hub's first
// resize from its unknown grid is not genuine (the PTY was spawned at that
// grid), so the #109 raw replay keeps the common never-resized case.
func (h *Hub) Join(sub *Subscriber, resume uint64, hasResume bool) Replay {
	r, data, raw := h.joinLocked(sub, resume, hasResume)
	// Strip and chunk outside emuMu so the drain loop is not held up.
	if raw {
		data = StripTerminalQueries(data)
	}
	for len(data) > 0 {
		n := min(len(data), ReplayChunk)
		r.Chunks = append(r.Chunks, MakeOutputFrame(data[:n]))
		data = data[n:]
	}
	return r
}

// Frames returns the replay as the caller writes it after the grid frame:
// MsgMeta{replay}, the chunks, MsgMeta{offset}.
func (r Replay) Frames() [][]byte {
	mode, end := r.Mode, r.End
	frames := make([][]byte, 0, len(r.Chunks)+2)
	frames = append(frames, MakeMeta(MetaPayload{Replay: &mode}))
	frames = append(frames, r.Chunks...)
	return append(frames, MakeMeta(MetaPayload{Offset: &end}))
}

// Bytes returns the total payload bytes of the replay chunks (diagnostics).
func (r Replay) Bytes() int {
	n := 0
	for _, c := range r.Chunks {
		n += len(c) - 1
	}
	return n
}

// joinLocked subscribes and captures the replay bytes under emuMu. raw reports
// ring bytes that still need query stripping.
func (h *Hub) joinLocked(sub *Subscriber, resume uint64, hasResume bool) (r Replay, data []byte, raw bool) {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	sub.replaying = true // before Subscribe publishes sub; ended by TakePending
	h.Subscribe(sub)     // lock order emuMu then hub.mu, as Cols()/Rows()
	h.ensureLiveEmulatorLocked()

	// A resume exactly at geomEnd may predate the resize (no output in between),
	// so only an offset past it is known to be laid out for the current grid.
	if hasResume && (h.geomEnd == 0 || resume > h.geomEnd) {
		if tail, end, ok := h.scrollback.SnapshotFrom(resume); ok {
			return Replay{Mode: ReplayDelta, End: end}, tail, true
		}
	}
	r = Replay{Mode: ReplayFull, End: h.scrollback.End()}
	if h.scrollback.Truncated() || h.geomEnd > 0 {
		// Ring wrapped (raw is incomplete) or it holds bytes laid out for an
		// older grid: prefer the emulator's complete screen, after the history
		// that scrolled off it (SHAR-07, 05-08).
		if screen := h.renderScreenLocked(); len(screen) > 0 {
			if hist := h.history.Render(); hist != "" {
				screen = append([]byte(hist+"\r\n"), screen...)
			}
			return r, screen, false
		}
	}
	// Ring intact at one grid (complete and byte-faithful), or the emulator has nothing to
	// render: fall back to the raw tail rather than sending nothing.
	return r, h.scrollback.Snapshot(), true
}

// Cols returns the current PTY column width as set by the host-authority resize
// arbiter. Returns 220 when no resize has been applied (fallback for
// scrollback VT extraction — wide enough to avoid spurious line wrapping).
// Phase 139 / CARD-05.
func (h *Hub) Cols() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ptyCols <= 0 {
		return 220
	}
	return h.ptyCols
}

// Rows returns the current PTY row height as set by the host-authority resize
// arbiter. Returns 50 when no resize has been applied (fallback mirrors
// engine.go emuRows default; needed by VIEW-03 join-push in Plan 02).
func (h *Hub) Rows() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ptyRows <= 0 {
		return 50
	}
	return h.ptyRows
}

// Done returns a channel that is closed when the hub shuts down.
func (h *Hub) Done() <-chan struct{} {
	return h.done
}
