// Package previewfile is the one implementation of the `portty open` file
// checks. The CLI calls it, and so does the relay hub's path click, so a
// clicked file passes exactly the checks a CLI file does. It imports nothing
// from this module (the caller passes the size cap), so the relay can import it.
package previewfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"
)

var (
	ErrNotExist = errors.New("file doesn't exist")
	ErrDir      = errors.New("path is a folder")
	// ErrUnsupported covers a path that is not a regular file and a file with no supported type.
	ErrUnsupported = errors.New("unsupported file")
	ErrTooLarge    = errors.New("file too large")
)

// TooLargeError carries the offending size; it unwraps to ErrTooLarge.
type TooLargeError struct{ Size int64 }

func (e *TooLargeError) Error() string { return fmt.Sprintf("file too large (%d bytes)", e.Size) }
func (e *TooLargeError) Unwrap() error { return ErrTooLarge }

// File is a checked, fully read preview candidate. Name is the base name.
type File struct {
	Name, MIME, Key string
	Data            []byte
}

// Open applies the checks in order: stat (missing, folder, not a regular file,
// over maxBytes), non-blocking open, regular-file re-check on the descriptor,
// a read bounded at maxBytes+1, over the cap after the read, the type, the key.
// The four sentinels mark the expected refusals; any stat, open, read or key
// failure is returned as the OS error itself.
func Open(path string, maxBytes int64) (File, error) {
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return File{}, ErrNotExist
	case err != nil:
		return File{}, err
	case fi.IsDir():
		return File{}, ErrDir
	case !fi.Mode().IsRegular(): // a FIFO or device would block or never end
		return File{}, ErrUnsupported
	case fi.Size() > maxBytes:
		return File{}, &TooLargeError{Size: fi.Size()}
	}
	// Non-blocking, then checked on the descriptor: a path swapped for a FIFO
	// after the stat can't hang the open (11-REVIEW IN-02).
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return File{}, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil {
		return File{}, err
	} else if !fi.Mode().IsRegular() {
		return File{}, ErrUnsupported
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return File{}, err
	}
	if int64(len(data)) > maxBytes { // it grew after the stat
		return File{}, &TooLargeError{Size: int64(len(data))}
	}
	mime := MIME(path, data)
	if mime == "" {
		return File{}, ErrUnsupported
	}
	key, err := Key(path)
	if err != nil {
		return File{}, err
	}
	return File{Name: filepath.Base(path), MIME: mime, Key: key, Data: data}, nil
}

// MIME decides the type: raster images and PDF by content whatever the
// name; SVG and Markdown by extension plus content. "" means unsupported.
func MIME(name string, data []byte) string {
	if len(data) > 0 {
		switch ct := http.DetectContentType(data); ct {
		case "image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf":
			return ct
		}
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".svg":
		if hasSVGRoot(data) {
			return "image/svg+xml"
		}
	case ".md", ".markdown":
		if utf8.Valid(data) && bytes.IndexByte(data, 0) < 0 {
			return "text/markdown"
		}
	}
	return ""
}

// hasSVGRoot reports whether an <svg> root element starts within the first
// 8 KiB (licence and generator headers often exceed 1 KiB), after an optional
// BOM, XML declaration, comments and DOCTYPE. A DOCTYPE internal subset is
// allowed (Illustrator exports declare plain namespace entities there) unless
// it holds an entity reference (&) or a parameter entity (%): the
// entity-expansion shape, which is refused (11-REVIEW WR-01).
func hasSVGRoot(data []byte) bool {
	b := data[:min(len(data), 8<<10)]
	b = bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))
	for {
		b = bytes.TrimLeft(b, " \t\r\n")
		var end string
		switch {
		case bytes.HasPrefix(b, []byte("<?")):
			end = "?>"
		case bytes.HasPrefix(b, []byte("<!--")):
			end = "-->"
		case bytes.HasPrefix(b, []byte("<!DOCTYPE")):
			gt, lb := bytes.IndexByte(b, '>'), bytes.IndexByte(b, '[')
			if lb < 0 || (gt >= 0 && gt < lb) { // no internal subset
				end = ">"
				break
			}
			// shortcut: a ']' inside a quoted value ends the subset early and the file is refused, safe.
			rb := bytes.IndexByte(b[lb:], ']')
			if rb < 0 || bytes.ContainsAny(b[lb+1:lb+rb], "&%") {
				return false
			}
			rest := bytes.TrimLeft(b[lb+rb+1:], " \t\r\n")
			if !bytes.HasPrefix(rest, []byte(">")) {
				return false
			}
			b = rest[1:]
			continue
		case bytes.HasPrefix(b, []byte("<svg")):
			return len(b) > 4 && bytes.IndexByte([]byte(" \t\r\n>/"), b[4]) >= 0
		default:
			return false
		}
		i := bytes.Index(b, []byte(end))
		if i < 0 {
			return false
		}
		b = b[i+len(end):]
	}
}

// Key is the first 16 bytes of SHA-256 over the absolute, symlink-resolved
// path, in lowercase hex: the same file always gets the same key, and the
// path itself never leaves the host.
func Key(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:16]), nil
}
