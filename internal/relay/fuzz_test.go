package relay

// Security fuzz harness (STRP-04) for the relay's untrusted-input surfaces:
// WebSocket frame parsing and the browser→PTY input absorber. Run one target
// at a time, e.g.
//
//	go test -run='^$' -fuzz='^FuzzParseFrame$' -fuzztime=10s ./internal/relay

import (
	"bytes"
	"testing"
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
