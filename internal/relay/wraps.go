package relay

// DEBT-02: vt records no soft-wrap flag. uv.Line is a plain []Cell, the wrap
// in handleGrapheme only moves the cursor, and scrolling copies cells, so a
// row has no identity to follow. The hub therefore notes each wrap as vt
// makes it (newLiveEmulator's CursorPosition callback) by hashing the wrapped
// row's render, and looks the hash up when the row drains into history or is
// rendered. Hashes keep no text and stay bounded; the cap covers one 32 KiB
// read, which can wrap about 400 rows at 80 columns, all drained after it.

import (
	"hash/fnv"

	uv "github.com/charmbracelet/ultraviolet"
)

const wrapSetMax = 4096

// wrapSet is a bounded multiset of wrapped-row keys; the oldest add is
// evicted past wrapSetMax. Zero value is empty. Only touched under emuMu.
type wrapSet struct {
	n    map[uint64]int
	ring []uint64 // adds in order, for eviction
	next int      // oldest ring slot once full
}

func (s *wrapSet) add(k uint64) {
	if s.n == nil {
		s.n = make(map[uint64]int)
	}
	if len(s.ring) < wrapSetMax {
		s.ring = append(s.ring, k)
	} else {
		// An already-taken key may drop a newer add of the same row: a missed join, never a false one.
		s.take(s.ring[s.next])
		s.ring[s.next] = k
		s.next = (s.next + 1) % wrapSetMax
	}
	s.n[k]++
}

func (s *wrapSet) has(k uint64) bool { return s.n[k] > 0 }

// take removes one k and reports whether there was one.
func (s *wrapSet) take(k uint64) bool {
	if s.n[k] == 0 {
		return false
	}
	if s.n[k]--; s.n[k] == 0 {
		delete(s.n, k)
	}
	return true
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
