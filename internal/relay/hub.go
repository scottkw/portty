package relay

import (
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

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
	// at subscribe time; the grid bootstrap (ResizeClient) and RemoteViewerCount read it.
	Origin string

	// Kick closes the connection with the given reason (RevokeWebViewers).
	// Called in its own goroutine; nil falls back to CloseSlow.
	Kick func(reason string)

	// Restart closes the connection with 1012 service restart
	// (RestartViewers). Called in its own goroutine; nil falls back to
	// CloseSlow.
	Restart func()

	// ID: per-hub connection number for viewer-diag log lines, assigned by
	// Subscribe before it returns. Diagnostics only.
	ID uint64
	// Remote: the viewer's IP for viewer-diag lines, set by the webserver
	// before Subscribe; empty for local viewers. Diagnostics only.
	Remote string

	// slowLogged: the first slow kick has been logged. Read/written under hub.mu.
	slowLogged bool

	// inputRevoked: RevokeWebViewers dropped this viewer's queued input and
	// every frame its pump queues from now on (CR-01). Under hub.inMu.
	inputRevoked bool

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

	// Phase 9 handoff (UPGR-01): freezing is set from Freeze until Thaw (or
	// a failed freeze) and frozen is closed by Run when it stops for it, so
	// a frozen hub has freezing set and frozen closed. Both under mu.
	// gridMu serialises ResizeClient with the start and end of a freeze, so
	// no resize lands between the last read and the export. startOnce makes
	// Start run one reader.
	freezing  bool
	frozen    chan struct{}
	gridMu    sync.Mutex
	startOnce sync.Once

	// Input FIFO (input.go, INPT-01): every viewer's MsgInput frames in
	// arrival order, written to the PTY by one goroutine with inMu released
	// around the write. inMu is a leaf lock: nothing else is taken under it,
	// and nothing logs under it (IN-02).
	// inq[inHead:] is the queue; inBytes counts queued plus in-flight bytes;
	// inDead: the writer stopped (write error or done) and the queue is gone;
	// inStarted: the writer and its done watcher are running; inHeld: the
	// writer takes no frame until ReleaseInput (WR-02). All under inMu.
	inMu      sync.Mutex
	inCond    *sync.Cond
	inq       []inputReq
	inHead    int
	inBytes   int
	inDead    bool
	inStarted bool
	inHeld    bool

	// ptyCols, ptyRows: current PTY dimensions. They move only to the grid
	// target (wantCols, wantRows), which only a claim or the bootstrap sets
	// (INPT-06, see ResizeClient). owner: the viewer that set the target
	// last, nil once it leaves; the grid stays (D-01). All under mu.
	ptyCols  int
	ptyRows  int
	wantCols int
	wantRows int
	owner    *Subscriber
	// ownerFast mirrors owner and ownerStale is set while the owner's newest
	// report differs from the target, so the owner's own keystrokes skip every
	// lock in NoteTyping (and a fast path that took mu could not then take
	// gridMu). Both are written under mu, by setOwnerLocked and sizeClient.
	ownerFast  atomic.Pointer[Subscriber]
	ownerStale atomic.Bool

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
	// set/reset), replayed by modeRestoresLocked. Hub-owned so it survives
	// emulator rebuilds, which start from default modes. Only touched under emuMu.
	modes map[int]bool
	// geomEnd is the scrollback end offset at the last genuine host geometry
	// change (0: none). Raw ring bytes are laid out for the width they were
	// written at; replayed at another width, Claude Code's relative redraws
	// land in the wrong rows (05-08, the reattach jumble). Join sends no delta
	// or raw replay that reaches back before it (a delta from exactly geomEnd
	// holds only post-resize bytes). Only touched under emuMu.
	geomEnd uint64
	// altExited is set by the vt AltScreen callback when the app leaves the
	// alternate screen; recordFrameLocked rebuilds the emulator after the
	// write (rebuildAfterAltExitLocked). Only touched under emuMu.
	altExited bool
	// mainRegion (DECSTBM, 1-based top and bottom; zero: the whole screen) is
	// the main screen's, which vt does not expose; the alt-exit rebuild
	// re-applies it (WR-02). Only touched under emuMu.
	mainRegion [2]int
	// wraps holds the keys of main-screen rows vt soft-wrapped (DEBT-02,
	// wraps.go). Hub-owned so it survives the alt-exit rebuild, whose seed
	// re-renders the same rows. trackWraps is set only around the live feed
	// write, so rebuild probes and seeds record nothing; lastMove is the
	// previous CursorPosition callback's (old, new). Only touched under emuMu.
	wraps      wrapSet
	trackWraps bool
	lastMove   [2]uv.Position
	// cs is the charset and OSC 8 link state the host terminal is in
	// (charstate.go, IN-08), which every replay body ends with. Hub-owned so
	// it survives rebuilds. Only touched under emuMu.
	cs charState
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
// (CSI 11 t window state, 13 t position, 14 t, 14;2 t, 15 t, 16 t, 18 t,
// 19 t sizes, 20 t the icon label and 21 t the title, the classic echo-back
// vectors); XTGETTCAP (DCS + q); DECRQSS (DCS $ q);
// OSC 10/11/12 colour queries, including the multi-query OSC 10;?;? form;
// OSC 4 palette queries; OSC 52 clipboard reads.
// Window-op sets (CSI 8;r;c t, 22;0 t), OSC 4 sets, OSC 52 writes, sixel DCS
// images, SGR and every other mode set pass through untouched.
//
// It is shared through StripTerminalQueries: internal/daemon imports relay
// (never the reverse), so the hub and the engine strip the same set.
//
// Known limits: the live feed strips each PTY read alone, so a query split
// across two reads reaches the emulator (harmless, see above; Join's replays
// strip across reads); an OSC 4 that mixes sets and queries is kept whole; a
// query not listed above is not stripped.
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
		`|\x1b\[(?:1[134-689]|2[01])(?:;[0-9]*)?t` + // XTWINOPS 11/13/14/15/16/18/19/20/21 reports (not the 8/22/23 sets)
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

// stripRingQueries is StripTerminalQueries for ring bytes, where a query split
// across two PTY reads has a 0x01 frame header inside it: matching runs with
// the headers skipped, and a match drops the headers inside it too (IN-04).
// Every other byte, headers included, stays, so a replay with no query is
// still the raw ring (#109). Clients take their position from the offset
// meta, never by counting replay bytes.
func stripRingQueries(data []byte) []byte {
	pos := make([]int, 0, len(data)) // data index of each byte in view
	view := make([]byte, 0, len(data))
	for i, c := range data {
		if c != MsgOutput {
			pos = append(pos, i)
			view = append(view, c)
		}
	}
	locs := liveEmuQueryStripPattern.FindAllIndex(view, -1)
	if locs == nil {
		return data
	}
	out, prev := make([]byte, 0, len(data)), 0
	for _, l := range locs {
		out = append(out, data[prev:pos[l[0]]]...)
		prev = pos[l[1]-1] + 1
	}
	return append(out, data[prev:]...)
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
	h := &Hub{
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
	h.inCond = sync.NewCond(&h.inMu)
	return h
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
	if h.owner == sub {
		h.setOwnerLocked(nil) // the grid and its target stay until someone claims (D-01)
	}
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
	// CR-01: revoked access must stop the shell receiving this viewer's input,
	// so its queued frames go before the kick (only one already in the PTY
	// write can still land). A plain disconnect keeps its backlog (INPT-01).
	frames, bytes := h.revokeInput(targets)
	log.Printf("viewer-diag: revoke session=%s reason=%q web=%d input-dropped frames=%d bytes=%d", DiagID(h.sessionID), reason, len(targets), frames, bytes)

	for _, sub := range targets {
		if sub.Kick != nil {
			go sub.Kick(reason)
		} else {
			go sub.CloseSlow()
		}
	}
}

// RestartViewers closes every subscriber with 1012 service restart (via
// Restart, else CloseSlow) when a handoff commits, and returns a channel
// closed once every close call has returned, so the old daemon can let the
// close handshakes finish before it exits. Done stays open: the session goes
// on in the new daemon and viewers reconnect with resume. Like
// RevokeWebViewers it collects under h.mu and closes after releasing it
// (T-157-04).
func (h *Hub) RestartViewers() <-chan struct{} {
	h.mu.Lock()
	targets := slices.Collect(maps.Keys(h.subscribers))
	h.mu.Unlock()
	log.Printf("viewer-diag: restart session=%s viewers=%d", DiagID(h.sessionID), len(targets))

	var wg sync.WaitGroup
	for _, sub := range targets {
		closeFn := sub.Restart
		if closeFn == nil {
			closeFn = sub.CloseSlow
		}
		wg.Go(closeFn)
	}
	done := make(chan struct{})
	if len(targets) == 0 {
		close(done)
		return done
	}
	go func() { wg.Wait(); close(done) }()
	return done
}

// Grid bounds for reported and claimed sizes (INPT-06, T-10-13). With web
// claims allowed a viewer could otherwise make resizeLiveEmulator allocate
// a 65535x65535 emulator.
const (
	MinGridCols = 20
	MaxGridCols = 500
	MinGridRows = 5
	MaxGridRows = 200
)

// ResizeClient stores a viewer's reported size (a plain MsgResize2 frame) and
// nothing else: a report never moves a known grid, whether or not it comes
// from the owner, so an owner's sidebar toggle, a banner or a reconnect's
// refit cannot reflow the shell (INPT-06, "viewer takes over size"). The one
// exception is the bootstrap: a local viewer reporting while the grid was
// never known and nobody owns it takes it, so a fresh session adopts its
// first tab's fit. Joins, replays, reconnects and output never claim.
//
// shortcut: a relaunched GUI (or a tab rejoining after a handoff) shows the
// current grid, scaled, until its user types, zooms or resizes; add "a lone
// local viewer on an ownerless hub adopts its fit" if the VM check finds it
// confusing (10-RESEARCH Pitfall 11).
//
// Chesterton's Fence for what this replaces: the VIEW-02 origin gate kept a
// web guest from driving or DoS-ing the PTY grid (T-157-01); the read-only
// gate, the clamp and "reports never move a known grid" do that job now, and
// a write-cap guest already has the shell. D-02 min-among-local let every
// host viewer see all output; the user chose instead that the viewer in use
// sets the size and the others scale (10-CONTEXT INPT-06).
func (h *Hub) ResizeClient(sub *Subscriber, cols, rows int) error {
	return h.sizeClient(sub, cols, rows, false)
}

// ClaimGrid is a user-initiated size change (MsgResize2 with ResizeClaim):
// it stores the size, makes sub the owner and moves the grid to it. Each grid
// change is broadcast to every viewer as a MsgResize (0x02) before resizeFn
// (VIEW-01).
func (h *Hub) ClaimGrid(sub *Subscriber, cols, rows int) error {
	return h.sizeClient(sub, cols, rows, true)
}

// NoteTyping is a typing claim (INPT-06): the pumps call it for MsgInput
// whose filtered bytes pass IsUserInput. sub becomes the owner and the grid
// moves to its last reported size; for the owner that applies a newer report
// (a sidebar toggle or settle refit, which only reported) on the next
// keystroke. A read-only viewer, a viewer that never reported a size (an
// older client: it keeps today's behaviour of never sizing the PTY) and one
// that already left change nothing. When nothing changed, the owner's
// keystroke costs two atomic loads and no lock.
func (h *Hub) NoteTyping(sub *Subscriber) error {
	if h.ownerFast.Load() == sub && !h.ownerStale.Load() {
		return nil
	}
	if sub.ReadOnly {
		return nil
	}
	h.gridMu.Lock()
	defer h.gridMu.Unlock()
	h.mu.Lock()
	if _, ok := h.subscribers[sub]; !ok || sub.Cols <= 0 || sub.Rows <= 0 {
		h.mu.Unlock()
		return nil
	}
	h.setOwnerLocked(sub)
	if h.freezing {
		h.mu.Unlock() // Thaw applies it, as for a claim
		return nil
	}
	return h.applyGridLocked("input")
}

// setOwnerLocked makes sub the owner and its stored size the grid target
// (nil: nobody owns the grid; the target stays). Caller holds mu.
func (h *Hub) setOwnerLocked(sub *Subscriber) {
	h.owner = sub
	h.ownerFast.Store(sub)
	if sub != nil {
		h.wantCols, h.wantRows = sub.Cols, sub.Rows
		h.ownerStale.Store(false)
	}
}

// sizeClient is ResizeClient and ClaimGrid. Read-only viewers can neither
// report nor claim (T-10-12; the pumps skip them too). Sizes are clamped and
// zero or negative ones ignored. On a frozen hub the owner and target are
// recorded and Thaw applies them.
func (h *Hub) sizeClient(sub *Subscriber, cols, rows int, claim bool) error {
	if sub.ReadOnly || cols <= 0 || rows <= 0 {
		return nil
	}
	cols, rows = min(max(cols, MinGridCols), MaxGridCols), min(max(rows, MinGridRows), MaxGridRows)
	h.gridMu.Lock()
	defer h.gridMu.Unlock()
	h.mu.Lock()
	sub.Cols, sub.Rows = cols, rows
	why := "claim"
	if !claim {
		if sub.Origin != "local" || h.owner != nil || (h.ptyCols > 0 && h.ptyRows > 0) {
			if sub == h.owner {
				h.ownerStale.Store(cols != h.wantCols || rows != h.wantRows) // NoteTyping applies it
			}
			h.mu.Unlock()
			return nil // a report: stored only
		}
		why = "bootstrap"
	}
	h.setOwnerLocked(sub)
	if h.freezing {
		// Frozen: the PTY grid and the emulator stay as exported. Thaw
		// re-runs applyGridLocked, so a claim during a rolled-back freeze
		// still lands.
		// shortcut: a committed handoff never thaws this hub and HubState
		// carries no target or owner, so a claim (or NoteTyping's) made while
		// frozen is lost; the viewer shows the old grid scaled until it claims
		// again. Exporting the target would not cover claims after Export;
		// have TerminalPanel re-send its last unanswered claim on reconnect if
		// this shows up (IN-01).
		h.mu.Unlock()
		return nil
	}
	return h.applyGridLocked(why)
}

// applyGridLocked moves the PTY to the grid target when one is set and
// differs. It never reads a viewer's reported size. why (bootstrap, claim,
// input, thaw) goes in the diag line. Caller holds gridMu and mu; mu is released
// before any IO (lock order gridMu then mu).
func (h *Hub) applyGridLocked(why string) error {
	oldCols, oldRows := h.ptyCols, h.ptyRows
	pc, pr := h.wantCols, h.wantRows
	if pc <= 0 || pr <= 0 || (pc == oldCols && pr == oldRows) {
		h.mu.Unlock()
		return nil // D-01: no target yet, or already there
	}
	h.ptyCols, h.ptyRows = pc, pr
	var by, client string
	var conn uint64
	if o := h.owner; o != nil {
		by, client, conn = o.Origin, o.Name, o.ID
	}
	h.mu.Unlock() // unlock-before-IO: broadcastResize and resizeFn must not run under mu

	// Diagnostic for UAT gap 1 (04-07 H-grid): who changed the host grid, to what, and why.
	log.Printf("viewer-diag: grid session=%s %dx%d -> %dx%d by=%s client=%q conn=%d why=%s",
		DiagID(h.sessionID), oldCols, oldRows, pc, pr, by, client, conn, why)
	// VIEW-01: broadcast the new authoritative grid to all subscribers.
	h.broadcastResize(uint16(pc), uint16(pr))
	// CR-01: keep the live emulator's geometry in lockstep with the PTY so a
	// later reconnect preamble (RenderSnapshot) is not mis-dimensioned. Safe
	// here — hub.mu is already released (resizeLiveEmulator takes only emuMu).
	// Genuine only when the hub already had a real grid: the first resize from
	// the unknown grid matches the size the PTY was spawned at.
	h.resizeLiveEmulator(pc, pr, oldCols > 0 && oldRows > 0)
	if h.resizeFn != nil {
		return h.resizeFn(pc, pr)
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
// Run returns when the reader returns any error (typically io.EOF on PTY close)
// and then calls Shutdown() to signal completion. The one exception is a
// freeze (Freeze): Run returns at the read deadline without Shutdown, so no
// viewer is told the session ended and the engine sees no exit.
func (h *Hub) Run() {
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
		if err == nil {
			continue
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			h.mu.Lock()
			stop := h.freezing
			if stop {
				close(h.frozen) // every byte read so far is in the ring and the emulator
			}
			h.mu.Unlock()
			if stop {
				return
			}
			continue // a freeze that gave up: its deadline is already cleared
		}
		h.Shutdown()
		return
	}
}

// ErrHubEnded: the session ended (Done is closed), so there is nothing to freeze.
var ErrHubEnded = errors.New("relay: hub ended")

// freezeTimeout bounds Freeze's wait for Run to stop at the deadline.
const freezeTimeout = 2 * time.Second

// deadliner is a reader that can stop a blocked Read: an os.File on a
// pollable fd, such as the non-blocking PTY master (Pattern 2).
type deadliner interface{ SetReadDeadline(time.Time) error }

// Freeze stops Run at a read boundary without closing Done (Pattern 5), so
// the hub can be exported. On a nil return every byte read so far is in the
// ring and the emulator (recordFrameLocked runs before the next Read), and
// nothing more is read until Thaw. It fails at once when the reader takes no
// read deadline (a blocking master returns os.ErrNoDeadline, Pitfall 6),
// returns ErrHubEnded when the shell exits before Run stops, and gives up
// after freezeTimeout, leaving the hub running.
func (h *Hub) Freeze() error {
	d, ok := h.reader.(deadliner)
	if !ok {
		return errors.New("relay: freeze: the reader takes no read deadline")
	}
	h.gridMu.Lock()
	h.mu.Lock()
	closed, already := h.closed, h.freezing
	frozen := make(chan struct{})
	if !closed && !already {
		h.freezing, h.frozen = true, frozen
	}
	h.mu.Unlock()
	h.gridMu.Unlock()
	switch {
	case closed:
		return ErrHubEnded
	case already:
		return errors.New("relay: freeze: already frozen")
	}
	if err := d.SetReadDeadline(time.Now()); err != nil {
		h.unfreeze(d)
		return fmt.Errorf("relay: freeze: %w", err)
	}
	select {
	case <-frozen:
		return nil
	case <-h.done:
		return ErrHubEnded
	case <-time.After(freezeTimeout):
		h.unfreeze(d)
		return errors.New("relay: freeze: the read loop did not stop in time")
	}
}

// Thaw resumes a frozen hub: the read loop picks up at the byte after the
// last one recorded, and resizes made while frozen are applied.
func (h *Hub) Thaw() {
	if d, ok := h.reader.(deadliner); ok {
		h.unfreeze(d)
	}
}

// unfreeze clears the deadline and the freeze, re-runs the arbiter for the
// resizes the freeze held back, and restarts Run if it stopped for the freeze.
func (h *Hub) unfreeze(d deadliner) {
	_ = d.SetReadDeadline(time.Time{}) // first: Run retries a stale deadline error once freezing is off
	h.gridMu.Lock()
	defer h.gridMu.Unlock()
	h.mu.Lock()
	stopped := h.frozenLocked()
	h.freezing = false
	_ = h.applyGridLocked("thaw") // releases mu; a failed PTY resize stays the next resize's to retry
	if stopped {
		go h.Run()
	}
}

// frozenLocked reports whether Run has stopped for a freeze that is still
// on. Caller holds mu.
func (h *Hub) frozenLocked() bool {
	if !h.freezing {
		return false
	}
	select {
	case <-h.frozen:
		return true
	default:
		return false
	}
}

// Start runs the read loop of a staged hub (NewHubFromState), once.
//
// No post-handoff grace period is needed (Phase 9 had one, 09-07-FIX2.md):
// under min-among-local, viewers rejoining one by one grew and then shrank
// the imported grid, and each change bumped geomEnd so later resumes were
// full replays. An imported hub's grid is known, so the bootstrap cannot
// fire, and joins and reports never move a known grid; only a claim does
// (INPT-06). TestHub_HandoffRejoinKeepsGrid pins V-02.
func (h *Hub) Start() {
	h.startOnce.Do(func() { go h.Run() })
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
	h.cs.scan(clean) // GL (SO/SI) in byte order; the handlers track the rest
	h.trackWraps = true
	h.liveEmu.Write(clean) //nolint:errcheck // emulator Write never returns a meaningful error
	h.trackWraps = false
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
	var s vtScan
	last := 0
	for i, c := range b {
		s.step(c)
		if s.state == scanGround && s.need == 0 {
			last = i + 1
		}
	}
	if last == len(b) {
		return nil
	}
	return b[last:]
}

// vtScan states.
const (
	scanGround = iota
	scanEsc
	scanCsi
	scanOsc
	scanStr // DCS, SOS, PM, APC: ST-terminated only
	scanStrEsc
)

// vtScan follows vt's parser bytewise, with C1 controls, far enough to know
// where escape sequences, strings and UTF-8 characters end (incompleteTail,
// charState.scan). need is the UTF-8 continuation bytes still expected.
type vtScan struct{ state, need int }

func (s *vtScan) step(c byte) {
	switch {
	// CAN, SUB abort an escape sequence. vt counts any byte, these
	// included, as part of an open UTF-8 character (WR-09).
	// shortcut: the scan assumes well-formed UTF-8; track vt's byte count
	// if malformed UTF-8 around an alt-screen exit ever shows up.
	case c == 0x18 || c == 0x1a:
		s.state, s.need = scanGround, 0
	case s.need > 0 && c&0xc0 == 0x80:
		s.need--
	case s.state == scanGround:
		s.need = 0
		switch {
		case c == 0x1b:
			s.state = scanEsc
		case c == 0x9b:
			s.state = scanCsi
		case c == 0x9d:
			s.state = scanOsc
		case c == 0x90 || c == 0x98 || c == 0x9e || c == 0x9f:
			s.state = scanStr
		case c&0xe0 == 0xc0:
			s.need = 1
		case c&0xf0 == 0xe0:
			s.need = 2
		case c&0xf8 == 0xf0:
			s.need = 3
		}
	case s.state == scanEsc:
		switch {
		case c == '[':
			s.state = scanCsi
		case c == ']':
			s.state = scanOsc
		case c == 'P' || c == 'X' || c == '^' || c == '_':
			s.state = scanStr
		case c >= 0x30 && c <= 0x7e:
			s.state = scanGround
		} // intermediates, C0 controls and ESC itself stay in esc
	case s.state == scanCsi:
		if c == 0x1b {
			s.state = scanEsc
		} else if c >= 0x40 && c <= 0x7e {
			s.state = scanGround
		}
	case s.state == scanOsc || s.state == scanStr:
		if c == 0x9c || (s.state == scanOsc && c == 0x07) {
			s.state = scanGround
		} else if c == 0x1b {
			s.state = scanStrEsc
		}
	case s.state == scanStrEsc:
		if c == '\\' {
			s.state = scanGround
		} else {
			s.state = scanEsc // the ESC ended the string and starts a sequence
			s.step(c)
		}
	}
}

// rebuildAfterAltExitLocked replaces the live emulator once the app has left
// the alternate screen (HARD-01). Normally the alt screen keeps no scrollback
// (disableAltScreenScrollback, DEBT-01), so there is none to free; the rebuild
// still runs on every exit, so it frees vt's 10,000-line alt scrollback if a
// vt layout change ever makes that call fail. Caller holds emuMu.
//
// Unlike resizeLiveEmulator it re-seeds the new emulator with the old main
// screen rather than pushing the rows into history: no SIGWINCH redraw follows
// an alt-screen exit, so the main screen must stay where the host shows it.
// h.history, h.modes and geomEnd are Hub-owned and stay (not a geometry change).
//
// WR-02: the rendered seed carries text (its links included), cursor and DEC
// modes only, so the rebuild also re-applies the main screen's scroll region
// (tracked by newLiveEmulator's DECSTBM handler), the pending wrap, the saved
// cursor with its pen and OSC 8 link, the current pen and link and the DEC
// line drawing G0 charset. vt exposes none of the last four, so they are read
// by printing probe cells into the old emulator, which is discarded anyway: a
// cell carries the pen and link it was printed with, and 'q' prints as '─'
// through a line-drawing G0. The probed G0 and link resync the tracker (IN-08),
// which then re-emits them with the tracked G1 and GL: vt ignores SO and SI
// (0x0e, 0x0f; TestVT_IgnoresShiftOut), so G1 never prints and cannot be
// probed. Then tail, the sequence or UTF-8 character the read cut off, is
// replayed so the next read completes it.
// shortcut: tab stops, G2/G3 and single shifts, the UK national set, the saved
// cursor's visibility, and vt's zero-width cells (an orphan combining mark,
// which the re-ingested seed merges into the previous cell or drops, IN-11)
// are lost; track them if a shell needs them.
//
// The rebuild is seedLocked then applySeedLocked, the same pair a handoff
// uses to move the emulator to another daemon (handoff_state.go), so the
// alt-exit tests cover the handoff seed too.
func (h *Hub) rebuildAfterAltExitLocked(tail []byte) {
	h.applySeedLocked(h.seedLocked(tail), tail, h.liveEmu.Width(), h.liveEmu.Height())
}

// seedLocked returns the bytes that rebuild the live emulator's main screen
// in a fresh one (rebuildAfterAltExitLocked has the full list), given tail,
// the sequence or UTF-8 character the last read cut off. Its probes print
// into the live emulator and resync h.cs, so the caller replaces the
// emulator afterwards (applySeedLocked). Caller holds emuMu.
func (h *Hub) seedLocked(tail []byte) []byte {
	old := h.liveEmu
	var seed []byte // restores and body; the charsets and link go last, from the probes
	if body := h.screenBodyLocked(); body != nil {
		seed = append([]byte(h.modeRestoresLocked()), body...)
	}
	p := h.probeCursorLocked(tail)
	pos, saved, cell, savedCell := p.pos, p.saved, p.cell, p.savedCell
	inferred, pending := p.inferred, p.pending
	// Resync the tracker: G0 and the current link as vt has them.
	switch {
	case cell.Content == "─":
		h.cs.g[0] = '0'
	case h.cs.g[0] == '0':
		h.cs.g[0] = 0
	}
	h.cs.setLink(cell.Link.URL, cell.Link.Params) // drops an unsafe one (WR-02)

	var b strings.Builder
	b.Write(seed)
	// A fresh emulator's saved cursor is home in the default pen, so restore
	// only a different one. CUPs clear a pending wrap, so this runs only when
	// needed, or when the seed's pending-wrap guess was wrong.
	r := h.mainRegion
	keepSaved := saved != (uv.Position{}) || !savedCell.Style.IsZero() || !savedCell.Link.IsZero()
	if r != [2]int{} || keepSaved || inferred != pending {
		om := h.modes[6] // DECOM makes the CUPs below region-relative
		if om {
			b.WriteString("\x1b[?6l")
		}
		if r != [2]int{} {
			fmt.Fprintf(&b, "\x1b[%d;%dr", r[0], r[1]) // homes the cursor
		}
		if keepSaved {
			fmt.Fprintf(&b, "%s%s\x1b[%d;%dH\x1b7", seedPen(savedCell), seedLink(savedCell), saved.Y+1, saved.X+1)
			if !savedCell.Link.IsZero() {
				b.WriteString(ansi.ResetHyperlink()) // the current link comes last
			}
		}
		fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
		if pending {
			// vt keeps one pending-wrap flag for both screens (an app's exit
			// or a DECRC can leave it set anywhere): set it at the alt
			// screen's edge and switch back, which leaves the cursor at pos.
			fmt.Fprintf(&b, "\x1b[?1047h\x1b[1;%dHx\x1b[?1047l", old.Width())
		}
		if om {
			b.WriteString("\x1b[?6h") // vt's DECOM set does not move the cursor
		}
	}
	b.WriteString(seedPen(cell))
	b.WriteString(h.cs.suffix())
	if cell.Link.Params == "" && cell.Link.URL != "" {
		b.WriteString(seedLink(cell)) // vt keeps a link with params and no URI; the tracker does not
	}
	return []byte(b.String())
}

// cursorProbe is what probeCursorLocked reads from the live emulator's
// active screen: the cursor, the saved cursor (DECRC), whether the cursor is
// in pending wrap and whether the rendered body re-creates it (inferred),
// and two probe cells printed in the current and the saved pen and link.
type cursorProbe struct {
	pos, saved        uv.Position
	inferred, pending bool
	cell, savedCell   uv.Cell
}

// probeCursorLocked prints probe cells into the live emulator's active
// screen to read what vt does not expose (seedLocked). Call it after
// rendering the screen: the probes overwrite cells. Caller holds emuMu.
func (h *Hub) probeCursorLocked(tail []byte) cursorProbe {
	old := h.liveEmu
	pos := old.CursorPosition()
	inferred := h.pendingWrapLocked(pos) != "" // what the seed re-created
	var cell, savedCell uv.Cell                // copies: CellAt points into the buffer the next probe overwrites
	// CAN first: the probes must start from ground whatever tail the read left
	// in the old parser (a bare DCS introducer swallowed the CUP, WR-05). A CAN
	// aborts an escape sequence, but vt counts it as a byte of a cut-off UTF-8
	// character (WR-09), so send one CAN per missing byte: the character then
	// prints as one replacement cell, then the probe 'x'. Only a pending wrap
	// wraps the first, putting 'x' on column n.
	// shortcut: a pending wrap on column 0 whose wrap scrolls reads as none.
	n, fill := 1, 1
	if len(tail) > 0 && !utf8.FullRune(tail) {
		size := 4
		switch {
		case tail[0]&0xe0 == 0xc0:
			size = 2
		case tail[0]&0xf0 == 0xe0:
			size = 3
		}
		n, fill = 2, size-len(tail)
	}
	old.Write([]byte(strings.Repeat("\x18", fill) + "x")) //nolint:errcheck // emulator Write never returns a meaningful error
	after := old.CursorPosition()
	pending := after.X == n && (pos.X != 0 || after.Y != pos.Y)
	old.Write([]byte("\x1b[1;1H")) //nolint:errcheck // emulator Write never returns a meaningful error
	probe := old.CursorPosition()  // region-relative under origin mode
	old.Write([]byte("q"))         //nolint:errcheck // 'q' is '─' in DEC line drawing
	if c := old.CellAt(probe.X, probe.Y); c != nil {
		cell = *c
	}
	old.Write([]byte("\x1b8")) //nolint:errcheck // DECRC: what a later ESC 8 restores (vt keeps it per screen)
	saved := old.CursorPosition()
	old.Write([]byte("q")) //nolint:errcheck // prints in the saved pen
	if c := old.CellAt(saved.X, saved.Y); c != nil {
		savedCell = *c
	}
	return cursorProbe{pos: pos, saved: saved, inferred: inferred, pending: pending, cell: cell, savedCell: savedCell}
}

// seedPen is the SGR that sets the pen c was printed in.
func seedPen(c uv.Cell) string {
	if c.Style.IsZero() {
		return "\x1b[m"
	}
	return "\x1b[m" + c.Style.String()
}

// seedLink is the OSC 8 that opens c's link, or "".
// vt stores OSC 8 params in Link.URL and the URI in Link.Params (charstate.go).
func seedLink(c uv.Cell) string {
	if c.Link.IsZero() {
		return ""
	}
	return ansi.SetHyperlink(c.Link.Params, c.Link.URL)
}

// applySeedLocked replaces the live emulator with a fresh cols x rows one
// fed seed (seedLocked), then tail. Caller holds emuMu.
func (h *Hub) applySeedLocked(seed, tail []byte, cols, rows int) {
	h.liveEmu = h.newLiveEmulator(cols, rows)
	h.liveEmu.Write(seed)     //nolint:errcheck // emulator Write never returns a meaningful error
	h.altExited = false       // the pending-wrap switch in the seed is not the app's exit
	h.drainScrollbackLocked() // the seed fills exactly Height rows, so nothing scrolls
	if len(tail) > 0 && (tail[0] == 0x1b || tail[0] == 0x9b) {
		// A C0 inside an ESC or CSI ran at once on the old screen, which the
		// seed captured; replayed, it would run twice (IN-10). Strings ignore
		// C0s, so an OSC or DCS after the ESC loses nothing. CAN and SUB cannot
		// be in a tail, ESC can.
		// shortcut: vt's parser also lets the C0 overwrite the sequence's
		// command bits (an LF turns a later m into o); xterm.js does not, and
		// the rebuild follows xterm.js there. Rare; leave it unless it shows up.
		tail = slices.DeleteFunc(slices.Clone(tail), func(c byte) bool { return c < 0x20 && c != 0x1b })
	}
	h.liveEmu.Write(tail) //nolint:errcheck // emulator Write never returns a meaningful error
}

// altScrollbackWarn logs disableAltScreenScrollback's failure once per process.
var altScrollbackWarn sync.Once

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
	// The alt screen keeps no scrollback (vtalt.go, DEBT-01). If vt's layout
	// changed, it grows until the app exits, and the exit rebuild frees it.
	if !disableAltScreenScrollback(emu) {
		altScrollbackWarn.Do(func() {
			log.Printf("relay: vt layout changed; alt-screen scrollback stays unbounded until the app leaves the alt screen (DEBT-01)")
		})
	}
	emu.SetScrollbackSize(HistoryMaxLines)
	// Track DEC private modes for the replay's restores; ANSI modes are not
	// replayed (the callbacks report both).
	track := func(on bool) func(ansi.Mode) {
		return func(m ansi.Mode) {
			if d, ok := m.(ansi.DECMode); ok {
				h.modes[int(d)] = on
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
			}
		},
		// A soft wrap from the last column moves the cursor down (index, or
		// ScrollUp's two moves), then the print lands it on column 1 or 2 of
		// the new row, from column W-1: record the row above (DEBT-02). Runs
		// inside emu.Write under emuMu and touches only Hub state (vt has no
		// lock of its own). vt's cursor-motion commands can make the same two
		// moves (an LF, IND or CUD from column W-1, then a CHA, CUP or HPA to
		// column 1 or 2); the handlers below keep them from pairing (WR-03).
		CursorPosition: func(old, new uv.Position) { //nolint:predeclared,revive // vt's parameter names
			prev := h.lastMove
			h.lastMove = [2]uv.Position{old, new}
			w := emu.Width()
			if !h.trackWraps || emu.IsAltScreen() || old.X != w-1 || new.Y != old.Y || new.Y < 1 ||
				(new.X != 1 && new.X != 2) || prev[1] != old || prev[0].X != w-1 || prev[0].Y == old.Y {
				return
			}
			if l := screenLine(emu, new.Y-1); lineFull(l, w) {
				h.wraps.add(rowKey(l.Render()))
			}
		},
	})
	// WR-03: only vt's autowrap counts as a wrap. Registered handlers run
	// before vt's, so every cursor-motion CSI and ESC forgets the last move
	// first, and its own move then never completes a pair. C0 moves cannot be
	// marked (registerCcHandler is unexported); of those only BS reaches
	// column 1 or 2 from the last one, on a 3- or 4-column grid.
	notWrap := func() { h.lastMove = [2]uv.Position{{X: -1, Y: -1}, {X: -1, Y: -1}} }
	for _, f := range "ABCDEFGHILMZ`adefrs" {
		emu.RegisterCsiHandler(int(f), func(ansi.Params) bool { notWrap(); return false })
	}
	for _, f := range "8DM" {
		emu.RegisterEscHandler(int(f), func() bool { notWrap(); return false })
	}
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
		n, _, _ := params.Param(0, 0)
		if n == 3 {
			h.history.Clear()
			h.wraps.clear()
		}
		// IN-02: ED 0 from home and ED 1 erase row 0's start without a
		// scroll; ED 2 pushes row 0 into history after the line, which stays.
		if p := emu.CursorPosition(); n == 1 || (n == 0 && p == uv.Position{}) {
			h.row0ErasedLocked(emu)
		}
		return false
	})
	emu.RegisterCsiHandler('K', func(params ansi.Params) bool {
		n, _, _ := params.Param(0, 0)
		if p := emu.CursorPosition(); p.Y == 0 && (n == 1 || n == 2 || (n == 0 && p.X == 0)) {
			h.row0ErasedLocked(emu)
		}
		return false
	})
	// IN-10: the other ways row 0's start changes without a scroll. RI at
	// the top, IL on row 0 and SD push row 0 down (a pager scrolling back),
	// only when the region starts at row 0; ECH and DCH at home blank or
	// shift its first cells.
	regionAtTop := func() bool { return h.mainRegion[0] <= 1 }
	row0Shift := func() bool {
		if emu.CursorPosition().Y == 0 && regionAtTop() {
			h.row0ErasedLocked(emu)
		}
		return false
	}
	emu.RegisterEscHandler('M', row0Shift)
	emu.RegisterCsiHandler('L', func(ansi.Params) bool { return row0Shift() })
	emu.RegisterCsiHandler('T', func(ansi.Params) bool {
		if regionAtTop() {
			h.row0ErasedLocked(emu)
		}
		return false
	})
	for _, f := range "XP" {
		emu.RegisterCsiHandler(int(f), func(ansi.Params) bool {
			if emu.CursorPosition() == (uv.Position{}) {
				h.row0ErasedLocked(emu)
			}
			return false
		})
	}
	// RIS (ESC c) resets the terminal: empty the history and the mode state.
	// The default reset does not touch the emulator scrollback, so clear lines
	// scrolled earlier in this same write as well. The default handler then
	// reports every mode at its default, which refills h.modes.
	emu.RegisterEscHandler('c', func() bool {
		h.history.Clear()
		h.wraps.clear()
		clear(h.modes)
		h.mainRegion = [2]int{}
		h.cs.reset()
		emu.ClearScrollback()
		return false
	})
	// G0/G1 designations and the OSC 8 link, for the replay suffix (IN-08).
	// Registered last, so they run before vt's own handlers.
	for set, intro := range []byte{'(', ')'} {
		for _, f := range []byte{'0', 'A', 'B'} {
			emu.RegisterEscHandler(ansi.Command(0, intro, f), func() bool {
				h.cs.designate(set, f)
				return false
			})
		}
	}
	emu.RegisterOscHandler(8, func(data []byte) bool {
		h.cs.osc8(data)
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
	aw, w := h.autowrapLocked(), h.liveEmu.Width()
	for _, l := range ls {
		r := l.Render() // wrap keys are on the raw render
		h.history.Push(fixRenderedLinks(r), aw && lineFull(l, w) && h.wraps.take(rowKey(r)))
	}
	clear(ls[:cap(ls)])
	sb.Clear()
}

// row0ErasedLocked is called when an erase is about to blank the start of
// emu's main-screen row 0 without scrolling: the line above it, the newest
// in emu's scrollback or else in history, no longer continues onto it
// (IN-02). Runs inside emu.Write under emuMu.
func (h *Hub) row0ErasedLocked(emu *xvt.Emulator) {
	switch {
	case emu.IsAltScreen():
	case emu.ScrollbackLen() > 0:
		// shortcut: takes one count by hash, so after a line editor's redraw
		// (a second count) the line stays joined to row 0, or an identical
		// wrapped row loses its mark instead (08-REVIEW IN-13). Reflow and
		// copy only, like the wraps.go shortcut; mark the newest line by
		// position in drainScrollbackLocked if that shows up.
		ls := emu.Scrollback().Lines()
		h.wraps.take(rowKey(ls[len(ls)-1].Render()))
	default:
		h.history.ClearEndWrap()
	}
}

// autowrapLocked reports whether DECAWM is on (vt's default). Caller holds emuMu.
func (h *Hub) autowrapLocked() bool {
	aw, ok := h.modes[7]
	return aw || !ok
}

// screenLine copies row y of emu's active screen.
func screenLine(emu *xvt.Emulator, y int) uv.Line {
	l := make(uv.Line, emu.Width())
	for x := range l {
		if c := emu.CellAt(x, y); c != nil {
			l[x] = *c
		}
	}
	return l
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
		lines = lines[:min(old.CursorPosition().Y+1, len(lines))]
		aw := h.autowrapLocked()
		for y, row := range lines {
			// The cursor row's continuation (below it) is dropped: never wrapped.
			wrapped := aw && y < len(lines)-1 && lineFull(screenLine(old, y), old.Width()) && h.wraps.take(rowKey(row))
			h.history.Push(fixRenderedLinks(row), wrapped)
		}
	}
	h.wraps.clear()         // the new emulator starts empty
	h.mainRegion = [2]int{} // the new emulator starts without one
	h.liveEmu = h.newLiveEmulator(cols, rows)
	// The host terminal keeps its charsets and open link across a resize; the
	// new emulator starts without them (IN-08).
	seed := h.cs.suffix()
	if inAlt {
		seed = altScreenEnterSeq + seed
	}
	h.liveEmu.Write([]byte(seed)) //nolint:errcheck // emulator Write never returns a meaningful error
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

// WriteInput writes raw input bytes to the underlying PTY writer (stdin),
// blocking. The read pumps use the input FIFO (OpenInput) instead; this stays
// for direct callers and tests.
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
// nil when there is no emulator or nothing to show: the mode restores, the
// screen body, then the charsets and open link (charState.suffix). joinLocked
// puts the history after the restores; the alt-exit rebuild seeds with the
// restores and body and re-emits the state from its probes. Caller holds emuMu.
func (h *Hub) renderScreenLocked() []byte {
	body := h.screenBodyLocked()
	if body == nil {
		return nil
	}
	return append([]byte(h.modeRestoresLocked()), append(body, h.cs.suffix()...)...)
}

// modeRestoresLocked returns the app's DEC private modes (bracketed paste,
// cursor keys, cursor visibility, mouse) in ascending order, so the viewer's
// terminal behaves like the host even though the ring wrapped past the sets.
// Never the alt-screen modes (the body's prefix), 2048 (in-band resize,
// stripped on purpose by liveEmuQueryStripPattern) or 7 (the body restores
// autowrap after its text).
//
// IN-06: they go before any replayed text, so a mode that changes layout
// (?2027 grapheme width) applies to the rows it governs. In a Join they go
// before the history too: xterm.js homes the cursor on ?6h and ?6l, and a
// RIS puts 6 in h.modes (WR-08), so between the history and the screen the
// home would put the screen rows over the history rows still in the viewport.
// Right after the viewer's reset the cursor is home anyway. No DECSTBM is
// replayed, so the region is the full screen and the body's CUP stays
// absolute. In vt (the alt-exit seed) only 47/1047/1048/1049 move the
// cursor, all excluded, and a new emulator has no region for ?6h to use.
// Caller holds emuMu.
func (h *Hub) modeRestoresLocked() string {
	var b strings.Builder
	for _, m := range slices.Sorted(maps.Keys(h.modes)) {
		switch m {
		case 7, 47, 1047, 1048, 1049, 2048, 2026:
			// 2026 (synchronized output): the replay is a complete screen, so a
			// join mid-update must not leave the viewer holding its rendering.
			// 1048 is an action, not a state: replayed, ?1048l restores the
			// saved cursor over the body's CUP (WR-06 fuzz).
			continue
		}
		set := byte('l')
		if h.modes[m] {
			set = 'h'
		}
		fmt.Fprintf(&b, "\x1b[?%d%c", m, set)
	}
	return b.String()
}

// screenBodyLocked returns the screen part of a rendered replay, or nil when
// there is no emulator or nothing to show: the alt-screen prefix, the rows,
// the autowrap restore, the cursor CUP and the pending-wrap re-print. Caller
// holds emuMu.
func (h *Hub) screenBodyLocked() []byte {
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
	// DEBT-02: a row vt soft-wrapped is joined to the next with no CRLF, so the
	// viewer wraps it too (screenRowWrappedLocked).
	var b strings.Builder
	if altScreen {
		b.WriteString(altScreenEnterSeq)
	}
	rows := strings.Split(h.liveEmu.Render(), "\n")
	for y, row := range rows {
		b.WriteString(fixRenderedLinks(row)) // wrap keys are on the raw row
		if y+1 < len(rows) && !h.screenRowWrappedLocked(y, row, rows[y+1]) {
			b.WriteString("\r\n")
		}
	}
	// Autowrap is the one restore after the text: it decides how the replayed
	// text lays out, and ?7l first would truncate history lines rendered
	// before a host narrowing and break the soft-wrap joins. Toggling it moves
	// no cursor in xterm.js or vt.
	if aw, ok := h.modes[7]; ok {
		set := byte('l')
		if aw {
			set = 'h'
		}
		fmt.Fprintf(&b, "\x1b[?7%c", set)
	}
	pos := h.liveEmu.CursorPosition()
	fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
	// A host cursor at the last column after printing there is in the pending
	// (deferred) wrap state, which a CUP clears: re-print that cell so the
	// viewer's next character wraps as the host's does. vt does not expose the
	// state, so this infers it. After ?7: pendingWrapLocked requires autowrap.
	b.WriteString(h.pendingWrapLocked(pos))
	return []byte(b.String())
}

// screenRowWrappedLocked reports whether screen row y (rendered row) soft-wrapped
// onto the next row (rendered next), so a replay may join them with no CRLF:
// main screen, autowrap on, row y full and recorded by the CursorPosition
// callback, and a non-empty next row (before an empty one, the next row's CR
// would cancel the viewer's pending wrap and lose that row). Caller holds emuMu.
func (h *Hub) screenRowWrappedLocked(y int, row, next string) bool {
	return next != "" && len(h.wraps.n) > 0 && !h.liveEmu.IsAltScreen() && h.autowrapLocked() &&
		h.wraps.has(rowKey(row)) && lineFull(screenLine(h.liveEmu, y), h.liveEmu.Width())
}

// pendingWrapLocked returns the re-print of the live emulator's cell at pos
// when the cursor there is in pending wrap, or "". Caller holds emuMu.
// shortcut: a cursor moved onto a filled last column (not printed there) is
// replayed as pending wrap too; track the state per write if that shows up.
func (h *Hub) pendingWrapLocked(pos uv.Position) string {
	if aw, ok := h.modes[7]; (aw || !ok) && pos.X == h.liveEmu.Width()-1 {
		if c := h.liveEmu.CellAt(pos.X, pos.Y); c != nil && c.Width == 1 && c.Content != "" && c.Content != " " {
			return fixRenderedLinks(uv.Line{*c}.Render())
		}
	}
	return ""
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
// size. The ring stores 0x01-framed chunks; stripRingQueries skips those
// headers while matching, so a query split across two PTY reads is stripped.
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
// client takes its position from the offset meta, never by counting them
// (relayClient.ts and attach/loop.go stop counting at the replay meta).
//
// Geometry (05-08): the ring stores raw bytes but not resizes. Bytes written
// before a genuine host resize, replayed into a viewer at the new width, put
// relative redraws (Claude Code's Ink frames) in the wrong rows: the reattach
// jumble. So a delta is sent only from an offset at or past the last genuine
// resize (geomEnd), and the raw full replay only while no genuine resize was
// recorded; otherwise the rendered history + screen goes out. The hub's first
// resize from its unknown grid is not genuine (the PTY was spawned at that
// grid), so the #109 raw replay keeps the common never-resized case.
func (h *Hub) Join(sub *Subscriber, resume uint64, hasResume bool) Replay {
	r, data, raw := h.joinLocked(sub, resume, hasResume)
	// Strip and chunk outside emuMu so the drain loop is not held up.
	if raw {
		data = stripRingQueries(data)
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

	// Every byte at or after geomEnd follows the resize, and the caller sends
	// the current grid frame first, so a viewer resuming at geomEnd ends like
	// one that stayed connected through it; only a resume before geomEnd would
	// replay pre-resize bytes at the new width (05-08). R-05 (10-07-FIX).
	if hasResume && (h.geomEnd == 0 || resume >= h.geomEnd) {
		if tail, end, ok := h.scrollback.SnapshotFrom(resume); ok {
			return Replay{Mode: ReplayDelta, End: end}, tail, true
		}
	}
	r = Replay{Mode: ReplayFull, End: h.scrollback.End()}
	if h.scrollback.Truncated() || h.geomEnd > 0 {
		// Ring wrapped (raw is incomplete) or it holds bytes laid out for an
		// older grid: prefer the emulator's complete screen, after the history
		// that scrolled off it (SHAR-07, 05-08).
		if screen := h.screenBodyLocked(); len(screen) > 0 {
			out := h.modeRestoresLocked() // before the history (IN-06, modeRestoresLocked)
			if hist := h.history.Render(); hist != "" {
				// The newest history line soft-wrapped onto the screen's first
				// row (DEBT-02): join them like screen rows.
				// shortcut: joined rows reflow on a remote `portty attach`
				// wider than the host grid (the CLI ignores the 0x02 grid, so
				// a narrower one is misaligned already; GUI and browser
				// viewers run at the host grid). Track the viewer's width if
				// that shows up.
				sep := "\r\n"
				if h.history.EndsWrapped() && !h.liveEmu.IsAltScreen() && h.autowrapLocked() &&
					screenLine(h.liveEmu, 0).Render() != "" {
					sep = ""
				}
				out += hist + sep
			}
			return r, append(append([]byte(out), screen...), h.cs.suffix()...), false
		}
	}
	// Ring intact at one grid (complete and byte-faithful), or the emulator has nothing to
	// render: fall back to the raw tail rather than sending nothing.
	return r, h.scrollback.Snapshot(), true
}

// Cols returns the current PTY column width as set by a claim, the bootstrap
// or a handoff import. Returns 220 when no resize has been applied (fallback for
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

// Rows returns the current PTY row height as set by a claim, the bootstrap
// or a handoff import. Returns 50 when no resize has been applied (fallback mirrors
// engine.go emuRows default; needed by VIEW-03 join-push in Plan 02).
func (h *Hub) Rows() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ptyRows <= 0 {
		return 50
	}
	return h.ptyRows
}

// GridKnown reports whether the grid was ever set (bootstrap, claim or a
// handoff import), so Cols and Rows are not the 220x50 fallback.
func (h *Hub) GridKnown() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ptyCols > 0 && h.ptyRows > 0
}

// Done returns a channel that is closed when the hub shuts down.
func (h *Hub) Done() <-chan struct{} {
	return h.done
}
