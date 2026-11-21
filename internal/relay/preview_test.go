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
