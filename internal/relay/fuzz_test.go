package relay

// Security fuzz harness (STRP-04) for the relay's untrusted-input surfaces:
// WebSocket frame parsing and the browser→PTY input absorber. Run one target
// at a time, e.g.
//
//	go test -run='^$' -fuzz='^FuzzParseFrame$' -fuzztime=10s ./internal/relay

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzParseFrame(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{MsgInput})
	f.Add(append([]byte{0x35}, `{"text":"evil command"}`...)) // retired inject frame
	f.Add(append([]byte{MsgOutput}, "hello\r\n"...))
	f.Add(append([]byte{0x31}, `{"content":"hi"}`...)) // retired chat-send frame

	f.Fuzz(func(t *testing.T, frame []byte) {
		msgType, payload, err := ParseFrame(frame)
		if len(frame) == 0 {
			if err == nil {
				t.Fatal("empty frame: want error, got nil")
			}
			return
		}
		if err != nil {
			t.Fatalf("non-empty frame: unexpected error %v", err)
		}
		if msgType != frame[0] {
			t.Fatalf("msgType = 0x%02x, want 0x%02x", msgType, frame[0])
		}
		if len(payload) != len(frame)-1 {
			t.Fatalf("len(payload) = %d, want %d", len(payload), len(frame)-1)
		}
	})
}

func FuzzInputAbsorberFilter(f *testing.F) {
	f.Add([]byte("\x1b]11;rgb:0000/0000/0000\x1b\\"), uint(5))
	f.Add([]byte("\x1b[?62;22c"), uint(3))
	f.Add([]byte("plain text ls -la\r"), uint(4))
	f.Add([]byte("\x1b]10;rgb:ffff/ffff/ffff\x07after"), uint(1)) // split right after ESC
	f.Add([]byte("\x1b[A\x1bb\x1b]52;c;aGk=\x07"), uint(7))

	f.Fuzz(func(t *testing.T, in []byte, split uint) {
		k := int(split % uint(len(in)+1))
		var a InputAbsorber // zero value is the valid start state
		out := append(a.Filter(in[:k]), a.Filter(in[k:])...)
		// The absorber only drops or re-emits buffered input; it never adds bytes.
		if len(out) > len(in) {
			t.Fatalf("output %d bytes > input %d bytes (in=%q split=%d out=%q)", len(out), len(in), in, k, out)
		}
		if !bytes.Contains(in, []byte{0x1b}) {
			var fresh InputAbsorber
			if got := fresh.Filter(in); !bytes.Equal(got, in) {
				t.Fatalf("ESC-free input changed: in=%q out=%q", in, got)
			}
		}
	})
}

// FuzzCharState (08-REVIEW IN-08): the SO/SI scan, the OSC 8 tracker and
// fixRenderedLinks on arbitrary PTY bytes. The scan carries its state across
// reads (any split gives the one-read state), the replay suffix holds only
// ESC, BEL, SO/SI and printable valid UTF-8 (no C1 code point a viewer would
// run), and every OSC 8 a replay or fixRenderedLinks emits has safe fields.
// The normal gate runs only the seeds below; fuzz with
//
//	go test -run='^$' -fuzz='^FuzzCharState$' -fuzztime=30s ./internal/relay
func FuzzCharState(f *testing.F) {
	for _, s := range []string{
		"plain text\r\n",
		"\x1b)0\x0eqqq\x0fabc",
		"\x1b(0lqk\x1b(B",
		"\x0e\x1b]0;title\x1bc",               // P4
		"\x0e\x1bP$qm\x1bc",                   // RIS after a DCS
		"\xe2\x9d\x0e",                        // SO ends a cut-off UTF-8 character
		"\x1b]0;t\x1b\x0eabc",                 // Q5: SO right after a string-ending ESC
		"\x0e\x1bP$q\x1b\x0f",                 // Q5: SI after a DCS
		"\x1b]8;;http://x/\u009b2J\x07$ ",     // P1
		"\x1b]8;;http://x/\u0090$qm\u009cabc", // DCS variant
		"\x1b]8;id=1;https://example.invalid/\x07LINK\x1b]8;;\x07",
		"\x1b]8;;http://x/" + strings.Repeat("a", 3000) + "\x07$ ", // over the field cap
		"\x9d8;;http://x/\x07eight-bit OSC\x9c",
		"\x1b]8;;http://x/\x07\x1b[?1049hALT\x1b[?1049l$ ", // link across an alt-screen visit
		"\x1b]8;;http://x/\x1b\\\x1b7\x1b]8;;\x1b\\\x1b8x",
	} {
		f.Add([]byte(s), uint(len(s)/2))
	}
	f.Fuzz(func(t *testing.T, in []byte, split uint) {
		if len(in) > 1<<16 {
			return
		}
		k := int(split % uint(len(in)+1))
		var one, two charState
		one.scan(in)
		two.scan(in[:k])
		two.scan(in[k:])
		if one != two {
			t.Fatalf("scan state depends on the read split at %d: %+v vs %+v", k, one, two)
		}

		h := NewHub("fuzz", nil, nil, DefaultScrollbackBytes, nil)
		probeFeed(h, string(in))
		h.emuMu.Lock()
		suffix, out := h.cs.suffix(), h.renderScreenLocked()
		h.emuMu.Unlock()
		if !utf8.ValidString(suffix) {
			t.Fatalf("suffix is not valid UTF-8: %q", suffix)
		}
		for _, r := range suffix {
			if (r < 0x20 && r != 0x1b && r != 0x07 && r != 0x0e && r != 0x0f) || (r >= 0x7f && r <= 0x9f) {
				t.Fatalf("suffix holds control %U: %q", r, suffix)
			}
		}
		for _, s := range []string{string(out), fixRenderedLinks(string(in))} {
			for _, m := range renderedLinkRE.FindAllStringSubmatch(s, -1) {
				if !safeLinkField(m[1]) || !safeLinkField(m[2]) {
					t.Fatalf("unsafe OSC 8 emitted: %.80q", m[0])
				}
			}
		}
	})
}
