package relay

import (
	"bytes"
	"testing"
)

// addedQueries are the reply-eliciting sequences HARD-03 added to the strip.
var addedQueries = []struct{ name, seq string }{
	{"CSI 14 t text area pixels", "\x1b[14t"},
	{"CSI 14;2 t text area pixels", "\x1b[14;2t"},
	{"CSI 16 t cell pixels", "\x1b[16t"},
	{"CSI 18 t text area chars", "\x1b[18t"},
	{"DA3", "\x1b[=c"},
	{"DA3 with param", "\x1b[=0c"},
	{"XTGETTCAP one name", "\x1bP+q544e\x1b\\"},
	{"XTGETTCAP two names", "\x1bP+q544e;636f\x1b\\"},
	{"DECRQSS SGR", "\x1bP$qm\x1b\\"},
	{"DECRQSS cursor style", "\x1bP$q q\x1b\\"},
	{"OSC 4 query BEL", "\x1b]4;1;?\x07"},
	{"OSC 4 two queries", "\x1b]4;1;?;2;?\x07"},
	{"OSC 4 query ST", "\x1b]4;15;?\x1b\\"},
	{"OSC 52 read BEL", "\x1b]52;c;?\x07"},
	{"OSC 52 read ST empty selection", "\x1b]52;;?\x1b\\"},
}

func TestStripTerminalQueries(t *testing.T) {
	regression := []struct{ name, seq string }{
		{"DA1", "\x1b[c"},
		{"DA2", "\x1b[>c"},
		{"DSR 6n", "\x1b[6n"},
		{"DECXCPR", "\x1b[?6n"},
		{"DECRQM ANSI", "\x1b[4$p"},
		{"DECRQM DEC", "\x1b[?2026$p"},
		{"in-band resize set", "\x1b[?2048h"},
		{"in-band resize reset", "\x1b[?2048l"},
		{"XTVERSION", "\x1b[>0q"},
		{"kitty keyboard query", "\x1b[?u"},
		{"OSC 10 query", "\x1b]10;?\x07"},
		{"OSC 11 query", "\x1b]11;?\x1b\\"},
		{"OSC 12 query", "\x1b]12;?\x07"},
		// IN-04 (07 review fix): multi-queries and the other XTWINOPS reports.
		{"OSC 10 two queries", "\x1b]10;?;?\x07"},
		{"OSC 11 three queries ST", "\x1b]11;?;?;?\x1b\\"},
		{"CSI 13 t window position", "\x1b[13t"},
		{"CSI 13;2 t text area position", "\x1b[13;2t"},
		{"CSI 19 t screen size chars", "\x1b[19t"},
		{"CSI 21 t title report", "\x1b[21t"},
	}
	for _, c := range append(append([]struct{ name, seq string }{}, addedQueries...), regression...) {
		t.Run("strip/"+c.name, func(t *testing.T) {
			if got := StripTerminalQueries([]byte("a" + c.seq + "b")); string(got) != "ab" {
				t.Fatalf("StripTerminalQueries(%q) = %q, want %q", "a"+c.seq+"b", got, "ab")
			}
		})
	}

	keep := []struct{ name, seq string }{
		{"window resize set", "\x1b[8;24;80t"},
		{"title stack push", "\x1b[22;0t"},
		{"title stack pop", "\x1b[23;0t"},
		{"OSC 10 colour set", "\x1b]10;rgb:ff/ff/ff\x07"},
		{"OSC 4 palette set", "\x1b]4;1;rgb:ff/00/00\x07"},
		{"sixel DCS", "\x1bPq#0;2;0;0;0~-\x1b\\"},
		{"SGR", "\x1b[31m"},
		{"bracketed paste", "\x1b[?2004h"},
		{"alt screen", "\x1b[?1049h"},
		{"plain text", "hello, world"},
	}
	for _, c := range keep {
		t.Run("keep/"+c.name, func(t *testing.T) {
			in := "a" + c.seq + "b"
			if got := StripTerminalQueries([]byte(in)); string(got) != in {
				t.Fatalf("StripTerminalQueries(%q) = %q, want it unchanged", in, got)
			}
		})
	}
}

// TestJoin_StripsAddedQueries: full (intact raw ring) and delta replays drop
// the HARD-03 sequences, so a joining terminal answers none of them.
func TestJoin_StripsAddedQueries(t *testing.T) {
	var payload, want []byte
	for i, q := range addedQueries {
		text := "line " + string(rune('A'+i)) + "\r\n"
		payload = append(payload, text+q.seq...)
		want = append(want, text...)
	}

	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	emit(t, hub, w, string(payload))
	if got := fullReplay(t, hub); !bytes.Equal(got, MakeOutputFrame(want)) {
		t.Errorf("full replay = %q; want %q", got, MakeOutputFrame(want))
	}

	off := join(t, hub, 0, false).End
	emit(t, hub, w, string(payload))
	r := join(t, hub, off, true)
	if got := payloads(r); r.Mode != ReplayDelta || !bytes.Equal(got, MakeOutputFrame(want)) {
		t.Errorf("mode %q delta = %q; want delta %q", r.Mode, got, MakeOutputFrame(want))
	}
}
