package relay

import (
	"errors"

	"github.com/coder/websocket"
)

// WebSocket close reasons sent with StatusNormalClosure. They are wire values
// read by peer daemons, the CLI and browsers; the frontend mirrors them as
// constants. Clients branch on them and show fixed copy; banners never render
// wire text.
const (
	// CloseReasonShareStopped: the owner turned Share off (or the link was re-minted).
	CloseReasonShareStopped = "share stopped"
	// CloseReasonSessionEnded: the owner ended or stopped the session.
	CloseReasonSessionEnded = "session ended"
)

// IsTerminalClose reports whether err is a close a reconnecting client must
// not retry: 1000 with share stopped or session ended. 1000 alone is not final
// (a client detach sends it too). Mirrors the frontend's isTerminalClose.
func IsTerminalClose(err error) bool {
	var ce websocket.CloseError
	return errors.As(err, &ce) && ce.Code == websocket.StatusNormalClosure &&
		(ce.Reason == CloseReasonShareStopped || ce.Reason == CloseReasonSessionEnded)
}
