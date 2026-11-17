package relay

// HistoryMaxLines and HistoryMaxBytes bound the hub's rendered-line history.
const (
	HistoryMaxLines = 10_000
	HistoryMaxBytes = 4 << 20
)

// History is a ring of rendered terminal lines (RED scaffold).
type History struct{}

// NewHistory returns an empty History (RED scaffold).
func NewHistory(maxLines, maxBytes int) *History { return &History{} }

// Push appends a line (RED scaffold).
func (h *History) Push(line string) {}

// Clear drops every line (RED scaffold).
func (h *History) Clear() {}

// Len returns the number of retained lines (RED scaffold).
func (h *History) Len() int { return 0 }

// Render joins the lines with CRLF (RED scaffold).
func (h *History) Render() string { return "" }
