package desktop_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// fakePasteboard is the Mac's clipboard for a test. It only has ReadText and
// WriteText: there is no way to type into an app through it.
type fakePasteboard struct {
	mu      sync.Mutex
	text    string
	outcome screencap.ReadOutcome
	err     error
	block   chan struct{} // a read waits for this to close
	reads   int
	written []string
}

func (p *fakePasteboard) ReadText(int) (string, screencap.ReadOutcome, error) {
	p.mu.Lock()
	p.reads++
	text, outcome, err, block := p.text, p.outcome, p.err, p.block
	p.mu.Unlock()
	if block != nil {
		<-block
	}
	return text, outcome, err
}

func (p *fakePasteboard) WriteText(s string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.written = append(p.written, s)
	return p.err
}

func (p *fakePasteboard) readCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reads
}

func (p *fakePasteboard) writes() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.written...)
}

// withPasteboard wires the fake pasteboard and a Mac that is not locked.
func withPasteboard(p *fakePasteboard, locked *bool) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) {
		d.Pasteboard = p
		d.Lock = func() (bool, error) { return locked != nil && *locked, nil }
	}
}

// clipResult waits for the clip_result of a transfer and returns its outcome.
func (v *surfViewer) clipResult(t *testing.T, xid string) string {
	t.Helper()
	m := v.wait(t, "clip_result", func(m map[string]any) bool { return m["xid"] == xid })
	return m["outcome"].(string)
}

// clipDatas returns the clip_data messages of a transfer in the order they arrived.
func (v *surfViewer) clipDatas(xid string) []map[string]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []map[string]any
	for _, m := range v.msgs {
		if m["type"] == "clip_data" && m["xid"] == xid {
			out = append(out, m)
		}
	}
	return out
}

func (v *surfViewer) messageBytes() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	b, _ := json.Marshal(v.msgs)
	return string(b)
}

// TestClipboardTracer_CopyFromMac: a viewer's clip_get returns the Mac's text as
// clip_data chunks and a clip_result ok; a long text arrives in frames under the
// socket's read limit; a concealed item returns only its outcome.
func TestClipboardTracer_CopyFromMac(t *testing.T) {
	pb := &fakePasteboard{text: "hello"}
	env := newTracerServer(t, withPasteboard(pb, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinSurf(ctx, t, env.token(t, "g1"))

	v.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "c1"})
	if got := v.clipResult(t, "c1"); got != "ok" {
		t.Fatalf("clip_result = %q, want ok", got)
	}
	d := v.clipDatas("c1")
	if len(d) != 1 || d[0]["seq"] != float64(0) || d[0]["total"] != float64(1) || d[0]["len"] != float64(5) || d[0]["text"] != "hello" {
		t.Fatalf("clip_data = %v, want one chunk {seq 0, total 1, len 5, text hello}", d)
	}

	// 40000 bytes: at most 16384 per chunk, so 3, in order, and rejoined they are the text.
	pb.mu.Lock()
	pb.text = strings.Repeat("0123456789", 4000)
	pb.mu.Unlock()
	v.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "c2"})
	if got := v.clipResult(t, "c2"); got != "ok" {
		t.Fatalf("long clip_result = %q, want ok", got)
	}
	var joined strings.Builder
	d = v.clipDatas("c2")
	if len(d) != 3 {
		t.Fatalf("%d chunks for 40000 bytes, want 3", len(d))
	}
	for i, c := range d {
		b, _ := json.Marshal(c)
		if len(b) > 30000 {
			t.Errorf("chunk %d frame is %d bytes, want at most 30000", i, len(b))
		}
		if c["seq"] != float64(i) || c["total"] != float64(3) || c["len"] != float64(40000) {
			t.Errorf("chunk %d header = %v", i, c)
		}
		joined.WriteString(c["text"].(string))
	}
	if joined.String() != pb.text {
		t.Errorf("chunks rejoin to %d bytes that differ from the text", joined.Len())
	}

	// A concealed item: only the outcome travels, never the content.
	pb.mu.Lock()
	pb.text, pb.outcome = "s3cret-value", screencap.ReadConcealed
	pb.mu.Unlock()
	v.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "c3"})
	if got := v.clipResult(t, "c3"); got != "concealed" {
		t.Fatalf("clip_result = %q, want concealed", got)
	}
	if n := len(v.clipDatas("c3")); n != 0 {
		t.Errorf("%d clip_data for a concealed item, want none", n)
	}
	if strings.Contains(v.messageBytes(), "s3cret-value") {
		t.Error("concealed content reached the viewer")
	}
}

// clipFrame is the encoded clip_data frame for a chunk, with the longest
// transfer ID made of the bytes that escape the most and the largest numbers.
func clipFrame(text string) int {
	b, _ := json.Marshal(map[string]any{
		"type": "clip_data", "xid": strings.Repeat("\x00", 64), "seq": 1 << 20, "total": 1 << 20, "len": 1 << 20, "text": text,
	})
	return len(b)
}

func TestClipboard_ChunkBoundaries(t *testing.T) {
	check := func(name, text string, wantChunks int) {
		t.Helper()
		chunks := desktop.ChunkTextForTest(text)
		if wantChunks > 0 && len(chunks) != wantChunks {
			t.Errorf("%s: %d chunks, want %d", name, len(chunks), wantChunks)
		}
		var joined strings.Builder
		for i, c := range chunks {
			if len(c) > 16384 {
				t.Errorf("%s: chunk %d is %d bytes, want at most 16384", name, i, len(c))
			}
			if !utf8.ValidString(c) {
				t.Errorf("%s: chunk %d is cut inside a character", name, i)
			}
			if n := clipFrame(c); n > 30000 {
				t.Errorf("%s: chunk %d frame is %d bytes, want at most 30000", name, i, n)
			}
			joined.WriteString(c)
		}
		if joined.String() != text {
			t.Errorf("%s: chunks do not rejoin to the text", name)
		}
	}
	check("16384 ascii", strings.Repeat("a", 16384), 1)
	check("16385 ascii", strings.Repeat("a", 16385), 2)
	check("e acute at the cut", strings.Repeat("a", 16383)+"é", 2)
	check("double quotes", strings.Repeat(`"`, 20000), 0)
	check("html and control", strings.Repeat("<&\x01\n", 9000), 0)
	check("four byte characters", strings.Repeat("\U0001F600", 9000), 0)
}

func TestClipboard_CopyOutcomes(t *testing.T) {
	pb := &fakePasteboard{}
	env := newTracerServer(t, withPasteboard(pb, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinSurf(ctx, t, env.token(t, "g1"))

	set := func(text string, outcome screencap.ReadOutcome, err error) {
		pb.mu.Lock()
		pb.text, pb.outcome, pb.err = text, outcome, err
		pb.mu.Unlock()
	}
	get := func(xid string) string {
		v.ask(ctx, t, map[string]any{"type": "clip_get", "xid": xid})
		return v.clipResult(t, xid)
	}
	set("", screencap.ReadEmpty, nil)
	if got := get("e1"); got != "empty" {
		t.Errorf("empty pasteboard = %q, want empty", got)
	}
	set("", screencap.ReadTooLarge, nil)
	if got := get("t1"); got != "too_large" {
		t.Errorf("too large = %q, want too_large", got)
	}
	set(strings.Repeat("x", 1<<20+1), screencap.ReadOK, nil)
	if got := get("t2"); got != "too_large" {
		t.Errorf("1 MiB + 1 byte from a bridge that did not check = %q, want too_large", got)
	}
	set(strings.Repeat("x", 1<<20), screencap.ReadOK, nil)
	if got := get("m1"); got != "ok" {
		t.Fatalf("exactly 1 MiB = %q, want ok", got)
	}
	if n := len(v.clipDatas("m1")); n != 64 {
		t.Errorf("1 MiB came in %d chunks, want 64", n)
	}
	set("", screencap.ReadDenied, nil)
	if got := get("d1"); got != "waiting_approval" {
		t.Errorf("denied = %q, want waiting_approval", got)
	}
	set("", screencap.ReadEmpty, errors.New("boom"))
	if got := get("f1"); got != "failed" {
		t.Errorf("read error = %q, want failed", got)
	}
	// A transfer ID over 64 bytes is refused with no ID to echo.
	v.ask(ctx, t, map[string]any{"type": "clip_get", "xid": strings.Repeat("x", 65)})
	if got := v.clipResult(t, ""); got != "failed" {
		t.Errorf("transfer ID over 64 bytes = %q, want failed", got)
	}
}

// TestClipboard_WaitingApproval: a read that never returns (a privacy prompt
// nobody can answer) answers waiting_approval after 2 s, every other copy
// answers at once with no second read, and the next copy reads again once the
// first read has returned.
func TestClipboard_WaitingApproval(t *testing.T) {
	pb := &fakePasteboard{text: "hi", block: make(chan struct{})}
	env := newTracerServer(t, withPasteboard(pb, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)

	start := time.Now()
	a.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "w1"})
	if got := a.clipResult(t, "w1"); got != "waiting_approval" {
		t.Fatalf("blocked read = %q, want waiting_approval", got)
	}
	if d := time.Since(start); d < 1900*time.Millisecond || d > 4*time.Second {
		t.Errorf("answered after %v, want about 2 s", d)
	}
	start = time.Now()
	b.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "w2"})
	if got := b.clipResult(t, "w2"); got != "waiting_approval" {
		t.Errorf("second viewer = %q, want waiting_approval", got)
	}
	a.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "w3"})
	if got := a.clipResult(t, "w3"); got != "waiting_approval" {
		t.Errorf("same viewer again = %q, want waiting_approval", got)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("copies during a pending read took %v, want at once", d)
	}
	if n := pb.readCount(); n != 1 {
		t.Errorf("%d reads started, want 1", n)
	}

	close(pb.block)
	n := 0
	eventually(t, 10*time.Second, "a copy after the read returned", func() bool {
		n++
		xid := "r" + strconv.Itoa(n)
		a.ask(ctx, t, map[string]any{"type": "clip_get", "xid": xid})
		return a.clipResult(t, xid) == "ok"
	})
	if n := pb.readCount(); n < 2 {
		t.Errorf("%d reads, want the next copy to read again", n)
	}
}

func put(ctx context.Context, t *testing.T, v *surfViewer, xid string, seq, total, size int, text string) {
	t.Helper()
	v.ask(ctx, t, map[string]any{"type": "clip_put", "xid": xid, "seq": seq, "total": total, "len": size, "text": text})
}

func TestClipboard_PutReassembly(t *testing.T) {
	pb := &fakePasteboard{}
	env := newTracerServer(t, withPasteboard(pb, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinSurf(ctx, t, env.token(t, "g1"))

	put(ctx, t, v, "p1", 0, 2, 7, "hel")
	if n := len(pb.writes()); n != 0 {
		t.Fatalf("%d writes after the first chunk, want 0", n)
	}
	put(ctx, t, v, "p1", 1, 2, 7, "lo, ")
	if got := v.clipResult(t, "p1"); got != "ok" {
		t.Fatalf("paste = %q, want ok", got)
	}
	if w := pb.writes(); len(w) != 1 || w[0] != "hello, " {
		t.Errorf("pasteboard writes = %q, want one write of %q", w, "hello, ")
	}
	if n := env.mgr.ClipPutsForTest(); n != 0 {
		t.Errorf("%d transfers left after a completed paste, want 0", n)
	}

	// The most a paste may hold: exactly 1 MiB in 64 chunks.
	for i := range 64 {
		put(ctx, t, v, "big", i, 64, 1<<20, strings.Repeat("y", 16384))
	}
	if got := v.clipResult(t, "big"); got != "ok" {
		t.Fatalf("1 MiB paste = %q, want ok", got)
	}
	if w := pb.writes(); len(w) != 2 || len(w[1]) != 1<<20 {
		t.Errorf("writes = %d, want a second of 1048576 bytes", len(w))
	}
}

func TestClipboard_PutRejects(t *testing.T) {
	pb := &fakePasteboard{}
	env := newTracerServer(t, withPasteboard(pb, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinSurf(ctx, t, env.token(t, "g1"))

	cases := []struct {
		name string
		send func(xid string)
		want string
	}{
		{"seq 1 before seq 0", func(x string) { put(ctx, t, v, x, 1, 2, 4, "ab") }, "failed"},
		{"duplicate seq", func(x string) {
			put(ctx, t, v, x, 0, 2, 4, "ab")
			put(ctx, t, v, x, 0, 2, 4, "ab")
		}, "failed"},
		{"gap in seq", func(x string) {
			put(ctx, t, v, x, 0, 3, 6, "ab")
			put(ctx, t, v, x, 2, 3, 6, "ab")
		}, "failed"},
		{"changed len", func(x string) {
			put(ctx, t, v, x, 0, 2, 4, "ab")
			put(ctx, t, v, x, 1, 2, 5, "cd")
		}, "failed"},
		{"changed total", func(x string) {
			put(ctx, t, v, x, 0, 2, 4, "ab")
			put(ctx, t, v, x, 1, 3, 4, "cd")
		}, "failed"},
		{"changed xid", func(x string) {
			put(ctx, t, v, x, 0, 2, 4, "ab")
			put(ctx, t, v, x+"z", 1, 2, 4, "cd")
		}, "failed"},
		{"chunk over 16384 bytes", func(x string) { put(ctx, t, v, x, 0, 1, 16385, strings.Repeat("a", 16385)) }, "failed"},
		{"total below 1", func(x string) { put(ctx, t, v, x, 0, 0, 4, "ab") }, "failed"},
		{"chunks past len", func(x string) {
			put(ctx, t, v, x, 0, 2, 3, "ab")
			put(ctx, t, v, x, 1, 2, 3, "cd")
		}, "failed"},
		{"final length short of len", func(x string) {
			put(ctx, t, v, x, 0, 2, 7, "hel")
			put(ctx, t, v, x, 1, 2, 7, "lo")
		}, "failed"},
		{"invalid utf-8", func(x string) {
			raw := `{"type":"clip_put","xid":"` + x + `","seq":0,"total":1,"len":1,"text":"` + "\xff" + `"}`
			if err := v.conn.Write(ctx, websocket.MessageText, []byte(raw)); err != nil {
				t.Fatal(err)
			}
		}, "failed"},
		{"len 0", func(x string) { put(ctx, t, v, x, 0, 1, 0, "") }, "empty"},
		{"len over 1 MiB", func(x string) { put(ctx, t, v, x, 0, 65, 1<<20+1, "ab") }, "too_large"},
	}
	for i, c := range cases {
		xid := "x" + strconv.Itoa(i)
		c.send(xid)
		resp := xid
		if c.name == "changed xid" {
			resp = xid + "z"
		}
		if got := v.clipResult(t, resp); got != c.want {
			t.Errorf("%s: answered %q, want %q", c.name, got, c.want)
		}
		if n := env.mgr.ClipPutsForTest(); n != 0 {
			t.Errorf("%s: %d transfers left behind, want 0", c.name, n)
		}
	}
	if w := pb.writes(); len(w) != 0 {
		t.Errorf("%d writes from rejected pastes, want none", len(w))
	}

	// A failure leaves no state: the next paste is clean.
	put(ctx, t, v, "good", 0, 1, 2, "ok")
	if got := v.clipResult(t, "good"); got != "ok" {
		t.Errorf("paste after failures = %q, want ok", got)
	}
}

func TestClipboard_RightsLockAndBusy(t *testing.T) {
	pb := &fakePasteboard{text: "hi"}
	env := newTracerServer(t, withPasteboard(pb, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)

	// Without the clipboard right: forbidden both ways, nothing read or written.
	env.mgr.SetViewerRightsForTest(desktop.ViewerID(a.id), desktop.RightsControl)
	a.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "f1"})
	put(ctx, t, a, "f2", 0, 1, 2, "ab")
	for _, xid := range []string{"f1", "f2"} {
		if got := a.clipResult(t, xid); got != "forbidden" {
			t.Errorf("%s without the right = %q, want forbidden", xid, got)
		}
	}
	if pb.readCount() != 0 || len(pb.writes()) != 0 {
		t.Error("a viewer without the right reached the pasteboard")
	}

	// A paste in flight: a copy from the same viewer is busy.
	put(ctx, t, b, "p1", 0, 2, 4, "ab")
	b.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "g1"})
	if got := b.clipResult(t, "g1"); got != "busy" {
		t.Errorf("copy during a paste = %q, want busy", got)
	}
	put(ctx, t, b, "p1", 1, 2, 4, "cd")
	if got := b.clipResult(t, "p1"); got != "ok" {
		t.Errorf("paste after the busy answer = %q, want ok", got)
	}

	// A viewer that leaves mid-paste leaves nothing behind.
	put(ctx, t, b, "p2", 0, 2, 4, "ab")
	eventually(t, 5*time.Second, "the transfer is recorded", func() bool { return env.mgr.ClipPutsForTest() == 1 })
	_ = b.conn.CloseNow()
	eventually(t, 5*time.Second, "the transfer is dropped with the viewer", func() bool { return env.mgr.ClipPutsForTest() == 0 })
	if w := pb.writes(); len(w) != 1 || w[0] != "abcd" {
		t.Errorf("writes = %q, want only the completed paste", w)
	}
}

// TestClipboard_LockedAnswersFailed: with the Mac's screen locked both
// directions answer failed and the pasteboard is never touched.
func TestClipboard_LockedAnswersFailed(t *testing.T) {
	pb := &fakePasteboard{text: "hi"}
	locked := true
	env := newTracerServer(t, withPasteboard(pb, &locked))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinSurf(ctx, t, env.token(t, "g1"))

	v.ask(ctx, t, map[string]any{"type": "clip_get", "xid": "l1"})
	put(ctx, t, v, "l2", 0, 1, 2, "ab")
	for _, xid := range []string{"l1", "l2"} {
		if got := v.clipResult(t, xid); got != "failed" {
			t.Errorf("%s while locked = %q, want failed", xid, got)
		}
	}
	if pb.readCount() != 0 || len(pb.writes()) != 0 {
		t.Error("the pasteboard was touched while locked")
	}
}
