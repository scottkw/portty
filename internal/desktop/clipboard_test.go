package desktop_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

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
