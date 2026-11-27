package main

import "github.com/scottkw/portty/internal/screencap"

// RED stub (Phase 21-09 Task 2): replaced in the GREEN commit.

const homeClipboardMax = 1 << 20

type HomeClipboardResult struct {
	Outcome string `json:"outcome"`
	Text    string `json:"text"`
}

var readHomePasteboard = func(maxBytes int) (string, screencap.ReadOutcome, error) {
	return screencap.NewPasteboard().ReadText(maxBytes)
}

func (a *App) ReadHomeClipboard() HomeClipboardResult {
	return HomeClipboardResult{Outcome: "failed"}
}
