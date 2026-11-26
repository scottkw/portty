package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeyGate(t *testing.T) {
	allow := map[int]bool{7: true}
	for _, tc := range []struct {
		input bool
		code  int
		want  bool
	}{{true, 7, true}, {true, 8, false}, {false, 7, false}} {
		if got := keyAllowed(tc.input, allow, tc.code); got != tc.want {
			t.Errorf("keyAllowed(%v, %d) = %v, want %v", tc.input, tc.code, got, tc.want)
		}
	}
	if code, ok := parseKeyMsg("key:46"); !ok || code != 46 {
		t.Errorf("parseKeyMsg(key:46) = %d, %v", code, ok)
	}
	for _, bad := range []string{"key:", "key:a", "key:-1", "key:70000", "wake"} {
		if _, ok := parseKeyMsg(bad); ok {
			t.Errorf("parseKeyMsg(%q) accepted", bad)
		}
	}
}

func TestParseHostRunTypingNeedsAllowList(t *testing.T) {
	base := []string{"--bind", "127.0.0.1", "--evidence-dir", "x", "--type-at", "5s", "--type-keycodes", "7"}
	for name, extra := range map[string][]string{
		"no allow-input":   {"--allow-keycodes", "7"},
		"code not allowed": {"--allow-input", "--allow-keycodes", "8"},
		"no allow-list":    {"--allow-input"},
	} {
		if _, err := parseHostRun(append(append([]string{}, base...), extra...)); !errors.Is(err, errUsage) {
			t.Errorf("%s: err = %v, want errUsage", name, err)
		}
	}
	o, err := parseHostRun(append(append([]string{}, base...), "--allow-input", "--allow-keycodes", "7"))
	if err != nil || len(o.typeKeys) != 1 {
		t.Fatalf("allowed typing: %v, %v", o.typeKeys, err)
	}
	// The default command carries no input flag at all.
	if o, err = parseHostRun([]string{"--bind", "127.0.0.1", "--evidence-dir", "x", "--duration", "180s", "--sleep-at", "120s", "--wake-after", "20s"}); err != nil || o.allowInput || len(o.allowKeys) != 0 {
		t.Fatalf("default command: allowInput=%v keys=%v err=%v", o.allowInput, o.allowKeys, err)
	}
}

func TestStartFlagsHideTokenPathAndAddress(t *testing.T) {
	o, err := parseHostRun([]string{"--bind", "192.168.64.5", "--allow-lan", "--token", strings.Repeat("ab", 16), "--evidence-dir", "/home/someone/ev"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(o.flags)
	for _, leak := range []string{strings.Repeat("ab", 16), "someone", "192.168.64.5"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("start flags leak %q: %s", leak, b)
		}
	}
}

func TestTimelineLinesCarryPinnedKeys(t *testing.T) {
	tl, err := newTimeline(filepath.Join(t.TempDir(), "t.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	tl.emit(evKey, map[string]any{"codes": []int{7}, "event": "overridden"})
	time.Sleep(time.Millisecond)
	tl.emit(evPerf, nil)
	if err := tl.close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(tl.f.Name())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
	for i, want := range []string{evKey, evPerf} {
		var rec map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &rec); err != nil {
			t.Fatal(err)
		}
		if rec["event"] != want || rec["wall"] == nil || rec["ms"] == nil {
			t.Errorf("line %d = %v, want event %s with wall and ms", i, rec, want)
		}
	}
}

func TestFrameClass(t *testing.T) {
	for _, tc := range []struct {
		d       statusCounts
		running bool
		want    string
	}{
		{statusCounts{Complete: 3, Idle: 9}, true, "complete"},
		{statusCounts{Idle: 5}, true, "idle"},
		{statusCounts{Blank: 1, Complete: 2}, true, "blank"},
		{statusCounts{}, true, "none"},
		{statusCounts{Complete: 4}, false, "stopped"},
	} {
		if got := frameClass(tc.d, tc.running); got != tc.want {
			t.Errorf("frameClass(%+v, %v) = %q, want %q", tc.d, tc.running, got, tc.want)
		}
	}
}
