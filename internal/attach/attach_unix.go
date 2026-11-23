//go:build !windows

package attach

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/websocket"
	"golang.org/x/term"
)

// WatchResize listens for SIGWINCH (terminal resize) signals on Unix platforms
// and sends a size claim (MakeClientClaimFrame, INPT-06: the user resized the
// terminal, so it takes the grid) whenever the terminal dimensions change. The
// claim carries the rows the PTY may use: reserve() fewer than the terminal
// has, one less while the status bar is shown. It runs in a background
// goroutine and exits when ctx is cancelled.
func WatchResize(ctx context.Context, conn *websocket.Conn, reserve func() int) {
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	go func() {
		defer signal.Stop(winchCh)
		watchResize(ctx, conn, winchCh, func() (int, int, error) { return term.GetSize(int(os.Stdin.Fd())) }, reserve)
	}()
}

// watchResize is WatchResize's loop with the signal channel and the size
// lookup passed in, so a test can drive it. reserve is read per signal. It
// returns when ctx is done.
func watchResize(ctx context.Context, conn *websocket.Conn, winch <-chan os.Signal, size func() (int, int, error), reserve func() int) {
	for {
		select {
		case <-winch:
			cols, rows, err := size()
			if err != nil {
				continue
			}
			_ = conn.Write(ctx, websocket.MessageBinary, MakeClientClaimFrame(uint16(cols), uint16(BarRows(rows, reserve()))))
		case <-ctx.Done():
			return
		}
	}
}
