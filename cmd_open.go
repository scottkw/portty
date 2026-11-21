package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/relay"
)

const openUsage = "usage: portty open <file>"

// cmdOpen reads a file in the user's shell and hands its bytes to the daemon
// hosting this session, which shows it in a preview tab (PREV-01, PREV-03).
// The returned error's text is the one line runCLI prints (UI-SPEC "CLI Copy").
// Order: arguments, session id, file checks, then the daemon, so a file
// mistake never touches the socket. It never starts or restarts the daemon.
//
//	portty open <file>
func cmdOpen(client *daemon.DaemonClient, args []string, out io.Writer) error {
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == restartDaemonFlag || a == keepDaemonFlag })
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(out, openUsage)
		fmt.Fprintln(out, "Show an image, PDF or Markdown file in a preview tab.")
		return nil
	}
	if len(args) != 1 {
		return errors.New(openUsage)
	}
	arg := args[0]
	errNoSession := errors.New("portty: this shell isn't a Portty session. Run portty open in a Portty terminal tab.")
	session := os.Getenv("PORTTY_SESSION_ID")
	if !isSessionID(session) {
		return errNoSession
	}

	fi, err := os.Stat(arg)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("portty: %q doesn't exist.", arg)
	case err != nil:
		return readErr(arg, err)
	case fi.IsDir():
		return fmt.Errorf("portty: %q is a folder, not a file.", arg)
	case !fi.Mode().IsRegular(): // a FIFO or device would block or never end
		return unsupportedErr(arg)
	case fi.Size() > relay.PreviewMaxBytes:
		return tooLargeErr(arg, fi.Size())
	}
	f, err := os.Open(arg)
	if err != nil {
		return readErr(arg, err)
	}
	data, err := io.ReadAll(io.LimitReader(f, relay.PreviewMaxBytes+1))
	f.Close()
	if err != nil {
		return readErr(arg, err)
	}
	if len(data) > relay.PreviewMaxBytes { // it grew after the stat
		return tooLargeErr(arg, int64(len(data)))
	}
	mime := previewMIME(arg, data)
	if mime == "" {
		return unsupportedErr(arg)
	}
	key, err := previewKey(arg)
	if err != nil {
		return readErr(arg, err)
	}

	h, err := client.HealthInfo()
	if err != nil {
		return reachErr(err)
	}
	if daemon.DaemonAPI(h) < daemon.PreviewAPI {
		v := h.Version
		if v == "" {
			v = "unknown"
		}
		return fmt.Errorf("portty: update needed. Portty's background service (version %s) can't show previews; relaunch Portty to update it.", v)
	}
	resp, err := client.SendPreview(session, filepath.Base(arg), mime, key, data)
	switch {
	case errors.Is(err, daemon.ErrPreviewNoSession), errors.Is(err, daemon.ErrPreviewEnded):
		return errNoSession // unknown id or ended session, e.g. a stale id inside tmux
	case errors.Is(err, daemon.ErrPreviewBusy):
		return errors.New("portty: another preview is still being sent to this session. Try again in a moment.")
	case errors.Is(err, daemon.ErrPreviewTooLarge):
		return tooLargeErr(arg, int64(len(data)))
	case errors.Is(err, daemon.ErrPreviewUnsupported):
		return unsupportedErr(arg)
	case err != nil:
		return reachErr(err)
	case resp.Viewers == 0:
		return errors.New("portty: nothing is showing this session, so no preview opened.")
	}
	return nil
}

func isSessionID(s string) bool {
	_, err := hex.DecodeString(s)
	return len(s) == 32 && err == nil && s == strings.ToLower(s)
}

// readErr words a stat, open or read failure with the OS reason only
// ("permission denied"), not Go's "open <path>:" prefix.
func readErr(arg string, err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return fmt.Errorf("portty: couldn't read %q: %v.", arg, err)
}

func unsupportedErr(arg string) error {
	return fmt.Errorf("portty: can't preview this file type (%q). Portty previews PNG, JPEG, GIF, WebP and SVG images, PDFs and Markdown files.", arg)
}

// tooLargeErr shows the size in decimal MB, one decimal (UI-SPEC "Size
// display"); only over-cap sizes reach it, so it never needs KB.
func tooLargeErr(arg string, size int64) error {
	return fmt.Errorf("portty: %q is too large to preview (%.1f MB; the limit is 20 MB).", arg, float64(size)/1e6)
}

// reachErr keeps the dial error (socket path and reason) when there is one.
func reachErr(err error) error {
	var oe *net.OpError
	if errors.As(err, &oe) {
		err = oe
	}
	return fmt.Errorf("portty: couldn't reach Portty's background service: %s.", strings.TrimRight(strings.TrimSpace(err.Error()), "."))
}

// previewMIME decides the type: raster images and PDF by content whatever the
// name; SVG and Markdown by extension plus content. "" means unsupported.
func previewMIME(name string, data []byte) string {
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

// previewKey is the first 16 bytes of SHA-256 over the absolute,
// symlink-resolved path, in lowercase hex: the same file always gets the same
// key, and the path itself never leaves the CLI.
func previewKey(path string) (string, error) {
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
