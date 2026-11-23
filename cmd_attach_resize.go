package main

import (
	"context"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/attach"
)

// watchResize delegates to the shared attach package which handles
// platform-specific resize detection internally (SIGWINCH on Unix, no-op on
// Windows). reserve is the number of rows the status bar takes from the PTY.
func watchResize(ctx context.Context, conn *websocket.Conn, reserve func() int) {
	attach.WatchResize(ctx, conn, reserve)
}
