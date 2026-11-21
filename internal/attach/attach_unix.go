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
// and sends a MsgResize2 frame to the relay whenever the terminal dimensions
// change. It runs in a background goroutine and exits when ctx is cancelled.
func WatchResize(ctx context.Context, conn *websocket.Conn) {
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	go func() {
		defer signal.Stop(winchCh)
		watchResize(ctx, conn, winchCh, func() (int, int, error) { return term.GetSize(int(os.Stdin.Fd())) })
	}()
}

// watchResize is WatchResize's loop with the signal channel and the size
// lookup passed in, so a test can drive it. It returns when ctx is done.
func watchResize(ctx context.Context, conn *websocket.Conn, winch <-chan os.Signal, size func() (int, int, error)) {
	for {
		select {
		case <-winch:
			cols, rows, err := size()
			if err != nil {
				continue
			}
			_ = conn.Write(ctx, websocket.MessageBinary, MakeClientResizeFrame(uint16(cols), uint16(rows)))
		case <-ctx.Done():
			return
		}
	}
}
