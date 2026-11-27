package main

import "github.com/scottkw/portty/internal/screencap"

// Phase 21-09: the home clipboard's plain text for a Cmd+V typed in control mode. The Desktop tab
// calls it only from that keystroke (no timer, focus handler or Cmd+C reads the clipboard), then sends
// the text to the remote Mac with Phase 20's explicit paste message.
//
// Only the types-first bridge reads the pasteboard: it answers ReadConcealed for a password manager or
// transient item without reading the text, which a plain-text read (Wails ClipboardGetText) cannot see.
// The text is never logged and never stored here.

// homeClipboardMax is the paste limit, 1 MiB of UTF-8, the same as Phase 20's paste message.
const homeClipboardMax = 1 << 20

// HomeClipboardResult is what ReadHomeClipboard returns. Outcome is ok (Text set), concealed, empty,
// too_large or failed; Text is empty for every outcome but ok, so a concealed item never carries text.
type HomeClipboardResult struct {
	Outcome string `json:"outcome"`
	Text    string `json:"text"`
}

// readHomePasteboard is the seam for tests; it refuses under a test binary.
var readHomePasteboard = func(maxBytes int) (string, screencap.ReadOutcome, error) {
	return screencap.NewPasteboard().ReadText(maxBytes)
}

// ReadHomeClipboard reads the home clipboard's plain text. It never errors: a refused or failed read
// is outcome failed, and the page then sends nothing.
func (a *App) ReadHomeClipboard() HomeClipboardResult {
	text, outcome, err := readHomePasteboard(homeClipboardMax)
	if err != nil {
		return HomeClipboardResult{Outcome: "failed"}
	}
	switch outcome {
	case screencap.ReadOK:
		// The bridge enforces the limit; the check is a backstop so a misbehaving bridge cannot send more.
		if len(text) > homeClipboardMax {
			return HomeClipboardResult{Outcome: "too_large"}
		}
		if text == "" {
			return HomeClipboardResult{Outcome: "empty"}
		}
		return HomeClipboardResult{Outcome: "ok", Text: text}
	case screencap.ReadConcealed:
		return HomeClipboardResult{Outcome: "concealed"}
	case screencap.ReadEmpty:
		return HomeClipboardResult{Outcome: "empty"}
	case screencap.ReadTooLarge:
		return HomeClipboardResult{Outcome: "too_large"}
	default: // ReadDenied and anything the bridge may add later
		return HomeClipboardResult{Outcome: "failed"}
	}
}
