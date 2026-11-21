package relay

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Working-directory tracking (PATH-01, Phase 12). A shell that reports its
// directory with OSC 7 (`ESC ] 7 ; file://host/dir ST`: the Portty zsh shim,
// or a user's bash or fish setup) lets a clicked relative path resolve against
// the directory in effect where the text was printed, not wherever the shell
// is now.
//
// The hub parses the report inside the live emulator's write, in PTY byte
// order, and records it against an ABSOLUTE row: the number of lines pushed to
// history so far, plus the emulator's undrained scrollback, plus the cursor
// row. History line i has row History.FirstRow()+i; main-screen row y has row
// History.Pushed()+y once the scrollback is drained. The numbers stay valid
// through scrolling, resize, ED 2, alt-screen exit and history trimming, so a
// line keeps its directory when it scrolls into history.
//
// vt's own Callbacks.WorkingDirectory is not used: it drops payloads with a
// second ';', and the row must be taken at the report, not after the write.

const (
	// cwdLogMaxRecords and cwdLogMaxBytes bound the log: 1024 directory
	// changes inside 10,000 history lines is far beyond normal use. Past
	// either cap the oldest lines lose their directory, so relative clicks on
	// them do nothing. 64 KiB fits inside the handoff state's size margin.
	cwdLogMaxRecords = 1024
	cwdLogMaxBytes   = 64 << 10

	// osc7MaxPayload bounds the URL after "7;"; cwdMaxPathBytes bounds the
	// decoded path.
	osc7MaxPayload  = 2048
	cwdMaxPathBytes = 1024
)

// CwdRecord says the shell was in Dir from output row Row on. The fields are
// exported for gob (the upgrade handoff).
type CwdRecord struct {
	Row uint64
	Dir string
}

// cwdLog holds the records, rows strictly increasing, oldest first. It has no
// mutex: the hub touches it only under Hub.emuMu. The zero value is an empty
// log.
type cwdLog struct {
	recs  []CwdRecord
	bytes int // total bytes of the retained directories
}

// add records that the shell is in dir from row on. firstRow is History's
// oldest retained row. A report at or above an earlier record's row replaces
// every record at or after row (the cursor moved up, or a second report landed
// on the same row); a repeat of the directory in effect adds nothing; records
// older than firstRow are dropped except the newest, which still governs the
// oldest retained line.
func (l *cwdLog) add(row uint64, dir string, firstRow uint64) {
	for n := len(l.recs); n > 0 && l.recs[n-1].Row >= row; n = len(l.recs) {
		l.bytes -= len(l.recs[n-1].Dir)
		l.recs = l.recs[:n-1]
	}
	if n := len(l.recs); n == 0 || l.recs[n-1].Dir != dir {
		l.recs = append(l.recs, CwdRecord{Row: row, Dir: dir})
		l.bytes += len(dir)
	}
	// The floor: the newest record at or before firstRow stays.
	l.dropOldest(sort.Search(len(l.recs), func(i int) bool { return l.recs[i].Row > firstRow }) - 1)
	for len(l.recs) > 1 && (len(l.recs) > cwdLogMaxRecords || l.bytes > cwdLogMaxBytes) {
		l.dropOldest(1)
	}
}

func (l *cwdLog) dropOldest(n int) {
	if n <= 0 {
		return
	}
	for _, r := range l.recs[:n] {
		l.bytes -= len(r.Dir)
	}
	l.recs = slices.Delete(l.recs, 0, n)
}

// at returns the directory in effect at row: the newest record at or before it.
func (l *cwdLog) at(row uint64) (string, bool) {
	i := sort.Search(len(l.recs), func(i int) bool { return l.recs[i].Row > row })
	if i == 0 {
		return "", false
	}
	return l.recs[i-1].Dir, true
}

// clear forgets every record (RIS: the next prompt reports again).
func (l *cwdLog) clear() { *l = cwdLog{} }

// ownHost is this Mac's hostname, looked up once. An error leaves it empty, so
// only the empty host and localhost match.
var ownHost = sync.OnceValue(func() string {
	h, _ := os.Hostname()
	return h
})

// hostKey lower-cases a host and drops a trailing dot and a `.local` suffix.
func hostKey(h string) string {
	h = strings.TrimSuffix(strings.ToLower(h), ".")
	return strings.TrimSuffix(h, ".local")
}

// sameHost reports whether host names this machine, as own does, ignoring
// case, a trailing dot and a `.local` suffix on either.
func sameHost(host, own string) bool { return own != "" && hostKey(host) == hostKey(own) }

// parseOSC7 returns the absolute, cleaned directory in an OSC 7 handler
// payload (data includes the leading "7;"), or false to ignore the report: an
// empty payload, a URL that is not a `file:` URL with an absolute path, an
// undecodable or control-character path, a host that is not empty, localhost
// or this Mac (reports from an ssh session name another host), or a payload or
// path over its cap. A raw `%` that is not an escape makes url.Parse fail, so
// emitters that do not percent-encode are not supported; the Portty shim
// always encodes. The host check is not a security boundary.
func parseOSC7(data []byte) (string, bool) {
	p, ok := strings.CutPrefix(string(data), "7;")
	if !ok || len(p) == 0 || len(p) > osc7MaxPayload {
		return "", false
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	if h := strings.TrimSuffix(strings.ToLower(u.Host), "."); h != "" && h != "localhost" && !sameHost(h, ownHost()) {
		return "", false
	}
	// Go decodes %00 and %0A into the path.
	if !filepath.IsAbs(u.Path) || len(u.Path) > cwdMaxPathBytes || hasControl(u.Path) {
		return "", false
	}
	return filepath.Clean(u.Path), true
}

// hasControl reports whether s holds a C0 control character or DEL.
func hasControl(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0
}
