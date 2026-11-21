package daemon

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/relay"
)

const previewTestKey = "0123456789abcdef0123456789abcdef"

// previewAPI is an API whose engine holds one hub, id "s1", fed by a pipe
// instead of a PTY. Closing the returned writer ends the hub.
func previewAPI(t *testing.T) (*API, *relay.Hub, *io.PipeWriter) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	engine := NewSessionEngine()
	engine.configDir = t.TempDir()
	a := NewAPI(engine)
	pr, pw := io.Pipe()
	hub := engine.Manager().Create("s1", pr, io.Discard, nil)
	t.Cleanup(func() { pw.Close(); engine.Manager().Shutdown() })
	return a, hub, pw
}

func previewSubscriber(hub *relay.Hub) *relay.Subscriber {
	sub := &relay.Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}}
	hub.Subscribe(sub)
	return sub
}

// previewRequest builds a POST through the freeze-gated socket handler's
// route with the three headers (name percent-encoded as the CLI sends it).
func previewRequest(id, name, mime, key string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/sessions/"+id+"/preview", bytes.NewReader(body))
	req.Header.Set("X-Portty-Name", url.QueryEscape(name))
	req.Header.Set("X-Portty-Mime", mime)
	req.Header.Set("X-Portty-Key", key)
	return req
}

func serve(a *API, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

type gotPreview struct {
	header relay.PreviewHeader
	data   []byte
	status byte
	ended  bool
}

// readPreview collects one preview's frames from sub (skipping non-preview
// frames), sleeping delay after each frame.
func readPreview(t *testing.T, sub *relay.Subscriber, delay, timeout time.Duration) gotPreview {
	t.Helper()
	var g gotPreview
	deadline := time.After(timeout)
	for !g.ended {
		select {
		case f := <-sub.Msgs:
			if len(f) == 0 || f[0] != relay.MsgPreview {
				continue
			}
			if hd, err := relay.ParsePreviewHeader(f); err == nil {
				g.header = hd
			} else if _, d, err := relay.ParsePreviewChunk(f); err == nil {
				g.data = append(g.data, d...)
			} else if _, st, err := relay.ParsePreviewEnd(f); err == nil {
				g.status, g.ended = st, true
			} else {
				t.Fatalf("unparseable preview frame % x", f[:min(len(f), 8)])
			}
			time.Sleep(delay)
		case <-deadline:
			t.Fatalf("preview not complete after %s (got %d bytes)", timeout, len(g.data))
		}
	}
	return g
}

func previewBody(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}

func TestPreview_EndToEnd(t *testing.T) {
	a, hub, _ := previewAPI(t)
	s1, s2 := previewSubscriber(hub), previewSubscriber(hub)
	body := previewBody(100 << 10)

	rec := serve(a, previewRequest("s1", "shot.png", "image/png", previewTestKey, body))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST preview = %d %s; want 202", rec.Code, rec.Body)
	}
	var resp PreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Viewers != 2 {
		t.Fatalf("body %s: want {\"viewers\":2} (%v)", rec.Body, err)
	}
	for i, sub := range []*relay.Subscriber{s1, s2} {
		g := readPreview(t, sub, 0, 10*time.Second)
		hd := g.header
		if hd.Name != "shot.png" || hd.MIME != "image/png" || hd.Size != len(body) || hd.Key != previewTestKey {
			t.Errorf("viewer %d header = %+v", i, hd)
		}
		if !bytes.Equal(g.data, body) || g.status != relay.PreviewOK {
			t.Errorf("viewer %d: %d bytes, equal=%v, status %d; want the body in order and status 0", i, len(g.data), bytes.Equal(g.data, body), g.status)
		}
	}
}

func TestPreview_NameIsPercentDecodedAndCleaned(t *testing.T) {
	for _, c := range []struct{ sent, want string }{
		{"ré sumé.md", "ré sumé.md"},
		{"../x/y.png", "y.png"},
		{"evil‮gnp.exe\x07.md", "evilgnp.exe.md"},
		{"a⁦b\u0085c.md", "abc.md"},
		{strings.Repeat("é", 200), strings.Repeat("é", 127)}, // 255-byte cap on a rune boundary
	} {
		a, hub, _ := previewAPI(t)
		sub := previewSubscriber(hub)
		rec := serve(a, previewRequest("s1", c.sent, "text/markdown", previewTestKey, []byte("# hi")))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("name %q: POST = %d %s", c.sent, rec.Code, rec.Body)
		}
		if got := readPreview(t, sub, 0, 5*time.Second).header.Name; got != c.want {
			t.Errorf("name %q arrived as %q; want %q", c.sent, got, c.want)
		}
	}
}

// noPreviewFrame fails if sub holds a preview frame.
func noPreviewFrame(t *testing.T, sub *relay.Subscriber, what string) {
	t.Helper()
	for {
		select {
		case f := <-sub.Msgs:
			if len(f) > 0 && f[0] == relay.MsgPreview {
				t.Errorf("%s: a preview frame reached the viewer", what)
			}
		default:
			return
		}
	}
}

// readCounter fails the test if the handler reads a body it should refuse
// from Content-Length alone.
type readCounter struct{ n atomic.Int64 }

func (r *readCounter) Read(p []byte) (int, error) { r.n.Add(int64(len(p))); return len(p), nil }

func TestPreview_Refusals(t *testing.T) {
	a, hub, _ := previewAPI(t)
	sub := previewSubscriber(hub)

	endedAPI, ended, pw := previewAPI(t)
	endedSub := previewSubscriber(ended)
	pw.Close()
	select {
	case <-ended.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("hub did not end after its pipe closed")
	}
	if rec := serve(endedAPI, previewRequest("s1", "a.png", "image/png", previewTestKey, []byte("x"))); rec.Code != http.StatusConflict {
		t.Errorf("ended hub = %d %s; want 409", rec.Code, rec.Body)
	}
	noPreviewFrame(t, endedSub, "ended hub")

	small := []byte("x")
	cases := []struct {
		name string
		req  func() *http.Request
		want int
	}{
		{"unknown session", func() *http.Request {
			return previewRequest("nope", "a.png", "image/png", previewTestKey, small)
		}, http.StatusNotFound},
		{"content-length over cap", func() *http.Request {
			rc := &readCounter{}
			req := httptest.NewRequest(http.MethodPost, "/sessions/s1/preview", rc)
			req.Header.Set("X-Portty-Name", "a.png")
			req.Header.Set("X-Portty-Mime", "image/png")
			req.Header.Set("X-Portty-Key", previewTestKey)
			req.ContentLength = relay.PreviewMaxBytes + 1
			t.Cleanup(func() {
				if rc.n.Load() != 0 {
					t.Errorf("content-length over cap: handler read %d bytes; want 0", rc.n.Load())
				}
			})
			return req
		}, http.StatusRequestEntityTooLarge},
		{"chunked body over cap", func() *http.Request {
			req := previewRequest("s1", "a.png", "image/png", previewTestKey, make([]byte, relay.PreviewMaxBytes+1))
			req.ContentLength = -1
			return req
		}, http.StatusRequestEntityTooLarge},
		{"mime text/html", func() *http.Request {
			return previewRequest("s1", "a.html", "text/html", previewTestKey, small)
		}, http.StatusUnsupportedMediaType},
		{"mime image/svg", func() *http.Request {
			return previewRequest("s1", "a.svg", "image/svg", previewTestKey, small)
		}, http.StatusUnsupportedMediaType},
		{"missing key", func() *http.Request {
			return previewRequest("s1", "a.png", "image/png", "", small)
		}, http.StatusBadRequest},
		{"31-hex key", func() *http.Request {
			return previewRequest("s1", "a.png", "image/png", previewTestKey[1:], small)
		}, http.StatusBadRequest},
		{"uppercase key", func() *http.Request {
			return previewRequest("s1", "a.png", "image/png", strings.ToUpper(previewTestKey), small)
		}, http.StatusBadRequest},
		{"empty name after cleaning", func() *http.Request {
			return previewRequest("s1", "dir/‮\x01 ", "image/png", previewTestKey, small)
		}, http.StatusBadRequest},
		{"malformed percent-encoding", func() *http.Request {
			req := previewRequest("s1", "", "image/png", previewTestKey, small)
			req.Header.Set("X-Portty-Name", "a%zz.png")
			return req
		}, http.StatusBadRequest},
	}
	for _, c := range cases {
		if rec := serve(a, c.req()); rec.Code != c.want {
			t.Errorf("%s = %d %s; want %d", c.name, rec.Code, rec.Body, c.want)
		}
		noPreviewFrame(t, sub, c.name)
	}
}

func TestPreview_BusyIs429(t *testing.T) {
	a, hub, _ := previewAPI(t)
	sub := previewSubscriber(hub) // never drained: the first preview stays in flight
	defer hub.Unsubscribe(sub)
	if rec := serve(a, previewRequest("s1", "a.png", "image/png", previewTestKey, previewBody(1<<20))); rec.Code != http.StatusAccepted {
		t.Fatalf("first POST = %d %s", rec.Code, rec.Body)
	}
	rec := serve(a, previewRequest("s1", "b.png", "image/png", previewTestKey, []byte("x")))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("second POST = %d Retry-After=%q; want 429 with Retry-After: 1", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestPreview_ZeroViewers(t *testing.T) {
	a, _, _ := previewAPI(t)
	rec := serve(a, previewRequest("s1", "a.pdf", "application/pdf", previewTestKey, []byte("%PDF-")))
	if rec.Code != http.StatusAccepted || strings.TrimSpace(rec.Body.String()) != `{"viewers":0}` {
		t.Fatalf("POST = %d %s; want 202 {\"viewers\":0}", rec.Code, rec.Body)
	}
}

// RESEARCH Pitfall 4: a handoff drains writes for only writeDrainTimeout, so
// the route must not hold the write gate while it streams.
func TestPreview_HandoffDrainNotBlocked(t *testing.T) {
	a, hub, _ := previewAPI(t)
	sub := previewSubscriber(hub)
	defer hub.Unsubscribe(sub) // ends the paced sender
	body := previewBody(relay.PreviewMaxBytes)

	recCh := make(chan *httptest.ResponseRecorder, 1)
	go func() { recCh <- serve(a, previewRequest("s1", "big.pdf", "application/pdf", previewTestKey, body)) }()

	var first, ended atomic.Bool
	stop := make(chan struct{})
	defer close(stop)
	go func() { // one frame per 10 ms: ~12 s for the whole preview
		for {
			select {
			case f := <-sub.Msgs:
				if len(f) > 0 && f[0] == relay.MsgPreview {
					first.Store(true)
					if _, _, err := relay.ParsePreviewEnd(f); err == nil {
						ended.Store(true)
					}
				}
				time.Sleep(10 * time.Millisecond)
			case <-stop:
				return
			}
		}
	}()
	for start := time.Now(); !first.Load(); time.Sleep(5 * time.Millisecond) {
		if time.Since(start) > 10*time.Second {
			t.Fatal("no preview frame reached the viewer")
		}
	}

	start := time.Now()
	if !a.drainWrites(writeDrainTimeout) {
		t.Fatalf("drainWrites failed while the preview streamed (the route held the write gate)")
	}
	a.writeGate.Unlock()
	if d := time.Since(start); d > writeDrainTimeout/2 {
		t.Errorf("drainWrites took %s; want well within %s", d, writeDrainTimeout)
	}
	if ended.Load() {
		t.Fatal("the preview finished before the drain; the test proved nothing")
	}
	if rec := <-recCh; rec.Code != http.StatusAccepted {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
}

func TestPreview_NotOnRelayHandler(t *testing.T) {
	a, _, _ := previewAPI(t)
	h := a.RelayHandler()
	for _, m := range []string{http.MethodPost, http.MethodGet} {
		req := previewRequest("s1", "a.png", "image/png", previewTestKey, []byte("x"))
		req.Method = m
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("relay loopback %s /sessions/s1/preview = %d; want 404 or 405", m, rec.Code)
		}
	}
}

// PREV-03: no preview code can read a file by path.
func TestPreview_SourceGate(t *testing.T) {
	for _, path := range []string{"preview.go", "../relay/preview.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == "os" || p == "io/fs" || p == "path/filepath" {
				t.Errorf("%s imports %q; preview code must never touch the file system", path, p)
			}
		}
	}
}

// PATH-02: the one file read a viewer's click can cause goes through
// previewfile (the `portty open` checks): the click file has no file-system
// imports, previewfile starts no process, and no other relay file reads files
// through it.
func TestPathClick_SourceGate(t *testing.T) {
	imports := func(path string) []string {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			out = append(out, p)
		}
		return out
	}
	for _, p := range imports("../relay/pathclick.go") {
		switch p {
		case "os", "os/exec", "io/fs", "path/filepath", "syscall", "io/ioutil":
			t.Errorf("relay/pathclick.go imports %q; every host file read must go through previewfile", p)
		}
	}
	for _, p := range imports("../previewfile/previewfile.go") {
		if p == "os/exec" {
			t.Errorf("previewfile.go imports %q; a path click must never start a process", p)
		}
	}
	files, err := filepath.Glob("../relay/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no relay sources found: %v", err)
	}
	var users []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		for _, p := range imports(f) {
			if p == "github.com/scottkw/portty/internal/previewfile" {
				users = append(users, filepath.Base(f))
			}
		}
	}
	if len(users) != 1 || users[0] != "pathclick.go" {
		t.Errorf("relay files importing previewfile: %v, want only pathclick.go", users)
	}
}
