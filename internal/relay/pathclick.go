package relay

import (
	"errors"
	"log"
	"math"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/scottkw/portty/internal/previewfile"
)

// Path click (PATH-02, Phase 12). A viewer that may write sends MsgPathClick
// with the text of a path it underlined; the host previews the file in every
// viewer, focused for the clicker. The host reads a clicked file only when all
// three hold:
//
//  1. the text appears, as a whole token, in this session's own retained
//     output (ANSI-stripped history plus the live screen; never the raw ring,
//     so a title or other escape string does not count),
//  2. the requester can write to the session (a read-only subscriber is
//     refused here and in both read pumps),
//  3. the file passes the `portty open` checks (previewfile.Open, the only
//     file read; this file imports no os, os/exec or path/filepath).
//
// Authorisation is "this exact text was printed by this session", not "inside
// a root", so `..` needs no special handling. Nothing is executed or handed to
// another application: the bytes only go into the Phase 11 preview.
//
// Every refusal is silent. Only an existing file of an unsupported type or over
// the cap gives the clicker (never the other viewers) a notice. No log line
// ever carries the clicked text, a path or a file name.

// Outcome codes: the seam's argument and the `outcome=` of the log line.
const (
	pcPreview     = "preview"
	pcNotPrinted  = "not-printed"
	pcNoCwd       = "no-cwd"
	pcMissing     = "missing"
	pcUnsupported = "unsupported"
	pcTooLarge    = "too-large"
	pcSendFailed  = "send-failed"
)

// PathClick starts resolving a click from sub and reports whether it did. It
// returns at once: the resolve runs on its own goroutine, so the read pump
// keeps answering pings (a slow disk must not get the viewer dropped as dead).
// Cheap checks come first and every refusal is silent: a nil or read-only
// subscriber, a payload that is empty, over PathClickMaxBytes, not UTF-8 or
// holds a control character, a hub that ended or is freezing for an upgrade,
// a preview still streaming, a click already resolving. A dropped click is
// never queued; the user clicks again.
func (h *Hub) PathClick(sub *Subscriber, payload []byte) bool {
	if sub == nil || sub.ReadOnly || len(payload) == 0 || len(payload) > PathClickMaxBytes || !utf8.Valid(payload) {
		return false
	}
	text := string(payload)
	if strings.ContainsFunc(text, unicode.IsControl) {
		return false
	}
	h.mu.Lock()
	idle := !h.closed && !h.freezing && !h.previewBusy
	h.mu.Unlock()
	if !idle || !h.pathClickBusy.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		outcome, n := h.resolvePathClick(sub, text)
		log.Printf("viewer-diag: path click session=%s conn=%d outcome=%s bytes=%d",
			DiagID(h.sessionID), sub.ID, outcome, n)
		h.pathClickBusy.Store(false)
		if h.pathClickResolved != nil {
			h.pathClickResolved(outcome)
		}
	}()
	return true
}

// resolvePathClick returns the outcome code and the bytes of the previewed (or
// refused-for-size) file, for the log line.
func (h *Hub) resolvePathClick(sub *Subscriber, text string) (string, int) {
	text = stripLineCol(text)
	if text == "" {
		return pcNotPrinted, 0
	}
	snap, ok := h.pathSnapshot()
	if !ok {
		return pcNotPrinted, 0
	}
	row, found := snap.findPrinted(text)
	if !found {
		return pcNotPrinted, 0
	}
	p, ok := resolvePath(text, row, &snap.cwds)
	if !ok {
		return pcNoCwd, 0
	}
	f, err := previewfile.Open(p, PreviewMaxBytes)
	var big *previewfile.TooLargeError
	switch {
	case errors.As(err, &big):
		h.pathNotice(sub, NoticePathTooLarge)
		return pcTooLarge, int(big.Size)
	case errors.Is(err, previewfile.ErrUnsupported):
		h.pathNotice(sub, NoticePathUnsupported)
		return pcUnsupported, 0
	case err != nil: // missing, a folder, unreadable: nothing, for anyone
		return pcMissing, 0
	}
	_, err = h.SendPreview(Preview{Name: CleanPreviewName(f.Name), MIME: f.MIME, Key: f.Key, Data: f.Data, FocusSub: sub})
	if err != nil {
		return pcSendFailed, len(f.Data)
	}
	return pcPreview, len(f.Data)
}

// pathNotice queues a notice to sub alone, never broadcast: every viewer would
// otherwise see "Can't preview this file type".
func (h *Hub) pathNotice(sub *Subscriber, code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscribers[sub]; ok && !sub.revoked && !h.closed {
		queueIfRoom(sub, MakeMeta(MetaPayload{Notice: &code}))
	}
}

// stripLineCol drops a trailing `:line` or `:line:col` (the viewer strips it
// too; stripping on the host as well keeps a hand-built frame honest).
func stripLineCol(s string) string {
	for range 2 {
		i := strings.LastIndexByte(s, ':')
		if i <= 0 || i == len(s)-1 || strings.Trim(s[i+1:], "0123456789") != "" {
			break
		}
		s = s[:i]
	}
	return s
}

// resolvePath maps the clicked text to a file path. `/…` is used as is; `~/…`
// is under the user's home (bare `~` and `~user` are refused); anything else is
// relative to the directory in effect at the row it was printed on, and with no
// directory known there it is refused (never seeded from the spawn directory).
func resolvePath(text string, row uint64, cwds *cwdLog) (string, bool) {
	switch {
	case strings.HasPrefix(text, "/"):
		return text, true
	case strings.HasPrefix(text, "~/"):
		home, err := userHomeDir()
		if err != nil || !path.IsAbs(home) {
			return "", false
		}
		return path.Join(home, text[2:]), true
	case strings.HasPrefix(text, "~"):
		return "", false
	}
	dir, ok := cwds.at(row)
	if !ok {
		return "", false
	}
	return path.Join(dir, text), true
}

// pathRow is one retained output row, ANSI-stripped. wrap is the hub's mark
// that the row continues onto the next one; full is whether the row fills the
// live width (fallback pass only); row is its absolute row number (cwd.go).
type pathRow struct {
	text string
	row  uint64
	wrap bool
	full bool
}

// pathSnap is what a click searches: a copy taken under emuMu, so the search
// itself runs with the lock released (Run records and broadcasts under emuMu;
// scanning 10,000 lines under it would stall every viewer).
type pathSnap struct {
	rows []pathRow
	cwds cwdLog
}

// pathSnapshot copies the history and the live screen with the wrap marks the
// replay uses, and the cwd log. ok is false before any output.
func (h *Hub) pathSnapshot() (pathSnap, bool) {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	if h.liveEmu == nil {
		return pathSnap{}, false
	}
	hist, wrapped, first := h.history.Snapshot()
	screen := strings.Split(h.liveEmu.Render(), "\n")
	width, alt := h.liveEmu.Width(), h.liveEmu.IsAltScreen()
	// The newest history line wrapped onto screen row 0 (as joinLocked decides).
	joinHist := h.history.EndsWrapped() && !alt && h.autowrapLocked() && screenLine(h.liveEmu, 0).Render() != ""

	rows := make([]pathRow, 0, len(hist)+len(screen))
	for i, line := range hist {
		wrap := wrapped[i]
		if i == len(hist)-1 {
			wrap = joinHist
		}
		rows = append(rows, pathRow{text: line, row: first + uint64(i), wrap: wrap})
	}
	pushed := h.history.Pushed()
	for y, line := range screen {
		wrap := y+1 < len(screen) && h.screenRowWrappedLocked(y, line, screen[y+1])
		full := lineFull(screenLine(h.liveEmu, y), width)
		rows = append(rows, pathRow{text: line, row: pushed + uint64(y), wrap: wrap, full: full})
	}
	snap := pathSnap{rows: rows, cwds: cwdLog{recs: h.cwds.records()}}
	if alt {
		// A full-screen program's rows have no cwd of their own: the newest
		// directory applies.
		for i := len(hist); i < len(rows); i++ {
			rows[i].row = math.MaxUint64
		}
	}
	snap.finish(len(hist), width)
	return snap, true
}

// finish strips the rows (outside emuMu's reach: the copy is ours) and marks
// the history rows that fill the live width. nHist is how many rows are
// history.
func (s *pathSnap) finish(nHist, width int) {
	for i := range s.rows {
		s.rows[i].text = ansi.Strip(s.rows[i].text)
		if i < nHist {
			s.rows[i].full = ansi.StringWidth(s.rows[i].text) == width
		}
	}
}

// findPrinted returns the absolute row of the newest logical line holding text
// as a whole token.
func (s *pathSnap) findPrinted(text string) (uint64, bool) {
	return s.firstPass(text)
}

// firstPass joins rows the hub marked as soft-wrapped into logical lines, walks
// them newest to oldest and returns the row of the first row of the first line
// that holds text as a token.
func (s *pathSnap) firstPass(text string) (uint64, bool) {
	// Walk backwards: a logical line ends at a row with no wrap mark.
	for end := len(s.rows) - 1; end >= 0; {
		start := end
		for start > 0 && s.rows[start-1].wrap {
			start--
		}
		var b strings.Builder
		for i := start; i <= end; i++ {
			b.WriteString(s.rows[i].text)
		}
		if hasToken(b.String(), text) {
			return s.rows[start].row, true
		}
		end = start - 1
	}
	return 0, false
}

// isPathChar is the path alphabet: letters, digits, `_ . ~ @ % + # -` and `/`.
// `=` and `:` are outside it, so `--out=/tmp/a.png` and `a/b.png:12:3` match.
func isPathChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_.~@%+#-/", r)
}

// hasToken reports whether text occurs in s as a whole token.
func hasToken(s, text string) bool {
	for off := 0; off <= len(s)-len(text); {
		i := strings.Index(s[off:], text)
		if i < 0 {
			return false
		}
		i += off
		if tokenAt(s, i, len(text)) {
			return true
		}
		off = i + 1
	}
	return false
}

// tokenAt: the match s[i:i+n] is a token when the character before it is absent
// or outside the alphabet, and the one after it is absent, outside the
// alphabet, or a `.` itself followed by absent or an outside character (the
// full stop that ends a sentence). `/x/.ssh/id_rsa` is therefore not a token of
// `/x/.ssh/id_rsa.pub`.
func tokenAt(s string, i, n int) bool {
	if i > 0 {
		if r, _ := utf8.DecodeLastRuneInString(s[:i]); isPathChar(r) {
			return false
		}
	}
	rest := s[i+n:]
	if rest == "" {
		return true
	}
	r, size := utf8.DecodeRuneInString(rest)
	if !isPathChar(r) {
		return true
	}
	if r != '.' {
		return false
	}
	rest = rest[size:]
	if rest == "" {
		return true
	}
	r, _ = utf8.DecodeRuneInString(rest)
	return !isPathChar(r)
}
