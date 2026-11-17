package relay

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
