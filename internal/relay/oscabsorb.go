package relay

import (
	"bytes"
	"log/slog"
	"strings"
)

// Phase 111 / Issue #54: streaming state machine that absorbs xterm.js's
// SYNTHESIZED replies to terminal queries (OSC 10/11 color queries and DA1
// device-attributes queries) emitted by the browser, BEFORE they reach the
// PTY's stdin.
//
// Three envelope shapes are absorbed:
//
//	OSC 10 FG color reply : "\x1b]10;rgb:RRRR/GGGG/BBBB" + (ST | BEL)
//	OSC 11 BG color reply : "\x1b]11;rgb:RRRR/GGGG/BBBB" + (ST | BEL)
//	DA1 reply             : "\x1b[?<digits-and-semicolons>c"
//
//	where ST = "\x1b\\" and BEL = "\x07".
//
// All other input (user keystrokes, arrow keys, function keys, OSC 52 clipboard,
// OSC 8 hyperlinks, bracketed-paste markers, …) is forwarded byte-for-byte.
//
// The absorber is per-WebSocket-subscriber. It MUST persist across MsgInput
// frames because envelopes may split across frames. The zero-value
// (&InputAbsorber{}) is the legitimate "outside, empty buffers" starting state
// — do not add a constructor.

const maxEnvelopeBytes = 4096

type absorbState int

const (
	stateOutside absorbState = iota
	stateGotEsc
	stateInOSC
	stateInOSCSeenEsc
	stateInCSI
)

// InputAbsorber is a streaming state machine that filters OSC 10/11 and DA1
// replies out of the browser→PTY input stream.
//
// The Filter method is the only public surface. Single-goroutine use only
// (the WS read pump goroutine).
type InputAbsorber struct {
	state  absorbState
	oscBuf []byte // body of an in-progress OSC envelope (after "\x1b]", excludes terminator)
	csiBuf []byte // body of an in-progress CSI envelope (after "\x1b[", includes the final byte once seen)
}

// Filter consumes in and returns the bytes that should be forwarded to the PTY.
// An empty (but non-nil) slice means "all input was absorbed; write nothing."
//
// A bare ESC at the end of a frame is forwarded at once. It used to be held
// until the next frame, so a lone Esc (the key bar's Esc, Claude Code's "Esc to
// interrupt", leaving vim's Insert mode) did nothing until the next key. xterm.js
// sends every key and every reply to a query in one onData call, so one
// MsgInput frame never ends between the ESC and the introducer of a reply (only
// encodeInputFrames' 32767-byte chunking splits a frame, and only pastes get
// that large). The one accepted cost: a raw terminal behind `portty attach` that
// split a reply exactly between ESC and ']' (or '[') now leaks the rest of the
// reply to the PTY as text; a split after the introducer is still absorbed
// (stateInOSC and stateInCSI persist across frames).
func (a *InputAbsorber) Filter(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for _, b := range in {
		switch a.state {
		case stateOutside:
			if b == 0x1b { // ESC
				a.state = stateGotEsc
			} else {
				out = append(out, b)
			}

		case stateGotEsc:
			switch b {
			case ']':
				a.state = stateInOSC
				a.oscBuf = a.oscBuf[:0]
			case '[':
				a.state = stateInCSI
				a.csiBuf = a.csiBuf[:0]
			default:
				// Bare ESC followed by ordinary byte (Alt-key combo, vim cmd-mode,
				// or anything that isn't an OSC/CSI introducer). Flush both bytes
				// as passthrough.
				out = append(out, 0x1b, b)
				a.state = stateOutside
			}

		case stateInOSC:
			switch b {
			case 0x07: // BEL terminator
				out = a.completeOSC(out)
				a.state = stateOutside
			case 0x1b: // possible ST terminator (next byte must be '\')
				a.state = stateInOSCSeenEsc
			default:
				if len(a.oscBuf) >= maxEnvelopeBytes {
					// Overflow guard: flush conservatively as passthrough; do not
					// silently drop bytes. Reset state and re-process this byte
					// from stateOutside (CLAUDE.md §"Silent fallbacks").
					out = append(out, 0x1b, ']')
					out = append(out, a.oscBuf...)
					out = append(out, b)
					a.oscBuf = a.oscBuf[:0]
					a.state = stateOutside
				} else {
					a.oscBuf = append(a.oscBuf, b)
				}
			}

		case stateInOSCSeenEsc:
			if b == '\\' {
				// Valid ST terminator — envelope complete.
				out = a.completeOSC(out)
				a.state = stateOutside
			} else {
				// Inner ESC was NOT part of a valid ST terminator. Flush the
				// in-progress OSC as passthrough, then re-process the ESC plus
				// the current byte as a new sequence.
				out = append(out, 0x1b, ']')
				out = append(out, a.oscBuf...)
				out = append(out, 0x1b, b)
				a.oscBuf = a.oscBuf[:0]
				a.state = stateOutside
			}

		case stateInCSI:
			if len(a.csiBuf) >= maxEnvelopeBytes {
				// Overflow guard — same conservative behavior as OSC overflow.
				out = append(out, 0x1b, '[')
				out = append(out, a.csiBuf...)
				out = append(out, b)
				a.csiBuf = a.csiBuf[:0]
				a.state = stateOutside
				break
			}
			a.csiBuf = append(a.csiBuf, b)
			// CSI final byte is the first byte in the range 0x40–0x7E that is
			// NOT a parameter-byte ('0'–'9', ';', ':') and NOT an intermediate
			// byte (0x20–0x2F). The simplest robust check: any byte ≥0x40 that
			// is not the leading '?' / '>' / '<' / '=' private-marker (which
			// only appears as the first byte) terminates the CSI sequence.
			if b >= 0x40 && b <= 0x7E {
				out = a.completeCSI(out)
				a.csiBuf = a.csiBuf[:0]
				a.state = stateOutside
			}
		}
	}
	if a.state == stateGotEsc {
		out = append(out, 0x1b)
		a.state = stateOutside
	}
	return out
}

// completeOSC decides whether the just-finished OSC envelope should be absorbed
// (OSC 10 or OSC 11 color reply) or flushed as passthrough (OSC 52, OSC 8,
// anything else). Reconstructs with the BEL terminator on passthrough — both
// terminators are accepted by OSC consumers, so the canonicalization is safe.
func (a *InputAbsorber) completeOSC(out []byte) []byte {
	code, _, _ := bytes.Cut(a.oscBuf, []byte{';'})
	codeStr := string(code)
	if codeStr == "10" || codeStr == "11" {
		slog.Debug("oscabsorb: absorbed OSC envelope",
			slog.String("code", codeStr),
			slog.Int("len_envelope", len(a.oscBuf)+3)) // +"\x1b]" + terminator
		a.oscBuf = a.oscBuf[:0]
		return out
	}
	out = append(out, 0x1b, ']')
	out = append(out, a.oscBuf...)
	out = append(out, 0x07)
	a.oscBuf = a.oscBuf[:0]
	return out
}

// completeCSI decides whether the just-finished CSI envelope is a DA1 reply
// ("\x1b[?...c") and should be absorbed, or anything else (arrow keys, function
// keys, bracketed-paste markers, …) and should be flushed as passthrough.
// csiBuf includes the final byte (the byte that terminated the sequence).
func (a *InputAbsorber) completeCSI(out []byte) []byte {
	n := len(a.csiBuf)
	if n >= 2 && a.csiBuf[0] == '?' && a.csiBuf[n-1] == 'c' {
		slog.Debug("oscabsorb: absorbed DA1 envelope",
			slog.String("code", "DA1"),
			slog.Int("len_envelope", n+2)) // +"\x1b["
		return out
	}
	out = append(out, 0x1b, '[')
	out = append(out, a.csiBuf...)
	return out
}

// IsUserInput reports whether filtered (the bytes InputAbsorber.Filter
// returned) holds any byte a person typed, so the viewer that sent it claims
// the grid (INPT-06 typing). Terminals answer queries on their own, on every
// connected viewer, and those answers must never claim, or idle viewers and
// reconnects would fight over the grid. A byte is not user input when it
// belongs to:
//   - a CSI sequence with a private marker (< = > ?) or an intermediate byte
//     (0x20-0x2F), or whose final byte is one of R n c t y I O M m (cursor
//     and status reports, DA, window reports, DECRPM, focus, mouse);
//   - an X10 mouse report, ESC [ M plus three raw bytes;
//   - an OSC, DCS, APC, PM or SOS string, up to BEL or ST;
//   - an unterminated sequence at the end of the frame.
//
// Everything else is user input: text, C0 controls, DEL, ESC + character
// (Alt keys), SS3 keys, and CSI keys with no marker (arrows, Home/End,
// function keys, bracketed paste markers, kitty CSI code;mods u). So a paste
// claims and the kitty flags report ESC [ ? 0 u does not. Conservative on
// purpose: a missed claim costs one more keystroke, a false one makes viewers
// fight over the grid. Known collision: a modified F3 (CSI 1;5R) reads as a
// cursor report and does not claim. filtered is never modified.
//
// A lone trailing ESC is not user input, even though Filter now forwards it at
// once: an Esc key alone does not claim the grid (INPT-06, conservative).
//
// shortcut: stateless per frame, so a reply split across two frames can claim
// once from its tail; xterm.js sends each reply in one onData call, so only a
// raw terminal behind portty attach could split one. Keep state per
// connection (like InputAbsorber) if the VM check ever sees that.
func IsUserInput(filtered []byte) bool {
	b := filtered
	for i := 0; i < len(b); {
		if b[i] != 0x1b {
			return true
		}
		if i+1 >= len(b) {
			return false // a lone trailing ESC: unterminated
		}
		switch b[i+1] {
		case '[':
			end, user := scanCSI(b, i+2)
			if user {
				return true
			}
			i = end
		case ']', 'P', '_', '^', 'X':
			i = scanString(b, i+2)
		case 'O':
			return i+2 < len(b) // an SS3 key (F1-F4, keypad); unterminated at the end
		default:
			return true // ESC + character: an Alt key
		}
	}
	return false
}

// scanCSI scans the CSI sequence whose parameters start at b[j] and returns
// the index after it and whether it is a user key. A C0 byte, ESC or a byte
// past 0x7E before the final byte aborts the sequence: scanning resumes there.
func scanCSI(b []byte, j int) (end int, user bool) {
	if j < len(b) && b[j] == 'M' {
		return min(j+4, len(b)), false // X10 mouse: M plus three raw bytes
	}
	marker := false
	for ; j < len(b); j++ {
		c := b[j]
		switch {
		case c >= 0x30 && c <= 0x3F: // parameter bytes; < = > ? are private markers
			marker = marker || c >= '<'
		case c >= 0x20 && c <= 0x2F: // intermediate bytes ($ in DECRPM)
			marker = true
		case c >= 0x40 && c <= 0x7E: // final byte
			return j + 1, !marker && !strings.ContainsRune("RnctyIOMm", rune(c))
		default:
			return j, false
		}
	}
	return j, false // unterminated
}

// scanString returns the index after the OSC, DCS, APC, PM or SOS string
// whose body starts at b[j]: after BEL or ST, at an ESC that starts anything
// else (it aborts the string), or at the end of b.
func scanString(b []byte, j int) int {
	for ; j < len(b); j++ {
		if b[j] == 0x07 {
			return j + 1
		}
		if b[j] == 0x1b && j+1 < len(b) {
			if b[j+1] == '\\' {
				return j + 2
			}
			return j
		}
	}
	return j
}
