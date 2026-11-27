package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/screencap"
)

// Phase 21-09: ReadHomeClipboard is the only reader of the home clipboard, and only a Cmd+V typed in
// control mode calls it. Every test drives it through the readHomePasteboard seam; none touches the
// host pasteboard.

func stubHomePasteboard(t *testing.T, text string, outcome screencap.ReadOutcome, err error) *int {
	t.Helper()
	orig := readHomePasteboard
	t.Cleanup(func() { readHomePasteboard = orig })
	maxSeen := new(int)
	readHomePasteboard = func(maxBytes int) (string, screencap.ReadOutcome, error) {
		*maxSeen = maxBytes
		return text, outcome, err
	}
	return maxSeen
}

func TestReadHomeClipboard_OKCarriesText(t *testing.T) {
	maxSeen := stubHomePasteboard(t, "hello", screencap.ReadOK, nil)
	got := (&App{}).ReadHomeClipboard()
	if got != (HomeClipboardResult{Outcome: "ok", Text: "hello"}) {
		t.Fatalf("got %+v, want ok with text", got)
	}
	if *maxSeen != 1048576 {
		t.Fatalf("seam called with %d, want 1048576", *maxSeen)
	}
}

func TestReadHomeClipboard_ConcealedNeverCarriesText(t *testing.T) {
	stubHomePasteboard(t, "hunter2", screencap.ReadConcealed, nil)
	got := (&App{}).ReadHomeClipboard()
	if got.Outcome != "concealed" || got.Text != "" {
		t.Fatalf("got %+v, want concealed with empty text even though the seam returned text", got)
	}
}

func TestReadHomeClipboard_OutcomeMapping(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		outcome screencap.ReadOutcome
		err     error
		want    string
	}{
		{"empty", "", screencap.ReadEmpty, nil, "empty"},
		{"too large", "", screencap.ReadTooLarge, nil, "too_large"},
		{"denied", "", screencap.ReadDenied, nil, "failed"},
		{"error", "", screencap.ReadEmpty, errors.New("pasteboard unavailable"), "failed"},
		{"ok with no text", "", screencap.ReadOK, nil, "empty"},
		{"unknown outcome", "x", screencap.ReadOutcome(99), nil, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubHomePasteboard(t, tc.text, tc.outcome, tc.err)
			got := (&App{}).ReadHomeClipboard()
			if got.Outcome != tc.want || got.Text != "" {
				t.Fatalf("got %+v, want outcome %q with empty text", got, tc.want)
			}
		})
	}
}

// The limit is 1 MiB of UTF-8: exactly that is sent, one byte more is not, even if a bridge returned it.
func TestReadHomeClipboard_LimitIsInclusive(t *testing.T) {
	stubHomePasteboard(t, strings.Repeat("a", 1048576), screencap.ReadOK, nil)
	if got := (&App{}).ReadHomeClipboard(); got.Outcome != "ok" || len(got.Text) != 1048576 {
		t.Fatalf("1048576 bytes: got outcome %q with %d bytes, want ok with all of it", got.Outcome, len(got.Text))
	}
	stubHomePasteboard(t, strings.Repeat("a", 1048577), screencap.ReadOK, nil)
	if got := (&App{}).ReadHomeClipboard(); got.Outcome != "too_large" || got.Text != "" {
		t.Fatalf("1048577 bytes: got outcome %q with %d bytes, want too_large with no text", got.Outcome, len(got.Text))
	}
}

// Without the seam the real bridge runs, and it refuses under a test binary: the binding reports failed.
func TestReadHomeClipboard_RealBridgeRefusesUnderTest(t *testing.T) {
	got := (&App{}).ReadHomeClipboard()
	if got.Outcome != "failed" || got.Text != "" {
		t.Fatalf("got %+v, want failed with empty text under a test binary", got)
	}
}
