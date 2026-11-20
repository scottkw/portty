package relay

// T-R3 (UPGR-01): the handoff seed against the alt-exit rebuild's stream
// generator. A stream exported at any read boundary, in or out of the alt
// screen, imported and continued, leaves the imported emulator, and the old
// one rebuilt from its own export, equal to an emulator fed every byte.

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

// handoffMismatch records reads[:cut] on a hub, exports it, imports the
// state (gob round trip included), then feeds reads[cut:] to the imported
// hub, the old hub and a reference emulator fed every byte. It returns the
// first divergence in render or cursor, or "".
func handoffMismatch(t *testing.T, reads []string, cut, cols, rows int) string {
	t.Helper()
	orig := NewHub("handoff-fuzz", nil, nil, DefaultScrollbackBytes, nil)
	resizeHub(t, orig, cols, rows)
	ref := NewQuietEmulator(cols, rows)
	orig.emuMu.Lock()
	orig.ensureLiveEmulatorLocked()
	for _, s := range reads[:cut] {
		orig.recordFrameLocked(MakeOutputFrame([]byte(s)), []byte(s))
		ref.Write([]byte(s)) //nolint:errcheck
	}
	st := orig.exportLocked() // nothing feeds this hub: no freeze needed
	orig.emuMu.Unlock()
	got, err := NewHubFromState(gobRoundTrip(t, st), nil, nil, nil)
	if err != nil {
		return fmt.Sprintf("import: %v", err)
	}
	for j := cut; j <= len(reads); j++ {
		if j > cut {
			s := reads[j-1]
			for _, h := range []*Hub{got, orig} {
				h.emuMu.Lock()
				h.recordFrameLocked(MakeOutputFrame([]byte(s)), []byte(s))
				h.emuMu.Unlock()
			}
			ref.Write([]byte(s)) //nolint:errcheck
		}
		want, wp := ref.Render(), ref.CursorPosition()
		for name, h := range map[string]*Hub{"imported": got, "old": orig} {
			h.emuMu.Lock()
			g, gp := h.liveEmu.Render(), h.liveEmu.CursorPosition()
			h.emuMu.Unlock()
			if g != want || gp != wp {
				return fmt.Sprintf("%s hub after read %d (export after %d): %q cursor %v, want %q cursor %v\nreads: %q",
					name, j, cut, g, gp, want, wp, reads)
			}
		}
	}
	return ""
}

// TestHandoffRoundTrip_Streams (T-R3): rebuildFuzzStream's streams, each
// exported at a random read boundary: 0 mismatches. Fixed seeds keep it
// deterministic.
func TestHandoffRoundTrip_Streams(t *testing.T) {
	const cols, rows = 20, 6
	streams := 300
	if testing.Short() {
		streams = 60
	}
	r := rand.New(rand.NewPCG(9, 2026))
	mismatches := 0
	for i := range streams {
		reads := rebuildFuzzStream(r, cols, rows)
		cut := r.IntN(len(reads) + 1)
		if msg := handoffMismatch(t, reads, cut, cols, rows); msg != "" {
			if mismatches < 5 {
				t.Errorf("stream %d: %s", i, msg)
			}
			mismatches++
		}
	}
	if mismatches > 0 {
		t.Errorf("%d of %d streams diverged from the reference", mismatches, streams)
	}
}

// knownSeedLoss matches bytes whose effect the seed documents as lost (the
// rebuildAfterAltExitLocked shortcut list): tabs, G2/G3 and single shifts,
// the UK set, REP. The fuzz target skips them.
var knownSeedLoss = regexp.MustCompile("\t|\x1b[NO*+n o|}~]|\x1b[()]A|\x1b\\[[0-9;]*b")

// FuzzHandoffRoundTrip: the T-R3 property on arbitrary streams, cut into
// reads of 1+step%24 bytes and exported after read cut.
func FuzzHandoffRoundTrip(f *testing.F) {
	r := rand.New(rand.NewPCG(11, 2026))
	for range 8 {
		reads := rebuildFuzzStream(r, 20, 6)
		f.Add(strings.Join(reads, ""), uint16(r.IntN(len(reads)+1)), uint8(r.IntN(24)))
	}
	f.Fuzz(func(t *testing.T, stream string, cut uint16, step uint8) {
		if len(stream) > 4096 || knownSeedLoss.MatchString(stream) {
			t.Skip()
		}
		reads := splitReads(stream, 1+int(step)%24)
		if msg := handoffMismatch(t, reads, min(int(cut), len(reads)), 20, 6); msg != "" {
			t.Fatal(msg)
		}
	})
}
