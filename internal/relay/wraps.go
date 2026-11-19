package relay

// DEBT-02: vt records no soft-wrap flag. uv.Line is a plain []Cell, the wrap
// in handleGrapheme only moves the cursor, and scrolling copies cells, so a
// row has no identity to follow. The hub therefore notes each wrap as vt
// makes it (newLiveEmulator's CursorPosition callback) under the wrapped row's
// absolute line number (Hub.absLine: lines drained so far, plus lines scrolled
// in this write, plus the screen row) with a hash of its render, and looks
// both up when the row drains into history or is rendered. Hashes keep no
// text and stay bounded; the cap covers one 32 KiB read, which can wrap about
// 400 rows at 80 columns, all drained after it.
//
// 08-REVIEW IN-01: the line number gives each wrap its row, so an identical
// hard row elsewhere cannot take it and a redraw of the same row records it
// once. The hash catches a row rewritten in place (erased, reprinted shorter).
// A wrap is missed, never moved onto another row, when vt moves rows without
// scrolling them off the top (IL, DL, RI, a partial scroll region) or one
// write scrolls more than vt's scrollback holds.

import (
	"hash/fnv"

	uv "github.com/charmbracelet/ultraviolet"
)

const wrapSetMax = 4096

// wrapSet maps the absolute line numbers of wrapped main-screen rows to their
// row keys, bounded: the oldest add is evicted past wrapSetMax. Zero value is
// empty. Only touched under emuMu.
type wrapSet struct {
	n    map[uint64]uint64 // line → row key
	ring []uint64          // lines in add order, for eviction
	next int               // oldest ring slot once full
}

// add records that line, whose row key is k, wrapped. Recording a line again
// replaces its key and adds no ring slot.
func (s *wrapSet) add(line, k uint64) {
	if s.n == nil {
		s.n = make(map[uint64]uint64)
	}
	if _, ok := s.n[line]; ok {
		s.n[line] = k
		return
	}
	if len(s.ring) < wrapSetMax {
		s.ring = append(s.ring, line)
	} else {
		// A slot whose line was taken and recorded again since evicts the newer record: a missed join.
		delete(s.n, s.ring[s.next])
		s.ring[s.next] = line
		s.next = (s.next + 1) % wrapSetMax
	}
	s.n[line] = k
}

// has reports whether line wrapped with row key k.
func (s *wrapSet) has(line, k uint64) bool {
	got, ok := s.n[line]
	return ok && got == k
}

// take removes line and reports whether it wrapped with row key k.
func (s *wrapSet) take(line, k uint64) bool {
	ok := s.has(line, k)
	delete(s.n, line)
	return ok
}

func (s *wrapSet) clear() { *s = wrapSet{} }

// rowKey hashes a row's Line.Render().
func rowKey(render string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(render)) //nolint:errcheck // hash writes never fail
	return h.Sum64()
}

// lineFull reports whether l's last non-blank cell reaches column width. vt
// drops a wide character that does not fit at the edge, so a row it wrapped
// short stays a hard break, as in vt's own grid.
func lineFull(l uv.Line, width int) bool {
	for x := len(l) - 1; x >= 0; x-- {
		if c := &l[x]; !c.IsZero() && !c.Equal(&uv.EmptyCell) {
			return x+max(c.Width, 1) >= width
		}
	}
	return false
}
