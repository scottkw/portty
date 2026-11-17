package relay

import (
	"io"
	"log"
	"regexp"
	"strings"
	"sync"

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
	// Run()'s drain loop via recordFrame, so its state survives any later wrap
	// of the raw scrollback ring — RenderSnapshot() never re-derives from the
	// (possibly truncated) ring (RESEARCH "Pitfall 3").
	//
	// emuMu is a SEPARATE lock from mu (T-175-04-02). Note the anti-hang
	// guarantee comes from liveEmuQueryStripPattern (which lets emu.Write return
	// synchronously — see recordFrame), NOT from goroutine isolation: recordFrame
	// runs INLINE in the drain goroutine, so an unstripped query sequence would
	// stall the drain loop. emuMu only keeps the emulator's own state consistent
	// (append+feed vs bootstrap vs resize) without contending on the fan-out mu.
	emuMu   sync.Mutex
	liveEmu *xvt.Emulator
}

// altScreenEnterSeq is the DEC private-mode sequence a full-screen TUI emits
// exactly once on entering the alternate screen buffer (ESC[?1049h).
// RenderSnapshot prefixes it onto the reconnect preamble whenever the live
// emulator reports IsAltScreen(), so a reconnecting/late-joining client's
// xterm.js re-enters the correct buffer before painting replayed content —
// reconstructing the mode marker the raw 256 KiB scrollback ring may have
// wrapped past (BUG-04, #119).
const altScreenEnterSeq = "\x1b[?1049h"

// liveEmulatorScrollbackLines bounds the live per-hub VT emulator's own
// internal scrollback buffer (T-175-04-01). The emulator exists solely to
// produce a current-screen RenderSnapshot() — it does not need real
// scrollback depth — so this is deliberately small, mirroring the
// emuRows=50 row bound the daemon's styled-tail renderer
// (internal/daemon/engine.go, GET /sessions/{id}/styled-tail) uses for its
// own headless emulator.
const liveEmulatorScrollbackLines = 50

// liveEmuQueryStripPattern mirrors internal/daemon/engine.go's
// queryStripPattern byte-for-byte. It strips terminal-query and in-band-
// resize escape sequences that would otherwise elicit a blocking response
// write into charmbracelet/x/vt's unbuffered response pipe (Emulator.pw) —
// nothing ever drains Emulator.Read() on this feed path, so an unstripped
// query would deadlock both EnsureLiveEmulator's bootstrap and Run()'s drain
// loop (#96/#100). SGR/color sequences are deliberately preserved so styled
// output survives. Join also applies it to the raw catch-up replay (UAT gap 1).
//
// relay MUST NOT import internal/daemon (daemon already imports relay — an
// import back the other way would be a cycle), so this pattern is
// duplicated here rather than shared.
var liveEmuQueryStripPattern = regexp.MustCompile(
	`\x1b\[[0-9;]*c` + // DA1 (and any params before c)
		`|\x1b\[>[0-9;]*c` + // DA2
		`|\x1b\[[0-9;]*n` + // DSR (5n/6n and any CSI n)
		`|\x1b\[\?[0-9;]*n` + // DECXCPR (ESC[?...n)
		`|\x1b\[[0-9;]*\$p` + // DECRQM ANSI
		`|\x1b\[\?[0-9;]*\$p` + // DECRQM DEC
		`|\x1b\[\?2048[hl]` + // in-band resize set/reset (set triggers a pw write)
		`|\x1b\[>[0-9;]*q` + // XTVERSION
		`|\x1b\[\?u` + // kitty keyboard flags query
		`|\x1b\]1[012];\?(?:\x07|\x1b\\)`, // OSC 10/11/12 color query, BEL- or ST-terminated
)

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
		h.resizeLiveEmulator(pc, pr)
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
		select {
		case sub.Msgs <- frame:
		default:
			h.kickSlow(sub)
		}
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
			// emulator as ONE atomic step under emuMu — see recordFrame. This
			// prevents a boundary frame being counted twice (once by
			// EnsureLiveEmulator's bootstrap ScrollbackSnapshot, once by the live
			// feed), the TestHub_TwoClientsFanOut "hello worldhello world" race.
			h.recordFrame(frame, buf[:n])
			h.broadcast(frame)
		}
		if err != nil {
			return
		}
	}
}

// recordFrame appends the framed bytes to the scrollback ring AND feeds the raw
// (pre-framing) PTY bytes to the live emulator (if one has been built) as ONE
// atomic step under emuMu. Called from Run()'s drain loop immediately after the
// PTY Read returns, OUTSIDE hub.mu (emuMu is a separate lock, T-175-04-02).
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
// The anti-hang mechanism for the emulator write is liveEmuQueryStripPattern
// (not goroutine isolation): stripping the query/response sequences lets
// emu.Write return synchronously with no drain goroutine — the proven engine.go
// #96/#100 mitigation. A query sequence NOT covered by that pattern would block
// emu.Write and therefore this drain loop.
func (h *Hub) recordFrame(frame, raw []byte) {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
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
		emu := xvt.NewEmulator(h.Cols(), h.Rows())
		emu.SetScrollbackSize(liveEmulatorScrollbackLines)
		h.liveEmu = emu
	}
	clean := liveEmuQueryStripPattern.ReplaceAll(raw, nil)
	h.liveEmu.Write(clean) //nolint:errcheck // emulator Write never returns a meaningful error
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
// build in recordFrame feeds a single emulator continuously, so completeness
// after a raw-ring wrap (the original BUG-04 fix) is preserved. recordFrame's
// nil-guard means if output arrives before this rebuild's first frame, the feed
// simply lands in the freshly-built emulator.
//
// Takes ONLY emuMu (never hub.mu): callers acquire emuMu→hub.mu via Cols()/Rows()
// elsewhere, so this MUST be called after hub.mu is released, or the reverse lock
// order would deadlock. No-op until the emulator exists (recordFrame will build
// at the current geometry on the first frame).
func (h *Hub) resizeLiveEmulator(cols, rows int) {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	if h.liveEmu == nil {
		return
	}
	emu := xvt.NewEmulator(cols, rows)
	emu.SetScrollbackSize(liveEmulatorScrollbackLines)
	h.liveEmu = emu
}

// broadcast sends frame to all current subscribers using a non-blocking send.
// Subscribers with a full channel have their CloseSlow invoked in a new goroutine.
func (h *Hub) broadcast(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		select {
		case sub.Msgs <- frame:
		default:
			h.kickSlow(sub)
		}
	}
}

// BroadcastMeta sends a metadata frame to all subscribers using a non-blocking
// send. Slow subscribers have CloseSlow called — MsgMeta must never block the
// PTY drain loop. Used by relay.Server and webserver to push viewer count updates.
func (h *Hub) BroadcastMeta(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		select {
		case sub.Msgs <- frame:
		default:
			h.kickSlow(sub)
		}
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
// from the first PTY frame in recordFrame (M-51 fix). Any session that has
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

	// No output has flowed yet (recordFrame never ran) — build an empty emulator
	// at the current fallback geometry so RenderSnapshot is well-defined. The
	// drain loop's recordFrame will feed this same instance once bytes arrive.
	emu := xvt.NewEmulator(h.Cols(), h.Rows())
	// T-175-04-01: bound the emulator's own scrollback — it is only ever used
	// to render the CURRENT screen, never scrolled through.
	emu.SetScrollbackSize(liveEmulatorScrollbackLines)
	h.liveEmu = emu
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

	// Render() pads every row up to the emulator's full height (default 50 —
	// Hub.Rows()'s fallback) with trailing blank lines. Trim them so a normal
	// (non-alt-screen) session's preamble is byte-identical to what raw PTY
	// output would have produced, matching the pre-existing scrollback-replay
	// contract for ordinary (non-wrapped, non-full-screen) sessions. Mirrors
	// the daemon styled-tail renderer's own "trim trailing blank rows"
	// convention (internal/daemon/engine.go).
	content := strings.TrimRight(h.liveEmu.Render(), "\n")
	altScreen := h.liveEmu.IsAltScreen()
	if content == "" && !altScreen {
		// Nothing to show yet, and no mode marker to reconstruct — sending an
		// empty frame would be pure overhead and would break the "no data
		// received before real output arrives" contract callers rely on.
		return nil
	}
	if altScreen {
		content = altScreenEnterSeq + content
	}
	return []byte(content)
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
// Both raw paths drop reply-eliciting terminal queries (liveEmuQueryStripPattern).
// A joining terminal answers every historic query in the replay (DSR, DA2,
// DECRQM, XTVERSION, kitty), and those answers would be typed into the shared
// shell for every viewer (UAT gap 1, H-replies). Queries change no terminal
// state, so the replay still renders the same; mode sets and colours stay. The
// ring stores 0x01-framed chunks, so a query split across two PTY reads is not
// matched: accepted, it needs a read boundary inside a few-byte sequence.
//
// SHAR-07 / Phase 5 (overnight survival): offsets are frame bytes of the ring
// (Scrollback.End). A client that sends hasResume with an offset the ring still
// covers gets ReplayDelta: only the ring bytes after it, query-stripped like the
// full path, so a rejoining terminal never answers hours-old queries into the
// PTY. Any other resume (absent, older than the ring start, past the end) is
// ReplayFull, never an error or a silent partial. Replays are chunked to
// ReplayChunk so no reader needs a raised limit for the catch-up. Subscribe and
// capture both happen under emuMu, the lock Run records under, so each frame
// reaches the viewer exactly once: in the replay or in sub.Msgs.
// The ring bytes ride along with their 0x01 frame headers as SOH bytes; the
// client takes its position from the offset meta, never by counting them.
func (h *Hub) Join(sub *Subscriber, resume uint64, hasResume bool) Replay {
	r, data, raw := h.joinLocked(sub, resume, hasResume)
	// Strip and chunk outside emuMu so the drain loop is not held up.
	if raw {
		data = liveEmuQueryStripPattern.ReplaceAll(data, nil)
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
	h.Subscribe(sub) // lock order emuMu then hub.mu, as Cols()/Rows()
	h.ensureLiveEmulatorLocked()

	if hasResume {
		if tail, end, ok := h.scrollback.SnapshotFrom(resume); ok {
			return Replay{Mode: ReplayDelta, End: end}, tail, true
		}
	}
	r = Replay{Mode: ReplayFull, End: h.scrollback.End()}
	if h.scrollback.Truncated() {
		// Ring wrapped — raw is incomplete; prefer the emulator's complete screen.
		if screen := h.renderScreenLocked(); len(screen) > 0 {
			return r, screen, false
		}
	}
	// Ring intact (complete and byte-faithful), or the emulator has nothing to
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
