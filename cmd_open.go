package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/previewfile"
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
	// The daemon flags (ignored) only lead, and -- ends the options, so any file name can be opened (11-REVIEW IN-06).
	for len(args) > 0 && (args[0] == restartDaemonFlag || args[0] == keepDaemonFlag) {
		args = args[1:]
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(out, openUsage)
		fmt.Fprintln(out, "Show an image, PDF or Markdown file in a preview tab.")
		return nil
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
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

	file, err := previewfile.Open(arg, relay.PreviewMaxBytes)
	var tl *previewfile.TooLargeError
	switch {
	case errors.Is(err, previewfile.ErrNotExist):
		return fmt.Errorf("portty: %q doesn't exist.", arg)
	case errors.Is(err, previewfile.ErrDir):
		return fmt.Errorf("portty: %q is a folder, not a file.", arg)
	case errors.Is(err, previewfile.ErrUnsupported):
		return unsupportedErr(arg)
	case errors.As(err, &tl):
		return tooLargeErr(arg, tl.Size)
	case err != nil:
		return readErr(arg, err)
	}
	data := file.Data

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
	resp, err := client.SendPreview(session, file.Name, file.MIME, file.Key, data)
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
