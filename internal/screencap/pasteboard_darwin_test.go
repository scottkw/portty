package screencap

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestPasteboard_RefusesUnderTest: no test reads or writes the host's clipboard
// or its privacy setting.
func TestPasteboard_RefusesUnderTest(t *testing.T) {
	p := NewPasteboard()
	_, _, readErr := p.ReadText(1 << 20)
	writeErr := p.WriteText("x")
	_, behaviorErr := p.Behavior()
	for name, err := range map[string]error{"ReadText": readErr, "WriteText": writeErr, "Behavior": behaviorErr} {
		if !errors.Is(err, ErrRefusedInTest) {
			t.Errorf("%s: %v, want ErrRefusedInTest", name, err)
		}
	}
}

// objcFunc returns the body of the C function name in the ObjC file path, from
// its definition line to the first closing brace in column 0.
func objcFunc(t *testing.T, path, name string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	i := strings.Index(src, " "+name+"(")
	if i < 0 {
		t.Fatalf("%s: no function %s", path, name)
	}
	end := strings.Index(src[i:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s: %s has no closing brace", path, name)
	}
	return src[i : i+end]
}

// TestPasteboard_ReadComparesChangeCount (M-06): the concealed-type check and
// the text read are not atomic, so a pasteboard read (pcPbReadOnce) snapshots the change count before
// the type check and compares it after the read; a write in between (a
// concealed secret) is never read out. Source check: no test reads the host
// pasteboard.
func TestPasteboard_ReadComparesChangeCount(t *testing.T) {
	body := objcFunc(t, "pasteboard_darwin.m", "pcPbReadOnce")
	before := strings.Index(body, "changeCount")
	types := strings.Index(body, "[pb types]")
	read := strings.Index(body, "stringForType:")
	if before < 0 || types < 0 || read < 0 || before > types {
		t.Fatalf("pcPbReadOnce must read changeCount before [pb types] (changeCount at %d, types at %d)", before, types)
	}
	if !strings.Contains(body[read:], "changeCount") {
		t.Error("pcPbReadOnce must compare changeCount again after stringForType:")
	}
}
