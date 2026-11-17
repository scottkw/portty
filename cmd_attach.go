package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/attach"
	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/statusbar"
	"golang.org/x/term"
)

// attachOpts are the optional attach flags.
type attachOpts struct {
	detachKey  byte
	clientName string
	statusTop  bool
}

func parseAttachFlags(args []string) attachOpts {
	// Default detach key: Ctrl-backslash (0x1C).
	o := attachOpts{detachKey: 0x1C}
	for _, arg := range args {
		if arg == "--status-top" {
			o.statusTop = true
		} else if v, ok := strings.CutPrefix(arg, "--client="); ok && v != "" {
			o.clientName = v
		} else if v, ok := strings.CutPrefix(arg, "--detach-key="); ok && v != "" {
			switch v {
			case `ctrl-\`, "ctrl-backslash":
				o.detachKey = 0x1C
			default:
				if len(v) == 1 {
					o.detachKey = v[0]
				}
			}
		}
	}
	return o
}

// cmdAttach connects the local terminal to a session on this Mac or to a session
// shared from another Mac (<host>:<session>). It sets the terminal to raw mode,
// relays all I/O, and restores the terminal on every exit path.
func cmdAttach(client *daemon.DaemonClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty attach <session> | <host>:<session> | <host>")
	}
	opts := parseAttachFlags(args[1:])
	if host, session, isRemote := parseRemoteID(args[0]); isRemote {
		return attachRemote(client, host, session, opts)
	}
	return attachLocal(client, args[0], opts)
}

func attachLocal(client *daemon.DaemonClient, sessionID string, opts attachOpts) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("attach: stdin is not a terminal")
	}
	port, err := client.GetRelayPort()
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	sessions, err := client.ListSessions()
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	var session *daemon.SessionInfo
	for _, s := range sessions {
		if s.ID == sessionID {
			session = &s
			break
		}
	}
	if session == nil {
		return fmt.Errorf("attach: session %q not found", sessionID)
	}

	// MC-05: build WebSocket URL with optional query params.
	u := url.URL{
		Scheme: "ws",
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
		Path:   fmt.Sprintf("/sessions/%s/ws", session.ID),
	}
	if opts.clientName != "" {
		u.RawQuery = url.Values{"client": {opts.clientName}}.Encode()
	}
	createdAt, _ := time.Parse(time.RFC3339, session.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return runAttach(u.String(), session.Name, session.Hostname, createdAt, opts, false)
}

// attachRemote connects to a session shared from another Mac. The local daemon
// resolves the host, checks access through the same-account peer routes and
// holds the capability; the CLI dials the daemon's relay proxy, which injects
// the Origin and the cap upstream (RESEARCH Pattern 5).
func attachRemote(client *daemon.DaemonClient, host, session string, opts attachOpts) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("attach: stdin is not a terminal")
	}
	res, err := client.ConnectPeer(host, session)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	port, err := client.GetRelayPort()
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	return runAttach(remoteRelayURL(port, res.SessionID), res.Name, res.Host, time.Now(), opts, true)
}

// remoteRelayURL is the local daemon's proxy route for a remote session. It
// carries no cap; the daemon holds it.
func remoteRelayURL(relayPort int, sessionID string) string {
	return fmt.Sprintf("ws://127.0.0.1:%d/api/relay/remote/%s/ws", relayPort, url.PathEscape(sessionID))
}

// remoteAttachEnd recognises the host turning sharing off: the proxy forwards
// the host's close (1000, "share stopped").
func remoteAttachEnd(err error, host, name string) (msg string, revoked bool) {
	var ce websocket.CloseError
	if errors.As(err, &ce) && ce.Reason == relay.CloseReasonShareStopped {
		return fmt.Sprintf("Portty: %s stopped sharing %s.", host, name), true
	}
	return "", false
}

// runAttach dials wsURL and runs the interactive attach loop with the banner or
// status bar. remote turns on the connection-state watcher and the
// stopped-sharing exit.
func runAttach(wsURL, name, hostname string, createdAt time.Time, opts attachOpts, remote bool) error {
	// Create signal context that cancels on SIGTERM or SIGHUP.
	// Do NOT catch SIGINT — in raw mode Ctrl-C is byte 0x03, not a signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("attach: dial relay: %w", err)
	}
	defer conn.CloseNow()

	// Wrap stdout in a LockedWriter to serialize PTY output and bar draws.
	stdout := attach.NewLockedWriter(os.Stdout)

	// Phase 117 / PAPER-02: clear the local terminal so the attached session
	// renders from a clean canvas. Pure ANSI: CSI 2 J + CSI H. Only emitted when
	// stdout is a TTY — non-TTY callers get the one-shot banner on stderr instead.
	if term.IsTerminal(int(os.Stdout.Fd())) {
		_, _ = stdout.Write([]byte("\x1b[2J\x1b[H"))
	}

	// Create status bar if stdout is a TTY (SB-03).
	var bar *statusbar.Bar
	if term.IsTerminal(int(os.Stdout.Fd())) {
		pos := statusbar.Bottom
		if opts.statusTop {
			pos = statusbar.Top
		}
		bar = statusbar.New(stdout, statusbar.Options{
			SessionName: name,
			Hostname:    hostname,
			CreatedAt:   createdAt,
			Position:    pos,
			Fd:          os.Stdout.Fd(),
		})
		bar.Start()
		defer bar.Stop()
	} else {
		// Non-TTY path: show one-shot banner on stderr (bar is suppressed).
		printAttachBanner(os.Stderr, name, hostname)
	}

	// Put terminal in raw mode. Restore on every exit path.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("attach: raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck

	// Send initial resize so PTY dimensions match the local terminal.
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err == nil {
		frame := attach.MakeClientResizeFrame(uint16(cols), uint16(rows))
		_ = conn.Write(ctx, websocket.MessageBinary, frame)
	}

	// Start platform-specific SIGWINCH watcher (no-op on Windows).
	watchResize(ctx, conn)

	// SB-05: connection state watcher for remote sessions.
	// Sets "reconnecting" if no frame received for >5s.
	var onFrame func()
	if remote && bar != nil {
		var mu sync.Mutex
		var lastFrame time.Time
		onFrame = func() {
			mu.Lock()
			lastFrame = time.Now()
			mu.Unlock()
			bar.SetConnectionState("")
		}
		onFrame() // initialize with current time
		go func() {
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			for {
				select {
				case <-tick.C:
					mu.Lock()
					lf := lastFrame
					mu.Unlock()
					if time.Since(lf) > 5*time.Second {
						bar.SetConnectionState("reconnecting")
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	err = attach.AttachSession(ctx, conn, os.Stdin, stdout, opts.detachKey, bar, onFrame)
	if msg, revoked := remoteAttachEnd(err, hostname, name); remote && revoked {
		// Restore the terminal and remove the bar before printing; the deferred
		// calls would run after the message. Both are safe to repeat.
		term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
		if bar != nil {
			bar.Stop()
		}
		fmt.Fprintf(os.Stdout, "\n%s\n", msg)
		return nil
	}
	printDetachMessage(os.Stderr)
	return err
}

// printAttachBanner writes the connection banner to w (typically os.Stderr).
// It shows session name, hostname, and detach key hint.
func printAttachBanner(w io.Writer, name, hostname string) {
	displayName := name
	if displayName == "" {
		displayName = "unnamed"
	}
	fmt.Fprintf(w, "───────────────────────────────────\n")
	fmt.Fprintf(w, " %s", displayName)
	if hostname != "" {
		fmt.Fprintf(w, " │ %s", hostname)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, " Press Ctrl-\\ to detach.\n")
	fmt.Fprintf(w, "───────────────────────────────────\n")
}

// printDetachMessage writes the detach confirmation to w (typically os.Stderr).
func printDetachMessage(w io.Writer) {
	fmt.Fprintf(w, "\nDetached.\n")
}
