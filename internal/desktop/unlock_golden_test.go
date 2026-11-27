package desktop

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestUnlock_GoldenLines: the Phase 23 unlock additions to the v1 wire. The
// server hello with caps unlock 1 and every {type: unlock} line decode into the
// Go message types and marshal back equal; each unlock state of the contract
// has a line; ProtocolVersion stays 1 (the additions are additive).
func TestUnlock_GoldenLines(t *testing.T) {
	if ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion is %d, want 1", ProtocolVersion)
	}
	semantically := func(b []byte) any {
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	states := map[string]bool{}
	seenHello := false
	for _, l := range readGolden(t, "testdata/signal-v1.jsonl") {
		var msg any
		switch {
		case l.Name == "hello-server-unlock":
			var h msgHello
			if err := json.Unmarshal(l.Wire, &h); err != nil || h.Caps["unlock"] != 1 {
				t.Errorf("%s decoded as %+v, %v", l.Name, h, err)
			}
			msg = &h
			seenHello = true
		case strings.HasPrefix(l.Name, "unlock-"):
			var u msgUnlock
			if err := json.Unmarshal(l.Wire, &u); err != nil || u.Type != typeUnlock {
				t.Errorf("%s decoded as %+v, %v", l.Name, u, err)
			}
			msg = &u
			states[u.State] = true
		default:
			continue
		}
		back, err := json.Marshal(msg)
		if err != nil {
			t.Errorf("%s: %v", l.Name, err)
			continue
		}
		if !reflect.DeepEqual(semantically(back), semantically(l.Wire)) {
			t.Errorf("%s: marshals back as %s, want %s", l.Name, back, l.Wire)
		}
	}
	if !seenHello {
		t.Error("golden line hello-server-unlock is missing")
	}
	for _, s := range []string{unlockUnlocking, unlockUnlocked} {
		if !states[s] {
			t.Errorf("no golden line for unlock state %q", s)
		}
	}
}
