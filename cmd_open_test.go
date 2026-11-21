package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/relay"
)

const openTestSession = "0123456789abcdef0123456789abcdef"

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{7}, 64)...)
	jpegBytes = append([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00"), bytes.Repeat([]byte{1}, 32)...)
	gifBytes  = append([]byte("GIF89a\x01\x00\x01\x00"), bytes.Repeat([]byte{2}, 32)...)
	webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{3}, 32)...)
	pdfBytes  = []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")
)

// shortTempDir is under /tmp because a unix socket path must fit sun_path (103 bytes).
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "p11")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// openDaemon serves the real daemon API on a temp socket with one hub, id
// openTestSession, fed by a pipe instead of a PTY.
func openDaemon(t *testing.T) (*daemon.DaemonClient, *relay.Hub) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	engine := daemon.NewSessionEngine()
	api := daemon.NewAPI(engine)
	sock := filepath.Join(shortTempDir(t), "d.sock")
	if err := api.Start(sock); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	pr, pw := io.Pipe()
	hub := engine.Manager().Create(openTestSession, pr, io.Discard, nil)
	t.Cleanup(func() { pw.Close(); engine.Manager().Shutdown(); api.Stop() })
	return daemon.NewDaemonClient(sock), hub
}

// fakeDaemon serves h on a temp socket, standing in for daemons the real API
// can't be made to be (an old API, a busy or ended session).
func fakeDaemon(t *testing.T, h http.HandlerFunc) *daemon.DaemonClient {
	t.Helper()
	sock := filepath.Join(shortTempDir(t), "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return daemon.NewDaemonClient(sock)
}

// previewDaemon answers /health with api and every preview POST with status
// and body, counting the POSTs.
func previewDaemon(t *testing.T, api, status int, body string, posts *atomic.Int32) *daemon.DaemonClient {
	return fakeDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			io.WriteString(w, `{"status":"ok","version":"1.1.0","api":`+strconv.Itoa(api)+`}`)
			return
		}
		if posts != nil {
			posts.Add(1)
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	})
}

func TestCmdOpen_EndToEnd(t *testing.T) {
	client, hub := openDaemon(t)
	sub := &relay.Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}}
	hub.Subscribe(sub)
	data := append(bytes.Clone(pngBytes), bytes.Repeat([]byte("pixels"), 10_000)...) // several chunks
	p := writeFile(t, t.TempDir(), "shot.png", data)
	t.Setenv("PORTTY_SESSION_ID", openTestSession)

	var out bytes.Buffer
	if err := cmdOpen(client, []string{p}, &out); err != nil {
		t.Fatalf("cmdOpen: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("success printed %q, want nothing", out.String())
	}

	var hd relay.PreviewHeader
	var got []byte
	deadline := time.After(10 * time.Second)
	for ended := false; !ended; {
		select {
		case f := <-sub.Msgs:
			if len(f) == 0 || f[0] != relay.MsgPreview {
				continue
			}
			if h, err := relay.ParsePreviewHeader(f); err == nil {
				hd = h
			} else if _, d, err := relay.ParsePreviewChunk(f); err == nil {
				got = append(got, d...)
			} else if _, st, err := relay.ParsePreviewEnd(f); err == nil {
				if st != relay.PreviewOK {
					t.Errorf("end status %d, want ok", st)
				}
				ended = true
			}
		case <-deadline:
			t.Fatalf("preview incomplete after 10s (%d bytes)", len(got))
		}
	}
	if hd.Name != "shot.png" || hd.MIME != "image/png" || hd.Size != len(data) || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(hd.Key) {
		t.Errorf("header = %+v", hd)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("got %d bytes, want the file's %d in order", len(got), len(data))
	}
}

func TestCmdOpen_Lines(t *testing.T) {
	dir := t.TempDir()
	png := writeFile(t, dir, "a.png", pngBytes)
	txt := writeFile(t, dir, "notes.txt", []byte("hello"))
	fakePNG := writeFile(t, dir, "fake.png", []byte("just text"))
	locked := writeFile(t, dir, "locked.png", pngBytes)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(dir, "big.png")
	if err := os.WriteFile(big, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, relay.PreviewMaxBytes+1); err != nil { // sparse: no 20 MB write
		t.Fatal(err)
	}
	unreachable := daemon.NewDaemonClient(filepath.Join(shortTempDir(t), "none.sock"))
	var posts atomic.Int32
	never := previewDaemon(t, 3, http.StatusAccepted, `{"viewers":1}`, &posts) // file errors must not reach it

	const noSession = "portty: this shell isn't a Portty session. Run portty open in a Portty terminal tab."
	cases := []struct {
		name    string
		session string
		client  *daemon.DaemonClient
		args    []string
		want    string // exact line, or a prefix ending in ": " for OS detail
	}{
		{"no args", openTestSession, never, nil, "usage: portty open <file>"},
		{"two args", openTestSession, never, []string{png, png}, "usage: portty open <file>"},
		{"no session", "", never, []string{png}, noSession},
		{"malformed session", "not-a-session", never, []string{png}, noSession},
		{"uppercase session", strings.ToUpper(openTestSession), never, []string{png}, noSession},
		{"missing", openTestSession, never, []string{"nope.png"}, `portty: "nope.png" doesn't exist.`},
		{"folder", openTestSession, never, []string{dir}, `portty: "` + dir + `" is a folder, not a file.`},
		{"unreadable", openTestSession, never, []string{locked}, `portty: couldn't read "` + locked + `": permission denied.`},
		{"txt", openTestSession, never, []string{txt}, `portty: can't preview this file type ("` + txt + `"). Portty previews PNG, JPEG, GIF, WebP and SVG images, PDFs and Markdown files.`},
		{"text as png", openTestSession, never, []string{fakePNG}, `portty: can't preview this file type ("` + fakePNG + `"). Portty previews PNG, JPEG, GIF, WebP and SVG images, PDFs and Markdown files.`},
		{"too large", openTestSession, never, []string{big}, `portty: "` + big + `" is too large to preview (20.0 MB; the limit is 20 MB).`},
		{"old daemon", openTestSession, previewDaemon(t, 2, http.StatusAccepted, `{"viewers":1}`, nil), []string{png},
			"portty: update needed. Portty's background service (version 1.1.0) can't show previews; relaunch Portty to update it."},
		{"unreachable", openTestSession, unreachable, []string{png}, "portty: couldn't reach Portty's background service: dial unix "},
		{"unknown session", openTestSession, previewDaemon(t, 3, http.StatusNotFound, "", nil), []string{png}, noSession},
		{"ended session", openTestSession, previewDaemon(t, 3, http.StatusConflict, "", nil), []string{png}, noSession},
		{"zero viewers", openTestSession, previewDaemon(t, 3, http.StatusAccepted, `{"viewers":0}`, nil), []string{png},
			"portty: nothing is showing this session, so no preview opened."},
		// 429 without Retry-After is what SendPreview returns once its retry
		// budget is spent; the budget itself is TestSendPreview_WaitsOutBusy's.
		{"busy", openTestSession, previewDaemon(t, 3, http.StatusTooManyRequests, "", nil), []string{png},
			"portty: another preview is still being sent to this session. Try again in a moment."},
		{"unexpected status", openTestSession, previewDaemon(t, 3, http.StatusBadRequest, "bad key\n", nil), []string{png},
			"portty: couldn't reach Portty's background service: daemon API POST preview: status 400: bad key."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("PORTTY_SESSION_ID", c.session)
			var out bytes.Buffer
			err := cmdOpen(c.client, c.args, &out)
			if err == nil {
				t.Fatalf("cmdOpen returned nil, want %q", c.want)
			}
			got := err.Error()
			if strings.HasSuffix(c.want, " ") {
				if !strings.HasPrefix(got, c.want) || !strings.HasSuffix(got, ".") || strings.Contains(got, "\n") {
					t.Errorf("got %q, want one line starting %q ending with a period", got, c.want)
				}
			} else if got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
			if out.Len() != 0 {
				t.Errorf("stdout %q, want nothing", out.String())
			}
		})
	}
	if n := posts.Load(); n != 0 {
		t.Errorf("file and argument errors sent %d previews, want 0", n)
	}
}

func TestCmdOpen_TypeDetection(t *testing.T) {
	svgBody := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`)
	prolog := []byte("\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<!-- made by hand -->\n<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		{"photo.md", pngBytes, "image/png"},
		{"x.txt", jpegBytes, "image/jpeg"},
		{"anim", gifBytes, "image/gif"},
		{"pic.gif", webpBytes, "image/webp"},
		{"paper.png", pdfBytes, "application/pdf"},
		{"icon.svg", svgBody, "image/svg+xml"},
		{"ICON.SVG", svgBody, "image/svg+xml"},
		{"icon.svg", append(bytes.Clone(prolog), svgBody...), "image/svg+xml"},
		{"icon.svg", []byte(`<html><svg/></html>`), ""},
		{"icon.svg", []byte(`<svgx/>`), ""},
		{"icon.svg", append(bytes.Repeat([]byte(" "), 9000), svgBody...), ""}, // root past the first 8 KiB
		// 11-REVIEW WR-01: an Illustrator header (internal subset of plain entities) and a long licence comment.
		{"logo.svg", append([]byte(illustratorProlog), svgBody...), "image/svg+xml"},
		{"logo.svg", append([]byte("<!-- "+strings.Repeat("licence text ", 160)+"-->\n"), svgBody...), "image/svg+xml"},
		// Entity expansion stays refused: nested references (billion laughs) or a parameter entity in the subset.
		{"lol.svg", readFixture(t, "hostile-external.svg"), ""},
		{"pe.svg", append([]byte("<!DOCTYPE svg [\n  <!ENTITY\t% x \"<!ENTITY y 'z'>\">\n]>\n"), svgBody...), ""},
		{"open.svg", append([]byte("<!DOCTYPE svg [ <!ENTITY a \"b\">\n"), svgBody...), ""}, // subset never closes
		{"icon.txt", svgBody, ""},
		{"README.md", []byte("# Title\n\nText — with UTF-8.\n"), "text/markdown"},
		{"notes.markdown", []byte("*hi*"), "text/markdown"},
		{"bad.md", []byte("# a\x00b"), ""},
		{"latin1.md", []byte("caf\xe9"), ""},
		{"empty.md", nil, "text/markdown"},
		{"empty.png", nil, ""},
		{"empty.pdf", nil, ""},
	} {
		if got := previewMIME(c.name, c.data); got != c.want {
			t.Errorf("previewMIME(%q, % x…) = %q, want %q", c.name, c.data[:min(len(c.data), 8)], got, c.want)
		}
	}
}

// A FIFO is refused without blocking (11-REVIEW IN-02: the open is non-blocking and checked on the descriptor too).
func TestCmdOpen_FIFORefusedPromptly(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe.png")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORTTY_SESSION_ID", openTestSession)
	errc := make(chan error, 1)
	go func() { errc <- cmdOpen(nil, []string{fifo}, io.Discard) }()
	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "can't preview this file type") {
			t.Errorf("err = %v, want the unsupported line", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cmdOpen blocked on a FIFO")
	}
}

// illustratorProlog is the header Adobe Illustrator's "SVG 1.1" export writes.
const illustratorProlog = `<?xml version="1.0" encoding="utf-8"?>
<!-- Generator: Adobe Illustrator 27.0.0, SVG Export Plug-In . SVG Version: 6.00 Build 0)  -->
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd" [
	<!ENTITY ns_extend "http://ns.adobe.com/Extensibility/1.0/">
	<!ENTITY ns_ai "http://ns.adobe.com/AdobeIllustrator/10.0/">
	<!ENTITY ns_graphs "http://ns.adobe.com/Graphs/1.0/">
]>
`

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("frontend", "src", "lib", "__tests__", "fixtures", "preview", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCmdOpen_Help(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		var out bytes.Buffer
		if err := cmdOpen(nil, []string{arg}, &out); err != nil {
			t.Fatalf("cmdOpen(%s): %v", arg, err)
		}
		if !strings.HasPrefix(out.String(), "usage: portty open <file>\n") || !strings.Contains(out.String(), "preview tab") {
			t.Errorf("cmdOpen(%s) printed %q", arg, out.String())
		}
	}
	// The daemon flags are dropped, not read as a second argument (or a file).
	t.Setenv("PORTTY_SESSION_ID", "")
	err := cmdOpen(nil, []string{"--restart-daemon", "x.png", "--keep-daemon"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "isn't a Portty session") {
		t.Errorf("with daemon flags: %v, want the not-a-session line", err)
	}
}

func TestCmdOpen_SameFileSameKey(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.png", pngBytes)
	other := writeFile(t, dir, "b.png", pngBytes)
	link := filepath.Join(dir, "latest.png")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	keys := map[string]string{}
	for _, path := range []string{p, "a.png", "./sub/../a.png", link, other} {
		k, err := previewKey(path)
		if err != nil {
			t.Fatalf("previewKey(%q): %v", path, err)
		}
		if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(k) {
			t.Errorf("previewKey(%q) = %q, want 32 lowercase hex", path, k)
		}
		keys[path] = k
	}
	for _, path := range []string{"a.png", "./sub/../a.png", link} {
		if keys[path] != keys[p] {
			t.Errorf("key(%q) = %s, key(%q) = %s; want the same file to give the same key", path, keys[path], p, keys[p])
		}
	}
	if keys[other] == keys[p] {
		t.Error("two files gave the same key")
	}
}
