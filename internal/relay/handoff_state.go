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

	uv "github.com/charmbracelet/ultraviolet"
)

// maxHandoffGrid bounds the grid and emulator sizes an import accepts.
const maxHandoffGrid = 1000

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
		st.Seed = h.seedLocked(st.Tail)
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
	h.scrollback.buf = append(h.scrollback.buf, st.Ring.Buf...)
	h.scrollback.truncated, h.scrollback.end = st.Ring.Truncated, st.Ring.End
	h.history = NewHistory(st.History.MaxLines, st.History.MaxBytes)
	for i, l := range st.History.Lines {
		h.history.Push(l, st.History.Wrapped[i])
	}
	maps.Copy(h.modes, st.Modes)
	h.geomEnd = st.GeomEnd
	h.mainRegion = st.MainRegion
	if len(st.Wraps.Ring) > 0 {
		h.wraps = wrapSet{n: maps.Clone(st.Wraps.N), ring: slices.Clone(st.Wraps.Ring), next: st.Wraps.Next}
	}
	h.cs = charState{g: st.CS.G, gl1: st.CS.GL1, link: st.CS.Link, vt: vtScan{state: st.CS.ScanState, need: st.CS.ScanNeed}, esc: st.CS.Esc}
	h.ptyCols, h.ptyRows = st.PtyCols, st.PtyRows
	if st.HasEmu {
		h.emuMu.Lock()
		h.applySeedLocked(st.Seed, st.Tail, st.EmuCols, st.EmuRows)
		h.lastMove = st.LastMove // the seed's own moves are not the app's
		h.emuMu.Unlock()
	}
	return h, nil
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

// Digest summarises what a viewer gets from the hub: the ring end and
// length, the history line count and the rendered screen. 09-04 compares the
// old and new hubs' digests before COMMIT.
func (h *Hub) Digest() [32]byte {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	d := sha256.New()
	var n [8]byte
	for _, v := range []uint64{h.scrollback.End(), uint64(len(h.scrollback.Snapshot())), uint64(h.history.Len())} {
		binary.BigEndian.PutUint64(n[:], v)
		d.Write(n[:]) //nolint:errcheck // hash writes never fail
	}
	d.Write(h.renderScreenLocked()) //nolint:errcheck // hash writes never fail
	return [32]byte(d.Sum(nil))
}
