package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testPreviewKey = "0123456789abcdef0123456789abcdef"

var updatePreviewFixture = flag.Bool("update-preview-fixture", false, "rewrite the frontend preview-frames fixture")

const previewFixturePath = "../../frontend/src/lib/__tests__/fixtures/preview-frames.json"

// previewSub is a plain subscriber whose CloseSlow records that it fired.
func previewSub() (*Subscriber, *atomic.Bool) {
	kicked := new(atomic.Bool)
	return &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() { kicked.Store(true) }}, kicked
}

func previewData(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

// makeOwner makes sub the hub's owner the way a typing viewer does.
func makeOwner(t *testing.T, hub *Hub, sub *Subscriber) {
	t.Helper()
	if err := hub.ResizeClient(sub, 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := hub.NoteTyping(sub); err != nil {
		t.Fatal(err)
	}
}

type previewGot struct {
	header  PreviewHeader
	data    []byte
	status  byte
	outputs int // MsgOutput frames between the header and the end
}

// readPreview reads sub's frames until a preview end frame, sleeping delay
// after each frame (a throttled viewer). Other frame types are skipped.
func readPreview(t *testing.T, sub *Subscriber, delay, timeout time.Duration) previewGot {
	t.Helper()
	var g previewGot
	var buf bytes.Buffer
	inPreview := false
	deadline := time.After(timeout)
	for {
		var f []byte
		select {
		case f = <-sub.Msgs:
		case <-deadline:
			t.Fatalf("timeout: header=%v got %d bytes", inPreview, buf.Len())
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		switch {
		case f[0] == MsgOutput && inPreview:
			g.outputs++
		case f[0] != MsgPreview:
		case f[1] == previewKindHeader:
			hd, err := ParsePreviewHeader(f)
			if err != nil || inPreview {
				t.Fatalf("header: err=%v twice=%v", err, inPreview)
			}
			g.header, inPreview = hd, true
		case f[1] == previewKindChunk:
			id, data, err := ParsePreviewChunk(f)
			if err != nil || !inPreview || id != g.header.ID {
				t.Fatalf("chunk: err=%v header=%v id=%d want %d", err, inPreview, id, g.header.ID)
			}
			buf.Write(data)
		default:
			id, status, err := ParsePreviewEnd(f)
			if err != nil || !inPreview || id != g.header.ID {
				t.Fatalf("end: err=%v header=%v id=%d want %d", err, inPreview, id, g.header.ID)
			}
			g.data, g.status = buf.Bytes(), status
			return g
		}
	}
}

func waitDone(t *testing.T, done <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("preview Done did not close")
	}
}

func TestPreviewFrames_RoundTrip(t *testing.T) {
	hd := PreviewHeader{ID: 7, Name: "notes <v2> é.md", MIME: "text/markdown", Size: 12345, Key: testPreviewKey, Focus: true}
	got, err := ParsePreviewHeader(MakePreviewHeader(hd))
	if err != nil || got != hd {
		t.Fatalf("header round trip: %+v err=%v", got, err)
	}
	payload := previewData(PreviewChunk)
	f := MakePreviewChunk(0x01020304, payload)
	if len(f)-6 > PreviewChunk {
		t.Fatalf("chunk payload %d > PreviewChunk", len(f)-6)
	}
	id, data, err := ParsePreviewChunk(f)
	if err != nil || id != 0x01020304 || !bytes.Equal(data, payload) {
		t.Fatalf("chunk round trip: id=%x err=%v equal=%v", id, err, bytes.Equal(data, payload))
	}
	for _, st := range []byte{PreviewOK, PreviewAborted, PreviewTooLarge} {
		id, status, err := ParsePreviewEnd(MakePreviewEnd(9, st))
		if err != nil || id != 9 || status != st {
			t.Fatalf("end round trip: id=%d status=%d err=%v", id, status, err)
		}
	}
}

func TestPreviewFrames_ParseRejects(t *testing.T) {
	if _, err := ParsePreviewHeader([]byte{MsgPreview, previewKindHeader, '{', 'x'}); err == nil {
		t.Error("header with invalid JSON parsed")
	}
	if _, err := ParsePreviewHeader([]byte{MsgPreview}); err == nil {
		t.Error("truncated header parsed")
	}
	if _, _, err := ParsePreviewChunk(MakePreviewChunk(1, nil)[:5]); err == nil {
		t.Error("truncated chunk parsed")
	}
	if _, _, err := ParsePreviewChunk(MakePreviewChunk(1, make([]byte, PreviewChunk+1))); err == nil {
		t.Error("oversize chunk parsed")
	}
	if _, _, err := ParsePreviewEnd(MakePreviewEnd(1, 0)[:6]); err == nil {
		t.Error("truncated end parsed")
	}
	if _, _, err := ParsePreviewEnd(append(MakePreviewEnd(1, 0), 0)); err == nil {
		t.Error("long end parsed")
	}
	unknown := []byte{MsgPreview, 0x04, 0, 0, 0, 1, 0}
	if _, err := ParsePreviewHeader(unknown); err == nil {
		t.Error("unknown sub-kind parsed as header")
	}
	if _, _, err := ParsePreviewChunk(unknown); err == nil {
		t.Error("unknown sub-kind parsed as chunk")
	}
	if _, _, err := ParsePreviewEnd(unknown); err == nil {
		t.Error("unknown sub-kind parsed as end")
	}
	if _, _, err := ParsePreviewEnd([]byte{MsgMeta, previewKindEnd, 0, 0, 0, 1, 0}); err == nil {
		t.Error("wrong type byte parsed as end")
	}
}

func TestPreviewFrames_UnderReadLimits(t *testing.T) {
	chunk := MakePreviewChunk(^uint32(0), make([]byte, PreviewChunk))
	longestMIME := ""
	for _, m := range PreviewMIMEs {
		if len(m) > len(longestMIME) {
			longestMIME = m
		}
	}
	for _, worst := range []string{"<", `"`, "&", "\\"} {
		header := MakePreviewHeader(PreviewHeader{
			ID: ^uint32(0), Name: strings.Repeat(worst, 255), MIME: longestMIME,
			Size: PreviewMaxBytes, Key: testPreviewKey, Focus: true,
		})
		for name, f := range map[string][]byte{"chunk": chunk, "header " + worst: header} {
			if len(f) >= ViewerReadLimit || len(f) > 32768 {
				t.Errorf("%s frame is %d bytes: over ViewerReadLimit (%d) or 32768", name, len(f), ViewerReadLimit)
			}
		}
	}
}

func TestHubSendPreview_TwoViewersOwnerFocus(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	a, kickedA := previewSub()
	b, kickedB := previewSub()
	hub.Subscribe(a)
	hub.Subscribe(b)
	makeOwner(t, hub, a)

	data := previewData(100_000)
	ps, err := hub.SendPreview(Preview{Name: "shot.png", MIME: "image/png", Key: testPreviewKey, Data: data})
	if err != nil || ps.Viewers != 2 {
		t.Fatalf("SendPreview: %+v err=%v", ps, err)
	}
	ga := readPreview(t, a, 0, 5*time.Second)
	gb := readPreview(t, b, 0, 5*time.Second)
	waitDone(t, ps.Done, 5*time.Second)
	for name, g := range map[string]previewGot{"a": ga, "b": gb} {
		if g.status != PreviewOK || !bytes.Equal(g.data, data) {
			t.Errorf("%s: status=%d bytes=%d equal=%v", name, g.status, len(g.data), bytes.Equal(g.data, data))
		}
		if g.header.Name != "shot.png" || g.header.MIME != "image/png" || g.header.Size != len(data) || g.header.Key != testPreviewKey || g.header.ID != 1 {
			t.Errorf("%s header: %+v", name, g.header)
		}
	}
	if !ga.header.Focus || gb.header.Focus {
		t.Errorf("focus: owner=%v other=%v, want true/false", ga.header.Focus, gb.header.Focus)
	}
	if kickedA.Load() || kickedB.Load() {
		t.Error("a viewer was kicked")
	}
}

type previewFixtureCase struct {
	Name    string         `json:"name"`
	Kind    string         `json:"kind"` // header, chunk or end
	Frame   string         `json:"frame"`
	Header  *PreviewHeader `json:"header,omitempty"`
	ID      *uint32        `json:"id,omitempty"`
	Payload *string        `json:"payload,omitempty"` // base64
	Status  *int           `json:"status,omitempty"`
}

type previewFixture struct {
	MaxBytes int                  `json:"maxBytes"`
	ChunkMax int                  `json:"chunkMax"`
	Cases    []previewFixtureCase `json:"cases"`
}

// TestPreviewFrames_Fixture pins the wire bytes the frontend parser (11-05)
// reads. Rewrite with -update-preview-fixture.
func TestPreviewFrames_Fixture(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString
	fx := previewFixture{MaxBytes: PreviewMaxBytes, ChunkMax: PreviewChunk}
	for _, hd := range []PreviewHeader{
		{ID: 1, Name: "screenshot.png", MIME: "image/png", Size: 20000, Key: testPreviewKey, Focus: true},
		{ID: 0xfffffffe, Name: "notes <v2> é.md", MIME: "text/markdown", Size: 5, Key: "ffffffffffffffffffffffffffffffff", Focus: false},
	} {
		fx.Cases = append(fx.Cases, previewFixtureCase{Name: fmt.Sprintf("header focus=%v", hd.Focus),
			Kind: "header", Frame: b64(MakePreviewHeader(hd)), Header: &hd})
	}
	for _, c := range []struct {
		name string
		id   uint32
		n    int
	}{{"full chunk", 1, PreviewChunk}, {"short chunk", 0x01020304, 5}} {
		id, p := c.id, previewData(c.n)
		enc := b64(p)
		fx.Cases = append(fx.Cases, previewFixtureCase{Name: c.name, Kind: "chunk", Frame: b64(MakePreviewChunk(id, p)), ID: &id, Payload: &enc})
	}
	for _, c := range []struct {
		name   string
		status byte
	}{{"end ok", PreviewOK}, {"end aborted", PreviewAborted}, {"end too large", PreviewTooLarge}} {
		id, st := uint32(1), int(c.status)
		fx.Cases = append(fx.Cases, previewFixtureCase{Name: c.name, Kind: "end", Frame: b64(MakePreviewEnd(id, c.status)), ID: &id, Status: &st})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fx); err != nil {
		t.Fatal(err)
	}
	if *updatePreviewFixture {
		if err := os.WriteFile(previewFixturePath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(previewFixturePath)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("fixture stale: run go test ./internal/relay -run TestPreviewFrames_Fixture -update-preview-fixture (read err: %v)", err)
	}
}

// drainUntil keeps reading sub's queue until stop closes, so a viewer that
// finished its preview never fills up on terminal output and gets kicked.
func drainUntil(sub *Subscriber, stop <-chan struct{}) {
	for {
		select {
		case <-sub.Msgs:
		case <-stop:
			return
		}
	}
}

// TestHubSendPreview_NearCapNoKick: a 20,000,000-byte preview to two
// throttled viewers (one read-only) while the terminal ticks: nobody is
// kicked, every byte arrives in order, and terminal output keeps flowing
// during the stream (ROADMAP Phase 11 SC3).
func TestHubSendPreview_NearCapNoKick(t *testing.T) {
	hub, w := makeTestHub(t)
	a, kickedA := previewSub()
	b, kickedB := previewSub()
	b.ReadOnly = true
	hub.Subscribe(a)
	hub.Subscribe(b)
	go hub.Run()

	stopTick := make(chan struct{})
	tickDone := make(chan struct{})
	go func() {
		defer close(tickDone)
		tk := time.NewTicker(5 * time.Millisecond)
		defer tk.Stop()
		for n := 0; ; n++ {
			select {
			case <-stopTick:
				return
			case <-tk.C:
				if _, err := fmt.Fprintf(w, "TICK %d\r\n", n); err != nil {
					return
				}
			}
		}
	}()

	data := previewData(PreviewMaxBytes)
	stopDrain := make(chan struct{})
	results := make(chan previewGot, 2)
	for _, sub := range []*Subscriber{a, b} {
		go func() {
			sent := false
			defer func() { // readPreview's t.Fatalf exits this goroutine: unblock the test
				if !sent {
					results <- previewGot{status: 0xff}
				}
			}()
			g := readPreview(t, sub, time.Millisecond, 50*time.Second)
			results <- g
			sent = true
			drainUntil(sub, stopDrain)
		}()
	}
	start := time.Now()
	ps, err := hub.SendPreview(Preview{Name: "big.pdf", MIME: "application/pdf", Key: testPreviewKey, Data: data})
	if err != nil || ps.Viewers != 2 {
		t.Fatalf("SendPreview: %+v err=%v", ps, err)
	}
	g1, g2 := <-results, <-results
	waitDone(t, ps.Done, 5*time.Second)
	t.Logf("near-cap preview to 2 throttled viewers took %s", time.Since(start).Round(time.Millisecond))
	close(stopTick)
	<-tickDone
	w.Close()
	<-hub.Done()
	close(stopDrain)

	for i, g := range []previewGot{g1, g2} {
		if g.status != PreviewOK || !bytes.Equal(g.data, data) {
			t.Errorf("viewer %d: status=%d bytes=%d equal=%v", i, g.status, len(g.data), bytes.Equal(g.data, data))
		}
		if g.outputs < 20 {
			t.Errorf("viewer %d: %d terminal frames during the preview, want >= 20", i, g.outputs)
		}
	}
	if kickedA.Load() || kickedB.Load() {
		t.Errorf("kicked: a=%v b=%v", kickedA.Load(), kickedB.Load())
	}
}

func hasPreviewFrame(frames [][]byte) bool {
	for _, f := range frames {
		if f[0] == MsgPreview {
			return true
		}
	}
	return false
}

func queued(sub *Subscriber) [][]byte {
	var out [][]byte
	for {
		select {
		case f := <-sub.Msgs:
			out = append(out, f)
		default:
			return out
		}
	}
}

func TestHubSendPreview_SkipsReplayingAndLateJoiners(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	replaying, kickedR := previewSub()
	hub.Join(replaying, 0, false) // replay not finished: TakePending never called
	n, kickedN := previewSub()
	hub.Subscribe(n)

	data := previewData(1 << 20)
	ps, err := hub.SendPreview(Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: data})
	if err != nil || ps.Viewers != 1 {
		t.Fatalf("SendPreview: %+v err=%v, want 1 viewer", ps, err)
	}
	late, kickedL := previewSub()
	hub.Subscribe(late)
	if g := readPreview(t, n, time.Millisecond, 10*time.Second); g.status != PreviewOK || !bytes.Equal(g.data, data) {
		t.Fatalf("viewer: status=%d bytes=%d", g.status, len(g.data))
	}
	waitDone(t, ps.Done, 5*time.Second)

	hub.mu.Lock()
	pending := replaying.pending
	hub.mu.Unlock()
	if hasPreviewFrame(pending) || hasPreviewFrame(queued(replaying)) {
		t.Error("replaying viewer got a preview frame")
	}
	if hasPreviewFrame(queued(late)) {
		t.Error("late joiner got a preview frame")
	}
	if kickedR.Load() || kickedN.Load() || kickedL.Load() {
		t.Error("a viewer was kicked")
	}
}

func TestHubSendPreview_NoOwnerNobodyFocuses(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	a, _ := previewSub()
	b, _ := previewSub()
	hub.Subscribe(a)
	hub.Subscribe(b)
	p := Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: previewData(10)}

	ps, err := hub.SendPreview(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []*Subscriber{a, b} {
		if g := readPreview(t, sub, 0, 5*time.Second); g.header.Focus {
			t.Error("focus true with no owner")
		}
	}
	waitDone(t, ps.Done, 5*time.Second)

	makeOwner(t, hub, a)
	hub.Unsubscribe(a)
	ps, err = hub.SendPreview(p)
	if err != nil || ps.Viewers != 1 {
		t.Fatalf("SendPreview: %+v err=%v", ps, err)
	}
	if g := readPreview(t, b, 0, 5*time.Second); g.header.Focus {
		t.Error("focus true after the owner left")
	}
	waitDone(t, ps.Done, 5*time.Second)
}

func TestHubSendPreview_Busy(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	stuck, _ := previewSub() // never drains: the first preview stays in flight
	hub.Subscribe(stuck)
	p := Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: previewData(1 << 20)}
	first, err := hub.SendPreview(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.SendPreview(p); err != ErrPreviewBusy {
		t.Fatalf("second SendPreview: %v, want ErrPreviewBusy", err)
	}
	hub.Unsubscribe(stuck)
	waitDone(t, first.Done, 5*time.Second)

	next, _ := previewSub()
	hub.Subscribe(next)
	ps, err := hub.SendPreview(p)
	if err != nil || ps.Viewers != 1 {
		t.Fatalf("after Done: %+v err=%v", ps, err)
	}
	if g := readPreview(t, next, 0, 10*time.Second); g.status != PreviewOK || g.header.ID != 2 {
		t.Errorf("after Done: status=%d id=%d", g.status, g.header.ID)
	}
	waitDone(t, ps.Done, 5*time.Second)
}

func waitQueued(t *testing.T, sub *Subscriber, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(sub.Msgs) < n {
		if time.Now().After(deadline) {
			t.Fatalf("queue reached %d, want %d", len(sub.Msgs), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestHubSendPreview_ViewerLeavesMidStream(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	stays, kickedS := previewSub()
	leaves, kickedL := previewSub()
	hub.Subscribe(stays)
	hub.Subscribe(leaves)
	data := previewData(1 << 20)
	ps, err := hub.SendPreview(Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	waitQueued(t, leaves, PreviewWindow) // leaves never drains: its sender waits at the window
	hub.Unsubscribe(leaves)
	if g := readPreview(t, stays, time.Millisecond, 10*time.Second); g.status != PreviewOK || !bytes.Equal(g.data, data) {
		t.Errorf("staying viewer: status=%d bytes=%d", g.status, len(g.data))
	}
	waitDone(t, ps.Done, 5*time.Second)
	frames := queued(leaves)
	if len(frames) > PreviewWindow {
		t.Errorf("leaver got %d frames, want <= %d", len(frames), PreviewWindow)
	}
	for _, f := range frames {
		if f[0] == MsgPreview && f[1] == previewKindEnd {
			t.Error("leaver got an end frame")
		}
	}
	if kickedS.Load() || kickedL.Load() {
		t.Error("a viewer was kicked")
	}
}

func TestHubSendPreview_HubEnds(t *testing.T) {
	hub, w := makeTestHub(t)
	a, kickedA := previewSub()
	b, kickedB := previewSub()
	hub.Subscribe(a)
	hub.Subscribe(b)
	go hub.Run()
	ps, err := hub.SendPreview(Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: previewData(1 << 20)})
	if err != nil || ps.Viewers != 2 {
		t.Fatalf("SendPreview: %+v err=%v", ps, err)
	}
	waitQueued(t, a, PreviewWindow)
	waitQueued(t, b, PreviewWindow)
	w.Close() // the shell exits: Run sees EOF and shuts the hub down
	waitDone(t, ps.Done, 5*time.Second)
	for name, sub := range map[string]*Subscriber{"a": a, "b": b} {
		frames := queued(sub)
		_, status, err := ParsePreviewEnd(frames[len(frames)-1])
		if err != nil || status != PreviewAborted {
			t.Errorf("%s: last frame not an aborted end (err=%v status=%d)", name, err, status)
		}
	}
	if kickedA.Load() || kickedB.Load() {
		t.Error("a viewer was kicked")
	}
	if _, err := hub.SendPreview(Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey}); err != ErrHubEnded {
		t.Errorf("SendPreview on an ended hub: %v, want ErrHubEnded", err)
	}
}

func TestHubSendPreview_StallAborts(t *testing.T) {
	old := previewStallTimeout
	previewStallTimeout = 200 * time.Millisecond
	t.Cleanup(func() { previewStallTimeout = old })
	hub, w := makeTestHub(t)
	defer w.Close()
	stuck, kicked := previewSub()
	hub.Subscribe(stuck)
	ps, err := hub.SendPreview(Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: previewData(1 << 20)})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, ps.Done, 5*time.Second)
	frames := queued(stuck)
	if len(frames) != PreviewWindow+1 {
		t.Errorf("stuck viewer holds %d frames, want the window (%d) plus the end", len(frames), PreviewWindow)
	}
	if _, status, err := ParsePreviewEnd(frames[len(frames)-1]); err != nil || status != PreviewAborted {
		t.Errorf("last frame not an aborted end (err=%v status=%d)", err, status)
	}
	if kicked.Load() {
		t.Error("stuck viewer was kicked")
	}
	hub.mu.Lock()
	_, still := hub.subscribers[stuck]
	hub.mu.Unlock()
	if !still {
		t.Error("stuck viewer was unsubscribed")
	}
}

func TestHubSendPreview_Invalid(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	sub, _ := previewSub()
	hub.Subscribe(sub)
	ok := Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: previewData(10)}
	bad := map[string]func(p *Preview){
		"oversize":     func(p *Preview) { p.Data = make([]byte, PreviewMaxBytes+1) },
		"mime":         func(p *Preview) { p.MIME = "text/html" },
		"key short":    func(p *Preview) { p.Key = testPreviewKey[1:] },
		"key upper":    func(p *Preview) { p.Key = strings.ToUpper(testPreviewKey) },
		"name control": func(p *Preview) { p.Name = "a\x1b[2J.png" },
		"name c1":      func(p *Preview) { p.Name = "a\u009b.png" },
		"name empty":   func(p *Preview) { p.Name = "" },
		"name long":    func(p *Preview) { p.Name = strings.Repeat("a", 256) },
		"name utf8":    func(p *Preview) { p.Name = "a\xff.png" },
	}
	for name, mutate := range bad {
		p := ok
		mutate(&p)
		if _, err := hub.SendPreview(p); err != ErrPreviewInvalid {
			t.Errorf("%s: %v, want ErrPreviewInvalid", name, err)
		}
	}
	if n := len(sub.Msgs); n != 0 {
		t.Errorf("invalid previews queued %d frames", n)
	}
	ps, err := hub.SendPreview(ok) // and nothing was left busy
	if err != nil {
		t.Fatal(err)
	}
	readPreview(t, sub, 0, 5*time.Second)
	waitDone(t, ps.Done, 5*time.Second)
}

func TestHubSendPreview_NoViewers(t *testing.T) {
	hub, w := makeTestHub(t)
	defer w.Close()
	ps, err := hub.SendPreview(Preview{Name: "a.png", MIME: "image/png", Key: testPreviewKey, Data: previewData(10)})
	if err != nil || ps.Viewers != 0 {
		t.Fatalf("SendPreview: %+v err=%v", ps, err)
	}
	select {
	case <-ps.Done:
	default:
		t.Error("Done not closed with no viewers")
	}
}
