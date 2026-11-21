package daemon

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

