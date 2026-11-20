package relay

// Phase 9 handoff (UPGR-01, UPGR-02): a frozen hub's state as plain data, so
// the old daemon can gob it over the 0600 handoff socket and the new daemon
// can rebuild the hub byte for byte (Research Pattern 1).
//
// The ring moves verbatim with its end offset, so every viewer's resume
// offset stays valid; the seed goes only into the emulator, never into the
// ring (Pitfall 5). The vt emulator has no serializer, so it is rebuilt from
// a rendered seed, the alt-exit rebuild's (Pattern 6). Subscribers are not
// moved: viewers reconnect and resume.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// maxHandoffGrid bounds the grid and emulator sizes an import accepts.
const maxHandoffGrid = 1000

// HandoffStateMaxBytes bounds one exported hub's gob (WR-02): the ring and
// history at their maxima, gob's per-line cost (a length and a wrap flag, at
// most 10 bytes a line), a seed of handoffSeedCells drawn twice (inside a
// full-screen app it carries both screens) at handoffSeedCellBytes a cell (a
// truecolor fg and bg, attributes and the character; truecolor on every cell
// measured 34), and 1 MiB for the rest. A 400x120 truecolor hub with a full
// history and the alt screen is 7.7 MB, under half of it.
// shortcut: a window much larger than 500x200 with truecolor on every cell
// can still exceed it, and its handoff aborts (the restart question); chunk
// the seed if that shows up.
const HandoffStateMaxBytes = DefaultScrollbackBytes + HistoryMaxBytes + 10*HistoryMaxLines +
	2*handoffSeedCells*handoffSeedCellBytes + 1<<20

const (
	handoffSeedCells     = 500 * 200
	handoffSeedCellBytes = 64
)

// HubState is a hub's exported state. Every field is plain data that gob
// encodes; no slice aliases the hub it came from. It holds terminal contents,
// which can include secrets: it moves memory to memory, never to a file.
type HubState struct {
	SessionID string
	Ring      RingState
	History   HistoryState
	Modes     map[int]bool // DEC private modes (Hub.modes)
	GeomEnd   uint64       // ring offset of the last genuine resize (Hub.geomEnd)
	// MainRegion is the main screen's DECSTBM, 1-based, zero for none.
	MainRegion [2]int
	Wraps      WrapState
	LastMove   [2]uv.Position
	CS         CharsetState
	// PtyCols, PtyRows: the host-authority grid; zero when no local viewer
	// ever sized it.
	PtyCols, PtyRows int
	// EmuCols, EmuRows: the live emulator's size. HasEmu is false when no
	// emulator was ever built (no output yet); then Seed and Tail are empty.
	EmuCols, EmuRows int
	HasEmu           bool
	// Seed rebuilds the emulator; Tail, the escape sequence or UTF-8
	// character the last read cut off, is written after it.
	Seed []byte
	Tail []byte
}

// RingState is the scrollback ring (Scrollback).
type RingState struct {
	Buf       []byte
	Max       int
	Truncated bool
	End       uint64
}

// HistoryState is the retained history lines, oldest first, with their wrap
// flags (History).
type HistoryState struct {
	Lines    []string
	Wrapped  []bool
	MaxLines int
	MaxBytes int
}

// WrapState is the soft-wrap key multiset (wrapSet).
type WrapState struct {
	N    map[uint64]int
	Ring []uint64
	Next int
}

// CharsetState is the charset and link tracker (charState), with the GL
// scan's parser state.
type CharsetState struct {
	G         [2]byte
	GL1       bool
	Link      [2]string
	ScanState int
	ScanNeed  int
	Esc       bool
}

// Export returns the frozen hub's state. It fails unless Freeze succeeded
// and Thaw has not run, so no byte arrives during or after the copy.
func (h *Hub) Export() (HubState, error) {
	h.mu.Lock()
	frozen := h.frozenLocked()
	h.mu.Unlock()
	if !frozen {
		return HubState{}, errors.New("relay: export: hub not frozen")
	}
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	return h.exportLocked(), nil
}

// exportLocked builds the state. Caller holds emuMu and nothing feeds the
// hub (frozen, or a test that records frames itself).
func (h *Hub) exportLocked() HubState {
	h.scrollback.mu.Lock()
	ring := RingState{
		Buf:       slices.Clone(h.scrollback.buf),
		Max:       h.scrollback.max,
		Truncated: h.scrollback.truncated,
		End:       h.scrollback.end,
	}
	h.scrollback.mu.Unlock()

	st := HubState{SessionID: h.sessionID, Ring: ring}
	if h.liveEmu != nil {
		st.HasEmu = true
		st.EmuCols, st.EmuRows = h.liveEmu.Width(), h.liveEmu.Height()
		st.Tail = h.exportTailLocked(ring.Buf)
		modes, move := maps.Clone(h.modes), h.lastMove
		st.Seed = h.handoffSeedLocked(st.Tail)
		region, cs := h.mainRegion, h.cs // cs as the probes resynced it
		// Pitfall 7: the probes and the ?1049l changed the live emulator, so
		// it is rebuilt from the export, as the new daemon rebuilds it. A
		// rolled-back handoff (Thaw) resumes on the very state it exported,
		// and the READY digests compare like with like.
		h.applySeedLocked(st.Seed, st.Tail, st.EmuCols, st.EmuRows)
		h.modes, h.lastMove, h.mainRegion, h.cs = modes, move, region, cs // the seed's own writes are not the app's
	}

	hist := h.history
	st.History = HistoryState{
		Lines:    slices.Clone(hist.lines[hist.head:]),
		Wrapped:  slices.Clone(hist.wrapped[hist.head:]),
		MaxLines: hist.maxLines,
		MaxBytes: hist.maxBytes,
	}
	st.Modes = maps.Clone(h.modes)
	st.GeomEnd = h.geomEnd
	st.MainRegion = h.mainRegion
	st.Wraps = WrapState{N: maps.Clone(h.wraps.n), Ring: slices.Clone(h.wraps.ring), Next: h.wraps.next}
	st.LastMove = h.lastMove
	st.CS = CharsetState{G: h.cs.g, GL1: h.cs.gl1, Link: h.cs.link, ScanState: h.cs.vt.state, ScanNeed: h.cs.vt.need, Esc: h.cs.esc}
	h.mu.Lock()
	st.PtyCols, st.PtyRows = h.ptyCols, h.ptyRows
	h.mu.Unlock()
	return st
}

// exportTailLocked returns the escape sequence or UTF-8 character the reads
// so far left open, from ring (the ring's bytes), so the new emulator
// completes it with the next read. The GL scan's parser state says whether
// one is open; the ring, without its frame headers, has its bytes even when
// it spans reads. Caller holds emuMu.
// shortcut: a sequence opened before the ring's oldest byte (a string over
// 256 KiB, such as a huge image) is cut to what the ring holds; keep the open
// sequence's bytes in the hub if that shows up.
func (h *Hub) exportTailLocked(ring []byte) []byte {
	if h.cs.vt == (vtScan{}) {
		return nil
	}
	return incompleteTail(bytes.ReplaceAll(ring, []byte{MsgOutput}, nil))
}

// NewHubFromState validates st and builds a staged hub from it: nothing is
// read from reader until Start. Any invalid field is an error and no hub
// (T-09-11), so the old daemon keeps the session.
func NewHubFromState(st HubState, reader io.Reader, writer io.Writer, resizeFn func(cols, rows int) error) (*Hub, error) {
	if err := st.validate(); err != nil {
		return nil, fmt.Errorf("relay: import %s: %w", DiagID(st.SessionID), err)
	}
	h := NewHub(st.SessionID, reader, writer, st.Ring.Max, resizeFn)
	if st.HasEmu {
		// First, so the state below replaces what the seed's own writes set
		// (modes, region, link, last move), as Export restores it on the old
		// hub. A seed has no erase or reset, so history and wraps are untouched.
		h.applySeedLocked(st.Seed, st.Tail, st.EmuCols, st.EmuRows)
	}
	h.scrollback.buf = append(h.scrollback.buf, st.Ring.Buf...)
	h.scrollback.truncated, h.scrollback.end = st.Ring.Truncated, st.Ring.End
	h.history = NewHistory(st.History.MaxLines, st.History.MaxBytes)
	for i, l := range st.History.Lines {
		h.history.Push(l, st.History.Wrapped[i])
	}
	h.modes = make(map[int]bool, len(st.Modes)) // never nil: the emulator's callbacks write to it
	maps.Copy(h.modes, st.Modes)
	h.geomEnd = st.GeomEnd
	h.mainRegion = st.MainRegion
	h.wraps = wrapSet{}
	if len(st.Wraps.Ring) > 0 {
		h.wraps = wrapSet{n: maps.Clone(st.Wraps.N), ring: slices.Clone(st.Wraps.Ring), next: st.Wraps.Next}
	}
	h.lastMove = st.LastMove
	h.cs = charState{g: st.CS.G, gl1: st.CS.GL1, link: st.CS.Link, vt: vtScan{state: st.CS.ScanState, need: st.CS.ScanNeed}, esc: st.CS.Esc}
	h.ptyCols, h.ptyRows = st.PtyCols, st.PtyRows
	h.settleCols, h.settleRows = st.PtyCols, st.PtyRows // Start begins the settle window
	return h, nil
}

// handoffSeedLocked returns the seed Export moves. On the main screen it is
// seedLocked's. Inside a full-screen app it carries both screens (Pattern 6):
// the alt screen is rendered and probed first, then ?1049l on the live
// emulator (the trick resizeLiveEmulator uses) shows the main screen for
// seedLocked, and the seed is the main seed, ?1047h, then the alt rows,
// cursor, region, saved cursor and pen. Its probes and ?1049l change the live
// emulator, so Export rebuilds it from the seed (Pitfall 7). Caller holds
// emuMu.
func (h *Hub) handoffSeedLocked(tail []byte) []byte {
	emu := h.liveEmu
	if !emu.IsAltScreen() {
		var pre string
		if h.screenBodyLocked() == nil {
			// seedLocked restores the modes only with a body; a blank screen
			// can still have them set (a `clear` after ?7l).
			pre = h.modeRestoresLocked()
			if aw, ok := h.modes[7]; ok {
				pre += map[bool]string{true: "\x1b[?7h", false: "\x1b[?7l"}[aw]
			}
		}
		return append([]byte(pre), h.seedLocked(tail)...)
	}

	body := bytes.TrimPrefix(h.screenBodyLocked(), []byte(altScreenEnterSeq))
	w := emu.Width()
	pos := emu.CursorPosition()
	var posCell, edge uv.Cell // copies, before the probes overwrite cells
	if c := emu.CellAt(pos.X, pos.Y); c != nil {
		posCell = *c
	}
	edgeRow := -1 // a row whose last cell can be re-printed as is
	for y := range emu.Height() {
		if c := emu.CellAt(w-1, y); c != nil && c.Width == 1 && c.Content != "" {
			edge, edgeRow = *c, y
			break
		}
	}
	p := h.probeCursorLocked(tail)
	region := h.probeRegionLocked()
	emu.Write([]byte("\x1b[?1049l"))                  //nolint:errcheck // emulator Write never returns a meaningful error
	h.altExited = false                               // our exit, not the app's
	main := h.seedLocked(nil)                         // the probes left the parser in ground
	h.cs.setLink(p.cell.Link.URL, p.cell.Link.Params) // the alt cursor's link is the current one (WR-02 drops an unsafe one)

	var b strings.Builder
	b.Write(main)
	if h.cs.g[0] != 0 {
		b.WriteString("\x1b(B") // the alt rows print as rendered; the suffix designates again
	}
	b.WriteString(h.modeRestoresLocked()) // the main seed has them only with a main body; none moves the cursor
	// ?1047h, not ?1049h: vt's ?1049h saves the cursor, which would replace
	// the main screen's saved cursor the main seed restored. The alt cursor
	// starts as a copy of the main one, pen and link included: reset both.
	b.WriteString("\x1b[?1047h\x1b[m" + ansi.ResetHyperlink())
	b.Write(body)
	keepSaved := p.saved != (uv.Position{}) || !p.savedCell.Style.IsZero() || !p.savedCell.Link.IsZero()
	if region != [2]int{} || keepSaved || p.inferred != p.pending {
		om := h.modes[6] // DECOM makes the CUPs below region-relative
		if om {
			b.WriteString("\x1b[?6l")
		}
		if region != [2]int{} {
			fmt.Fprintf(&b, "\x1b[%d;%dr", region[0], region[1]) // homes the cursor
		}
		if keepSaved {
			fmt.Fprintf(&b, "%s%s\x1b[%d;%dH\x1b7", seedPen(p.savedCell), seedLink(p.savedCell), p.saved.Y+1, p.saved.X+1)
			if !p.savedCell.Link.IsZero() {
				b.WriteString(ansi.ResetHyperlink())
			}
		}
		fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
		if p.pending {
			// Entering the alt screen clears vt's pending wrap, so the ?1047
			// switch seedLocked uses cannot set it here. Printing a cell at
			// the last column sets it, and DECRC keeps it.
			// shortcut: pending wrap elsewhere than the last column or the
			// saved cursor (a tab or DECRC to another place after the wrap)
			// is lost; set it through a scratch saved cursor if that shows up.
			switch {
			case pos.X == w-1 && posCell.Width == 1 && posCell.Content != "":
				b.WriteString(seedCell(posCell))
			case pos == p.saved && edgeRow >= 0:
				fmt.Fprintf(&b, "\x1b[%d;%dH%s\x1b8", edgeRow+1, w, seedCell(edge))
			}
		}
		if om {
			b.WriteString("\x1b[?6h") // vt's DECOM set does not move the cursor
		}
	}
	b.WriteString(seedPen(p.cell))
	b.WriteString(h.cs.suffix())
	if p.cell.Link.Params == "" && p.cell.Link.URL != "" {
		b.WriteString(seedLink(p.cell)) // vt keeps a link with params and no URI; the tracker does not
	}
	return []byte(b.String())
}

// seedCell re-prints c, a width-1 cell, in its own pen and link.
func seedCell(c uv.Cell) string {
	s := seedPen(c) + ansi.ResetHyperlink() + seedLink(c) + c.Content
	if !c.Link.IsZero() {
		s += ansi.ResetHyperlink()
	}
	return s
}

// probeRegionLocked reads the live emulator's active scroll region (1-based,
// zero for the whole screen) through origin mode, where a CUP clamps to the
// region; vt does not expose it. DECOM and h.modes[6] are put back. Caller
// holds emuMu.
func (h *Hub) probeRegionLocked() [2]int {
	emu := h.liveEmu
	om, had := h.modes[6]
	emu.Write([]byte("\x1b[?6h\x1b[1;1H")) //nolint:errcheck // emulator Write never returns a meaningful error
	top := emu.CursorPosition().Y
	emu.Write([]byte("\x1b[9999;1H")) //nolint:errcheck // emulator Write never returns a meaningful error
	bottom := emu.CursorPosition().Y
	if !om {
		emu.Write([]byte("\x1b[?6l")) //nolint:errcheck // emulator Write never returns a meaningful error
	}
	if had {
		h.modes[6] = om
	} else {
		delete(h.modes, 6)
	}
	if top == 0 && bottom == emu.Height()-1 {
		return [2]int{}
	}
	return [2]int{top + 1, bottom + 1}
}

// validate checks every field an import trusts before any hub exists.
func (st *HubState) validate() error {
	r := st.Ring
	switch {
	case r.Max < 1 || r.Max > DefaultScrollbackBytes:
		return fmt.Errorf("ring max %d outside 1..%d", r.Max, DefaultScrollbackBytes)
	case len(r.Buf) > r.Max:
		return fmt.Errorf("ring holds %d bytes, over its max %d", len(r.Buf), r.Max)
	case uint64(len(r.Buf)) > r.End:
		return fmt.Errorf("ring end %d below its %d bytes", r.End, len(r.Buf))
	case r.Truncated != (r.End > uint64(len(r.Buf))) || (r.Truncated && len(r.Buf) != r.Max):
		return fmt.Errorf("ring truncated %v does not match end %d, %d of %d bytes", r.Truncated, r.End, len(r.Buf), r.Max)
	case st.GeomEnd > r.End:
		return fmt.Errorf("geometry offset %d past the ring end %d", st.GeomEnd, r.End)
	}

	hs := st.History
	n := 0
	for _, l := range hs.Lines {
		n += len(l)
	}
	switch {
	case hs.MaxLines < 1 || hs.MaxLines > HistoryMaxLines || hs.MaxBytes < 1 || hs.MaxBytes > HistoryMaxBytes:
		return fmt.Errorf("history caps %d lines %d bytes outside %d, %d", hs.MaxLines, hs.MaxBytes, HistoryMaxLines, HistoryMaxBytes)
	case len(hs.Lines) > hs.MaxLines || n > hs.MaxBytes:
		return fmt.Errorf("history of %d lines, %d bytes is over its caps", len(hs.Lines), n)
	case len(hs.Wrapped) != len(hs.Lines):
		return fmt.Errorf("history has %d wrap flags for %d lines", len(hs.Wrapped), len(hs.Lines))
	}

	w := st.Wraps
	if len(w.Ring) > wrapSetMax || w.Next < 0 || w.Next >= wrapSetMax || (w.Next != 0 && len(w.Ring) < wrapSetMax) || len(w.N) > len(w.Ring) {
		return fmt.Errorf("wrap keys: %d in the ring, %d counted, next %d", len(w.Ring), len(w.N), w.Next)
	}

	inGrid := func(c, r int) bool { return c >= 1 && c <= maxHandoffGrid && r >= 1 && r <= maxHandoffGrid }
	if (st.PtyCols != 0 || st.PtyRows != 0) && !inGrid(st.PtyCols, st.PtyRows) {
		return fmt.Errorf("grid %dx%d outside 1..%d", st.PtyCols, st.PtyRows, maxHandoffGrid)
	}
	if st.HasEmu {
		if !inGrid(st.EmuCols, st.EmuRows) {
			return fmt.Errorf("emulator %dx%d outside 1..%d", st.EmuCols, st.EmuRows, maxHandoffGrid)
		}
		if reg := st.MainRegion; reg != [2]int{} && (reg[0] < 1 || reg[0] >= reg[1] || reg[1] > st.EmuRows) {
			return fmt.Errorf("main region %v outside rows 1..%d", reg, st.EmuRows)
		}
	} else if len(st.Seed) > 0 || len(st.Tail) > 0 || st.MainRegion != [2]int{} {
		return errors.New("emulator state without an emulator")
	}
	if len(st.Tail) > len(r.Buf) {
		return fmt.Errorf("tail of %d bytes is longer than the ring", len(st.Tail))
	}

	cs := st.CS
	for _, g := range cs.G {
		if g != 0 && g != '0' && g != 'A' {
			return fmt.Errorf("charset designation %q", g)
		}
	}
	if cs.ScanState < scanGround || cs.ScanState > scanStrEsc || cs.ScanNeed < 0 || cs.ScanNeed > 3 {
		return fmt.Errorf("parser state %d need %d", cs.ScanState, cs.ScanNeed)
	}
	if cs.Link[1] == "" && cs.Link[0] != "" || cs.Link[1] != "" && (!safeLinkField(cs.Link[0]) || !safeLinkField(cs.Link[1])) {
		return errors.New("unsafe or malformed open link")
	}
	return nil
}

// Digest summarises what a viewer gets from the hub: the ring end offset and
// bytes (which every delta comes from), the history line count and the
// rendered screen. Tests compare it to check a rebuilt hub; READY hashes
// the SESSION frame bytes instead, which no build difference can break.
func (h *Hub) Digest() [32]byte {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	ring := h.scrollback.Snapshot()
	d := sha256.New()
	var n [8]byte
	for _, v := range []uint64{h.scrollback.End(), uint64(len(ring)), uint64(h.history.Len())} {
		binary.BigEndian.PutUint64(n[:], v)
		d.Write(n[:]) //nolint:errcheck // hash writes never fail
	}
	d.Write(ring)                   //nolint:errcheck // hash writes never fail
	d.Write(h.renderScreenLocked()) //nolint:errcheck // hash writes never fail
	return [32]byte(d.Sum(nil))
}
