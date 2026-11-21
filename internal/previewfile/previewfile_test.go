package previewfile

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{7}, 64)...)
	jpegBytes = append([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00"), bytes.Repeat([]byte{1}, 32)...)
	gifBytes  = append([]byte("GIF89a\x01\x00\x01\x00"), bytes.Repeat([]byte{2}, 32)...)
	webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{3}, 32)...)
	pdfBytes  = []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")
)

const small = 1000

func write(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpen_Order(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe.png")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	unreadable := write(t, dir, "locked.png", pngBytes)
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	over := write(t, dir, "big.png", append(bytes.Clone(pngBytes), bytes.Repeat([]byte{0}, small)...))
	for _, c := range []struct {
		name string
		path string
		is   error
	}{
		{"missing", filepath.Join(dir, "nope.png"), ErrNotExist},
		{"folder", dir, ErrDir},
		{"fifo", fifo, ErrUnsupported},
		{"over the cap", over, ErrTooLarge},
		{"unsupported type", write(t, dir, "notes.txt", []byte("plain")), ErrUnsupported},
	} {
		done := make(chan error, 1)
		go func() { _, err := Open(c.path, small); done <- err }()
		select {
		case err := <-done:
			if !errors.Is(err, c.is) {
				t.Errorf("%s: err = %v, want %v", c.name, err, c.is)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: Open blocked", c.name)
		}
	}

	_, err := Open(over, small)
	var tl *TooLargeError
	if !errors.As(err, &tl) || tl.Size != int64(len(pngBytes)+small) {
		t.Errorf("over the cap: err = %#v, want TooLargeError with the file size", err)
	}

	// An unreadable file returns the OS error itself, not a sentinel.
	if os.Getuid() != 0 {
		_, err = Open(unreadable, small)
		if err == nil || !errors.Is(err, os.ErrPermission) {
			t.Errorf("unreadable: err = %v, want a permission error", err)
		}
		for _, s := range []error{ErrNotExist, ErrDir, ErrUnsupported, ErrTooLarge} {
			if errors.Is(err, s) {
				t.Errorf("unreadable: err wraps sentinel %v", s)
			}
		}
	}

	good := write(t, dir, "a.png", pngBytes)
	f, err := Open(good, small)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "a.png" || f.MIME != "image/png" || !bytes.Equal(f.Data, pngBytes) ||
		!regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(f.Key) {
		t.Errorf("File = %+v, want name a.png, image/png, 32 hex key, the bytes", f)
	}
}

// The size error carries the stat size, which the CLI words as "(20.0 MB; ...)".
func TestOpen_TooLargeSize(t *testing.T) {
	p := write(t, t.TempDir(), "a.png", pngBytes)
	_, err := Open(p, int64(len(pngBytes))-1)
	var tl *TooLargeError
	if !errors.As(err, &tl) || tl.Size != int64(len(pngBytes)) {
		t.Fatalf("err = %v, want TooLargeError(%d)", err, len(pngBytes))
	}
	if !strings.Contains(tl.Error(), strconv.Itoa(len(pngBytes))) {
		t.Errorf("TooLargeError text %q lacks the size", tl.Error())
	}
}

func TestOpen_Types(t *testing.T) {
	svgBody := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`)
	prolog := []byte("\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<!-- made by hand -->\n<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	subset := []byte("<!DOCTYPE svg [\n  <!ENTITY ns_svg \"http://www.w3.org/2000/svg\">\n]>\n")
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
		{"icon.svg", append(bytes.Clone(subset), svgBody...), "image/svg+xml"},
		{"icon.svg", []byte(`<html><svg/></html>`), ""},
		{"icon.svg", []byte(`<svgx/>`), ""},
		{"icon.svg", []byte(`<svg`), ""},
		{"icon.svg", append(bytes.Repeat([]byte(" "), 9000), svgBody...), ""},
		{"icon.svg", append([]byte("<?xml version"), svgBody...), ""},
		{"icon.svg", append([]byte("<!-- never closed"), svgBody...), ""},
		{"lol.svg", append([]byte("<!DOCTYPE svg [ <!ENTITY a \"&b;\"> ]>\n"), svgBody...), ""},
		{"pe.svg", append([]byte("<!DOCTYPE svg [\n  <!ENTITY\t% x \"<!ENTITY y 'z'>\">\n]>\n"), svgBody...), ""},
		{"open.svg", append([]byte("<!DOCTYPE svg [ <!ENTITY a \"b\">\n"), svgBody...), ""},
		{"junk.svg", append([]byte("<!DOCTYPE svg [ ]junk"), svgBody...), ""},
		{"icon.txt", svgBody, ""},
		{"README.md", []byte("# Title\n\nText — with UTF-8.\n"), "text/markdown"},
		{"notes.markdown", []byte("*hi*"), "text/markdown"},
		{"bad.md", []byte("# a\x00b"), ""},
		{"latin1.md", []byte("caf\xe9"), ""},
		{"empty.md", nil, "text/markdown"},
		{"empty.png", nil, ""},
	} {
		if got := MIME(c.name, c.data); got != c.want {
			t.Errorf("MIME(%q, % x…) = %q, want %q", c.name, c.data[:min(len(c.data), 8)], got, c.want)
		}
	}
}

func TestOpen_KeyFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "a.png", pngBytes)
	other := write(t, dir, "b.png", pngBytes)
	link := filepath.Join(dir, "latest.png")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	keys := map[string]string{}
	for _, path := range []string{p, "a.png", "./sub/../a.png", link, other} {
		k, err := Key(path)
		if err != nil {
			t.Fatalf("Key(%q): %v", path, err)
		}
		if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(k) {
			t.Errorf("Key(%q) = %q, want 32 lowercase hex", path, k)
		}
		keys[path] = k
	}
	for _, path := range []string{"a.png", "./sub/../a.png", link} {
		if keys[path] != keys[p] {
			t.Errorf("Key(%q) = %s, Key(%q) = %s; want the same file to give the same key", path, keys[path], p, keys[p])
		}
	}
	if keys[other] == keys[p] {
		t.Error("two files gave the same key")
	}
	if _, err := Key(filepath.Join(dir, "gone.png")); err == nil {
		t.Error("Key of a missing path succeeded, want an error")
	}
}

// previewfile is a leaf so the relay hub can import it without a cycle.
func TestOpen_NoModuleImports(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "previewfile.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		if strings.HasPrefix(strings.Trim(imp.Path.Value, `"`), "github.com/scottkw/portty/") {
			t.Errorf("previewfile.go imports %s, want no package of this module", imp.Path.Value)
		}
	}
}
