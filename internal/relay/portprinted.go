package relay

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Phase 13 anti-scan check for the port grant: only a port the session itself
// printed as a loopback http URL can be granted. The search is the one path
// clicks use (pathSnapshot and its two passes) with URL token boundaries, and
// every refusal looks the same to the caller.

// loopbackHosts are the hosts a dev server prints for its own listener.
var loopbackHosts = []string{"localhost", "127.0.0.1", "[::1]", "0.0.0.0", "[::]"}

// PortPrinted reports whether the hub's retained output holds a loopback http
// URL for port. A search already running on this hub answers false: a
// write-cap holder cannot stall emuMu with back-to-back snapshots.
func (h *Hub) PortPrinted(port int) bool {
	if port < 1 || port > 65535 {
		return false
	}
	if !h.portPrintedBusy.CompareAndSwap(false, true) {
		return false
	}
	defer h.portPrintedBusy.Store(false)
	snap, ok := h.pathSnapshot()
	if !ok {
		return false
	}
	// The longest needle is the 127.0.0.1 one.
	needleLen := len("http://127.0.0.1:") + len(strconv.Itoa(port))
	_, found := snap.findMatch(func(run string) bool { return hasURLPort(run, port) }, needleLen)
	return found
}

// hasURLPort reports whether line holds http://<loopback host>:<port> as a
// whole URL token. Before the needle: start of line, whitespace or one of
// ( [ { < " ' , = ;. After it: end of line, whitespace, one of
// / ? # ) ] } > " ' , ; or a full stop followed by end of line or a
// non-alphanumeric. Anything else (a digit, a letter, @ : - _ % ~ +) refuses,
// so :80 never matches inside :8000 and localhost:8000@evil never matches.
// Case-sensitive: printed text must be canonical lower case.
func hasURLPort(line string, port int) bool {
	p := strconv.Itoa(port)
	for _, host := range loopbackHosts {
		needle := "http://" + host + ":" + p
		for off := 0; off <= len(line)-len(needle); {
			i := strings.Index(line[off:], needle)
			if i < 0 {
				break
			}
			i += off
			if urlBefore(line[:i]) && urlAfter(line[i+len(needle):]) {
				return true
			}
			off = i + 1
		}
	}
	return false
}

func urlBefore(before string) bool {
	if before == "" {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(before)
	return unicode.IsSpace(r) || strings.ContainsRune(`([{<"',=;`, r)
}

func urlAfter(rest string) bool {
	if rest == "" {
		return true
	}
	r, size := utf8.DecodeRuneInString(rest)
	if unicode.IsSpace(r) || strings.ContainsRune(`/?#)]}>"',;`, r) {
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
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
