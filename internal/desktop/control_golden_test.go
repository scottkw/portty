package desktop

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestControl_GoldenServerLines: the daemon-to-viewer lines of the frozen v1
// file decode into the daemon's message types and encode back to the same JSON.
func TestControl_GoldenServerLines(t *testing.T) {
	var seen int
	for _, l := range readGolden(t, "testdata/input-v1.jsonl") {
		if l.Dir != "server" {
			continue
		}
		var probe struct {
			T string `json:"t"`
		}
		if err := json.Unmarshal(l.Wire, &probe); err != nil {
			t.Fatalf("%s: %v", l.Name, err)
		}
		var back []byte
		var err error
		switch probe.T {
		case "control":
			var m msgControl
			if err = json.Unmarshal(l.Wire, &m); err == nil {
				back, err = json.Marshal(m)
			}
		case "cursor":
			var m msgCursor
			if err = json.Unmarshal(l.Wire, &m); err == nil {
				back, err = json.Marshal(m)
			}
		default:
			t.Fatalf("%s: unknown server message %q", l.Name, probe.T)
		}
		if err != nil {
			t.Fatalf("%s: %v", l.Name, err)
		}
		var a, b any
		_ = json.Unmarshal(l.Wire, &a)
		_ = json.Unmarshal(back, &b)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: %s round-trips to %s", l.Name, l.Wire, back)
		}
		seen++
	}
	if seen != 3 {
		t.Errorf("%d server lines, want 3", seen)
	}
}
